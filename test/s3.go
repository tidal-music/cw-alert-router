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
	"io"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client/metadata"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	log "github.com/sirupsen/logrus"
)

// MockRetryer is the request.Retryer to use in our fake requests
type MockRetryer struct{}

// RetryRules implements said fn
func (mr MockRetryer) RetryRules(r *request.Request) time.Duration {
	return time.Second * 3
}

// ShouldRetry implements said fn
func (mr MockRetryer) ShouldRetry(r *request.Request) bool {
	return true
}

// MaxRetries implements said fn
func (mr MockRetryer) MaxRetries() int {
	return 3
}

// MockS3Client is a mock S3 client for testing
type MockS3Client struct {
	s3iface.S3API
}

func (m *MockS3Client) clientInfo() metadata.ClientInfo {
	ci := metadata.ClientInfo{
		ServiceName: "s3",
		ServiceID:   "s3",
		APIVersion:  "v4",
		Endpoint:    "https://s3-fake-endpoint.fake.notexist",
	}
	return ci
}

var s3PutObjectChannel chan []byte
var s3PutObjectChannelComplete chan bool

// SetS3ReceiveChannel sets up a channel so we can tap into s3 object puts to just make sure we're writing what we think we are
func SetS3ReceiveChannel(ch chan []byte, quit chan bool) {
	s3PutObjectChannel = ch
	s3PutObjectChannelComplete = quit
}

func writeObject(r io.Reader) {
	go func() {
		for {
			buf := make([]byte, 8)
			n, err := r.Read(buf)
			log.Infof("sending %v (%d)", buf, n)
			if n > 0 {
				s3PutObjectChannel <- buf[:n]
			}
			if err == io.EOF {
				break
			}
		}
		log.Infof("sending quit...")
		s3PutObjectChannelComplete <- true
	}()
}

// PutObject implements the said function for the s3 client
func (m *MockS3Client) PutObject(req *s3.PutObjectInput) (*s3.PutObjectOutput, error) {
	log.Infof("mock.PutObject(%v)", req)
	if s3PutObjectChannel != nil {
		writeObject(req.Body)
	}
	resp := &s3.PutObjectOutput{
		ETag: aws.String("blah"),
	}
	return resp, nil
}

func fakeSign(i request.HandlerListRunItem) bool {
	log.Infof("fakeSign(req)")
	return false
}

func fakeBeforePresign(r *request.Request) error {
	log.Infof("fakeBeforePresign(req)")
	return nil
}

// GetObjectRequest implements that guy
func (m *MockS3Client) GetObjectRequest(req *s3.GetObjectInput) (*request.Request, *s3.GetObjectOutput) {
	handlers := request.Handlers{
		Sign: request.HandlerList{
			AfterEachFn: fakeSign,
		},
	}

	op := &request.Operation{
		Name:            "GetObject",
		HTTPMethod:      "GET",
		HTTPPath:        "/get/blah/blah/blah",
		BeforePresignFn: fakeBeforePresign,
	}

	mockRequest := request.New(aws.Config{}, m.clientInfo(), handlers, MockRetryer{}, op, nil, nil)

	return mockRequest, nil
}
