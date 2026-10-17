variable "name" {
  type = string
}

variable "environment" {
  type = string
}

variable "domain" {
  type = string
}

variable "replicas" {
  type = number
}

variable "port" {
  type = number
}

variable "secret_names" {
  type = list(string)
}

variable "rotation_days" {
  type = number
}

variable "manifest_dir" {
  type = string
}

variable "labels" {
  type = map(string)
}
