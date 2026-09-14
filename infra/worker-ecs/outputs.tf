output "cluster_id" {
  description = "ARN/id of the ECS cluster hosting the worker service. Consumed by the otel-collector module."
  value       = aws_ecs_cluster.worker.id
}
