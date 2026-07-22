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

// Package pagerduty provides a simple interface to send alerts to a specific service
package pagerduty

import (
	"context"
	"fmt"
	"log/slog"

	pdapi "github.com/PagerDuty/go-pagerduty"
	"github.com/tidal-music/cw-alert-router/v2/cw"
)

const (
	defaultEventSeverity = "critical"
	clientName           = "cw-alert-router"
)

// Actions we submit to the PagerDuty events API.
const (
	ActionTrigger = "trigger"
	ActionResolve = "resolve"
	ActionNone    = ""
)

// API is the subset of the PagerDuty client this service uses.
type API interface {
	ManageEventWithContext(ctx context.Context, e *pdapi.V2Event) (*pdapi.V2EventResponse, error)
}

// Client is a wrapper for the pagerduty client.
type Client struct {
	api API
}

// ClientOptions provides the method to configure the new client.
type ClientOptions func(*Client)

// WithAPI allows overriding the pagerduty API client (for testing).
func WithAPI(api API) ClientOptions {
	return func(c *Client) {
		c.api = api
	}
}

// New returns a new Client.
func New(opts ...ClientOptions) (*Client, error) {
	c := &Client{}
	for _, opt := range opts {
		opt(c)
	}
	if c.api == nil {
		// the events API authenticates via routing key, not account token
		c.api = pdapi.NewClient("")
	}
	return c, nil
}

// Action returns the PagerDuty event action for an alarm state transition:
// ActionTrigger, ActionResolve or ActionNone.
func Action(previousState, currentState string) string {
	switch currentState {
	case cw.StateAlarm:
		return ActionTrigger
	case cw.StateOK:
		if previousState == cw.StateAlarm {
			return ActionResolve
		}
		return ActionNone
	default:
		return ActionNone
	}
}

// SubmitEvent sends an event with the given action to PagerDuty using alarm
// details from the CloudWatch event.
func (c *Client) SubmitEvent(ctx context.Context, routingKey string, action string, evt *cw.Event) error {
	if action == ActionNone {
		return nil
	}

	alarmARN, err := evt.AlarmARN()
	if err != nil {
		return err
	}

	slog.Info("submitting pagerduty event",
		"routing_key", maskKey(routingKey), "action", action, "alarm", evt.Detail.AlarmName)

	resp, err := c.api.ManageEventWithContext(ctx, &pdapi.V2Event{
		RoutingKey: routingKey,
		Action:     action,
		DedupKey:   alarmARN,
		Client:     clientName,
		Payload: &pdapi.V2Payload{
			Summary:   evt.Detail.AlarmName,
			Source:    alarmARN,
			Severity:  defaultEventSeverity,
			Timestamp: evt.Detail.State.Timestamp,
			Details:   evt,
		},
	})
	if err != nil {
		return fmt.Errorf("submitting pagerduty event for %s: %w", evt.Detail.AlarmName, err)
	}
	slog.Debug("pagerduty response", "status", resp.Status, "message", resp.Message)
	return nil
}

func maskKey(s string) string {
	rs := []rune(s)
	for i := 0; i < len(rs)-4; i++ {
		rs[i] = 'X'
	}
	return string(rs)
}
