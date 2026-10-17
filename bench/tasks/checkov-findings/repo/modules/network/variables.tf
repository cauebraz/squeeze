variable "name" {
  type        = string
  description = "prefix of every resource name"
}

variable "cidr_block" {
  type        = string
  description = "CIDR block of the VPC"
}

variable "availability_zones" {
  type        = list(string)
  description = "zones the subnets are spread over"
}

variable "operators_cidr" {
  type        = string
  description = "network of the operators"
}
