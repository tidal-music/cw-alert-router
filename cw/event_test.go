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
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/tidal-music/cw-alert-router/v2/cw"
	"github.com/tidal-music/cw-alert-router/v2/test"
)

func TestAlarmParse(t *testing.T) {
	d := &cw.Event{}
	if err := json.Unmarshal([]byte(test.TestEventJSON), d); err != nil {
		t.Fatalf("Error unmarshaling: %v", err)
	}
	if !reflect.DeepEqual(d, &test.ExpectedAlarmDetails) {
		t.Errorf("parsed event didn't match.\nparsed:   %+v\nexpected: %+v", d, test.ExpectedAlarmDetails)
	}
}

func TestMetricSummary(t *testing.T) {
	summary := test.TriggeredAlarmDetails.MetricSummary()

	if len(summary.Namespaces) != 1 || summary.Namespaces[0] != "AWS/EC2" {
		t.Errorf("expected namespaces [AWS/EC2], got %v", summary.Namespaces)
	}
	if len(summary.Names) != 1 || summary.Names[0] != "CPUUtilization" {
		t.Errorf("expected names [CPUUtilization], got %v", summary.Names)
	}
	if len(summary.Dimensions) != 1 || summary.Dimensions[0] != "AutoScalingGroupName:test-service" {
		t.Errorf("expected dimensions [AutoScalingGroupName:test-service], got %v", summary.Dimensions)
	}
}

func TestMetricSummaryWithExpression(t *testing.T) {
	evt := cw.Event{
		Detail: cw.AlarmStateChange{
			Configuration: cw.Configuration{
				Metrics: []cw.MetricDataQuery{
					{ID: "e1", Expression: "m1/m2*100", ReturnData: true},
					{ID: "m1", MetricStat: &cw.MetricStat{Stat: "Sum", Period: 60, Metric: cw.Metric{Namespace: "AWS/SQS", Name: "NumberOfMessagesReceived"}}},
					{ID: "m2", MetricStat: &cw.MetricStat{Stat: "Sum", Period: 60, Metric: cw.Metric{Namespace: "AWS/SQS", Name: "NumberOfMessagesSent"}}},
				},
			},
		},
	}
	summary := evt.MetricSummary()
	if len(summary.Expressions) != 1 || summary.Expressions[0] != "m1/m2*100" {
		t.Errorf("expected expressions [m1/m2*100], got %v", summary.Expressions)
	}
	if len(summary.Names) != 2 {
		t.Errorf("expected 2 metric names, got %v", summary.Names)
	}
}

func TestConsoleLink(t *testing.T) {
	expectedURL := "https://console.aws.amazon.com/cloudwatch/home?region=us-east-1#alarmsV2:alarm/test-service-alarm-abcd"
	url := test.TriggeredAlarmDetails.ConsoleLink()
	if url != expectedURL {
		t.Errorf("Did not get expected url (%s) - instead, we got this: %s", expectedURL, url)
	}
}

func TestAlarmARN(t *testing.T) {
	arn, err := test.TriggeredAlarmDetails.AlarmARN()
	if err != nil {
		t.Fatalf("AlarmARN returned error: %v", err)
	}
	expected := "arn:aws:cloudwatch:us-east-1:1234567890123:alarm:test-service-alarm-abcd"
	if arn != expected {
		t.Errorf("arn (%s) didn't match expected (%s)", arn, expected)
	}

	empty := cw.Event{}
	if _, err := empty.AlarmARN(); err == nil {
		t.Errorf("expected error for event without resources")
	}
}

func TestStateChangeTime(t *testing.T) {
	got := test.TriggeredAlarmDetails.StateChangeTime().UTC()
	want := time.Date(2020, time.July, 31, 6, 56, 5, 606000000, time.UTC)
	if !got.Equal(want) {
		t.Errorf("state change time (%v) didn't match expected (%v)", got, want)
	}

	// falls back to the envelope time when the state timestamp is missing
	evt := cw.Event{Time: "2020-07-31T06:56:05Z"}
	got = evt.StateChangeTime().UTC()
	want = time.Date(2020, time.July, 31, 6, 56, 5, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("fallback state change time (%v) didn't match expected (%v)", got, want)
	}
}
