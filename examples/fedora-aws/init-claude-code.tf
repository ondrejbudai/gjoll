locals {
  init_claude_install = <<-EOT
    curl -fsSL https://claude.ai/install.sh | bash
  EOT

  init_claude_env = {
    vertex = <<-EOT
      cat >> ~/.bashrc <<'RCEOF'
      export CLAUDE_CODE_USE_VERTEX=1
      export CLOUD_ML_REGION=${var.vertex_region}
      export ANTHROPIC_VERTEX_PROJECT_ID=${var.vertex_project_id}
      export ANTHROPIC_VERTEX_BASE_URL=http://localhost:${var.proxy_port}
      export CLAUDE_CODE_SKIP_VERTEX_AUTH=1
      export ANTHROPIC_MODEL=claude-opus-4-6
      alias claude='claude --dangerously-skip-permissions'
      RCEOF
    EOT
    anthropic = <<-EOT
      cat >> ~/.bashrc <<'RCEOF'
      export ANTHROPIC_BASE_URL=http://localhost:${var.proxy_port}
      export ANTHROPIC_MODEL=claude-sonnet-4-5
      alias claude='claude --dangerously-skip-permissions'
      RCEOF
    EOT
    local-llm = ""
  }
}
