resource "aws_launch_template" "app" {
  name_prefix   = "${var.name}-app-"
  image_id      = var.ami_id
  instance_type = "t3.medium"

  iam_instance_profile {
    name = var.instance_profile
  }

  network_interfaces {
    associate_public_ip_address = false
    security_groups             = [var.app_security_group]
  }

  block_device_mappings {
    device_name = "/dev/xvda"

    ebs {
      volume_size = 40
      volume_type = "gp3"
    }
  }

  user_data = base64encode(<<-EOT
    #!/bin/bash
    systemctl enable --now shop-app
  EOT
  )
}

resource "aws_autoscaling_group" "app" {
  name                = "${var.name}-app"
  min_size            = 2
  max_size            = 6
  desired_capacity    = 2
  vpc_zone_identifier = var.private_subnet_ids
  target_group_arns   = [aws_lb_target_group.app.arn]
  health_check_type   = "ELB"

  launch_template {
    id      = aws_launch_template.app.id
    version = "$Latest"
  }
}

resource "aws_instance" "bastion" {
  ami                         = var.ami_id
  instance_type               = "t3.micro"
  subnet_id                   = var.public_subnet_ids[0]
  vpc_security_group_ids      = [var.bastion_security_group]
  associate_public_ip_address = true

  root_block_device {
    volume_size = 20
  }

  user_data = <<-EOT
    #!/bin/bash
    echo "bastion ready"
  EOT
}

resource "aws_instance" "worker" {
  ami                    = var.ami_id
  instance_type          = "c6i.large"
  subnet_id              = var.private_subnet_ids[0]
  vpc_security_group_ids = [var.app_security_group]
  iam_instance_profile   = var.instance_profile
  monitoring             = true
  ebs_optimized          = true

  root_block_device {
    volume_size = 50
    encrypted   = true
  }
}

resource "aws_ebs_volume" "worker_scratch" {
  availability_zone = "us-east-1a"
  size              = 200
  type              = "gp3"
}

resource "aws_lb" "public" {
  name               = "${var.name}-public"
  load_balancer_type = "application"
  subnets            = var.public_subnet_ids
  security_groups    = [var.alb_security_group]
}

resource "aws_lb_target_group" "app" {
  name     = "${var.name}-app"
  port     = 8080
  protocol = "HTTP"
  vpc_id   = var.vpc_id

  health_check {
    path    = "/healthz"
    matcher = "200"
  }
}

resource "aws_lb_listener" "http" {
  load_balancer_arn = aws_lb.public.arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.app.arn
  }
}

resource "aws_lb_listener" "https" {
  load_balancer_arn = aws_lb.public.arn
  port              = 443
  protocol          = "HTTPS"
  ssl_policy        = "ELBSecurityPolicy-2016-08"
  certificate_arn   = var.certificate_arn

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.app.arn
  }
}

resource "aws_ecr_repository" "app" {
  name                 = "${var.name}-app"
  image_tag_mutability = "MUTABLE"
}
