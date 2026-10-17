locals {
  base_labels = {
    environment = var.environment
    managed_by  = "terraform"
    owners      = join(",", var.owners)
  }

  manifest_dir = "${path.root}/${var.output_dir}"
}
