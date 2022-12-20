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

package lambda

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"

	awsevents "github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/tidal-open-source/cw-alert-router/cw"
)

var cfg *Config

// SetConfig allows injecting Config externally (ie: for testing)
func SetConfig(c *Config) {
	cfg = c
}

// GetOwnerFromTags returns the owning team name from the given tags
func GetOwnerFromTags(tags cw.AlarmTags) string {
	if val, ok := tags[cfg.OwnerTagKey]; ok {
		return val
	}
	return ""
}

// GetSlackChannelOverrideFromTags returns the slack channel override from the given tags
func GetSlackChannelOverrideFromTags(tags cw.AlarmTags) string {
	if val, ok := tags[DefaultSlackChannelOverrideTagKey]; ok {
		return val
	}
	return ""
}

// GetServiceNameFromTags returns the service name from the given tags
func GetServiceNameFromTags(tags cw.AlarmTags) string {
	if val, ok := tags[cfg.ServiceNameTagKey]; ok {
		return val
	}
	return ""
}

// GetSlackChannelFromOwner returns the slack channel name given the owner name
// note:
//  1. if owner is empty, the default slack channel (stored in config) is used
//  - owner name is converted to lowercase
//  2. else, use "${owner}-alarms"
func GetSlackChannelFromOwner(owner string) string {
	fmt.Printf("GetSlackChannelFromOwner(%s)\n", owner)
	if owner == "" {
		return cfg.DefaultSlackChannel
	}

	lowerOwner := strings.ToLower(owner)

	return fmt.Sprintf("%s-alarms", lowerOwner)
}

// GetPagerDutyRoutingKey returns the PagerDuty routing key given the service name.
// 1. if ServiceName is empty, then the default pagerduty routing key is returned
// - ServiceName is lowercased, and hyphens replaced with underscores
// 2. if /service/alert-router/pagerduty/routing_keys/${service_name} exists - use that value
// 3. else return the default routing key
func GetPagerDutyRoutingKey(ServiceName string) string {
	if ServiceName == "" {
		log.Printf("Using the default pagerduty routing key.")
		return cfg.DefaultPagerDutyRoutingKey
	}
	lServiceName := strings.ReplaceAll(strings.ToLower(ServiceName), "-", "_")
	pagerDutyRoutingKeySSMKey := fmt.Sprintf(DefaultPagerDutyRoutingKeySSMPattern, lServiceName)

	val, err := cfg.ParameterStoreClient().GetParameterValue(pagerDutyRoutingKeySSMKey)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case ssm.ErrCodeParameterNotFound:
				log.Warnf("parameter key %s doesn't exist.  Using default pagerduty routing key", pagerDutyRoutingKeySSMKey)
				return cfg.DefaultPagerDutyRoutingKey
			default:
				log.Fatalf("There was an AWS error retrieving ssm key %s: %v", pagerDutyRoutingKeySSMKey, aerr)
			}
		} else {

			log.Fatalf("There was a generic error retrieving ssm key %s: %v", pagerDutyRoutingKeySSMKey, err)
		}
	}
	if val == "" {
		log.Printf("SSM key for %s was empty - using the default pagerduty routing key.", lServiceName)
		return cfg.DefaultPagerDutyRoutingKey
	}
	log.Printf("Found a PagerDuty Routing Key SSM parameter for %s - using that", lServiceName)
	return val
}

// GetSlackChannel determines which slack channel we send messages to based on owner or the alerts:slack_channel override (fallback = default channel via env variable)
func GetSlackChannel(tags cw.AlarmTags) string {
	slackChannelOverride := GetSlackChannelOverrideFromTags(tags)
	var slackChannel string
	if slackChannelOverride != "" {
		slackChannel = slackChannelOverride
		log.Printf("Slack channel override: %s", slackChannel)
	} else {
		alertOwner := GetOwnerFromTags(tags)
		slackChannel = GetSlackChannelFromOwner(alertOwner)
		log.Printf("Owner (from tags): %s -> using slack channel: %s", alertOwner, slackChannel)
	}
	return slackChannel
}

// ProcessSQSEvent handles the cloudwatch event from SQS
func ProcessSQSEvent(d *cw.EventDetails) error {
	if d == nil {
		return fmt.Errorf("passed event details are nil")
	}
	d.SetCWClient(cfg.CWClient())
	err := d.InjectTags()
	if err != nil {
		return fmt.Errorf("error injecting tags for cloudwatch event: %v", err)
	}

	// get the service name from the tags
	ServiceName := GetServiceNameFromTags(d.Detail.Tags)
	log.Printf("service name (from tags): %s", ServiceName)
	pagerDutyRoutingKey := GetPagerDutyRoutingKey(ServiceName)
	if pagerDutyRoutingKey == "" {
		return fmt.Errorf("failed fetching the pager duty routing key")
	}

	// get the slack channel from owner/etc
	slackChannel := GetSlackChannel(d.Detail.Tags)

	// now send to slack/pagerduty
	pdClient := cfg.PagerDutyClient()
	pdAction := pdClient.EventAction(d)

	slackClient := cfg.SlackClient()

	var slackError error
	var channelID, ts string

	imgURL, err := GenerateMetricsGraphAndLink(d, cfg)
	if err != nil {
		imgURL = ""
		log.Errorf("Failed generating metrics graph: %v", err)
	}

	switch pdAction {
	case "resolve":
		log.Printf("Sending a message to slack (resolved)!")
		channelID, ts, slackError = slackClient.SendEventResolved(slackChannel, d, imgURL)
	case "trigger":
		log.Printf("Sending a message to slack (triggered)!")
		channelID, ts, slackError = slackClient.SendEventTriggered(slackChannel, d, imgURL)
	default:
		log.Printf("ignoring alarm (pagerduty action = [%s]", pdAction)
	}

	if slackError != nil {
		return slackError
	}
	log.Printf("Slack message details: channelID: %s, timestamp: %s", channelID, ts)

	return cfg.PagerDutyClient().SubmitEvent(pagerDutyRoutingKey, d)
}

// HandleRequest is the main entrypoint for the lambda
func HandleRequest(ctx context.Context, sqsEvent awsevents.SQSEvent) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("lambda Config not initialized")
	}

	if len(sqsEvent.Records) <= 0 {
		return "", fmt.Errorf("no events to process")
	}

	for _, msg := range sqsEvent.Records {
		log.Printf("Processing SQS message ID: %s (source: %s)", msg.MessageId, msg.EventSource)
		log.Printf("Body: %s", msg.Body)
		detail := &cw.EventDetails{}
		err := json.Unmarshal([]byte(msg.Body), detail)
		if err != nil {
			log.Errorf("Failed decoding message body - error: %s - body: %s", err, msg.Body)
		}
		err = ProcessSQSEvent(detail)
		if err != nil {
			log.Errorf("Failed processing event: %s", err)
			return "", err
		}
	}

	return "", nil
}

// Start begins the lambda handler
func Start() {
	lambda.Start(HandleRequest)
}
