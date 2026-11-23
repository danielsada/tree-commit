# Getting started

This walkthrough sets up tree-commit in a small repository with a Python backend and a shared set of checks. It takes about five minutes.

## 1. Install tree-commit

You need git and [Go](https://go.dev/dl/) 1.23 or newer.

```sh
go install github.com/danielsada/tree-commit@latest
tree-commit --version
```

If `tree-commit` is not found, add Go's bin directory to your `PATH`. It is `$(go env GOPATH)/bin`, usually `~/go/bin`.

The examples below also use [uv](https://docs.astral.sh/uv/getting-started/installation/) to run Python tools without installing them globally. Hooks from remote repositories that are written in Python need Python 3.

## 2. Add a config for the whole repository

Create `.tree-commit-config.yaml` at the root of your repository. These hooks apply to every file:

```yaml
repos:
  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v6.0.0
    hooks:
      - id: trailing-whitespace
      - id: end-of-file-fixer
      - id: check-yaml
      - id: check-merge-conflict
```

These hooks come from another git repository. tree-commit downloads it once, at the version given in `rev`, and caches it.

## 3. Add a config for one project

Say your backend lives in `backend/`. Create `backend/.tree-commit-config.yaml`:

```yaml
repos:
  - repo: local
    hooks:
      - id: ruff-check
        name: ruff check
        entry: uvx ruff check --fix
        language: system
        types: [python]

      - id: ruff-format
        name: ruff format
        entry: uvx ruff format
        language: system
        types: [python]

      - id: pytest
        name: pytest
        entry: uv run pytest -q
        language: system
        types: [python]
        pass_filenames: false
```

What this says:

- `repo: local`: the hooks are commands defined right here.
- `entry`: the command to run. tree-commit adds the names of the files to check at the end, so `ruff check --fix app.py models.py`.
- `types: [python]`: only check Python files.
- `pass_filenames: false`: run the command once, without file names. pytest finds its own tests.

Because this config is in `backend/`, these hooks only see files under `backend/`, and they run **inside** `backend/`. That is why `uv run pytest` finds `backend/pyproject.toml`.

## 4. See what will run

```sh
git add -A
tree-commit tree
tree-commit run --dry-run
```

`tree` shows the configs it found. `--dry-run` lists every hook that would run on your staged files, with the files each one gets, without running anything.

## 5. Run the hooks

```sh
tree-commit run --all-files
```

Every hook prints one line: `Passed`, `Failed` or `Skipped`. A failing hook also shows its output. A hook that **changes** a file (a formatter, for example) counts as failed too, so you can review the change before committing.

## 6. Install the git hook

```sh
tree-commit install
```

From now on, `git commit` runs tree-commit on the files you staged. If anything fails, the commit is stopped. Fix the problem, or `git add` the changes a formatter made, and commit again.

To skip a hook once:

```sh
SKIP=pytest git commit -m "WIP"
```

To bypass all hooks once, use git's own `git commit --no-verify`.

## Next steps

- Read [How it works](how-it-works.md) to learn how nested configs override each other.
- Browse the [Recipes](recipes.md) for Go, frontend, CI and more.
- Look at the [example monorepo](../example_repos/example_monorepo), which uses every feature.
