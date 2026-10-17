resource "aws_lambda_function" "webhook" {
  function_name = "${var.name}-webhook"
  role          = aws_iam_role.lambda.arn
  runtime       = "python3.12"
  handler       = "app.handler"
  filename      = "webhook.zip"
  timeout       = 15

  environment {
    variables = {
      QUEUE_URL = aws_sqs_queue.orders.url
      LOG_LEVEL = "info"
    }
  }
}

resource "aws_lambda_function" "reports" {
  function_name = "${var.name}-reports"
  role          = aws_iam_role.lambda.arn
  runtime       = "python3.12"
  handler       = "reports.handler"
  filename      = "reports.zip"
  timeout       = 300
  memory_size   = 1024
}

resource "aws_api_gateway_rest_api" "webhooks" {
  name = "${var.name}-webhooks"
}

resource "aws_api_gateway_deployment" "webhooks" {
  rest_api_id = aws_api_gateway_rest_api.webhooks.id

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_api_gateway_stage" "webhooks" {
  rest_api_id   = aws_api_gateway_rest_api.webhooks.id
  deployment_id = aws_api_gateway_deployment.webhooks.id
  stage_name    = "v1"
}

resource "aws_cloudfront_distribution" "assets" {
  enabled             = true
  default_root_object = "index.html"

  origin {
    domain_name = module.storage.assets_bucket_domain
    origin_id   = "assets"
  }

  default_cache_behavior {
    target_origin_id       = "assets"
    viewer_protocol_policy = "allow-all"
    allowed_methods        = ["GET", "HEAD"]
    cached_methods         = ["GET", "HEAD"]

    forwarded_values {
      query_string = false
      cookies {
        forward = "none"
      }
    }
  }

  restrictions {
    geo_restriction {
      restriction_type = "none"
    }
  }

  viewer_certificate {
    cloudfront_default_certificate = true
  }
}

resource "aws_cloudfront_distribution" "downloads" {
  enabled = true

  origin {
    domain_name = aws_api_gateway_rest_api.webhooks.id
    origin_id   = "reports-api"

    custom_origin_config {
      http_port              = 80
      https_port             = 443
      origin_protocol_policy = "http-only"
      origin_ssl_protocols   = ["TLSv1"]
    }
  }

  default_cache_behavior {
    target_origin_id       = "reports-api"
    viewer_protocol_policy = "allow-all"
    allowed_methods        = ["GET", "HEAD", "OPTIONS"]
    cached_methods         = ["GET", "HEAD"]

    forwarded_values {
      query_string = true
      cookies {
        forward = "all"
      }
    }
  }

  restrictions {
    geo_restriction {
      restriction_type = "none"
    }
  }

  viewer_certificate {
    cloudfront_default_certificate = true
  }
}
