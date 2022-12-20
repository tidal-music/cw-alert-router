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

package lambda_test

import (
	"testing"

	"github.com/tidal-open-source/cw-alert-router/cw"
	"github.com/tidal-open-source/cw-alert-router/lambda"
)

var testTagsWithNoOwnerOrSlackOverride cw.AlarmTags = cw.AlarmTags{
	lambda.DefaultServiceNameTagKey: "test-service",
}

var testTagsWithoutSlackOverride cw.AlarmTags = cw.AlarmTags{
	lambda.DefaultOwnerTagKey:       "plateng",
	lambda.DefaultServiceNameTagKey: "test-service",
}

var testTagsWithSlackOverride cw.AlarmTags = cw.AlarmTags{
	lambda.DefaultOwnerTagKey:                "plateng",
	lambda.DefaultServiceNameTagKey:          "test-service",
	lambda.DefaultSlackChannelOverrideTagKey: "plateng-alarms",
}

func TestGetSlackChannelFromOwner(t *testing.T) {
	cfg := &lambda.Config{
		DefaultSlackChannel: "test-alarms",
		OwnerTagKey:         lambda.DefaultOwnerTagKey,
	}
	lambda.SetConfig(cfg)
	expectedSlackChannel := "plateng-alarms"
	channel := lambda.GetSlackChannel(testTagsWithoutSlackOverride)
	if channel != expectedSlackChannel {
		t.Errorf("channel (%s) didn't match expected (%s)", channel, expectedSlackChannel)
	}
}

func TestSlackChannelOverrideFromTags(t *testing.T) {
	cfg := &lambda.Config{
		DefaultSlackChannel: "test-alarms",
	}
	lambda.SetConfig(cfg)
	expectedSlackChannel := "plateng-alarms"
	channel := lambda.GetSlackChannel(testTagsWithSlackOverride)
	if channel != expectedSlackChannel {
		t.Errorf("channel (%s) didn't match expected (%s)", channel, expectedSlackChannel)
	}
}

func TestSlackChannelWithNoTags(t *testing.T) {
	cfg := &lambda.Config{
		DefaultSlackChannel: "test-alarms",
	}
	lambda.SetConfig(cfg)
	expectedSlackChannel := "test-alarms"
	channel := lambda.GetSlackChannel(testTagsWithNoOwnerOrSlackOverride)
	if channel != expectedSlackChannel {
		t.Errorf("channel (%s) didn't match expected (%s)", channel, expectedSlackChannel)
	}
}
