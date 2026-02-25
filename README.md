# gjoll

A CLI tool to provision cloud VM sandboxes for coding agents. Each environment is a standard OpenTofu `.tf` file — you get the full power of HCL with no abstractions in the way.

## Install

```bash
go install github.com/obudai/gjoll/cmd/gjoll@latest
```

### Prerequisites

- [OpenTofu](https://opentofu.org/) (`tofu`)
- [AWS CLI](https://aws.amazon.com/cli/) (`aws`) — for stop/start
- `ssh`, `scp`, `ssh-keygen`, `git`

## Quick Start

```bash
# Spin up a Fedora dev VM
gjoll up examples/fedora-dev.tf

# SSH in
gjoll ssh fedora-dev

# Push your current repo to the VM
gjoll push fedora-dev

# Pull changes back as a local branch
gjoll pull fedora-dev agent-changes

# Copy files
gjoll cp fedora-dev ./config.env :/home/fedora/
gjoll cp fedora-dev :/home/fedora/output.log ./

# Stop/start (preserves disk)
gjoll stop fedora-dev
gjoll start fedora-dev

# Tear down
gjoll down fedora-dev
```

## Commands

| Command | Description |
|---|---|
| `gjoll up <env> [-n name]` | Create and launch a VM |
| `gjoll down <name>` | Destroy VM and all resources |
| `gjoll stop <name>` | Stop VM (preserves disk) |
| `gjoll start <name>` | Resume a stopped VM |
| `gjoll list` | List all sandboxes |
| `gjoll status <name>` | Show sandbox details |
| `gjoll ssh <name>` | SSH into sandbox |
| `gjoll push <name> [--path]` | Git push current repo to VM |
| `gjoll pull <name> [branch]` | Git fetch from VM, create local branch |
| `gjoll cp <name> <src> <dest>` | Copy files (prefix remote paths with `:`) |

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

See `examples/` for complete environment files.

## How It Works

1. `gjoll up` copies your `.tf` file to a workspace directory
2. Generates an SSH keypair and injects `gjoll_ssh_pubkey` + `gjoll_name` as OpenTofu variables
3. Runs `tofu init` and `tofu apply`
4. Reads outputs (`public_ip`, `instance_id`, `ssh_user`)
5. If `init_script` output exists, waits for SSH and runs it on the VM
6. Saves instance metadata for other commands

## Git Sync

`gjoll push` sets up the VM as a git remote using `receive.denyCurrentBranch=updateInstead`, so the working tree updates on push. `gjoll pull` fetches from the VM and creates a local branch.

## Development

```bash
just build    # Build binary
just test     # Run tests
just lint     # Vet + golangci-lint
just all      # fmt + lint + test + build
```
