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

// Package test provides utilities and setup data for all tests
package test

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"

	awsevents "github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	"github.com/tidal-music/cw-alert-router/v2/cw"
)

// TestPNG is a minimal valid PNG (1x1 transparent pixel) returned by the mock
// GetMetricWidgetImage.
var TestPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
	0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, 0x89, 0x00, 0x00, 0x00,
	0x0a, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00, 0x00, 0x00, 0x49,
	0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
}

var (
	// TestEventJSONFromSQS - a test event as seen from eventbridge -> SQS -> lambda
	TestEventJSONFromSQS = "{\"version\":\"0\",\"id\":\"19905c58-8563-0126-6a9d-c8530ac6c240\",\"detail-type\":\"CloudWatch Alarm State Change\",\"source\":\"aws.cloudwatch\",\"account\":\"1234567890123\",\"time\":\"2020-10-05T06:47:38Z\",\"region\":\"us-east-1\",\"resources\":[\"arn:aws:cloudwatch:us-east-1:1234567890123:alarm:test-service_aurora_serverless_cpu_utilization\"],\"detail\":{\"alarmName\":\"test-service_aurora_serverless_cpu_utilization\",\"state\":{\"value\":\"OK\",\"reason\":\"Threshold Crossed: 2 out of the last 2 datapoints [10.0 (05/10/20 06:42:00), 10.0 (05/10/20 06:37:00)] were not greater than or equal to the threshold (30.0) (minimum 1 datapoint for ALARM -> OK transition).\",\"reasonData\":\"{\\\"version\\\":\\\"1.0\\\",\\\"queryDate\\\":\\\"2020-10-05T06:47:38.031+0000\\\",\\\"startDate\\\":\\\"2020-10-05T06:37:00.000+0000\\\",\\\"statistic\\\":\\\"Average\\\",\\\"period\\\":300,\\\"recentDatapoints\\\":[10.0,10.0],\\\"threshold\\\":30.0}\",\"timestamp\":\"2020-10-05T06:47:38.033+0000\"},\"previousState\":{\"value\":\"ALARM\",\"reason\":\"Threshold Crossed: 2 out of the last 2 datapoints [10.25 (05/10/20 06:25:00), 10.0 (05/10/20 06:20:00)] were greater than or equal to the threshold (10.0) (minimum 2 datapoints for OK -> ALARM transition).\",\"reasonData\":\"{\\\"version\\\":\\\"1.0\\\",\\\"queryDate\\\":\\\"2020-10-05T06:30:58.947+0000\\\",\\\"startDate\\\":\\\"2020-10-05T06:20:00.000+0000\\\",\\\"statistic\\\":\\\"Average\\\",\\\"period\\\":300,\\\"recentDatapoints\\\":[10.0,10.25],\\\"threshold\\\":10.0}\",\"timestamp\":\"2020-10-05T06:30:58.967+0000\"},\"configuration\":{\"description\":\"High CPU Utilization\",\"metrics\":[{\"id\":\"62ba7bc1-7c4c-3747-4ab5-3a3dc4e40530\",\"metricStat\":{\"metric\":{\"namespace\":\"AWS/RDS\",\"name\":\"CPUUtilization\",\"dimensions\":{\"DBClusterIdentifier\":\"testdb\"}},\"period\":300,\"stat\":\"Average\"},\"returnData\":true}]}}}"

	// TestEventJSON provides a test cw alarm eventbridge event
	TestEventJSON = `{
		"version": "0",
		"time": "2020-07-31T06:56:05Z",
		"source": "aws.cloudwatch",
		"resources": [
			"arn:aws:cloudwatch:us-east-1:1234567890123:alarm:test-service-alarm-abcd"
		],
		"region": "us-east-1",
		"id": "1ebd31a7-11e5-b52a-03e4-be4110b0f1e0",
		"detail-type": "CloudWatch Alarm State Change",
		"detail": {
			"state": {
				"value": "OK",
				"timestamp": "2020-07-31T06:56:05.606+0000",
				"reasonData": "{\"version\":\"1.0\",\"queryDate\":\"2020-07-31T06:56:05.603+0000\",\"startDate\":\"2020-07-31T06:50:00.000+0000\",\"statistic\":\"Average\",\"period\":60,\"recentDatapoints\":[0.2872183414376726],\"threshold\":60.0}",
				"reason": "Threshold Crossed: 1 datapoint [0.2872183414376726 (31/07/20 06:50:00)] was not greater than the threshold (60.0)."
			},
			"previousState": {
				"value": "INSUFFICIENT_DATA",
				"timestamp": "2020-07-31T06:52:05.601+0000",
				"reasonData": "{\"version\":\"1.0\",\"queryDate\":\"2020-07-31T06:52:05.598+0000\",\"statistic\":\"Average\",\"period\":60,\"recentDatapoints\":[],\"threshold\":60.0}",
				"reason": "Insufficient Data: 1 datapoint was unknown."
			},
			"configuration": {
				"metrics": [
					{
						"returnData": true,
						"metricStat": {
							"stat": "Average",
							"period": 60,
							"metric": {
								"namespace": "AWS/EC2",
								"name": "CPUUtilization",
								"dimensions": {
									"AutoScalingGroupName": "test-service"
								}
							}
						},
						"id": "beb9cf5d-585d-3d43-179b-1e0898d8e237"
					}
				]
			},
			"alarmName": "test-service-alarm-abcd"
		},
		"account": "1234567890123"
	}`

	// testSQSEvent is for the integration testing of the lambda handler
	testSQSEvent = awsevents.SQSEvent{
		Records: []awsevents.SQSMessage{
			{
				MessageId:      "blahblahblah1234567",
				ReceiptHandle:  "yep!",
				Body:           TestEventJSONFromSQS,
				Md5OfBody:      "2d01d5d9c24034d54fe4fba0ede5182d",
				EventSourceARN: "arn:aws:sqs:us-east-1:123456789012:some_random_sqs_queue",
				AWSRegion:      "us-east-1",
				EventSource:    "aws:sqs",
			},
		},
	}

	// TestOKAlarm is just a test cloudwatch alarm detail used for various tests (in the OK state)
	TestOKAlarm = cw.AlarmStateChange{
		AlarmName: "test-service-alarm-abcd",
		State: cw.State{
			Value:      "OK",
			Timestamp:  "2020-07-31T06:56:05.606+0000",
			ReasonData: "{\"version\":\"1.0\",\"queryDate\":\"2020-07-31T06:56:05.603+0000\",\"startDate\":\"2020-07-31T06:50:00.000+0000\",\"statistic\":\"Average\",\"period\":60,\"recentDatapoints\":[0.2872183414376726],\"threshold\":60.0}",
			Reason:     "Threshold Crossed: 1 datapoint [0.2872183414376726 (31/07/20 06:50:00)] was not greater than the threshold (60.0).",
		},
		PreviousState: cw.State{
			Value:      "INSUFFICIENT_DATA",
			Timestamp:  "2020-07-31T06:52:05.601+0000",
			ReasonData: "{\"version\":\"1.0\",\"queryDate\":\"2020-07-31T06:52:05.598+0000\",\"statistic\":\"Average\",\"period\":60,\"recentDatapoints\":[],\"threshold\":60.0}",
			Reason:     "Insufficient Data: 1 datapoint was unknown.",
		},
		Configuration: cw.Configuration{
			Metrics: []cw.MetricDataQuery{
				{
					ReturnData: true,
					ID:         "beb9cf5d-585d-3d43-179b-1e0898d8e237",
					MetricStat: &cw.MetricStat{
						Stat:   "Average",
						Period: 60,
						Metric: cw.Metric{
							Namespace: "AWS/EC2",
							Name:      "CPUUtilization",
							Dimensions: map[string]string{
								"AutoScalingGroupName": "test-service",
							},
						},
					},
				},
			},
		},
	}

	// TestTriggeredAlarm is TestOKAlarm transitioned into the ALARM state
	TestTriggeredAlarm = cw.AlarmStateChange{
		AlarmName: "test-service-alarm-abcd",
		State: cw.State{
			Value:      "ALARM",
			Timestamp:  "2020-07-31T06:56:05.606+0000",
			ReasonData: "{\"version\":\"1.0\",\"queryDate\":\"2020-07-31T06:56:05.603+0000\",\"startDate\":\"2020-07-31T06:50:00.000+0000\",\"statistic\":\"Average\",\"period\":60,\"recentDatapoints\":[0.2872183414376726],\"threshold\":60.0}",
			Reason:     "Threshold Crossed: 1 datapoint [200.0 (31/07/20 06:50:00)] was greater than the threshold (60.0).",
		},
		PreviousState: cw.State{
			Value:      "INSUFFICIENT_DATA",
			Timestamp:  "2020-07-31T06:52:05.601+0000",
			ReasonData: "{\"version\":\"1.0\",\"queryDate\":\"2020-07-31T06:52:05.598+0000\",\"statistic\":\"Average\",\"period\":60,\"recentDatapoints\":[],\"threshold\":60.0}",
			Reason:     "Insufficient Data: 1 datapoint was unknown.",
		},
		Configuration: TestOKAlarm.Configuration,
	}

	// ExpectedAlarmDetails is the parsed struct of TestEventJSON
	ExpectedAlarmDetails = cw.Event{
		Account: "1234567890123",
		Version: "0",
		Time:    "2020-07-31T06:56:05Z",
		Source:  "aws.cloudwatch",
		Resources: []string{
			"arn:aws:cloudwatch:us-east-1:1234567890123:alarm:test-service-alarm-abcd",
		},
		Region:     "us-east-1",
		ID:         "1ebd31a7-11e5-b52a-03e4-be4110b0f1e0",
		DetailType: "CloudWatch Alarm State Change",
		Detail:     TestOKAlarm,
	}

	// TriggeredAlarmDetails is a sample CW alarm event in ALARM state
	TriggeredAlarmDetails = cw.Event{
		Account: "1234567890123",
		Version: "0",
		Time:    "2020-07-31T06:56:05Z",
		Source:  "aws.cloudwatch",
		Resources: []string{
			"arn:aws:cloudwatch:us-east-1:1234567890123:alarm:test-service-alarm-abcd",
		},
		Region:     "us-east-1",
		ID:         "1ebd31a7-11e5-b52a-03e4-be4110b0f1e0",
		DetailType: "CloudWatch Alarm State Change",
		Detail:     TestTriggeredAlarm,
	}

	// SuppressedAlarmDetails is TriggeredAlarmDetails pointed at an alarm
	// whose tags carry alerts:suppress_pagerduty=true (see TagsByARN).
	SuppressedAlarmDetails = cw.Event{
		Account: "1234567890123",
		Version: "0",
		Time:    "2020-07-31T06:56:05Z",
		Source:  "aws.cloudwatch",
		Resources: []string{
			"arn:aws:cloudwatch:us-east-1:1234567890123:alarm:suppressed-alarm",
		},
		Region:     "us-east-1",
		ID:         "0cd5bdb7-df8b-f066-50c5-cc06faac60c2",
		DetailType: "CloudWatch Alarm State Change",
		Detail:     TestTriggeredAlarm,
	}

	// TagsByARN holds the tags the mock CloudWatch client returns per alarm ARN.
	TagsByARN = map[string]map[string]string{
		"arn:aws:cloudwatch:us-east-1:1234567890123:alarm:test-service-alarm-abcd": {
			"owner":   "test",
			"service": "test-service",
		},
		"arn:aws:cloudwatch:us-east-1:1234567890123:alarm:test-service_aurora_serverless_cpu_utilization": {
			"owner":   "test",
			"service": "test-service",
		},
		"arn:aws:cloudwatch:us-east-1:1234567890123:alarm:suppressed-alarm": {
			"owner":                     "test",
			"service":                   "test-service",
			"alerts:suppress_pagerduty": "true",
		},
	}
)

// MockCWAPI is a mock CloudWatch API for testing.
type MockCWAPI struct {
	// LastWidgetJSON records the widget definition of the most recent
	// GetMetricWidgetImage call.
	LastWidgetJSON string
}

// ListTagsForResource implements the list tags api call.
func (m *MockCWAPI) ListTagsForResource(ctx context.Context, r *cloudwatch.ListTagsForResourceInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.ListTagsForResourceOutput, error) {
	tags, ok := TagsByARN[aws.ToString(r.ResourceARN)]
	if !ok {
		return nil, &cwtypes.ResourceNotFoundException{
			Message: aws.String(fmt.Sprintf("resource %s not found", aws.ToString(r.ResourceARN))),
		}
	}
	out := &cloudwatch.ListTagsForResourceOutput{}
	for k, v := range tags {
		out.Tags = append(out.Tags, cwtypes.Tag{Key: aws.String(k), Value: aws.String(v)})
	}
	return out, nil
}

// GetMetricWidgetImage implements the metric widget rendering api call.
func (m *MockCWAPI) GetMetricWidgetImage(ctx context.Context, r *cloudwatch.GetMetricWidgetImageInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricWidgetImageOutput, error) {
	m.LastWidgetJSON = aws.ToString(r.MetricWidget)
	return &cloudwatch.GetMetricWidgetImageOutput{MetricWidgetImage: TestPNG}, nil
}

// GenTestSQSEvent returns a test SQS event with a correct body md5.
func GenTestSQSEvent() awsevents.SQSEvent {
	evt := testSQSEvent
	for idx := range evt.Records {
		h := md5.New()
		io.WriteString(h, evt.Records[idx].Body)
		evt.Records[idx].Md5OfBody = fmt.Sprintf("%x", h.Sum(nil))
	}
	return evt
}
