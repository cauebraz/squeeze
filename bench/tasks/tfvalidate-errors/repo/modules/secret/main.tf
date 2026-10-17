variable "name" {
  type = string
}

variable "labels" {
  type = map(string)
}

resource "random_password" "value" {
  lenght           = 32
  special          = true
  override_special = "-_"
}

resource "random_id" "version" {
  byte_length = 4

  keepers = {
    name = var.name
  }
}

output "id" {
  value = "${var.name}-${random_id.version.hex}"
}

output "value" {
  value     = random_password.value.result
  sensitive = true
}
