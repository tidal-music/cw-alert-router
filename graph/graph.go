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

package graph

import (
	"bytes"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/wcharczuk/go-chart"
)

const (
	// DefaultGraphWidth default image size of graphs generated
	DefaultGraphWidth = 512
	// DefaultGraphHeight default image size of graphs generated
	DefaultGraphHeight = 200
)

func timeFormat(v interface{}) string {
	var t time.Time
	var isTyped bool
	t, isTyped = v.(time.Time)
	if !isTyped {
		var typed int64
		if typed, isTyped = v.(int64); isTyped {
			t = time.Unix(0, typed)
		}
	}

	if !isTyped {
		var typed float64
		if typed, isTyped = v.(float64); isTyped {
			t = time.Unix(0, int64(typed))
		}
	}

	if isTyped {
		// we have a time.  Adjust it to UTC
		t = t.UTC()
		ts := fmt.Sprintf("%02d:%02d:%02d", t.Hour(), t.Minute(), t.Second())
		return ts
	}

	return ""
}

// CreateFromCWMetricDataResult creates a PNG chart from the given metric-data query result set
func CreateFromCWMetricDataResult(res *cloudwatch.MetricDataResult) (*bytes.Buffer, error) {
	//series := make([]chart.Series, 1)
	dataPoints := len(res.Timestamps)
	xvals := make([]time.Time, dataPoints)
	yvals := make([]float64, dataPoints)

	for idx, v := range res.Timestamps {
		xvals[idx] = *v
		yvals[idx] = *res.Values[idx]
	}

	graph := chart.Chart{
		Width:  DefaultGraphWidth,
		Height: DefaultGraphHeight,
		YAxis: chart.YAxis{
			Name: "Value",
		},
		XAxis: chart.XAxis{
			Name:           "Time",
			ValueFormatter: timeFormat,
		},
		Series: []chart.Series{
			chart.TimeSeries{
				XValues: xvals,
				YValues: yvals,
			},
		},
	}

	buffer := bytes.NewBuffer([]byte{})
	err := graph.Render(chart.PNG, buffer)
	if err != nil {
		return nil, err
	}
	return buffer, nil
}
