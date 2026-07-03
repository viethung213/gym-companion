---
name: pr-creator
description: Automatically writes high-quality Pull Request (PR) titles and descriptions in markdown. Trigger this skill when the user asks to write, generate, create, or format a Pull Request description, or mentions /pr-create or /pr.
---

# Pull Request Creator Skill

This skill generates a professional Pull Request title and description based on codebase changes, git status, git log, active conversation context, and testing history.

## Workflow

### Step 1: Gather Information & Analyze Changes
1. **Analyze Git Changes**: Run `git status` and `git diff` against the target branch (default `main`) to see changed files and detailed diffs.
2. **Review Commit History**: Run `git log -n 5` on the current branch to understand recent commits.
3. **Analyze Conversation Context**:
   - Determine which issue is being resolved from the current conversation context.
   - Match issue IDs or bug reports to link the PR.
   - If the issue to close cannot be determined automatically, prompt the user for clarification.
4. **Retrieve Test History**:
   - Look for test files matching modified source files (e.g. `test_*.py`, `*.test.ts`).
   - Check if any test/build commands were recently executed.

### Step 2: Normalize File Paths (CRITICAL)
> [!IMPORTANT]
> **Do NOT use local absolute paths** (e.g., `e:/LEAN/TTTN/src/main.js` or `C:\Users\ACER\...`).
> Convert all absolute paths to workspace-relative paths (e.g., `src/main.js`). Prefer formatting them as GitHub markdown relative links, e.g., `[src/main.js](src/main.js)`.

### Step 3: Format PR Title and Description
Generate the PR title and description in the user's preferred language (defaulting to Vietnamese if the user prompts in Vietnamese) using this exact template:

```markdown
## Proposed PR Title
[PR Type]: [Short summary of changes, e.g., "feat: add user login validation" or "fix: resolve memory leak in API"]

---

### 1. Problem to Solve
- Clear description of the problem, bug, or feature.
- Include auto-closing keywords for issues, e.g., `Closes #123`, `Fixes #456`.
- *Note: If no issue can be identified, ask the user or leave a placeholder.*

### 2. Proposed Solution
- Technical summary of the solution implemented.
- Rationale behind the chosen approach.

### 3. Changes & Impact
List modified, new, or deleted files using relative paths or markdown relative links:
- `[relative-path-1](relative-path-1)`: Explain the change in this file, how it solves the issue, and its impact.
- `[relative-path-2](relative-path-2)`: Change summary and impact.

### 4. Testing & Verification
Details of verification steps performed before creating the PR to prevent regressions:
- **Automated Tests**: List test commands run (e.g., `npm run test`, `pytest`) and their outcome (pass/fail).
- **Manual Verification**: Description of manual verification performed (UI checks, API requests via Postman/curl, logs check, etc.).
```

### Step 4: Present the Output
- Display the generated PR title and description wrapped inside a markdown code block (fenced with triple backticks) to make it easy for the user to copy.
- Do NOT create or overwrite files unless explicitly requested by the user.

