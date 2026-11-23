# tree-commit documentation

tree-commit runs git hooks that are compatible with [pre-commit](https://pre-commit.com). You configure the hooks for each directory.

Use this table to find the correct page:

| Page | Read it when you want to… |
| --- | --- |
| [Getting started](getting-started.md) | install and configure tree-commit in a repository for the first time |
| [Configuration](configuration.md) | find a config file option |
| [How it works](how-it-works.md) | understand which hooks run on which files, and in which order |
| [Hooks and languages](hooks-and-languages.md) | write local hooks or use hooks from hook repositories |
| [Command line](cli.md) | find a command, flag or environment variable |
| [Recipes](recipes.md) | copy a ready-made configuration (uv and ruff, Go, CI, …) |
| [Migrating from pre-commit](migrating-from-pre-commit.md) | migrate an existing pre-commit configuration |
| [Compatibility report](compatibility.md) | see how tree-commit compares with pre-commit on real projects |
| [Troubleshooting](troubleshooting.md) | fix an error |
| [Development](development.md) | work on tree-commit itself |

## Main concepts

The main concepts of tree-commit are:

- A config file is a `.tree-commit-config.yaml` file. It has the same format as a `.pre-commit-config.yaml` file.
- You can put a config file in **any directory**. Its hooks check only the files **below that directory**.
- In a config file, paths are **relative to the directory of the config file**. Hooks **run from that directory**. For example, `go test ./...` in `go_lib/.tree-commit-config.yaml` tests `go_lib`.
- A parent directory and a child directory can both define a hook with the same `id`. In this case, the hook from the **closest config file** applies to the files in the child directory.
- Hooks that operate on different files **run in parallel**.
