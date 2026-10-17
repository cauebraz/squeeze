data "aws_caller_identity" "current" {}

resource "aws_kms_key" "main" {
  description             = "${var.name} data encryption"
  deletion_window_in_days = 30
  enable_key_rotation     = true

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid       = "AccountAdmin"
        Effect    = "Allow"
        Principal = { AWS = "arn:aws:iam::${data.aws_caller_identity.current.account_id}:root" }
        Action    = "kms:*"
        Resource  = "*"
      }
    ]
  })
}

resource "aws_cloudwatch_log_group" "app" {
  name              = "/${var.name}/app"
  retention_in_days = 7
}

resource "aws_cloudwatch_log_group" "workers" {
  name = "/${var.name}/workers"
}

resource "aws_cloudwatch_log_group" "api_access" {
  name              = "/${var.name}/api-access"
  retention_in_days = 30
}

resource "aws_cloudtrail" "main" {
  name                          = "${var.name}-trail"
  s3_bucket_name                = module.storage.logs_bucket_id
  include_global_service_events = true
  is_multi_region_trail         = false
}
