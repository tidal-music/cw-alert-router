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

// Package parameterstore wraps AWS Systems Manager Parameter Store in a basic interface that provides calls specific to our needs
package parameterstore

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
)

// Client is our own parameter store client
type Client struct {
	psClient ssmiface.SSMAPI
}

// New returns a new Client instance
func New() (*Client, error) {
	client := ssm.New(session.New())
	return &Client{
		psClient: client,
	}, nil
}

// NewWithSSMClient creates a new Client instance with a provided ssm client object
func NewWithSSMClient(c ssmiface.SSMAPI) (*Client, error) {
	return &Client{
		psClient: c,
	}, nil
}

// GetParameterValue returns the string value of a given parameter store key (unencrypted if so)
func (c *Client) GetParameterValue(key string) (string, error) {
	req := &ssm.GetParameterInput{
		Name:           aws.String(key),
		WithDecryption: aws.Bool(true),
	}

	resp, err := c.psClient.GetParameter(req)

	if err != nil {
		return "", err
	}

	return *resp.Parameter.Value, nil
}
