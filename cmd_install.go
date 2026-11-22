package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

const (
	hookName    = "pre-commit"
	legacyName  = "pre-commit.legacy"
	hookMarker  = "# installed by tree-commit"
	hookPerm    = 0o755
	hooksDirPrm = 0o750
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install tree-commit as the git pre-commit hook",
	Long: `Install tree-commit as the pre-commit hook of the current repository.

An existing pre-commit hook that was not installed by tree-commit is kept as
pre-commit.legacy and still runs before tree-commit.`,
	Args: cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		return installPreCommitHook()
	},
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove the tree-commit git hook",
	Args:  cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		return uninstallPreCommitHook()
	},
}

func init() {
	rootCmd.AddCommand(installCmd, uninstallCmd)
}

// hooksDir returns the hooks directory of the repository in the current
// directory, honoring core.hooksPath and worktrees.
func hooksDir() (string, error) {
	out, err := gitOutput(".", "rev-parse", "--git-path", "hooks")
	if err != nil {
		return "", errors.New("not in a git repository")
	}
	return filepath.FromSlash(strings.TrimSpace(out)), nil
}

func hookScript(execPath string) string {
	exe := filepath.ToSlash(execPath)
	return fmt.Sprintf(`#!/bin/sh
%s
HOOKS_DIR="$(dirname "$0")"
if [ -x "$HOOKS_DIR/%s" ]; then
    "$HOOKS_DIR/%s" "$@" || exit $?
fi
TREE_COMMIT='%s'
if [ ! -x "$TREE_COMMIT" ]; then
    TREE_COMMIT=tree-commit
fi
exec "$TREE_COMMIT" run --hook-stage pre-commit
`, hookMarker, legacyName, legacyName, strings.ReplaceAll(exe, "'", `'\''`))
}

func isTreeCommitHook(path string) bool {
	data, err := os.ReadFile(path) //nolint:gosec // git hook file
	return err == nil && strings.Contains(string(data), hookMarker)
}

func installPreCommitHook() error {
	dir, err := hooksDir()
	if err != nil {
		return err
	}
	if err = os.MkdirAll(dir, hooksDirPrm); err != nil {
		return fmt.Errorf("failed to create hooks directory: %w", err)
	}

	hookPath := filepath.Join(dir, hookName)
	legacyPath := filepath.Join(dir, legacyName)
	if fileExists(hookPath) && !isTreeCommitHook(hookPath) {
		if fileExists(legacyPath) {
			return fmt.Errorf("%s already exists and %s is taken; move one of them away first", hookPath, legacyPath)
		}
		if err = os.Rename(hookPath, legacyPath); err != nil {
			return fmt.Errorf("failed to back up existing hook: %w", err)
		}
		fmt.Printf("Existing pre-commit hook moved to %s (it still runs before tree-commit)\n", legacyPath)
	}

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	if err := os.WriteFile(hookPath, []byte(hookScript(execPath)), hookPerm); err != nil { //nolint:gosec // hooks must be executable
		return fmt.Errorf("failed to write hook: %w", err)
	}
	// WriteFile does not change the mode of an existing file.
	if err := os.Chmod(hookPath, hookPerm); err != nil { //nolint:gosec // hooks must be executable
		return fmt.Errorf("failed to make hook executable: %w", err)
	}

	fmt.Printf("tree-commit installed at %s\n", hookPath)
	return nil
}

func uninstallPreCommitHook() error {
	dir, err := hooksDir()
	if err != nil {
		return err
	}
	hookPath := filepath.Join(dir, hookName)
	if !isTreeCommitHook(hookPath) {
		fmt.Println("tree-commit is not installed, nothing to do.")
		return nil
	}
	if err := os.Remove(hookPath); err != nil {
		return fmt.Errorf("failed to remove hook: %w", err)
	}

	legacyPath := filepath.Join(dir, legacyName)
	if fileExists(legacyPath) {
		if err := os.Rename(legacyPath, hookPath); err != nil {
			return fmt.Errorf("failed to restore previous hook: %w", err)
		}
		fmt.Printf("Restored previous hook %s\n", hookPath)
	}
	fmt.Println("tree-commit uninstalled.")
	return nil
}
