variable "name" {
  type        = string
  description = "prefix of every resource name"
}

variable "subnet_ids" {
  type        = list(string)
  description = "private subnets of the data stores"
}

variable "db_security_group" {
  type        = string
  description = "security group of the orders database"
}

variable "cache_security_group" {
  type        = string
  description = "security group of the session cache"
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

variable "kms_key_arn" {
  type        = string
  description = "customer managed KMS key for data at rest"
}

variable "monitoring_role_arn" {
  type        = string
  description = "IAM role RDS enhanced monitoring publishes with"
}
