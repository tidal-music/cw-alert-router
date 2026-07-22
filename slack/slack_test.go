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
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/tidal-music/cw-alert-router/v2/slack"
	"github.com/tidal-music/cw-alert-router/v2/test"
)

func newTestClient(t *testing.T, server *test.SlackServer) *slack.Client {
	t.Helper()
	sc, err := slack.New("blah", slack.WithAlternativeURL(server.APIURL()))
	if err != nil {
		t.Fatalf("failed initializing slack client: %v", err)
	}
	return sc
}

func postedBlocks(t *testing.T, body []byte) string {
	t.Helper()
	values, err := url.ParseQuery(string(body))
	if err != nil {
		t.Fatalf("couldn't parse posted body: %v (%s)", err, body)
	}
	blocks := values.Get("blocks")
	if blocks == "" {
		t.Fatalf("blocks parameter wasn't found in the body: %s", values)
	}
	return blocks
}

func TestHeaderBlock(t *testing.T) {
	server := test.NewSlackServer()
	defer server.Close()
	sc := newTestClient(t, server)

	expectedJSON := `{"type":"section","text":{"type":"mrkdwn","text":"*:white_check_mark: testing CloudWatch Alarm: test-service-alarm-abcd*"}}`
	block := sc.HeaderBlock(&test.ExpectedAlarmDetails, ":white_check_mark: testing")
	got, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("Couldn't marshal the header block: %v", err)
	}
	if string(got) != expectedJSON {
		t.Errorf("JSON didn't match - we wanted: %s -- but we got: %s", expectedJSON, got)
	}
}

func TestSummaryBlock(t *testing.T) {
	server := test.NewSlackServer()
	defer server.Close()
	sc := newTestClient(t, server)

	block := sc.SummaryBlock(&test.TriggeredAlarmDetails)
	got, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("Couldn't marshal the summary block: %v", err)
	}
	for _, want := range []string{"CPUUtilization", "AWS/EC2", "AutoScalingGroupName:test-service", "Threshold Crossed"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("summary block missing %q: %s", want, got)
		}
	}
}

func TestLinkBlock(t *testing.T) {
	server := test.NewSlackServer()
	defer server.Close()
	sc := newTestClient(t, server)

	expectedJSON := "{\"type\":\"section\",\"text\":{\"type\":\"mrkdwn\",\"text\":\"Link: \\u003chttps://console.aws.amazon.com/cloudwatch/home?region=us-east-1#alarmsV2:alarm/test-service-alarm-abcd|AWS Console\\u003e\"}}"
	block := sc.LinkBlock(&test.TriggeredAlarmDetails)
	got, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("Couldn't marshal the link block: %v", err)
	}
	if string(got) != expectedJSON {
		t.Errorf("JSON result didn't match expected.  We wanted: %s -- but we got: %s", expectedJSON, got)
	}
}

func TestSendEventResolvedWithImageURL(t *testing.T) {
	server := test.NewSlackServer()
	defer server.Close()
	sc := newTestClient(t, server)

	_, _, err := sc.SendEventResolved(context.Background(), "test-channel",
		&test.ExpectedAlarmDetails, slack.ImageRef{URL: "https://test-link.com/test123.png"})
	if err != nil {
		t.Fatalf("failed sending resolved event: %v", err)
	}

	messages := server.Messages()
	if len(messages) != 1 {
		t.Fatalf("expected 1 posted message, got %d", len(messages))
	}
	blocks := postedBlocks(t, messages[0])
	if !strings.Contains(blocks, "https://test-link.com/test123.png") {
		t.Errorf("posted blocks missing image url: %s", blocks)
	}
	if !strings.Contains(blocks, "resolved") {
		t.Errorf("posted blocks missing resolved prefix: %s", blocks)
	}
}

func TestSendEventTriggeredWithSlackFile(t *testing.T) {
	server := test.NewSlackServer()
	defer server.Close()
	sc := newTestClient(t, server)

	_, _, err := sc.SendEventTriggered(context.Background(), "test-channel",
		&test.TriggeredAlarmDetails, slack.ImageRef{SlackFileID: "F0000001"})
	if err != nil {
		t.Fatalf("failed sending triggered event: %v", err)
	}

	messages := server.Messages()
	if len(messages) != 1 {
		t.Fatalf("expected 1 posted message, got %d", len(messages))
	}
	blocks := postedBlocks(t, messages[0])
	if !strings.Contains(blocks, "slack_file") || !strings.Contains(blocks, "F0000001") {
		t.Errorf("posted blocks missing slack_file reference: %s", blocks)
	}
	if !strings.Contains(blocks, "triggered") {
		t.Errorf("posted blocks missing triggered prefix: %s", blocks)
	}
}

func TestSendEventWithoutImage(t *testing.T) {
	server := test.NewSlackServer()
	defer server.Close()
	sc := newTestClient(t, server)

	_, _, err := sc.SendEventTriggered(context.Background(), "test-channel",
		&test.TriggeredAlarmDetails, slack.ImageRef{})
	if err != nil {
		t.Fatalf("failed sending triggered event: %v", err)
	}

	blocks := postedBlocks(t, server.Messages()[0])
	if strings.Contains(blocks, "\"image\"") {
		t.Errorf("posted blocks should not contain an image block: %s", blocks)
	}
}

func TestUploadImage(t *testing.T) {
	server := test.NewSlackServer()
	defer server.Close()
	sc := newTestClient(t, server)

	fileID, err := sc.UploadImage(context.Background(), "graph.png", test.TestPNG)
	if err != nil {
		t.Fatalf("failed uploading image: %v", err)
	}
	if fileID == "" {
		t.Fatalf("expected a file ID")
	}

	uploaded, ok := server.Uploads()[fileID]
	if !ok {
		t.Fatalf("no upload recorded for file %s (uploads: %v)", fileID, server.Uploads())
	}
	if !bytes.Equal(uploaded, test.TestPNG) {
		t.Errorf("uploaded content didn't match the source PNG")
	}
}

func TestSendSimpleTextMessage(t *testing.T) {
	server := test.NewSlackServer()
	defer server.Close()
	sc := newTestClient(t, server)

	cid, ts, err := sc.SendSimpleTextMessage(context.Background(), "test-channel", "Hello There!")
	if err != nil {
		t.Errorf("Error sending message: %v", err)
	}
	t.Logf("Sent message to channel %s (ts: %s)", cid, ts)
}
