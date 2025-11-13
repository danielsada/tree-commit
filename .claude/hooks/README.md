# Claude Code Hooks

This directory contains hooks that run during Claude Code sessions to ensure code quality.

## Installed Hooks

### `user-prompt-submit-hook.sh`

Runs **before** each user prompt is processed by Claude.

**What it does:**
- Detects modified or new Go files
- Runs `golangci-lint` on the entire project
- Blocks Claude from proceeding if linting issues are found

**Configuration:**
- Lint rules are defined in `.golangci.yml` at the project root
- Includes comprehensive checks:
  - Complexity analysis (cyclomatic, cognitive, function length)
  - Error handling
  - Code quality and style
  - Security issues (gosec)
  - Performance optimizations
  - Bug detection

## Managing Hooks

### Temporarily Disable Hooks

If you need to bypass the linting hook temporarily:

```bash
# Set environment variable
export SKIP_CLAUDE_HOOKS=1
```

### Run Linter Manually

```bash
# Check all issues
golangci-lint run ./...

# Auto-fix issues
golangci-lint run --fix ./...

# Run specific linter
golangci-lint run --disable-all --enable=gocyclo ./...
```

### Update Lint Configuration

Edit `.golangci.yml` to adjust:
- Complexity thresholds
- Enabled/disabled linters
- Excluded paths or rules

## Why These Hooks?

1. **Catch issues early**: Find bugs, complexity, and style issues before they're committed
2. **Consistent code quality**: Enforce standards across all AI-generated and human-written code
3. **Educational**: See lint errors and learn best practices
4. **Fast feedback**: Run on every change, not just at commit time

## Debugging Hooks

If a hook is failing unexpectedly:

```bash
# Run the hook manually
bash -x .claude/hooks/user-prompt-submit-hook.sh

# Check hook output
echo $?  # Exit code (0 = success, 1 = failure)
```
