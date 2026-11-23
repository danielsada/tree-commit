package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var treeCmd = &cobra.Command{
	Use:   "tree [dir]",
	Short: "Show the tree of config files that apply in this repository",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		dir := "."
		if len(args) == 1 {
			dir = args[0]
		} else if root, err := repoRoot("."); err == nil {
			dir = root
		}
		tree, err := DiscoverConfigs(dir)
		if err != nil {
			return err
		}
		if len(tree.FlattenConfigs()) == 0 {
			fmt.Printf("No %s found in %s\n", configFileNames[0], dir)
			return nil
		}
		fmt.Print(tree.String())
		return nil
	},
}

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove cached hook repositories and environments",
	Args:  cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		dir, err := cleanCache()
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "Cleaned %s\n", dir)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(treeCmd, cleanCmd)
}
