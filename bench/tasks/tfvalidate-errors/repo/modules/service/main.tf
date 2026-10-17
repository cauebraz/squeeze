locals {
  full_name = "${var.environment}-${var.name}"
  hostname  = "${var.name}.${var.domain}"
  manifest  = "${var.manifest_dir}/${local.full_name}.json"
}

module "naming" {
  source = "../naming"

  name        = var.name
  environment = var.environment
  domain      = var.domain
}

module "labels" {
  source = "../labels"

  name   = local.full_name
  labels = var.labels
}

module "secret" {
  source   = "../secret"
  for_each = toset(var.secret_names)

  name   = "${local.full_name}-${each.key}"
  labels = var.labels
}

module "certificate" {
  source = "../certificate"

  common_name = local.hostname
  dns_names   = [local.hostname, "${var.name}.internal"]
  labels      = var.labels
}

module "rotation" {
  source = "../rotation"

  name          = local.full_name
  rotation_days = var.rotation_days
}

module "manifest" {
  source = "../manifest"

  name      = local.full_name
  directory = var.manifest_dir
  settings = {
    replicas = var.replicas
    port     = var.port
    hostname = local.hostname
    secrets  = var.secret_names
  }
  labels = var.labels
}

module "deploy_hook" {
  source = "../hook"
  count  = var.replicas

  name     = "${local.full_name}-deploy-${count.index}"
  manifest = local.manifest
  port     = var.port
  labels   = var.labels
}

module "smoke_hook" {
  source = "../hook"

  name     = "${local.full_name}-smoke"
  manifest = local.manifest
  port     = var.port
  labels   = var.labels
}
