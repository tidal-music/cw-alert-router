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

package main_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/aws/aws-lambda-go/lambdacontext"

	"github.com/tidal-open-source/cw-alert-router/cw"
	"github.com/tidal-open-source/cw-alert-router/lambda"
	"github.com/tidal-open-source/cw-alert-router/pagerduty"
	"github.com/tidal-open-source/cw-alert-router/parameterstore"
	"github.com/tidal-open-source/cw-alert-router/s3"
	"github.com/tidal-open-source/cw-alert-router/test"
)

var cwclient *cw.Client
var cfg *lambda.Config
var once sync.Once
var slackServerAddr string

const (
	defaultSlackChannel        = "test-alarms"
	defaultPagerDutyRoutingKey = "pagerduty-key-1"
	defaultImageBucket         = "test-bucket-123"
	defaultImageBucketRegion   = "eu-west-1"
	defaultImageBucketRoleArn  = ""
	defaultImageHost           = "https://test.image.host.com"
	defaultLogLevel            = "DEBUG"
)

func startSlackServer() {
	slackServer := httptest.NewServer(nil)
	slackServerAddr = slackServer.Listener.Addr().String()
}

func setup(t *testing.T) {
	os.Setenv(lambda.SlackTokenSSMKeyEnv, test.SlackTokenSSMKey)
	os.Setenv(lambda.DefaultPagerDutyRoutingKeyEnv, defaultPagerDutyRoutingKey)
	os.Setenv(lambda.DefaultSlackChannelEnv, defaultSlackChannel)
	os.Setenv(lambda.ImageBucketEnv, defaultImageBucket)
	os.Setenv(lambda.ImageBucketRoleArnEnv, defaultImageBucketRoleArn)
	os.Setenv(lambda.LogLevelEnv, defaultLogLevel)
	os.Setenv(lambda.ImageHostEnv, defaultImageHost)
	os.Setenv(lambda.ImageBucketRegionEnv, defaultImageBucketRegion)
	once.Do(startSlackServer)

	cwclient = &cw.Client{
		CWClient: &test.MockCWClient{},
	}

	mc := &test.MockSSMClient{}
	psclient, err := parameterstore.NewWithSSMClient(mc)

	if err != nil {
		t.Fatalf("Failed creating new parameterstore client: %v", err)
	}

	pdclient, err := pagerduty.New(pagerduty.WithPDAPIClient(&test.MockPDClient{}))

	if err != nil {
		t.Fatalf("failed creating new pagerduty client: %v", err)
	}

	s3client, err := s3.New(s3.WithS3APIClient(&test.MockS3Client{}))

	if err != nil {
		t.Fatalf("failed initializing mock s3 client: %v", err)
	}

	cfg, err = lambda.NewConfig(
		lambda.WithParameterStoreClient(psclient),
		lambda.WithPagerDutyClient(pdclient),
		lambda.WithCWClient(cwclient),
		lambda.WithSlackAlternativeURL("http://"+slackServerAddr+"/"),
		lambda.WithS3Client(s3client),
	)

	if err != nil {
		t.Fatalf("Failed creating new lambda config: %v", err)
	}
}

func TestLambdaHandler(t *testing.T) {
	http.DefaultServeMux = new(http.ServeMux)
	http.HandleFunc("/chat.postMessage", test.SendSlackMessage)
	setup(t)
	lambda.SetConfig(cfg)
	ctx := context.Background()

	lc := new(lambdacontext.LambdaContext)
	ctx = lambdacontext.NewContext(ctx, lc)
	body, err := lambda.HandleRequest(ctx, test.GenTestSQSEvent())
	if err != nil {
		t.Errorf("HandleRequest returned an error: %v", err)
	}
	t.Logf("body: %s", body)
}

func TestGetFunctions(t *testing.T) {
	expectedSlackChannel := "test-alarms"
	expectedPagerDutyRoutingKey := "pagerduty-key-1"
	expectedServiceName := "test-service"
	http.DefaultServeMux = new(http.ServeMux)
	http.HandleFunc("/chat.postMessage", test.SendSlackMessage)
	setup(t)
	lambda.SetConfig(cfg)

	alarmDetail := test.TriggeredAlarmDetails
	alarmDetail.SetCWClient(cfg.CWClient())
	err := alarmDetail.InjectTags()

	if err != nil {
		t.Errorf("Failed injecting tags: %v", err)
	}

	if len(alarmDetail.Detail.Tags) <= 0 {
		t.Fatalf("No tags were injected!")
	}

	slackChan := lambda.GetSlackChannel(alarmDetail.Detail.Tags)
	if slackChan != expectedSlackChannel {
		t.Errorf("Didn't see the expected slack channel: %s (we got %s)", expectedSlackChannel, slackChan)
	}

	ServiceName := lambda.GetServiceNameFromTags(alarmDetail.Detail.Tags)

	if ServiceName != expectedServiceName {
		t.Errorf("App name from tags (%s) didn't match expected (%s)", ServiceName, expectedServiceName)
	}

	pdkey := lambda.GetPagerDutyRoutingKey(ServiceName)
	if pdkey != expectedPagerDutyRoutingKey {
		t.Errorf("Didn't see the expected PagerDuty routing key: %s (we got: %s)", expectedPagerDutyRoutingKey, pdkey)
	}
}

func TestGetOwner(t *testing.T) {
	tests := []struct {
		key           string
		tags          cw.AlarmTags
		expectedOwner string
	}{
		{
			key: "",
			tags: cw.AlarmTags{
				lambda.DefaultOwnerTagKey: "somebody",
			},
			expectedOwner: "somebody",
		},
		{
			key: "some_owner",
			tags: cw.AlarmTags{
				"some_owner": "other_person",
			},
			expectedOwner: "other_person",
		},
	}
	for _, test := range tests {
		t.Setenv(lambda.OwnerTagKeyEnv, test.key)
		setup(t)
		lambda.SetConfig(cfg)
		owner := lambda.GetOwnerFromTags(test.tags)
		if owner != test.expectedOwner {
			t.Errorf("owner (%s) didn't match expected (%s)", owner, test.expectedOwner)
		}
	}
}

func TestGetServiceName(t *testing.T) {
	tests := []struct {
		key             string
		tags            cw.AlarmTags
		expectedService string
	}{
		{
			key: "",
			tags: cw.AlarmTags{
				lambda.DefaultServiceNameTagKey: "service1",
			},
			expectedService: "service1",
		},
		{
			key: "some_service",
			tags: cw.AlarmTags{
				"some_service": "service6",
			},
			expectedService: "service6",
		},
	}
	for _, test := range tests {
		t.Setenv(lambda.ServiceNameTagKeyEnv, test.key)
		setup(t)
		lambda.SetConfig(cfg)
		owner := lambda.GetServiceNameFromTags(test.tags)
		if owner != test.expectedService {
			t.Errorf("service (%s) didn't match expected (%s)", owner, test.expectedService)
		}
	}
}
