package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runCLI runs tree-commit with args in the current directory and returns its
// exit code and output.
func runCLI(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	savedOpts := runOpts
	t.Cleanup(func() {
		runOpts = savedOpts
		rootCmd.SetArgs(nil)
		// Cobra only gives a command the context of Execute when it has none yet.
		for _, cmd := range rootCmd.Commands() {
			cmd.SetContext(nil) //nolint:staticcheck // reset to the zero value
		}
		if f := rootCmd.Flags().Lookup("version"); f != nil {
			_ = f.Value.Set("false")
			f.Changed = false
		}
	})

	dir := t.TempDir()
	outFile, err := os.Create(filepath.Join(dir, "stdout"))
	if err != nil {
		t.Fatal(err)
	}
	errFile, err := os.Create(filepath.Join(dir, "stderr"))
	if err != nil {
		t.Fatal(err)
	}
	origOut, origErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outFile, errFile
	func() {
		defer func() { os.Stdout, os.Stderr = origOut, origErr }()
		rootCmd.SetArgs(args)
		code = execute()
	}()
	_ = outFile.Close()
	_ = errFile.Close()
	return code, readFile(t, outFile.Name()), readFile(t, errFile.Name())
}

func TestCLI(t *testing.T) {
	t.Setenv("TREE_COMMIT_HOME", filepath.Join(t.TempDir(), "cache"))
	repo := newRepo(t, map[string]string{
		"sub/.tree-commit-config.yaml": fmt.Sprintf(`repos:
  - repo: local
    hooks:
      - id: ok
        name: Passing hook
        entry: %q
      - id: broken
        name: Failing hook
        entry: %q
        files: \.fail$
`, helperEntry(t), helperEntry(t, "@fail")),
		"sub/a.txt":  "a\n",
		"sub/b.fail": "b\n",
	})
	runGit(t, repo, "add", ".")
	noRepo := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(noRepo))

	tests := []struct {
		name       string
		dir        string
		args       []string
		code       int
		stdout     []string
		stderr     string
		noStderrOf string
	}{
		{name: "version", dir: repo, args: []string{"--version"}, stdout: []string{"tree-commit version " + version}},
		{name: "unknown command", dir: repo, args: []string{"frobnicate"}, code: 1, stderr: `tree-commit: unknown command "frobnicate"`},
		{name: "too many arguments", dir: repo, args: []string{"run", "a", "b"}, code: 1, stderr: "tree-commit: accepts at most 1 arg(s)"},
		{name: "passing hooks", dir: repo, args: []string{"run", "--color", "never", "--files", filepath.Join("sub", "a.txt")}, stdout: []string{"Passing hook", "Passed"}},
		// A failing hook sets the exit code without an error message.
		{name: "failing hook", dir: repo, args: []string{"run", "--all-files", "--color", "never"}, code: 1,
			stdout: []string{"Failing hook", "Failed", "boom: something is wrong"}, noStderrOf: "tree-commit:"},
		{name: "single hook", dir: filepath.Join(repo, "sub"), args: []string{"run", "ok", "--files", "b.fail", "--color=never"}, stdout: []string{"Passing hook"}},
		{name: "run outside a repository", dir: noRepo, args: []string{"run"}, code: 1, stderr: "tree-commit: not in a git repository"},
		{name: "tree", dir: repo, args: []string{"tree"}, stdout: []string{"ConfigTree(Root: ", treeCommitConfigFile}},
		{name: "tree without configs", dir: repo, args: []string{"tree", noRepo}, stdout: []string{"No " + treeCommitConfigFile + " found in " + noRepo}},
		{name: "clean", dir: repo, args: []string{"clean"}, stdout: []string{"Cleaned " + os.Getenv("TREE_COMMIT_HOME")}},
		{name: "install", dir: repo, args: []string{"install"}, stdout: []string{"tree-commit installed at"}},
		{name: "uninstall", dir: repo, args: []string{"uninstall"}, stdout: []string{"tree-commit uninstalled."}},
		{name: "uninstall when not installed", dir: repo, args: []string{"uninstall"}, stdout: []string{"tree-commit is not installed, nothing to do."}},
		{name: "install outside a repository", dir: noRepo, args: []string{"install"}, code: 1, stderr: "tree-commit: not in a git repository"},
		{name: "uninstall outside a repository", dir: noRepo, args: []string{"uninstall"}, code: 1, stderr: "tree-commit: not in a git repository"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chdir(t, tt.dir)
			code, stdout, stderr := runCLI(t, tt.args...)
			if code != tt.code {
				t.Errorf("exit code = %d, want %d\nstdout:\n%s\nstderr:\n%s", code, tt.code, stdout, stderr)
			}
			for _, want := range tt.stdout {
				if !strings.Contains(stdout, want) {
					t.Errorf("stdout missing %q:\n%s", want, stdout)
				}
			}
			if !strings.Contains(stderr, tt.stderr) {
				t.Errorf("stderr missing %q:\n%s", tt.stderr, stderr)
			}
			if tt.noStderrOf != "" && strings.Contains(stderr, tt.noStderrOf) {
				t.Errorf("stderr should not contain %q:\n%s", tt.noStderrOf, stderr)
			}
		})
	}
}

func TestExitCodeError(t *testing.T) {
	err := fmt.Errorf("wrapped: %w", exitCodeError(3))
	var code exitCodeError
	if !errors.As(err, &code) || int(code) != 3 || code.Error() != "exit code 3" {
		t.Errorf("exitCodeError = %v (%d)", code, code)
	}
}

func TestInstallRefusesToOverwriteLegacyHook(t *testing.T) {
	dir := newRepo(t, nil)
	chdir(t, dir)
	hooks := filepath.Join(dir, ".git", "hooks")
	writeFiles(t, hooks, map[string]string{
		hookName:   "#!/bin/sh\necho custom\n",
		legacyName: "#!/bin/sh\necho older\n",
	})

	err := installPreCommitHook()
	if err == nil || !strings.Contains(err.Error(), "is taken") {
		t.Fatalf("expected an error, got %v", err)
	}
	if got := readFile(t, filepath.Join(hooks, hookName)); got != "#!/bin/sh\necho custom\n" {
		t.Errorf("existing hook was modified:\n%s", got)
	}
	if got := readFile(t, filepath.Join(hooks, legacyName)); got != "#!/bin/sh\necho older\n" {
		t.Errorf("legacy hook was modified:\n%s", got)
	}

	// uninstall leaves hooks that tree-commit did not install alone.
	if err := uninstallPreCommitHook(); err != nil {
		t.Fatal(err)
	}
	if !fileExists(filepath.Join(hooks, hookName)) {
		t.Error("uninstall removed a hook it did not install")
	}
}

func TestUseColor(t *testing.T) {
	regular, err := os.Create(filepath.Join(t.TempDir(), "out"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = regular.Close() })

	tests := []struct {
		name, mode, noColor, term string
		out                       io.Writer
		want                      bool
	}{
		{"always", "always", "1", "dumb", &bytes.Buffer{}, true},
		{"never", "never", "", "xterm", regular, false},
		{"NO_COLOR", "auto", "1", "xterm", regular, false},
		{"dumb terminal", "auto", "", "dumb", regular, false},
		{"not a file", "auto", "", "xterm", &bytes.Buffer{}, false},
		{"regular file", "auto", "", "xterm", regular, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("NO_COLOR", tt.noColor)
			t.Setenv("TERM", tt.term)
			if got := useColor(tt.mode, tt.out); got != tt.want {
				t.Errorf("useColor = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRunHooksErrors(t *testing.T) {
	noRepo := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(noRepo))
	tests := []struct {
		name   string
		files  map[string]string
		opts   runOptions
		inRepo bool
		want   string
	}{
		{name: "not a repository", want: "not in a git repository"},
		{name: "file outside the repository", inRepo: true, opts: runOptions{files: []string{filepath.Join("..", "outside.txt")}},
			want: "is outside the repository"},
		{name: "invalid config", inRepo: true, files: map[string]string{"lib/.tree-commit-config.yaml": "repos: [broken\n"},
			want: "failed to parse config file"},
		{name: "invalid top-level pattern", inRepo: true, files: map[string]string{".tree-commit-config.yaml": "files: '('\nrepos: []\n"},
			want: "invalid top-level 'files'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := noRepo
			if tt.inRepo {
				dir = newRepo(t, tt.files)
			}
			opts := tt.opts
			opts.dir = dir
			opts.allFiles = len(opts.files) == 0
			_, err := runHooks(context.Background(), &opts, &strings.Builder{})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("runHooks error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestRunHooksNothingToRun(t *testing.T) {
	dir := newRepo(t, map[string]string{
		".tree-commit-config.yaml": `repos:
  - repo: local
    hooks:
      - id: py
        entry: python-only
        files: \.py$
`,
		"a.txt": "a\n",
	})
	runGit(t, dir, "add", ".")

	// Like pre-commit, hooks in scope that match no file are listed as skipped.
	if code, out := runInRepo(t, dir, &runOptions{}); code != 0 || !strings.Contains(out, "py...") ||
		!strings.HasSuffix(strings.TrimSpace(out), "(no files to check)Skipped") {
		t.Errorf("expected the hook to be skipped, got %d\n%s", code, out)
	}
	if _, out := runInRepo(t, dir, &runOptions{dryRun: true}); !strings.Contains(out, "0 hook(s) would run") {
		t.Errorf("dry run should not list hooks without files:\n%s", out)
	}
	t.Setenv("SKIP", "py")
	if code, out := runInRepo(t, dir, &runOptions{}); code != 0 || !strings.Contains(out, "No hooks apply to the 2 selected file(s).") {
		t.Errorf("expected nothing to run, got %d\n%s", code, out)
	}
	t.Setenv("SKIP", "")
	if code, out := runInRepo(t, dir, &runOptions{hookID: "nope"}); code != 1 || !strings.Contains(out, `No hook with id "nope" applies to the selected files.`) {
		t.Errorf("expected unknown hook id to fail, got %d\n%s", code, out)
	}
}

func TestRunReportsBrokenHookDefinitions(t *testing.T) {
	dir := newRepo(t, map[string]string{
		".tree-commit-config.yaml": `repos:
  - repo: meta
    hooks:
      - id: check-hooks-apply
  - repo: local
    hooks:
      - id: no-entry
        name: Missing entry
      - id: bad-pattern
        name: Bad pattern
        entry: tool
        exclude: '[a-'
      - id: missing-tool
        name: Missing tool
        entry: tree-commit-no-such-tool --check
`,
		"a.txt": "a\n",
	})
	runGit(t, dir, "add", ".")

	code, out := runInRepo(t, dir, &runOptions{allFiles: true})
	if code != 1 {
		t.Fatalf("expected failure, got %d\n%s", code, out)
	}
	for _, want := range []string{
		"check-hooks-apply", "Skipped", "- meta hooks are not supported",
		"Missing entry", "- error: " + treeCommitConfigFile + `: hook "no-entry": local hook is missing required field 'entry'`,
		"Bad pattern", `hook "bad-pattern": invalid 'exclude' pattern`,
		"Missing tool", `- error: executable "tree-commit-no-such-tool" not found on PATH`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}

	// The dry run shows the same problems without running anything.
	_, out = runInRepo(t, dir, &runOptions{allFiles: true, dryRun: true})
	for _, want := range []string{"skipped: meta hooks are not supported", "error: " + treeCommitConfigFile + `: hook "no-entry"`} {
		if !strings.Contains(out, want) {
			t.Errorf("dry run output missing %q:\n%s", want, out)
		}
	}
}
