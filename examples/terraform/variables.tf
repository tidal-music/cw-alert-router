variable "region" {
  description = "AWS region to deploy into (alarms are region-scoped; deploy one router per region you alarm in)"
  type        = string
  default     = "us-east-1"
}

variable "name" {
  description = "Base name for all created resources"
  type        = string
  default     = "cw-alert-router"
}

variable "default_slack_channel" {
  description = "Slack channel used when no owner can be inferred from the alarm tags"
  type        = string
}

variable "pagerduty_default_routing_key" {
  description = "Fallback PagerDuty Events API v2 routing key"
  type        = string
  sensitive   = true
}

variable "slack_token_ssm_key" {
  description = "Parameter Store key holding the Slack bot token (create it before applying - see the README)"
  type        = string
  default     = "/service/cw_alert_router/slack/token"
}

variable "log_level" {
  description = "Log level: debug, info, warn or error"
  type        = string
  default     = "info"
}

variable "function_zip" {
  description = "Path to the built lambda package (task publish / task build-local)"
  type        = string
  default     = "../../function.zip"
}
