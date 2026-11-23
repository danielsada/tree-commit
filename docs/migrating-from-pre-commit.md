# Migrating from pre-commit

tree-commit reads pre-commit config files and runs pre-commit hooks. Because of this, most repositories migrate without changes.

## Step 1: Keep your config file

Keep `.pre-commit-config.yaml` in its current location. tree-commit uses it in each directory that has no `.tree-commit-config.yaml`.

To see what tree-commit does, do a dry run first. Then run the hooks on all files:

```sh
tree-commit run --all-files --dry-run
tree-commit run --all-files
```

## Step 2: Replace the git hook

Run this command:

```sh
tree-commit install
```

This command replaces the hook that `pre-commit install` created. tree-commit keeps the old hook, but does not run it, because tree-commit already runs your config file. `tree-commit uninstall` restores the old hook.

## Step 3 (optional): Split the config file

The reason to change to tree-commit is to move project-specific hooks into the directories of the projects. For example, start with this root config file:

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

Move these hooks to `services/billing/.tree-commit-config.yaml`. Remove the `files:` prefixes and the `cd`:

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

When you move hooks into a subdirectory, remember these rules:

- **Patterns become relative** to the new directory: `^services/billing/src/` becomes `^src/`.
- **Hooks run from the new directory**, and get file names relative to it.
- **The same `id` means override**: if the root config file and the subdirectory both define `ruff-check`, only the hook in the subdirectory runs on its files. If you want both hooks to run, give them different ids.

## Differences from pre-commit

This table shows the differences between pre-commit and tree-commit:

| | pre-commit | tree-commit |
| --- | --- | --- |
| Config files | One, at the root | One for each directory |
| Hooks run from | Repository root | The directory of the config file |
| Same hook id in two config files | n/a | The closest config file wins |
| Remote hook languages | All | `system`, `script`, `python`, `node`, `golang`, `pygrep`, `fail`; other languages are skipped. tree-commit also has `grep`, with baselines |
| Hook types installed | `pre-commit`, `pre-push`, `commit-msg`, … | `pre-commit` only; run other stages with `--hook-stage` |
| `meta` hooks, `minimum_pre_commit_version`, `ci:` | Supported | Ignored |
| prek's `repo: builtin` | Not supported | Runs the hooks from pre-commit-hooks v6.0.0 |
| Parallelism | In a hook | Also across hooks that touch different files |

As in pre-commit, tree-commit removes unstaged changes while hooks run on staged files, and restores them after the hooks.

## Hooks in languages that tree-commit cannot install

Remote hooks in Node.js and Go run when Node.js and npm, or Go, are on your `PATH`. tree-commit reports remote hooks in other languages (for example Rust, Docker and Ruby) as `Skipped`. Replace each of these hooks with a hook that runs the tool from your `PATH`. Many hook repositories have such a hook:

```yaml
# Before: a docker_image hook, which tree-commit skips
- repo: https://github.com/hadolint/hadolint
  rev: v2.15.1
  hooks:
    - id: hadolint-docker

# After: the same repository's hook that runs hadolint from your PATH
- repo: https://github.com/hadolint/hadolint
  rev: v2.15.1
  hooks:
    - id: hadolint
```
