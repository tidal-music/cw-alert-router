// Copyright 2022 Aspiro AB Music AS
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
	"testing"

	"github.com/tidal-open-source/cw-alert-router/parameterstore"
	"github.com/tidal-open-source/cw-alert-router/test"
)

var (
	psclient                    *parameterstore.Client
	validParameterStoreKey      = "/service/cw_alert_router/pagerduty/routing_keys/shared_key"
	expectedParameterStoreValue = "shared-key-test-string"
	invalidParameterStoreKey    = "/service/nonexists/something"
)

func setup(t *testing.T) {
	var err error
	mc := &test.MockSSMClient{}
	psclient, err = parameterstore.NewWithSSMClient(mc)
	if err != nil {
		t.Fatalf("Failed initializing mock client: %v", err)
	}
}

func TestGetValidResponse(t *testing.T) {
	setup(t)
	value, err := psclient.GetParameterValue(validParameterStoreKey)
	if err != nil {
		t.Errorf("Error fetching key %s: %v", validParameterStoreKey, err)
	}
	if value != expectedParameterStoreValue {
		t.Errorf("parameter store value for key (%s) - %s - didn't match expected (%s)", validParameterStoreKey, value, expectedParameterStoreValue)
	}
}

func TestGetInvalidKey(t *testing.T) {
	setup(t)
	_, err := psclient.GetParameterValue(invalidParameterStoreKey)
	if err == nil {
		t.Errorf("Expected error when fetching key %s, but no error was returned.", invalidParameterStoreKey)
	}
}
