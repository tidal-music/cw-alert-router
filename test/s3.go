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

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	s3api "github.com/aws/aws-sdk-go-v2/service/s3"
)

// MockS3API is a mock S3 client that records written objects.
type MockS3API struct {
	mu      sync.Mutex
	objects map[string][]byte
}

// PutObject implements the said function for the s3 client.
func (m *MockS3API) PutObject(ctx context.Context, req *s3api.PutObjectInput, optFns ...func(*s3api.Options)) (*s3api.PutObjectOutput, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.objects == nil {
		m.objects = make(map[string][]byte)
	}
	m.objects[fmt.Sprintf("%s/%s", aws.ToString(req.Bucket), aws.ToString(req.Key))] = body
	return &s3api.PutObjectOutput{ETag: aws.String("blah")}, nil
}

// Object returns the recorded content written to bucket/key, if any.
func (m *MockS3API) Object(bucket, key string) ([]byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	obj, ok := m.objects[fmt.Sprintf("%s/%s", bucket, key)]
	return obj, ok
}

// Objects returns all recorded object keys ("bucket/key").
func (m *MockS3API) Objects() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var keys []string
	for k := range m.objects {
		keys = append(keys, k)
	}
	return keys
}

// MockS3Presigner is a mock S3 presign client returning deterministic URLs.
type MockS3Presigner struct{}

// PresignGetObject implements the presigner interface.
func (m *MockS3Presigner) PresignGetObject(ctx context.Context, req *s3api.GetObjectInput, optFns ...func(*s3api.PresignOptions)) (*v4.PresignedHTTPRequest, error) {
	return &v4.PresignedHTTPRequest{
		URL:    fmt.Sprintf("https://%s.s3.amazonaws.com/%s?X-Amz-Signature=fake", aws.ToString(req.Bucket), aws.ToString(req.Key)),
		Method: "GET",
	}, nil
}
