# Cloudwatch Alert Router

Cloudwatch Alert Router is a simple lambda that will take cloudwatch alarms (via 
event bridge) and deliver them to Slack and/or PagerDuty depending.

Slack channels and PagerDuty delivery can be configured via AWS tags on the
Cloudwatch alarms.


# API keys


# Minimum required environment variables

These are required to be set, otherwise the application may not run correctly.

|Environment variable|Description|Example|
|:--------------------|:---------|:------|
|`DEFAULT_SLACK_CHANNEL`|Default slack channel where alarms are sent if no channel can be inferred|`test-alarms`|
|`SLACK_TOKEN_SSM_KEY`|The SSM Parameter name where your Slack Oauth access token is stored|`/lambda/my-lambda-name/slack/app/oauth/access_token`|
|`PAGERDUTY_DEFAULT_ROUTING_KEY`|Default PagerDuty routing key (otherwise some logic is used to search for a key in SSM)|`xxxxxxx`|
|`IMAGE_BUCKET`|Name of the S3 bucket where temporary graphs are stored|`my-bucket-name`|
|`IMAGE_BUCKET_REGION`|Region of the bucket|`us-west-2`|
|`IMAGE_HOST`||`https://my-bucket-cdn.mysite.com`|

# Optional environment variables

The following environment variables are optional

| Environment variable | Description | Default | Example |
|:----------------------|:------------|:--------|:--------|
|`IMAGE_BUCKET_ROLE_ARN`|Role we need to assume to write to the image bucket (leave empty to use the lambda role)|`""`|`arn:aws:iam::123456789012:role/role_with_bucket_access`|
|`LOG_LEVEL`|Log level|`INFO`|`DEBUG`|
|`OWNER_TAG_KEY`|AWS tag to determine who owns the alarm - used to infer the slack channel name (<team>-alarms)|`owner`||
|`APP_NAME_TAG_KEY`|AWS tag for the service that owns the alarm - used to infer a pagerduty routing key from ssm (eg: /service/cw_alert_router/pagerduty/routing_keys/<app-name>)|`service`||


# Getting started

A simple example of how to get the lambda up and running:

```go
package main

import (
	log "github.com/sirupsen/logrus"
	"github.com/tidal-open-source/cw-alert-router/lambda"
)

func main() {
	cfg, err := lambda.NewConfig()

	if err != nil {
		log.Fatalf("Error initializing lambda: %v", err)
	}

	lambda.SetConfig(cfg)
	lambda.Start()
}
```

Compile and upload the lambda - then set the required (and/or optional) environment variables to configure the service to your needs.

