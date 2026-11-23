# How tree-commit decides what runs

tree-commit uses a **scope-based execution model** with **closest-wins** conflict resolution. Hooks only run on files within their scope, and nested configurations can override parent configurations. This keeps monorepos fast and lets every team own its own checks.

## Core Principles

### 1. Scope-Based Execution

**Rule**: A hook only runs on a changed file if its configuration file lives in an ancestor directory of that file.

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

**Benefit**: Irrelevant hooks never run, which improves performance and reduces noise.

In every directory tree-commit looks for `.tree-commit-config.yaml` first and falls back to `.pre-commit-config.yaml`, so an existing pre-commit setup works unchanged as a root config.

---

### 2. Relative Paths and Working Directory

**Rule**: Everything in a nested config is **relative to the config's directory**:

- `files` / `exclude` regexes (and the top-level `files` / `exclude`) are matched against the path relative to the config's directory.
- `include_folders` / `exclude_folders` are relative to the config's directory.
- The hook runs **with the config's directory as its working directory**, and receives file names relative to it.

Patterns always use forward slashes (like in pre-commit), on every platform. File names passed to hooks use the platform's native separator (backslashes on Windows).

```yaml
# repo/frontend/.tree-commit-config.yaml
repos:
  - repo: local
    hooks:
      - id: eslint
        entry: eslint --fix
        files: ^src/.*\.js$
        # Changed file: frontend/src/components/Button.js
        # Pattern tested against: src/components/Button.js ✅
        # Runs as: (cd frontend && eslint --fix src/components/Button.js)
```

For a config at the repository root this is exactly pre-commit's behavior. For nested configs it means `go test ./...`, `tsc --noEmit` or `pytest` run in the project they belong to.

**Benefit**: Config files are portable and work the same wherever the project lives in the repository.

#### Folder filters

tree-commit adds two optional hook fields:

- `include_folders`: the file must be inside one of these folders. An empty list or `.` means everything.
- `exclude_folders`: the file must not be inside one of these folders. `build/` excludes `build/x.js` and also nested `pkg/build/x.js`.

They are applied together with `files`, `exclude`, `types`, `types_or` and `exclude_types`. As in pre-commit, `types` defaults to `[file]`, so symlinks are only passed to hooks that ask for them.

---

### 3. Closest-Wins Conflict Resolution

**Rule**: When several ancestor configs define hooks with the same `id`, only the hook(s) from the **closest** config to the changed file apply to that file.

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
- `frontend/.tree-commit-config.yaml` is closer, so **its `eslint` hook executes**
- The root `eslint` is skipped for this file

The override replaces the whole definition: if the closer hook's `files` pattern does not match a file, the parent's hook does not run on it either. Within a single config file the same id may appear several times (for example the same linter with different `args`), and all of those entries apply.

**Rationale**:
- Teams can override project-wide defaults
- More specific configurations take precedence
- Intuitive: "closer to code = more specific = higher priority"

---

### 4. Parallel Execution Without Conflicts

**Rule**: Hooks run in parallel unless they operate on the same file(s). Hooks that share files run one after the other, in execution order (see below).

Overlap detection is based on **the actual files each hook will receive**, not on static regex analysis:

1. Build a map of `hook → [files it will operate on]`.
2. A hook that shares a file with an earlier hook waits for that hook to finish.
3. Every other hook starts as soon as a worker is free (`--jobs`, default: number of CPUs).

Hooks with `pass_filenames: false` (and `always_run` hooks without files) are treated as operating on their **whole directory**, so they wait for every earlier hook that touches a file inside that directory. That way `go test ./...` never runs while `gofmt -w` is rewriting the same package.

**Example**:

```
Changed files:
  - frontend/src/App.js
  - backend/api.py

Hooks and their files (in execution order):
  1. black       (backend)  → [backend/api.py]
  2. eslint      (frontend) → [frontend/src/App.js]
  3. whitespace  (root)     → [frontend/src/App.js, backend/api.py]

Execution:
  black and eslint start in parallel (disjoint files)
  whitespace waits for both, because it touches both files
```

**Benefits**:
- Maximizes parallelism for performance
- No file corruption from concurrent writes
- No complex regex overlap analysis needed

Use `tree-commit run --dry-run` to see the plan, including which hook runs after which.

---

## Execution Order

When hooks must run serially (they operate on the same files), the order is:

1. **Config depth**: deeper configs run first (closest to the code)
2. **Config path**: alphabetical, for configs at the same depth
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

Output is always printed in this order, even when hooks run in parallel.

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

- `frontend/src/App.js`, `frontend/src/utils.js`: `repo/`, `repo/frontend/`
- `backend/api.py`: `repo/`, `repo/backend/`
- `docs/README.md`: `repo/`

#### Step 2: Closest-Wins and Matching

For `frontend/src/*.js`:
- `eslint` is defined by `repo/frontend/` and `repo/`: **frontend wins**
- `prettier` from `repo/frontend/`
- `trailing-whitespace` from `repo/`

For `backend/api.py`:
- `black`, `flake8` from `repo/backend/`
- `trailing-whitespace` from `repo/`
- root `eslint` does not match `\.py$`

For `docs/README.md`:
- `trailing-whitespace` from `repo/`

#### Step 3: Hook → Files

In execution order (deeper configs first, then path, then definition order):

```
1. black               (backend/)  → [api.py]
2. flake8              (backend/)  → [api.py]
3. eslint              (frontend/) → [src/App.js, src/utils.js]
4. prettier            (frontend/) → [src/App.js, src/utils.js]
5. trailing-whitespace (root)      → [backend/api.py, docs/README.md, frontend/src/App.js, frontend/src/utils.js]
```

#### Step 4: Execution Plan

```
[Parallel]
  ├─ black    → then flake8     (share backend/api.py)
  └─ eslint   → then prettier   (share frontend/src/*.js)
[After all of the above]
  └─ trailing-whitespace        (shares files with every other hook)
```

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

Different configurations can reference the same external repository at different revisions. Each `repo@rev` is cloned (and its environment built) separately and cached.

```yaml
# frontend/.tree-commit-config.yaml
repos:
  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.4.0
    hooks:
      - id: trailing-whitespace

# backend/.tree-commit-config.yaml
repos:
  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.5.0
    hooks:
      - id: trailing-whitespace
```

**Resolution**: frontend files are checked with v4.4.0 and backend files with v4.5.0. A root config defining the same id would be overridden in both directories.

---

## Disabling Inherited Hooks

To disable a hook defined in a parent config, redefine its id with `disable: true`:

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
        disable: true  # Disables the inherited hook for experimental/
```

A disabled hook needs no `entry`. It still claims the id, so the parent hook does not run on files under `experimental/`.

To skip hooks for a single run, use pre-commit's `SKIP` variable: `SKIP=strict-linter,eslint git commit`.

---

## Error Handling

### Configuration Errors
- **Invalid YAML or missing required fields** (`repo`, `rev`, hook `id`, local `entry`): tree-commit stops with an error naming the file. A broken config should never silently turn checks off.
- **Invalid regular expression in a hook**: that hook is reported as failed.
- **Unknown hook id in a remote repository**: that hook is reported as failed.

### Runtime Errors
- **Repository clone or environment install fails** (network, missing python): the affected hooks are reported as failed, other hooks still run.
- **Hook exits non-zero or modifies files**: the hook is reported as failed, other hooks still run.
- **Executable not found**: the hook is reported as failed.
- **Unsupported language in a remote repository** (anything other than `system`, `script`, `python`, `pygrep` and `fail`): the hook is reported as skipped.

**Philosophy**: One broken hook does not stop the others from running, so you see every problem at once, but any failure blocks the commit.

---

## Best Practices

### 1. Use Descriptive Hook IDs

```yaml
# Good: Specific to scope and purpose
- id: frontend-eslint-strict
- id: backend-python-formatter
- id: docs-markdown-linter

# Avoid generic ids unless you want them to override each other
- id: lint
- id: format
```

### 2. Leverage Scope-Based Architecture

```yaml
# repo/.tree-commit-config.yaml - Project-wide rules
repos:
  - repo: local
    hooks:
      - id: no-secrets
        entry: detect-secrets-hook
        language: system

# repo/frontend/.tree-commit-config.yaml - Frontend-specific
repos:
  - repo: local
    hooks:
      - id: eslint
        entry: eslint --config .eslintrc.frontend.js
        language: system
```

### 3. Pin Repository Revisions

```yaml
repos:
  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.5.0  # Pinned version, not 'main' or 'latest'
```

Revisions are cached by `repo@rev`, so a moving branch name will not be refetched. Run `tree-commit clean` to clear the cache.

### 4. Design File Patterns for Parallelism

Hooks that never share files run in parallel:

```yaml
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

### Per-file Serial Chains

Today a hook runs once with all of its files and waits for any earlier hook that shares one of them. A finer-grained scheduler could build one serial chain per file and run chains in parallel, so `trailing-whitespace` on `docs/` would not wait for `eslint` on `frontend/`.

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
