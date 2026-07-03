output "public_ip" {
  value = var.gjoll_instance_state == "running" ? aws_instance.sandbox.public_ip : ""
}

output "instance_id" { value = aws_instance.sandbox.id }

output "ssh_user" { value = "fedora" }

output "init_script" {
  value = local.init_script
}

output "proxies" {
  value = local.proxies
}
