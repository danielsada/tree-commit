# tree-commit

**Git hooks for monorepos.** You can put a config file in any directory. Its hooks run only on the files in that directory.

tree-commit reads the same config file format as [pre-commit](https://pre-commit.com) and runs the same hooks. The difference is that you can have **one config file per directory**, not one config file for the full repository:

```
my-repo/
├── .tree-commit-config.yaml          ← whitespace, YAML and spelling checks for everything
├── backend/
│   ├── .tree-commit-config.yaml      ← ruff and pytest, only for backend/ files
│   └── app.py
└── frontend/
    ├── .tree-commit-config.yaml      ← eslint and prettier, only for frontend/ files
    └── src/App.tsx
```

If you change `backend/app.py` and commit, the root hooks and the backend hooks run. The frontend hooks do not run.

## Install

You need [Go](https://go.dev/dl/) 1.23 or newer and git.

Run this command to install tree-commit:

```sh
go install github.com/danielsada/tree-commit@latest
```

Run this command to make sure that tree-commit works:

```sh
tree-commit --version
```

## Quick start

**1. Add a config file** to any directory. This example is for a Python project. It uses [uv](https://docs.astral.sh/uv/) to run [ruff](https://docs.astral.sh/ruff/), so you do not have to install ruff:

```yaml
# backend/.tree-commit-config.yaml
repos:
  - repo: local
    hooks:
      - id: ruff-check
        name: ruff check
        entry: uvx ruff check --fix
        language: system
        types: [python]

      - id: pytest
        name: pytest
        entry: uv run pytest -q    # runs inside backend/
        language: system
        types: [python]
        pass_filenames: false
```

**2. Install the git hook** in your repository:

```sh
tree-commit install
```

**3. Commit your changes as usual.** tree-commit checks the files that you staged:

```console
$ git commit -m "Add the orders endpoint"
ruff check (backend/)....................................................Failed
- hook id: ruff-check
- exit code: 1

F821 Undefined name `order`
 --> app.py:2:41
  |
1 | def total(orders):
2 |     return sum(order.price for order in order)
  |                                         ^^^^^

Found 1 error.

pytest (backend/)........................................................Passed
trim trailing whitespace.................................................Passed
```

If a hook fails, or if a hook fixes a file, the commit stops. Then do these steps:

1. Read the output.
2. Use `git add` to stage the fixes.
3. Commit again.

## Common commands

These are the commands that you use most frequently:

```sh
tree-commit run                 # check staged files, like the git hook does
tree-commit run --all-files     # check all files in the repository
tree-commit run ruff-check      # run one hook only
tree-commit run --dry-run       # list the hooks to run, but do not run them
tree-commit tree                # show where your config files are
SKIP=pytest git commit          # skip a hook one time
tree-commit uninstall           # remove the git hook
```

## Use an existing pre-commit config file

You do not have to change your `.pre-commit-config.yaml` file. If a directory has no `.tree-commit-config.yaml`, tree-commit reads the `.pre-commit-config.yaml` in that directory. tree-commit downloads and runs hooks from hook repositories such as [pre-commit-hooks](https://github.com/pre-commit/pre-commit-hooks) and [ruff-pre-commit](https://github.com/astral-sh/ruff-pre-commit). For more information, see [Migrating from pre-commit](docs/migrating-from-pre-commit.md).

## Documentation

These pages give more information:

- [Getting started](docs/getting-started.md): a full walkthrough
- [Configuration](docs/configuration.md): all config file options
- [How it works](docs/how-it-works.md): nested config files, overrides, order of execution and parallel execution
- [Hooks and languages](docs/hooks-and-languages.md): local hooks, remote hook repositories, Python
- [Command line](docs/cli.md): commands, flags and environment variables
- [Recipes](docs/recipes.md): monorepo patterns, CI, uv and ruff, Go
- [Migrating from pre-commit](docs/migrating-from-pre-commit.md)
- [Troubleshooting](docs/troubleshooting.md)

The [example monorepo](example_repos/example_monorepo) shows all features in a realistic configuration.

## Contributing

See [Development](docs/development.md).
