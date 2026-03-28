---
name: compose-rule
description: >
  Guide for composing custom TW rules. Use when the user wants to create
  deny, ask, or allow rules for commands that TW should block, require
  confirmation for, or explicitly permit.
license: MIT
metadata:
  author: thgrace
  version: "2.0"
compatibility: Requires tw >= 1.0 with structural rule support.
---

# Compose Rule — TW Custom Rule Builder

This skill guides you through creating custom TW rules using `tw rule add`.

## When to Use

- User wants to deny a specific command pattern
- User wants to require confirmation (ask) for certain commands
- User wants to permanently allow a command that TW currently blocks
- User wants to disable a specific built-in pack rule by ID

## Step 1: Clarify Intent

Ask the user:
- What command(s) should be affected?
- What should happen? (deny, ask, or allow)
- Are there exceptions? (e.g., deny `gh pr create` except with `--draft`)

## Step 2: Identify Structural Conditions

Identify the structural components of the command to match:
- **Command:** The base command (e.g., `git`, `rm`, `gh`)
- **Subcommand:** The subcommand if applicable (e.g., `push`, `reset`, `pr create`)
- **Flag:** Any flag that should trigger the rule (e.g., `--force`, `-rf`)
- **Arg Prefix/Exact:** Positional arguments to match (e.g., `/` for root, `~` for home)
- **Exemptions:** Flags or arguments that make the command safe (e.g., `--draft`, `--force-with-lease`)

## Step 3: Validate with `tw test`

Test your proposed conditions against example commands:

```sh
# Should be denied:
tw test "gh pr create --title 'My PR'"

# Should be allowed (safe exception):
tw test "gh pr create --draft --title 'My PR'"
```

## Step 4: Generate the Command

### Deny rule (with exemption)
```sh
tw rule add deny \
  --name "require-draft-pr" \
  --command "gh" \
  --subcommand "pr" \
  --subcommand "create" \
  --unless-flag "--draft" \
  --reason "PRs must be created as drafts" \
  --severity medium \
  --explanation "Non-draft PRs trigger reviews prematurely" \
  --suggest "gh pr create --draft||Create a draft pull request"
```

### Ask rule (require confirmation)
```sh
tw rule add ask \
  --name "review-force-push" \
  --command "git" \
  --subcommand "push" \
  --flag "--force" \
  --unless-flag "--force-with-lease" \
  --reason "Force push requires human confirmation" \
  --severity high
```

### Allow rule (structural)
```sh
tw rule add allow \
  --name "allow-dist-cleanup" \
  --command "rm" \
  --flag "-rf" \
  --arg-exact "./dist" \
  --reason "Build output cleanup"
```

### Allow rule (by rule ID)
```sh
tw rule add allow \
  --name "allow-reset-hard" \
  --rule "core.git:reset-hard" \
  --reason "Needed for CI workflow"
```

## Step 5: Execute or Suggest

- You MAY execute `tw rule add deny` and `tw rule add ask` yourself — these tighten security.
- You MUST only suggest `tw rule add allow` and `tw rule remove` to the user — these loosen security.

## Management Commands

```sh
tw rule list              # Show all rules (user + project)
tw rule list --json       # JSON output
tw rule remove <name> --yes  # Remove a rule
tw rule add ... --dry-run # Preview JSON without saving
```

## Flags Reference (`tw rule add`)

| Flag | Description |
|------|-------------|
| `--name` | Unique rule name (lowercase, hyphens, underscores) |
| `--command` | Base command name (repeatable, OR) |
| `--subcommand` | Subcommand to match (repeatable, OR) |
| `--flag` | Any flag triggers match (repeatable, OR) |
| `--all-flags` | All flags must be present (repeatable, AND) |
| `--arg-exact` | Arg equals value (repeatable, OR) |
| `--arg-prefix` | Arg starts with value (repeatable, OR) |
| `--arg-contains`| Arg contains value (repeatable, OR) |
| `--unless-flag` | Exempt if flag present (repeatable) |
| `--unless-arg` | Exempt if arg equals value (repeatable) |
| `--rule` | Match by rule-ID (allow action only) |
| `--reason` | Human-readable reason |
| `--severity` | critical/high/medium/low (default: medium) |
| `--explanation` | Detailed explanation (default: same as reason) |
| `--suggest` | Safer alternative: "cmd\|\|description" (repeatable) |
| `--project` | Write to project-level rules |
| `--dry-run` | Preview JSON without saving |
