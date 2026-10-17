output "hostname" {
  value = module.naming.hostname
}

output "tags" {
  value = module.labels.tags
}

output "manifest_files" {
  value = [module.manifest.path]
}

output "certificate_pem" {
  value = module.certificate.cert_pem
}

output "secret_ids" {
  value = [for s in module.secret : s.id]
}

output "rotate_at" {
  value = module.rotation.rotate_at
}

output "deploy_ids" {
  value = module.deploy_hook[*].id
}

output "smoke_id" {
  value = module.smoke_hook.id
}
