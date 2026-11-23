# Example monorepo

A small but realistic monorepo used to try out and test tree-commit. Every
directory owns its checks in a `.tree-commit-config.yaml`, and Python
tooling runs through [uv](https://docs.astral.sh/uv/) and
[ruff](https://docs.astral.sh/ruff/).

| Path | What its config shows |
| --- | --- |
| `.tree-commit-config.yaml` | Repo-wide hooks: pre-commit-hooks, ruff and codespell through `uvx` |
| `python_lib/` | A uv project whose tests run with `uv run --locked pytest` from `python_lib/` |
| `python_lib/src/calc/core/` | Overrides `ruff-check` with stricter rules (closest config wins) |
| `python_lib/src/calc/utils/` | Adds a complexity check, except for `helpers/` (`exclude_folders`) |
| `python_lib/src/calc/utils/helpers/` | Overrides `ruff-format` with a 79 column limit |
| `services/api/` | Pins ruff through the remote `astral-sh/ruff-pre-commit` hooks |
| `legacy/` | Vendored code: ruff and codespell are disabled with `disable: true` |
| `infra/k8s/` | Overrides `check-yaml` to allow multi-document manifests |
| `scripts/` | shellcheck and shfmt, installed on demand with `uvx` |
| `docs/` | Markdown linting with pymarkdown through `uvx` |
| `go_lib/` | gofmt, `go vet ./...` and `go test ./...`, run inside `go_lib/` |
| `frontend/` | prettier, eslint and tsc through `npx` (needs Node.js) |

Try it from a copy of this directory:

```sh
git init -q && git add -A
tree-commit tree
tree-commit run --all-files --dry-run
SKIP=prettier,eslint,tsc tree-commit run --all-files
```
