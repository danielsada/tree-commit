# Hooks and languages

A hook is a command plus rules about which files it gets. Hooks are defined in one of two places:

- **Local hooks** (`repo: local`) are defined in your config.
- **Remote hooks** come from another git repository that has a `.pre-commit-hooks.yaml` file, like [pre-commit-hooks](https://github.com/pre-commit/pre-commit-hooks) or [ruff-pre-commit](https://github.com/astral-sh/ruff-pre-commit). You pick them by `id`.

## How a hook runs

1. tree-commit picks the files the hook applies to (see [How it works](how-it-works.md)).
2. It runs `entry`, then `args`, then the file names, **from the config's directory**. File names are relative to that directory.
3. If there are many files, the command runs several times with a batch of files each, so the command line does not get too long.
4. The hook **fails** if the command exits with a non-zero code, or if it **changed** any of the files.

With `pass_filenames: false`, the command runs once without file names. tree-commit then watches the whole config directory for changes, so a `go generate` or `go mod tidy` hook that rewrites other files also fails the commit.

## Languages

| `language` | Local hooks | Remote hooks |
| --- | --- | --- |
| `system` | ✅ Runs `entry` as found on your `PATH`. | ✅ |
| `script` | ✅ `entry` is a script path, relative to the config's directory. | ✅ Relative to the hook repository. |
| `python` | ✅ Runs from `PATH`. With `additional_dependencies`, runs in a virtual environment with those packages. | ✅ The repository is installed in a cached virtual environment. |
| `pygrep` | ✅ Built in, see below. | ✅ |
| `fail` | ✅ Built in, see below. | ✅ |
| anything else (`node`, `golang`, `rust`, `docker`, …) | ✅ Treated like `system`: the tool must be on your `PATH`. | ⏭️ Reported as `Skipped`. |

Tip: `uvx` and `npx` make most tools available without installing anything globally, and work with `language: system`:

```yaml
- id: ruff-check
  name: ruff check
  entry: uvx ruff@0.17.6 check --fix   # pinned version, cached by uv
  language: system
  types: [python]
```

### `pygrep`: forbid a pattern

`entry` is a regular expression. The hook fails for every line that matches:

```yaml
- id: no-print
  name: no print() in library code
  entry: '^\s*print\('
  language: pygrep
  types: [python]
```

Options, set with `args`:

- `-i` / `--ignore-case`
- `--multiline`: match across lines
- `--negate`: fail for files that do **not** contain the pattern, e.g. a required license header

### `fail`: forbid files

The hook fails for every matching file and prints `entry` as the message:

```yaml
- id: no-rej-files
  name: no .rej files
  entry: Merge leftovers must not be committed
  language: fail
  files: \.(rej|orig)$
```

## Remote hook repositories

```yaml
repos:
  - repo: https://github.com/astral-sh/ruff-pre-commit
    rev: v0.17.6
    hooks:
      - id: ruff-check
        args: [--fix]    # overrides the repository's defaults
      - id: ruff-format
```

- The repository is cloned at `rev` the first time it is needed, then reused from the cache.
- Python hooks get their own virtual environment with the repository installed. This needs Python 3 on your `PATH`, or set `TREE_COMMIT_PYTHON`.
- Any option you set (`args`, `files`, `exclude`, `types`, `stages`, …) replaces the repository's value.
- `repo` can also be a local path, which is handy for testing your own hook repository.

### The cache

| What | Where |
| --- | --- |
| Cache directory | `$TREE_COMMIT_HOME`, or `tree-commit` inside your user cache directory: `~/.cache` on Linux, `~/Library/Caches` on macOS, `%LocalAppData%` on Windows |
| Cloned repositories | `repos/` |
| Python environments | `envs/` |

Repositories are cached by URL and `rev`. If `rev` is a branch name, tree-commit does not notice when the branch moves, so prefer tags or commit hashes. Run `tree-commit clean` to delete the cache. It is rebuilt on the next run.

Several tree-commit processes can share the cache safely.
