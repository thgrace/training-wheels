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
2. TW performs minimal pre-parse cleanup so shell syntax is interpreted consistently across inputs
3. For supported shells, TW parses the command into an AST and evaluates structural patterns against the parsed commands instead of relying on raw regex matching
4. Built-in packs, session overrides, and persistent custom rules determine whether the command is allowed, denied, or must ask
5. Safe commands pass through silently (exit 0); blocked or confirmation-required commands return structured output to the agent

The evaluation pipeline includes context-aware sanitization — commands like `git commit -m "rm -rf /"` are correctly allowed because the destructive string is in a data context (commit message), not executed code.

## Commands

| Command | Description |
|---|---|
| `tw hook` | Core hook mode — reads JSON from stdin and returns allow/deny/ask |
| `tw test <cmd>` | Test if a command would be blocked |
| `tw explain <cmd>` | Show why a command is allowed, denied, or asked |
| `tw install` | Install TW hooks and skills into detected agent settings |
| `tw uninstall` | Remove TW hooks and skills from agent settings |
| `tw override ...` | Add, list, clear, or remove session/time-scoped allow or ask overrides |
| `tw rule ...` | Add, list, or remove persistent custom allow/deny/ask rules |
| `tw config` | Show resolved configuration |
| `tw packs` | List available packs and whether each one is enabled |
| `tw packs apply` | Add selected project-local pack additions |
| `tw packs remove` | Remove selected project-local pack additions |
| `tw doctor` | Check binary, config, hooks, packs, and installed skills |
| `tw update` | Check for or install the latest version |
| `tw version` | Print version |

`tw install` and `tw uninstall` also support `--agent claude,cursor,gemini,copilot` and `--project`.

## Overrides

When Training Wheels blocks or asks on a command you have already reviewed, inspect it first and then add an ephemeral override. Overrides are session- or time-scoped only. For persistent policy, use `tw rule`.

```sh
# Explain the match first
tw explain "git reset --hard HEAD"

# Exact command match for this session
tw override add allow --session "rm -rf ./dist"

# Allow for a fixed time window
tw override add allow --time 4h "git push --force"

# Require confirmation for a specific command during this session
tw override add ask --session "git push --force"

# Require confirmation for a specific matched rule for 2 hours
tw override add ask --time 2h --rule "core.git:reset-hard" --reason "review"

# Manage entries
tw override list
tw override remove sa-1a2b
tw override clear
```

Session and time-scoped entries are stored under `~/.tw/` and can be cleared with `tw override clear`.

## Custom Rules

Persistent policy lives in rules files:

- **Project:** `.tw/rules.json` (higher precedence)
- **User:** `~/.tw/rules.json`

Use `tw rule` for durable allow, deny, and ask behavior:

```sh
# Deny a structural command pattern
tw rule add deny --name no-rm-rf --command rm --flag "-rf" --arg-prefix "/" --reason "dangerous"

# Require confirmation for force-push unless --force-with-lease is used
tw rule add ask --name review-force-push --command git --subcommand push --flag "--force" --unless-flag "--force-with-lease" --reason "review force pushes"

# Permanently allow a structural command pattern
tw rule add allow --name allow-git-status --command git --subcommand status --reason "safe"

# Permanently allow a specific matched built-in rule
tw rule add allow --name allow-reset --rule "core.git:reset-hard" --reason "reviewed for this repo"

# Project-scoped rule
tw rule add deny --project --name no-prod-delete --command kubectl --subcommand delete --arg-contains production --reason "protect prod"

# Inspect the JSON entry before saving
tw rule add deny --name no-rm-rf --command rm --flag "-rf" --arg-prefix "/" --reason "dangerous" --dry-run --json
```

Rules now use structural matching instead of legacy regex-based deny/ask matching:

- `kind: "command"` matches against parsed command structure with `when` and optional `unless` conditions.
- `when.command` is required for command-kind rules.
- Other structural fields include `subcommand`, `flag`, `allFlags`, `argExact`, `argPrefix`, and `argContains`.
- `kind: "rule"` allows a matched built-in or custom rule by rule ID such as `core.git:reset-hard`.
- Legacy regex, exact, and prefix deny/ask rules are no longer used for pack matching; migrate them to `kind: "command"` rules.

Example command-kind rule entry:

```json
{
  "name": "no-rm-rf",
  "action": "deny",
  "kind": "command",
  "when": {
    "Command": ["rm"],
    "Flag": ["-rf"],
    "ArgPrefix": ["/"]
  },
  "reason": "dangerous",
  "severity": "medium",
  "explanation": "dangerous"
}
```

Example allow-by-rule entry:

```json
{
  "name": "allow-reset",
  "action": "allow",
  "kind": "rule",
  "pattern": "core.git:reset-hard",
  "reason": "reviewed"
}
```

## Configuration

`tw install` creates `~/.tw/config.json` if it does not exist. Use `tw config` to inspect the resolved configuration or `tw config --json` for machine-readable output.

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

For machine-readable pack review data, including applicability metadata, pattern names, and any project-specific additions already applied, use:

```sh
tw packs --json
```

### Project-Aware Pack Application

TW exposes pack applicability metadata so your agent can inspect the repo, decide which packs fit, and then add only that selected set.

Apply the pack IDs you want into user-local project state:

```sh
tw packs apply containers.docker cicd.github_actions
```

Comma-separated input works too:

```sh
tw packs apply containers.docker,cicd.github_actions
```

In scripts or other non-interactive contexts, skip the confirmation prompt:

```sh
tw packs apply containers.docker cicd.github_actions --yes
```

Use `--dry-run` to inspect what would change without writing:

```sh
tw packs apply containers.docker cicd.github_actions --dry-run
```

To remove project-local additions after confirming that change with the user:

```sh
tw packs remove containers.docker
```

Project-aware pack application only writes to user-local state under `~/.tw/projects/`. It does not modify repository-tracked config, and `packs.disabled` in `~/.tw/config.json` still takes precedence over selected additions.

## Project Policies

- Security policy and vulnerability reporting: [SECURITY.md](SECURITY.md)
- Contribution guide: [CONTRIBUTING.md](CONTRIBUTING.md)
- Code of conduct: [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)
- General bug reports and feature requests: GitHub issues

## Performance

| Path | Target |
|---|--------|
| Quick-reject (no keyword match) | <5us   |
| Full pipeline (keyword match → pack eval) | <20ms  |
| Absolute maximum (fail-open) | 200ms  |

TW fails open — if evaluation exceeds the timeout, the command is allowed. Safety should never block productivity.
