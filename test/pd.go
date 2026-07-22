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

import (
	"context"
	"sync"

	pdapi "github.com/PagerDuty/go-pagerduty"
)

// MockPDClient is a mock PagerDuty API client that records submitted events.
type MockPDClient struct {
	mu     sync.Mutex
	events []*pdapi.V2Event
}

// ManageEventWithContext implements the pagerduty events API call.
func (p *MockPDClient) ManageEventWithContext(ctx context.Context, e *pdapi.V2Event) (*pdapi.V2EventResponse, error) {
	p.mu.Lock()
	p.events = append(p.events, e)
	p.mu.Unlock()
	return &pdapi.V2EventResponse{
		Status:   "success",
		Message:  "Event processed.",
		DedupKey: e.DedupKey,
	}, nil
}

// Events returns the events submitted so far.
func (p *MockPDClient) Events() []*pdapi.V2Event {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]*pdapi.V2Event(nil), p.events...)
}
