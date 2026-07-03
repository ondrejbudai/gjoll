locals {
  init_opencode = <<-EOT
    curl -fsSL https://opencode.ai/install | bash
    echo 'export PATH="$HOME/.opencode/bin:$PATH"' >> ~/.bashrc
  EOT
}
