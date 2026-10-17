variable "environment" {
  type        = string
  description = "Deployment environment name."
  default     = "staging"
}

variable "domain" {
  type        = string
  description = "Base DNS domain of the services."
  default     = "staging.example.internal"
}

variable "owners" {
  type        = list(string)
  description = "Teams that own the stack."
  default     = ["platform", "billing"]
}

variable "output_dir" {
  type        = string
  description = "Directory that receives the rendered manifests."
  default     = "out"
}

variable "rotation_days" {
  type        = number
  description = "Days between secret rotations."
  default     = 30
}
