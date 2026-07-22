# Example deployment (Terraform)

A complete, self-contained deployment of the CloudWatch Alert Router:

- an **EventBridge rule** matching every `CloudWatch Alarm State Change` in the region
- an **SQS queue** (plus dead-letter queue) the rule delivers to
- the **Lambda** (`provided.al2023`, arm64) consuming the queue, with
  per-message failure reporting enabled
- the **IAM role** with the minimum permissions the router needs

Graphs are delivered in the default `slack` mode - uploaded directly to
Slack, so no S3 bucket or CDN is required.

## Deploy in 5 steps

**1. Create the Slack app** at <https://api.slack.com/apps>: add the bot
token scopes `chat:write`, `chat:write.public` and `files:write`, install it
to your workspace, and copy the bot token (`xoxb-...`).

**2. Store the token in Parameter Store** (done outside Terraform so the
secret never touches your state):

```sh
aws ssm put-parameter --name /service/cw_alert_router/slack/token \
  --type SecureString --value xoxb-your-token
```

**3. Build the lambda package** from the repository root:

```sh
task publish       # via docker
# or: task build-local
```

**4. Apply**:

```sh
cd examples/terraform
terraform init
terraform apply \
  -var default_slack_channel=my-alarms-channel \
  -var pagerduty_default_routing_key=YOUR_ROUTING_KEY
```

**5. Tag an alarm and test it.** Tag alarms with `owner` (Slack channel
becomes `<owner>-alarms`) and/or `service` (PagerDuty routing key lookup),
then force a state change:

```sh
aws cloudwatch tag-resource \
  --resource-arn arn:aws:cloudwatch:...:alarm:my-test-alarm \
  --tags Key=owner,Value=myteam

aws cloudwatch set-alarm-state --alarm-name my-test-alarm \
  --state-value ALARM --state-reason "cw-alert-router test"
```

Within a few seconds `#myteam-alarms` should get the triggered message with
a graph, and your PagerDuty service an incident. Set the state back to `OK`
to see the resolve flow.

## Notes

- Alarms are region-scoped: deploy one router per region you have alarms in.
- Per-service PagerDuty routing keys can be registered any time under
  `/service/cw_alert_router/pagerduty/routing_keys/<service_name>`
  (lowercased, hyphens as underscores) - no redeploy needed.
- To keep graphs in your own bucket instead of uploading to Slack, set
  `GRAPH_MODE=s3` plus the `IMAGE_BUCKET*` variables on the lambda (and give
  the role `s3:PutObject` on the bucket).
- The dead-letter queue holds events that repeatedly failed processing -
  alarm on its depth if you want to know when alerts are being dropped.
