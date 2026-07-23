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

// Package cw models the EventBridge "CloudWatch Alarm State Change" event and
// provides a small CloudWatch client for the calls this service needs.
package cw

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

// Alarm state values as sent by CloudWatch.
const (
	StateOK               = "OK"
	StateAlarm            = "ALARM"
	StateInsufficientData = "INSUFFICIENT_DATA"
)

// stateTimestampLayout is the timestamp format CloudWatch uses inside the
// alarm state payload (differs from the RFC3339 event envelope time).
const stateTimestampLayout = "2006-01-02T15:04:05.000-0700"

// Event is the EventBridge envelope for a CloudWatch alarm state change, as
// received via EventBridge -> SQS -> Lambda.
type Event struct {
	Version    string           `json:"version"`
	ID         string           `json:"id"`
	DetailType string           `json:"detail-type"`
	Source     string           `json:"source"`
	Account    string           `json:"account"`
	Time       string           `json:"time"`
	Region     string           `json:"region"`
	Resources  []string         `json:"resources"`
	Detail     AlarmStateChange `json:"detail"`
}

// AlarmStateChange is the event detail payload for a CloudWatch alarm state change.
type AlarmStateChange struct {
	AlarmName     string        `json:"alarmName"`
	State         State         `json:"state"`
	PreviousState State         `json:"previousState"`
	Configuration Configuration `json:"configuration"`
}

// State describes one alarm state (current or previous).
type State struct {
	Value      string `json:"value"`
	Reason     string `json:"reason"`
	ReasonData string `json:"reasonData"`
	Timestamp  string `json:"timestamp"`
}

// Configuration is the alarm configuration included in the event payload.
type Configuration struct {
	Description string            `json:"description"`
	Metrics     []MetricDataQuery `json:"metrics"`
}

// MetricDataQuery is one metric (or metric-math expression) of the alarm configuration.
type MetricDataQuery struct {
	ID         string      `json:"id"`
	Expression string      `json:"expression,omitempty"`
	Label      string      `json:"label,omitempty"`
	ReturnData bool        `json:"returnData"`
	MetricStat *MetricStat `json:"metricStat,omitempty"`
}

// MetricStat describes how a metric is aggregated.
type MetricStat struct {
	Metric Metric `json:"metric"`
	Period int64  `json:"period"`
	Stat   string `json:"stat"`
}

// Metric identifies a CloudWatch metric.
type Metric struct {
	Namespace  string            `json:"namespace"`
	Name       string            `json:"name"`
	Dimensions map[string]string `json:"dimensions"`
}

// MetricSummary is a flattened, display-friendly view of the alarm's metrics.
type MetricSummary struct {
	Names       []string
	Namespaces  []string
	Dimensions  []string
	Expressions []string
}

// AlarmARN returns the ARN of the alarm that emitted this event.
func (e *Event) AlarmARN() (string, error) {
	if len(e.Resources) != 1 {
		return "", fmt.Errorf("expected exactly 1 resource in the alarm event, got %d", len(e.Resources))
	}
	return e.Resources[0], nil
}

// ConsoleLink returns a URL to the alarm in the AWS console.
func (e *Event) ConsoleLink() string {
	return fmt.Sprintf("https://console.aws.amazon.com/cloudwatch/home?region=%s#alarmsV2:alarm/%s",
		e.Region, url.PathEscape(e.Detail.AlarmName))
}

// Threshold returns the alarm threshold parsed from the state's reason data,
// and whether one was present (composite and some anomaly-detection alarms
// don't carry one).
func (e *Event) Threshold() (float64, bool) {
	var rd struct {
		Threshold *float64 `json:"threshold"`
	}
	if err := json.Unmarshal([]byte(e.Detail.State.ReasonData), &rd); err != nil || rd.Threshold == nil {
		return 0, false
	}
	return *rd.Threshold, true
}

// StateChangeTime returns the time the alarm changed state, falling back to
// the event envelope time and finally time.Now if neither parses.
func (e *Event) StateChangeTime() time.Time {
	if t, err := time.Parse(stateTimestampLayout, e.Detail.State.Timestamp); err == nil {
		return t
	}
	if t, err := time.Parse(time.RFC3339, e.Time); err == nil {
		return t
	}
	return time.Now()
}

// MetricSummary returns the metric names, namespaces, dimensions and
// expressions of the alarm configuration for display purposes.
func (e *Event) MetricSummary() MetricSummary {
	var s MetricSummary
	for _, q := range e.Detail.Configuration.Metrics {
		if q.Expression != "" {
			s.Expressions = append(s.Expressions, q.Expression)
		}
		if q.MetricStat == nil {
			continue
		}
		m := q.MetricStat.Metric
		if m.Namespace != "" {
			s.Namespaces = append(s.Namespaces, m.Namespace)
		}
		if m.Name != "" {
			s.Names = append(s.Names, m.Name)
		}
		for k, v := range m.Dimensions {
			s.Dimensions = append(s.Dimensions, fmt.Sprintf("%s:%s", k, v))
		}
	}
	return s
}
