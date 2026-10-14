variable "cidr" {
  type        = string
  description = "vpc cidr"
}

output "zones" {
  value = [var.cidr]
}
