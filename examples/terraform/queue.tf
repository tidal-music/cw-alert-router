# EventBridge rule: every CloudWatch alarm state change in this account and
# region lands on the SQS queue.
resource "aws_cloudwatch_event_rule" "alarm_state_change" {
  name        = "${var.name}-alarm-state-change"
  description = "Route CloudWatch alarm state changes to the alert router"

  event_pattern = jsonencode({
    source      = ["aws.cloudwatch"]
    detail-type = ["CloudWatch Alarm State Change"]
  })
}

resource "aws_cloudwatch_event_target" "to_sqs" {
  rule = aws_cloudwatch_event_rule.alarm_state_change.name
  arn  = aws_sqs_queue.alarms.arn
}

# Undeliverable / repeatedly failing events end up here instead of looping
# forever (the handler reports per-message batch failures).
resource "aws_sqs_queue" "dlq" {
  name                      = "${var.name}-dlq"
  message_retention_seconds = 1209600 # 14 days
}

resource "aws_sqs_queue" "alarms" {
  name = var.name

  # should be at least 6x the lambda timeout
  visibility_timeout_seconds = 180

  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.dlq.arn
    maxReceiveCount     = 3
  })
}

data "aws_iam_policy_document" "queue_policy" {
  statement {
    sid       = "AllowEventBridge"
    effect    = "Allow"
    actions   = ["sqs:SendMessage"]
    resources = [aws_sqs_queue.alarms.arn]

    principals {
      type        = "Service"
      identifiers = ["events.amazonaws.com"]
    }

    condition {
      test     = "ArnEquals"
      variable = "aws:SourceArn"
      values   = [aws_cloudwatch_event_rule.alarm_state_change.arn]
    }
  }
}

resource "aws_sqs_queue_policy" "alarms" {
  queue_url = aws_sqs_queue.alarms.id
  policy    = data.aws_iam_policy_document.queue_policy.json
}
