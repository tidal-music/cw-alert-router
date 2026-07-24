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

package cw_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tidal-music/cw-alert-router/v2/cw"
	"github.com/tidal-music/cw-alert-router/v2/test"
)

func TestAlarmTags(t *testing.T) {
	client := cw.NewClientWithAPI(&test.MockCWAPI{})
	testARN := "arn:aws:cloudwatch:us-east-1:1234567890123:alarm:test-service-alarm-abcd"

	tags, err := client.AlarmTags(context.Background(), testARN)
	if err != nil {
		t.Fatalf("Error getting tags: %v", err)
	}
	if tags["owner"] != "test" {
		t.Errorf("expected owner tag 'test', got %q (all tags: %v)", tags["owner"], tags)
	}

	if _, err := client.AlarmTags(context.Background(), "arn:aws:cloudwatch:us-east-1:1234567890123:alarm:nonexistent"); err == nil {
		t.Errorf("expected error for unknown alarm arn")
	}
}

func TestAlarmWidgetImage(t *testing.T) {
	api := &test.MockCWAPI{}
	client := cw.NewClientWithAPI(api)
	evt := test.ExpectedAlarmDetails
	end := time.Date(2020, time.July, 31, 6, 56, 5, 0, time.UTC)

	png, err := client.AlarmWidgetImage(context.Background(), &evt, end, 3*time.Hour)
	if err != nil {
		t.Fatalf("Error rendering widget image: %v", err)
	}
	if !bytes.Equal(png, test.TestPNG) {
		t.Errorf("returned image didn't match mock response")
	}

	// the widget definition should carry the alarm's metric, its threshold as
	// a horizontal annotation, and the aligned time window. The auto-generated
	// UUID metric id must be dropped (GetMetricWidgetImage rejects it).
	var widget struct {
		Start       string  `json:"start"`
		End         string  `json:"end"`
		Metrics     [][]any `json:"metrics"`
		Annotations struct {
			Horizontal []struct {
				Value float64 `json:"value"`
			} `json:"horizontal"`
		} `json:"annotations"`
	}
	if err := json.Unmarshal([]byte(api.LastWidgetJSON), &widget); err != nil {
		t.Fatalf("widget definition is not valid JSON: %v (%s)", err, api.LastWidgetJSON)
	}
	wantRow := `["AWS/EC2","CPUUtilization","AutoScalingGroupName","test-service",{"period":60,"stat":"Average"}]`
	if got, _ := json.Marshal(widget.Metrics[0]); len(widget.Metrics) != 1 || string(got) != wantRow {
		t.Errorf("expected metrics [%s], got %s", wantRow, api.LastWidgetJSON)
	}
	if len(widget.Annotations.Horizontal) != 1 || widget.Annotations.Horizontal[0].Value != 60.0 {
		t.Errorf("expected horizontal threshold annotation at 60.0, got %s", api.LastWidgetJSON)
	}
	if widget.End != "2020-07-31T06:56:05Z" {
		t.Errorf("expected end 2020-07-31T06:56:05Z, got %s", widget.End)
	}
	if !strings.HasPrefix(widget.Start, "2020-07-31T03:56:05") {
		t.Errorf("expected start 3 hours before end, got %s", widget.Start)
	}
}

func TestAlarmWidgetImageMetricMath(t *testing.T) {
	api := &test.MockCWAPI{}
	client := cw.NewClientWithAPI(api)
	evt := test.ExpectedAlarmDetails
	evt.Detail.State.ReasonData = "{}" // no threshold available
	evt.Detail.Configuration = cw.Configuration{
		Metrics: []cw.MetricDataQuery{
			{ID: "e1", Expression: "m1*2", Label: "doubled", ReturnData: true},
			{ID: "m1", ReturnData: false, MetricStat: &cw.MetricStat{
				Stat:   "Sum",
				Period: 300,
				Metric: cw.Metric{
					Namespace:  "AWS/SQS",
					Name:       "NumberOfMessagesSent",
					Dimensions: map[string]string{"QueueName": "test-queue"},
				},
			}},
		},
	}

	if _, err := client.AlarmWidgetImage(context.Background(), &evt, time.Now(), 0); err != nil {
		t.Fatalf("Error rendering widget image: %v", err)
	}

	// expression queries become option-object rows, hidden inputs keep their
	// id and get visible:false, and without a threshold there are no
	// annotations at all.
	wantMetrics := `[[{"expression":"m1*2","id":"e1","label":"doubled"}],` +
		`["AWS/SQS","NumberOfMessagesSent","QueueName","test-queue",{"id":"m1","period":300,"stat":"Sum","visible":false}]]`
	var widget map[string]json.RawMessage
	if err := json.Unmarshal([]byte(api.LastWidgetJSON), &widget); err != nil {
		t.Fatalf("widget definition is not valid JSON: %v (%s)", err, api.LastWidgetJSON)
	}
	if string(widget["metrics"]) != wantMetrics {
		t.Errorf("expected metrics %s, got %s", wantMetrics, widget["metrics"])
	}
	if _, ok := widget["annotations"]; ok {
		t.Errorf("expected no annotations without a threshold, got %s", api.LastWidgetJSON)
	}
}

func TestAlarmWidgetImageNoMetrics(t *testing.T) {
	client := cw.NewClientWithAPI(&test.MockCWAPI{})
	evt := test.ExpectedAlarmDetails
	evt.Detail.Configuration = cw.Configuration{}

	if _, err := client.AlarmWidgetImage(context.Background(), &evt, time.Now(), 0); err == nil {
		t.Errorf("expected error for an event without metrics (e.g. composite alarm)")
	}
}
