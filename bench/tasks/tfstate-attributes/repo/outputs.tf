output "app_name" {
  value = random_pet.app.id
}

output "api_url" {
  value = "https://${random_pet.app.id}.${random_string.suffix.result}.svc.example.com/v1"
}

output "staging_url" {
  value = "https://${random_pet.app_staging.id}.${random_string.suffix.result}.svc.example.com/v1"
}

output "db_password" {
  value     = random_password.db.result
  sensitive = true
}

output "api_key" {
  value     = random_password.api_key.result
  sensitive = true
}

output "cache_nodes" {
  value = module.cache.nodes
}

output "cache_auth" {
  value     = module.cache.auth
  sensitive = true
}

output "zones" {
  value = random_shuffle.azs.result
}

output "worker_count" {
  value = random_integer.worker_count.result
}
