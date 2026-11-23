package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var baselineCmd = &cobra.Command{
	Use:   "baseline [hook-id...]",
	Short: "Record the current violations of grep hooks in their baseline files",
	Long: `Record the current violations of grep and pygrep hooks that have a baseline.

The hooks run on every file tracked by git, and their baseline files are
replaced with the violations they find. Use this to adopt a new rule in an
existing codebase: from then on, only new violations fail, and the baseline
shrinks as violations are fixed.

Without hook ids, every hook with a baseline is recorded.`,
	RunE: func(_ *cobra.Command, args []string) error {
		dir, err := os.Getwd()
		if err != nil {
			return err
		}
		return recordBaselines(dir, args, os.Stdout)
	},
}

func init() {
	rootCmd.AddCommand(baselineCmd)
}

// recordBaselines replaces the baselines of the selected grep hooks with their
// current violations on all tracked files.
func recordBaselines(dir string, hookIDs []string, out io.Writer) error {
	root, err := repoRoot(dir)
	if err != nil {
		return err
	}
	files, err := selectFiles(root, dir, &runOptions{allFiles: true})
	if err != nil {
		return err
	}
	tree, err := DiscoverConfigs(root)
	if err != nil {
		return err
	}
	scopes, err := scopesFromTree(tree)
	if err != nil {
		return err
	}
	cache, err := newRepoCache()
	if err != nil {
		return err
	}
	plan := buildPlan(root, scopes, files, cache, planOptions{allStages: true})

	wanted := make(map[string]bool, len(hookIDs))
	for _, id := range hookIDs {
		wanted[id] = true
	}
	x := &executor{root: root}
	recorded := 0
	var failures []string
	for _, r := range plan {
		d := r.def
		if d.hook.Baseline == "" || (len(wanted) > 0 && !wanted[d.id]) {
			continue
		}
		delete(wanted, d.id)
		if err := x.recordBaseline(r, out); err != nil {
			failures = append(failures, fmt.Sprintf("%s (%s): %v", d.id, d.scope.path, err))
			continue
		}
		recorded++
	}

	for id := range wanted {
		failures = append(failures, fmt.Sprintf("%s: no grep hook with a baseline has this id", id))
	}
	if len(failures) > 0 {
		return errors.New(strings.Join(failures, "\n"))
	}
	if recorded == 0 {
		return errors.New("no grep or pygrep hook has a baseline")
	}
	return nil
}

// recordBaseline replaces the baseline of one hook with its current violations.
func (x *executor) recordBaseline(r *hookRun, out io.Writer) error {
	d := r.def
	if d.err != nil {
		return d.err
	}
	opts, err := parseGrep(d.hook)
	if err != nil {
		return err
	}
	found := map[string][]violation{}
	if !r.noFiles {
		if found, err = x.scanGrep(r, opts); err != nil {
			return err
		}
	}

	path := x.baselinePath(d)
	doc, err := loadBaseline(path)
	if err != nil {
		return err
	}
	delete(doc.Hooks, d.id)
	hb, err := doc.hookEntry(d.id, baselineMode(d.hook))
	if err != nil {
		return err
	}
	hb.record(found)
	total := 0
	for _, fb := range hb.Files {
		total += fb.size()
	}
	if total == 0 {
		delete(doc.Hooks, d.id)
	}
	if err := saveBaseline(path, doc); err != nil {
		return err
	}

	rel, relErr := filepath.Rel(x.root, path)
	if relErr != nil {
		rel = path
	}
	fmt.Fprintf(out, "%s: recorded %d violation(s) in %d file(s) to %s\n",
		hookLabel(d), total, len(hb.Files), filepath.ToSlash(rel))
	return nil
}
