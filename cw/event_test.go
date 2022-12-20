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

	"github.com/tidal-open-source/cw-alert-router/cw"
	"github.com/tidal-open-source/cw-alert-router/test"
)

var cwClient *cw.Client

func setup(t *testing.T) {
	cwClient = &cw.Client{
		CWClient: &test.MockCWClient{},
	}
}

func TestAlarmParse(t *testing.T) {
	dat := []byte(test.TestEventJSON)
	d := &cw.EventDetails{}
	if err := json.Unmarshal(dat, d); err != nil {
		t.Fatalf("Error unmarshaling: %v", err)
	}
	if !reflect.DeepEqual(d, &test.ExpectedAlarmDetails) {
		expectedBytes, err := json.Marshal(test.ExpectedAlarmDetails)
		if err != nil {
			t.Fatalf("Error marshaling: %v", err)
		}
		parsedBytes, err := json.Marshal(d)
		if err != nil {
			t.Fatalf("Error marshaling: %v", err)
		}
		err = test.DiffJSON(t, parsedBytes, expectedBytes)
		if err != nil {
			t.Errorf("diffJSON err: %v\nobject dumps:\nobj1 (parsed):   %+v\nobj2 (expected): %+v\n", err, d, test.ExpectedAlarmDetails)
		} else {
			t.Errorf("request didn't match - see diff for details.")
		}
	}
}

func TestEventMetricList(t *testing.T) {
	list := test.TriggeredAlarmDetails.MetricList()

	if len(list[cw.MetricNamespacesKey]) != 1 {
		t.Errorf("Expected 1 namespace in the cloudwatch sample alarm - got %d", len(list[cw.MetricNamespacesKey]))
	} else {
		t.Logf("found namespace: %s", list[cw.MetricNamespacesKey][0])
	}

	if len(list[cw.MetricNamesKey]) != 1 {
		t.Errorf("Expected 1 name in the metrics, got %d", len(list[cw.MetricNamesKey]))
	} else {
		t.Logf("found metric name: %s", list[cw.MetricNamesKey][0])
	}

	if len(list[cw.MetricDimensionsKey]) != 1 {
		t.Errorf("Expected 1 dimension in the metrics, got %d", len(list[cw.MetricDimensionsKey]))
	} else {
		t.Logf("found metric dimension: %s", list[cw.MetricDimensionsKey][0])
	}
}

func TestAlarmLink(t *testing.T) {
	expectedURL := "https://console.aws.amazon.com/cloudwatch/home?region=us-east-1#alarmsV2:alarm/test-service-alarm-abcd"
	alarm := test.TriggeredAlarmDetails
	url := alarm.AlarmConsoleLink()
	if url != expectedURL {
		t.Errorf("Did not get expected url (%s) - instead, we got this: %s", expectedURL, url)
	}
}

func TestGetMetricDataFor1Hr(t *testing.T) {
	setup(t)
	expectedValues := test.GetMetricDataResponses["m0"].MetricDataResults[0].Values
	eventDetail := &test.TriggeredAlarmDetails
	eventDetail.SetCWClient(cwClient)
	data, err := eventDetail.GetMetricDataRequestForHrs()
	if err != nil {
		t.Fatalf("error fetching metric data: %v", err)
	}
	if data == nil {
		t.Fatalf("Data was nil!")
	}
	if !reflect.DeepEqual(expectedValues, data.Values) {
		t.Errorf("Values didn't match.  expected: %+v, got: %+v", expectedValues, data.Values)
	}
	t.Logf("metric data: %+v", data)
}
