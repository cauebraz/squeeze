output "service_hostnames" {
  value = {
    api       = module.api.hostname
    worker    = module.worker.hostname
    scheduler = module.scheduler.hostname
    billing   = module.billing.hostname
  }
}

output "service_tags" {
  value = {
    api       = module.api.tags
    worker    = module.worker.tags
    scheduler = module.scheduler.tags
    billing   = module.billing.tags
  }
}

output "manifest_files" {
  value = concat(
    module.api.manifest_files,
    module.worker.manifest_files,
    module.scheduler.manifest_files,
    module.billing.manifest_files,
  )
}
