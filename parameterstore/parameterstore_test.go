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

package parameterstore_test

import (
	"context"
	"testing"

	"github.com/tidal-music/cw-alert-router/v2/parameterstore"
	"github.com/tidal-music/cw-alert-router/v2/test"
)

var (
	validParameterStoreKey      = "/service/cw_alert_router/pagerduty/routing_keys/shared_key"
	expectedParameterStoreValue = "shared-key-test-string"
	invalidParameterStoreKey    = "/service/nonexists/something"
)

func TestGetValidResponse(t *testing.T) {
	psclient := parameterstore.NewWithAPI(&test.MockSSMClient{})
	value, err := psclient.GetParameterValue(context.Background(), validParameterStoreKey)
	if err != nil {
		t.Errorf("Error fetching key %s: %v", validParameterStoreKey, err)
	}
	if value != expectedParameterStoreValue {
		t.Errorf("parameter store value for key (%s) - %s - didn't match expected (%s)", validParameterStoreKey, value, expectedParameterStoreValue)
	}
}

func TestGetInvalidKey(t *testing.T) {
	psclient := parameterstore.NewWithAPI(&test.MockSSMClient{})
	_, err := psclient.GetParameterValue(context.Background(), invalidParameterStoreKey)
	if err == nil {
		t.Fatalf("Expected error when fetching key %s, but no error was returned.", invalidParameterStoreKey)
	}
	if !parameterstore.IsNotFound(err) {
		t.Errorf("expected IsNotFound to be true for error: %v", err)
	}
}
