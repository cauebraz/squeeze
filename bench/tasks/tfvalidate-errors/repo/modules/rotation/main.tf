variable "name" {
  type = string
}

variable "rotation_days" {
  type = number
}

resource "time_static" "created" {}

resource "time_rotating" "this" {
  rfc3339      = time_static.created.rfc3339
  rotation_day = var.rotation_days
}

output "rotate_at" {
  value = time_rotating.this.rotation_rfc3339
}
