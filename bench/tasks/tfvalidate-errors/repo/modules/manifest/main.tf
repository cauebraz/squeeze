variable "name" {
  type = string
}

variable "directory" {
  type = string
}

variable "settings" {
  type = object({
    replicas = number
    port     = number
    hostname = string
    secrets  = list(string)
  })
}

variable "labels" {
  type = map(string)
}

resource "local_file" "manifest" {
  filename = "${var.directory}/${var.name}.json"
  content = jsonencode({
    name     = var.name
    settings = var.settings
    labels   = var.labels
  })
  file_permission = "0644"
}

resource "local_file" "summary" {
  filename = "${var.directory}/${var.name}.txt"
  content = join("\n", [
    "name=${var.name}",
    "hostname=${var.settings.hostname}",
    "replicas=${var.settings.replicas}",
    "port=${var.settings.port}",
    var.settings.secrets,
  ])
  file_permission = "0644"
}

resource "local_file" "env" {
  filename          = "${var.directory}/${var.name}.env"
  sensitive_content = join("\n", [for k, v in var.labels : "${upper(k)}=${v}"])
  file_permission   = "0600"
}

resource "local_file" "marker" {
  filename          = "${var.directory}/${var.name}.marker"
  sensitive_content = "managed by terraform"
}

output "path" {
  value = local_file.manifest.filename
}
