terraform {
  required_version = ">= 1.16.0"

  required_providers {
    random = {
      source  = "hashicorp/random"
      version = "3.7.2"
    }
    null = {
      source  = "hashicorp/null"
      version = "3.2.4"
    }
  }
}

variable "release" {
  type    = string
  default = "2026.10.2"
}

resource "random_pet" "app" {
  length = 2
  keepers = {
    stage = "production"
  }
}

resource "random_pet" "app_staging" {
  length = 2
  keepers = {
    stage = "staging"
  }
}

resource "random_id" "assets" {
  byte_length = 6
  prefix      = "assets-"
}

resource "random_string" "suffix" {
  length  = 8
  special = false
  upper   = false
}

resource "random_password" "db" {
  length           = 24
  override_special = "!#%*-_"
}

resource "random_password" "api_key" {
  length  = 40
  special = false
}

resource "random_password" "signing" {
  length  = 32
  special = false
}

resource "random_bytes" "session_key" {
  length = 32
}

resource "random_integer" "db_port" {
  min = 15000
  max = 15999
}

resource "random_integer" "worker_count" {
  min = 2
  max = 8
}

resource "random_uuid" "release" {
  keepers = {
    release = var.release
  }
}

resource "random_uuid" "canary" {
  keepers = {
    release = var.release
  }
}

resource "random_shuffle" "azs" {
  input        = ["zone-a", "zone-b", "zone-c", "zone-d"]
  result_count = 2
}

resource "null_resource" "build" {
  for_each = toset(["api", "web", "worker"])

  triggers = {
    service = each.key
    release = var.release
  }
}

resource "null_resource" "notify" {
  triggers = {
    release = random_uuid.release.result
  }
}

resource "terraform_data" "migrate" {
  input = {
    release = var.release
    port    = random_integer.db_port.result
  }
}

resource "terraform_data" "seed" {
  input = "fixtures-v3"
}

resource "terraform_data" "app_secret" {
  input = random_password.signing.result
}

module "cache" {
  source = "./modules/cache"
  nodes  = 3
  prefix = random_pet.app.id
}
