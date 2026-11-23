# Command line

```
tree-commit [command] [flags]
```

| Command | What it does |
| --- | --- |
| [`run`](#run) | Run hooks |
| [`install`](#install-and-uninstall) | Install tree-commit as the git `pre-commit` hook |
| [`uninstall`](#install-and-uninstall) | Remove it again |
| [`tree`](#tree) | Show the configs that were found |
| [`clean`](#clean) | Delete cached hook repositories and environments |
| `completion` | Print a shell completion script (`tree-commit completion --help`) |

Flags:

| Flag | Meaning |
| --- | --- |
| `-c`, `--config <name>` | Config file name to look for in each directory, instead of `.tree-commit-config.yaml`. Works with every command. |
| `--version` | Print the version (`tree-commit -v` also works; `tree-commit run -v` means `--verbose`). |
| `-h`, `--help` | Help for any command. |

## run

```sh
tree-commit run [hook-id] [flags]
```

Runs the hooks that apply to the selected files. By default the selected files are the ones **staged** for commit (deleted files are left out).

| Flag | Meaning |
| --- | --- |
| `hook-id` | Run only the hook with this id. |
| `-a`, `--all-files` | Check every file tracked by git. |
| `--files a,b,c` | Check these files instead. Separate them with commas, or repeat the flag. |
| `--dry-run` | Print which hooks would run, on which files and in what order, without running them. Remote hook repositories are still downloaded, because their hook definitions are needed. |
| `--hook-stage <stage>` | Only run hooks for this stage. Default `pre-commit`. |
| `-j`, `--jobs <n>` | Run at most this many hooks at the same time. Default: the number of CPUs. |
| `-v`, `--verbose` | Show the output and duration of passing hooks too. |
| `--color auto\|always\|never` | Color the output. `auto` colors when printing to a terminal. |

You can run `tree-commit run` from any directory inside the repository.

### Output

```
ruff check (backend/)....................................................Failed
- hook id: ruff-check
- exit code: 1

F821 Undefined name `order`
 --> app.py:2:41
```

- One line per hook: `Passed`, `Failed` or `Skipped`.
- Hooks from a nested config show their directory, here `(backend/)`.
- A failing hook shows its id, exit code, error and output. `- files were modified by this hook` means it changed files: review them and `git add` them.
- `Skipped` hooks say why, for example an unsupported language.

### Exit code

| Code | Meaning |
| --- | --- |
| `0` | All hooks passed, or nothing had to run. |
| `1` | A hook failed or modified files, or tree-commit hit an error (it prints `tree-commit: <error>`). |

## install and uninstall

```sh
tree-commit install
tree-commit uninstall
```

`install` writes the `pre-commit` hook of the current repository. It uses git's own hooks directory, so `core.hooksPath` and `git worktree` checkouts work.

- If there already is a `pre-commit` hook, it is renamed `pre-commit.tree-commit-legacy` and still runs **before** tree-commit.
- If that hook was installed by the pre-commit framework, it is kept but not run, because tree-commit reads `.pre-commit-config.yaml` itself.
- `uninstall` removes the tree-commit hook and puts the previous hook back.
- Installing twice is safe.

The hook runs the `tree-commit` binary you installed it with, or the one on your `PATH` if that one is gone.

## tree

```sh
tree-commit tree [dir]
```

Prints the configs found in the repository (or in `dir`):

```
└── 📁 my-repo/ (📄 .tree-commit-config.yaml)
    ├── 📁 backend/ (📄 .tree-commit-config.yaml)
    └── 📁 frontend/ (📄 .tree-commit-config.yaml)
```

## clean

```sh
tree-commit clean
```

Deletes the cache of cloned hook repositories and Python environments. See [Hooks and languages](hooks-and-languages.md#the-cache).

## Environment variables

| Variable | Meaning |
| --- | --- |
| `SKIP` | Comma-separated hook ids to skip, e.g. `SKIP=pytest,eslint git commit`. |
| `TREE_COMMIT_HOME` | Cache directory. |
| `TREE_COMMIT_PYTHON` | Python interpreter used to create virtual environments. |
| `NO_COLOR` | Set to anything to disable colors. |
