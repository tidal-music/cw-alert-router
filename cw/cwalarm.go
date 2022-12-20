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
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
)

const (
	//MetricNamesKey is the map[string]string key used for the metric names in MetricList()
	MetricNamesKey = "names"
	//MetricDimensionsKey is the map[string]string key used for the metric dimensions in MetricList()
	MetricDimensionsKey = "dimensions"
	//MetricNamespacesKey is the map[string]string key used for the metric namespaces in MetricList()
	MetricNamespacesKey = "namespaces"
)

// AlarmEventDetail wraps the cloudwatch alarm detail as sent via Eventbridge -> SQS -> Lambda
type AlarmEventDetail struct {
	State         AlarmState         `json:"state"`
	PreviousState AlarmState         `json:"previousState"`
	Configuration AlarmConfiguration `json:"configuration"`
	AlarmName     string             `json:"alarmName"`
	// Tags is not actually present in the event payload.  We need to inject this ourselves
	Tags AlarmTags `json:"tags"`
}

// AlarmState contains the cloudwatch alarm state details
type AlarmState struct {
	Value      string `json:"value"`
	Timestamp  string `json:"timestamp"`
	ReasonData string `json:"reasonData"`
	Reason     string `json:"reason"`
}

// AlarmConfiguration wraps cloudwatch alarm config from the event payload
type AlarmConfiguration struct {
	Metrics []AlarmMetric `json:"metrics"`
}

// AlarmMetric wraps the cloudwatch alarm metric config
type AlarmMetric struct {
	ReturnData bool            `json:"returnData"`
	ID         string          `json:"id"`
	MetricStat AlarmMetricStat `json:"metricStat"`
}

// AlarmMetricStat wraps the cloudwatch alarm metric stat config
type AlarmMetricStat struct {
	Stat   string            `json:"stat"`
	Period int64             `json:"period"`
	Metric AlarmMetricDetail `json:"metric"`
}

// AlarmMetricDetail wraps the individual alarm metric
type AlarmMetricDetail struct {
	Namespace  string            `json:"namespace"`
	Name       string            `json:"name"`
	Dimensions map[string]string `json:"dimensions"`
}

// AWSDimensions returns our map[string]string dimensions in aws formatted cloudwatch.Dimension
func (d AlarmMetricDetail) AWSDimensions() []*cloudwatch.Dimension {
	var dims []*cloudwatch.Dimension

	for k, v := range d.Dimensions {
		dims = append(dims, &cloudwatch.Dimension{
			Name:  aws.String(k),
			Value: aws.String(v),
		})
	}

	return dims
}
