output "alb_dns_name" {
  description = "DNS name of the public load balancer"
  value       = module.compute.alb_dns_name
}

output "api_url" {
  description = "invoke URL of the webhook API"
  value       = aws_api_gateway_stage.webhooks.invoke_url
}

output "db_endpoint" {
  description = "endpoint of the orders database"
  value       = module.data.db_endpoint
}

output "assets_domain" {
  description = "domain of the assets distribution"
  value       = aws_cloudfront_distribution.assets.domain_name
}
