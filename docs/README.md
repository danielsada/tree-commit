# tree-commit documentation

tree-commit runs [pre-commit](https://pre-commit.com) compatible git hooks, configured per directory.

| Page | Read it when you want to… |
| --- | --- |
| [Getting started](getting-started.md) | set up tree-commit in a repository for the first time |
| [Configuration](configuration.md) | look up a config option |
| [How it works](how-it-works.md) | understand which hooks run on which files, and in what order |
| [Hooks and languages](hooks-and-languages.md) | write local hooks or use hooks from other repositories |
| [Command line](cli.md) | look up a command, flag or environment variable |
| [Recipes](recipes.md) | copy a ready-made setup (uv and ruff, Go, CI, …) |
| [Migrating from pre-commit](migrating-from-pre-commit.md) | move an existing pre-commit setup over |
| [Troubleshooting](troubleshooting.md) | fix an error |
| [Development](development.md) | work on tree-commit itself |

## The idea in one minute

- A config file is a `.tree-commit-config.yaml`, in the same format as `.pre-commit-config.yaml`.
- You can put one in **any directory**. Its hooks only check files **below that directory**.
- Inside a config, paths are **relative to its directory**, and hooks **run from that directory**. `go test ./...` in `go_lib/.tree-commit-config.yaml` tests `go_lib`.
- If a parent and a child directory both define a hook with the same `id`, the **closest config wins** for files in the child directory.
- Hooks that touch different files **run in parallel**.
