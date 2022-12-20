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
	"encoding/json"
	"io/ioutil"
	"net/http"

	slackapi "github.com/slack-go/slack"

	log "github.com/sirupsen/logrus"
)

var slackMessageChannel chan []byte

// GetSlackMessageChannel allows hooking into the SendSlackMessage callback (to inspect messages)
func GetSlackMessageChannel() <-chan []byte {
	slackMessageChannel = make(chan []byte, 1)
	return slackMessageChannel
}

// SendSlackMessage is for sending a test slack message (should have started a test server that listens to /chat.PostMessage - see slack_test.go)
func SendSlackMessage(rw http.ResponseWriter, r *http.Request) {

	rw.Header().Set("Content-Type", "application/json")

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Errorf("error reading request body: %v", err)
	} else {
		if slackMessageChannel != nil {
			log.Printf("sending request body to (go) channel....")
			slackMessageChannel <- body
		}
	}

	response, _ := json.Marshal(struct {
		slackapi.SlackResponse
		Channel            string `json:"channel"`
		Timestamp          string `json:"ts"`
		MessageTimeStamp   string `json:"message_ts"`
		ScheduledMessageID string `json:"scheduled_message_id,omitempty"`
		Text               string `json:"text"`
	}{
		SlackResponse:      slackapi.SlackResponse{Ok: true},
		Channel:            "XVB123123123",
		Timestamp:          "123123123123123",
		MessageTimeStamp:   "123123123123123",
		ScheduledMessageID: "ASDGADASDGSDG",
		Text:               "This is a message?",
	})
	rw.Write(response)
}
