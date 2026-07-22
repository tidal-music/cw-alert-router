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

package cw

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
)

// DefaultGraphWindow is how much metric history the alarm graph shows.
const DefaultGraphWindow = 3 * time.Hour

// Default size of rendered alarm graphs.
const (
	DefaultGraphWidth  = 600
	DefaultGraphHeight = 300
)

// API is the subset of the CloudWatch API this service uses.
type API interface {
	ListTagsForResource(ctx context.Context, params *cloudwatch.ListTagsForResourceInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.ListTagsForResourceOutput, error)
	GetMetricWidgetImage(ctx context.Context, params *cloudwatch.GetMetricWidgetImageInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricWidgetImageOutput, error)
}

// Client provides the CloudWatch calls this service needs.
type Client struct {
	api API
}

// NewClient returns a Client backed by the real CloudWatch API.
func NewClient(cfg aws.Config) *Client {
	return &Client{api: cloudwatch.NewFromConfig(cfg)}
}

// NewClientWithAPI returns a Client backed by the given API implementation (for testing).
func NewClientWithAPI(api API) *Client {
	return &Client{api: api}
}

// AlarmTags returns the AWS tags on the given alarm as a map.
func (c *Client) AlarmTags(ctx context.Context, alarmARN string) (map[string]string, error) {
	resp, err := c.api.ListTagsForResource(ctx, &cloudwatch.ListTagsForResourceInput{
		ResourceARN: aws.String(alarmARN),
	})
	if err != nil {
		return nil, fmt.Errorf("listing tags for %s: %w", alarmARN, err)
	}
	tags := make(map[string]string, len(resp.Tags))
	for _, tag := range resp.Tags {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}
	return tags, nil
}

// widget is the metric-widget definition passed to GetMetricWidgetImage.
// Annotating with the alarm ARN makes CloudWatch render the alarm's own
// metrics (including metric math) with its threshold line.
type widget struct {
	Width       int               `json:"width"`
	Height      int               `json:"height"`
	Start       string            `json:"start"`
	End         string            `json:"end"`
	Annotations widgetAnnotations `json:"annotations"`
}

type widgetAnnotations struct {
	Alarms []string `json:"alarms"`
}

// AlarmWidgetImage renders a PNG graph of the alarm's metrics (with threshold
// annotation) ending at the given time and spanning the given window.
func (c *Client) AlarmWidgetImage(ctx context.Context, alarmARN string, end time.Time, window time.Duration) ([]byte, error) {
	if window <= 0 {
		window = DefaultGraphWindow
	}
	def, err := json.Marshal(widget{
		Width:       DefaultGraphWidth,
		Height:      DefaultGraphHeight,
		Start:       end.Add(-window).UTC().Format(time.RFC3339),
		End:         end.UTC().Format(time.RFC3339),
		Annotations: widgetAnnotations{Alarms: []string{alarmARN}},
	})
	if err != nil {
		return nil, err
	}
	resp, err := c.api.GetMetricWidgetImage(ctx, &cloudwatch.GetMetricWidgetImageInput{
		MetricWidget: aws.String(string(def)),
	})
	if err != nil {
		return nil, fmt.Errorf("rendering metric widget image for %s: %w", alarmARN, err)
	}
	return resp.MetricWidgetImage, nil
}
