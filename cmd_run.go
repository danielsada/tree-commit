package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

type runOptions struct {
	dir      string
	allFiles bool
	files    []string
	hookID   string
	stage    string
	dryRun   bool
	verbose  bool
	jobs     int
	color    string
}

var runOpts = runOptions{stage: "pre-commit", color: "auto", jobs: runtime.NumCPU()}

var runCmd = &cobra.Command{
	Use:   "run [hook-id]",
	Short: "Run hooks on staged files (or on --all-files / --files)",
	Long: `Run the hooks that apply to the changed files.

Each .tree-commit-config.yaml only applies to files below its directory, its
patterns are matched relative to that directory, and its hooks run with that
directory as the working directory. When several configs define the same hook
id, the config closest to the file wins.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := runOpts
		if len(args) == 1 {
			opts.hookID = args[0]
		}
		code, err := runHooks(cmd.Context(), &opts, os.Stdout)
		if err != nil {
			return err
		}
		if code != 0 {
			return exitCodeError(code)
		}
		return nil
	},
}

func init() {
	f := runCmd.Flags()
	f.BoolVarP(&runOpts.allFiles, "all-files", "a", false, "run on all files tracked by git")
	f.StringSliceVar(&runOpts.files, "files", nil, "run on these files instead of the staged files")
	f.StringVar(&runOpts.stage, "hook-stage", runOpts.stage, "only run hooks configured for this stage")
	f.BoolVar(&runOpts.dryRun, "dry-run", false, "print which hooks would run on which files, without running them (remote hook repositories are still fetched)")
	f.BoolVarP(&runOpts.verbose, "verbose", "v", false, "show output of passing hooks and durations")
	f.IntVarP(&runOpts.jobs, "jobs", "j", runOpts.jobs, "maximum number of hooks to run in parallel")
	f.StringVar(&runOpts.color, "color", runOpts.color, "colorize output: auto, always or never")
	rootCmd.AddCommand(runCmd)
}

// runHooks runs the hooks and returns the process exit code (0 when all hooks pass).
func runHooks(ctx context.Context, opts *runOptions, out io.Writer) (int, error) {
	dir := opts.dir
	if dir == "" {
		var err error
		if dir, err = os.Getwd(); err != nil {
			return 0, err
		}
	}
	root, err := repoRoot(dir)
	if err != nil {
		return 0, err
	}

	files, err := selectFiles(root, dir, opts)
	if err != nil {
		return 0, err
	}

	tree, err := DiscoverConfigs(root)
	if err != nil {
		return 0, err
	}
	scopes, err := scopesFromTree(tree)
	if err != nil {
		return 0, err
	}
	if len(scopes) == 0 {
		fmt.Fprintf(out, "No %s found in %s, nothing to do.\n", configFileNames[0], root)
		return 0, nil
	}

	cache, err := newRepoCache()
	if err != nil {
		return 0, err
	}
	plan := buildPlan(root, scopes, files, cache, planOptions{
		stage:  opts.stage,
		hookID: opts.hookID,
		skip:   skipFromEnv(),
	})

	if opts.dryRun {
		printPlan(out, plan, len(files))
		return 0, nil
	}
	if len(plan) == 0 {
		if opts.hookID != "" {
			fmt.Fprintf(out, "No hook with id %q applies to the selected files.\n", opts.hookID)
			return 1, nil
		}
		fmt.Fprintf(out, "No hooks apply to the %d selected file(s).\n", len(files))
		return 0, nil
	}

	x := &executor{
		ctx:     ctx,
		root:    root,
		envs:    cache,
		jobs:    opts.jobs,
		out:     out,
		color:   useColor(opts.color, out),
		verbose: opts.verbose,
	}
	if x.runAll(plan) > 0 {
		return 1, nil
	}
	return 0, nil
}

// selectFiles returns the files to check, relative to root.
func selectFiles(root, cwd string, opts *runOptions) ([]string, error) {
	var (
		files []string
		err   error
	)
	switch {
	case opts.allFiles:
		files, err = trackedFiles(root)
	case len(opts.files) > 0:
		files, err = filesFromArgs(root, cwd, opts.files)
	default:
		files, err = stagedFiles(root)
	}
	if err != nil {
		return nil, err
	}

	existing := files[:0]
	for _, f := range files {
		info, err := os.Lstat(filepath.Join(root, f))
		if err == nil && !info.IsDir() {
			existing = append(existing, f)
		}
	}
	return existing, nil
}

func filesFromArgs(root, cwd string, args []string) ([]string, error) {
	// Resolve symlinks so paths compare correctly with git's toplevel (e.g. /tmp on macOS).
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		realRoot = root
	}
	files := make([]string, 0, len(args))
	for _, f := range args {
		abs := f
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(cwd, f)
		}
		if realDir, err := filepath.EvalSymlinks(filepath.Dir(abs)); err == nil {
			abs = filepath.Join(realDir, filepath.Base(abs))
		}
		if !isWithin(realRoot, abs) {
			return nil, fmt.Errorf("%s is outside the repository", f)
		}
		rel, err := filepath.Rel(realRoot, abs)
		if err != nil {
			return nil, err
		}
		files = append(files, rel)
	}
	return files, nil
}

// skipFromEnv reads the comma separated SKIP variable, like pre-commit.
func skipFromEnv() map[string]bool {
	skip := make(map[string]bool)
	for _, id := range strings.Split(os.Getenv("SKIP"), ",") {
		if id = strings.TrimSpace(id); id != "" {
			skip[id] = true
		}
	}
	return skip
}

func useColor(mode string, out io.Writer) bool {
	switch mode {
	case "always":
		return true
	case "never":
		return false
	}
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	f, ok := out.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// printPlan lists the hooks that would run. Hooks without files are left out.
func printPlan(out io.Writer, all []*hookRun, fileCount int) {
	plan := make([]*hookRun, 0, len(all))
	for _, r := range all {
		if !r.noFiles {
			plan = append(plan, r)
		}
	}
	fmt.Fprintf(out, "%d hook(s) would run on %d file(s):\n", len(plan), fileCount)
	deps := hookDependencies(plan)
	for i, r := range plan {
		d := r.def
		cwd := scopeLabel(d.scope.dir)
		if d.scope.dir == rootDir {
			cwd = "(repository root)"
		}
		fmt.Fprintf(out, "\n%d. %s [id: %s, config: %s]\n", i+1, d.hook.Name, d.id, d.scope.path)
		fmt.Fprintf(out, "   cwd: %s\n", cwd)
		if len(deps[i]) > 0 {
			after := make([]string, len(deps[i]))
			for k, dep := range deps[i] {
				after[k] = fmt.Sprint(dep + 1)
			}
			fmt.Fprintf(out, "   runs after: %s\n", strings.Join(after, ", "))
		}
		switch {
		case d.err != nil:
			fmt.Fprintf(out, "   error: %v\n", d.err)
		case d.skipReason != "":
			fmt.Fprintf(out, "   skipped: %s\n", d.skipReason)
		}
		for _, f := range r.files {
			fmt.Fprintf(out, "   - %s\n", relToScope(d.scope.dir, f))
		}
	}
}
