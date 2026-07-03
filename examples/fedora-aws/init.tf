locals {
  init_agent = var.agent_backend == "opencode" ? local.init_opencode : join("\n", compact([
    local.init_claude_install,
    lookup(local.init_claude_env, var.proxy_mode, ""),
  ]))

  init_script = <<-EOT
    #!/bin/bash
    set -euo pipefail
    ${local.init_base}
    ${local.init_agent}
  EOT
}
