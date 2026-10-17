variable "common_name" {
  type = string
}

variable "dns_names" {
  type = list(string)
}

variable "labels" {
  type = map(string)
}

resource "tls_private_key" "this" {
  algorithm   = "ECDSA"
  ecdsa_curve = "P256"
}

resource "tls_self_signed_cert" "this" {
  private_key_pem       = tls_private_key.this.private_key_pem
  validity_period_hours = 720
  dns_names             = var.dns_names

  subject = {
    common_name  = var.common_name
    organization = lookup(var.labels, "owners", "platform")
  }
}

output "cert_pem" {
  value = tls_self_signed_cert.this.cert_pem
}
