# Troubleshooting

Start with a dry run. It shows which hooks would run, on which files, from which config:

```sh
tree-commit run --dry-run --all-files
```

## A hook does not run

- **Is the file below the config's directory?** A config only applies to files in its own directory and below. `tree-commit tree` shows where your configs are.
- **Are patterns relative to the config?** In `frontend/.tree-commit-config.yaml`, `files: ^frontend/src/` never matches. Use `^src/`.
- **Is the file staged?** Without flags, only staged files are checked. Try `--all-files` or `--files`.
- **Does a closer config use the same `id`?** The closest config wins, and it may have different `files` or `disable: true`.
- **Is it a symlink?** Symlinks are skipped unless the hook sets `types: [symlink]`.
- **Is the hook limited to another stage?** Check `stages` and `default_stages`.
- **Is it in `SKIP`?** Check `echo $SKIP`.

## `executable "x" not found on PATH`

The hook's command is not installed. Install it, or run it through a tool runner so nobody has to:

```yaml
entry: uvx ruff check      # instead of: ruff check
entry: npx --no-install eslint
```

Relative `PATH` entries (like `node_modules/.bin`) are resolved from the config's directory, where the hook runs.

## `files were modified by this hook`

Not an error in your code: a formatter or fixer changed files. Review the changes, `git add` them, and commit again.

With `pass_filenames: false`, any change to tracked files in the config's directory counts, so a hook that writes generated files also triggers this.

## `python 3 is required for python hooks but was not found`

Remote hooks written in Python (and local `python` hooks with `additional_dependencies`) need Python 3 to create a virtual environment. Install it, or point tree-commit to an interpreter:

```sh
export TREE_COMMIT_PYTHON=/usr/bin/python3.12
```

On Windows, make sure `python` is a real interpreter and not the Microsoft Store shortcut.

## `pip install failed` or a remote repository does not download

- Check your network and proxy settings. git and pip use your usual configuration.
- Check that `rev` exists in the repository.
- Run `tree-commit clean` to delete the cache, and try again.

## `hook id not found in <repo> at <rev>`

The repository does not define that hook at that version. Look at its `.pre-commit-hooks.yaml` at that tag. Hook ids sometimes change between versions (for example ruff-pre-commit added `ruff-check` next to the older `ruff`).

## `invalid config file`

tree-commit stops when a config is invalid, so a typo never silently disables your checks. The message names the file and the problem, for example `repos[0] (https://…): missing required field 'rev'`.

## A hook changes lines I did not stage

tree-commit does not stash unstaged changes, so hooks see the files as they are on disk. Stage whole files, or run:

```sh
git stash --keep-index
git commit
git stash pop
```

## Windows

- Write patterns with forward slashes (`^src/.*\.py$`). File names passed to hooks use backslashes.
- Scripts with a `#!` line are run with their interpreter, or with Git's `sh`.
- `.cmd` and `.bat` tools (such as npm's `eslint.cmd`) are supported. tree-commit keeps their command lines short enough for `cmd.exe`.

## Still stuck?

Run with `--verbose` to see the output of every hook, and open an issue with the output of `tree-commit tree` and `tree-commit run --dry-run`.
