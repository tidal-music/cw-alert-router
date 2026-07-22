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

package s3_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/tidal-music/cw-alert-router/v2/s3"
	"github.com/tidal-music/cw-alert-router/v2/test"
)

func TestWriteBytes(t *testing.T) {
	mock := &test.MockS3API{}
	client, err := s3.New(context.Background(), s3.WithAPI(mock))
	if err != nil {
		t.Fatalf("Failed initializing mock s3 client: %v", err)
	}

	testData := []byte("abc123")
	key := "test/object1.png"
	bucket := "test-bucket-1"

	if err := client.WriteBytes(context.Background(), bucket, key, bytes.NewReader(testData)); err != nil {
		t.Fatalf("Error writing data to s3: %v", err)
	}

	written, ok := mock.Object(bucket, key)
	if !ok {
		t.Fatalf("object %s/%s was not written (objects: %v)", bucket, key, mock.Objects())
	}
	if !bytes.Equal(testData, written) {
		t.Errorf("written data (%s) didn't match original (%s)", written, testData)
	}
}

func TestPresignedURL(t *testing.T) {
	client, err := s3.New(context.Background(), s3.WithAPI(&test.MockS3API{}), s3.WithPresigner(&test.MockS3Presigner{}))
	if err != nil {
		t.Fatalf("Failed initializing mock s3 client: %v", err)
	}

	url, err := client.PresignedURL(context.Background(), "test-bucket-1", "test/object1.png", time.Hour)
	if err != nil {
		t.Fatalf("Error presigning url: %v", err)
	}
	if !strings.Contains(url, "test-bucket-1") || !strings.Contains(url, "test/object1.png") {
		t.Errorf("presigned url doesn't reference the object: %s", url)
	}
}
