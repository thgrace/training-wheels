# Training Wheels (TW)

Pre-execution safety hook for AI coding agents. Training Wheels intercepts shell commands before they run and blocks destructive operations like `rm -rf /`, `git reset --hard`, `DROP TABLE`, and `terraform destroy`.

## Quick Start

Install from the latest GitHub release with [`install.sh`](install.sh):

```sh
curl -fsSL https://raw.githubusercontent.com/thgrace/training-wheels/main/install.sh | sh
```

The shell installer downloads the correct macOS/Linux release binary and installs it to `/usr/local/bin` when writable, otherwise `~/.local/bin`. Set `TW_INSTALL_DIR` to override the destination or `TW_VERSION` to pin a specific release.

Install on Windows with [`install.ps1`](install.ps1):

```powershell
irm https://raw.githubusercontent.com/thgrace/training-wheels/main/install.ps1 | iex
```

The PowerShell installer downloads the correct Windows release binary, installs it under `%LOCALAPPDATA%\Programs\tw\bin` by default, and adds that directory to the user `Path` if needed. Set `TW_INSTALL_DIR` or `TW_VERSION` before running it to override the defaults.

Set up the hook in your agent settings:

```sh
tw install                   # auto-detect supported agents
tw install --agent claude    # or target one agent explicitly

# Test it
tw test "rm -rf /"        # → DENY
tw test "git status"      # → ALLOW
```

## How It Works

Training Wheels runs as a pre-execution hook. When an AI agent tries to run a shell command:

1. The agent's tool-use framework sends the command to TW via stdin
2. TW evaluates it against 100+ built-in pattern packs covering git, filesystem, databases, Kubernetes, cloud, containers, CI/CD, package managers, secrets, and more
3. Safe commands pass through silently (exit 0)
4. Destructive commands are blocked with an explanation (exit 1)

The evaluation pipeline includes context-aware sanitization — commands like `git commit -m "rm -rf /"` are correctly allowed because the destructive string is in a data context (commit message), not executed code.

## Commands

| Command | Description |
|---|---|
| `tw hook` | Core hook mode — reads JSON from stdin and returns allow/deny/ask |
| `tw test <cmd>` | Test if a command would be blocked |
| `tw explain <cmd>` | Show why a command is allowed, denied, or asked |
| `tw install` | Install TW hooks and skills into detected agent settings |
| `tw uninstall` | Remove TW hooks and skills from agent settings |
| `tw allow ...` | Add, list, clear, or remove session, timed, or permanent allow/deny entries |
| `tw config` | Show resolved configuration |
| `tw packs` | List available packs and whether each one is enabled |
| `tw doctor` | Check binary, config, hooks, packs, and installed skills |
| `tw update` | Check for or install the latest version |
| `tw version` | Print version |

`tw install` and `tw uninstall` also support `--agent claude,cursor,gemini,copilot` and `--project`.

## Allow Entries

When Training Wheels blocks a command you know is safe, inspect it first, then add an allow entry. Exactly one of `--session`, `--time`, or `--permanent` is required when creating an entry.

```sh
# Explain the match first
tw explain "git reset --hard HEAD"

# Exact command match for this session
tw allow --session "rm -rf ./dist"

# Allow for a fixed time window
tw allow --time 4h "git push --force"

# Permanent exact-command allow
tw allow --permanent "rm -rf ./dist" --reason "Build output cleanup"

# Permanent prefix or rule match
tw allow --permanent --prefix "make clean" --reason "Standard build task"
tw allow --permanent --rule "core.git:reset-hard" --reason "Known safe in this repo"

# Project-scoped permanent allow
tw allow --permanent --project "rm -rf ./tmp" --reason "Repo-local cleanup"

# Permanently deny a command
tw allow --permanent --deny "evil-command" --reason "Never allow this"

# Manage entries
tw allow --list
tw allow --remove sa-1a2b
tw allow --clear
```

Permanent entries are stored in JSON at two levels:
- **Project:** `.tw/overrides.json` (higher precedence)
- **User:** `~/.tw/overrides.json`

Session and time-scoped entries are stored under `~/.tw/` and can be cleared with `tw allow --clear`.

## Configuration

`tw install` creates `~/.tw/config.json` if it does not exist. Use `tw config` to inspect the resolved configuration or `tw config --format json` for machine-readable output.

Example config:
```json
{
  "general": {
    "hook_timeout_ms": 200,
    "max_command_bytes": 131072
  },
  "packs": {
    "enabled": ["core.git", "core.filesystem", "core.tw"],
    "disabled": [],
    "paths": [],
    "default_action": "deny",
    "min_severity": "low"
  },
  "allow": {
    "require_reason": false
  }
}
```

TW also auto-loads external packs from `~/.tw/packs` and `.tw/packs`. Add extra files or directories with `packs.paths`.

### Pack Categories

Run `tw packs` to see the full list of pack IDs and enabled status. Common categories include `core`, `database`, `kubernetes`, `cloud`, `containers`, `infrastructure`, `storage`, `remote`, `secrets`, `cicd`, `package_managers`, and `windows`.

Enable more packs in `~/.tw/config.json`:

```json
{
  "packs": {
    "enabled": ["core.git", "core.filesystem", "core.tw", "database", "kubernetes"],
    "default_action": "ask",
    "min_severity": "medium"
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
|---|--------|
| Quick-reject (no keyword match) | <5μm   |
| Full pipeline (keyword match → pack eval) | <20ms  |
| Absolute maximum (fail-open) | 200ms  |

TW fails open — if evaluation exceeds the timeout, the command is allowed. Safety should never block productivity.

