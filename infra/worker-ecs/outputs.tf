output "cluster_name" {
  description = "Name of the ECS cluster hosting the worker service."
  value       = aws_ecs_cluster.worker.name
}

output "service_name" {
  description = "Name of the ECS worker service. Consumed by the autoscaler module's scalable target."
  value       = aws_ecs_service.worker.name
}
