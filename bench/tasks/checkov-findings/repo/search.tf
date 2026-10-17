resource "aws_opensearch_domain" "catalog" {
  domain_name    = "${var.name}-catalog"
  engine_version = "OpenSearch_2.13"

  cluster_config {
    instance_type  = "r6g.large.search"
    instance_count = 2
  }

  ebs_options {
    ebs_enabled = true
    volume_size = 100
    volume_type = "gp3"
  }

  vpc_options {
    subnet_ids         = [module.network.private_subnet_ids[0]]
    security_group_ids = [module.network.app_security_group_id]
  }
}
