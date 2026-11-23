# tree-commit

A tree-based git hook runner that is compatible with [pre-commit](https://pre-commit.com) hooks.

Put a `.tree-commit-config.yaml` (same format as `.pre-commit-config.yaml`) in **any directory** of your repository. Its hooks only run on changed files below that directory, from that directory, and a config closer to the code overrides hooks with the same id from its parents. Every team in a monorepo owns its own checks, and nobody runs `eslint` because they touched a Go file.

```
repo/
├── .tree-commit-config.yaml        # trailing-whitespace, end-of-file-fixer for everything
├── frontend/
│   ├── .tree-commit-config.yaml    # eslint, prettier, tsc (run inside frontend/)
│   └── src/App.tsx
├── go_lib/
│   ├── .tree-commit-config.yaml    # gofmt, go vet, go test ./... (run inside go_lib/)
│   └── pkg/math.go
└── python_lib/
    ├── .tree-commit-config.yaml    # black, flake8
    └── src/
        └── .tree-commit-config.yaml  # overrides "black" with --line-length 79
```

```console
$ git commit -m "Fix rounding"
Go Format (go_lib/)......................................................Passed
Go Vet (go_lib/).........................................................Passed
Go Test (go_lib/)........................................................Failed
- hook id: go-test
- exit code: 1

--- FAIL: TestRound (0.00s)
    math_test.go:12: Round(2.5) = 2, want 3

trim trailing whitespace.................................................Passed
```

## Why

pre-commit is great, but it has one config for the whole repository. In a monorepo that means:

- one huge config with `files:` regexes like `^services/payments/.*\.py$` everywhere,
- hooks like `go test ./...` or `tsc` that need to run from a sub-project directory,
- every team editing the same file, and every commit evaluating every hook.

tree-commit keeps the pre-commit format and the pre-commit hook ecosystem, and makes the config **hierarchical**:

| | pre-commit | tree-commit |
|---|---|---|
| Config files | one, at the root | one per directory, any depth |
| Patterns are matched against | repo-relative paths | paths relative to the config's directory |
| Hooks run from | repository root | the config's directory |
| Same hook id in two places | runs twice | the config closest to the file wins |
| Parallelism | batches of one hook | different hooks run in parallel when they don't share files |

## Install

```sh
go install github.com/danielsada/tree-commit@latest
```

Then, in your repository:

```sh
tree-commit install      # installs .git/hooks/pre-commit
```

An existing hook is kept as `pre-commit.tree-commit-legacy` and still runs first (a hook installed by the pre-commit framework is kept but not run, since tree-commit reads `.pre-commit-config.yaml` itself). `tree-commit uninstall` restores it. `core.hooksPath` and git worktrees are supported.

## Configuration

A config file is a pre-commit config. A root `.pre-commit-config.yaml` works as is; tree-commit looks for `.tree-commit-config.yaml` first and falls back to `.pre-commit-config.yaml` in every directory.

```yaml
# go_lib/.tree-commit-config.yaml
repos:
  - repo: local
    hooks:
      - id: gofmt
        name: Go Format
        entry: gofmt -l -w
        language: system
        files: \.go$             # matched against paths relative to go_lib/
        exclude_folders: [vendor/]

      - id: go-test
        name: Go Test
        entry: go test ./...     # runs inside go_lib/
        language: system
        files: \.go$
        pass_filenames: false

  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.4.0
    hooks:
      - id: trailing-whitespace
      - id: check-yaml
```

Supported hook fields: `id`, `name`, `entry`, `language`, `files`, `exclude`, `types` (default `[file]`, so symlinks are skipped), `types_or`, `exclude_types`, `args`, `pass_filenames`, `always_run`, `stages`, `additional_dependencies`, `verbose`, and the top-level `files`, `exclude` and `default_stages`. Python's verbose regex flag `(?x)` is supported.

tree-commit adds:

| Field | Meaning |
|---|---|
| `include_folders` | Only run on files inside these folders (relative to the config). `.` means everything. |
| `exclude_folders` | Never run on files inside these folders. `build/` also excludes nested `a/build/`. |
| `disable: true` | Turn off a hook inherited from a parent config, for this directory. |

See [CONFLICT_RESOLUTION.md](CONFLICT_RESOLUTION.md) for the full resolution rules (scope, closest-wins, ordering and parallelism).

### Languages

| Language | Local hooks (`repo: local`) | Remote hooks (`repo: https://...`) |
|---|---|---|
| `system` | ✅ runs `entry` from `PATH` | ✅ |
| `script` | ✅ relative to the config directory | ✅ relative to the hook repository |
| `python` | ✅ in a virtualenv when `additional_dependencies` is set, otherwise from `PATH` | ✅ repository installed in a cached virtualenv |
| `pygrep`, `fail` | ✅ built in | ✅ built in |
| `node`, `golang`, `rust`, `docker`, … | ✅ treated as `system` (the tool must be on `PATH`) | ⏭️ reported as skipped |

Remote repositories are cloned at `rev` into the cache (`$TREE_COMMIT_HOME`, or your user cache directory under `tree-commit/`). `tree-commit clean` removes it.

## Usage

```sh
tree-commit run                    # run on staged files (what the git hook does)
tree-commit run --all-files        # run on every tracked file
tree-commit run --files a.go,b.py  # run on specific files
tree-commit run go-test            # run a single hook id
tree-commit run --dry-run          # show which hooks would run on which files, and in which order (still fetches remote repos)
tree-commit tree                   # show the config tree of the repository
SKIP=go-test,eslint git commit     # skip hooks by id, like pre-commit
```

`run` exits with status 1 when any hook fails or modifies files (for `pass_filenames: false` hooks, any change to tracked files in their directory counts). Other flags: `--jobs`, `--verbose`, `--color`, `--hook-stage`, and `-c/--config` to use a different config file name.

## Limitations

- Unstaged changes are not stashed before running hooks, so hooks see the working tree version of partially staged files.
- Only `system`, `script`, `python`, `pygrep` and `fail` hooks are installed from remote repositories.
- `meta` hooks and `minimum_pre_commit_version` are ignored.

## Development

```sh
go test ./...
golangci-lint run
```

`example_repos/example_monorepo` is a realistic sample monorepo (uv + ruff Python projects, Go, shell, Kubernetes YAML, docs and a frontend) whose configs demonstrate every feature; see its [README](example_repos/example_monorepo/README.md). Try `go run . tree example_repos/example_monorepo`.

`TREE_COMMIT_E2E=1 go test -run ExampleMonorepoEndToEnd .` runs all of its hooks for real (needs network, uv, python and go; CI runs it on every platform).
