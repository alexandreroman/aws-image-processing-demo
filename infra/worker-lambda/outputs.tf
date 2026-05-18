output "function_arn" {
  description = "ARN of the worker Lambda function."
  value       = aws_lambda_function.worker.arn
}

output "function_name" {
  description = "Name of the worker Lambda function."
  value       = aws_lambda_function.worker.function_name
}

output "function_invoke_arn" {
  description = "Invocation ARN of the worker Lambda function (use for API Gateway / EventBridge wiring)."
  value       = aws_lambda_function.worker.invoke_arn
}

output "invoker_role_arn" {
  description = "ARN of the role Temporal Cloud assumes to invoke the worker. Null when the role is not created."
  value       = length(aws_iam_role.worker_invoker) > 0 ? aws_iam_role.worker_invoker[0].arn : null
}
