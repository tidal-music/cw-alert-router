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

package cw

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	log "github.com/sirupsen/logrus"
)

// EventDetails contains the actual event details as received by SQS
type EventDetails struct {
	Account    string           `json:"account"`
	Version    string           `json:"version"`
	Time       string           `json:"time"`
	Source     string           `json:"source"`
	Resources  []string         `json:"resources"`
	Region     string           `json:"region"`
	ID         string           `json:"id"`
	DetailType string           `json:"detail-type"`
	Detail     AlarmEventDetail `json:"detail"`
	cwClient   *Client
}

// MetricList returns a map[string]string with all metric names & namespaces for this alarm
// (it will stick all namespaces, dimensions and names under the respective keys...  if there are more than 1 of each)
func (d *EventDetails) MetricList() map[string][]string {
	list := make(map[string][]string)
	list[MetricDimensionsKey] = make([]string, 0)
	list[MetricNamesKey] = make([]string, 0)
	list[MetricNamespacesKey] = make([]string, 0)
	for _, metric := range d.Detail.Configuration.Metrics {
		m := metric.MetricStat.Metric
		if m.Namespace != "" {
			list[MetricNamespacesKey] = append(list[MetricNamespacesKey], m.Namespace)
		}
		if m.Name != "" {
			list[MetricNamesKey] = append(list[MetricNamesKey], m.Name)
		}
		for k, v := range m.Dimensions {
			list[MetricDimensionsKey] = append(list[MetricDimensionsKey], fmt.Sprintf("%s:%s", k, v))
		}
	}
	return list
}

// AlarmARN returns the arn for the alarm that sent this event
func (d *EventDetails) AlarmARN() (string, error) {
	if len(d.Resources) != 1 {
		return "", fmt.Errorf("resources in the cloudwatch alarm details must be exactly 1 (we got %d)", len(d.Resources))
	}
	return d.Resources[0], nil
}

// AlarmConsoleLink provides a URL straight to the AWS console so users can view the alarm details
func (d *EventDetails) AlarmConsoleLink() string {
	baseURL := "https://console.aws.amazon.com/cloudwatch/home"
	return fmt.Sprintf("%s?region=%s#alarmsV2:alarm/%s", baseURL, d.Region, d.Detail.AlarmName)
}

// SetCWClient sets the cloudwatch client to use for internal operations
func (d *EventDetails) SetCWClient(c *Client) {
	d.cwClient = c
}

// InjectTags adds the AWS tags on the related cloudwatch alarm
func (d *EventDetails) InjectTags() error {
	if d.cwClient == nil {
		return fmt.Errorf("cloudwatch client is nil")
	}
	arn, err := d.AlarmARN()
	if err != nil {
		return err
	}

	tags, err := d.cwClient.GetAlarmTags(arn)
	if err != nil {
		return err
	}

	d.Detail.Tags = tags

	return nil
}

// GetMetricDataForHrs returns metric data for the given event over the last N hours (default is 1 hour)
func (d *EventDetails) GetMetricDataRequestForHrs(hrs ...int) (*cloudwatch.MetricDataResult, error) {

	if d.cwClient == nil {
		return nil, fmt.Errorf("cloudwatch client is nil")
	}

	hours := 1
	if len(hrs) > 0 {
		hours = hrs[0]
	}

	// Align the start/end time with the period of the metric query
	currTime := time.Now()
	period := d.Detail.Configuration.Metrics[0].MetricStat.Period
	endTime := currTime.Truncate(time.Duration(period/60) * time.Minute)
	startTime := endTime.Add(time.Duration(-hours) * time.Hour)
	// FIXME: the whole event/cwalarm struct is a mess...  really need to change this all around so we can just pass in as is to diff
	// AWS functions
	// (ie: see below where we are manually mapping our fields to the same damn things on an aws defined struct...)
	// (should have just gone straight from JSON -> cloudwatch.structs)
	var metrics []*cloudwatch.MetricDataQuery

	//TODO: this only handles simple queries.  If we have advanced queries - ie: multiple queries with expressions,
	// then this needs work...
	for idx, m := range d.Detail.Configuration.Metrics {
		// FIXME: this is to work around the fact that:
		// a) AWS have a validation on metric data query ids: ^[a-z][a-zA-Z0-9_]*$
		// b) the sqs event payload coming from cloudwatch can contain ids that don't conform to this...
		mID := fmt.Sprintf("m%d", idx)
		metrics = append(metrics, &cloudwatch.MetricDataQuery{
			Id:         aws.String(mID),
			ReturnData: aws.Bool(m.ReturnData),
			MetricStat: &cloudwatch.MetricStat{
				Period: aws.Int64(m.MetricStat.Period),
				Stat:   aws.String(m.MetricStat.Stat),
				Metric: &cloudwatch.Metric{
					MetricName: aws.String(m.MetricStat.Metric.Name),
					Namespace:  aws.String(m.MetricStat.Metric.Namespace),
					Dimensions: m.MetricStat.Metric.AWSDimensions(),
				},
			},
		})
	}

	metricRequest := &cloudwatch.GetMetricDataInput{
		EndTime:           &endTime,
		StartTime:         &startTime,
		MetricDataQueries: metrics,
	}
	// FIXME: see - this is horrible...
	resp, err := d.cwClient.CWClient.GetMetricData(metricRequest)
	if err != nil {
		return nil, err
	}

	if len(resp.MetricDataResults) != 1 {
		return nil, fmt.Errorf("number of metric data results != 1 (got %d)", len(resp.MetricDataResults))
	}

	log.Debugf("received metrics: %+v", resp.MetricDataResults[0])

	return resp.MetricDataResults[0], nil
}
