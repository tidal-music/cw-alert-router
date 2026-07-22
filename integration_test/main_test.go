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

// Package main_test drives the full lambda handler end-to-end against mocked
// AWS/Slack/PagerDuty backends, including the env-var configuration path.
package main_test

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/lambdacontext"

	"github.com/tidal-music/cw-alert-router/v2/cw"
	"github.com/tidal-music/cw-alert-router/v2/lambda"
	"github.com/tidal-music/cw-alert-router/v2/pagerduty"
	"github.com/tidal-music/cw-alert-router/v2/parameterstore"
	"github.com/tidal-music/cw-alert-router/v2/test"
)

const (
	defaultSlackChannel        = "test-alarms"
	defaultPagerDutyRoutingKey = "default-pd-key"
)

type fixture struct {
	handler *lambda.Handler
	slack   *test.SlackServer
	pd      *test.MockPDClient
}

func setup(t *testing.T) *fixture {
	t.Helper()

	// configuration comes in via the environment, like in a real deployment
	t.Setenv(lambda.DefaultSlackChannelEnv, defaultSlackChannel)
	t.Setenv(lambda.DefaultPagerDutyRoutingKeyEnv, defaultPagerDutyRoutingKey)
	t.Setenv(lambda.SlackTokenSSMKeyEnv, test.SlackTokenSSMKey)
	t.Setenv(lambda.GraphModeEnv, lambda.GraphModeSlack)
	t.Setenv(lambda.LogLevelEnv, "debug")

	f := &fixture{
		slack: test.NewSlackServer(),
		pd:    &test.MockPDClient{},
	}
	t.Cleanup(f.slack.Close)

	pdclient, err := pagerduty.New(pagerduty.WithAPI(f.pd))
	if err != nil {
		t.Fatalf("failed creating pagerduty client: %v", err)
	}

	// note: no WithSlackToken here - the token is fetched from the (mocked)
	// parameter store, exercising the real startup path
	f.handler, err = lambda.New(context.Background(), lambda.ConfigFromEnv(),
		lambda.WithCWClient(cw.NewClientWithAPI(&test.MockCWAPI{})),
		lambda.WithParameterStoreClient(parameterstore.NewWithAPI(&test.MockSSMClient{})),
		lambda.WithPagerDutyClient(pdclient),
		lambda.WithSlackAPIURL(f.slack.APIURL()),
	)
	if err != nil {
		t.Fatalf("failed creating handler: %v", err)
	}
	return f
}

func TestLambdaHandler(t *testing.T) {
	f := setup(t)

	ctx := lambdacontext.NewContext(context.Background(), new(lambdacontext.LambdaContext))
	resp, err := f.handler.HandleRequest(ctx, test.GenTestSQSEvent())
	if err != nil {
		t.Fatalf("HandleRequest returned an error: %v", err)
	}
	if len(resp.BatchItemFailures) != 0 {
		t.Fatalf("expected no batch item failures, got %+v", resp.BatchItemFailures)
	}

	// the test event is an ALARM -> OK transition: expect a resolved slack
	// message (with an uploaded graph) and a pagerduty resolve
	messages := f.slack.Messages()
	if len(messages) != 1 {
		t.Fatalf("expected 1 slack message, got %d", len(messages))
	}
	body := string(messages[0])
	if !strings.Contains(body, "blocks") {
		t.Errorf("slack message has no blocks: %s", body)
	}
	if !strings.Contains(body, "resolved") {
		t.Errorf("slack message should be a resolved notification: %s", body)
	}
	if !strings.Contains(body, "slack_file") {
		t.Errorf("slack message should embed the uploaded graph: %s", body)
	}
	if len(f.slack.Uploads()) != 1 {
		t.Errorf("expected 1 uploaded graph, got %d", len(f.slack.Uploads()))
	}

	events := f.pd.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 pagerduty event, got %d", len(events))
	}
	if events[0].Action != pagerduty.ActionResolve {
		t.Errorf("expected pagerduty resolve, got %s", events[0].Action)
	}
	if events[0].RoutingKey != "pagerduty-key-1" {
		t.Errorf("expected service routing key pagerduty-key-1, got %s", events[0].RoutingKey)
	}
}
