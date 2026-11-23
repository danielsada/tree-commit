package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"

	"github.com/spf13/cobra"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "tree-commit",
	Short: "A tree-based git hook runner compatible with pre-commit hooks",
	Long: `tree-commit runs pre-commit compatible hooks, configured per directory.

Drop a .tree-commit-config.yaml (same format as .pre-commit-config.yaml) in any
directory of a repository. Its hooks only run on changed files below that
directory, so every team in a monorepo can own its own checks.`,
	Version:       version,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// exitCodeError makes the process exit with a specific code without printing an error.
type exitCodeError int

func (e exitCodeError) Error() string { return "exit code " + strconv.Itoa(int(e)) }

func main() {
	os.Exit(execute())
}

func execute() int {
	// Ctrl-C cancels the context, which stops running hooks.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		var code exitCodeError
		if errors.As(err, &code) {
			return int(code)
		}
		fmt.Fprintln(os.Stderr, "tree-commit:", err)
		return 1
	}
	return 0
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configFileNames[0], "config", "c", configFileNames[0],
		"config file name to look for in each directory")
}
