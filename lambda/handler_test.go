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
	"context"
	"strings"
	"testing"

	"github.com/tidal-music/cw-alert-router/v2/cw"
	"github.com/tidal-music/cw-alert-router/v2/lambda"
	"github.com/tidal-music/cw-alert-router/v2/pagerduty"
	"github.com/tidal-music/cw-alert-router/v2/parameterstore"
	"github.com/tidal-music/cw-alert-router/v2/s3"
	"github.com/tidal-music/cw-alert-router/v2/test"
)

// testFixture bundles a handler with the mocks behind it.
type testFixture struct {
	handler *lambda.Handler
	slack   *test.SlackServer
	pd      *test.MockPDClient
	s3      *test.MockS3API
	cw      *test.MockCWAPI
}

func newFixture(t *testing.T, cfg lambda.Config) *testFixture {
	t.Helper()

	f := &testFixture{
		slack: test.NewSlackServer(),
		pd:    &test.MockPDClient{},
		s3:    &test.MockS3API{},
		cw:    &test.MockCWAPI{},
	}
	t.Cleanup(f.slack.Close)

	pdclient, err := pagerduty.New(pagerduty.WithAPI(f.pd))
	if err != nil {
		t.Fatalf("failed creating pagerduty client: %v", err)
	}
	s3client, err := s3.New(context.Background(), s3.WithAPI(f.s3), s3.WithPresigner(&test.MockS3Presigner{}))
	if err != nil {
		t.Fatalf("failed creating s3 client: %v", err)
	}

	f.handler, err = lambda.New(context.Background(), cfg,
		lambda.WithCWClient(cw.NewClientWithAPI(f.cw)),
		lambda.WithParameterStoreClient(parameterstore.NewWithAPI(&test.MockSSMClient{})),
		lambda.WithPagerDutyClient(pdclient),
		lambda.WithS3Client(s3client),
		lambda.WithSlackToken("test-token"),
		lambda.WithSlackAPIURL(f.slack.APIURL()),
	)
	if err != nil {
		t.Fatalf("failed creating handler: %v", err)
	}
	return f
}

func baseConfig() lambda.Config {
	return lambda.Config{
		DefaultSlackChannel:        "test-alarms",
		DefaultPagerDutyRoutingKey: "default-pd-key",
		GraphMode:                  lambda.GraphModeNone,
	}
}

func TestSlackChannel(t *testing.T) {
	f := newFixture(t, baseConfig())
	tests := []struct {
		name     string
		tags     map[string]string
		expected string
	}{
		{
			name:     "derived from owner tag",
			tags:     map[string]string{"owner": "PlatEng", "service": "test-service"},
			expected: "plateng-alarms",
		},
		{
			name:     "override tag wins",
			tags:     map[string]string{"owner": "plateng", "alerts:slack_channel": "special-channel"},
			expected: "special-channel",
		},
		{
			name:     "default when no owner",
			tags:     map[string]string{"service": "test-service"},
			expected: "test-alarms",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := f.handler.SlackChannel(tc.tags); got != tc.expected {
				t.Errorf("channel (%s) didn't match expected (%s)", got, tc.expected)
			}
		})
	}
}

func TestPagerDutyRoutingKey(t *testing.T) {
	f := newFixture(t, baseConfig())
	tests := []struct {
		name     string
		service  string
		expected string
	}{
		{"registered service", "test-service", "pagerduty-key-1"},
		{"unknown service falls back to default", "unknown-service", "default-pd-key"},
		{"empty service uses default", "", "default-pd-key"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := f.handler.PagerDutyRoutingKey(context.Background(), tc.service)
			if err != nil {
				t.Fatalf("PagerDutyRoutingKey returned error: %v", err)
			}
			if got != tc.expected {
				t.Errorf("routing key (%s) didn't match expected (%s)", got, tc.expected)
			}
		})
	}
}

func TestConfigValidation(t *testing.T) {
	// missing default channel
	if _, err := lambda.New(context.Background(), lambda.Config{DefaultPagerDutyRoutingKey: "x"}); err == nil {
		t.Errorf("expected error for missing default slack channel")
	}
	// missing routing key
	if _, err := lambda.New(context.Background(), lambda.Config{DefaultSlackChannel: "x"}); err == nil {
		t.Errorf("expected error for missing default pagerduty routing key")
	}
	// s3 mode requires a bucket
	cfg := baseConfig()
	cfg.GraphMode = lambda.GraphModeS3
	if _, err := lambda.New(context.Background(), cfg); err == nil {
		t.Errorf("expected error for s3 graph mode without a bucket")
	}
	// invalid mode
	cfg = baseConfig()
	cfg.GraphMode = "whatever"
	if _, err := lambda.New(context.Background(), cfg); err == nil {
		t.Errorf("expected error for invalid graph mode")
	}
}

func TestGraphModeDefaults(t *testing.T) {
	cfg := baseConfig()
	cfg.GraphMode = ""
	f := newFixture(t, cfg)
	if got := f.handler.Config().GraphMode; got != lambda.GraphModeSlack {
		t.Errorf("expected default graph mode %s, got %s", lambda.GraphModeSlack, got)
	}

	cfg.ImageBucket = "some-bucket"
	f = newFixture(t, cfg)
	if got := f.handler.Config().GraphMode; got != lambda.GraphModeS3 {
		t.Errorf("expected graph mode %s when an image bucket is configured, got %s", lambda.GraphModeS3, got)
	}
}

func TestProcessEventTriggered(t *testing.T) {
	f := newFixture(t, baseConfig())

	evt := test.TriggeredAlarmDetails
	if err := f.handler.ProcessEvent(context.Background(), &evt); err != nil {
		t.Fatalf("ProcessEvent returned error: %v", err)
	}

	if len(f.slack.Messages()) != 1 {
		t.Errorf("expected 1 slack message, got %d", len(f.slack.Messages()))
	}
	events := f.pd.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 pagerduty event, got %d", len(events))
	}
	if events[0].Action != pagerduty.ActionTrigger {
		t.Errorf("expected trigger action, got %s", events[0].Action)
	}
	// tags register service test-service -> pagerduty-key-1
	if events[0].RoutingKey != "pagerduty-key-1" {
		t.Errorf("expected routing key pagerduty-key-1, got %s", events[0].RoutingKey)
	}
}

func TestProcessEventSuppressedStillGoesToSlack(t *testing.T) {
	f := newFixture(t, baseConfig())

	evt := test.SuppressedAlarmDetails
	if err := f.handler.ProcessEvent(context.Background(), &evt); err != nil {
		t.Fatalf("ProcessEvent returned error: %v", err)
	}

	if len(f.slack.Messages()) != 1 {
		t.Errorf("suppressed alarm should still reach slack - got %d messages", len(f.slack.Messages()))
	}
	if len(f.pd.Events()) != 0 {
		t.Errorf("suppressed alarm should not reach pagerduty - got %d events", len(f.pd.Events()))
	}
}

func TestProcessEventIgnoredTransition(t *testing.T) {
	f := newFixture(t, baseConfig())

	// INSUFFICIENT_DATA -> OK is not routed anywhere
	evt := test.ExpectedAlarmDetails
	if err := f.handler.ProcessEvent(context.Background(), &evt); err != nil {
		t.Fatalf("ProcessEvent returned error: %v", err)
	}
	if len(f.slack.Messages()) != 0 || len(f.pd.Events()) != 0 {
		t.Errorf("expected no notifications for an ignored transition (slack=%d pd=%d)",
			len(f.slack.Messages()), len(f.pd.Events()))
	}
}

func TestProcessEventGraphModeSlack(t *testing.T) {
	cfg := baseConfig()
	cfg.GraphMode = lambda.GraphModeSlack
	f := newFixture(t, cfg)

	evt := test.TriggeredAlarmDetails
	if err := f.handler.ProcessEvent(context.Background(), &evt); err != nil {
		t.Fatalf("ProcessEvent returned error: %v", err)
	}

	uploads := f.slack.Uploads()
	if len(uploads) != 1 {
		t.Fatalf("expected 1 uploaded graph, got %d", len(uploads))
	}
	messages := f.slack.Messages()
	if len(messages) != 1 {
		t.Fatalf("expected 1 slack message, got %d", len(messages))
	}
	if !strings.Contains(string(messages[0]), "slack_file") {
		t.Errorf("message should reference the uploaded graph via slack_file: %s", messages[0])
	}
}

func TestProcessEventGraphModeS3(t *testing.T) {
	cfg := baseConfig()
	cfg.GraphMode = lambda.GraphModeS3
	cfg.ImageBucket = "test-bucket-123"
	cfg.ImageBucketPrefix = "/graphs/"
	f := newFixture(t, cfg)

	evt := test.TriggeredAlarmDetails
	if err := f.handler.ProcessEvent(context.Background(), &evt); err != nil {
		t.Fatalf("ProcessEvent returned error: %v", err)
	}

	objects := f.s3.Objects()
	if len(objects) != 1 {
		t.Fatalf("expected 1 s3 object, got %v", objects)
	}
	if !strings.HasPrefix(objects[0], "test-bucket-123/graphs/2020/07/31/") {
		t.Errorf("unexpected object key layout: %s", objects[0])
	}
	if strings.Contains(objects[0], "//") {
		t.Errorf("object key contains a double slash: %s", objects[0])
	}
	// no image host configured -> presigned URL in the message
	if !strings.Contains(string(f.slack.Messages()[0]), "X-Amz-Signature") {
		t.Errorf("message should contain a presigned url: %s", f.slack.Messages()[0])
	}
}

func TestHandleRequestBatchFailures(t *testing.T) {
	f := newFixture(t, baseConfig())

	sqsEvent := test.GenTestSQSEvent()
	sqsEvent.Records[0].Body = "this is not json{"
	resp, err := f.handler.HandleRequest(context.Background(), sqsEvent)
	if err != nil {
		t.Fatalf("HandleRequest returned error: %v", err)
	}
	if len(resp.BatchItemFailures) != 1 {
		t.Fatalf("expected 1 batch item failure, got %d", len(resp.BatchItemFailures))
	}
	if resp.BatchItemFailures[0].ItemIdentifier != sqsEvent.Records[0].MessageId {
		t.Errorf("unexpected failed item identifier: %s", resp.BatchItemFailures[0].ItemIdentifier)
	}
}
