# Conflict Resolution

## Hook ID Conflicts

When multiple configuration files define hooks with the same `id`, tree-commit uses a topmost-wins strategy for conflict resolution.

### Resolution Strategy

**Topmost Priority**: If multiple hooks share the same `id`, only the hook defined in the topmost (closest to repository root) configuration file will be executed. All other hooks with the same `id` are skipped.

### Example

Given this directory structure:
```
repo/
    .tree-commit-config.yaml
 frontend/
    .tree-commit-config.yaml
 backend/
     .tree-commit-config.yaml
```

If all three configuration files define a hook with `id: "eslint"`:

- `repo/.tree-commit-config.yaml`  `eslint` hook **EXECUTES**
- `frontend/.tree-commit-config.yaml`  `eslint` hook **SKIPPED**
- `backend/.tree-commit-config.yaml`  `eslint` hook **SKIPPED**

### Rationale

This approach ensures:
1. **Predictable behavior** - The topmost configuration takes precedence
2. **Override capability** - Project-wide settings can be overridden at the root level
3. **Simplicity** - Clear hierarchy without complex merging rules

## Repository Version Conflicts

### Same Repository, Different Revisions

Multiple configurations can reference the same external repository with different revisions, as long as they have different hook IDs:

```yaml
# frontend/.tree-commit-config.yaml
repos:
  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.4.0
    hooks:
      - id: frontend-trailing-whitespace

# backend/.tree-commit-config.yaml
repos:
  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.5.0
    hooks:
      - id: backend-trailing-whitespace
```

**Resolution**: Both versions are installed and both hooks execute.

## Execution Strategy

### Parallel vs Serial Execution

- **Non-overlapping hooks**: Run in parallel for performance
- **Overlapping hooks**: Run serially to prevent file conflicts

**File Overlap Detection**: Hooks are considered overlapping if their file patterns could match the same files based on:
- `files` regex patterns
- `include_folders` and `exclude_folders` specifications

### Example

```yaml
# These run in PARALLEL (different file patterns)
- id: js-linter
  files: \.js$
- id: py-linter
  files: \.py$

# These run SERIALLY (overlapping patterns)
- id: formatter
  files: \.(js|ts)$
- id: linter
  files: \.ts$
```

## Other Conflict Scenarios

### Configuration Validation
- Invalid YAML syntax → Configuration file is skipped with warning
- Missing required hook fields → Hook is skipped with warning
- Invalid repository URLs → Repository installation fails, hooks are skipped

### Runtime Conflicts
- File permission errors → Hook fails but others continue
- Missing dependencies → Hook fails but others continue
- Network failures during repo installation → Affected hooks are skipped

### Best Practices

To minimize conflicts:
- Use unique hook IDs across different directories
- Place project-wide hooks in the root configuration
- Use descriptive IDs like `frontend-eslint` instead of just `eslint`
- Design file patterns to minimize overlap when performance is critical
- Pin repository revisions to avoid unexpected changes