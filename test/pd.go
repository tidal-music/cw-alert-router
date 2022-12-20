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

package test

// Package test provides utilities and setup data for all tests

import (
	pdapi "github.com/PagerDuty/go-pagerduty"
)

// MockPDClient is a mock PagerDuty (our own) client
type MockPDClient struct{}

// ManageEvent - implement the same event from the pagerduty library
func (p *MockPDClient) ManageEvent(e pdapi.V2Event) (*pdapi.V2EventResponse, error) {
	resp := &pdapi.V2EventResponse{
		Status:   "success",
		Message:  "Event processed.",
		DedupKey: e.DedupKey,
	}
	return resp, nil
}
