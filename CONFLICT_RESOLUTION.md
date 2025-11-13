# Conflict Resolution Strategy

## Overview

tree-commit uses a **scope-based execution model** with **closest-wins** conflict resolution. Hooks only run on files within their scope, and nested configurations can override parent configurations for maximum flexibility and performance.

## Core Principles

### 1. Scope-Based Execution

**Rule**: A hook only runs if its configuration file is an ancestor directory of the changed file.

```
repo/
  .tree-commit-config.yaml      (Hook A)
  frontend/
    .tree-commit-config.yaml    (Hook B)
    src/App.js                  (CHANGED)
  backend/
    .tree-commit-config.yaml    (Hook C)
    api.py                      (CHANGED)
```

**For changed file `frontend/src/App.js`**:
- ✅ Hook A runs (ancestor: `repo/`)
- ✅ Hook B runs (ancestor: `repo/frontend/`)
- ❌ Hook C does NOT run (not an ancestor)

**For changed file `backend/api.py`**:
- ✅ Hook A runs (ancestor: `repo/`)
- ❌ Hook B does NOT run (not an ancestor)
- ✅ Hook C runs (ancestor: `repo/backend/`)

**Benefit**: Avoids running irrelevant hooks, improving performance and reducing noise.

---

### 2. Relative Path Evaluation

**Rule**: File patterns (`files` regex) in nested configs are evaluated **relative to the config's directory**, not the repository root.

```
repo/frontend/.tree-commit-config.yaml:
  hooks:
    - id: eslint
      files: \.js$        # Matches relative to frontend/
                          # Changed file: frontend/src/App.js
                          # Pattern tests against: src/App.js ✅
```

**Example**:

```yaml
# repo/frontend/.tree-commit-config.yaml
repos:
  - repo: local
    hooks:
      - id: eslint
        files: \.js$
        # For changed file: repo/frontend/src/components/Button.js
        # Pattern matches against: src/components/Button.js
```

**Benefit**: Config files are portable and work the same regardless of repository structure.

---

### 3. Closest-Wins Conflict Resolution

**Rule**: When multiple ancestor configs define hooks with the same `id`, the hook from the **closest** config to the changed file wins.

```
repo/
  .tree-commit-config.yaml
    - id: eslint (entry: eslint --max-warnings 10)
  frontend/
    .tree-commit-config.yaml
      - id: eslint (entry: eslint --max-warnings 0)  ← WINS
    src/App.js (CHANGED)
```

**Resolution for `frontend/src/App.js`**:
- Both configs are ancestors
- Both define hook `id: eslint`
- `frontend/.tree-commit-config.yaml` is closer → **its `eslint` hook executes**
- Root `eslint` is skipped

**Rationale**:
- Allows teams to override project-wide defaults
- More specific configurations take precedence
- Intuitive: "closer to code = more specific = higher priority"

---

### 4. Smart Parallel Execution

**Rule**: Hooks run in parallel unless they operate on the same file(s).

Overlap detection is based on **actual file matching**, not static regex analysis:

1. Build a map of `hook → [files it will operate on]`
2. Group hooks by file overlap
3. Hooks operating on disjoint file sets run in parallel
4. Hooks operating on overlapping file sets run serially

**Example**:

```
Changed files:
  - frontend/src/App.js
  - backend/api.py

Hooks and their matched files:
  - eslint      → [frontend/src/App.js]
  - prettier    → [frontend/src/App.js]
  - black       → [backend/api.py]
  - whitespace  → [frontend/src/App.js, backend/api.py]

Execution plan:
  [Parallel Group 1]
    - black (only touches backend/api.py)

  [Serial Group for frontend/src/App.js]
    - whitespace (touches frontend/src/App.js)
    - eslint (touches frontend/src/App.js)
    - prettier (touches frontend/src/App.js)

  Note: whitespace also touches backend/api.py, so it runs
        serially with black as well
```

**Benefits**:
- Maximizes parallelism for performance
- No file corruption from concurrent writes
- No complex regex overlap analysis needed

---

## Complete Example

### Repository Structure

```
repo/
  .tree-commit-config.yaml
  frontend/
    .tree-commit-config.yaml
    src/
      App.js        (CHANGED)
      utils.js      (CHANGED)
  backend/
    .tree-commit-config.yaml
    api.py          (CHANGED)
  docs/
    README.md       (CHANGED)
```

### Configuration Files

```yaml
# repo/.tree-commit-config.yaml
repos:
  - repo: local
    hooks:
      - id: trailing-whitespace
        entry: remove-trailing-whitespace
        files: .*

      - id: eslint
        entry: eslint --max-warnings 10
        files: \.js$

# repo/frontend/.tree-commit-config.yaml
repos:
  - repo: local
    hooks:
      - id: eslint
        entry: eslint --max-warnings 0 --config .eslintrc.frontend.js
        files: \.js$
        # Relative to frontend/: matches src/App.js, src/utils.js

      - id: prettier
        entry: prettier --write
        files: \.js$

# repo/backend/.tree-commit-config.yaml
repos:
  - repo: local
    hooks:
      - id: black
        entry: black
        files: \.py$
        # Relative to backend/: matches api.py

      - id: flake8
        entry: flake8
        files: \.py$
```

### Resolution Process

#### Step 1: Scope Resolution

For each changed file, find ancestor configs:

- `frontend/src/App.js`:
  - Ancestors: `repo/`, `repo/frontend/`

- `frontend/src/utils.js`:
  - Ancestors: `repo/`, `repo/frontend/`

- `backend/api.py`:
  - Ancestors: `repo/`
  - Ancestors: `repo/`, `repo/backend/`

- `docs/README.md`:
  - Ancestors: `repo/`

#### Step 2: Collect Applicable Hooks

For `frontend/src/App.js` and `frontend/src/utils.js`:

From `repo/`:
- `trailing-whitespace` (matches `.*`)
- `eslint` (matches `\.js$`)

From `repo/frontend/`:
- `eslint` (matches `\.js$` relative → `src/App.js` ✅)
- `prettier` (matches `\.js$`)

For `backend/api.py`:

From `repo/`:
- `trailing-whitespace` (matches `.*`)
- `eslint` (matches `\.js$` → NO MATCH for .py file)

From `repo/backend/`:
- `black` (matches `\.py$` relative → `api.py` ✅)
- `flake8` (matches `\.py$`)

For `docs/README.md`:

From `repo/`:
- `trailing-whitespace` (matches `.*`)

#### Step 3: Deduplicate (Closest-Wins)

For `frontend/src/App.js`:
- `trailing-whitespace` from `repo/` (no conflict)
- `eslint`: CONFLICT
  - `repo/`: `eslint --max-warnings 10`
  - `repo/frontend/`: `eslint --max-warnings 0` ← **WINNER** (closer)
- `prettier` from `repo/frontend/` (no conflict)

**Final hooks for frontend files**:
- `trailing-whitespace` (from repo/)
- `eslint` (from frontend/)
- `prettier` (from frontend/)

For `backend/api.py`:
- `trailing-whitespace` from `repo/` (no conflict)
- `black` from `repo/backend/` (no conflict)
- `flake8` from `repo/backend/` (no conflict)

**Final hooks for backend files**:
- `trailing-whitespace` (from repo/)
- `black` (from backend/)
- `flake8` (from backend/)

For `docs/README.md`:
- `trailing-whitespace` from `repo/`

#### Step 4: Build File → Hooks Mapping

```
frontend/src/App.js → [trailing-whitespace, eslint, prettier]
frontend/src/utils.js → [trailing-whitespace, eslint, prettier]
backend/api.py → [trailing-whitespace, black, flake8]
docs/README.md → [trailing-whitespace]
```

#### Step 5: Detect Overlaps

Reverse mapping (Hook → Files):

```
trailing-whitespace → [frontend/src/App.js, frontend/src/utils.js, backend/api.py, docs/README.md]
eslint → [frontend/src/App.js, frontend/src/utils.js]
prettier → [frontend/src/App.js, frontend/src/utils.js]
black → [backend/api.py]
flake8 → [backend/api.py]
```

Overlap groups:
- `trailing-whitespace` overlaps with ALL hooks
- `eslint` and `prettier` overlap with each other
- `black` and `flake8` overlap with each other

#### Step 6: Execution Plan

Since `trailing-whitespace` touches all files, it cannot run in parallel with anything.

**Option A**: Run `trailing-whitespace` first, then parallelize others:

```
[Serial - runs first]
  └─ trailing-whitespace (all 4 files)

[Parallel after trailing-whitespace]
  ├─ [Serial for frontend/*.js]
  │   ├─ eslint (frontend/src/App.js, frontend/src/utils.js)
  │   └─ prettier (frontend/src/App.js, frontend/src/utils.js)
  │
  └─ [Serial for backend/*.py]
      ├─ black (backend/api.py)
      └─ flake8 (backend/api.py)
```

**Option B**: Interleave based on actual file access:

```
[Parallel]
  ├─ [Serial Group 1: frontend/src/App.js]
  │   ├─ trailing-whitespace
  │   ├─ eslint
  │   └─ prettier
  │
  ├─ [Serial Group 2: frontend/src/utils.js]
  │   ├─ trailing-whitespace
  │   ├─ eslint
  │   └─ prettier
  │
  ├─ [Serial Group 3: backend/api.py]
  │   ├─ trailing-whitespace
  │   ├─ black
  │   └─ flake8
  │
  └─ [Serial Group 4: docs/README.md]
      └─ trailing-whitespace
```

**Implementation choice**: Option A is simpler; Option B has better parallelism. We can start with Option A and optimize to Option B later.

---

## Hook ID Conflicts: Closest-Wins

### Example 1: Override Parent Hook

```
repo/
  .tree-commit-config.yaml
    hooks:
      - id: python-format
        entry: black --line-length 88

  experimental/
    .tree-commit-config.yaml
      hooks:
        - id: python-format
          entry: black --line-length 120  # Different line length
```

**Changed file**: `experimental/ml/model.py`

**Result**: Uses `black --line-length 120` from `experimental/` config (closer wins)

### Example 2: Same Hook, Different Scopes

```
repo/
  frontend/.tree-commit-config.yaml
    hooks:
      - id: test-runner
        entry: npm test

  backend/.tree-commit-config.yaml
    hooks:
      - id: test-runner
        entry: pytest
```

**Changed files**: `frontend/App.js`, `backend/api.py`

**Result**:
- `frontend/App.js` → runs `npm test` (from frontend/)
- `backend/api.py` → runs `pytest` (from backend/)

No conflict! Same hook ID in different scopes.

---

## Repository Version Conflicts

Multiple configurations can reference the same external repository with different revisions. Each hook gets its own isolated environment.

```yaml
# frontend/.tree-commit-config.yaml
repos:
  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.4.0
    hooks:
      - id: trailing-whitespace-v4.4

# backend/.tree-commit-config.yaml
repos:
  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.5.0
    hooks:
      - id: trailing-whitespace-v4.5
```

**Resolution**: Both versions are installed in separate environments. Both hooks execute (assuming they have different IDs).

---

## Disabling Inherited Hooks

To disable a hook defined in a parent config:

```yaml
# repo/.tree-commit-config.yaml
repos:
  - repo: local
    hooks:
      - id: strict-linter
        entry: super-strict-linter

# experimental/.tree-commit-config.yaml
repos:
  - repo: local
    hooks:
      - id: strict-linter
        disable: true  # Disables the inherited hook
```

**Alternative**: Override with a no-op:

```yaml
repos:
  - repo: local
    hooks:
      - id: strict-linter
        entry: "true"  # no-op command
```

---

## Execution Order Within Serial Groups

When hooks must run serially (operating on same files), execution order is:

1. **Explicit priority** (if `priority` field is added in future)
2. **Config depth** (deeper configs run first - closest to code)
3. **Definition order** within the same config file

**Example**:

```yaml
# repo/frontend/.tree-commit-config.yaml
repos:
  - repo: local
    hooks:
      - id: formatter    # Runs 1st (deeper config, defined first)
      - id: linter       # Runs 2nd (deeper config, defined second)

# repo/.tree-commit-config.yaml (parent)
repos:
  - repo: local
    hooks:
      - id: validator    # Runs 3rd (shallower config)
```

For a changed file `frontend/src/App.js`, execution order:
1. `formatter` (from frontend/, first in file)
2. `linter` (from frontend/, second in file)
3. `validator` (from repo/, parent config)

---

## Error Handling

### Configuration Validation
- **Invalid YAML syntax** → Configuration file skipped with warning
- **Missing required hook fields** → Hook skipped with warning
- **Invalid repository URLs** → Repository installation fails, hooks skipped

### Runtime Errors
- **File permission errors** → Hook fails, error reported, other hooks continue
- **Missing dependencies** → Hook fails, error reported, other hooks continue
- **Network failures** → Repository installation fails, affected hooks skipped
- **Hook execution timeout** → Hook killed, error reported, other hooks continue

**Philosophy**: Fail gracefully. One broken hook shouldn't block the entire commit.

---

## Best Practices

### 1. Use Descriptive Hook IDs

```yaml
# Good: Specific to scope and purpose
- id: frontend-eslint-strict
- id: backend-python-formatter
- id: docs-markdown-linter

# Avoid: Generic IDs that might conflict
- id: lint
- id: format
- id: check
```

### 2. Leverage Scope-Based Architecture

```yaml
# repo/.tree-commit-config.yaml - Project-wide rules
repos:
  - repo: local
    hooks:
      - id: no-secrets
        entry: detect-secrets
        files: .*

# repo/frontend/.tree-commit-config.yaml - Frontend-specific
repos:
  - repo: local
    hooks:
      - id: eslint
        entry: eslint --config .eslintrc.frontend.js
```

### 3. Pin Repository Revisions

```yaml
repos:
  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.5.0  # Pinned version, not 'main' or 'latest'
```

### 4. Design File Patterns for Parallelism

If performance is critical, design hooks with non-overlapping file patterns:

```yaml
# These can run in parallel
- id: format-js
  files: \.js$

- id: format-py
  files: \.py$

- id: format-go
  files: \.go$
```

### 5. Order Hooks Intentionally

Put formatters before linters in config files:

```yaml
repos:
  - repo: local
    hooks:
      - id: black           # Format first
      - id: flake8          # Lint second
      - id: mypy            # Type check third
```

---

## Future Enhancements

### Explicit Priority

```yaml
hooks:
  - id: formatter
    priority: 100  # Lower number = runs first

  - id: linter
    priority: 200
```

### Dependency Declaration

```yaml
hooks:
  - id: linter
    depends_on: [formatter]  # Only run after formatter succeeds
```

### Conditional Execution

```yaml
hooks:
  - id: expensive-check
    run_if: staged_files > 10  # Only run for large changesets
```
