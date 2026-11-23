package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Regression tests for issues found in code review.

func TestRunRejectsOptionLikeRev(t *testing.T) {
	dir := newRepo(t, map[string]string{
		".tree-commit-config.yaml": `repos:
  - repo: .
    rev: "--upload-pack=touch pwned"
    hooks:
      - id: x
`,
		"a.txt": "a\n",
	})
	runGit(t, dir, "add", ".")

	opts := &runOptions{dir: dir, allFiles: true, dryRun: true, stage: "pre-commit", color: "never"}
	_, err := runHooks(context.Background(), opts, &strings.Builder{})
	if err == nil || !strings.Contains(err.Error(), "must not start with '-'") {
		t.Fatalf("expected config validation error, got %v", err)
	}
	if fileExists(filepath.Join(dir, "pwned")) {
		t.Fatal("rev was passed to git as an option")
	}
}

func TestRunHonorsDefaultStages(t *testing.T) {
	logPath := filepath.ToSlash(filepath.Join(t.TempDir(), "hooks.log"))
	dir := newRepo(t, map[string]string{
		".tree-commit-config.yaml": fmt.Sprintf(`default_stages: [push]
repos:
  - repo: local
    hooks:
      - id: slow-tests
        entry: %q
        files: \.txt$
      - id: lint
        entry: %q
        files: \.txt$
        stages: [commit]
`, helperEntry(t, "@log="+logPath, "@name=slow"), helperEntry(t, "@log="+logPath, "@name=lint")),
		"a.txt": "a\n",
	})
	runGit(t, dir, "add", ".")

	if code, out := runInRepo(t, dir, &runOptions{}); code != 0 {
		t.Fatalf("pre-commit run failed: %d\n%s", code, out)
	}
	assertLines(t, readLog(t, logPath), []string{"lint|" + filepath.Base(dir) + "|a.txt"})

	if code, out := runInRepo(t, dir, &runOptions{stage: "pre-push"}); code != 0 {
		t.Fatalf("pre-push run failed: %d\n%s", code, out)
	}
	assertLines(t, readLog(t, logPath), []string{
		"lint|" + filepath.Base(dir) + "|a.txt",
		"slow|" + filepath.Base(dir) + "|a.txt",
	})
}

func TestRunDoesNotPassSymlinksByDefault(t *testing.T) {
	logPath := filepath.ToSlash(filepath.Join(t.TempDir(), "hooks.log"))
	dir := newRepo(t, map[string]string{
		".tree-commit-config.yaml": fmt.Sprintf(`repos:
  - repo: local
    hooks:
      - id: fixer
        entry: %q
      - id: links
        entry: %q
        types: [symlink]
`, helperEntry(t, "@log="+logPath, "@name=fixer"), helperEntry(t, "@log="+logPath, "@name=links")),
		"real/a.txt": "a\n",
	})
	if err := os.Symlink("real", filepath.Join(dir, "link")); err != nil {
		t.Skipf("cannot create symlinks: %v", err)
	}
	runGit(t, dir, "add", ".")

	if code, out := runInRepo(t, dir, &runOptions{allFiles: true}); code != 0 {
		t.Fatalf("run failed: %d\n%s", code, out)
	}
	root := filepath.Base(dir)
	assertLines(t, readLog(t, logPath), []string{
		"fixer|" + root + "|.tree-commit-config.yaml,real/a.txt",
		"links|" + root + "|link",
	})
}

func TestPygrepIgnoresFinalNewline(t *testing.T) {
	dir := newRepo(t, map[string]string{
		".tree-commit-config.yaml": `repos:
  - repo: local
    hooks:
      - id: no-blank-lines
        entry: '^\s*$'
        language: pygrep
        files: \.txt$
`,
		"a.txt":     "line1\nline2\n",
		"empty.txt": "",
	})
	runGit(t, dir, "add", ".")

	if code, out := runInRepo(t, dir, &runOptions{allFiles: true}); code != 0 {
		t.Fatalf("expected no match, got %d\n%s", code, out)
	}
}

func TestDiscoveryIgnoresDeletedAndIgnoredConfigs(t *testing.T) {
	dir := newRepo(t, map[string]string{
		".gitignore":                               "node_modules/\n",
		"sub/.tree-commit-config.yaml":             "repos: []\n",
		"node_modules/pkg/.pre-commit-config.yaml": "repos: [broken\n",
		"a.txt": "a\n",
	})
	runGit(t, dir, "add", ".")
	if err := os.Remove(filepath.Join(dir, "sub", ".tree-commit-config.yaml")); err != nil {
		t.Fatal(err)
	}

	if code, out := runInRepo(t, dir, &runOptions{allFiles: true}); code != 0 {
		t.Fatalf("run failed: %d\n%s", code, out)
	}
}

func TestResolveExecutableRelativePathEntry(t *testing.T) {
	dir := t.TempDir()
	name := "tool"
	if runtime.GOOS == windows {
		name = "tool.exe"
	}
	tool := filepath.Join(dir, "bin", name)
	if err := os.MkdirAll(filepath.Dir(tool), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tool, nil, 0o700); err != nil { //nolint:gosec // test executable
		t.Fatal(err)
	}

	got, err := resolveExecutable("tool", dir, "bin")
	if err != nil || got != tool {
		t.Fatalf("resolveExecutable = %q, %v; want %q", got, err, tool)
	}
}
