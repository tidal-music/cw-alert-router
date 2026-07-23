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
	"maps"
	"regexp"
	"slices"
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
// It is built from the metric queries in the alarm event payload:
// GetMetricWidgetImage does not accept the dashboard-only alarm annotation
// ("annotations": {"alarms": [...]}), so the alarm's threshold is drawn as a
// horizontal annotation instead.
type widget struct {
	Width       int                `json:"width"`
	Height      int                `json:"height"`
	Start       string             `json:"start"`
	End         string             `json:"end"`
	Metrics     [][]any            `json:"metrics"`
	Annotations *widgetAnnotations `json:"annotations,omitempty"`
}

type widgetAnnotations struct {
	Horizontal []widgetThreshold `json:"horizontal,omitempty"`
}

type widgetThreshold struct {
	Value float64 `json:"value"`
	Label string  `json:"label"`
}

// widgetIDPattern is the metric id format GetMetricWidgetImage accepts.
// CloudWatch auto-generates UUID ids for simple alarms; those are rejected
// by the API (and nothing references them), so they are dropped.
var widgetIDPattern = regexp.MustCompile(`^[a-z][a-zA-Z0-9_]*$`)

// widgetMetrics converts the alarm's metric queries to the widget "metrics"
// array syntax: one row per query, either a metric-math expression object or
// [namespace, name, dimName, dimValue, ..., {options}]. Queries with
// returnData=false (inputs to metric math) are included but hidden.
func widgetMetrics(queries []MetricDataQuery) [][]any {
	var rows [][]any
	for _, q := range queries {
		opts := map[string]any{}
		if widgetIDPattern.MatchString(q.ID) {
			opts["id"] = q.ID
		}
		if q.Label != "" {
			opts["label"] = q.Label
		}
		if !q.ReturnData {
			opts["visible"] = false
		}
		if q.Expression != "" {
			opts["expression"] = q.Expression
			rows = append(rows, []any{opts})
			continue
		}
		if q.MetricStat == nil {
			continue
		}
		opts["stat"] = q.MetricStat.Stat
		opts["period"] = q.MetricStat.Period
		m := q.MetricStat.Metric
		row := []any{m.Namespace, m.Name}
		for _, k := range slices.Sorted(maps.Keys(m.Dimensions)) {
			row = append(row, k, m.Dimensions[k])
		}
		rows = append(rows, append(row, opts))
	}
	return rows
}

// AlarmWidgetImage renders a PNG graph of the alarm's metrics (with its
// threshold as a horizontal annotation) ending at the given time and
// spanning the given window.
func (c *Client) AlarmWidgetImage(ctx context.Context, evt *Event, end time.Time, window time.Duration) ([]byte, error) {
	metrics := widgetMetrics(evt.Detail.Configuration.Metrics)
	if len(metrics) == 0 {
		return nil, fmt.Errorf("alarm %s has no metrics to graph", evt.Detail.AlarmName)
	}
	if window <= 0 {
		window = DefaultGraphWindow
	}
	w := widget{
		Width:   DefaultGraphWidth,
		Height:  DefaultGraphHeight,
		Start:   end.Add(-window).UTC().Format(time.RFC3339),
		End:     end.UTC().Format(time.RFC3339),
		Metrics: metrics,
	}
	if threshold, ok := evt.Threshold(); ok {
		w.Annotations = &widgetAnnotations{
			Horizontal: []widgetThreshold{{Value: threshold, Label: "threshold"}},
		}
	}
	def, err := json.Marshal(w)
	if err != nil {
		return nil, err
	}
	resp, err := c.api.GetMetricWidgetImage(ctx, &cloudwatch.GetMetricWidgetImageInput{
		MetricWidget: aws.String(string(def)),
	})
	if err != nil {
		return nil, fmt.Errorf("rendering metric widget image for %s: %w", evt.Detail.AlarmName, err)
	}
	return resp.MetricWidgetImage, nil
}
