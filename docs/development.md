# Development

## Build and test

```sh
go build -o tree-commit .
go test ./...
golangci-lint run          # v2, configured in .golangci.yml
```

The tests create temporary git repositories and use the test binary itself as a fake hook, so they work offline on Linux, macOS and Windows.

The [example monorepo](../example_repos/example_monorepo) has an end-to-end test that runs all of its real hooks. It needs network, uv, Python and Go:

```sh
TREE_COMMIT_E2E=1 go test -run ExampleMonorepoEndToEnd .
```

CI runs lint, all tests (with `-race` on Linux and macOS) and the end-to-end test on all three platforms.

## Code layout

Everything is in one `main` package:

| File | Responsibility |
| --- | --- |
| `main.go`, `cmd_*.go` | Command line (cobra) |
| `config.go` | Config and hook schema, validation |
| `config_discovery.go` | Finding config files |
| `resolver.go` | Deciding which hooks run on which files (scope, closest-wins, filters) |
| `executor.go` | Running hooks: scheduling, batching, output |
| `remote.go` | Cloning hook repositories, Python environments, cache |
| `filetypes.go` | File type detection for `types` |
| `paths.go` | Path helpers |
| `git.go` | git commands |
| `exec_windows.go` | Quoting for `.cmd`/`.bat` hooks on Windows |

Paths inside tree-commit are native paths handled with `path/filepath`. They are converted only at the edges: git output, regular expressions (matched with forward slashes) and the git hook script.

## Design

[How it works](how-it-works.md) describes the rules the resolver and executor implement, including ideas for future improvements.
