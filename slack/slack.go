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

package slack

import (
	"encoding/json"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/tidal-open-source/cw-alert-router/cw"

	slackapi "github.com/slack-go/slack"
)

// Client wraps slack with simpler more specific calls suited for this lambda
type Client struct {
	slackClient  *slackapi.Client
	alternateURL string
	debug        bool
}

// ClientOptions provides the function opts pattern for overriding
type ClientOptions func(*Client)

// WithAlternativeURL supplies an alternative URL to make slack api calls
func WithAlternativeURL(url string) ClientOptions {
	return func(c *Client) {
		c.alternateURL = url
	}
}

// OptionDebug allows enabling/disabling debug
func OptionDebug(d bool) ClientOptions {
	return func(c *Client) {
		c.debug = d
	}
}

// New returns a newly initialized slack client
func New(slackAPIToken string, opts ...ClientOptions) (*Client, error) {
	if slackAPIToken == "" {
		return nil, fmt.Errorf("empty slack token provided")
	}

	c := &Client{}

	for _, opt := range opts {
		opt(c)
	}

	var api *slackapi.Client

	if c.alternateURL != "" {
		log.Infof("Initializing slack with alternate URL: %s", c.alternateURL)
		api = slackapi.New(slackAPIToken, slackapi.OptionAPIURL(c.alternateURL), slackapi.OptionDebug(c.debug))
	} else {
		api = slackapi.New(slackAPIToken, slackapi.OptionDebug(c.debug))
	}
	return &Client{slackClient: api}, nil
}

// CWAlarmHeaderBlock produces a slack block for the alarm details - regardless of state
func (c *Client) CWAlarmHeaderBlock(evt *cw.EventDetails, prefix string) *slackapi.SectionBlock {
	header := slackapi.NewTextBlockObject("mrkdwn", fmt.Sprintf("*%s Cloudwatch Alarm: %s*", prefix, evt.Detail.AlarmName), false, false)

	return slackapi.NewSectionBlock(header, nil, nil)
}

// CWAlarmSummary returns a slack block with the cloudwatch alarm summary (ie: metrics, reason, etc)
func (c *Client) CWAlarmSummary(evt *cw.EventDetails) *slackapi.SectionBlock {
	metricList := evt.MetricList()
	metricsStrings := make([]string, 0)
	if len(metricList[cw.MetricNamesKey]) > 0 {
		metricsStrings = append(metricsStrings, fmt.Sprintf("Names: %s", strings.Join(metricList[cw.MetricNamesKey], ",")))
	}
	if len(metricList[cw.MetricNamespacesKey]) > 0 {
		metricsStrings = append(metricsStrings, fmt.Sprintf("Namespaces: %s", strings.Join(metricList[cw.MetricNamespacesKey], ",")))
	}
	if len(metricList[cw.MetricDimensionsKey]) > 0 {
		metricsStrings = append(metricsStrings, fmt.Sprintf("Dimensions: %s", strings.Join(metricList[cw.MetricDimensionsKey], ",")))
	}

	var metricDataText string
	vals, cwerr := evt.GetMetricDataRequestForHrs()
	if cwerr != nil {
		metricDataText = fmt.Sprintf("error fetching metrics: %v", cwerr)
	} else {
		metricDataText = fmt.Sprintf("%.2f @ %s", *vals.Values[0], *vals.Timestamps[0])
	}

	var metricBlock *slackapi.TextBlockObject
	if len(metricsStrings) > 0 {
		metricBlock = slackapi.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Metrics*: `%s`\nRecent data: `%s`", strings.Join(metricsStrings, " - "), metricDataText), false, false)
	} else {
		metricBlock = slackapi.NewTextBlockObject("mrkdwn", "*Metrics*\n`None found`", false, false)
	}

	return slackapi.NewSectionBlock(metricBlock, nil, nil)
}

// ImageLink adds an image ref to the given url
func (c *Client) ImageLink(url string) *slackapi.ImageBlock {
	imageText := slackapi.NewTextBlockObject("plain_text", "MetricData", false, false)
	imageBlock := slackapi.NewImageBlock(url, "MetricData", "metricdata", imageText)
	return imageBlock
}

// CWAlarmLink adds a link to the cloudwatch console to the slack message
func (c *Client) CWAlarmLink(evt *cw.EventDetails) *slackapi.SectionBlock {
	link := evt.AlarmConsoleLink()
	linkBlock := slackapi.NewTextBlockObject("mrkdwn", fmt.Sprintf("Link: <%s|AWS Console>", link), false, false)
	return slackapi.NewSectionBlock(linkBlock, nil, nil)
}

// SendEventResolved will send a resolved message given the event details
func (c *Client) SendEventResolved(slackChannel string, evt *cw.EventDetails, imageLink string) (string, string, error) {
	var blocks slackapi.MsgOption
	header := c.CWAlarmHeaderBlock(evt, ":white_check_mark: (resolved)")
	info := c.CWAlarmSummary(evt)
	link := c.CWAlarmLink(evt)
	if imageLink != "" {
		graph := c.ImageLink(imageLink)
		json, jerr := json.Marshal(slackapi.NewBlockMessage(header, info, graph, link))
		log.Debugf("blocks json: %+v err: %v", string(json), jerr)
		blocks = slackapi.MsgOptionBlocks(header, info, graph, link)
	} else {
		blocks = slackapi.MsgOptionBlocks(header, info, link)
	}

	return c.SendMessage(slackChannel, blocks)
}

// SendEventTriggered will send a triggered message given the event details
func (c *Client) SendEventTriggered(slackChannel string, evt *cw.EventDetails, imageLink string) (string, string, error) {
	var blocks slackapi.MsgOption
	header := c.CWAlarmHeaderBlock(evt, ":red_alert_parrot: (triggered)")
	info := c.CWAlarmSummary(evt)
	link := c.CWAlarmLink(evt)
	if imageLink != "" {
		graph := c.ImageLink(imageLink)
		json, jerr := json.Marshal(slackapi.NewBlockMessage(header, info, graph, link))
		log.Debugf("blocks json: %+v err: %v", string(json), jerr)
		blocks = slackapi.MsgOptionBlocks(header, info, graph, link)
	} else {
		blocks = slackapi.MsgOptionBlocks(header, info, link)
	}

	return c.SendMessage(slackChannel, blocks)
}

// SendMessage sends a message to a slack channel
func (c *Client) SendMessage(channel string, opts ...slackapi.MsgOption) (string, string, error) {
	log.Infof("slack.SendMessage: Sending message to channel %s", channel)
	if c == nil {
		log.Fatalf("self is nil!")
	}
	if c.slackClient == nil {
		log.Fatalf("slack client is nil!")
	}
	channelID, timestamp, err := c.slackClient.PostMessage(
		channel,
		opts...,
	)
	log.Infof("channelID: %s, ts: %s, err: %v", channelID, timestamp, err)

	if err != nil {
		return "", "", err
	}
	return channelID, timestamp, nil
}

// SendSimpleTextMessage sends a message to a slack channel
func (c *Client) SendSimpleTextMessage(channel string, message string) (string, string, error) {
	return c.SendMessage(channel, slackapi.MsgOptionText(message, false))
}
