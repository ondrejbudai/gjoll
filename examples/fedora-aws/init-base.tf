locals {
  init_base = <<-EOT
    sudo dnf install -y git-core tmux gcc make
  EOT
}
