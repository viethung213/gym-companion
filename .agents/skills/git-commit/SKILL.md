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

## Conventional Commit with Gitmoji Format

Every commit message must begin with a Gitmoji followed by a space, then the conventional commit prefix:

```
<emoji> <type>(scope): <description>

[optional body]

[optional footer(s)]
```

Or using Gitmoji shortcodes (e.g. for environment compatibility):

```
:shortcode: <type>(scope): <description>
```

### Examples:
- `✨ feat(auth): add google login`
- `🐛 fix(db): resolve deadlock on transaction`
- `:pencil2: docs(readme): fix typos in installation steps`

## Commit Types and Gitmojis

Below is the mapping between Conventional Commit types and their corresponding Gitmojis (from [gitmoji.dev](https://gitmoji.dev)):

| Type | Primary Gitmoji | Direct Emoji | Shortcode | Purpose |
| :--- | :---: | :---: | :---: | :--- |
| `feat` | ✨ / 🚀 | `✨` / `🚀` | `:sparkles:` / `:rocket:` | New feature / Deploy stuff |
| `fix` | 🐛 / 🚑️ | `🐛` / `🚑️` | `:bug:` / `:ambulance:` | Bug fix / Critical hotfix |
| `docs` | 📝 | `📝` | `:memo:` | Documentation only |
| `style` | 🎨 / 💄 | `🎨` / `💄` | `:art:` / `:lipstick:` | Format (no logic) / UI and styles |
| `refactor` | ♻️ | `♻️` | `:recycle:` | Code refactor |
| `perf` | ⚡️ | `⚡️` | `:zap:` | Performance improvement |
| `test` | ✅ / 🧪 | `✅` / `🧪` | `:white_check_mark:` / `:test_tube:` | Add/update tests / Failing test |
| `build` | 📦️ / 📌 | `📦️` / `📌` | `:package:` / `:pushpin:` | Build system/dependencies |
| `ci` | 👷 / 💚 | `👷` / `💚` | `:construction_worker:` / `:green_heart:` | CI config / Fix CI build |
| `chore` | 🔧 / 🔨 | `🔧` / `🔨` | `:wrench:` / `:hammer:` | Maintenance/config/scripts |
| `revert` | ⏪️ | `⏪️` | `:rewind:` | Revert commit |

## Common Gitmoji Reference

| Emoji | Shortcode | Description |
| :---: | :--- | :--- |
| 🔒️ | `:lock:` | Fix security or privacy issues |
| 🔐 | `:closed_lock_with_key:` | Add or update secrets |
| 🔖 | `:bookmark:` | Release / Version tags |
| 🚧 | `:construction:` | Work in progress |
| ➕ | `:heavy_plus_sign:` | Add a dependency |
| ➖ | `:heavy_minus_sign:` | Remove a dependency |
| 🌐 | `:globe_with_meridians:` | Internationalization & localization |
| 💩 | `:poop:` | Write bad code that needs to be improved |
| 🔀 | `:twisted_rightwards_arrows:` | Merge branches |
| 👽️ | `:alien:` | Update code due to external API changes |
| 🚚 | `:truck:` | Move or rename resources/files |
| 📄 | `:page_facing_up:` | Add or update license |
| 💥 | `:boom:` | Introduce breaking changes |
| 🍱 | `:bento:` | Add or update assets |
| ♿️ | `:wheelchair:` | Improve accessibility |
| 💡 | `:bulb:` | Add or update comments in source code |
| 🍻 | `:beers:` | Write code drunkenly |
| 💬 | `:speech_balloon:` | Add or update text and literals |
| 🗃️ | `:card_file_box:` | Perform database related changes |
| 🔊 | `:loud_sound:` | Add or update logs |
| 🔇 | `:mute:` | Remove logs |
| 👥 | `:busts_in_silhouette:` | Add or update contributor(s) |
| 🚸 | `:children_crossing:` | Improve user experience / usability |
| 🏗️ | `:building_construction:` | Make architectural changes |
| 📱 | `:iphone:` | Work on responsive design |
| 🤡 | `:clown_face:` | Mock things |
| 🥚 | `:egg:` | Add or update an easter egg |
| 🙈 | `:see_no_evil:` | Add or update a .gitignore file |
| 📸 | `:camera_flash:` | Add or update snapshots |
| ⚗️ | `:alembic:` | Perform experiments |
| 🔍️ | `:mag:` | Improve SEO |
| 🏷️ | `:label:` | Add or update types |
| 🌱 | `:seedling:` | Add or update seed files |
| 🚩 | `:triangular_flag_on_post:` | Add, update, or remove feature flags |
| 🥅 | `:goal_net:` | Catch errors |
| 💫 | `:dizzy:` | Add or update animations and transitions |
| 🗑️ | `:wastebasket:` | Deprecate code that needs to be cleaned up |
| 🛂 | `:passport_control:` | Work on code related to authorization/roles |
| 🩹 | `:adhesive_bandage:` | Simple fix for a non-critical issue |
| 🧐 | `:monocle_face:` | Data exploration/inspection |
| ⚰️ | `:coffin:` | Remove dead code |
| 👔 | `:necktie:` | Add or update business logic |
| 🩺 | `:stethoscope:` | Add or update healthcheck |
| 🧱 | `:bricks:` | Infrastructure related changes |
| 🧑‍💻 | `:technologist:` | Improve developer experience |
| 💸 | `:money_with_wings:` | Add sponsorships or money related infrastructure |
| 🧵 | `:thread:` | Concurrency or multithreading |
| 🦺 | `:safety_vest:` | Validation checks |
| 🦖 | `:t-rex:` | Backwards compatibility |

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