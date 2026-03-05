terraform {
  required_providers {
    libvirt = { source = "dmacvicar/libvirt", version = "~> 0.9" }
  }
}

provider "libvirt" { uri = "qemu:///system" }

resource "libvirt_volume" "base" {
  name   = "fedora-base-${var.gjoll_name}.qcow2"
  pool   = "default"
  target = { format = { type = "qcow2" } }
  create = {
    content = {
      url = "https://download.fedoraproject.org/pub/fedora/linux/releases/43/Cloud/x86_64/images/Fedora-Cloud-Base-Generic-43-1.6.x86_64.qcow2"
    }
  }
}

resource "libvirt_volume" "root" {
  name          = "root-${var.gjoll_name}.qcow2"
  pool          = "default"
  capacity      = 53687091200 # 50 GiB
  target        = { format = { type = "qcow2" } }
  backing_store = { path = libvirt_volume.base.path, format = { type = "qcow2" } }
}

resource "libvirt_cloudinit_disk" "init" {
  name = "cloudinit-${var.gjoll_name}.iso"
  meta_data = jsonencode({
    instance-id    = "gjoll-${var.gjoll_name}"
    local-hostname = "gjoll-${var.gjoll_name}"
  })
  user_data = <<-EOF
    #cloud-config
    users:
      - name: fedora
        sudo: ALL=(ALL) NOPASSWD:ALL
        shell: /bin/bash
        ssh_authorized_keys:
          - ${var.gjoll_ssh_pubkey}
  EOF
}

resource "libvirt_domain" "sandbox" {
  name        = "gjoll-${var.gjoll_name}"
  type        = "kvm"
  memory      = 4096
  memory_unit = "MiB"
  vcpu        = 2
  running     = true

  os = { type = "hvm" }

  devices = {
    disks = [
      {
        # Use source.file (disk type='file') instead of source.volume so that
        # virt-aa-helper can resolve the path and whitelist it in AppArmor.
        source = { file = { file = libvirt_volume.root.path } }
        target = { dev = "vda", bus = "virtio" }
        driver = { name = "qemu", type = "qcow2" }
      },
      {
        device = "cdrom"
        source = { file = { file = libvirt_cloudinit_disk.init.path } }
        target = { dev = "sda", bus = "sata" }
        driver = { name = "qemu", type = "raw" }
      },
    ]
    interfaces = [
      {
        source      = { network = { network = "default" } }
        model       = { type = "virtio" }
        wait_for_ip = { source = "lease" }
      },
    ]
    consoles = [
      { target = { type = "serial", port = 0 } },
    ]
  }
}

data "libvirt_domain_interface_addresses" "sandbox" {
  domain = libvirt_domain.sandbox.name
  source = "lease"
}

output "public_ip" {
  value = data.libvirt_domain_interface_addresses.sandbox.interfaces[0].addrs[0].addr
}
output "instance_id" { value = tostring(libvirt_domain.sandbox.id) }
output "ssh_user"    { value = "fedora" }
output "init_script" {
  value = <<-EOT
    #!/bin/bash
    set -euo pipefail
    sudo dnf install -y git tmux gcc make
  EOT
}
