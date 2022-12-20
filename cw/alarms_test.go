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
	"testing"

	"github.com/tidal-open-source/cw-alert-router/cw"
	"github.com/tidal-open-source/cw-alert-router/test"
)

var a *cw.Client

func setupAlarms(t *testing.T) {
	a = &cw.Client{
		CWClient: &test.MockCWClient{},
	}
}

func TestGetTags(t *testing.T) {
	setupAlarms(t)
	testARN := "arn:aws:cloudwatch:us-east-1:1234567890123:alarm:test-service-alarm-abcd"
	tags, err := a.GetAlarmTags(testARN)
	if err != nil {
		t.Errorf("Error getting tags: %v", err)
	}
	expectedTag := map[string]string{
		"owner": "test",
	}
	foundTag := false
	for k, v := range tags {
		if val, ok := expectedTag[k]; ok {
			if v == val {
				foundTag = true
			}
		}
	}
	if !foundTag {
		t.Errorf("did not find expected tag: %+v", expectedTag)
	}
}
