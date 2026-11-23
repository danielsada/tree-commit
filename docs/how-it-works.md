# How tree-commit decides what runs

tree-commit uses a **scope-based execution model** with **closest-wins** conflict resolution. A hook runs only on the files in its scope. A nested config file can override the hooks of a parent config file. This keeps monorepos fast, and each team can own its checks.

## Core Principles

### 1. Scope-Based Execution

**Rule**: A hook runs on a changed file only if its config file is in an ancestor directory of that file.

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

**For the changed file `frontend/src/App.js`**:
- ✅ Hook A runs (ancestor: `repo/`)
- ✅ Hook B runs (ancestor: `repo/frontend/`)
- ❌ Hook C does NOT run (not an ancestor)

**For the changed file `backend/api.py`**:
- ✅ Hook A runs (ancestor: `repo/`)
- ❌ Hook B does NOT run (not an ancestor)
- ✅ Hook C runs (ancestor: `repo/backend/`)

**Benefit**: Hooks that do not apply to a file never run. This improves performance and reduces noise.

In each directory, tree-commit looks for `.tree-commit-config.yaml` first. If that file does not exist, tree-commit uses `.pre-commit-config.yaml`. Thus, an existing pre-commit config file works without changes as a root config file.

---

### 2. Relative Paths and Working Directory

**Rule**: All paths in a nested config file are **relative to the directory of the config file**:

- tree-commit matches the `files` / `exclude` regexes (and the top-level `files` / `exclude`) against the path relative to the directory of the config file.
- `include_folders` / `exclude_folders` are relative to the directory of the config file.
- The hook runs **with the directory of the config file as its working directory**. It receives file names relative to that directory.

Patterns always use forward slashes on all platforms, as in pre-commit. The file names that tree-commit passes to hooks use the native separator of the platform (backslashes on Windows).

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

For a config file at the repository root, this is the same as the pre-commit behavior. For a nested config file, commands such as `go test ./...`, `tsc --noEmit` or `pytest` run in the project that they belong to.

**Benefit**: Config files are portable. They work the same in all locations in the repository.

#### Directory filters

tree-commit adds two optional hook fields:

- `include_folders`: the file must be in one of these directories. An empty list or `.` matches all files.
- `exclude_folders`: the file must not be in one of these directories. `build/` excludes `build/x.js` and also the nested `pkg/build/x.js`.

tree-commit applies these filters together with `files`, `exclude`, `types`, `types_or` and `exclude_types`. As in pre-commit, the default value of `types` is `[file]`. Thus, tree-commit passes symlinks only to hooks that ask for them.

---

### 3. Closest-Wins Conflict Resolution

**Rule**: More than one ancestor config file can define hooks with the same `id`. For a changed file, only the hooks from the config file **closest** to that file apply.

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
- Both config files are ancestors.
- Both config files define the hook `id: eslint`.
- `frontend/.tree-commit-config.yaml` is closer, so **its `eslint` hook runs**.
- tree-commit skips the root `eslint` hook for this file.

The override replaces the full definition. If the `files` pattern of the closer hook does not match a file, the parent hook also does not run on that file. In one config file, the same id can occur more than one time, for example the same linter with different `args`. In this case, all of these entries apply.

**Rationale**:
- Teams can override project-wide defaults.
- More specific config files have priority.
- The rule is intuitive: "closer to the code = more specific = higher priority".

---

### 4. Parallel Execution Without Conflicts

**Rule**: Hooks run in parallel, unless they operate on the same files. Hooks that share files run one after the other, in the execution order (refer to the next section).

tree-commit finds overlaps from **the files that each hook actually receives**. It does not do a static analysis of the regexes. tree-commit uses this procedure:

1. tree-commit makes a map of `hook → [files it will operate on]`.
2. A hook that shares a file with an earlier hook waits until that hook is complete.
3. All other hooks start when a worker is available (`--jobs`, default: the number of CPUs).

tree-commit treats a hook with `pass_filenames: false` as a hook that operates on its **whole directory**. The same applies to an `always_run` hook without files. Such a hook waits for all earlier hooks that touch a file in that directory. Thus, `go test ./...` never runs while `gofmt -w` writes changes to the same package.

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
- Maximum parallelism, for better performance.
- No file corruption from concurrent writes.
- No need for a complex analysis of regex overlaps.

To see the plan, run `tree-commit run --dry-run`. The plan shows which hook runs after which other hook.

---

## Execution Order

If hooks must run one after the other (because they operate on the same files), the order is:

1. **Config depth**: deeper config files run first (closest to the code).
2. **Config path**: alphabetical order, for config files at the same depth.
3. **Definition order** in the same config file.

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

For the changed file `frontend/src/App.js`, the execution order is:
1. `formatter` (from frontend/, first in the file)
2. `linter` (from frontend/, second in the file)
3. `validator` (from repo/, parent config file)

tree-commit always prints the output in this order, also when hooks run in parallel.

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

For each changed file, tree-commit finds the ancestor config files:

- `frontend/src/App.js`, `frontend/src/utils.js`: `repo/`, `repo/frontend/`
- `backend/api.py`: `repo/`, `repo/backend/`
- `docs/README.md`: `repo/`

#### Step 2: Closest-Wins and Matching

For `frontend/src/*.js`, these hooks apply:
- `eslint` from `repo/frontend/`. `repo/` also defines `eslint`, but **frontend wins**.
- `prettier` from `repo/frontend/`
- `trailing-whitespace` from `repo/`

For `backend/api.py`, these hooks apply:
- `black`, `flake8` from `repo/backend/`
- `trailing-whitespace` from `repo/`

The root `eslint` hook does not apply to `backend/api.py`, because its `files` pattern (`\.js$`) does not match `.py` files.

For `docs/README.md`, this hook applies:
- `trailing-whitespace` from `repo/`

#### Step 3: Hook → Files

This list shows each hook and its files in the execution order (deeper config files first, then path, then definition order):

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

**Result**: tree-commit uses `black --line-length 120` from the `experimental/` config file, because the closest config file wins.

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

There is no conflict, because the two hooks with the same ID are in different scopes.

---

## Repository Version Conflicts

Different config files can refer to the same hook repository at different revisions. tree-commit clones each `repo@rev` and installs its environment separately, and keeps each one in the cache.

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

**Resolution**: tree-commit checks frontend files with v4.4.0 and backend files with v4.5.0. If a root config file defines the same id, these two config files override it in both directories.

---

## Disable Inherited Hooks

To disable a hook from a parent config file, define its id again with `disable: true`:

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

A disabled hook does not need an `entry`. It still owns the id. Thus, the parent hook does not run on files in `experimental/`.

To skip hooks for one run, use the `SKIP` variable, as in pre-commit: `SKIP=strict-linter,eslint git commit`.

---

## Error Handling

### Configuration Errors
- **Invalid YAML or missing required fields** (`repo`, `rev`, hook `id`, local `entry`): tree-commit stops and shows an error with the file name. A broken config file must not silently disable checks.
- **Invalid regular expression in a hook**: tree-commit reports that hook as `Failed`.
- **Unknown hook id in a hook repository**: tree-commit reports that hook as `Failed`.

### Runtime Errors
- **Repository clone or environment installation fails** (network, missing python): tree-commit reports the affected hooks as `Failed`. Other hooks still run.
- **Hook exits with a non-zero code or changes files**: tree-commit reports the hook as `Failed`. Other hooks still run.
- **Executable not found**: tree-commit reports the hook as `Failed`.
- **Unsupported language in a hook repository** (all languages other than `system`, `script`, `python`, `node`, `golang`, `grep`, `pygrep` and `fail`): tree-commit reports the hook as `Skipped`.

**Philosophy**: One broken hook does not stop the other hooks. Thus, you see all problems in one run. But any failure blocks the commit.

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

### 2. Use the Scope-Based Model

Put project-wide rules in the root config file, and rules for one project in a nested config file:

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

Pin each hook repository to a fixed revision:

```yaml
repos:
  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.5.0  # Pinned version, not 'main' or 'latest'
```

tree-commit caches revisions by `repo@rev`. Thus, if `rev` is a branch name and the branch moves, tree-commit does not fetch it again. To clear the cache, run `tree-commit clean`.

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

NOTE: The features in this section are not available yet. They are possible changes for future versions.

### Per-file Serial Chains

Today, a hook runs one time with all of its files. It waits for each earlier hook that shares one of these files. A more precise scheduler could make one serial chain for each file and run the chains in parallel. Then `trailing-whitespace` on `docs/` would not wait for `eslint` on `frontend/`.

### Explicit Priority

A `priority` field could set the order of hooks:

```yaml
hooks:
  - id: formatter
    priority: 100  # Lower number = runs first

  - id: linter
    priority: 200
```

### Dependency Declaration

A `depends_on` field could make a hook run only after another hook succeeds:

```yaml
hooks:
  - id: linter
    depends_on: [formatter]  # Only run after formatter succeeds
```

### Conditional Execution

A `run_if` field could make a hook run only when a condition is true:

```yaml
hooks:
  - id: expensive-check
    run_if: staged_files > 10  # Only run for large changesets
```
