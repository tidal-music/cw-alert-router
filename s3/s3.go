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

// Package s3 wraps AWS S3 API in a basic interface that provides calls specific to our needs
package s3

import (
	"io"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	s3api "github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	log "github.com/sirupsen/logrus"
)

// Client is our own parameter store client
type Client struct {
	s3Client s3iface.S3API
	region   string
	roleArn  string
}

// ClientOptions provides the ability to override client settings during initialization
type ClientOptions func(*Client)

// WithS3APIClient allows providing the AWS S3 API client instead of initializing one
func WithS3APIClient(s3c s3iface.S3API) ClientOptions {
	return func(c *Client) {
		c.s3Client = s3c
	}
}

// WithRegion allows specifying the region we're operating in
func WithRegion(r string) ClientOptions {
	return func(c *Client) {
		c.region = r
	}
}

// WithRoleARN allows specifying a role ARN to assume for S3 operations
func WithRoleARN(r string) ClientOptions {
	return func(c *Client) {
		c.roleArn = r
	}
}

// New returns a new Client instance
func New(opts ...ClientOptions) (*Client, error) {
	c := &Client{}

	for _, opt := range opts {
		opt(c)
	}

	if c.s3Client == nil {
		cfg := &aws.Config{}
		sess := session.Must(session.NewSession())
		if c.region != "" {
			log.Infof("s3: overriding default region to %s", c.region)
			cfg.Region = aws.String(c.region)
		}
		if c.roleArn != "" {
			log.Infof("s3: using role arn: %s", c.roleArn)
			cfg.Credentials = stscreds.NewCredentials(sess, c.roleArn)
		}
		c.s3Client = s3api.New(sess, cfg)
	}

	return c, nil
}

// WriteBytes writes the given bytes to an object key
func (c *Client) WriteBytes(bucket string, key string, r io.ReadSeeker) error {
	req := &s3api.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		ACL:    aws.String("private"),
		Body:   r,
	}
	// we don't really need to check the response for anything... just need to know if it worked or not
	_, err := c.s3Client.PutObject(req)
	if err != nil {
		return err
	}
	return nil
}

// GetObjectLink creates a presigned url to the given object
//func (c *Client) GetObjectLink(bucket string, key string) (string, error) {
//	req, _ := c.s3Client.GetObjectRequest(&s3api.GetObjectInput{
//		Bucket: aws.String(bucket),
//		Key:    aws.String(key),
//	})
//	if req == nil {
//		return "", fmt.Errorf("request was nil")
//	}
//	return req.Presign(7 * time.Hour * 24)
//}
