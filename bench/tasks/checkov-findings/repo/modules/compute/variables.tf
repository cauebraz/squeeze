variable "name" {
  type        = string
  description = "prefix of every resource name"
}

variable "vpc_id" {
  type        = string
  description = "VPC of the hosts"
}

variable "public_subnet_ids" {
  type        = list(string)
  description = "subnets of the load balancer and the bastion"
}

variable "private_subnet_ids" {
  type        = list(string)
  description = "subnets of the application hosts"
}

variable "alb_security_group" {
  type        = string
  description = "security group of the load balancer"
}

variable "app_security_group" {
  type        = string
  description = "security group of the application hosts"
}

variable "bastion_security_group" {
  type        = string
  description = "security group of the bastion"
}

variable "ami_id" {
  type        = string
  description = "AMI of the hosts"
}

variable "instance_profile" {
  type        = string
  description = "instance profile of the application hosts"
}

variable "certificate_arn" {
  type        = string
  description = "ACM certificate of the HTTPS listener"
}
