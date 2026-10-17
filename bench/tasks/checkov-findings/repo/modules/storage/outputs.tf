output "logs_bucket_id" {
  value = aws_s3_bucket.logs.id
}

output "assets_bucket_domain" {
  value = aws_s3_bucket.assets.bucket_regional_domain_name
}
