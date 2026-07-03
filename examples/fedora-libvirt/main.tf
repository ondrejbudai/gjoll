terraform {
  required_providers {
    libvirt = { source = "dmacvicar/libvirt", version = "= 0.9.7" }
  }
}

provider "libvirt" { uri = "qemu:///system" }

resource "libvirt_volume" "base" {
  name     = "fedora-base-${var.gjoll_name}.qcow2"
  pool     = "default"
  capacity = 5368709120 # 5 GiB (required when download has no Content-Length)
  target   = { format = { type = "qcow2" } }
  create = {
    content = {
      url = local.base_image_source
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
  running     = var.gjoll_instance_state == "running"

  cpu = { mode = "host-passthrough" }
  os  = { type = "hvm" }

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
  count  = var.gjoll_instance_state == "running" ? 1 : 0
  domain = libvirt_domain.sandbox.name
  source = "lease"
}
