# Configuration

## Location of config files

In each directory, tree-commit looks for one of these config files:

1. `.tree-commit-config.yaml`, or
2. `.pre-commit-config.yaml`, if `.tree-commit-config.yaml` does not exist.

tree-commit uses only one config file in each directory. tree-commit uses git to find config files. Thus, it never finds config files that `.gitignore` ignores (for example, in `node_modules/`). tree-commit also finds untracked config files. Thus, you can try a config file before you commit it.

To see the config files that tree-commit found, run `tree-commit tree`.

## File format

This example shows the structure of a config file:

```yaml
# Optional: only use files that match / do not match these patterns.
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
| `repos` | A list of hook repositories. Required. |
| `files` | A regular expression. The hooks in this config file use only the files that match it. |
| `exclude` | A regular expression. All hooks in this config file ignore the files that match it. |
| `default_stages` | The stages for hooks that do not set `stages`. See [Stages](#stages). |

### Repository keys

| Key | Meaning |
| --- | --- |
| `repo` | `local`, or the URL (or local path) of a git repository that contains a `.pre-commit-hooks.yaml` file. |
| `rev` | The tag, branch, or commit of a remote repository. Required if `repo` is not `local`. |
| `hooks` | The hooks to use. |

## Hook options

For `repo: local` hooks, you define all the options. For remote hooks, only `id` is necessary. The hook repository supplies the other options. An option that you set **overrides** the value from the hook repository.

These are the hook options:

| Option | Default | Meaning |
| --- | --- | --- |
| `id` | | Required. Identifies the hook. A hook with the same `id` in a closer config file replaces this hook. |
| `name` | the `id` | The name in the output. |
| `entry` | | Required for local hooks. The command to run. Quotes work as in a shell. |
| `language` | `system` | How tree-commit runs the command. See [Hooks and languages](hooks-and-languages.md). |
| `args` | `[]` | More arguments. tree-commit adds them after `entry`. |
| `files` | everything | A regular expression. The file path must match it. |
| `exclude` | nothing | A regular expression. tree-commit skips the file paths that match it. |
| `types` | `[file]` | The file must have **all** of these types, for example `[python]`. |
| `types_or` | | The file must have **at least one** of these types, for example `[javascript, ts]`. |
| `exclude_types` | | Skip the files that have one or more of these types. |
| `pass_filenames` | `true` | Add the file names that match to the command. Set to `false` for commands such as `pytest` or `go test ./...`. |
| `always_run` | `false` | Run also when no file matches. The hook still runs only when the directory of the config file has changes, or with `--all-files`. |
| `stages` | all | The git hook stages in which the hook runs. |
| `additional_dependencies` | | The Python packages to install for `language: python` hooks. |
| `verbose` | `false` | Show the output of the hook also when it passes. |
| `include_folders` | everything | tree-commit only. Check only the files in these directories. |
| `exclude_folders` | nothing | tree-commit only. Skip the files in these directories, at all depths. |
| `disable` | `false` | tree-commit only. Disable a hook that comes from a parent config file. |

### Paths and patterns

All paths in a config file are **relative to the directory of the config file**. For example:

```yaml
# frontend/.tree-commit-config.yaml
- id: eslint
  entry: npx eslint
  language: system
  files: ^src/.*\.tsx?$     # matches frontend/src/App.tsx, as "src/App.tsx"
```

These rules apply to patterns:

- Patterns are regular expressions. tree-commit matches them anywhere in the path. To anchor a pattern, use `^` and `$`.
- Always use forward slashes `/` for paths in patterns, also on Windows.
- tree-commit supports the Python verbose mode `(?x)`. Thus, multi-line patterns that you copy from pre-commit config files work.

### File types

tree-commit detects the types from the file name, the extension, and the first line (the `#!` shebang). These are the common types:

`file`, `symlink`, `text`, `binary`, `executable`, `python`, `pyi`, `javascript`, `jsx`, `ts`, `tsx`, `go`, `rust`, `java`, `shell`, `bash`, `yaml`, `json`, `toml`, `markdown`, `html`, `css`, `scss`, `sql`, `dockerfile`, `makefile`, `terraform`, `image`.

The default value of `types` is `[file]`. Thus, tree-commit does not give symlinks to a hook, unless the hook requests them with `types: [symlink]`.

### Directory filters

For the common case, you can use `include_folders` and `exclude_folders` instead of regular expressions. They are less complex:

```yaml
- id: complexity
  entry: uvx ruff check --select C90
  language: system
  types: [python]
  include_folders: [src/]                       # only files under src/
  exclude_folders: [src/generated/, __pycache__/]
```

tree-commit skips a directory in `exclude_folders` at all positions in the path. For example, `build/` excludes `build/x.py` and also `pkg/build/x.py`. If a directory has more than one part, such as `src/generated/`, these parts must match in the same order.

### Disable an inherited hook

This example disables a hook that the root config file defines:

```yaml
# vendor/.tree-commit-config.yaml
repos:
  - repo: local
    hooks:
      - id: ruff-check   # defined in the root config
        disable: true
```

The `ruff-check` hook from the root config file does not run on files in `vendor/` now. A disabled hook needs only its `id`.

### Stages

tree-commit installs itself as the `pre-commit` git hook. By default, `tree-commit run` uses the `pre-commit` stage. Thus, tree-commit skips a hook with `stages: [pre-push]` (or the older name `push`) in this stage. To run this hook, use this command:

```sh
tree-commit run --hook-stage pre-push --all-files
```

`default_stages` sets the stages of each hook in that config file that does not set `stages`.

## Validation

If a config file is not valid, tree-commit stops with an error that names the file. A config file is not valid if it has one of these problems:

- Broken YAML.
- A missing `repo`, `rev`, or `id`.
- A local hook without `entry`.
- A `repo` or `rev` that starts with `-`.

tree-commit never disables checks silently because of a broken config file.

If a hook has a regular expression that is not valid, **that hook** fails. A hook also fails if the remote hook repository does not define its `id`. In these two conditions, the other hooks continue to run.
