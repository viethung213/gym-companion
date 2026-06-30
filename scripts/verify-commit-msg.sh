#!/usr/bin/env bash

# Commit message linter for Conventional Commits with Gitmoji
# Pattern: <emoji> <type>(scope): <description>
# Or: :shortcode: <type>(scope): <description>

# Exit on any error
set -e

# Regex pattern:
# - ^([^[:space:][:alnum:]]+|:[a-zA-Z0-9_-]+:) matches the emoji character or a shortcode like :sparkles:
# - [[:space:]]+ matches the whitespace between emoji and conventional commit type
# - [a-zA-Z0-9_-]+ matches type (feat, fix, etc.)
# - (\([^)]+\))? matches optional scope
# - !? matches optional breaking change indicator
# - :[[:space:]]+ matches the colon and whitespace
# - .+$ matches the description
REGEX="^([^[:space:][:alnum:]]+|:[a-zA-Z0-9_-]+:)[[:space:]]+[a-zA-Z0-9_-]+(\([^)]+\))?!?:[[:space:]]+.+$"

validate_msg() {
  local msg="$1"
  # Trim leading/trailing whitespace
  msg=$(echo "$msg" | xargs)

  # Check if message matches the regex
  # We use grep with ERE (Extended Regular Expressions)
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

# Mode 1: Check a single commit message file (Git commit-msg hook)
if [ -n "$1" ] && [ -f "$1" ]; then
  COMMIT_MSG=$(cat "$1")
  # Ignore empty messages
  if [ -z "$COMMIT_MSG" ]; then
    exit 0
  fi
  validate_msg "$COMMIT_MSG"
  exit $?
fi

# Mode 2: Check range of commits (CI environment or manual run)
# Use BASE_REF or check HEAD by default
RANGE=""
if [ -n "$COMMIT_RANGE" ]; then
  RANGE="$COMMIT_RANGE"
elif [ -n "$1" ]; then
  RANGE="$1"
fi

if [ -n "$RANGE" ]; then
  echo "Checking commits in range: $RANGE"
  # Get commit subjects in range
  FAILED=0
  while IFS= read -r msg; do
    # Skip empty lines
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
  # Default: Check the last commit (HEAD)
  LAST_COMMIT_MSG=$(git log -1 --format=%s HEAD 2>/dev/null || echo "")
  if [ -z "$LAST_COMMIT_MSG" ]; then
    echo "No commits found to validate."
    exit 0
  fi
  echo "Checking last commit (HEAD): $LAST_COMMIT_MSG"
  validate_msg "$LAST_COMMIT_MSG"
  exit $?
fi
