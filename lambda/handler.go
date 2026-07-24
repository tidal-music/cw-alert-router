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

// Package lambda wires the alert-router together: it consumes CloudWatch
// alarm state changes from SQS and routes them to Slack and PagerDuty based
// on the alarm's AWS tags.
package lambda

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	awsevents "github.com/aws/aws-lambda-go/events"
	awslambda "github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/google/uuid"

	"github.com/tidal-music/cw-alert-router/v2/cw"
	"github.com/tidal-music/cw-alert-router/v2/pagerduty"
	"github.com/tidal-music/cw-alert-router/v2/parameterstore"
	"github.com/tidal-music/cw-alert-router/v2/s3"
	"github.com/tidal-music/cw-alert-router/v2/slack"
)

// presignTTL is how long presigned graph URLs stay valid (the SigV4 maximum).
const presignTTL = 7 * 24 * time.Hour

// graphFilename returns a unique PNG filename for the event's graph image.
func graphFilename(evt *cw.Event) string {
	name := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		default:
			return '-'
		}
	}, evt.Detail.AlarmName)
	return fmt.Sprintf("%s-%s.png", name, uuid.NewString())
}

// Handler processes CloudWatch alarm state change events from SQS.
type Handler struct {
	cfg Config

	cw *cw.Client
	ps *parameterstore.Client
	pd *pagerduty.Client
	s3 *s3.Client
	sl *slack.Client

	slackToken  string
	slackAPIURL string
}

// Option overrides a Handler dependency (mostly for testing).
type Option func(*Handler)

// WithCWClient allows overriding the CloudWatch client.
func WithCWClient(c *cw.Client) Option {
	return func(h *Handler) { h.cw = c }
}

// WithParameterStoreClient allows overriding the parameterstore client.
func WithParameterStoreClient(c *parameterstore.Client) Option {
	return func(h *Handler) { h.ps = c }
}

// WithPagerDutyClient allows overriding the pagerduty client.
func WithPagerDutyClient(c *pagerduty.Client) Option {
	return func(h *Handler) { h.pd = c }
}

// WithS3Client allows overriding the S3 client.
func WithS3Client(c *s3.Client) Option {
	return func(h *Handler) { h.s3 = c }
}

// WithSlackClient allows overriding the slack client.
func WithSlackClient(c *slack.Client) Option {
	return func(h *Handler) { h.sl = c }
}

// WithSlackToken sets the Slack token directly instead of fetching it from parameter store.
func WithSlackToken(token string) Option {
	return func(h *Handler) { h.slackToken = token }
}

// WithSlackAPIURL points the Slack client at an alternative API endpoint (for testing).
func WithSlackAPIURL(url string) Option {
	return func(h *Handler) { h.slackAPIURL = url }
}

// New builds a Handler from the given Config, initializing any dependencies
// that were not supplied via options.
func New(ctx context.Context, cfg Config, opts ...Option) (*Handler, error) {
	cfg = cfg.withDefaults()
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: cfg.slogLevel(),
	})))

	h := &Handler{cfg: cfg}
	for _, opt := range opts {
		opt(h)
	}

	if h.cw == nil || h.ps == nil {
		awscfg, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			return nil, fmt.Errorf("loading aws config: %w", err)
		}
		if h.cw == nil {
			h.cw = cw.NewClient(awscfg)
		}
		if h.ps == nil {
			h.ps = parameterstore.New(awscfg)
		}
	}

	if h.pd == nil {
		pd, err := pagerduty.New()
		if err != nil {
			return nil, err
		}
		h.pd = pd
	}

	if h.s3 == nil && cfg.GraphMode == GraphModeS3 {
		s3c, err := s3.New(ctx, s3.WithRegion(cfg.ImageBucketRegion), s3.WithRoleARN(cfg.ImageBucketRoleArn))
		if err != nil {
			return nil, err
		}
		h.s3 = s3c
	}

	if h.sl == nil {
		token := h.slackToken
		if token == "" {
			if cfg.SlackTokenSSMKey == "" {
				return nil, fmt.Errorf("slack token ssm key is required (%s)", SlackTokenSSMKeyEnv)
			}
			var err error
			token, err = h.ps.GetParameterValue(ctx, cfg.SlackTokenSSMKey)
			if err != nil {
				return nil, fmt.Errorf("fetching slack token from %s: %w", cfg.SlackTokenSSMKey, err)
			}
		}
		var slackOpts []slack.ClientOptions
		if h.slackAPIURL != "" {
			slackOpts = append(slackOpts, slack.WithAlternativeURL(h.slackAPIURL))
		}
		sl, err := slack.New(token, slackOpts...)
		if err != nil {
			return nil, err
		}
		h.sl = sl
	}

	return h, nil
}

// Config returns the handler's effective configuration.
func (h *Handler) Config() Config {
	return h.cfg
}

// OwnerFromTags returns the owning team name from the given tags.
func (h *Handler) OwnerFromTags(tags map[string]string) string {
	return tags[h.cfg.OwnerTagKey]
}

// ServiceNameFromTags returns the service name from the given tags.
func (h *Handler) ServiceNameFromTags(tags map[string]string) string {
	return tags[h.cfg.ServiceNameTagKey]
}

// SlackChannel determines which slack channel we send messages to:
//  1. the alerts:slack_channel override tag, if present
//  2. "<owner>-alarms" (lowercased) derived from the owner tag
//  3. the configured default channel
func (h *Handler) SlackChannel(tags map[string]string) string {
	if override := tags[SlackChannelOverrideTagKey]; override != "" {
		return override
	}
	if owner := h.OwnerFromTags(tags); owner != "" {
		return fmt.Sprintf("%s-alarms", strings.ToLower(owner))
	}
	return h.cfg.DefaultSlackChannel
}

// PagerDutyRoutingKey returns the routing key for the given service name:
//  1. if serviceName is empty, the default routing key
//  2. the parameter-store key <pattern>/<service_name> (lowercased,
//     hyphens replaced with underscores), if it exists and is non-empty
//  3. otherwise the default routing key
func (h *Handler) PagerDutyRoutingKey(ctx context.Context, serviceName string) (string, error) {
	if serviceName == "" {
		return h.cfg.DefaultPagerDutyRoutingKey, nil
	}
	name := strings.ReplaceAll(strings.ToLower(serviceName), "-", "_")
	key := fmt.Sprintf(h.cfg.PagerDutyRoutingKeySSMPattern, name)

	val, err := h.ps.GetParameterValue(ctx, key)
	if err != nil {
		if parameterstore.IsNotFound(err) {
			slog.Debug("no service-specific pagerduty routing key, using default", "ssm_key", key)
			return h.cfg.DefaultPagerDutyRoutingKey, nil
		}
		return "", fmt.Errorf("fetching pagerduty routing key %s: %w", key, err)
	}
	if val == "" {
		return h.cfg.DefaultPagerDutyRoutingKey, nil
	}
	slog.Debug("using service-specific pagerduty routing key", "ssm_key", key)
	return val, nil
}

// graphImage renders the alarm graph and returns a reference to embed in the
// Slack message. Failures are logged, not returned - a missing graph should
// never block an alert.
func (h *Handler) graphImage(ctx context.Context, evt *cw.Event) slack.ImageRef {
	if h.cfg.GraphMode == GraphModeNone {
		return slack.ImageRef{}
	}

	png, err := h.cw.AlarmWidgetImage(ctx, evt, evt.StateChangeTime(), cw.DefaultGraphWindow)
	if err != nil {
		slog.Error("failed rendering alarm graph", "alarm", evt.Detail.AlarmName, "error", err)
		return slack.ImageRef{}
	}

	switch h.cfg.GraphMode {
	case GraphModeSlack:
		fileID, err := h.sl.UploadImage(ctx, graphFilename(evt), png)
		if err != nil {
			slog.Error("failed uploading alarm graph to slack", "alarm", evt.Detail.AlarmName, "error", err)
			return slack.ImageRef{}
		}
		return slack.ImageRef{SlackFileID: fileID}
	case GraphModeS3:
		url, err := h.storeGraphInS3(ctx, evt, png)
		if err != nil {
			slog.Error("failed storing alarm graph in s3", "alarm", evt.Detail.AlarmName, "error", err)
			return slack.ImageRef{}
		}
		return slack.ImageRef{URL: url}
	}
	return slack.ImageRef{}
}

// storeGraphInS3 writes the graph PNG to the image bucket and returns a URL
// for it: either on the configured image host, or a presigned S3 URL.
func (h *Handler) storeGraphInS3(ctx context.Context, evt *cw.Event, png []byte) (string, error) {
	t := evt.StateChangeTime()
	key := fmt.Sprintf("%d/%02d/%02d/%s", t.Year(), t.Month(), t.Day(), graphFilename(evt))
	if prefix := strings.Trim(h.cfg.ImageBucketPrefix, "/"); prefix != "" {
		key = fmt.Sprintf("%s/%s", prefix, key)
	}

	slog.Info("writing graph image", "bucket", h.cfg.ImageBucket, "key", key)
	if err := h.s3.WriteBytes(ctx, h.cfg.ImageBucket, key, bytes.NewReader(png)); err != nil {
		return "", err
	}

	if h.cfg.ImageHost != "" {
		return fmt.Sprintf("%s/%s", strings.TrimRight(h.cfg.ImageHost, "/"), key), nil
	}
	return h.s3.PresignedURL(ctx, h.cfg.ImageBucket, key, presignTTL)
}

// ProcessEvent handles one CloudWatch alarm state change event.
func (h *Handler) ProcessEvent(ctx context.Context, evt *cw.Event) error {
	alarmARN, err := evt.AlarmARN()
	if err != nil {
		return err
	}

	action := pagerduty.Action(evt.Detail.PreviousState.Value, evt.Detail.State.Value)
	if action == pagerduty.ActionNone {
		slog.Info("ignoring alarm state transition",
			"alarm", evt.Detail.AlarmName,
			"previous", evt.Detail.PreviousState.Value,
			"current", evt.Detail.State.Value)
		return nil
	}

	tags, err := h.cw.AlarmTags(ctx, alarmARN)
	if err != nil {
		return fmt.Errorf("fetching alarm tags: %w", err)
	}

	channel := h.SlackChannel(tags)
	img := h.graphImage(ctx, evt)

	var channelID, ts string
	var slackErr error
	switch action {
	case pagerduty.ActionResolve:
		channelID, ts, slackErr = h.sl.SendEventResolved(ctx, channel, evt, img)
	case pagerduty.ActionTrigger:
		channelID, ts, slackErr = h.sl.SendEventTriggered(ctx, channel, evt, img)
	}
	if slackErr != nil {
		return slackErr
	}
	slog.Info("sent slack message", "channel_id", channelID, "timestamp", ts)

	if tags[SuppressPagerDutyTagKey] == "true" {
		slog.Info("pagerduty suppressed via tag", "alarm", evt.Detail.AlarmName)
		return nil
	}

	serviceName := h.ServiceNameFromTags(tags)
	routingKey, err := h.PagerDutyRoutingKey(ctx, serviceName)
	if err != nil {
		return err
	}
	if routingKey == "" {
		return fmt.Errorf("no pagerduty routing key available for service %q", serviceName)
	}
	return h.pd.SubmitEvent(ctx, routingKey, action, evt)
}

// HandleRequest is the main entrypoint for the lambda. Records are processed
// in order; on the first failure the failed record and every remaining record
// are reported as batch item failures, so only successfully processed records
// are deleted from the queue. Stopping at the first failure preserves FIFO
// queue ordering: successors are never handled before a failed record's
// redelivery. The SQS event source mapping must enable
// ReportBatchItemFailures.
func (h *Handler) HandleRequest(ctx context.Context, sqsEvent awsevents.SQSEvent) (awsevents.SQSEventResponse, error) {
	var resp awsevents.SQSEventResponse

	for i, msg := range sqsEvent.Records {
		slog.Info("processing sqs message", "message_id", msg.MessageId, "source", msg.EventSource)
		slog.Debug("sqs message body", "body", msg.Body)

		if err := h.processRecord(ctx, msg); err != nil {
			slog.Error("failed processing sqs message", "message_id", msg.MessageId, "error", err)
			for _, unprocessed := range sqsEvent.Records[i:] {
				resp.BatchItemFailures = append(resp.BatchItemFailures,
					awsevents.SQSBatchItemFailure{ItemIdentifier: unprocessed.MessageId})
			}
			break
		}
	}

	return resp, nil
}

// processRecord decodes and processes a single SQS record.
func (h *Handler) processRecord(ctx context.Context, msg awsevents.SQSMessage) error {
	evt := &cw.Event{}
	if err := json.Unmarshal([]byte(msg.Body), evt); err != nil {
		return fmt.Errorf("decoding sqs message body: %w", err)
	}
	return h.ProcessEvent(ctx, evt)
}

// Start begins the lambda handler.
func (h *Handler) Start() {
	awslambda.Start(h.HandleRequest)
}
