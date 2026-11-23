# Recipes

This page gives config files that you can copy for common situations. The [example monorepo](../example_repos/example_monorepo) combines most of them.

## Python with uv and ruff

Put the shared settings in the config file at the root. Then tree-commit lints all Python projects the same way:

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

Each uv project adds a hook for its tests. The hook runs in the project directory, so `uv run` uses the lockfile of the project:

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

You can use the official mirror, [ruff-pre-commit](https://github.com/astral-sh/ruff-pre-commit), instead. For this, Python 3 must be on your `PATH`:

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

Define a hook with the same `id` in a nested config file. For the files in that directory and its subdirectories, this hook replaces the hook from the parent config file:

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

Disable the hooks that must not run on this code:

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

To skip directories for only one hook, use `exclude_folders: [generated/]`.

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

Run `npm install` first. Then use the tools of the project with `npx --no-install`:

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

shellcheck and shfmt are available as Python wheels. Thus, `uvx` can install them when they are necessary:

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

Use a `pygrep` hook to forbid a pattern and a `fail` hook to forbid files:

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

This hook runs only in the `pre-push` stage:

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

To run these hooks before you push, or in CI, use this command:

```sh
tree-commit run --hook-stage pre-push --all-files
```

## Continuous integration

This workflow runs all hooks on all files:

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
      - uses: astral-sh/setup-uv@v10.4.0
      - run: go install github.com/danielsada/tree-commit@latest
      - run: tree-commit run --all-files --color always
```

To check only the files that the pull request changed, use these commands:

```sh
git fetch origin main
tree-commit run --files "$(git diff --name-only --diff-filter=d origin/main... | paste -sd, -)"
```

## Try a config file before you commit it

tree-commit also finds config files that are not committed yet. To see what they do, use these commands:

```sh
tree-commit run --all-files --dry-run
tree-commit run my-new-hook --all-files
```
