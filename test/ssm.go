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
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
)

const (
	// SlackTokenSSMKey is a test SSM parameter store key
	SlackTokenSSMKey = "/service/cw_alert_router/slack/app/oauth/auth_token"
)

var (
	// TestGetParametersByPathResponse defines some test responses for get parameters by path
	TestSSMGetParametersByPathResponse = map[string]*ssm.GetParametersByPathOutput{
		"/service/cw_alert_router/pagerduty/routing_keys/shared_key": {
			Parameters: []*ssm.Parameter{
				{
					Value: aws.String("shared-key-test-string"),
				},
			},
		},
		"/service/cw_alert_router/pagerduty/routing_keys/test_service": {
			Parameters: []*ssm.Parameter{
				{
					Value: aws.String("pagerduty-key-1"),
				},
			},
		},
	}

	// TestGetParameterResponse defines some test reponses for get parameter
	TestSSMGetParameterResponse = map[string]*ssm.GetParameterOutput{
		"/service/cw_alert_router/pagerduty/routing_keys/test_service": {
			Parameter: &ssm.Parameter{
				Value: aws.String("pagerduty-key-1"),
			},
		},
		"/service/cw_alert_router/pagerduty/routing_keys/shared_key": {
			Parameter: &ssm.Parameter{
				Value: aws.String("shared-key-test-string"),
			},
		},
		SlackTokenSSMKey: {
			Parameter: &ssm.Parameter{
				Value: aws.String("abc123"),
			},
		},
	}
)

// MockSSMClient is a mock SSM client for testing
type MockSSMClient struct {
	ssmiface.SSMAPI
}

// GetParametersByPath implements the said function for the ssm client
func (m *MockSSMClient) GetParametersByPath(req *ssm.GetParametersByPathInput) (*ssm.GetParametersByPathOutput, error) {
	if response, ok := TestSSMGetParametersByPathResponse[*req.Path]; ok {
		return response, nil
	}
	return nil, awserr.New(ssm.ErrCodeInvalidKeyId, fmt.Sprintf("resources %+v not found", req), nil)
}

// GetParameter implements the same function from ssm
func (m *MockSSMClient) GetParameter(req *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
	if response, ok := TestSSMGetParameterResponse[*req.Name]; ok {
		return response, nil
	}

	return nil, awserr.New(ssm.ErrCodeInvalidKeyId, fmt.Sprintf("resources %+v not found", req), nil)
}
