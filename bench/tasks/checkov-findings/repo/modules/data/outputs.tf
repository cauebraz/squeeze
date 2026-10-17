output "db_endpoint" {
  value = aws_db_instance.orders.endpoint
}

output "carts_table" {
  value = aws_dynamodb_table.carts.name
}

output "sessions_endpoint" {
  value = aws_elasticache_replication_group.sessions.primary_endpoint_address
}
