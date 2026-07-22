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

// Package s3 wraps the AWS S3 API in a basic interface that provides calls specific to our needs
package s3

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	s3api "github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// API is the subset of the S3 API this service uses.
type API interface {
	PutObject(ctx context.Context, params *s3api.PutObjectInput, optFns ...func(*s3api.Options)) (*s3api.PutObjectOutput, error)
}

// Presigner generates presigned GET URLs for S3 objects.
type Presigner interface {
	PresignGetObject(ctx context.Context, params *s3api.GetObjectInput, optFns ...func(*s3api.PresignOptions)) (*v4.PresignedHTTPRequest, error)
}

// Client is our own S3 client.
type Client struct {
	api       API
	presigner Presigner
	region    string
	roleArn   string
}

// ClientOptions provides the ability to override client settings during initialization.
type ClientOptions func(*Client)

// WithAPI allows providing the S3 API client instead of initializing one (for testing).
func WithAPI(api API) ClientOptions {
	return func(c *Client) {
		c.api = api
	}
}

// WithPresigner allows providing the presigner instead of initializing one (for testing).
func WithPresigner(p Presigner) ClientOptions {
	return func(c *Client) {
		c.presigner = p
	}
}

// WithRegion allows specifying the region of the bucket we write to.
func WithRegion(r string) ClientOptions {
	return func(c *Client) {
		c.region = r
	}
}

// WithRoleARN allows specifying a role ARN to assume for S3 operations.
func WithRoleARN(r string) ClientOptions {
	return func(c *Client) {
		c.roleArn = r
	}
}

// New returns a new Client instance.
func New(ctx context.Context, opts ...ClientOptions) (*Client, error) {
	c := &Client{}
	for _, opt := range opts {
		opt(c)
	}

	if c.api == nil {
		var loadOpts []func(*config.LoadOptions) error
		if c.region != "" {
			loadOpts = append(loadOpts, config.WithRegion(c.region))
		}
		cfg, err := config.LoadDefaultConfig(ctx, loadOpts...)
		if err != nil {
			return nil, fmt.Errorf("loading aws config for s3: %w", err)
		}
		if c.roleArn != "" {
			provider := stscreds.NewAssumeRoleProvider(sts.NewFromConfig(cfg), c.roleArn)
			cfg.Credentials = aws.NewCredentialsCache(provider)
		}
		client := s3api.NewFromConfig(cfg)
		c.api = client
		if c.presigner == nil {
			c.presigner = s3api.NewPresignClient(client)
		}
	}

	return c, nil
}

// WriteBytes writes the given bytes to an object key.
func (c *Client) WriteBytes(ctx context.Context, bucket string, key string, r io.Reader) error {
	_, err := c.api.PutObject(ctx, &s3api.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		ACL:    s3types.ObjectCannedACLPrivate,
		Body:   r,
	})
	if err != nil {
		return fmt.Errorf("writing s3://%s/%s: %w", bucket, key, err)
	}
	return nil
}

// PresignedURL returns a presigned GET URL for the given object.
func (c *Client) PresignedURL(ctx context.Context, bucket string, key string, ttl time.Duration) (string, error) {
	if c.presigner == nil {
		return "", fmt.Errorf("no presigner configured")
	}
	req, err := c.presigner.PresignGetObject(ctx, &s3api.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, s3api.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("presigning s3://%s/%s: %w", bucket, key, err)
	}
	return req.URL, nil
}
