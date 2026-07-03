variable "proxy_mode" {
  type        = string
  description = "Proxy mode: vertex (default), local-llm, anthropic"
  default     = "vertex"

  validation {
    condition     = contains(["vertex", "local-llm", "anthropic"], var.proxy_mode)
    error_message = "proxy_mode must be vertex, local-llm, or anthropic"
  }
}

variable "agent_backend" {
  type        = string
  description = "Coding agent to install: opencode or claude-code"
  default     = "opencode"

  validation {
    condition     = contains(["opencode", "claude-code"], var.agent_backend)
    error_message = "agent_backend must be opencode or claude-code"
  }
}

variable "ami_id" {
  type        = string
  description = "Fedora cloud AMI ID (us-east-1 Fedora 43 x86_64 by default)"
  default     = "ami-0edf1d45580ac3fa3"
}

variable "aws_region" {
  type        = string
  description = "AWS region for EC2 resources"
  default     = "us-east-1"
}

variable "instance_type" {
  type        = string
  description = "EC2 instance type"
  default     = "m8i.large"
}

variable "vertex_region" {
  type        = string
  description = "GCP region for Vertex AI proxy target"
  default     = "us-east5"
}

variable "vertex_project_id" {
  type        = string
  description = "GCP project ID for Vertex AI (Claude Code vertex mode)"
  default     = ""
}

variable "llm_host_port" {
  type        = number
  description = "Port on the orchestrator host where the local LLM API listens"
  default     = 11434
}

variable "llm_proxy_port" {
  type        = number
  description = "Port exposed inside the VM for the local LLM gjoll proxy"
  default     = 11434
}

variable "proxy_port" {
  type        = number
  description = "Port inside the VM for credential proxies (vertex, anthropic)"
  default     = 18080
}

variable "anthropic_key_file" {
  type        = string
  description = "Host path to Anthropic API key file (anthropic proxy mode)"
  default     = "~/.anthropic/api_key"
}
