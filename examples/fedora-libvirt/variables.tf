variable "proxy_mode" {
  type        = string
  description = "Proxy mode: vertex (default), local-llm, anthropic"
  default     = "vertex"

  validation {
    condition     = contains(["vertex", "local-llm", "anthropic"], var.proxy_mode)
    error_message = "proxy_mode must be vertex, local-llm, or anthropic"
  }
}

variable "vertex_region" {
  type        = string
  description = "GCP region for Vertex AI proxy target"
  default     = "us-east5"
}

variable "vertex_project_id" {
  type        = string
  description = "GCP project ID for Vertex AI (informational; auth via host ADC)"
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

  proxies = var.proxy_mode == "local-llm" ? [
    {
      name   = "llm"
      target = "http://127.0.0.1:${var.llm_host_port}"
      port   = var.llm_proxy_port
    },
    ] : var.proxy_mode == "anthropic" ? [
    {
      name         = "anthropic"
      target       = "https://api.anthropic.com"
      auth         = "api-key"
      api_key_file = var.anthropic_key_file
      port         = var.proxy_port
    },
    ] : [
    {
      name   = "vertex"
      target = "https://${var.vertex_region}-aiplatform.googleapis.com/v1"
      auth   = "gcp"
      port   = var.proxy_port
    },
  ]
}
