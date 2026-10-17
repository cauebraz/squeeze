terraform {
  required_version = ">= 1.6.0"

  required_providers {
    null = {
      source  = "hashicorp/null"
      version = "3.3.2"
    }
    random = {
      source  = "hashicorp/random"
      version = "3.9.1"
    }
    local = {
      source  = "hashicorp/local"
      version = "2.9.1"
    }
    time = {
      source  = "hashicorp/time"
      version = "0.14.2"
    }
    tls = {
      source  = "hashicorp/tls"
      version = "4.4.1"
    }
  }
}
