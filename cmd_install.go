package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install tree-commit as a git hook",
	Long: `Install tree-commit as a git hook in the current repository.
This will create a hook that runs tree-commit automatically
before each commit.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return installPreCommitHook()
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
}

func installPreCommitHook() error {
	fmt.Println("Installing tree-commit as git hook...")

	gitDir := ".git"
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return fmt.Errorf("not in a git repository")
	}

	hooksDir := filepath.Join(gitDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("failed to create hooks directory: %v", err)
	}

	preCommitPath := filepath.Join(hooksDir, "tree-commit")
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %v", err)
	}

	hookContent := fmt.Sprintf("#!/bin/sh\n%s run\n", execPath)

	if err := os.WriteFile(preCommitPath, []byte(hookContent), 0755); err != nil {
		return fmt.Errorf("failed to write hook: %v", err)
	}

	fmt.Printf("Successfully installed tree-commit hook at %s\n", preCommitPath)
	return nil
}
