variable "name" {
  type = string
}

variable "labels" {
  type = map(string)
}

locals {
  defaults = {
    name       = var.name
    managed_by = "terraform"
  }
}

output "tags" {
  value = merge(local.default_labels, var.labels)
}
