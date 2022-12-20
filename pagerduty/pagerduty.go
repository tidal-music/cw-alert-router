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
	pdapi "github.com/PagerDuty/go-pagerduty"
	log "github.com/sirupsen/logrus"
	"github.com/tidal-open-source/cw-alert-router/cw"
)

const (
	defaultEventSeverity = "critical"
	clientName           = "cw-notify"
)

// PDAPIClientInterface is an interface for the purpose of overriding the pagerduty client behaviour
type PDAPIClientInterface interface {
	ManageEvent(pdapi.V2Event) (*pdapi.V2EventResponse, error)
}

type defaultPDAPIClient struct {
}

func (c *defaultPDAPIClient) ManageEvent(e pdapi.V2Event) (*pdapi.V2EventResponse, error) {
	return pdapi.ManageEvent(e)
}

// Client is a wrapper for the pagerduty client
type Client struct {
	pd PDAPIClientInterface
}

// ClientOptions provides the method to configure the new client
type ClientOptions func(*Client)

// WithPDAPIClient allows overriding the pdapi client
func WithPDAPIClient(pd PDAPIClientInterface) ClientOptions {
	return func(c *Client) {
		c.pd = pd
	}
}

// New returns a new Client
func New(opts ...ClientOptions) (*Client, error) {
	c := &Client{}

	for _, opt := range opts {
		opt(c)
	}

	if c.pd == nil {
		c.pd = &defaultPDAPIClient{}
	}

	return c, nil
}

// EventAction returns the pagerduty action we'll take based on the Cloudwatch alarm
func (c *Client) EventAction(detail *cw.EventDetails) string {
	previousState := detail.Detail.PreviousState.Value
	currentState := detail.Detail.State.Value
	suppressPagerduty := detail.Detail.Tags["alerts:suppress_pagerduty"]

	// Do not send event if alerts:suppress_pagerduty tag is set to "true"
	if suppressPagerduty == "true" {
		return ""
	}

	switch currentState {
	case "OK":
		if previousState == "ALARM" {
			return "resolve"
		}
		return ""
	case "ALARM":
		return "trigger"
	default:
		return ""
	}
}

// SubmitEvent sends an event to PagerDuty using alarm details from cloudwatch
// - note: some logic here - it will change the PagerDuty action based on the current cw alarm status
func (c *Client) SubmitEvent(routingKey string, detail *cw.EventDetails) error {
	log.Printf("SubmitEvent(routingKey(%s), %+v)", maskKey(routingKey), detail)

	alarmARN, err := detail.AlarmARN()
	if err != nil {
		return err
	}

	severity := defaultEventSeverity
	timestamp := detail.Detail.State.Timestamp
	suppressalert := detail.Detail.Tags["alerts:suppress_pagerduty"]

	payload := &pdapi.V2Payload{
		Summary:   detail.Detail.AlarmName,
		Source:    alarmARN,
		Severity:  severity,
		Timestamp: timestamp,
		Details:   detail,
	}

	action := c.EventAction(detail)
	log.Infof("Pagerduty action: %s", action)

	if action == "" {
		log.Warnf("PagerDuty action was empty.  Not sending anything.")
		return nil
	}

	if suppressalert == "true" {
		log.Warnf("'suppress_pagerduty' tag is set to 'true'. Not sending event to PagerDuty.")
		return nil
	}

	ev := pdapi.V2Event{
		RoutingKey: routingKey,
		Action:     action,
		DedupKey:   alarmARN,
		Client:     clientName,
		Payload:    payload,
	}
	// So - PagerDuty have updated their go library to use the Client struct for many api calls...
	// ie: ManageEvent is just a function exported from the pagerduty package.  It does _not_ use
	// the initalized client - hence, we cannot override its settings (eg: for testing).
	// So we cannot test this very well unfortunately...
	// TODO: actually, this was merged: https://github.com/PagerDuty/go-pagerduty/pull/241
	status, err := c.pd.ManageEvent(ev)
	log.Infof("pagerduty response: %+v", status)
	return err
}

func maskKey(s string) string {
	rs := []rune(s)
	for i := 0; i < len(rs)-4; i++ {
		rs[i] = 'X'
	}
	return string(rs)
}
