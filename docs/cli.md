# Command line

```
tree-commit [command] [flags]
```

tree-commit has these commands:

| Command | What it does |
| --- | --- |
| [`run`](#run) | Run hooks. |
| [`install`](#install-and-uninstall) | Install tree-commit as the git `pre-commit` hook. |
| [`uninstall`](#install-and-uninstall) | Remove the tree-commit hook. |
| [`tree`](#tree) | Show the config files that tree-commit found. |
| [`clean`](#clean) | Delete the cached hook repositories and environments. |
| `completion` | Print a shell completion script (`tree-commit completion --help`). |

These are the global flags:

| Flag | Meaning |
| --- | --- |
| `-c`, `--config <name>` | The config file name to find in each directory, instead of `.tree-commit-config.yaml`. Works with all commands. |
| `--version` | Print the version. `tree-commit -v` also works, but `tree-commit run -v` means `--verbose`. |
| `-h`, `--help` | Show the help for a command. |

## run

```sh
tree-commit run [hook-id] [flags]
```

This command runs the hooks that apply to the selected files. By default, the selected files are the **staged** files. Deleted files are not included.

These are the arguments and flags of `run`:

| Flag | Meaning |
| --- | --- |
| `hook-id` | Run only the hook with this id. |
| `-a`, `--all-files` | Check all the files that git tracks. |
| `--files a,b,c` | Check these files instead of the staged files. Separate the files with commas, or repeat the flag. |
| `--dry-run` | Print the hooks that would run, their files, and their order. The hooks do not run. tree-commit still downloads remote hook repositories, because it needs their hook definitions. |
| `--hook-stage <stage>` | Run only the hooks for this stage. The default is `pre-commit`. |
| `-j`, `--jobs <n>` | The maximum number of hooks that run at the same time. The default is the number of CPUs. |
| `-v`, `--verbose` | Also show the output and duration of hooks that pass. |
| `--color auto\|always\|never` | Color the output. With `auto`, tree-commit uses color only when it prints to a terminal. |

You can run `tree-commit run` from any directory in the repository.

### Output

This is an example of the output for a hook that failed:

```
ruff check (backend/)....................................................Failed
- hook id: ruff-check
- exit code: 1

F821 Undefined name `order`
 --> app.py:2:41
```

The output has these parts:

- One line for each hook, with its result: `Passed`, `Failed`, or `Skipped`.
- Hooks from a nested config file show their directory, in this example `(backend/)`.
- A hook that fails shows its id, its exit code, the error, and the output. `- files were modified by this hook` means that the hook changed files. Review these files and `git add` them.
- A `Skipped` hook shows the reason, for example a language that tree-commit does not support.

### Exit code

| Code | Meaning |
| --- | --- |
| `0` | All hooks passed, or no hook had to run. |
| `1` | A hook failed or changed files, or tree-commit had an error. tree-commit prints `tree-commit: <error>`. |

## install and uninstall

```sh
tree-commit install
tree-commit uninstall
```

`install` writes the `pre-commit` hook of the current repository. It uses the hooks directory of git. Thus, `core.hooksPath` and `git worktree` checkouts work.

These rules apply:

- If a `pre-commit` hook exists, tree-commit renames it to `pre-commit.tree-commit-legacy`. This hook continues to run **before** tree-commit.
- If the pre-commit framework installed that hook, tree-commit keeps it but does not run it. The reason is that tree-commit reads `.pre-commit-config.yaml` itself.
- `uninstall` removes the tree-commit hook and restores the previous hook.
- It is safe to run `install` two times.

The hook runs the `tree-commit` binary that you used to install it. If that binary does not exist, the hook runs the `tree-commit` binary on your `PATH`.

## tree

```sh
tree-commit tree [dir]
```

This command prints the config files in the repository (or in `dir`):

```
└── 📁 my-repo/ (📄 .tree-commit-config.yaml)
    ├── 📁 backend/ (📄 .tree-commit-config.yaml)
    └── 📁 frontend/ (📄 .tree-commit-config.yaml)
```

## clean

```sh
tree-commit clean
```

This command deletes the cache of cloned hook repositories and Python environments. See [Hooks and languages](hooks-and-languages.md#the-cache).

## Environment variables

tree-commit uses these environment variables:

| Variable | Meaning |
| --- | --- |
| `SKIP` | A comma-separated list of hook ids to skip, for example `SKIP=pytest,eslint git commit`. |
| `TREE_COMMIT_HOME` | The cache directory. |
| `TREE_COMMIT_PYTHON` | The Python interpreter that tree-commit uses to create virtual environments. |
| `NO_COLOR` | Set to any value to disable colors. |
