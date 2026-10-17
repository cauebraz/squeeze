resource "aws_db_subnet_group" "orders" {
  name       = "${var.name}-orders"
  subnet_ids = var.subnet_ids
}

resource "aws_db_instance" "orders" {
  identifier                 = "${var.name}-orders"
  engine                     = "postgres"
  engine_version             = "16.4"
  instance_class             = "db.t4g.medium"
  allocated_storage          = 100
  max_allocated_storage      = 500
  storage_type               = "gp3"
  db_name                    = "orders"
  username                   = "orders_admin"
  password                   = var.db_password
  db_subnet_group_name       = aws_db_subnet_group.orders.name
  vpc_security_group_ids     = [var.db_security_group]
  publicly_accessible        = true
  backup_retention_period    = 7
  auto_minor_version_upgrade = true
  ca_cert_identifier         = "rds-ca-rsa2048-g1"
  copy_tags_to_snapshot      = true
  skip_final_snapshot        = false
  final_snapshot_identifier  = "${var.name}-orders-final"
}

resource "aws_dynamodb_table" "carts" {
  name         = "${var.name}-carts"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "CartId"
  range_key    = "ItemId"

  attribute {
    name = "CartId"
    type = "S"
  }

  attribute {
    name = "ItemId"
    type = "S"
  }

  ttl {
    attribute_name = "ExpiresAt"
    enabled        = true
  }
}

resource "aws_elasticache_subnet_group" "sessions" {
  name       = "${var.name}-sessions"
  subnet_ids = var.subnet_ids
}

resource "aws_elasticache_replication_group" "sessions" {
  replication_group_id       = "${var.name}-sessions"
  description                = "user sessions"
  engine                     = "redis"
  engine_version             = "7.1"
  node_type                  = "cache.t4g.small"
  num_cache_clusters         = 2
  port                       = 6379
  automatic_failover_enabled = true
  subnet_group_name          = aws_elasticache_subnet_group.sessions.name
  security_group_ids         = [var.cache_security_group]
  at_rest_encryption_enabled = true
  snapshot_retention_limit   = 3
}
