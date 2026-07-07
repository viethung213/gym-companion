#!/usr/bin/env bash

# Commit message linter for Conventional Commits with Gitmoji
# Pattern: <emoji> <type>(scope): <description>
# Or: :shortcode: <type>(scope): <description>

set -e

# Force C locale to run grep in raw byte mode, avoiding multibyte surrogate pair bugs in Git Bash on Windows
export LC_ALL=C
export LANG=C

# Regex pattern:
# - ^(:[a-zA-Z0-9_-]+:|[^a-zA-Z[:space:]][^[:space:]]*) matches any shortcode or unicode icon/emoji
# - [[:space:]]+ matches whitespace between emoji and type
# - [a-zA-Z0-9_-]+ matches type (feat, fix, docs, style, etc.)
# - (\([^)]+\))? matches optional scope
# - !? matches optional breaking change indicator
# - :[[:space:]]+ matches colon and whitespace
# - .+$ matches description
REGEX="^(:[a-zA-Z0-9_-]+:|[^a-zA-Z[:space:]][^[:space:]]*)[[:space:]]+[a-zA-Z0-9_-]+(\([^)]+\))?!?:[[:space:]]+.+$"

# Clean carriage return from REGEX variable in case the script file is saved with CRLF line endings on Windows
REGEX=$(echo "$REGEX" | tr -d '\r')

validate_msg() {
  local msg="$1"
  # Trim leading/trailing whitespace safely without breaking on quotes/apostrophes
  msg=$(echo "$msg" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')

  if echo "$msg" | grep -Eq "$REGEX"; then
    return 0
  else
    echo "❌ Invalid commit message: '$msg'"
    echo "   Expected format: <emoji> <type>(scope): <description>"
    echo "   Or shortcode:    :shortcode: <type>(scope): <description>"
    echo "   Example:         ✨ feat(auth): add google login"
    echo "                    :sparkles: feat(auth): add google login"
    return 1
  fi
}

# Mode 1: Check a commit message file (Git commit-msg hook)
if [ -n "$1" ] && [ -f "$1" ]; then
  # Extract first non-comment, non-empty line (the commit subject) and strip CR character
  COMMIT_MSG=$(grep -v '^[[:space:]]*#' "$1" | tr -d '\r' | sed '/^[[:space:]]*$/d' | head -n 1 || echo "")
  if [ -z "$COMMIT_MSG" ]; then
    exit 0
  fi
  validate_msg "$COMMIT_MSG"
  exit $?
fi

# Mode 2: Check range of commits or HEAD
RANGE=""
if [ -n "$COMMIT_RANGE" ]; then
  RANGE="$COMMIT_RANGE"
elif [ -n "$1" ]; then
  RANGE="$1"
fi

if [ -n "$RANGE" ]; then
  echo "Checking commits in range: $RANGE"
  FAILED=0
  while IFS= read -r msg; do
    [ -z "$msg" ] && continue
    if ! validate_msg "$msg"; then
      FAILED=1
    fi
  done < <(git log --format=%s "$RANGE")

  if [ $FAILED -ne 0 ]; then
    echo "❌ Some commit messages do not follow the Gitmoji Conventional Commits specification."
    exit 1
  else
    echo "✨ All commit messages in range are valid!"
    exit 0
  fi
else
  LAST_COMMIT_MSG=$(git log -1 --format=%s HEAD 2>/dev/null || echo "")
  if [ -z "$LAST_COMMIT_MSG" ]; then
    echo "No commits found to validate."
    exit 0
  fi
  echo "Checking last commit (HEAD): $LAST_COMMIT_MSG"
  validate_msg "$LAST_COMMIT_MSG"
  exit $?
fi
