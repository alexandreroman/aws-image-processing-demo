output "cluster_name" {
  description = "Name of the ECS cluster hosting the worker service."
  value       = aws_ecs_cluster.worker.name
}

output "cluster_id" {
  description = "ARN/id of the ECS cluster hosting the worker service. Consumed by the otel-collector module."
  value       = aws_ecs_cluster.worker.id
}

output "service_name" {
  description = "Name of the ECS worker service."
  value       = aws_ecs_service.worker.name
}
