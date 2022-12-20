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
	"fmt"
	"testing"

	diff "github.com/yudai/gojsondiff"
	"github.com/yudai/gojsondiff/formatter"
)

// DiffJSON - function to prettify json diffs
func DiffJSON(t *testing.T, src []byte, dst []byte) error {
	differ := diff.New()
	d, err := differ.Compare(src, dst)
	if err != nil {
		t.Fatalf("Failed running diff on the json: %v", err)
	}

	if d.Modified() {
		var aJSON map[string]interface{}
		json.Unmarshal(src, &aJSON)

		config := formatter.AsciiFormatterConfig{
			ShowArrayIndex: true,
			Coloring:       true,
		}

		formatter := formatter.NewAsciiFormatter(aJSON, config)
		diffString, err := formatter.Format(d)
		if err != nil {
			t.Fatalf("failed formatting json diff: %v", err)
		}
		fmt.Print(diffString)
	} else {
		return fmt.Errorf("JSON diff contained no changeds")
	}
	return nil
}
