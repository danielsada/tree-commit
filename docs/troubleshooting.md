# Troubleshooting

Start with a dry run. A dry run shows which hooks run, on which files, and from which config file:

```sh
tree-commit run --dry-run --all-files
```

## A hook does not run

If a hook does not run, do these checks:

- **Make sure that the file is in the directory of the config file, or below it.** A config file applies only to files in its directory and in its subdirectories. `tree-commit tree` shows the locations of your config files.
- **Make sure that the patterns are relative to the config file.** In `frontend/.tree-commit-config.yaml`, `files: ^frontend/src/` never matches. Use `^src/`.
- **Make sure that the file is staged.** Without flags, tree-commit checks only staged files. Try `--all-files` or `--files`.
- **Look for a closer config file with the same `id`.** The closest config file wins, and it can have different `files` or `disable: true`.
- **Check if the file is a symlink.** tree-commit skips symlinks, unless the hook sets `types: [symlink]`.
- **Check if the hook is limited to a different stage.** Check `stages` and `default_stages`.
- **Check if the hook is in `SKIP`.** Check `echo $SKIP`.

## `executable "x" not found on PATH`

The command of the hook is not installed. Install it, or run it through a tool runner, so that other users do not have to install it:

```yaml
entry: uvx ruff check      # instead of: ruff check
entry: npx --no-install eslint
```

tree-commit resolves relative `PATH` entries (such as `node_modules/.bin`) from the directory of the config file, where the hook runs.

## `files were modified by this hook`

This message does not show an error in your code. It shows that a formatter or fixer changed files. Review the changes, `git add` them, and commit again.

With `pass_filenames: false`, any change to tracked files in the directory of the config file counts. Because of this, a hook that writes generated files also causes this message.

## `python 3 is required for python hooks but was not found`

Remote hooks in Python need Python 3 to create a virtual environment. Local `python` hooks with `additional_dependencies` also need it. Install Python 3, or set tree-commit to use a specific interpreter:

```sh
export TREE_COMMIT_PYTHON=/usr/bin/python3.12
```

On Windows, make sure that `python` is a real interpreter and not the Microsoft Store shortcut.

## `pip install failed` or a remote repository does not download

If pip fails or a remote repository does not download, do these checks:

- Check your network and proxy settings. git and pip use your usual settings.
- Make sure that `rev` exists in the repository.
- Run `tree-commit clean` to delete the cache, and try again.

## `hook id not found in <repo> at <rev>`

The repository does not define that hook at that version. Read the `.pre-commit-hooks.yaml` of the repository at that tag. Hook ids sometimes change between versions. For example, ruff-pre-commit added `ruff-check` next to the older `ruff`.

## `invalid config file`

tree-commit stops when a config file is not valid, so that a typo never silently disables your checks. The message gives the file and the problem, for example `repos[0] (https://…): missing required field 'rev'`.

## A hook changes lines that you did not stage

tree-commit does not stash unstaged changes, so hooks see the files as they are on disk. To prevent this problem, stage full files, or run these commands:

```sh
git stash --keep-index
git commit
git stash pop
```

## Windows

These rules apply on Windows:

- Write patterns with forward slashes (`^src/.*\.py$`). File names that tree-commit gives to hooks use backslashes.
- tree-commit runs scripts that have a `#!` line with their interpreter, or with the `sh` of Git.
- tree-commit supports `.cmd` and `.bat` tools (such as `eslint.cmd` from npm). tree-commit keeps their command lines short enough for `cmd.exe`.

## Get more help

To see the output of each hook, run with `--verbose`. Then open an issue with the output of `tree-commit tree` and `tree-commit run --dry-run`.
