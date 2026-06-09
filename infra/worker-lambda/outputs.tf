output "function_arn" {
  description = "Version-qualified ARN of the worker Lambda function."
  # Qualified ARN pins each Temporal Worker Deployment version to an
  # immutable Lambda version, avoiding $LATEST races during rollouts.
  value = aws_lambda_function.worker.qualified_arn
}

output "invoker_role_arn" {
  description = "ARN of the role Temporal Cloud assumes to invoke the worker. Null when the role is not created."
  value       = length(aws_iam_role.worker_invoker) > 0 ? aws_iam_role.worker_invoker[0].arn : null
}

output "deployment_name" {
  description = "Temporal Worker Deployment name the Lambda worker registers under. Consumed by scripts/register-worker-deployment.sh."
  value       = local.deployment_name
}
