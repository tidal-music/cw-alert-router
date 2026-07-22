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
	testARN := "arn:aws:cloudwatch:us-east-1:1234567890123:alarm:test-service-alarm-abcd"
	end := time.Date(2020, time.July, 31, 6, 56, 5, 0, time.UTC)

	png, err := client.AlarmWidgetImage(context.Background(), testARN, end, 3*time.Hour)
	if err != nil {
		t.Fatalf("Error rendering widget image: %v", err)
	}
	if !bytes.Equal(png, test.TestPNG) {
		t.Errorf("returned image didn't match mock response")
	}

	// the widget definition should annotate with the alarm ARN and use the
	// aligned time window
	var widget struct {
		Start       string `json:"start"`
		End         string `json:"end"`
		Annotations struct {
			Alarms []string `json:"alarms"`
		} `json:"annotations"`
	}
	if err := json.Unmarshal([]byte(api.LastWidgetJSON), &widget); err != nil {
		t.Fatalf("widget definition is not valid JSON: %v (%s)", err, api.LastWidgetJSON)
	}
	if len(widget.Annotations.Alarms) != 1 || widget.Annotations.Alarms[0] != testARN {
		t.Errorf("expected alarm annotation [%s], got %v", testARN, widget.Annotations.Alarms)
	}
	if widget.End != "2020-07-31T06:56:05Z" {
		t.Errorf("expected end 2020-07-31T06:56:05Z, got %s", widget.End)
	}
	if !strings.HasPrefix(widget.Start, "2020-07-31T03:56:05") {
		t.Errorf("expected start 3 hours before end, got %s", widget.Start)
	}
}
