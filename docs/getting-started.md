# Getting started

This walkthrough shows how to install and configure tree-commit in a small repository. The repository has a Python backend and a set of hooks for all files. It takes approximately five minutes.

## 1. Install tree-commit

You need git and [Go](https://go.dev/dl/) 1.23 or newer.

Run these commands to install tree-commit and make sure that it works:

```sh
go install github.com/danielsada/tree-commit@latest
tree-commit --version
```

If your shell cannot find `tree-commit`, add the Go bin directory to your `PATH`. This directory is `$(go env GOPATH)/bin`, usually `~/go/bin`.

The examples below also use [uv](https://docs.astral.sh/uv/getting-started/installation/) to run Python tools. With uv, you do not have to install these tools globally. Remote hooks that are written in Python need Python 3.

## 2. Add a config file for the full repository

Create a `.tree-commit-config.yaml` file at the root of your repository, with this content. These hooks apply to all files:

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

These hooks come from a different git repository (a hook repository). tree-commit downloads this repository one time, at the version in `rev`, and keeps it in its cache.

## 3. Add a config file for one project

In this example, your backend is in `backend/`. Create a `backend/.tree-commit-config.yaml` file with this content:

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

The config file has these parts:

- `repo: local`: the hooks are local hooks. Their commands are in this config file.
- `entry`: the command to run. tree-commit adds the names of the files to check at the end of the command. For example: `ruff check --fix app.py models.py`.
- `types: [python]`: check only Python files.
- `pass_filenames: false`: run the command one time, without file names. pytest finds its own tests.

Because this config file is in `backend/`, its hooks get only the files below `backend/`. The hooks also run **inside** `backend/`. Thus, `uv run pytest` finds `backend/pyproject.toml`.

## 4. Preview the hooks

Stage your changes, then show the config files and the hooks:

```sh
git add -A
tree-commit tree
tree-commit run --dry-run
```

`tree` shows the config files that tree-commit found. `--dry-run` lists each hook for your staged files, and the files that each hook gets. It does not run the hooks.

## 5. Run the hooks

Run all hooks on all files:

```sh
tree-commit run --all-files
```

Each hook prints one line: `Passed`, `Failed` or `Skipped`. If a hook fails, it also shows its output. If a hook **changes** a file (for example, a formatter), the result is also `Failed`. This lets you examine the change before you commit.

## 6. Install the git hook

Run this command:

```sh
tree-commit install
```

After this step, `git commit` runs tree-commit on the files that you staged. If a hook fails, the commit stops. Then do these steps:

1. Fix the problem, or use `git add` to stage the changes that a formatter made.
2. Commit again.

To skip a hook one time, use the `SKIP` environment variable:

```sh
SKIP=pytest git commit -m "WIP"
```

To skip all hooks one time, use `git commit --no-verify`. This is a standard git option.

## Next steps

These pages give more information:

- Read [How it works](how-it-works.md) to learn how nested config files override each other.
- See the [Recipes](recipes.md) for Go, frontend, CI and other configurations.
- Examine the [example monorepo](../example_repos/example_monorepo). It uses all features.
