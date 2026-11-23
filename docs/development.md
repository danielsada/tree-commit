# Development

## Build and test

Use these commands to build, test, and lint tree-commit:

```sh
go build -o tree-commit .
go test ./...
golangci-lint run          # v2, configured in .golangci.yml
```

The tests create temporary git repositories. They use the test binary as a fake hook. Thus, the tests work offline on Linux, macOS, and Windows.

The [example monorepo](../example_repos/example_monorepo) has an end-to-end test that runs all of its real hooks. This test needs network access, uv, Python, and Go. To run it, use this command:

```sh
TREE_COMMIT_E2E=1 go test -run ExampleMonorepoEndToEnd .
```

CI runs the linter, all tests, and the end-to-end test on all three platforms. On Linux and macOS, CI runs the tests with `-race`.

## Code layout

All of the code is in one `main` package. This table shows the responsibility of each file:

| File | Responsibility |
| --- | --- |
| `main.go`, `cmd_*.go` | Command line (cobra) |
| `config.go` | Config file and hook schema, validation |
| `config_discovery.go` | Finds config files |
| `resolver.go` | Decides which hooks run on which files (scope, closest config file, filters) |
| `executor.go` | Runs hooks: scheduling, batching, output |
| `remote.go` | Hook repository clones, Python environments, cache |
| `filetypes.go` | File type detection for `types` |
| `paths.go` | Path helpers |
| `git.go` | git commands |
| `exec_windows.go` | Quoting for `.cmd`/`.bat` hooks on Windows |

In tree-commit, paths are native paths, and the code uses `path/filepath` for them. tree-commit converts paths only at these edges: git output, regular expressions (which match with forward slashes), and the git hook script.

## Design

[How it works](how-it-works.md) describes the rules that the resolver and the executor implement. It also gives ideas for future improvements.
