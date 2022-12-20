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
	"crypto/md5"
	"fmt"
	"io"
	"regexp"
	"time"

	"github.com/tidal-open-source/cw-alert-router/cw"

	awsevents "github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/cloudwatch/cloudwatchiface"
)

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

	TestSuppressPagerDuty = cw.AlarmEventDetail{
		AlarmName: "arn:aws:cloudwatch:us-east-1:1234567890123:alarm:test-service-alarm-abcd",
		State: cw.AlarmState{
			Value:      "ALARM",
			Timestamp:  "2022-09-29T22:10:04.692+0000",
			ReasonData: "{\\\"version\\\":\\\"1.0\\\",\\\"queryDate\\\":\\\"2022-09-29T22:10:04.689+0000\\\",\\\"startDate\\\":\\\"2022-09-29T21:40:00.000+0000\\\",\\\"statistic\\\":\\\"Sum\\\",\\\"period\\\":300,\\\"recentDatapoints\\\":[0.0,0.0,0.0,0.0,0.0,0.0],\\\"threshold\\\":0.0,\\\"evaluatedDatapoints\\\":[{\\\"timestamp\\\":\\\"2022-09-29T22:05:00.000+0000\\\",\\\"sampleCount\\\":5.0,\\\"value\\\":0.0},{\\\"timestamp\\\":\\\"2022-09-29T22:00:00.000+0000\\\",\\\"sampleCount\\\":5.0,\\\"value\\\":0.0},{\\\"timestamp\\\":\\\"2022-09-29T21:55:00.000+0000\\\",\\\"sampleCount\\\":5.0,\\\"value\\\":0.0},{\\\"timestamp\\\":\\\"2022-09-29T21:50:00.000+0000\\\",\\\"sampleCount\\\":5.0,\\\"value\\\":0.0},{\\\"timestamp\\\":\\\"2022-09-29T21:45:00.000+0000\\\",\\\"sampleCount\\\":5.0,\\\"value\\\":0.0},{\\\"timestamp\\\":\\\"2022-09-29T21:40:00.000+0000\\\",\\\"sampleCount\\\":5.0,\\\"value\\\":0.0}]}",
			Reason:     "Threshold Crossed: 6 datapoints were less than or equal to the threshold (0.0). The most recent datapoints which crossed the threshold: [0.0 (29/09/22 22:05:00), 0.0 (29/09/22 22:00:00), 0.0 (29/09/22 21:55:00), 0.0 (29/09/22 21:50:00), 0.0 (29/09/22 21:45:00)].",
		},
		PreviousState: cw.AlarmState{
			Value:      "OK",
			Timestamp:  "2022-08-29T13:09:04.688+0000",
			ReasonData: "{\\\"version\\\":\\\"1.0\\\",\\\"queryDate\\\":\\\"2022-08-29T13:09:04.685+0000\\\",\\\"startDate\\\":\\\"2022-08-29T12:39:00.000+0000\\\",\\\"statistic\\\":\\\"Sum\\\",\\\"period\\\":300,\\\"recentDatapoints\\\":[0.0,0.0,0.0,0.0,0.0,9.0],\\\"threshold\\\":0.0,\\\"evaluatedDatapoints\\\":[{\\\"timestamp\\\":\\\"2022-08-29T13:04:00.000+0000\\\",\\\"sampleCount\\\":14.0,\\\"value\\\":9.0}]}",
			Reason:     "Threshold Crossed: 1 datapoint [9.0 (29/08/22 13:04:00)] was not less than or equal to the threshold (0.0).",
		},
		Tags: cw.AlarmTags{
			"Env":                       "test",
			"alerts:suppress_pagerduty": "true",
			"service":                   "test-service",
			"owner":                     "test",
		},
		Configuration: cw.AlarmConfiguration{
			Metrics: []cw.AlarmMetric{
				{
					ReturnData: true,
					ID:         "76bafab0-8d7b-65d4-ad90-808e4af8f7dd",
					MetricStat: cw.AlarmMetricStat{
						Stat:   "Sum",
						Period: 300,
						Metric: cw.AlarmMetricDetail{
							Namespace: "AWS/SQS",
							Name:      "NumberOfEmptyReceives",
							Dimensions: map[string]string{
								"QueueName": "testQueue.fifo",
							},
						},
					},
				},
			},
		},
	}

	// TestOKAlarm is just a test cloudwatch alarm used for various tests (in the OK state)
	TestOKAlarm = cw.AlarmEventDetail{
		AlarmName: "test-service-alarm-abcd",
		State: cw.AlarmState{
			Value:      "OK",
			Timestamp:  "2020-07-31T06:56:05.606+0000",
			ReasonData: "{\"version\":\"1.0\",\"queryDate\":\"2020-07-31T06:56:05.603+0000\",\"startDate\":\"2020-07-31T06:50:00.000+0000\",\"statistic\":\"Average\",\"period\":60,\"recentDatapoints\":[0.2872183414376726],\"threshold\":60.0}",
			Reason:     "Threshold Crossed: 1 datapoint [0.2872183414376726 (31/07/20 06:50:00)] was not greater than the threshold (60.0).",
		},
		PreviousState: cw.AlarmState{
			Value:      "INSUFFICIENT_DATA",
			Timestamp:  "2020-07-31T06:52:05.601+0000",
			ReasonData: "{\"version\":\"1.0\",\"queryDate\":\"2020-07-31T06:52:05.598+0000\",\"statistic\":\"Average\",\"period\":60,\"recentDatapoints\":[],\"threshold\":60.0}",
			Reason:     "Insufficient Data: 1 datapoint was unknown.",
		},
		Configuration: cw.AlarmConfiguration{
			Metrics: []cw.AlarmMetric{
				{
					ReturnData: true,
					ID:         "beb9cf5d-585d-3d43-179b-1e0898d8e237",
					MetricStat: cw.AlarmMetricStat{
						Stat:   "Average",
						Period: 60,
						Metric: cw.AlarmMetricDetail{
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

	// TestNoDataToOKAlarm is for testing with an alarm that just transitioned to OK from INSUFFICIENT_DATA
	TestNoDataToOKAlarm = cw.AlarmEventDetail{
		AlarmName: "test-123-alarm-aabe21de-1236-4742-bf18-34b1c3b2682a",
		State: cw.AlarmState{
			Value:      "OK",
			Timestamp:  "2020-11-11T06:56:05.606+0000",
			ReasonData: "{\"version\":\"1.0\",\"queryDate\":\"2020-11-11T06:56:05.603+0000\",\"startDate\":\"2020-11-11T06:50:00.000+0000\",\"statistic\":\"Average\",\"period\":60,\"recentDatapoints\":[0.2872183414376726],\"threshold\":60.0}",
			Reason:     "Threshold Crossed: 1 datapoint [200.0 (11/11/20 06:50:00)] was greater than the threshold (60.0).",
		},
		PreviousState: cw.AlarmState{
			Value:      "INSUFFICIENT_DATA",
			Timestamp:  "2020-11-11T06:52:05.601+0000",
			ReasonData: "{\"version\":\"1.0\",\"queryDate\":\"2020-11-11T06:52:05.598+0000\",\"statistic\":\"Average\",\"period\":60,\"recentDatapoints\":[],\"threshold\":60.0}",
			Reason:     "Insufficient Data: 1 datapoint was unknown.",
		},
		Configuration: cw.AlarmConfiguration{
			Metrics: []cw.AlarmMetric{
				{
					ReturnData: true,
					ID:         "1eb9cf5d-a85d-3d43-179c-1e0898d8e237",
					MetricStat: cw.AlarmMetricStat{
						Stat:   "Average",
						Period: 60,
						Metric: cw.AlarmMetricDetail{
							Namespace: "AWS/EC2",
							Name:      "CPUUtilization",
							Dimensions: map[string]string{
								"AutoScalingGroupName": "test-123",
							},
						},
					},
				},
			},
		},
	}

	// TestTriggeredAlarm is just a test cloudwatch alarm used for various tests (in the OK state)
	TestTriggeredAlarm = cw.AlarmEventDetail{
		AlarmName: "test-service-alarm-abcd",
		State: cw.AlarmState{
			Value:      "ALARM",
			Timestamp:  "2020-07-31T06:56:05.606+0000",
			ReasonData: "{\"version\":\"1.0\",\"queryDate\":\"2020-07-31T06:56:05.603+0000\",\"startDate\":\"2020-07-31T06:50:00.000+0000\",\"statistic\":\"Average\",\"period\":60,\"recentDatapoints\":[0.2872183414376726],\"threshold\":60.0}",
			Reason:     "Threshold Crossed: 1 datapoint [200.0 (31/07/20 06:50:00)] was greater than the threshold (60.0).",
		},
		PreviousState: cw.AlarmState{
			Value:      "INSUFFICIENT_DATA",
			Timestamp:  "2020-07-31T06:52:05.601+0000",
			ReasonData: "{\"version\":\"1.0\",\"queryDate\":\"2020-07-31T06:52:05.598+0000\",\"statistic\":\"Average\",\"period\":60,\"recentDatapoints\":[],\"threshold\":60.0}",
			Reason:     "Insufficient Data: 1 datapoint was unknown.",
		},
		Configuration: cw.AlarmConfiguration{
			Metrics: []cw.AlarmMetric{
				{
					ReturnData: true,
					ID:         "beb9cf5d-585d-3d43-179b-1e0898d8e237",
					MetricStat: cw.AlarmMetricStat{
						Stat:   "Average",
						Period: 60,
						Metric: cw.AlarmMetricDetail{
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

	// ExpectedAlarmDetails is the parsed struct of the above JSON
	ExpectedAlarmDetails = cw.EventDetails{
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

	// TriggeredAlarmDetails is a sample CW alarm in ALARM state
	TriggeredAlarmDetails = cw.EventDetails{
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

	// InsufficientDataToOKAlarmDetails is for testing insufficient data -> ok alarm
	InsufficientDataToOKAlarmDetails = cw.EventDetails{
		Account: "1234567890123456",
		Version: "0",
		Time:    "2020-11-11T06:56:05Z",
		Source:  "aws.cloudwatch",
		Resources: []string{
			"arn:aws:cloudwatch:us-east-1:1234567890123:alarm:test-service-alarm-abcd",
		},
		Region:     "us-east-1",
		ID:         "1ebd31a7-11e5-b52a-03e4-be4110b0f1e0",
		DetailType: "CloudWatch Alarm State Change",
		Detail:     TestNoDataToOKAlarm,
	}

	// SuppressPagerDutyTrue is for testing events with disabled PD alerts
	SuppressPagerDutyTrue = cw.EventDetails{
		Account: "1234567890123",
		Version: "0",
		Time:    "2022-09-29T22:10:04Z",
		Source:  "aws.cloudwatch",
		Resources: []string{
			"arn:aws:cloudwatch:us-east-1:1234567890123:alarm:test-service-alarm-abcd",
		},
		Region:     "us-east-1",
		ID:         "0cd5bdb7-df8b-f066-50c5-cc06faac60c2",
		DetailType: "CloudWatch Alarm State Change",
		Detail:     TestSuppressPagerDuty,
	}

	_listTagsForResourceResponses = map[string]*cloudwatch.ListTagsForResourceOutput{
		"arn:aws:cloudwatch:us-east-1:1234567890123:alarm:test-service-alarm-abcd": {
			Tags: []*cloudwatch.Tag{
				{
					Key:   aws.String("owner"),
					Value: aws.String("test"),
				},
				{
					Key:   aws.String("service"),
					Value: aws.String("test-service"),
				},
			},
		},
		"arn:aws:cloudwatch:us-east-1:1234567890123:alarm:test-service_aurora_serverless_cpu_utilization": {
			Tags: []*cloudwatch.Tag{
				{
					Key:   aws.String("owner"),
					Value: aws.String("test"),
				},
				{
					Key:   aws.String("service"),
					Value: aws.String("test-service"),
				},
			},
		},
	}

	// GetMetricDataResponses provides some test GetMetricData api responses map key is the MetricDataQuery.Id
	GetMetricDataResponses = map[string]*cloudwatch.GetMetricDataOutput{
		"m0": {
			MetricDataResults: []*cloudwatch.MetricDataResult{
				{
					Id: aws.String("m0"),
					Timestamps: []*time.Time{
						aws.Time(time.Date(2020, time.October, 30, 15, 0, 0, 0, time.UTC)),
						aws.Time(time.Date(2020, time.October, 30, 15, 5, 0, 0, time.UTC)),
						aws.Time(time.Date(2020, time.October, 30, 15, 10, 0, 0, time.UTC)),
						aws.Time(time.Date(2020, time.October, 30, 15, 15, 0, 0, time.UTC)),
						aws.Time(time.Date(2020, time.October, 30, 15, 20, 0, 0, time.UTC)),
						aws.Time(time.Date(2020, time.October, 30, 15, 25, 0, 0, time.UTC)),
					},
					Values: []*float64{
						aws.Float64(15.0),
						aws.Float64(3.0),
						aws.Float64(11.0),
						aws.Float64(9.0),
						aws.Float64(17.0),
						aws.Float64(15.0),
					},
				},
			},
		},
	}
)

// MockCWClient is a mock Cloudwatch client for testing
type MockCWClient struct {
	cloudwatchiface.CloudWatchAPI
}

// ListTagsForResource attempts to implement the list tags api call
func (m *MockCWClient) ListTagsForResource(r *cloudwatch.ListTagsForResourceInput) (*cloudwatch.ListTagsForResourceOutput, error) {
	if response, ok := _listTagsForResourceResponses[*r.ResourceARN]; ok {
		return response, nil
	}

	return nil, awserr.New(cloudwatch.ErrCodeResourceNotFoundException, fmt.Sprintf("resource %+v not found", r), nil)
}

// GetMetricData implements the same fn in cloudwatch
func (m *MockCWClient) GetMetricData(r *cloudwatch.GetMetricDataInput) (*cloudwatch.GetMetricDataOutput, error) {
	// FIXME: only works with single queries
	// note: something aws doesn't advertise - ValidationError: The value 62ba7bc1-7c4c-3747-4ab5-3a3dc4e40530 for parameter MetricDataQueries.member.1.Id is not matching the expected pattern ^[a-z][a-zA-Z0-9_]*$.
	mID := *r.MetricDataQueries[0].Id
	re := regexp.MustCompile(`^[a-z][a-zA-Z0-9_]*$`)
	if !re.MatchString(mID) {
		return nil, fmt.Errorf("%s: The value %s for parameter MetricDataQueries.member.1.Id is not matching the expected pattern ^[a-z][a-zA-Z0-9_]*$", "ValidationError", mID)
	}
	if response, ok := GetMetricDataResponses[*r.MetricDataQueries[0].Id]; ok {
		return response, nil
	}

	return nil, awserr.New(cloudwatch.ErrCodeResourceNotFoundException, fmt.Sprintf("resource %+v not found", r), nil)
}

/* not used for now
func (m *MockCWClient) DescribeAlarms(input *cloudwatch.DescribeAlarmsInput) (*cloudwatch.DescribeAlarmsOutput, error) {
	log.Infof("mock client describe alarms")
	return nil, nil
}
*/

func GenTestSQSEvent() awsevents.SQSEvent {
	evt := testSQSEvent
	for idx := range evt.Records {
		h := md5.New()
		io.WriteString(h, evt.Records[idx].Body)
		evt.Records[idx].Md5OfBody = fmt.Sprintf("%x", h.Sum(nil))
	}
	return evt

}
