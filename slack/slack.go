// Copyright 2022 Aspiro AB
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package slack wraps the Slack API with calls specific to alarm notifications.
package slack

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	slackapi "github.com/slack-go/slack"

	"github.com/tidal-music/cw-alert-router/v2/cw"
)

// Emoji prefixes for the alarm state headers.
const (
	triggeredPrefix = ":rotating_light: (triggered)"
	resolvedPrefix  = ":white_check_mark: (resolved)"
)

// Retry budget for referencing a just-uploaded file from an image block.
const (
	uploadedFileAttempts   = 3
	uploadedFileRetryDelay = 500 * time.Millisecond
)

// ImageRef points at a graph image to embed in a message: either a public
// URL, or the ID of a file already uploaded to Slack. The zero value means
// "no image".
type ImageRef struct {
	URL         string
	SlackFileID string
}

// Client wraps slack with simpler more specific calls suited for this lambda.
type Client struct {
	api          *slackapi.Client
	alternateURL string
	debug        bool
}

// ClientOptions provides the function opts pattern for overriding.
type ClientOptions func(*Client)

// WithAlternativeURL supplies an alternative URL to make slack api calls (for testing).
func WithAlternativeURL(url string) ClientOptions {
	return func(c *Client) {
		c.alternateURL = url
	}
}

// OptionDebug allows enabling/disabling debug.
func OptionDebug(d bool) ClientOptions {
	return func(c *Client) {
		c.debug = d
	}
}

// New returns a newly initialized slack client.
func New(slackAPIToken string, opts ...ClientOptions) (*Client, error) {
	if slackAPIToken == "" {
		return nil, fmt.Errorf("empty slack token provided")
	}

	c := &Client{}
	for _, opt := range opts {
		opt(c)
	}

	apiOpts := []slackapi.Option{slackapi.OptionDebug(c.debug)}
	if c.alternateURL != "" {
		slog.Info("initializing slack client with alternate URL", "url", c.alternateURL)
		apiOpts = append(apiOpts, slackapi.OptionAPIURL(c.alternateURL))
	}
	c.api = slackapi.New(slackAPIToken, apiOpts...)
	return c, nil
}

// HeaderBlock produces a slack block for the alarm details - regardless of state.
func (c *Client) HeaderBlock(evt *cw.Event, prefix string) *slackapi.SectionBlock {
	header := slackapi.NewTextBlockObject(slackapi.MarkdownType,
		fmt.Sprintf("*%s CloudWatch Alarm: %s*", prefix, evt.Detail.AlarmName), false, false)
	return slackapi.NewSectionBlock(header, nil, nil)
}

// SummaryBlock returns a slack block with the alarm summary (metrics and state reason).
func (c *Client) SummaryBlock(evt *cw.Event) *slackapi.SectionBlock {
	summary := evt.MetricSummary()
	var parts []string
	if len(summary.Names) > 0 {
		parts = append(parts, fmt.Sprintf("Names: %s", strings.Join(summary.Names, ",")))
	}
	if len(summary.Namespaces) > 0 {
		parts = append(parts, fmt.Sprintf("Namespaces: %s", strings.Join(summary.Namespaces, ",")))
	}
	if len(summary.Dimensions) > 0 {
		parts = append(parts, fmt.Sprintf("Dimensions: %s", strings.Join(summary.Dimensions, ",")))
	}
	if len(summary.Expressions) > 0 {
		parts = append(parts, fmt.Sprintf("Expressions: %s", strings.Join(summary.Expressions, ",")))
	}

	var text string
	if len(parts) > 0 {
		text = fmt.Sprintf("*Metrics*: `%s`", strings.Join(parts, " - "))
	} else {
		text = "*Metrics*\n`None found`"
	}
	if reason := evt.Detail.State.Reason; reason != "" {
		text = fmt.Sprintf("%s\nReason: `%s`", text, reason)
	}

	block := slackapi.NewTextBlockObject(slackapi.MarkdownType, text, false, false)
	return slackapi.NewSectionBlock(block, nil, nil)
}

// LinkBlock adds a link to the CloudWatch console to the slack message.
func (c *Client) LinkBlock(evt *cw.Event) *slackapi.SectionBlock {
	link := slackapi.NewTextBlockObject(slackapi.MarkdownType,
		fmt.Sprintf("Link: <%s|AWS Console>", evt.ConsoleLink()), false, false)
	return slackapi.NewSectionBlock(link, nil, nil)
}

// imageBlock builds an image block from an ImageRef, or nil for the zero value.
func (c *Client) imageBlock(img ImageRef) *slackapi.ImageBlock {
	title := slackapi.NewTextBlockObject(slackapi.PlainTextType, "MetricData", false, false)
	if img.SlackFileID != "" {
		return &slackapi.ImageBlock{
			Type:      slackapi.MBTImage,
			SlackFile: &slackapi.SlackFileObject{ID: img.SlackFileID},
			AltText:   "metric graph",
			BlockID:   "metricdata",
			Title:     title,
		}
	}
	if img.URL != "" {
		return slackapi.NewImageBlock(img.URL, "metric graph", "metricdata", title)
	}
	return nil
}

// UploadImage uploads a PNG to Slack (unshared) and returns its file ID for
// referencing from an image block.
func (c *Client) UploadImage(ctx context.Context, filename string, png []byte) (string, error) {
	file, err := c.api.UploadFileContext(ctx, slackapi.UploadFileParameters{
		Filename: filename,
		Title:    filename,
		FileSize: len(png),
		Reader:   bytes.NewReader(png),
	})
	if err != nil {
		return "", fmt.Errorf("uploading %s to slack: %w", filename, err)
	}
	return file.ID, nil
}

// SendEventResolved will send a resolved message given the event details.
func (c *Client) SendEventResolved(ctx context.Context, channel string, evt *cw.Event, img ImageRef) (string, string, error) {
	return c.sendEvent(ctx, channel, evt, img, resolvedPrefix)
}

// SendEventTriggered will send a triggered message given the event details.
func (c *Client) SendEventTriggered(ctx context.Context, channel string, evt *cw.Event, img ImageRef) (string, string, error) {
	return c.sendEvent(ctx, channel, evt, img, triggeredPrefix)
}

func (c *Client) sendEvent(ctx context.Context, channel string, evt *cw.Event, img ImageRef, prefix string) (string, string, error) {
	buildBlocks := func(withImage bool) []slackapi.Block {
		blocks := []slackapi.Block{c.HeaderBlock(evt, prefix), c.SummaryBlock(evt)}
		if withImage {
			if imgBlock := c.imageBlock(img); imgBlock != nil {
				blocks = append(blocks, imgBlock)
			}
		}
		blocks = append(blocks, c.LinkBlock(evt))
		return blocks
	}

	// A freshly uploaded slack file can take a moment before it is
	// referenceable from an image block; retry briefly, then fall back to
	// sending without the graph - the alert matters more than the image.
	var lastErr error
	for attempt := 0; attempt < uploadedFileAttempts; attempt++ {
		id, ts, err := c.SendMessage(ctx, channel, slackapi.MsgOptionBlocks(buildBlocks(true)...))
		if err == nil {
			return id, ts, nil
		}
		lastErr = err
		if img.SlackFileID == "" || !isTransientFileError(err) {
			return "", "", err
		}
		slog.Warn("uploaded slack file not referenceable yet, retrying", "attempt", attempt+1, "error", err)
		select {
		case <-ctx.Done():
			return "", "", ctx.Err()
		case <-time.After(uploadedFileRetryDelay):
		}
	}
	slog.Error("giving up embedding the uploaded graph, sending without it", "error", lastErr)
	return c.SendMessage(ctx, channel, slackapi.MsgOptionBlocks(buildBlocks(false)...))
}

// isTransientFileError reports whether a postMessage failure looks like the
// uploaded file simply isn't ready to be referenced yet.
func isTransientFileError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "invalid_blocks") || strings.Contains(msg, "file_not_found")
}

// SendMessage sends a message to a slack channel and returns the channel ID and timestamp.
func (c *Client) SendMessage(ctx context.Context, channel string, opts ...slackapi.MsgOption) (string, string, error) {
	slog.Info("sending slack message", "channel", channel)
	channelID, timestamp, err := c.api.PostMessageContext(ctx, channel, opts...)
	if err != nil {
		return "", "", fmt.Errorf("posting slack message to %s: %w", channel, err)
	}
	return channelID, timestamp, nil
}

// SendSimpleTextMessage sends a plain text message to a slack channel.
func (c *Client) SendSimpleTextMessage(ctx context.Context, channel string, message string) (string, string, error) {
	return c.SendMessage(ctx, channel, slackapi.MsgOptionText(message, false))
}
