terraform {
  required_providers {
    aws = { source = "hashicorp/aws", version = "~> 6.0" }
  }
}

provider "aws" {
  region = var.aws_region
  ignore_tags {
    keys = ["architecture"]
  }
}

resource "aws_key_pair" "gjoll" {
  key_name   = "gjoll-${var.gjoll_name}"
  public_key = var.gjoll_ssh_pubkey
  tags = {
    ManagedBy = "drella"
    Project   = "drella"
  }
}

resource "aws_security_group" "gjoll" {
  name = "gjoll-${var.gjoll_name}"
  ingress {
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
  tags = {
    ManagedBy = "drella"
    Project   = "drella"
  }
}

resource "aws_instance" "sandbox" {
  ami                    = var.ami_id
  instance_type          = var.instance_type
  key_name               = aws_key_pair.gjoll.key_name
  vpc_security_group_ids = [aws_security_group.gjoll.id]

  cpu_options {
    nested_virtualization = "enabled"
  }

  root_block_device {
    volume_size = 50
    tags = {
      ManagedBy = "drella"
      Project   = "drella"
    }
  }

  tags = {
    Name      = "gjoll-${var.gjoll_name}"
    ManagedBy = "drella"
    persist   = "true"
    Project   = "drella"
  }
}

resource "aws_ec2_instance_state" "sandbox" {
  instance_id = aws_instance.sandbox.id
  state       = var.gjoll_instance_state
}
