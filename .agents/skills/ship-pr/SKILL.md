---
name: ship-pr
description: Automatically pushes the current branch and creates/opens a Pull Request (PR) on GitHub using GitHub CLI (gh). Use when the user requests to ship, open, submit, or create a PR on GitHub, or mentions "/ship-pr" or "/open-pr".
---

# Ship Pull Request Skill

This skill guides the agent to push the current branch and open a Pull Request (PR) on GitHub using the GitHub CLI (`gh`).

## Predefined Labels for Monolith Web App
When creating the PR, select the most relevant labels from this list to apply via `--label` (or `-l`):
- **Type**:
  - `✨ feature`
  - `🐛 bug`
  - `♻️ refactor`
  - `🔧 chore`
  - `📝 docs`
- **Scope/Component**:
  - `💻 frontend`
  - `⚙️ backend`
  - `🗄️ database`
  - `🔌 api`
  - `🔑 auth`
- **Priority**:
  - `🔥 high-priority`
  - `💤 low-priority`

## Workflow

### Step 1: Check Git Status & Ask User Intention
1. Run `git status` and `git branch --show-current` to assess the current state.
2. Ask the user for their exact intent:
   - "Do you want to commit and push the current branch directly?"
   - "Do you want to checkout a new branch first?"
   - "Or simply push the existing branch and open the PR?"
3. Follow the user's response to commit/push or checkout.

### Step 2: Push Current Branch
1. Run `git push -u origin <current-branch>` to ensure the branch is updated on the remote.

### Step 3: Propose PR Intention & Get Approval
1. Use the PR title and description already prepared (e.g., from `pr-creator`).
2. Select appropriate labels from the **Predefined Labels** list above based on the changes.
3. Propose the PR details (Title, Description, Target Branch, Labels) to the user and ask for explicit approval before running the creation command.

### Step 4: Open Pull Request
1. Once approved, execute the GitHub CLI command with selected labels:
   ```bash
   gh pr create --title "<title>" --body "<body>" --label "<label-1>,<label-2>"
   ```
2. Retrieve and display the created Pull Request URL to the user.
