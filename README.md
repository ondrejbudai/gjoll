# gjoll

> **Experimental**: This project is experimental, may change at any time, and is unsupported.

A CLI tool to provision cloud VM sandboxes for coding agents. Each environment is a standard OpenTofu `.tf` file — you get the full power of HCL with no abstractions in the way. Supports any provider with an OpenTofu provider (AWS, Proxmox, etc.).

## Install

```bash
go install github.com/obudai/gjoll/cmd/gjoll@latest
```

### Prerequisites

- [OpenTofu](https://opentofu.org/) (`tofu`)
- `ssh`, `scp`, `ssh-keygen`, `git`

## Quick Start

```bash
# Spin up a Fedora dev VM
gjoll up examples/fedora-dev.tf

# SSH in
gjoll ssh fedora-dev

# Run a command over SSH
gjoll ssh fedora-dev uname -a

# Push your current repo to the VM
gjoll push fedora-dev

# Pull changes back as a local branch
gjoll pull fedora-dev agent-changes

# Copy files
gjoll cp fedora-dev ./config.env :/home/fedora/
gjoll cp fedora-dev :/home/fedora/output.log ./

# Tear down
gjoll down fedora-dev
```

## Commands

| Command | Description |
|---|---|
| `gjoll up <env> [-n name]` | Create and launch a VM |
| `gjoll down <name>` | Destroy VM and all resources |
| `gjoll list` | List all sandboxes |
| `gjoll status <name>` | Show sandbox details |
| `gjoll ssh <name> [command...]` | SSH into sandbox (or run a command) |
| `gjoll push <name> [--path]` | Git push current repo to VM |
| `gjoll pull <name> [refspec] [--path]` | Git fetch from VM, create local branch |
| `gjoll cp <name> <src> <dest>` | Copy files (prefix remote paths with `:`) |
| `gjoll proxy <name>` | Start credential-injecting proxy with SSH reverse tunnel |

## Environment Files

Environments are standard `.tf` files. gjoll injects two variables and reads outputs:

**Injected variables** (available in your `.tf`):
- `var.gjoll_ssh_pubkey` — public key for SSH access
- `var.gjoll_name` — sandbox name

**Required outputs:**
- `public_ip` — VM's SSH-reachable IP
- `instance_id` — cloud instance ID
- `ssh_user` — SSH username

**Optional outputs:**
- `init_script` — bash script run over SSH after boot
- `copy_files` — list of `{from, to}` objects; copies local files/directories to the VM after init. If `to` is omitted, it defaults to the same path as `from`
- `proxy` — HTTP reverse proxy configuration for credential-free API access (see [Proxy](#proxy) below)

See `examples/` for complete environment files.

## How It Works

1. `gjoll up` copies your `.tf` file to a workspace directory
2. Generates an SSH keypair and injects `gjoll_ssh_pubkey` + `gjoll_name` as OpenTofu variables
3. Runs `tofu init` and `tofu apply`
4. Reads outputs (`public_ip`, `instance_id`, `ssh_user`)
5. If `init_script` output exists, waits for SSH and runs it on the VM
6. If `copy_files` output exists, copies each file from the local machine to the VM
7. Saves instance metadata for other commands

## Git Sync

`gjoll push` sets up the VM as a git remote using `receive.denyCurrentBranch=updateInstead`, so the working tree updates on push. `gjoll pull` fetches from the VM and creates a local branch. Both commands create the git remote automatically if it doesn't exist yet.

Use `--path` to change where the repo lives on the VM (default `~/project`):

```bash
gjoll push my-vm --path ~/myapp
gjoll pull my-vm --path ~/myapp
```

`gjoll pull` accepts an optional refspec to control which remote branch to fetch
and what local branch name to use:

```bash
gjoll pull my-vm                         # auto-detect remote branch → gjoll/my-vm
gjoll pull my-vm feature                 # fetch "feature" → gjoll/my-vm
gjoll pull my-vm feature:my-branch       # fetch "feature" → my-branch
gjoll pull my-vm :my-branch              # auto-detect remote branch → my-branch
```

## Proxy

The `gjoll proxy` command enables secure API access from sandboxed VMs **without copying credentials to the VM**. It runs a local HTTP reverse proxy that injects authentication headers (GCP Application Default Credentials or API keys) and creates an SSH reverse tunnel to the VM. This is ideal for tools like Claude Code running in sandboxes that need to call cloud APIs.

### How it works

```
App on VM  →  http://localhost:18080
           →  SSH reverse tunnel (-R 18080:127.0.0.1:<local-port>)
           →  Local proxy on host (127.0.0.1:<local-port>)
           →  Injects auth header (GCP Bearer token or API key)
           →  https://<target>
```

All credentials stay on your local machine. The VM never sees any secrets.

### Configuration

Add a `proxy` output to your `.tf` file:

```hcl
# Example: Vertex AI for Claude Code
output "proxy" {
  value = {
    target = "https://us-east5-aiplatform.googleapis.com/v1"
    auth   = "gcp"          # "gcp" or "api-key"
    port   = 18080          # optional, defaults to 18080
  }
}

# Example: Direct Anthropic API
output "proxy" {
  value = {
    target       = "https://api.anthropic.com"
    auth         = "api-key"
    api_key_file = "~/.anthropic/api_key"  # required for api-key auth
    port         = 18080
  }
}
```

Fields:
- `target` (required) — upstream URL to forward requests to
- `auth` (required) — `"gcp"` or `"api-key"`
- `api_key_file` (required for `api-key` auth) — local path to API key file (~ expanded)
- `port` (optional, default 18080) — remote port on VM for the tunnel

### Usage

```bash
# Provision VM with proxy config
gjoll up examples/ubuntu-claude-vertex.tf

# Terminal 1: Start proxy (keeps running)
gjoll proxy mybox

# Terminal 2: SSH to VM and use API
gjoll ssh mybox
curl http://localhost:18080/v1/messages  # authenticated request
```

Applications on the VM connect to `http://localhost:<port>` and `gjoll proxy` handles the rest.

See `examples/ubuntu-claude-vertex.tf` for a complete Vertex AI + Claude Code setup.

## Development

```bash
just build         # Build binary
just test          # Run unit tests
just integration   # Run integration tests (requires libvirt)
just lint          # Vet + golangci-lint
just all           # fmt + lint + test + build
```

The integration tests provision a real VM via libvirt/QEMU and exercise all
commands (`up`, `list`, `status`, `ssh`, `cp`, `push`, `pull`, `down`).
Prerequisites: `tofu`, `qemu-kvm`, `libvirt`, and a running `default` network.
