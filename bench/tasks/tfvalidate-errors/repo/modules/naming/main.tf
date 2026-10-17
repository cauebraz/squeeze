variable "name" {
  type = string
}

variable "environment" {
  type = string
}

variable "domain" {
  type = string
}

resource "random_id" "suffix" {
  byte_length = 3
}

output "full_name" {
  value = "${var.environment}-${var.name}-${random_id.suffix.hex}"
}

output "hostname" {
  value = "${var.name}.${var.domain}"
}
