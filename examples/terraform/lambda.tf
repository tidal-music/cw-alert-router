data "aws_iam_policy_document" "assume_role" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }
}

data "aws_iam_policy_document" "lambda_permissions" {
  statement {
    sid    = "ConsumeQueue"
    effect = "Allow"
    actions = [
      "sqs:ReceiveMessage",
      "sqs:DeleteMessage",
      "sqs:GetQueueAttributes",
    ]
    resources = [aws_sqs_queue.alarms.arn]
  }

  statement {
    sid    = "ReadAlarms"
    effect = "Allow"
    actions = [
      "cloudwatch:ListTagsForResource",
      "cloudwatch:GetMetricWidgetImage",
      # GetMetricWidgetImage renders on our behalf, which reads metric data
      "cloudwatch:GetMetricData",
    ]
    resources = ["*"]
  }

  statement {
    sid    = "ReadParameters"
    effect = "Allow"
    actions = [
      "ssm:GetParameter",
    ]
    resources = [
      "arn:aws:ssm:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:parameter${var.slack_token_ssm_key}",
      "arn:aws:ssm:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:parameter/service/cw_alert_router/pagerduty/routing_keys/*",
    ]
  }
}

resource "aws_iam_role" "lambda" {
  name               = var.name
  assume_role_policy = data.aws_iam_policy_document.assume_role.json
}

resource "aws_iam_role_policy" "lambda" {
  name   = var.name
  role   = aws_iam_role.lambda.id
  policy = data.aws_iam_policy_document.lambda_permissions.json
}

resource "aws_iam_role_policy_attachment" "basic_execution" {
  role       = aws_iam_role.lambda.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

resource "aws_lambda_function" "router" {
  function_name = var.name
  role          = aws_iam_role.lambda.arn

  filename         = var.function_zip
  source_code_hash = filebase64sha256(var.function_zip)

  runtime       = "provided.al2023"
  architectures = ["arm64"]
  handler       = "bootstrap"
  timeout       = 30
  memory_size   = 128

  environment {
    variables = {
      DEFAULT_SLACK_CHANNEL         = var.default_slack_channel
      SLACK_TOKEN_SSM_KEY           = var.slack_token_ssm_key
      PAGERDUTY_DEFAULT_ROUTING_KEY = var.pagerduty_default_routing_key
      GRAPH_MODE                    = "slack"
      LOG_LEVEL                     = var.log_level
    }
  }
}

resource "aws_lambda_event_source_mapping" "sqs" {
  event_source_arn = aws_sqs_queue.alarms.arn
  function_name    = aws_lambda_function.router.arn
  batch_size       = 10

  # the handler reports failures per message; without this a single bad
  # message would re-deliver (and re-alert) the whole batch
  function_response_types = ["ReportBatchItemFailures"]
}
