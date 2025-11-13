#!/bin/bash
# Claude Code hook that runs golangci-lint on Go file changes
# This runs before processing each user prompt

set -e

# Only run if there are Go files in the repository
if ! find . -maxdepth 3 -name "*.go" -not -path "./vendor/*" -not -path "./example_repos/*" | grep -q .; then
    exit 0
fi

# Check if golangci-lint is installed
if ! command -v golangci-lint &> /dev/null; then
    echo "⚠️  golangci-lint is not installed. Skipping lint checks."
    exit 0
fi

# Get list of modified Go files (staged, unstaged, and untracked)
MODIFIED_GO_FILES=$(git diff --name-only HEAD 2>/dev/null | grep '\.go$' || true)
UNTRACKED_GO_FILES=$(git ls-files --others --exclude-standard | grep '\.go$' || true)
ALL_GO_FILES=$(echo -e "${MODIFIED_GO_FILES}\n${UNTRACKED_GO_FILES}" | grep -v '^$' | sort -u)

# If no Go files changed, exit successfully
if [ -z "$ALL_GO_FILES" ]; then
    exit 0
fi

echo "🔍 Running golangci-lint on modified Go files..."
echo "Files to check:"
echo "$ALL_GO_FILES" | sed 's/^/  - /'

# Run golangci-lint on the whole project (it will cache unchanged files)
if golangci-lint run --config .golangci.yml ./...; then
    echo "✅ golangci-lint passed!"
    exit 0
else
    echo ""
    echo "❌ golangci-lint found issues. Please fix them before proceeding."
    echo ""
    echo "To see detailed issues, run:"
    echo "  golangci-lint run ./..."
    echo ""
    echo "To auto-fix some issues, run:"
    echo "  golangci-lint run --fix ./..."
    exit 1
fi
