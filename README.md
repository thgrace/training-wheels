# Training Wheels (TW)

Pre-execution safety hook for AI coding agents. Training Wheels intercepts shell commands before they run and blocks destructive operations like `rm -rf /`, `git reset --hard`, `DROP TABLE`, and `terraform destroy`.

## Quick Start

```sh
# Install
go install github.com/thgrace/training-wheels/cmd/tw@latest

# Set up the hook in Claude Code
tw install

# Test it
tw test "rm -rf /"        # → DENY
tw test "git status"       # → ALLOW
```

## How It Works

Training Wheels runs as a pre-execution hook. When an AI agent tries to run a shell command:

1. The agent's tool-use framework sends the command to TW via stdin
2. TW evaluates it against 80+ pattern packs covering git, filesystem, databases, Kubernetes, cloud, containers, and more
3. Safe commands pass through silently (exit 0)
4. Destructive commands are blocked with an explanation (exit 1)

The evaluation pipeline includes context-aware sanitization — commands like `git commit -m "rm -rf /"` are correctly allowed because the destructive string is in a data context (commit message), not executed code.

## Commands

| Command | Description |
|---|---|
| `tw hook` | Core hook mode — reads JSON from stdin, outputs allow/deny |
| `tw test <cmd>` | Test if a command would be blocked |
| `tw install` | Install hook into Claude Code settings |
| `tw uninstall` | Remove hook from settings |
| `tw override <cmd>` | Add an override entry (allow or block) |
| `tw unoverride <id>` | Remove an override entry |
| `tw override list` | Show all override entries |
| `tw config` | Show resolved configuration |
| `tw init` | Generate starter `.tw.json` |
| `tw packs` | List available pattern packs |
| `tw doctor` | Check installation health |
| `tw update` | Self-update to latest version |
| `tw completions` | Generate shell completions (bash/zsh/fish) |
| `tw version` | Print version |

## Overrides

When Training Wheels blocks a command you know is safe, add an override:

```sh
# Exact command match (allow)
tw override "rm -rf ./dist" --reason "Build output cleanup"

# Match by rule ID
tw override --rule "core.git:reset-hard" --reason "Known safe in this repo"

# Prefix match
tw override --prefix "make clean" --reason "Standard build task"

# Block a specific command unconditionally
tw override --block "evil-command" --reason "Never allow this"

# Remove an entry
tw unoverride ov-7f3a

# List all entries
tw override list
```

Overrides are stored in JSON at two levels:
- **Project:** `.tw/overrides.json` (higher precedence)
- **User:** `~/.tw/overrides.json`

## Configuration

Generate a starter config:

```sh
tw init
```

This creates `.tw.json`:

```json
{
  "general": {
    "hook_timeout_ms": 200,
    "max_command_bytes": 131072
  },
  "packs": {
    "enabled": ["core"],
    "disabled": []
  },
  "update": {
    "url": "https://api.github.com/repos/thgrace/training-wheels/releases/latest"
  }
}
```

### Pack Categories

| Category | What it covers |
|---|---|
| `core` | Git, filesystem (rm -rf) — enabled by default |
| `database` | PostgreSQL, MySQL, MongoDB, Redis, etc. |
| `kubernetes` | kubectl delete, helm uninstall, etc. |
| `cloud` | AWS, GCP, Azure destructive operations |
| `containers` | Docker/Podman system prune, volume rm, etc. |
| `infrastructure` | Terraform destroy, Ansible, Vagrant, etc. |
| `storage` | S3 rm, gsutil rm, etc. |
| `remote` | SSH, SCP with destructive payloads |

Enable more packs in `.tw.json`:

```json
{
  "packs": {
    "enabled": ["core", "database", "kubernetes"]
  }
}
```

## Project Policies

- Security policy and vulnerability reporting: [SECURITY.md](SECURITY.md)
- Contribution guide: [CONTRIBUTING.md](CONTRIBUTING.md)
- Code of conduct: [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)
- General bug reports and feature requests: GitHub issues

## Performance

| Path | Target |
|---|---|
| Quick-reject (no keyword match) | <5μs |
| Full pipeline (keyword match → pack eval) | <5ms |
| Absolute maximum (fail-open) | 200ms |

TW fails open — if evaluation exceeds the timeout, the command is allowed. Safety should never block productivity.

## Building

```sh
make build          # Build binary
make test           # Run unit tests
make smoke          # Run smoke tests in Docker
make ci             # All checks
```
