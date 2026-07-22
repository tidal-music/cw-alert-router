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
	"log/slog"
	"os"
	"strings"
)

// Graph delivery modes.
const (
	// GraphModeSlack renders the alarm graph and uploads it to Slack directly (default).
	GraphModeSlack = "slack"
	// GraphModeS3 renders the alarm graph and stores it in an S3 bucket,
	// linking either via IMAGE_HOST or a presigned URL.
	GraphModeS3 = "s3"
	// GraphModeNone disables graph rendering.
	GraphModeNone = "none"
)

// Default configuration values.
const (
	// DefaultOwnerTagKey is the AWS tag key to look for the team owner of the alarm.
	DefaultOwnerTagKey = "owner"
	// DefaultServiceNameTagKey is the AWS tag key to look for the service name the alert is from.
	DefaultServiceNameTagKey = "service"
	// SlackChannelOverrideTagKey is the AWS tag which specifies the slack channel these alerts should be sent to.
	SlackChannelOverrideTagKey = "alerts:slack_channel"
	// SuppressPagerDutyTagKey is the AWS tag which, when set to "true", stops
	// the alarm from being sent to PagerDuty (Slack messages are still sent).
	SuppressPagerDutyTagKey = "alerts:suppress_pagerduty"
	// DefaultPagerDutyRoutingKeySSMPattern is the parameter-store key pattern
	// where services can register their own PagerDuty routing key.
	DefaultPagerDutyRoutingKeySSMPattern = "/service/cw_alert_router/pagerduty/routing_keys/%s"
)

// Environment variable keys.
const (
	// SlackTokenSSMKeyEnv is the environment variable key for the slack token ssm key value.
	SlackTokenSSMKeyEnv = "SLACK_TOKEN_SSM_KEY"
	// DefaultSlackChannelEnv is the environment variable key for the default slack channel.
	DefaultSlackChannelEnv = "DEFAULT_SLACK_CHANNEL"
	// DefaultPagerDutyRoutingKeyEnv is the environment variable key for the pagerduty routing key.
	DefaultPagerDutyRoutingKeyEnv = "PAGERDUTY_DEFAULT_ROUTING_KEY"
	// GraphModeEnv selects how alarm graphs are delivered: slack (default), s3 or none.
	GraphModeEnv = "GRAPH_MODE"
	// ImageBucketEnv is the environment variable key for the images bucket (s3 graph mode).
	ImageBucketEnv = "IMAGE_BUCKET"
	// ImageBucketRegionEnv is the environment variable key for the images bucket region.
	ImageBucketRegionEnv = "IMAGE_BUCKET_REGION"
	// ImageBucketRoleArnEnv is the env var key to find the role arn to use when writing to the images bucket.
	ImageBucketRoleArnEnv = "IMAGE_BUCKET_ROLE_ARN"
	// ImageBucketPrefixEnv is the env var key to find the s3 prefix used for storing images.
	ImageBucketPrefixEnv = "IMAGE_BUCKET_PREFIX"
	// ImageHostEnv is the env variable key for the host images are served from
	// (e.g. a CloudFront distribution in front of the bucket). If empty in s3
	// graph mode, presigned URLs are used instead.
	ImageHostEnv = "IMAGE_HOST"
	// LogLevelEnv is the environment variable key for setting the global log level.
	LogLevelEnv = "LOG_LEVEL"
	// OwnerTagKeyEnv is used to override the default owner tag key.
	OwnerTagKeyEnv = "OWNER_TAG_KEY"
	// ServiceNameTagKeyEnv is used to override the default service name tag key.
	ServiceNameTagKeyEnv = "SERVICE_NAME_TAG_KEY"
)

// Config holds configuration options for the lambda.
type Config struct {
	// DefaultSlackChannel is used when no owner tag is found - ie: we don't know who owns the alert.
	DefaultSlackChannel string

	// DefaultPagerDutyRoutingKey is used when no service-specific routing key
	// is registered in parameter store.
	DefaultPagerDutyRoutingKey string

	// SlackTokenSSMKey is the parameter-store key holding the Slack bot token.
	SlackTokenSSMKey string

	// OwnerTagKey is the AWS tag key for the owner of the service (used to generate slack channel names).
	OwnerTagKey string

	// ServiceNameTagKey is the AWS tag key for the name of the service (used
	// to look up the PagerDuty routing key in parameter store).
	ServiceNameTagKey string

	// PagerDutyRoutingKeySSMPattern is the parameter-store key pattern for
	// service-specific PagerDuty routing keys (must contain one %s).
	PagerDutyRoutingKeySSMPattern string

	// GraphMode selects how alarm graphs are delivered: GraphModeSlack,
	// GraphModeS3 or GraphModeNone.
	GraphMode string

	// ImageBucket used for hosting our graph images (GraphModeS3 only).
	ImageBucket string

	// ImageBucketRegion is used to specify the image bucket region when the region differs from the default.
	ImageBucketRegion string

	// ImageBucketRoleArn is the role to assume when writing to the images bucket (empty = lambda role).
	ImageBucketRoleArn string

	// ImageBucketPrefix is the s3 prefix to use for images.
	ImageBucketPrefix string

	// ImageHost is the public host serving the image bucket (e.g. a
	// CloudFront URL). If empty, presigned S3 URLs are used.
	ImageHost string

	// LogLevel is the slog level name (debug, info, warn, error).
	LogLevel string
}

// ConfigFromEnv builds a Config from the environment variables documented in the README.
func ConfigFromEnv() Config {
	cfg := Config{
		DefaultSlackChannel:        os.Getenv(DefaultSlackChannelEnv),
		DefaultPagerDutyRoutingKey: os.Getenv(DefaultPagerDutyRoutingKeyEnv),
		SlackTokenSSMKey:           os.Getenv(SlackTokenSSMKeyEnv),
		OwnerTagKey:                os.Getenv(OwnerTagKeyEnv),
		ServiceNameTagKey:          os.Getenv(ServiceNameTagKeyEnv),
		GraphMode:                  os.Getenv(GraphModeEnv),
		ImageBucket:                os.Getenv(ImageBucketEnv),
		ImageBucketRegion:          os.Getenv(ImageBucketRegionEnv),
		ImageBucketRoleArn:         os.Getenv(ImageBucketRoleArnEnv),
		ImageBucketPrefix:          os.Getenv(ImageBucketPrefixEnv),
		ImageHost:                  os.Getenv(ImageHostEnv),
		LogLevel:                   os.Getenv(LogLevelEnv),
	}
	return cfg.withDefaults()
}

// withDefaults fills in defaults for unset fields.
func (c Config) withDefaults() Config {
	if c.OwnerTagKey == "" {
		c.OwnerTagKey = DefaultOwnerTagKey
	}
	if c.ServiceNameTagKey == "" {
		c.ServiceNameTagKey = DefaultServiceNameTagKey
	}
	if c.PagerDutyRoutingKeySSMPattern == "" {
		c.PagerDutyRoutingKeySSMPattern = DefaultPagerDutyRoutingKeySSMPattern
	}
	if c.GraphMode == "" {
		// backwards compatible default: deployments configured with an image
		// bucket keep using it; everything else uploads straight to Slack
		if c.ImageBucket != "" {
			c.GraphMode = GraphModeS3
		} else {
			c.GraphMode = GraphModeSlack
		}
	}
	return c
}

// validate checks required fields and value ranges.
func (c Config) validate() error {
	if c.DefaultSlackChannel == "" {
		return fmt.Errorf("default slack channel is required (%s)", DefaultSlackChannelEnv)
	}
	if c.DefaultPagerDutyRoutingKey == "" {
		return fmt.Errorf("default pagerduty routing key is required (%s)", DefaultPagerDutyRoutingKeyEnv)
	}
	switch c.GraphMode {
	case GraphModeSlack, GraphModeNone:
	case GraphModeS3:
		if c.ImageBucket == "" {
			return fmt.Errorf("image bucket is required in s3 graph mode (%s)", ImageBucketEnv)
		}
	default:
		return fmt.Errorf("invalid graph mode %q (%s must be %s, %s or %s)",
			c.GraphMode, GraphModeEnv, GraphModeSlack, GraphModeS3, GraphModeNone)
	}
	return nil
}

// slogLevel converts the configured log level name to a slog.Level.
func (c Config) slogLevel() slog.Level {
	switch strings.ToLower(c.LogLevel) {
	case "error":
		return slog.LevelError
	case "warn":
		return slog.LevelWarn
	case "debug":
		return slog.LevelDebug
	default:
		return slog.LevelInfo
	}
}
