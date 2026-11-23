# Hooks and languages

A hook is a command and the rules that tell which files it gets. You define a hook in one of two places:

- **Local hooks** (`repo: local`) are defined in your config file.
- **Remote hooks** come from a hook repository. This is a git repository that has a `.pre-commit-hooks.yaml` file, like [pre-commit-hooks](https://github.com/pre-commit/pre-commit-hooks) or [ruff-pre-commit](https://github.com/astral-sh/ruff-pre-commit). You select these hooks by `id`.

## How a hook runs

tree-commit runs a hook in these steps:

1. tree-commit finds the files that the hook applies to (see [How it works](how-it-works.md)).
2. It runs `entry`, then `args`, then the file names, **from the directory of the config file**. The file names are relative to that directory.
3. If there are many files, the command runs several times, with a batch of files each time. This prevents a command line that is too long.
4. The hook **fails** if the command exits with a non-zero code, or if it **changed** any of the files.

With `pass_filenames: false`, the command runs one time without file names. tree-commit then looks for changes in the full directory of the config file. Thus, a `go generate` or `go mod tidy` hook that changes other files also makes the commit fail.

## Languages

| `language` | Local hooks | Remote hooks |
| --- | --- | --- |
| `system` | ✅ Runs `entry` as found on your `PATH`. | ✅ |
| `script` | ✅ `entry` is a script path, relative to the config file directory. | ✅ Relative to the hook repository. |
| `python` | ✅ Runs from `PATH`. With `additional_dependencies`, runs in a virtual environment with those packages. | ✅ tree-commit installs the repository in a cached virtual environment. |
| `node` | ✅ Same as `system`. With `additional_dependencies`, runs with those npm packages installed. | ✅ tree-commit installs the repository with npm. Node.js and npm must be on your `PATH`. |
| `golang` | ✅ Same as `system`. With `additional_dependencies` (`module@version`), runs with those commands installed. | ✅ tree-commit builds the repository with `go install`. Go must be on your `PATH`. |
| `grep`, `pygrep` | ✅ Built in, see below. | ✅ |
| `fail` | ✅ Built in, see below. | ✅ |
| all other values (`rust`, `docker`, `ruby`, …) | ✅ Same as `system`: the tool must be on your `PATH`. | ⏭️ Reported as `Skipped`. |

With `uvx` and `npx`, most tools are available without a global installation. They work with `language: system`:

```yaml
- id: ruff-check
  name: ruff check
  entry: uvx ruff@0.17.6 check --fix   # pinned version, cached by uv
  language: system
  types: [python]
```

### `grep`: forbid a pattern

`grep` is a built-in language of tree-commit. `entry` is a regular expression. The hook fails for each line that matches it:

```yaml
- id: no-print
  name: no print() in library code
  entry: '^\s*print\('
  language: grep
  types: [python]
```

The regular expressions use Python syntax, as in pre-commit. For example, lookahead works: `TODO(?!\(#\d+\))` matches a TODO without a ticket number.

You can set these options with `args`:

- `-i` / `--ignore-case`
- `--multiline`: match across lines. tree-commit reports only the first match of each file.
- `--negate`: fail for files that do **not** contain the pattern, for example a required license header.

`pygrep` is the same language, with the name that pre-commit uses. Hooks from repositories such as [pygrep-hooks](https://github.com/pre-commit/pygrep-hooks) use it.

### Baselines: adopt a rule step by step

A new rule often has many violations in existing code. A baseline lets you add the rule now and fix the old violations later:

- The baseline file records the violations that exist now. tree-commit accepts them.
- A new violation fails the hook.
- When you fix a violation, tree-commit removes it from the baseline. The baseline can only get smaller (it is a ratchet).

To use a baseline, do these steps:

1. Add `baseline` to a `grep` or `pygrep` hook. The path is relative to the directory of the config file:

   ```yaml
   - id: todo-needs-ticket
     name: TODO needs a ticket
     entry: 'TODO(?!\(#\d+\))'
     language: grep
     types_or: [python, go, ts]
     baseline: .grep-baseline.json
   ```

1. Record the current violations:

   ```sh
   tree-commit baseline todo-needs-ticket
   ```

1. Commit the config file and the baseline file.

When a commit fixes a violation, tree-commit updates the baseline file. Then the hook fails, as a formatter does. Stage the baseline file and commit again.

The `baseline_mode` option sets how the baseline identifies violations:

| `baseline_mode` | The baseline records | A violation is new when |
| --- | --- | --- |
| `lines` (default) | The text of each matching line, without the indentation | Its text is not in the baseline, or it occurs more times than recorded |
| `count` | The number of violations in each file | The file has more violations than recorded |

`lines` mode continues to accept a violation when you move or reindent its line. `count` mode makes smaller baseline files, and it does not change when the text of a line changes. In `count` mode, tree-commit cannot tell which violation is new, so it reports all violations of the file.

More than one hook can use the same baseline file. tree-commit keeps the entries of each hook separate. A hook never checks its own baseline file.

### `fail`: forbid files

The hook fails for each file that matches. It prints `entry` as the message:

```yaml
- id: no-rej-files
  name: no .rej files
  entry: Merge leftovers must not be committed
  language: fail
  files: \.(rej|orig)$
```

## Remote hook repositories

This example uses two hooks from a hook repository:

```yaml
repos:
  - repo: https://github.com/astral-sh/ruff-pre-commit
    rev: v0.17.6
    hooks:
      - id: ruff-check
        args: [--fix]    # overrides the repository's defaults
      - id: ruff-format
```

These rules apply to remote hooks:

- tree-commit clones the repository at `rev` the first time that it needs it. After that, it uses the copy in the cache.
- Python hooks get their own virtual environment, with the repository installed in it. If [uv](https://docs.astral.sh/uv/) is on your `PATH`, tree-commit uses it to create the environment. This is faster, and uv can download Python if necessary. Without uv, Python 3 must be on your `PATH`, or you must set `TREE_COMMIT_PYTHON`.
- `repo: builtin` (from [prek](https://github.com/j178/prek)) does not need a `rev`. tree-commit runs these hooks from [pre-commit-hooks](https://github.com/pre-commit/pre-commit-hooks) v6.0.0.
- Each option that you set (`args`, `files`, `exclude`, `types`, `stages`, …) replaces the value from the repository.
- `repo` can also be a local path. This is useful when you test your own hook repository.

### The cache

| What | Where |
| --- | --- |
| Cache directory | `$TREE_COMMIT_HOME`, or `tree-commit` in your user cache directory: `~/.cache` on Linux, `~/Library/Caches` on macOS, `%LocalAppData%` on Windows |
| Cloned repositories | `repos/` |
| Python, Node.js and Go environments | `envs/` |

tree-commit caches repositories by URL and `rev`. If `rev` is a branch name, tree-commit does not see when the branch moves. Thus, use tags or commit hashes when possible. Run `tree-commit clean` to delete the cache. tree-commit builds the cache again on the next run.

Many tree-commit processes can safely use the same cache.
