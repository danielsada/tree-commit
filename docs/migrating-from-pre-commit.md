# Migrating from pre-commit

tree-commit reads pre-commit configs and runs pre-commit hooks, so most setups move over without changes.

## Step 1: keep your config

Leave `.pre-commit-config.yaml` where it is. tree-commit uses it in any directory that has no `.tree-commit-config.yaml`. Check what it would do:

```sh
tree-commit run --all-files --dry-run
tree-commit run --all-files
```

## Step 2: switch the git hook

```sh
tree-commit install
```

This replaces the hook that `pre-commit install` created. That hook is kept, but it is not run (tree-commit already runs your config). `tree-commit uninstall` puts it back.

## Step 3 (optional): split the config

The reason to switch is to move project-specific hooks next to the projects. Take a root config like this:

```yaml
# .pre-commit-config.yaml
repos:
  - repo: https://github.com/astral-sh/ruff-pre-commit
    rev: v0.17.6
    hooks:
      - id: ruff-check
        files: ^services/billing/
  - repo: local
    hooks:
      - id: billing-tests
        name: billing tests
        entry: bash -c 'cd services/billing && uv run pytest -q'
        language: system
        files: ^services/billing/
        pass_filenames: false
```

It becomes `services/billing/.tree-commit-config.yaml`, without the `files:` prefixes or the `cd`:

```yaml
repos:
  - repo: https://github.com/astral-sh/ruff-pre-commit
    rev: v0.17.6
    hooks:
      - id: ruff-check
  - repo: local
    hooks:
      - id: billing-tests
        name: billing tests
        entry: uv run pytest -q
        language: system
        pass_filenames: false
```

Things to keep in mind when moving hooks into a subdirectory:

- **Patterns become relative** to the new directory: `^services/billing/src/` becomes `^src/`.
- **Hooks run from the new directory**, and receive file names relative to it.
- **Same `id` means override**: if the root config and the subdirectory both define `ruff-check`, only the subdirectory's runs on its files. Give hooks different ids if you want both to run.

## What is different

| | pre-commit | tree-commit |
| --- | --- | --- |
| Config files | One, at the root | One per directory |
| Hooks run from | Repository root | The config's directory |
| Same hook id in two configs | n/a | The closest config wins |
| Unstaged changes | Stashed while hooks run | **Not stashed**: hooks see your working copy |
| Remote hook languages | All | `system`, `script`, `python`, `pygrep`, `fail`; others are skipped |
| Hook types installed | `pre-commit`, `pre-push`, `commit-msg`, … | `pre-commit` only; run other stages with `--hook-stage` |
| `meta` hooks, `minimum_pre_commit_version`, `ci:` | Supported | Ignored |
| Parallelism | Within a hook | Also across hooks that touch different files |

The most important difference is **unstaged changes**. If you staged only part of a file, hooks check the whole file as it is on disk, and formatters may change lines you did not stage. Stage whole files, or stash first with `git stash --keep-index`.

## Hooks in languages tree-commit can't install

Remote hooks written in Node, Go, Rust and other languages are reported as `Skipped`. Replace them with a local hook that runs the tool, for example through `npx` or `uvx`:

```yaml
# Before
- repo: https://github.com/pre-commit/mirrors-prettier
  rev: v3.1.0
  hooks:
    - id: prettier

# After
- repo: local
  hooks:
    - id: prettier
      name: prettier
      entry: npx --yes prettier@3.1.0 --write --ignore-unknown
      language: system
```
