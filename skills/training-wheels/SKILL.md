---
name: training-wheels
description: >
  TW (Training Wheels) pre-execution safety hook. Denies destructive shell
  commands before execution. Use when a command is denied by TW, when
  suggesting potentially destructive shell commands, or when the user asks
  about TW configuration or allow rules.
license: MIT
metadata:
  author: thgrace
  version: "1.0"
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

1. **NEVER execute `tw allow` commands.** You may only suggest them to the user to run manually in their own terminal. TW denies `tw allow` when run through the hook — only the human user can modify safety rules.
2. **NEVER override a denial without explicit user approval.**
3. **Defer to the user** on all security decisions.

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
- That they can add an allow rule if needed (see step 3)

### 3. Suggest alternatives or allow rules

**Try a safer alternative first.** Common substitutions:

| Blocked command | Safe alternative |
|----------------|-----------------|
| `rm -rf ./worktree` | `git worktree remove ./worktree` |
| `rm -rf ./build` | `rm -rf /tmp/build` or add an override |
| `git reset --hard` | `git stash` then `git checkout <branch>` |
| `git push --force` | `git push --force-with-lease` |
| `git clean -fd` | `git clean -n` (dry run first) |
| `docker system prune -a` | `docker image prune` (targeted) |

**If no alternative works**, suggest the user run an allow command in their own terminal:

```sh
# Session-scoped allow (current agent session only)
tw allow --session "rm -rf ./dist"

# Time-scoped allow (expires after duration)
tw allow --time 30m "rm -rf ./build"

# Permanent allow
tw allow --permanent "rm -rf ./dist" --reason "Build output cleanup"

# Prefix-based allow (matches any command starting with this)
tw allow --permanent --prefix "rm -rf ./build" --reason "Build artifacts"

# Rule-based allow (disables a specific pattern)
tw allow --permanent --rule "core:rm-rf-aggressive" --reason "Needed for CI cleanup"
```

**Important:** You must NOT execute `tw allow` yourself — only suggest the command for the user to run manually. TW's self-protection denies `tw allow` when run through the hook.

### 4. NEVER weaken protections without user approval

**Do not suggest `tw allow` unless the user explicitly asks for it.** A denial
means Training Wheels determined the command is dangerous. Always defer to the user's
judgment before bypassing protection.

## Useful Commands

| Command | Purpose |
|---------|---------|
| `tw test "command"` | Check if a command would be denied (without executing it) |
| `tw explain "command"` | Detailed explanation of why a command is denied or allowed |
| `tw explain --json "command"` | Machine-readable explanation output |
| `tw allow --list` | List all active allow rules |
| `tw allow --session "command"` | Allow for current session only (suggest to user) |
| `tw allow --permanent "command"` | Permanent allow rule (suggest to user) |

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

- **Inline code is scanned:** `bash -c`, `sh -c`, `eval`, and similar
  wrappers are unpacked — the inner command is evaluated.
- **Temp directories are always safe:** `/tmp/*`, `/var/tmp/*`, and
  `$TMPDIR/*` paths are never denied for filesystem operations.
- **`sudo` and `env` are stripped:** `sudo rm -rf /` is evaluated as
  `rm -rf /` — the wrapper doesn't bypass detection.
- **Self-protection:** `tw allow` commands are denied when run through the
  hook. Only the human user can modify allow rules directly in their terminal.
- **Config location:** Project config is `.tw.json` at the project root.
  Allow rules are at `.tw/overrides.json` (project) or
  `~/.tw/overrides.json` (user-level).

## Behavioral Rules

1. **NEVER execute `tw allow` commands** — only suggest them to the user.
2. **NEVER weaken protections without explicit user approval.**
3. **NEVER attempt to bypass a TW denial.** When a command is denied, do NOT
   try to circumvent the protection by:
   - Base64-encoding the command (e.g., `echo "cm0gLXJmIC8=" | base64 -d | sh`)
   - Hex-encoding or other encoding schemes
   - Writing the denied command to a script file and executing the script
   - Using `eval`, `source`, or similar indirection to obscure the command
   - Splitting the command across multiple steps to avoid pattern detection
   - Using variable expansion or string concatenation to assemble the command
   - Invoking an interpreter (python, perl, ruby, node) to run the equivalent operation
   - Any other obfuscation or evasion technique

   A denial means the command is dangerous. The correct response is to find a
   safer alternative or ask the user — never to work around the safety check.
4. **Use `tw test` to pre-check** potentially destructive commands before
   suggesting them.
5. **Use `tw explain` when a denial occurs** to provide informed guidance.
6. **Prefer safer alternatives** over allow rules.
7. **Defer to the user** on all security decisions.
