variable "name" {
  type = string
}

variable "manifest" {
  type = string
}

variable "port" {
  type = number
}

variable "labels" {
  type = map(string)
}

resource "null_resource" "deploy" {
  triggers = {
    name        = var.name
    manifest    = var.manifest
    port        = var.port
    environment = var.labels["environment"]
    component   = var.labels["component"]
    owners      = var.labels["owners"]
    managed_by  = var.labels["managed_by"]
    checksum    = sha256(var.manifest)
    labels      = var.labels
  }
}

output "id" {
  value = null_resource.deploy.id
}
