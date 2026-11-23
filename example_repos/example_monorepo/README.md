# Example monorepo

This directory is a small but realistic monorepo. Use it to try and test
tree-commit. Each directory has its own checks in a
`.tree-commit-config.yaml`. The Python tools run through
[uv](https://docs.astral.sh/uv/) and [ruff](https://docs.astral.sh/ruff/).

This table gives each path and what its config file shows:

| Path | What its config file shows |
| --- | --- |
| `.tree-commit-config.yaml` | Repo-wide hooks: pre-commit-hooks, ruff and codespell through `uvx` |
| `python_lib/` | A uv project whose tests run with `uv run --locked pytest` from `python_lib/` |
| `python_lib/src/calc/core/` | Overrides `ruff-check` with stricter rules (closest config wins) |
| `python_lib/src/calc/utils/` | Adds a complexity check, except for `helpers/` (`exclude_folders`) |
| `python_lib/src/calc/utils/helpers/` | Overrides `ruff-format` with a 79 column limit |
| `services/api/` | Pins ruff through the remote `astral-sh/ruff-pre-commit` hooks |
| `legacy/` | Vendored code: `disable: true` disables ruff and codespell |
| `.grep-baseline.json` | The baseline of the `todo-needs-ticket` grep hook. It accepts the old TODO in `legacy/`, but new TODOs need a ticket number |
| `infra/k8s/` | Overrides `check-yaml` to allow multi-document manifests |
| `scripts/` | shellcheck and shfmt, installed on demand with `uvx` |
| `docs/` | Markdown linting with pymarkdown through `uvx` |
| `go_lib/` | gofmt, `go vet ./...` and `go test ./...`, run inside `go_lib/` |
| `frontend/` | prettier, eslint and tsc through `npx` (needs Node.js) |

To try it, run these commands in a copy of this directory:

```sh
git init -q && git add -A
tree-commit tree
tree-commit run --all-files --dry-run
SKIP=prettier,eslint,tsc tree-commit run --all-files
```
