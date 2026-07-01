---
name: git-commit
description: 'Execute git commit with conventional commit message analysis, intelligent staging, and message generation. Use when user asks to commit changes, create a git commit, or mentions "/commit". Supports: (1) Auto-detecting type and scope from changes, (2) Generating conventional commit messages from diff, (3) Interactive commit with optional type/scope/description overrides, (4) Intelligent file staging for logical grouping'
license: MIT
allowed-tools: Bash
source: https://github.com/github/awesome-copilot/blob/main/skills/git-commit/SKILL.md
---

# Git Commit with Conventional Commits

## Overview

Create standardized, semantic git commits using the Conventional Commits specification. Analyze the actual diff to determine appropriate type, scope, and message.

## Commit Format

Every commit message must begin with a Gitmoji followed by a space, then the conventional commit prefix:

```
<emoji> <type>(scope): <description>

[optional body]

[optional footer(s)]
```

### Examples:

- Single-line commit:
  `✨ feat(auth): add google login`
- Multi-line commit with body and footer:
  ```
  🐛 fix(db): resolve deadlock on transaction

  The database was experiencing deadlocks when concurrent transactions tried to update the user balance. Wrapping the update query in a row lock resolves this issue.

  Closes #42
  ```
- Reference commit (no conventional emoji matched, or emoji not used):
  `docs(readme): update installation steps`

## Conventional Commit Types and Gitmojis

Below is the mapping for conventional types. If one of these types is used, prepend the primary emoji:

| Type | Emoji | Description |
| :--- | :---: | :--- |
| `feat` | `✨` | New feature |
| `fix` | `🐛` | Bug fix |
| `docs` | `📝` | Documentation changes |
| `style` | `🎨` | Formatting, UI, style changes (no logic changes) |
| `refactor` | `♻️` | Code refactoring |
| `perf` | `⚡️` | Performance improvements |
| `test` | `✅` | Adding or updating tests |
| `build` | `📦️` | Build system or external dependencies |
| `ci` | `👷` | CI configuration and scripts |
| `chore` | `🔧` | General maintenance, chores, config changes |
| `revert` | `⏪️` | Reverting a previous commit |

### Specific Scopes/Use-cases (Optional but Recommended)

For more specific scenarios, use these combinations:

| Type | Emoji | Description |
| :--- | :---: | :--- |
| `chore(deps)` | `➕` / `➖` / `⬆️` | Add, remove, or upgrade dependencies |
| `fix(security)` | `🔒️` | Fix security issues / secrets management |
| `chore(db)` | `🗃️` | Database migrations / database related changes |
| `chore(release)` | `🔖` | Release / Version tags |
| `chore(wip)` | `🚧` | Work in progress |
| `style(ui)` | `💄` | UI layout, CSS, and aesthetic design changes |
| `refactor(cleanup)` | `🔥` | Remove dead code or files |
| `chore(locales)` | `🌐` | Internationalization / translation updates |

## Workflow

### 1. Analyze Diff

```bash
# If files are staged, use staged diff
git diff --staged

# If nothing staged, use working tree diff
git diff

# Also check status
git status --porcelain
```

### 2. Stage Files (if needed)

If nothing is staged or you want to group changes differently:

```bash
# Stage specific files
git add path/to/file1 path/to/file2

# Stage by pattern
git add *.test.*
git add src/components/*

# Interactive staging
git add -p
```

**Never commit secrets** (.env, credentials.json, private keys).

### 3. Generate Commit Message

Analyze the diff to determine:

- **Gitmoji**: Select the most appropriate emoji or shortcode representing the change.
- **Type**: What kind of change is this (must map to Conventional Commits)?
- **Scope**: What area/module is affected? (Mandatory)
- **Description**: One-line summary of what changed (present tense, imperative mood, <72 chars)

Prepend the chosen Gitmoji to the conventional commit message (e.g. `✨ feat(auth): add login`).

### 4. Execute Commit

```bash
# Single line (using Unicode Emoji)
git commit -m "✨ feat(scope): <description>"

# Single line (using shortcode if preferred or terminal issues)
git commit -m ":sparkles: feat(scope): <description>"

# Multi-line with body/footer
git commit -m "$(cat <<'EOF'
✨ feat(scope): <description>

<optional body>

<optional footer>
EOF
)"
```

## Best Practices

- One logical change per commit
- **Gitmoji is mandatory**: Every commit must begin with an emoji or shortcode.
- **Scope is mandatory**: Always provide a scope in parentheses, e.g., `feat(auth): add login`
- Present tense: "add" not "added"
- Imperative mood: "fix bug" not "fixes bug"
- Reference issues: `Closes #123`, `Refs #456`
- Keep description under 72 characters

## Git Safety Protocol

- NEVER update git config
- NEVER run destructive commands (--force, hard reset) without explicit request
- NEVER skip hooks (--no-verify) unless user asks
- NEVER force push to main/master
- If commit fails due to hooks, fix and create NEW commit (don't amend)