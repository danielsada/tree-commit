package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run linting rules on modified files",
	Long:  `Run linting rules on modified files in the git tree.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Running tree-commit linting...")
		// TODO: Implement git tree walking and linting logic
		return nil
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}