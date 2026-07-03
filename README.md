# gjoll

> **Experimental**: This project is experimental, may change at any time, and is unsupported.

A CLI tool to provision cloud VM sandboxes for coding agents. Each environment is a standard OpenTofu `.tf` file — you get the full power of HCL with no abstractions in the way. Supports any provider with an OpenTofu provider (AWS, Proxmox, etc.).

## Install

```bash
go install github.com/ondrejbudai/gjoll/cmd/gjoll@latest
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
| `gjoll create <env> [-n name]` | Create a VM, run init, then stop it |
| `gjoll down <name>` | Destroy VM and all resources |
| `gjoll start <name>` | Start a stopped sandbox |
| `gjoll stop <name>` | Stop a running sandbox |
| `gjoll list` | List all sandboxes |
| `gjoll status <name>` | Show sandbox details |
| `gjoll ssh <name> [command...]` | SSH into sandbox (or run a command) |
| `gjoll ssh --wakeup <name> -- <cmd>` | Start, run command, stop |
| `gjoll ssh --proxy <name>` | SSH with proxies tunneled through the connection |
| `gjoll ssh -R port:host:hostport <name>` | SSH with extra reverse tunnels |
| `gjoll push <name> [--path]` | Git push current repo to VM |
| `gjoll pull <name> [refspec] [--path]` | Git fetch from VM, create local branch |
| `gjoll cp <name> <src> <dest>` | Copy files (prefix remote paths with `:`) |
| `gjoll proxy <name>` | Start credential-injecting proxies with SSH reverse tunnels |
| `gjoll proxy -R port:host:hostport <name>` | Proxy with extra reverse tunnels |

## Environment Files

Environments are standard `.tf` files. gjoll injects three variables and reads outputs:

**Injected variables** (available in your `.tf`):
- `var.gjoll_ssh_pubkey` — public key for SSH access
- `var.gjoll_name` — sandbox name
- `var.gjoll_instance_state` — desired state: `"running"` or `"stopped"` (default `"running"`)

**Required outputs:**
- `public_ip` — VM's SSH-reachable IP
- `instance_id` — cloud instance ID
- `ssh_user` — SSH username

**Optional outputs:**
- `init_script` — bash script run over SSH after boot
- `copy_files` — list of `{from, to}` objects; copies local files/directories to the VM after init. If `to` is omitted, it defaults to the same path as `from`
- `proxies` — list of HTTP reverse proxy configurations for credential-free API access (see [Proxy](#proxy) below)

See `examples/` for complete environment files. For libvirt on Fedora with configurable proxy modes (Vertex AI, local LLM, Anthropic API), use `examples/fedora-libvirt/`.

### Base image cache

Libvirt examples that define `variable "base_image_url"` download the Fedora cloud image once to `~/.cache/gjoll/images/` (or `$XDG_CACHE_HOME/gjoll/images/`). Subsequent sandboxes upload from the cached file instead of re-downloading over HTTP.

## How It Works

1. `gjoll up` copies your `.tf` file(s) to a workspace directory
2. Caches the base cloud image locally when the env defines `base_image_url` (see below)
3. Generates an SSH keypair and injects `gjoll_ssh_pubkey`, `gjoll_name`, and `gjoll_instance_state` as OpenTofu variables
4. Runs `tofu init` and `tofu apply`
5. Reads outputs (`public_ip`, `instance_id`, `ssh_user`)
6. If `init_script` output exists, waits for SSH and runs it on the VM
7. If `copy_files` output exists, copies each file from the local machine to the VM
8. Saves instance metadata for other commands

`gjoll start` and `gjoll stop` change `gjoll_instance_state` and re-run `tofu apply`. Your `.tf` file should use this variable to control the instance state (e.g., `running = var.gjoll_instance_state == "running"` for libvirt). The IP address may change after a restart — gjoll updates the SSH config automatically.

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
gjoll pull my-vm                         # auto-detect remote branch → gjoll-my-vm
gjoll pull my-vm feature                 # fetch "feature" → gjoll-my-vm
gjoll pull my-vm feature:my-branch       # fetch "feature" → my-branch
gjoll pull my-vm :my-branch              # auto-detect remote branch → my-branch
```

## Proxy

The `gjoll proxy` command enables secure API access from sandboxed VMs **without copying credentials to the VM**. It starts one or more local HTTP reverse proxies that optionally inject authentication headers (GCP Application Default Credentials, API keys, or no auth) and creates SSH reverse tunnels to the VM. This is ideal for tools like Claude Code running in sandboxes that need to call cloud APIs.

### How it works

```
App on VM  →  http://localhost:18080
           →  SSH reverse tunnel (-R 18080:127.0.0.1:<local-port>)
           →  Local proxy on host (127.0.0.1:<local-port>)
           →  Optionally injects auth header (GCP Bearer token or API key)
           →  https://<target>
```

All credentials stay on your local machine. The VM never sees any secrets.

### Configuration

Add a `proxies` output to your `.tf` file:

```hcl
output "proxies" {
  value = [
    # Vertex AI with GCP auth
    {
      name   = "vertex"
      target = "https://us-east5-aiplatform.googleapis.com/v1"
      auth   = "gcp"          # "gcp", "api-key", or omit for no auth
      port   = 18080          # optional, defaults to 18080
    },
    # Direct Anthropic API with API key
    {
      name         = "anthropic"
      target       = "https://api.anthropic.com"
      auth         = "api-key"
      api_key_file = "~/.anthropic/api_key"
      port         = 18081
    },
    # No-auth passthrough proxy
    {
      name   = "internal"
      target = "https://internal.api.example.com"
      port   = 18082
    },
  ]
}
```

Fields (per proxy):
- `name` (required) — unique identifier for this proxy
- `target` (required) — upstream URL to forward requests to
- `auth` (optional) — `"gcp"`, `"api-key"`, or omit for no authentication
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

### Extra reverse tunnels (-R)

Both `gjoll proxy` and `gjoll ssh` accept `-R` (long form `--reverse`) to set up
additional SSH reverse tunnels, using the same syntax as `ssh -R`. This is useful
for forwarding services that don't need credential injection (e.g. a local MCP
server or database).

```bash
# Forward local port 3000 to port 8080 on the VM
gjoll ssh mybox -R 8080:localhost:3000

# Combine with --proxy for terraform-configured proxies + extra tunnels
gjoll ssh mybox --proxy -R 9090:localhost:80

# Multiple tunnels
gjoll proxy mybox -R 8080:localhost:3000 -R 9090:localhost:5432

# Works without any terraform proxy config
gjoll proxy mybox -R 8080:localhost:3000
```

The `-R` flag can be specified multiple times and composes with terraform-configured
proxies. When using `gjoll proxy`, at least one of terraform proxies or `-R` flags
must be provided.

See `examples/ubuntu-claude-vertex.tf` for AWS + Vertex AI, or `examples/fedora-libvirt/` for libvirt with dynamic proxy modes (Vertex, local LLM, Anthropic).

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
