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
	"testing"

	"github.com/tidal-open-source/cw-alert-router/pagerduty"
	"github.com/tidal-open-source/cw-alert-router/test"
)

var (
	pdclient pagerduty.PDAPIClientInterface
	client   *pagerduty.Client
)

func setup(t *testing.T) {
	var err error
	pdclient = &test.MockPDClient{}
	client, err = pagerduty.New(pagerduty.WithPDAPIClient(pdclient))
	if err != nil {
		t.Fatalf("Failed creating pagerduty client: %v", err)
	}
}

func TestPDCallOKAlarm(t *testing.T) {
	setup(t)
	err := client.SubmitEvent("abc123", &test.ExpectedAlarmDetails)
	if err != nil {
		t.Errorf("Failed sending event to pagerduty: %v", err)
	}
}

func TestPDEventAction(t *testing.T) {
	setup(t)
	e := &test.TriggeredAlarmDetails
	pdAction := client.EventAction(e)
	expectedAction := "trigger"
	if pdAction != expectedAction {
		t.Errorf("action %s didn't match expected: %s", pdAction, expectedAction)
	}
	e = &test.InsufficientDataToOKAlarmDetails
	pdAction = client.EventAction(e)
	expectedAction = ""
	if pdAction != expectedAction {
		t.Errorf("action %s didn't match expected: %s", pdAction, expectedAction)
	}
}

func TestSuppressPagerDuty(t *testing.T) {
	setup(t)
	e := &test.SuppressPagerDutyTrue
	pdAction := client.EventAction(e)
	expectedAction := ""
	if pdAction != expectedAction {
		t.Errorf("action #{pdAction} didn't match expected: #{expectedAction}")
	}
}
