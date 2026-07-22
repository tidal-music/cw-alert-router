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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

const (
	// SlackTokenSSMKey is a test SSM parameter store key
	SlackTokenSSMKey = "/service/cw_alert_router/slack/app/oauth/auth_token"
	// SlackTokenValue is the token stored under SlackTokenSSMKey
	SlackTokenValue = "abc123"
)

// TestSSMParameters defines the parameters the mock Systems Manager client serves.
var TestSSMParameters = map[string]string{
	"/service/cw_alert_router/pagerduty/routing_keys/test_service": "pagerduty-key-1",
	"/service/cw_alert_router/pagerduty/routing_keys/shared_key":   "shared-key-test-string",
	SlackTokenSSMKey: SlackTokenValue,
}

// MockSSMClient is a mock Systems Manager client for testing.
type MockSSMClient struct{}

// GetParameter implements the same function from ssm.
func (m *MockSSMClient) GetParameter(ctx context.Context, req *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
	if value, ok := TestSSMParameters[aws.ToString(req.Name)]; ok {
		return &ssm.GetParameterOutput{
			Parameter: &ssmtypes.Parameter{Value: aws.String(value)},
		}, nil
	}
	return nil, &ssmtypes.ParameterNotFound{
		Message: aws.String(fmt.Sprintf("parameter %s not found", aws.ToString(req.Name))),
	}
}
