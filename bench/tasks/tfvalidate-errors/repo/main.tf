module "api" {
  source = "./modules/service"

  name          = "api"
  environment   = var.environment
  domain        = var.domain
  replicas      = 3
  port          = 8080
  secret_names  = ["db", "session"]
  rotation_days = var.rotation_days
  manifest_dir  = local.manifest_dir
  labels        = merge(local.base_labels, { component = "api" })
}

module "worker" {
  source = "./modules/service"

  name          = "worker"
  environment   = var.environment
  domain        = var.domain
  replicas      = 2
  port          = 9090
  secret_names  = ["db", "queue"]
  rotation_days = var.rotation_days
  manifest_dir  = local.manifest_dir
  labels        = merge(local.base_labels, { component = "worker" })
}

module "scheduler" {
  source = "./modules/service"

  name          = "scheduler"
  environment   = var.environment
  domain        = var.domain
  replicas      = 1
  port          = 9100
  secret_names  = ["db"]
  rotation_days = var.rotation_days
  manifest_dir  = local.manifest_dir
  labels        = merge(local.base_labels, { component = "scheduler" })
}

module "billing" {
  source = "./modules/service"

  name          = "billing"
  environment   = var.environment
  domain        = var.domain
  replicas      = 2
  port          = 8443
  secret_names  = ["db", "stripe", "ledger"]
  rotation_days = var.rotation_days
  manifest_dir  = local.manifest_dir
  labels        = merge(local.base_labels, { component = "billing" })
}
