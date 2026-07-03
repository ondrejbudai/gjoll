output "public_ip" {
  value = var.gjoll_instance_state == "running" ? data.libvirt_domain_interface_addresses.sandbox[0].interfaces[0].addrs[0].addr : ""
}

output "instance_id" { value = tostring(libvirt_domain.sandbox.id) }

output "ssh_user" { value = "fedora" }

output "init_script" {
  value = local.init_script
}

output "proxies" {
  value = local.proxies
}
