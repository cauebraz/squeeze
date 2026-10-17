variable "region" {
  type    = string
  default = "us-east-1"
}

variable "environment" {
  type    = string
  default = "production"
}

variable "worker_count" {
  type    = number
  default = 2
}

variable "db_password" {
  type      = string
  sensitive = true
}
