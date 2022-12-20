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

package graph_test

import (
	"testing"

	"github.com/tidal-open-source/cw-alert-router/graph"
	"github.com/tidal-open-source/cw-alert-router/test"
)

func TestChartGen(t *testing.T) {
	minSize := 1024
	result := test.GetMetricDataResponses["m0"].MetricDataResults[0]
	buf, err := graph.CreateFromCWMetricDataResult(result)
	if err != nil {
		t.Errorf("Failed creating chart: %v", err)
	}
	if buf.Len() < minSize {
		t.Errorf("image size (%d) below minimum (%d)", buf.Len(), minSize)
	}
	/*
		// writing to file for now for some manual eyeball testing
		testFile := "test_chart.png"
		f, err := os.Create(testFile)
		if err != nil {
			t.Errorf("Failed creating file: %v", err)
		}
		defer f.Close()

		n, err := buf.WriteTo(f)
		if err != nil {
			t.Errorf("Failed writing to file: %v", err)
		}
		t.Logf("Wrote %d bytes to %s", n, testFile)
	*/
}
