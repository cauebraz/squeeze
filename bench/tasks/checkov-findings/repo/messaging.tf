resource "aws_sqs_queue" "orders" {
  name                       = "${var.name}-orders"
  visibility_timeout_seconds = 60
  message_retention_seconds  = 345600
}

resource "aws_sqs_queue" "orders_dlq" {
  name                      = "${var.name}-orders-dlq"
  message_retention_seconds = 1209600
}

resource "aws_sns_topic" "alerts" {
  name = "${var.name}-alerts"
}

resource "aws_dynamodb_table" "locks" {
  name         = "${var.name}-locks"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "LockID"

  attribute {
    name = "LockID"
    type = "S"
  }

  point_in_time_recovery {
    enabled = true
  }
}
