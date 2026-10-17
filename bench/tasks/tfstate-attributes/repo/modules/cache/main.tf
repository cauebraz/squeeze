variable "nodes" {
  type = number
}

variable "prefix" {
  type = string
}

resource "random_pet" "node" {
  count  = var.nodes
  length = 1
  prefix = "${var.prefix}-cache"
}

resource "random_password" "auth" {
  length  = 32
  special = false
}

resource "null_resource" "warmup" {
  triggers = {
    primary = random_pet.node[0].id
  }
}

output "nodes" {
  value = random_pet.node[*].id
}

output "auth" {
  value     = random_password.auth.result
  sensitive = true
}
