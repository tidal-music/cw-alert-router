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

// Package parameterstore wraps AWS Systems Manager Parameter Store in a basic
// interface that provides calls specific to our needs.
package parameterstore

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

// API is the subset of the Systems Manager API this service uses.
type API interface {
	GetParameter(ctx context.Context, params *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
}

// Client is our own parameter store client.
type Client struct {
	api API
}

// New returns a Client backed by the real Systems Manager API.
func New(cfg aws.Config) *Client {
	return &Client{api: ssm.NewFromConfig(cfg)}
}

// NewWithAPI returns a Client backed by the given API implementation (for testing).
func NewWithAPI(api API) *Client {
	return &Client{api: api}
}

// GetParameterValue returns the string value of a given parameter store key
// (decrypted if it is a SecureString).
func (c *Client) GetParameterValue(ctx context.Context, key string) (string, error) {
	resp, err := c.api.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           aws.String(key),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return "", err
	}
	return aws.ToString(resp.Parameter.Value), nil
}

// IsNotFound reports whether the error means the requested parameter does not exist.
func IsNotFound(err error) bool {
	var nf *types.ParameterNotFound
	return errors.As(err, &nf)
}
