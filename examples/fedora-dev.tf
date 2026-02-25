terraform {
  required_providers {
    aws = { source = "hashicorp/aws", version = "~> 5.0" }
  }
}

provider "aws" { region = "us-east-1" }

resource "aws_key_pair" "gjoll" {
  key_name   = "gjoll-${var.gjoll_name}"
  public_key = var.gjoll_ssh_pubkey
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
}

resource "aws_instance" "sandbox" {
  ami                    = "ami-0edf1d45580ac3fa3" # Fedora 43 x86_64 in us-east-1
  instance_type          = "m8i.large"
  key_name               = aws_key_pair.gjoll.key_name
  vpc_security_group_ids = [aws_security_group.gjoll.id]

  root_block_device {
    volume_size = 50
  }

  tags = {
    Name      = "gjoll-${var.gjoll_name}"
    ManagedBy = "gjoll"
  }
}

output "public_ip"   { value = aws_instance.sandbox.public_ip }
output "instance_id" { value = aws_instance.sandbox.id }
output "ssh_user"    { value = "fedora" }
output "init_script" {
  value = <<-EOT
    #!/bin/bash
    set -euo pipefail
    sudo dnf install -y git tmux gcc make python3 python3-pip
  EOT
}
