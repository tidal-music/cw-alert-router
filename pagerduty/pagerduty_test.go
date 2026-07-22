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

package pagerduty_test

import (
	"context"
	"testing"

	"github.com/tidal-music/cw-alert-router/v2/pagerduty"
	"github.com/tidal-music/cw-alert-router/v2/test"
)

func TestSubmitEvent(t *testing.T) {
	mock := &test.MockPDClient{}
	client, err := pagerduty.New(pagerduty.WithAPI(mock))
	if err != nil {
		t.Fatalf("Failed creating pagerduty client: %v", err)
	}

	evt := test.TriggeredAlarmDetails
	if err := client.SubmitEvent(context.Background(), "abc123", pagerduty.ActionTrigger, &evt); err != nil {
		t.Errorf("Failed sending event to pagerduty: %v", err)
	}

	events := mock.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 pagerduty event, got %d", len(events))
	}
	if events[0].Action != pagerduty.ActionTrigger {
		t.Errorf("expected trigger action, got %s", events[0].Action)
	}
	if events[0].DedupKey != "arn:aws:cloudwatch:us-east-1:1234567890123:alarm:test-service-alarm-abcd" {
		t.Errorf("unexpected dedup key: %s", events[0].DedupKey)
	}
}

func TestSubmitEventNoneAction(t *testing.T) {
	mock := &test.MockPDClient{}
	client, err := pagerduty.New(pagerduty.WithAPI(mock))
	if err != nil {
		t.Fatalf("Failed creating pagerduty client: %v", err)
	}

	evt := test.ExpectedAlarmDetails
	if err := client.SubmitEvent(context.Background(), "abc123", pagerduty.ActionNone, &evt); err != nil {
		t.Errorf("SubmitEvent with ActionNone should be a no-op, got error: %v", err)
	}
	if len(mock.Events()) != 0 {
		t.Errorf("expected no pagerduty events, got %d", len(mock.Events()))
	}
}

func TestAction(t *testing.T) {
	tests := []struct {
		previous, current, expected string
	}{
		{"OK", "ALARM", pagerduty.ActionTrigger},
		{"INSUFFICIENT_DATA", "ALARM", pagerduty.ActionTrigger},
		{"ALARM", "OK", pagerduty.ActionResolve},
		{"INSUFFICIENT_DATA", "OK", pagerduty.ActionNone},
		{"OK", "INSUFFICIENT_DATA", pagerduty.ActionNone},
		{"ALARM", "INSUFFICIENT_DATA", pagerduty.ActionNone},
	}
	for _, tc := range tests {
		if got := pagerduty.Action(tc.previous, tc.current); got != tc.expected {
			t.Errorf("Action(%s -> %s) = %q, expected %q", tc.previous, tc.current, got, tc.expected)
		}
	}
}
