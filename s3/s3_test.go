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
	"reflect"
	"testing"

	"github.com/tidal-open-source/cw-alert-router/s3"
	"github.com/tidal-open-source/cw-alert-router/test"
)

var (
	s3client                   *s3.Client
	s3PutObjectChannel         chan []byte
	s3PutObjectCompleteChannel chan bool
)

func setup(t *testing.T) {
	var err error
	mc := &test.MockS3Client{}
	s3PutObjectChannel = make(chan []byte, 1)
	s3PutObjectCompleteChannel = make(chan bool, 1)
	s3client, err = s3.New(s3.WithS3APIClient(mc))
	if err != nil {
		t.Fatalf("Failed initializing mock s3 client: %v", err)
	}
}

func TestGetValidResponse(t *testing.T) {
	setup(t)
	testData := []byte("abc123")
	key := "test/object1.png"
	bucket := "test-bucket-1"
	test.SetS3ReceiveChannel(s3PutObjectChannel, s3PutObjectCompleteChannel)
	err := s3client.WriteBytes(bucket, key, bytes.NewReader(testData))
	if err != nil {
		t.Errorf("Error writing data to s3: %v", err)
	}

	var b []byte
readloop:
	for {
		select {
		case <-s3PutObjectCompleteChannel:
			t.Logf("quit received")
			break readloop
		default:
			b = <-s3PutObjectChannel
			t.Logf("read: %v", b)
		}
	}
	if !reflect.DeepEqual(testData, b) {
		t.Errorf("written data (%s) didn't match original (%s)", string(b), string(testData))
	}
}

// No longer used.  instance role presigned links expire in 6 hours!
//func TestGetObjectLink(t *testing.T) {
//	setup(t)
//	key := "test/object1.png"
//	bucket := "test-bucket-1"
//	link, err := s3client.GetObjectLink(bucket, key)
//	if err != nil {
//		t.Errorf("error getting object link: %v", err)
//	}
//	t.Logf("got link: %s", link)
//}
