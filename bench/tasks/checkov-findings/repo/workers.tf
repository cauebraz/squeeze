resource "aws_lambda_function" "thumbnails" {
  function_name = "${var.name}-thumbnails"
  role          = aws_iam_role.lambda.arn
  runtime       = "nodejs20.x"
  handler       = "index.handler"
  filename      = "thumbnails.zip"
  timeout       = 60
  memory_size   = 512

  environment {
    variables = {
      ASSETS_BUCKET = module.storage.assets_bucket_domain
    }
  }
}

resource "aws_lambda_function" "order_events" {
  function_name = "${var.name}-order-events"
  role          = aws_iam_role.lambda.arn
  runtime       = "python3.12"
  handler       = "events.handler"
  filename      = "events.zip"
  timeout       = 30

  environment {
    variables = {
      TOPIC_ARN = aws_sns_topic.alerts.arn
    }
  }
}

resource "aws_lambda_event_source_mapping" "order_events" {
  event_source_arn = aws_sqs_queue.orders.arn
  function_name    = aws_lambda_function.order_events.arn
  batch_size       = 10
}
