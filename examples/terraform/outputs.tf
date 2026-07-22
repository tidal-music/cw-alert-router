output "function_name" {
  value = aws_lambda_function.router.function_name
}

output "queue_url" {
  value = aws_sqs_queue.alarms.url
}

output "dead_letter_queue_url" {
  value = aws_sqs_queue.dlq.url
}
