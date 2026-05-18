output "cluster_name" {
  description = "Name of the ECS cluster running the worker service."
  value       = aws_ecs_cluster.worker.name
}

output "service_name" {
  description = "Name of the ECS service running the worker tasks."
  value       = aws_ecs_service.worker.name
}
