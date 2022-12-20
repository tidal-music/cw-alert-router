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

package lambda

import (
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/tidal-open-source/cw-alert-router/cw"
	"github.com/tidal-open-source/cw-alert-router/pagerduty"
	"github.com/tidal-open-source/cw-alert-router/parameterstore"
	"github.com/tidal-open-source/cw-alert-router/s3"
	"github.com/tidal-open-source/cw-alert-router/slack"
)

const (
	// default configuration values:
	// DefaultlOwnerTagKey is the AWS tag key to look for the team owner of the alarm
	DefaultOwnerTagKey = "owner"
	// DefaultServiceNameTagKey is the AWS tag key to look for the service name the alert is from
	DefaultServiceNameTagKey = "service"
	// DefaultSlackChannelOverrideTagKey is the AWS tag which specifies the slack channel these alerts should be sent to
	DefaultSlackChannelOverrideTagKey = "alerts:slack_channel"
	//PagerDutySuppressAlarmsTagKey is the AWS tag which specifies whether we should suppress alarms in PagerDuty for this alarm
	// must contain "true" to enable - else it will be considered false (whether empty or not)
	// note: the logic to handle this is actually in PagerDuty event rules - since we can inspect the tags there
	DefaultPagerDutySuppressAlarmsTagKey = "alerts:suppress_pagerduty"
	// PagerDutyRoutingKeySSMPattern is the base ssm key name where other services put their routing key
	DefaultPagerDutyRoutingKeySSMPattern = "/service/cw_alert_router/pagerduty/routing_keys/%s"

	// environment variable keys:
	// SlackTokenSSMKeyEnv is the environment variable key for the slack token ssm key value
	SlackTokenSSMKeyEnv = "SLACK_TOKEN_SSM_KEY"
	// DefaultSlackChannelEnv is the environment variable key for the default slack channel
	DefaultSlackChannelEnv = "DEFAULT_SLACK_CHANNEL"
	// DefaultPagerDutyRoutingKeyEnv is the environment variable key for the pagerduty routing key
	DefaultPagerDutyRoutingKeyEnv = "PAGERDUTY_DEFAULT_ROUTING_KEY"
	// ImageBucketEnv is the environment variable key for the images bucket
	ImageBucketEnv = "IMAGE_BUCKET"
	// ImageBucketRegionEnv is the environment variable key for the images bucket region
	ImageBucketRegionEnv = "IMAGE_BUCKET_REGION"
	// ImageBucketRoleArnEnv is the env var key to find the role arn to use when writing to the images bucket
	ImageBucketRoleArnEnv = "IMAGE_BUCKET_ROLE_ARN"
	// ImageBucketPrefix is the env var key to find the s3 prefix used for storing images
	ImageBucketPrefixEnv = "IMAGE_BUCKET_PREFIX"
	// LogLevelEnv is the environment variable key for setting the global log level
	LogLevelEnv = "LOG_LEVEL"
	// ImageHostEnv is the env variable key for the images host (ie: where images can be fetched externally)
	ImageHostEnv = "IMAGE_HOST"
	// OwnerTagKeyEnv is used to override the default owner tag key
	OwnerTagKeyEnv = "OWNER_TAG_KEY"
	// ServiceNameTagKey is used to override the default service name tag key
	ServiceNameTagKeyEnv = "SERVICE_NAME_TAG_KEY"
)

// Config holds configuration options for the lambda
type Config struct {
	// DefaultSlackChannel is used when no owner tag is found - ie: we don't know who owns the alert.
	// This must be set via environment variables on the lambda
	DefaultSlackChannel string

	// ServiceNameTagKey is the AWS tag key for the name of the service (used to lookup SSM keys for the pagerduty routing key)
	ServiceNameTagKey string

	// OwnerTagKEy is the AWS tag key for the owner of the service (used to generate slack channel names)
	OwnerTagKey string

	// ImageBucket used for hosting our graph images in the slack messages
	ImageBucket string

	// ImageBucketRegion is used to specify the image bucket region when the region differs from the default
	ImageBucketRegion string

	// ImageBucketRoleArn is the role to assume when writing to the images bucket
	ImageBucketRoleArn string

	// ImageBucketPrefix is the s3 prefix to use for images
	ImageBucketPrefix string

	// ImageHost is used for actually accessing the images we write to s3.  eg:  https://cf-site.test.com
	ImageHost string

	// DefaultPagerDutyRoutingKey is for when an alert comes through and we cannot determine the pagerduty routing key,
	//  we use this.  The normal way to get this value is via parameter store - the default routing key should be
	// configured via env
	DefaultPagerDutyRoutingKey string

	// private as we need to use a setter so we can configure on change (see fetchEnv)
	logLevel string

	slackToken          string
	slackAlternativeURL string
	slackTokenSSMKey    string

	cw *cw.Client
	s3 *s3.Client
	ps *parameterstore.Client
	pd *pagerduty.Client
	s  *slack.Client
}

// ConfigOptions provides a way to override settings in the lambda Config struct
type ConfigOptions func(*Config)

// NewConfig returns a new lambda Config struct
func NewConfig(options ...ConfigOptions) (*Config, error) {
	var err error

	c := &Config{}

	for _, opt := range options {
		opt(c)
	}

	// FIXME: this is getting a bit out of hand... chain these together in a list instead
	err = c.initParameterStoreClientIfEmpty()
	if err != nil {
		return nil, err
	}

	err = c.fetchEnv()
	if err != nil {
		return nil, err
	}

	err = c.fetchSSM()
	if err != nil {
		return nil, err
	}

	err = c.initCWClientIfEmpty()
	if err != nil {
		return nil, err
	}

	err = c.initS3ClientIfEmpty()
	if err != nil {
		return nil, err
	}

	err = c.initPagerDutyClientIfEmpty()
	if err != nil {
		return nil, err
	}

	err = c.initSlackClientIfEmpty()
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Config) fetchEnv() error {
	if c.DefaultSlackChannel == "" {
		c.DefaultSlackChannel = os.Getenv(DefaultSlackChannelEnv)
		log.Infof("Fetching default slack channel from %s - got %s", DefaultSlackChannelEnv, c.DefaultSlackChannel)
	}
	if c.DefaultPagerDutyRoutingKey == "" {
		log.Infof("Fetching default pagerduty routing key from %s", DefaultPagerDutyRoutingKeyEnv)
		c.DefaultPagerDutyRoutingKey = os.Getenv(DefaultPagerDutyRoutingKeyEnv)
	}
	if c.slackTokenSSMKey == "" {
		c.slackTokenSSMKey = os.Getenv(SlackTokenSSMKeyEnv)
		log.Infof("Fetching slack token SSM Key from %s - got %s", SlackTokenSSMKeyEnv, c.slackTokenSSMKey)
	}

	if c.ImageBucket == "" {
		c.ImageBucket = os.Getenv(ImageBucketEnv)
		log.Infof("Fetching image bucket from %s - got %s", ImageBucketEnv, c.ImageBucket)
	}

	if c.ImageBucketRegion == "" {
		c.ImageBucketRegion = os.Getenv(ImageBucketRegionEnv)
		log.Infof("Fetching image bucket region from %s - got %s", ImageBucketRegionEnv, c.ImageBucketRegion)
	}

	if c.ImageBucketRoleArn == "" {
		c.ImageBucketRoleArn = os.Getenv(ImageBucketRoleArnEnv)
		log.Infof("Fetching image bucket role arn from %s - got %s", ImageBucketRoleArnEnv, c.ImageBucketRoleArn)
	}

	if c.ImageBucketPrefix == "" {
		c.ImageBucketPrefix = os.Getenv(ImageBucketPrefixEnv)
		log.Infof("Fetching image bucket prefix from %s - got %s", ImageBucketPrefixEnv, c.ImageBucketPrefix)
	}

	if c.ImageHost == "" {
		c.ImageHost = os.Getenv(ImageHostEnv)
		log.Infof("Fetching image host from %s - got %s", ImageHostEnv, c.ImageHost)
	}

	if c.logLevel == "" {
		log.Infof("Fetching log level from %s", LogLevelEnv)
		c.logLevel = os.Getenv(LogLevelEnv)
		c.SetLogLevel(c.logLevel)
	}

	// We only set the following from env vars.
	// TODO: this is all a bit inconsistent...  consolidate all these configuration options and standardize on how to read
	c.ServiceNameTagKey = os.Getenv(ServiceNameTagKeyEnv)
	if c.ServiceNameTagKey == "" {
		c.ServiceNameTagKey = DefaultServiceNameTagKey
	}

	c.OwnerTagKey = os.Getenv(OwnerTagKeyEnv)
	if c.OwnerTagKey == "" {
		c.OwnerTagKey = DefaultOwnerTagKey
	}
	return nil
}

func (c *Config) fetchSSM() error {
	var err error
	if c.slackTokenSSMKey == "" {
		return fmt.Errorf("slack token key is empty")
	}
	log.Printf("Fetching slack token from ssm key %s", c.slackTokenSSMKey)
	c.slackToken, err = c.ps.GetParameterValue(c.slackTokenSSMKey)

	return err
}

func (c *Config) initParameterStoreClientIfEmpty() error {
	var err error
	if c.ps == nil {
		log.Printf("Initializing default parameterstoreclient.")
		c.ps, err = parameterstore.New()
		if err != nil {
			return err
		}
	} else {
		log.Printf("Using provided parameterstore client")
	}
	return nil
}

func (c *Config) initSlackClientIfEmpty() error {
	var err error
	if c.s == nil {
		log.Printf("Initializing default slack client.")
		c.s, err = slack.New(c.slackToken, slack.WithAlternativeURL(c.slackAlternativeURL))
		if err != nil {
			return err
		}
	} else {
		log.Printf("Using the supplied slack client")
	}
	return nil
}

func (c *Config) initPagerDutyClientIfEmpty() error {
	var err error
	if c.pd == nil {
		log.Printf("Initializing default pagerduty client.")
		c.pd, err = pagerduty.New()
		if err != nil {
			return err
		}
	} else {
		log.Printf("Using provided pagerduty client.")
	}
	return nil
}

func (c *Config) initCWClientIfEmpty() error {
	var err error
	if c.cw == nil {
		log.Printf("Initializing default cloudwatch client.")
		c.cw, err = cw.New()
		if err != nil {
			return err
		}
	} else {
		log.Printf("Using provided cloudwatch client.")
	}
	return nil
}

func (c *Config) initS3ClientIfEmpty() error {
	var err error
	if c.s3 == nil {
		log.Printf("Initializing default s3 client.")
		c.s3, err = s3.New(s3.WithRegion(c.ImageBucketRegion), s3.WithRoleARN(c.ImageBucketRoleArn))
		if err != nil {
			return err
		}
	} else {
		log.Printf("Using provided s3 client.")
	}
	return nil
}

// SetLogLevel sets the global log level
func (c *Config) SetLogLevel(level string) {
	switch strings.ToLower(level) {
	case "error":
		log.SetLevel(log.ErrorLevel)
	case "warn":
		log.SetLevel(log.WarnLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	case "debug":
		log.SetLevel(log.DebugLevel)
	default:
		log.SetLevel(log.InfoLevel)
	}
}

// WithParameterStoreClient allows overriding of the parameterstore client
func WithParameterStoreClient(psclient *parameterstore.Client) ConfigOptions {
	return func(c *Config) {
		c.ps = psclient
	}
}

// WithPagerDutyClient allows overriding of the pagerduty client
func WithPagerDutyClient(pdclient *pagerduty.Client) ConfigOptions {
	return func(c *Config) {
		c.pd = pdclient
	}
}

// WithCWClient allows overriding of the Alarms client
func WithCWClient(cw *cw.Client) ConfigOptions {
	return func(c *Config) {
		c.cw = cw
	}
}

// WithS3Client allows overriding of the S3 client
func WithS3Client(s3c *s3.Client) ConfigOptions {
	return func(c *Config) {
		c.s3 = s3c
	}
}

// WithSlackClient allows overriding of the slack client
func WithSlackClient(sclient *slack.Client) ConfigOptions {
	return func(c *Config) {
		c.s = sclient
	}
}

// WithDefaultSlackChannel allows setting the default slack channel during New
func WithDefaultSlackChannel(channel string) ConfigOptions {
	return func(c *Config) {
		c.DefaultSlackChannel = channel
	}
}

// WithSlackToken allows setting the slack API token during New
func WithSlackToken(token string) ConfigOptions {
	return func(c *Config) {
		c.slackToken = token
	}
}

// WithSlackAlternativeURL allows setting the slack API url during New
func WithSlackAlternativeURL(url string) ConfigOptions {
	return func(c *Config) {
		c.slackAlternativeURL = url
	}
}

// WithDefaultPagerDutyRoutingKey allows setting the PagerDuty default routing key during New
func WithDefaultPagerDutyRoutingKey(key string) ConfigOptions {
	return func(c *Config) {
		c.DefaultPagerDutyRoutingKey = key
	}
}

// WithImageBucket allows setting the image bucket during New instead of fetching from env
func WithImageBucket(bucket string) ConfigOptions {
	return func(c *Config) {
		c.ImageBucket = bucket
	}
}

// WithImageBucketRegion allows setting the image bucket region during New instead of fetching from env
func WithImageBucketRegion(r string) ConfigOptions {
	return func(c *Config) {
		c.ImageBucketRegion = r
	}
}

// WithImageBucketRoleArn allows specifying the role arn to use for s3 writes instead of fetching from env
func WithImageBucketRoleArn(r string) ConfigOptions {
	return func(c *Config) {
		c.ImageBucketRoleArn = r
	}
}

// WithImageBucketPrefix sets the s3 bucket prefix used for writing images
func WithImageBucketPrefix(p string) ConfigOptions {
	return func(c *Config) {
		c.ImageBucketPrefix = p
	}
}

// WithImageHost allows setting the image host during New instead of fetching from env
func WithImageHost(host string) ConfigOptions {
	return func(c *Config) {
		c.ImageHost = host
	}
}

// WithLogLevel allows specifying log level during New
func WithLogLevel(level string) ConfigOptions {
	return func(c *Config) {
		c.SetLogLevel(level)
	}
}

// ParameterStoreClient just returns the internal parameter store client
func (c *Config) ParameterStoreClient() *parameterstore.Client {
	return c.ps
}

// CWClient returns the Cloudwatch client
func (c Config) CWClient() *cw.Client {
	return c.cw
}

// S3Client returns the S3 client
func (c Config) S3Client() *s3.Client {
	return c.s3
}

// PagerDutyClient returns the internal PagerDuty client
func (c Config) PagerDutyClient() *pagerduty.Client {
	return c.pd
}

// SlackClient returns the internal Slack client
func (c Config) SlackClient() *slack.Client {
	return c.s
}
