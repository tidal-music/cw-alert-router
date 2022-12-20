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

package cw

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/cloudwatch/cloudwatchiface"
)

// Client provides methods to get information on a specific cloudwatch alarm.
type Client struct {
	CWClient cloudwatchiface.CloudWatchAPI
}

// AlarmTags is an AWS tag list converted to a map[string]string
type AlarmTags map[string]string

// New returns a new cw Client
func New() (*Client, error) {
	sess, err := session.NewSession()
	if err != nil {
		return nil, err
	}
	svc := cloudwatch.New(sess)
	return &Client{
		CWClient: svc,
	}, nil
}

// GetAlarmTags returns a list of alarm tags from the given alarm ARN
func (c *Client) GetAlarmTags(alarmARN string) (AlarmTags, error) {
	tags := make(AlarmTags)
	req := &cloudwatch.ListTagsForResourceInput{
		ResourceARN: aws.String(alarmARN),
	}
	resp, err := c.CWClient.ListTagsForResource(req)
	if err != nil {
		return nil, err
	}
	for _, tag := range resp.Tags {
		tags[*tag.Key] = *tag.Value
	}

	return tags, nil
}
