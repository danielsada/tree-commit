# Configuration

## Where configs live

In every directory, tree-commit looks for:

1. `.tree-commit-config.yaml`, or
2. `.pre-commit-config.yaml` if the first one does not exist.

Only one config per directory is used. Configs are found with git, so files ignored by `.gitignore` (for example inside `node_modules/`) are never picked up. Untracked config files are found too, so you can try a config before committing it.

Use `tree-commit tree` to see which configs were found.

## File format

```yaml
# Optional: only consider files matching / not matching these patterns.
files: ''
exclude: ''
# Optional: stages for hooks that do not set their own.
default_stages: [pre-commit]

repos:
  - repo: local                 # hooks defined in this file
    hooks:
      - id: my-hook
        name: My hook
        entry: my-command --flag
        language: system

  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v6.0.0                 # tag or commit to use (required for remote repos)
    hooks:
      - id: trailing-whitespace # hook defined by that repository
```

### Top-level keys

| Key | Meaning |
| --- | --- |
| `repos` | List of hook repositories. Required. |
| `files` | Regular expression. Only files matching it are considered by any hook of this config. |
| `exclude` | Regular expression. Files matching it are ignored by every hook of this config. |
| `default_stages` | Stages for hooks without `stages`. See [Stages](#stages). |

### Repository keys

| Key | Meaning |
| --- | --- |
| `repo` | `local`, or the URL (or local path) of a git repository containing a `.pre-commit-hooks.yaml`. |
| `rev` | Tag, branch or commit of a remote repository. Required unless `repo` is `local`. |
| `hooks` | The hooks to use. |

## Hook options

For `repo: local` hooks you define everything. For remote hooks you only need `id`; the rest comes from the repository, and anything you set **overrides** it.

| Option | Default | Meaning |
| --- | --- | --- |
| `id` | | Required. Identifies the hook. A hook with the same `id` in a closer config replaces this one. |
| `name` | the `id` | Shown in the output. |
| `entry` | | Required for local hooks. The command to run. Quotes work like in a shell. |
| `language` | `system` | How to run the command. See [Hooks and languages](hooks-and-languages.md). |
| `args` | `[]` | Extra arguments, added after `entry`. |
| `files` | everything | Regular expression a file path must match. |
| `exclude` | nothing | Regular expression of file paths to skip. |
| `types` | `[file]` | The file must have **all** of these types, e.g. `[python]`. |
| `types_or` | | The file must have **at least one** of these types, e.g. `[javascript, ts]`. |
| `exclude_types` | | Skip files with any of these types. |
| `pass_filenames` | `true` | Add the matching file names to the command. Set to `false` for commands like `pytest` or `go test ./...`. |
| `always_run` | `false` | Run even when no file matches (still only when the config's directory has changes, or with `--all-files`). |
| `stages` | all | Git hook stages the hook runs in. |
| `additional_dependencies` | | Python packages to install for `language: python` hooks. |
| `verbose` | `false` | Show the hook's output even when it passes. |
| `include_folders` | everything | tree-commit only. Only check files inside these folders. |
| `exclude_folders` | nothing | tree-commit only. Skip files inside these folders, at any depth. |
| `disable` | `false` | tree-commit only. Turn off a hook inherited from a parent config. |

### Paths and patterns

All paths in a config are **relative to the config's directory**:

```yaml
# frontend/.tree-commit-config.yaml
- id: eslint
  entry: npx eslint
  language: system
  files: ^src/.*\.tsx?$     # matches frontend/src/App.tsx, as "src/App.tsx"
```

- Patterns are regular expressions, matched anywhere in the path (use `^` and `$` to anchor them).
- Always write paths in patterns with forward slashes `/`, also on Windows.
- Python's verbose mode `(?x)` is supported, so multi-line patterns copied from pre-commit configs work.

### File types

Types are detected from the file name, extension and first line (`#!` shebang). Common ones:

`file`, `symlink`, `text`, `binary`, `executable`, `python`, `pyi`, `javascript`, `jsx`, `ts`, `tsx`, `go`, `rust`, `java`, `shell`, `bash`, `yaml`, `json`, `toml`, `markdown`, `html`, `css`, `scss`, `sql`, `dockerfile`, `makefile`, `terraform`, `image`.

`types` defaults to `[file]`, so symlinks are not passed to hooks unless a hook asks for them with `types: [symlink]`.

### Folder filters

`include_folders` and `exclude_folders` are an easier alternative to regular expressions for the common case:

```yaml
- id: complexity
  entry: uvx ruff check --select C90
  language: system
  types: [python]
  include_folders: [src/]                       # only files under src/
  exclude_folders: [src/generated/, __pycache__/]
```

A folder in `exclude_folders` is skipped wherever it appears: `build/` excludes `build/x.py` and also `pkg/build/x.py`. A folder with several parts, such as `src/generated/`, must match those parts in order.

### Disabling an inherited hook

```yaml
# vendor/.tree-commit-config.yaml
repos:
  - repo: local
    hooks:
      - id: ruff-check   # defined in the root config
        disable: true
```

The root `ruff-check` no longer runs on files under `vendor/`. A disabled hook needs nothing but its `id`.

### Stages

tree-commit installs itself as the `pre-commit` git hook, and `tree-commit run` uses the `pre-commit` stage by default. A hook with `stages: [pre-push]` (or the older name `push`) is skipped there. Run it with:

```sh
tree-commit run --hook-stage pre-push --all-files
```

`default_stages` sets the stages of every hook in that config that does not set `stages` itself.

## Validation

tree-commit stops with an error naming the file when a config is invalid: broken YAML, a missing `repo`, `rev` or `id`, a local hook without `entry`, or a `repo`/`rev` that starts with `-`. A broken config never silently turns checks off.

An invalid regular expression in a hook, or a hook `id` that a remote repository does not define, makes **that hook** fail, while the others still run.
