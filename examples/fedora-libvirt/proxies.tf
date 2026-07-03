locals {
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
