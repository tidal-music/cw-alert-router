# CloudWatch Alert Router

![build&test](https://github.com/tidal-music/cw-alert-router/actions/workflows/main.yml/badge.svg)

CloudWatch Alert Router is a small AWS Lambda that takes CloudWatch alarm
state changes (via EventBridge and SQS) and delivers them to **Slack** and/or
**PagerDuty** - with a rendered metric graph attached.

Why? Waiting for a third-party monitoring platform to crawl CloudWatch
metrics can add 10-15+ minutes before anyone gets paged. Routing the alarm
state change event directly gets the alert out within seconds.

```
CloudWatch Alarm ──> EventBridge rule ──> SQS queue ──> Lambda ──┬──> Slack (+ graph)
                                                                 └──> PagerDuty
```

Routing is controlled by **AWS tags on the alarm** - no configuration files
to maintain as teams add alarms.

## How routing works

| Tag on the alarm | Effect |
|:--|:--|
| `owner` | Slack messages go to `<owner>-alarms` (lowercased) |
| `service` | PagerDuty routing key is looked up in Parameter Store at `/service/cw_alert_router/pagerduty/routing_keys/<service>` (lowercased, `-` → `_`) |
| `alerts:slack_channel` | Overrides the Slack channel entirely |
| `alerts:suppress_pagerduty` | `"true"` = skip PagerDuty for this alarm (Slack still gets the message) |

If no tag matches, the default Slack channel and default PagerDuty routing
key (from the environment) are used. Only `OK -> ALARM` (trigger) and
`ALARM -> OK` (resolve) transitions are routed; everything else is ignored.

## Graphs

Graphs are rendered server-side by CloudWatch
([`GetMetricWidgetImage`](https://docs.aws.amazon.com/AmazonCloudWatch/latest/APIReference/API_GetMetricWidgetImage.html))
from the metric queries in the alarm event, with the alarm's threshold drawn
as a horizontal annotation - metric math and multi-metric alarms work out of
the box. (Composite alarms carry no metrics, so they get no graph.) Delivery
is controlled by `GRAPH_MODE`:

| Mode | Behaviour |
|:--|:--|
| `slack` (default) | The PNG is uploaded to Slack and embedded in the message. No bucket or CDN needed. |
| `s3` | The PNG is written to `IMAGE_BUCKET` and linked via `IMAGE_HOST` (e.g. a CloudFront distribution), or a presigned URL if `IMAGE_HOST` is unset. |
| `none` | No graph. |

If `GRAPH_MODE` is unset but `IMAGE_BUCKET` is configured, `s3` is assumed
(backwards compatible with v1 deployments).

## Configuration

Required environment variables:

| Environment variable | Description | Example |
|:--|:--|:--|
| `DEFAULT_SLACK_CHANNEL` | Fallback channel when no owner can be inferred | `test-alarms` |
| `SLACK_TOKEN_SSM_KEY` | Parameter Store key holding the Slack bot token | `/service/cw_alert_router/slack/token` |
| `PAGERDUTY_DEFAULT_ROUTING_KEY` | Fallback PagerDuty Events API v2 routing key | `xxxxxxx` |

Optional environment variables:

| Environment variable | Description | Default |
|:--|:--|:--|
| `GRAPH_MODE` | Graph delivery: `slack`, `s3` or `none` | `slack` |
| `OWNER_TAG_KEY` | Tag key used to derive the Slack channel | `owner` |
| `SERVICE_NAME_TAG_KEY` | Tag key used to look up the PagerDuty routing key | `service` |
| `LOG_LEVEL` | `debug`, `info`, `warn` or `error` | `info` |
| `IMAGE_BUCKET` | Bucket for graph images (`s3` mode only) | |
| `IMAGE_BUCKET_REGION` | Region of the image bucket | lambda's region |
| `IMAGE_BUCKET_ROLE_ARN` | Role to assume for bucket writes (empty = lambda role) | |
| `IMAGE_BUCKET_PREFIX` | Key prefix for graph images | |
| `IMAGE_HOST` | Public host serving the bucket (e.g. CloudFront); presigned URLs if empty | |

## Setting up the API keys

**Slack**: create a [Slack app](https://api.slack.com/apps), add the bot
token scopes `chat:write`, `chat:write.public` and `files:write`, install it
to your workspace, and store the bot token (`xoxb-...`) in Parameter Store as
a SecureString:

```sh
aws ssm put-parameter --name /service/cw_alert_router/slack/token \
  --type SecureString --value xoxb-your-token
```

Private channels need the bot invited (`/invite @your-app`).

**PagerDuty**: on each PagerDuty service, add an *Events API v2* integration
and copy its routing key. Use one as the default
(`PAGERDUTY_DEFAULT_ROUTING_KEY`) and optionally register per-service keys:

```sh
aws ssm put-parameter --name /service/cw_alert_router/pagerduty/routing_keys/my_service \
  --type SecureString --value your-routing-key
```

## Deploying

A complete, copy-pasteable Terraform deployment (EventBridge rule, SQS queue
with dead-letter queue, Lambda, IAM) lives in
[`examples/terraform`](examples/terraform/) - see its README for a 5-step
walkthrough.

The short version:

```sh
task publish          # builds function.zip (arm64 bootstrap, provided.al2023) via docker
# or without docker:
task build-local
```

Deploy `function.zip` on the `provided.al2023` runtime (arm64), wire an
EventBridge rule for `CloudWatch Alarm State Change` events into an SQS
queue consumed by the Lambda, and set the environment variables above. The
SQS event source mapping should enable `ReportBatchItemFailures` - records
are processed in order and successfully processed records aren't re-delivered
(and re-alerted) when a later record in the batch fails. On a failure the
handler stops and reports the failed record plus the rest of the batch, so
FIFO queue ordering is preserved on redelivery.

The Lambda role needs: `cloudwatch:ListTagsForResource`,
`cloudwatch:GetMetricWidgetImage`, `ssm:GetParameter` on the keys above, and
the usual SQS consume + CloudWatch Logs permissions (plus `s3:PutObject` on
the image bucket in `s3` graph mode).

## Using as a library

The lambda is also usable as a library if you want to customize the entrypoint:

```go
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/tidal-music/cw-alert-router/v2/lambda"
)

func main() {
	h, err := lambda.New(context.Background(), lambda.ConfigFromEnv())
	if err != nil {
		slog.Error("failed initializing lambda", "error", err)
		os.Exit(1)
	}
	h.Start()
}
```

Every dependency (CloudWatch, Parameter Store, PagerDuty, Slack, S3) can be
overridden via `lambda.With*` options - see `lambda/handler_test.go` for
examples.

## Development

```sh
go test ./...   # or: task test
task build      # docker image + function.zip
```
