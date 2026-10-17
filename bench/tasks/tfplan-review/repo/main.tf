locals {
  tags = {
    Environment = var.environment
    ManagedBy   = "terraform"
    Team        = "platform"
  }
}

resource "aws_vpc" "main" {
  cidr_block           = "10.40.0.0/16"
  enable_dns_hostnames = true
  tags                 = merge(local.tags, { Name = "acme-prod" })
}

resource "aws_subnet" "private" {
  for_each          = { a = "10.40.1.0/24", b = "10.40.2.0/24" }
  vpc_id            = aws_vpc.main.id
  cidr_block        = each.value
  availability_zone = "${var.region}${each.key}"
  tags              = merge(local.tags, { Name = "acme-prod-private-${each.key}" })
}

resource "aws_security_group" "db" {
  name        = "acme-prod-db"
  description = "PostgreSQL from the private subnets"
  vpc_id      = aws_vpc.main.id

  ingress {
    description = "postgres from private subnets"
    from_port   = 5432
    to_port     = 5432
    protocol    = "tcp"
    cidr_blocks = ["10.40.1.0/24", "10.40.2.0/24"]
  }

  tags = local.tags
}

resource "aws_db_subnet_group" "main" {
  name       = "acme-prod"
  subnet_ids = [for s in aws_subnet.private : s.id]
  tags       = local.tags
}

resource "aws_db_instance" "main" {
  identifier              = "acme-production"
  engine                  = "postgres"
  engine_version          = "16.4"
  instance_class          = "db.t3.large"
  allocated_storage       = 100
  storage_encrypted       = true
  username                = "acme"
  password                = var.db_password
  db_subnet_group_name    = aws_db_subnet_group.main.name
  vpc_security_group_ids  = [aws_security_group.db.id]
  backup_retention_period = 7
  skip_final_snapshot     = false
  tags                    = local.tags
}

resource "aws_s3_bucket" "assets" {
  bucket = "acme-prod-assets"
  tags   = local.tags
}

resource "aws_s3_bucket_versioning" "assets" {
  bucket = aws_s3_bucket.assets.id

  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_iam_role" "app" {
  name = "acme-prod-app"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Action    = "sts:AssumeRole"
      Principal = { Service = "ec2.amazonaws.com" }
    }]
  })
  tags = local.tags
}

resource "aws_iam_instance_profile" "app" {
  name = "acme-prod-app"
  role = aws_iam_role.app.name
}

resource "aws_instance" "web" {
  ami                  = "ami-0c55b159cbfafe1f0"
  instance_type        = "t3.medium"
  subnet_id            = aws_subnet.private["a"].id
  iam_instance_profile = aws_iam_instance_profile.app.name
  tags                 = merge(local.tags, { Name = "acme-prod-web", Role = "web" })
}

resource "aws_instance" "worker" {
  count                = var.worker_count
  ami                  = "ami-0c55b159cbfafe1f0"
  instance_type        = "t3.small"
  subnet_id            = aws_subnet.private["b"].id
  iam_instance_profile = aws_iam_instance_profile.app.name
  tags                 = merge(local.tags, { Name = "acme-prod-worker-${count.index}" })
}

resource "aws_cloudwatch_log_group" "app" {
  name              = "/acme/prod/app"
  retention_in_days = 30
  tags              = local.tags
}

resource "aws_sqs_queue" "jobs" {
  name                       = "acme-prod-jobs"
  visibility_timeout_seconds = 300
  message_retention_seconds  = 345600
  tags                       = local.tags
}
