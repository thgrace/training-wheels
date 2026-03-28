---
name: training-wheels
description: >
  TW (Training Wheels) pre-execution safety hook. Denies destructive shell
  commands before execution. Use when a command is denied by TW, when
  suggesting potentially destructive shell commands, or when the user asks
  about TW configuration or override rules.
license: MIT
metadata:
  author: thgrace
  version: "2.0"
compatibility: Requires the tw binary installed and in PATH.
hooks:
  PreToolUse:
    - matcher: "Bash"
      hooks:
        - type: command
          command: "tw hook"
---

# Training Wheels (TW) — Agent Skill

Training Wheels is a pre-execution safety hook for AI coding agents. It **evaluates but
never executes** shell commands, denying destructive operations before they
run. Training Wheels is installed as a hook in your agent configuration and runs
automatically on every shell command.

## Critical Rules

1. **NEVER execute `tw override add allow` or `tw rule add allow` or `tw rule remove` commands.** You may only suggest them to the user to run manually. TW denies these when run through the hook — only the human user can loosen safety rules.
2. **You MAY execute `tw rule add deny` and `tw rule add ask`** — these tighten security.
3. **NEVER override a denial without explicit user approval.**
4. **Defer to the user** on all security decisions.

## What Gets Denied

Training Wheels denies commands matching patterns in its enabled pack set. Common examples:

| Category | Examples |
|----------|----------|
| Filesystem | `rm -rf /`, `rm -rf ~`, `rm -rf .` (non-temp paths) |
| Git | `git reset --hard`, `git push --force`, `git clean -fd`, `git checkout .` |
| Database | `DROP TABLE`, `DROP DATABASE`, `TRUNCATE`, `DELETE FROM` without `WHERE` |
| Containers | `docker system prune -a --force`, `docker rm -f $(docker ps -aq)` |
| Kubernetes | `kubectl delete namespace`, `kubectl delete --all` |
| Infrastructure | `terraform destroy`, `terraform apply -auto-approve` |
| System | `mkfs`, `dd if=`, `chmod -R 777 /`, `:(){:|:&};:` |

## Context-Aware Exceptions

Training Wheels understands shell context. These are **allowed** because the
destructive string appears in a data position, not as an executed command:

- `git commit -m "rm -rf cleanup"` — commit messages are data
- `echo "DROP TABLE users"` — echo arguments are data
- `grep "git reset --hard" file.txt` — search patterns are data
- `# rm -rf /` — comments are ignored
- Commands targeting `/tmp`, `/var/tmp`, or `$TMPDIR` — temp dirs are safe

**Inline code IS scanned:** `bash -c "rm -rf /"` will be denied because the
argument to `bash -c` is executed code.

## When a Command Is Denied

Follow these steps when Training Wheels denies a command:

### 1. Understand the denial

Run `tw explain` to get the full details:

```sh
tw explain "the denied command"
```

This shows: which rule denied it, the severity, the normalized form Training Wheels
evaluated, and any suggested alternatives.

### 2. Inform the user

Tell the user:
- Which rule denied the command and why
- The safer alternatives suggested by `tw explain`, if any
- That they can add an override if needed (see step 3)

### 3. Suggest alternatives or overrides

**Try a safer alternative first.** Common substitutions:

| Blocked command | Safe alternative |
|----------------|-----------------|
| `rm -rf ./worktree` | `git worktree remove ./worktree` |
| `rm -rf ./build` | `rm -rf /tmp/build` or add an override |
| `git reset --hard` | `git stash` then `git checkout <branch>` |
| `git push --force` | `git push --force-with-lease` |
| `git clean -fd` | `git clean -n` (dry run first) |
| `docker system prune -a` | `docker image prune` (targeted) |

**If no alternative works**, suggest the user run an override or rule command in their own terminal:

```sh
# Session-scoped allow (current agent session only)
tw override add allow --session "rm -rf ./dist"

# Permanent allow (via tw rule — all permanent policy uses tw rule)
tw rule add allow --name "allow-dist-cleanup" --command "rm" --flag "-rf" --arg-exact "./dist" --reason "Build output cleanup"

# Rule-based allow (disables a specific pack pattern permanently)
tw rule add allow --name "allow-reset-hard" --rule "core.git:reset-hard" --reason "Needed for CI"

# Permanent ask (requires confirmation)
tw rule add ask --name "review-force-push" --command "git" --subcommand "push" --flag "--force" --unless-flag "--force-with-lease" --reason "Require human review"
```

**Important:** You must NOT execute `tw override add allow`, `tw rule add allow`, or `tw rule remove` yourself — only suggest them for the user to run manually. You MAY execute `tw rule add deny` and `tw rule add ask` since these tighten security.

### 4. NEVER weaken protections without user approval

**Do not suggest `tw override` unless the user explicitly asks for it.** A denial
means Training Wheels determined the command is dangerous. Always defer to the user's
judgment before bypassing protection.

## Useful Commands

| Command | Purpose |
|---------|---------|
| `tw test "command"` | Check if a command would be denied (without executing it) |
| `tw explain "command"` | Detailed explanation of why a command is denied or allowed |
| `tw explain --json "command"` | Machine-readable explanation output |
| `tw packs --json` | List pack metadata and current enablement |
| `tw override list` | List all active session/time overrides |
| `tw override add allow --session "command"` | Allow for current session only (suggest to user) |
| `tw rule add deny --name N --command C --reason R` | Add a permanent deny rule (you may execute) |
| `tw rule add allow --name N --rule ID --reason R` | Permanent allow by rule ID (suggest to user) |
| `tw rule list` | List all custom rules |

## Pre-checking Commands

Before suggesting a potentially destructive command, use `tw test` to check
if it will be allowed:

```sh
tw test "your command here"
# Exit code 0 = allowed, 1 = denied, 2 = warn
```

This avoids the disruptive flow of suggesting a command, having it denied,
then needing to explain and find alternatives.

## Gotchas

- **Structural Matching:** TW uses an AST parser (tree-sitter) to understand command structure. It doesn't just use regex; it understands commands, subcommands, flags, and arguments.
- **Inline code is scanned:** `bash -c`, `sh -c`, `eval`, and similar
  wrappers are unpacked — the inner command is evaluated.
- **Temp directories are always safe:** `/tmp/*`, `/var/tmp/*`, and
  `$TMPDIR/*` paths are never denied for filesystem operations.
- **`sudo` and `env` are stripped:** `sudo rm -rf /` is evaluated as
  `rm -rf /` — the wrapper doesn't bypass detection.
- **Self-protection:** `tw override add allow`, `tw rule add allow`, and `tw rule remove` commands are denied when run through the
  hook. Only the human user can loosen safety rules directly in their terminal.
  You may run `tw rule add deny` and `tw rule add ask`.
- **Config location:** Global config is `~/.tw/config.json`.
- **Override scope:** `tw override` is session/time-scoped only — no permanent
  overrides. All permanent policy (deny, ask, allow) goes through `tw rule`.

## Behavioral Rules

1. **NEVER execute `tw override add allow`, `tw rule add allow`, or `tw rule remove`** — only suggest them to the user. You MAY execute `tw rule add deny` and `tw rule add ask`.
2. **NEVER weaken protections without explicit user approval.**
3. **NEVER attempt to bypass a TW denial.** When a command is denied, do NOT
   try to circumvent the protection by:
   - Base64-encoding the command
   - Hex-encoding or other encoding schemes
   - Writing the denied command to a script file and executing the script
   - Using `eval`, `source`, or similar indirection
   - Splitting the command across multiple steps
   - Using variable expansion or string concatenation
   - Invoking an interpreter (python, perl, ruby, node) to run the equivalent operation
   - Any other obfuscation or evasion technique

   A denial means the command is dangerous. The correct response is to find a
   safer alternative or ask the user — never to work around the safety check.
4. **Use `tw test` to pre-check** potentially destructive commands before
   suggesting them.
5. **Use `tw explain` when a denial occurs** to provide informed guidance.
6. **Prefer safer alternatives** over override rules.
7. **Defer to the user** on all security decisions.
