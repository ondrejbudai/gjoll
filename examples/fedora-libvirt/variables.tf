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

variable "base_image_url" {
  type        = string
  description = "Fedora cloud image URL when no local cache path is set"
  default     = "https://download.fedoraproject.org/pub/fedora/linux/releases/43/Cloud/x86_64/images/Fedora-Cloud-Base-Generic-43-1.6.x86_64.qcow2"
}

variable "base_image_local_path" {
  type        = string
  description = "Host path to cached qcow2 image (set by gjoll); skips HTTP download"
  default     = ""
}

locals {
  base_image_source = var.base_image_local_path != "" ? var.base_image_local_path : var.base_image_url
}
