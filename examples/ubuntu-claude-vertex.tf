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
    cidr_blocks = ["0.0.0.0/0"] # Restrict to your IP in production
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

    # Configure Claude Code to use Vertex AI via local proxy
    # IMPORTANT: Start the proxy with `gjoll proxy <name>` before running claude!
    mkdir -p ~/.config/anthropic
    cat > ~/.config/anthropic/claude_code.json <<'CONFIG'
    {
      "model": "claude-sonnet-4-5@20250929",
      "useVertex": true,
      "vertexProjectId": "YOUR_GCP_PROJECT_ID",
      "vertexRegion": "us-east5",
      "vertexBaseUrl": "http://localhost:18080",
      "skipVertexAuth": true
    }
    CONFIG

    echo ""
    echo "========================================="
    echo "Claude Code installed!"
    echo ""
    echo "IMPORTANT: Edit ~/.config/anthropic/claude_code.json and set YOUR_GCP_PROJECT_ID"
    echo ""
    echo "Usage:"
    echo "  1. On your local machine: gjoll proxy <name>"
    echo "  2. SSH to VM: gjoll ssh <name>"
    echo "  3. Run Claude: claude"
    echo "========================================="
  EOT
}

# Proxy configuration — no secrets on VM!
output "proxies" {
  value = [
    {
      name   = "vertex"
      target = "https://us-east5-aiplatform.googleapis.com/v1"
      auth   = "gcp"
      port   = 18080
    },
  ]
}

# Note: No copy_files output — credentials stay on your local machine!
# The proxy injects GCP credentials via Application Default Credentials.
