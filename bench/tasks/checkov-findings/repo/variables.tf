variable "region" {
  type        = string
  description = "AWS region of the stack"
  default     = "us-east-1"
}

variable "name" {
  type        = string
  description = "prefix of every resource name"
  default     = "shop"
}

variable "environment" {
  type        = string
  description = "deployment environment"
  default     = "staging"
}

variable "vpc_cidr" {
  type        = string
  description = "CIDR block of the VPC"
  default     = "10.40.0.0/16"
}

variable "availability_zones" {
  type        = list(string)
  description = "zones the subnets are spread over"
  default     = ["us-east-1a", "us-east-1b", "us-east-1c"]
}

variable "operators_cidr" {
  type        = string
  description = "network of the operators"
  default     = "10.20.0.0/16"
}

variable "ami_id" {
  type        = string
  description = "AMI of the application hosts"
}

variable "certificate_arn" {
  type        = string
  description = "ACM certificate of the public load balancer"
}

variable "db_password" {
  type        = string
  description = "master password of the orders database"
  sensitive   = true
}

variable "cache_auth_token" {
  type        = string
  description = "auth token of the session cache"
  sensitive   = true
}
