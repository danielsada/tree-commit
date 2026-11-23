# tree-commit

**Git hooks for monorepos.** Put a hook config in any folder, and its checks only run on the files in that folder.

tree-commit reads the same config format as [pre-commit](https://pre-commit.com) and runs the same hooks. The difference is that you can have **one config per directory** instead of one for the whole repository:

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

Change `backend/app.py` and commit: the root checks and the backend checks run. The frontend checks don't.

## Install

You need [Go](https://go.dev/dl/) 1.23 or newer and git.

```sh
go install github.com/danielsada/tree-commit@latest
```

Check that it works:

```sh
tree-commit --version
```

## Quick start

**1. Add a config** to any folder. Here is one for a Python project, using [uv](https://docs.astral.sh/uv/) to run [ruff](https://docs.astral.sh/ruff/) without installing it:

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

**3. Commit as usual.** tree-commit checks the files you staged:

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

If a check fails, or a hook fixes a file for you, the commit stops. Look at the output, `git add` the fixes, and commit again.

## Everyday commands

```sh
tree-commit run                 # check staged files, like the git hook does
tree-commit run --all-files     # check every file in the repository
tree-commit run ruff-check      # run one hook only
tree-commit run --dry-run       # show which hooks would run, without running them
tree-commit tree                # show where your configs are
SKIP=pytest git commit          # skip a hook once
tree-commit uninstall           # remove the git hook
```

## Already using pre-commit?

Your `.pre-commit-config.yaml` works as it is: tree-commit reads it when a folder has no `.tree-commit-config.yaml`. Hooks from repositories such as [pre-commit-hooks](https://github.com/pre-commit/pre-commit-hooks) and [ruff-pre-commit](https://github.com/astral-sh/ruff-pre-commit) are downloaded and run for you. See [Migrating from pre-commit](docs/migrating-from-pre-commit.md).

## Documentation

- [Getting started](docs/getting-started.md): a full walkthrough
- [Configuration](docs/configuration.md): every config option
- [How it works](docs/how-it-works.md): nested configs, overrides, ordering and parallelism
- [Hooks and languages](docs/hooks-and-languages.md): local hooks, remote hook repositories, Python
- [Command line](docs/cli.md): commands, flags and environment variables
- [Recipes](docs/recipes.md): monorepo patterns, CI, uv and ruff, Go
- [Migrating from pre-commit](docs/migrating-from-pre-commit.md)
- [Troubleshooting](docs/troubleshooting.md)

The [example monorepo](example_repos/example_monorepo) shows every feature in a realistic setup.

## Contributing

See [Development](docs/development.md).
