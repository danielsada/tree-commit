# Recipes

Copy-paste setups for common situations. The [example monorepo](../example_repos/example_monorepo) combines most of them.

## Python with uv and ruff

Shared settings at the root, so every Python project is linted the same way:

```yaml
# .tree-commit-config.yaml
repos:
  - repo: local
    hooks:
      - id: ruff-check
        name: ruff check
        entry: uvx ruff@0.17.6 check --fix --force-exclude
        language: system
        types_or: [python, pyi]

      - id: ruff-format
        name: ruff format
        entry: uvx ruff@0.17.6 format --force-exclude
        language: system
        types_or: [python, pyi]
```

Each uv project adds its tests. The hook runs inside the project, so `uv run` uses the project's lockfile:

```yaml
# services/billing/.tree-commit-config.yaml
repos:
  - repo: local
    hooks:
      - id: pytest
        name: pytest (billing)
        entry: uv run --locked pytest -q
        language: system
        types: [python]
        pass_filenames: false
```

Prefer the official mirror? Use [ruff-pre-commit](https://github.com/astral-sh/ruff-pre-commit) instead; Python 3 must be on your `PATH`:

```yaml
repos:
  - repo: https://github.com/astral-sh/ruff-pre-commit
    rev: v0.17.6
    hooks:
      - id: ruff-check
        args: [--fix]
      - id: ruff-format
```

## Stricter rules for one directory

Redefine the same `id` in a nested config. For files below that directory, it replaces the parent's hook:

```yaml
# src/core/.tree-commit-config.yaml
repos:
  - repo: local
    hooks:
      - id: ruff-check
        name: ruff check (strict)
        entry: uvx ruff@0.17.6 check --fix --extend-select D,ANN
        language: system
        types_or: [python, pyi]
```

## Vendored or generated code

Turn off the hooks that should not touch it:

```yaml
# third_party/.tree-commit-config.yaml
repos:
  - repo: local
    hooks:
      - id: ruff-check
        disable: true
      - id: ruff-format
        disable: true
```

Or skip folders for a single hook with `exclude_folders: [generated/]`.

## Go module

```yaml
# go_lib/.tree-commit-config.yaml
repos:
  - repo: local
    hooks:
      - id: gofmt
        name: gofmt
        entry: gofmt -l -w
        language: system
        types: [go]

      - id: go-vet
        name: go vet
        entry: go vet ./...     # runs in go_lib/, the module root
        language: system
        types: [go]
        pass_filenames: false

      - id: go-test
        name: go test
        entry: go test ./...
        language: system
        types: [go]
        pass_filenames: false
```

## JavaScript / TypeScript project

Use the project's own tools with `npx --no-install` (run `npm install` first):

```yaml
# frontend/.tree-commit-config.yaml
repos:
  - repo: local
    hooks:
      - id: prettier
        name: prettier
        entry: npx --no-install prettier --write --ignore-unknown
        language: system
        types_or: [javascript, jsx, ts, tsx, css, json]

      - id: eslint
        name: eslint
        entry: npx --no-install eslint --fix
        language: system
        types_or: [javascript, jsx, ts, tsx]

      - id: tsc
        name: tsc
        entry: npx --no-install tsc --noEmit
        language: system
        types_or: [ts, tsx]
        pass_filenames: false
```

## Shell scripts

shellcheck and shfmt are available as Python wheels, so `uvx` can install them on demand:

```yaml
repos:
  - repo: local
    hooks:
      - id: shellcheck
        name: shellcheck
        entry: uvx --from shellcheck-py shellcheck
        language: system
        types: [shell]

      - id: shfmt
        name: shfmt
        entry: uvx --from shfmt-py shfmt -w -i 4
        language: system
        types: [shell]
```

## Forbid patterns and files

```yaml
repos:
  - repo: local
    hooks:
      - id: no-debugger
        name: no breakpoint()
        entry: '\bbreakpoint\(\)'
        language: pygrep
        types: [python]

      - id: no-env-files
        name: no .env files
        entry: Do not commit .env files, use .env.example
        language: fail
        files: (^|/)\.env$
```

## Slow checks on push only

```yaml
default_stages: [pre-commit]
repos:
  - repo: local
    hooks:
      - id: integration-tests
        name: integration tests
        entry: uv run pytest -q tests/integration
        language: system
        pass_filenames: false
        stages: [pre-push]
```

Run them before pushing, or in CI:

```sh
tree-commit run --hook-stage pre-push --all-files
```

## Continuous integration

Run every hook on every file:

```yaml
# .github/workflows/checks.yml
name: checks
on: [pull_request]
jobs:
  tree-commit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-go@v7
        with:
          go-version: stable
      - uses: astral-sh/setup-uv@v10
      - run: go install github.com/danielsada/tree-commit@latest
      - run: tree-commit run --all-files --color always
```

Or only check what the pull request changed:

```sh
git fetch origin main
tree-commit run --files "$(git diff --name-only --diff-filter=d origin/main... | paste -sd, -)"
```

## Try a config before committing it

Configs that are not committed yet are picked up too. Preview what they would do:

```sh
tree-commit run --all-files --dry-run
tree-commit run my-new-hook --all-files
```
