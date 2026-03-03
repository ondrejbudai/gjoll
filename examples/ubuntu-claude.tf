terraform {
  required_providers {
    aws = { source = "hashicorp/aws", version = "~> 5.0" }
  }
}

provider "aws" { region = "us-east-1" }

data "aws_ami" "ubuntu" {
  most_recent = true
  owners      = ["099720109477"] # Canonical
  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd-gp3/ubuntu-noble-24.04-amd64-server-*"]
  }
  filter {
    name   = "architecture"
    values = ["x86_64"]
  }
}

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
  ami                    = data.aws_ami.ubuntu.id
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
output "ssh_user"    { value = "ubuntu" }
output "init_script" {
  value = <<-EOT
    #!/bin/bash
    set -euo pipefail
    export DEBIAN_FRONTEND=noninteractive
    sudo apt-get update
    sudo apt-get install -y git tmux gcc make curl

    # Install Node.js (for Claude Code)
    curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash -
    sudo apt-get install -y nodejs

    # Install Claude Code
    sudo npm install -g @anthropic-ai/claude-code
  EOT
}

# Uncomment to copy secrets from your local machine to the VM after init:
# output "clone_secrets" {
#   value = [
#     { from = "~/.ssh/id_ed25519.pub" },
#     { from = "~/.anthropic/api_key", to = "/home/ubuntu/.config/anthropic/api_key" },
#   ]
# }
