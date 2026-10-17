terraform {
  required_version = ">= 1.5"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.region

  default_tags {
    tags = {
      Project     = var.name
      Environment = var.environment
    }
  }
}

module "network" {
  source = "./modules/network"

  name               = var.name
  cidr_block         = var.vpc_cidr
  availability_zones = var.availability_zones
  operators_cidr     = var.operators_cidr
}

module "storage" {
  source = "./modules/storage"

  name        = var.name
  environment = var.environment
}

module "data" {
  source = "./modules/data"

  name                 = var.name
  subnet_ids           = module.network.private_subnet_ids
  db_security_group    = module.network.db_security_group_id
  cache_security_group = module.network.cache_security_group_id
  db_password          = var.db_password
  cache_auth_token     = var.cache_auth_token
  kms_key_arn          = aws_kms_key.main.arn
  monitoring_role_arn  = aws_iam_role.rds_monitoring.arn
}

module "compute" {
  source = "./modules/compute"

  name                   = var.name
  vpc_id                 = module.network.vpc_id
  public_subnet_ids      = module.network.public_subnet_ids
  private_subnet_ids     = module.network.private_subnet_ids
  alb_security_group     = module.network.alb_security_group_id
  app_security_group     = module.network.app_security_group_id
  bastion_security_group = module.network.bastion_security_group_id
  ami_id                 = var.ami_id
  instance_profile       = aws_iam_instance_profile.app.name
  certificate_arn        = var.certificate_arn
}
