terraform {
  required_version = ">= 1.5"
  required_providers {
    cloud = {
      source = "example/cloud"
    }
  }
}

variable "region" {
  default = "us-east-1"
}

variable "unused" {
  type        = string
  description = "never read"
}

variable "instance_type" {
  type        = string
  description = "instance size"
  default     = "small"
}

locals {
  name = "${var.region}-app"
}

resource "cloud_bucket" "logs" {
  name = "${local.name}-logs"
}

resource "cloud_instance" "web" {
  image = "img-0123456789abcdef0"
  size  = var.instance_type
}

module "network" {
  source = "./modules/network"
  cidr   = "10.0.0.0/16"
}

output "first_zone" {
  value = module.network.zones.0
}
