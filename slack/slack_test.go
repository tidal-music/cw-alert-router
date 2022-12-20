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

package slack_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	log "github.com/sirupsen/logrus"

	"github.com/tidal-open-source/cw-alert-router/slack"
	"github.com/tidal-open-source/cw-alert-router/test"
)

var once sync.Once
var serverAddr string
var sc *slack.Client
var server *httptest.Server

func startServer() {
	server = httptest.NewServer(nil)
	serverAddr = server.Listener.Addr().String()
	log.Printf("Started test server at %s", serverAddr)
}

func setup(t *testing.T) {
	var err error
	log.SetLevel(log.DebugLevel)
	once.Do(startServer)
	sc, err = slack.New("blah", slack.WithAlternativeURL("http://"+serverAddr+"/"), slack.OptionDebug(true))
	if err != nil {
		t.Fatalf("failed initializing slack client: %v", err)
	}
}

func TestCWAlarmHeader(t *testing.T) {
	expectedJSON := `{"type":"section","text":{"type":"mrkdwn","text":"*:white_check_mark: testing Cloudwatch Alarm: test-service-alarm-abcd*"}}`
	slackBlock := sc.CWAlarmHeaderBlock(&test.ExpectedAlarmDetails, ":white_check_mark: testing")
	// TODO: more validation on the slack stuff we're building
	t.Logf("slackBlock: %+v", slackBlock)
	inf, err := json.Marshal(slackBlock)
	if err != nil {
		t.Errorf("Couldn't marshal the header block")
	}
	if string(inf) != expectedJSON {
		t.Errorf("JSON didn't match - we wanted: %s -- but we got: %s", expectedJSON, string(inf))
	}
}

func TestSendEventResolved(t *testing.T) {
	http.DefaultServeMux = new(http.ServeMux)
	http.HandleFunc("/chat.postMessage", test.SendSlackMessage)
	setup(t)
	// a Go channel - not a slack channel ;)
	// - we want to inspect the actual request that gets delivered to Slack - to ensure it contains "blocks"
	slackChan := test.GetSlackMessageChannel()
	cid, ts, err := sc.SendEventResolved("test-channel", &test.ExpectedAlarmDetails, "https://test-link.com/test123")

	if err != nil {
		t.Fatalf("failed sending resolved event: %v", err)
	}

	body := <-slackChan

	actualBody, err := url.ParseQuery(string(body))

	if err != nil {
		t.Logf("Body: %s", string(body))
		t.Errorf("Couldn't parse body: %v", err)
	}

	// don't worry about the structure... slack seems to change quite frequently and we use their functions to build it.
	// we just want some certainty that we've sent a seemingly correct requests off to them ;)
	if _, ok := actualBody["blocks"]; !ok {
		t.Errorf("blocks parameter wasn't found in the body: %s", actualBody)
	}

	t.Logf("SendEventResolved: cid: %s, ts: %s, err: %v", cid, ts, err)
}

func TestSendEventTriggered(t *testing.T) {
	http.DefaultServeMux = new(http.ServeMux)
	http.HandleFunc("/chat.postMessage", test.SendSlackMessage)
	setup(t)
	// a Go channel - not a slack channel ;)
	// - we want to inspect the actual request that gets delivered to Slack - to ensure it contains "blocks"
	slackChan := test.GetSlackMessageChannel()
	cid, ts, err := sc.SendEventResolved("test-channel", &test.TriggeredAlarmDetails, "https://test-image-link.com/test123.png")

	if err != nil {
		t.Fatalf("failed sending resolved event: %v", err)
	}

	body := <-slackChan

	actualBody, err := url.ParseQuery(string(body))

	if err != nil {
		t.Logf("Body: %s", string(body))
		t.Errorf("Couldn't parse body: %v", err)
	}

	// don't worry about the structure... slack seems to change quite frequently and we use their functions to build it.
	// we just want some certainty that we've sent a seemingly correct requests off to them ;)
	if _, ok := actualBody["blocks"]; !ok {
		t.Errorf("blocks parameter wasn't found in the body: %s", actualBody)
	}

	t.Logf("blocks content: %s", actualBody["blocks"])

	t.Logf("SendEventResolved: cid: %s, ts: %s, err: %v", cid, ts, err)
}

func TestSendSimpleTextMessage(t *testing.T) {
	http.DefaultServeMux = new(http.ServeMux)
	http.HandleFunc("/chat.postMessage", test.SendSlackMessage)
	setup(t)
	cid, ts, err := sc.SendSimpleTextMessage("test-channel", "Hello There!")
	if err != nil {
		t.Errorf("Error sending message: %v", err)
	}
	t.Logf("Sent message to channel %s (ts: %s)", cid, ts)
}

func TestCWAlarmSummary(t *testing.T) {
	expectedJSON := "{\"type\":\"section\",\"text\":{\"type\":\"mrkdwn\",\"text\":\"*Metrics*: `Names: CPUUtilization - Namespaces: AWS/EC2 - Dimensions: AutoScalingGroupName:test-service`\\nRecent data: `error fetching metrics: cloudwatch client is nil`\"}}"
	setup(t)
	summary := sc.CWAlarmSummary(&test.TriggeredAlarmDetails)
	if summary == nil {
		t.Errorf("slack summary for cw alarm was nil")
	}
	inf, err := json.Marshal(summary)
	if err != nil {
		t.Errorf("Couldn't marshal the summary blocks: %v", err)
	}
	if string(inf) != expectedJSON {
		t.Errorf("JSON result didn't match expected.  We wanted: %s -- but we got: %s", expectedJSON, string(inf))
	}
}

func TestCWAlarmLink(t *testing.T) {
	expectedJSON := "{\"type\":\"section\",\"text\":{\"type\":\"mrkdwn\",\"text\":\"Link: \\u003chttps://console.aws.amazon.com/cloudwatch/home?region=us-east-1#alarmsV2:alarm/test-service-alarm-abcd|AWS Console\\u003e\"}}"
	setup(t)
	linkBlock := sc.CWAlarmLink(&test.TriggeredAlarmDetails)
	if linkBlock == nil {
		t.Errorf("Slack alarm link was nil")
	}
	inf, err := json.Marshal(linkBlock)
	if err != nil {
		t.Errorf("Couldn't marshal the link block: %v", err)
	}
	if string(inf) != expectedJSON {
		t.Errorf("JSON result didn't match expected.  We wanted: %s -- but we got: %s", expectedJSON, string(inf))
	}
}
