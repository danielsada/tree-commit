package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunScopeClosestWinsAndDisable(t *testing.T) {
	logPath := filepath.ToSlash(filepath.Join(t.TempDir(), "hooks.log"))
	hook := func(name string) string { return helperEntry(t, "@log="+logPath, "@name="+name) }

	dir := newRepo(t, map[string]string{
		".tree-commit-config.yaml": fmt.Sprintf(`repos:
  - repo: local
    hooks:
      - id: common
        name: Common
        entry: %q
      - id: lint
        name: Root lint
        entry: %q
        files: \.js$
      - id: strict
        name: Strict
        entry: %q
`, hook("root-common"), hook("root-lint"), hook("root-strict")),
		"frontend/.tree-commit-config.yaml": fmt.Sprintf(`repos:
  - repo: local
    hooks:
      - id: lint
        name: Frontend lint
        entry: %q
        files: ^src/.*\.js$
      - id: build
        name: Frontend build
        entry: %q
        pass_filenames: false
`, hook("frontend-lint"), hook("frontend-build")),
		"backend/.tree-commit-config.yaml": `repos:
  - repo: local
    hooks:
      - id: strict
        disable: true
`,
		"frontend/src/a.js": "a\n",
		"frontend/b.js":     "b\n",
		"backend/c.py":      "c\n",
		"docs/d.md":         "d\n",
	})
	runGit(t, dir, "add", "frontend/src/a.js", "frontend/b.js", "backend/c.py", "docs/d.md")

	code, out := runInRepo(t, dir, &runOptions{})
	if code != 0 {
		t.Fatalf("expected success, got %d\n%s", code, out)
	}

	root := filepath.Base(dir)
	assertLines(t, readLog(t, logPath), []string{
		// Root hooks run from the repository root on every file in scope.
		"root-common|" + root + "|backend/c.py,docs/d.md,frontend/b.js,frontend/src/a.js",
		// backend/ disables "strict" for its files only.
		"root-strict|" + root + "|docs/d.md,frontend/b.js,frontend/src/a.js",
		// frontend/ overrides "lint": patterns and paths are relative to frontend/,
		// and the root "lint" never runs on frontend files (closest wins).
		"frontend-lint|frontend|src/a.js",
		"frontend-build|frontend|",
	})
	for _, want := range []string{"Frontend lint (frontend/)", "Common", "Passed"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRunReportsFailuresAndModifiedFiles(t *testing.T) {
	dir := newRepo(t, map[string]string{
		".tree-commit-config.yaml": fmt.Sprintf(`repos:
  - repo: local
    hooks:
      - id: ok
        name: Passing hook
        entry: %q
      - id: broken
        name: Failing hook
        entry: %q
      - id: fixer
        name: Fixing hook
        entry: %q
        files: \.txt$
`, helperEntry(t), helperEntry(t, "@fail"), helperEntry(t, "@modify")),
		"a.txt": "hello\n",
	})
	runGit(t, dir, "add", ".")

	code, out := runInRepo(t, dir, &runOptions{allFiles: true})
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d\n%s", code, out)
	}
	for _, want := range []string{
		"Passing hook", "Passed",
		"Failing hook", "- hook id: broken", "- exit code: 1", "boom: something is wrong",
		"Fixing hook", "- files were modified by this hook",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRunRemoteRepository(t *testing.T) {
	t.Setenv("TREE_COMMIT_HOME", t.TempDir())
	logPath := filepath.ToSlash(filepath.Join(t.TempDir(), "hooks.log"))

	hooksRepo := newRepo(t, map[string]string{
		".pre-commit-hooks.yaml": fmt.Sprintf(`- id: remote-hook
  name: Remote hook
  entry: %q
  language: system
  files: \.txt$
- id: node-hook
  name: Node hook
  entry: eslint
  language: node
`, helperEntry(t, "@log="+logPath, "@name=manifest")),
	})
	runGit(t, hooksRepo, "add", ".")
	runGit(t, hooksRepo, "commit", "-q", "-m", "hooks")
	runGit(t, hooksRepo, "tag", "v1")

	dir := newRepo(t, map[string]string{
		"sub/.tree-commit-config.yaml": fmt.Sprintf(`repos:
  - repo: %s
    rev: v1
    hooks:
      - id: remote-hook
        args: ["@name=overridden"]
      - id: node-hook
`, filepath.ToSlash(hooksRepo)),
		"sub/a.txt": "a\n",
		"sub/b.md":  "b\n",
		"c.txt":     "c\n",
	})
	runGit(t, dir, "add", ".")

	code, out := runInRepo(t, dir, &runOptions{allFiles: true})
	if code != 0 {
		t.Fatalf("expected success, got %d\n%s", code, out)
	}
	// The user's args are merged on top of the manifest; files come from the manifest.
	assertLines(t, readLog(t, logPath), []string{"overridden|sub|a.txt"})
	if !strings.Contains(out, "Skipped") || !strings.Contains(out, `language "node" is not supported yet`) {
		t.Errorf("expected node hook to be skipped:\n%s", out)
	}

	// Second run uses the cache.
	if code, out := runInRepo(t, dir, &runOptions{allFiles: true}); code != 0 {
		t.Fatalf("cached run failed: %d\n%s", code, out)
	}
}

func TestRunRemoteRepositoryMissingHook(t *testing.T) {
	t.Setenv("TREE_COMMIT_HOME", t.TempDir())
	hooksRepo := newRepo(t, map[string]string{".pre-commit-hooks.yaml": "[]\n"})
	runGit(t, hooksRepo, "add", ".")
	runGit(t, hooksRepo, "commit", "-q", "-m", "hooks")

	dir := newRepo(t, map[string]string{
		".tree-commit-config.yaml": fmt.Sprintf(`repos:
  - repo: %s
    rev: HEAD
    hooks:
      - id: does-not-exist
`, filepath.ToSlash(hooksRepo)),
		"a.txt": "a\n",
	})
	runGit(t, dir, "add", ".")

	code, out := runInRepo(t, dir, &runOptions{allFiles: true})
	if code != 1 || !strings.Contains(out, "hook id not found") {
		t.Fatalf("expected failure for unknown hook id, got %d\n%s", code, out)
	}
}

func TestRunSkipHookIDAndDryRun(t *testing.T) {
	logPath := filepath.ToSlash(filepath.Join(t.TempDir(), "hooks.log"))
	dir := newRepo(t, map[string]string{
		".tree-commit-config.yaml": fmt.Sprintf(`repos:
  - repo: local
    hooks:
      - id: one
        entry: %q
      - id: two
        entry: %q
      - id: push-only
        entry: %q
        stages: [pre-push]
`, helperEntry(t, "@log="+logPath, "@name=one"), helperEntry(t, "@log="+logPath, "@name=two"),
			helperEntry(t, "@log="+logPath, "@name=push")),
		"a.txt": "a\n",
	})
	runGit(t, dir, "add", "a.txt")

	if _, out := runInRepo(t, dir, &runOptions{dryRun: true}); !strings.Contains(out, "2 hook(s) would run on 1 file(s)") {
		t.Errorf("unexpected dry-run output:\n%s", out)
	}
	if got := readLog(t, logPath); len(got) != 0 {
		t.Fatalf("dry run executed hooks: %v", got)
	}

	t.Setenv("SKIP", "one")
	if code, out := runInRepo(t, dir, &runOptions{}); code != 0 {
		t.Fatalf("run failed: %d\n%s", code, out)
	}
	assertLines(t, readLog(t, logPath), []string{"two|" + filepath.Base(dir) + "|a.txt"})

	t.Setenv("SKIP", "")
	if code, out := runInRepo(t, dir, &runOptions{hookID: "push-only", stage: "pre-push"}); code != 0 {
		t.Fatalf("run failed: %d\n%s", code, out)
	}
	assertLines(t, readLog(t, logPath), []string{
		"two|" + filepath.Base(dir) + "|a.txt",
		"push|" + filepath.Base(dir) + "|a.txt",
	})
}

func TestRunBuiltinLanguages(t *testing.T) {
	dir := newRepo(t, map[string]string{
		".tree-commit-config.yaml": `repos:
  - repo: local
    hooks:
      - id: no-todo
        name: No TODO
        entry: TODO
        language: pygrep
        args: [-i]
      - id: no-rej
        name: No .rej files
        entry: rejection files are not allowed
        language: fail
        files: \.rej$
`,
		"pkg/a.go":  "package a\n// todo: later\n",
		"pkg/b.rej": "x\n",
		"ok.txt":    "fine\n",
	})
	runGit(t, dir, "add", ".")

	code, out := runInRepo(t, dir, &runOptions{allFiles: true})
	if code != 1 {
		t.Fatalf("expected failure, got %d\n%s", code, out)
	}
	for _, want := range []string{"pkg/a.go:2:// todo: later", "rejection files are not allowed", "pkg/b.rej"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRunWithoutConfig(t *testing.T) {
	dir := newRepo(t, map[string]string{"a.txt": "a\n"})
	runGit(t, dir, "add", ".")
	code, out := runInRepo(t, dir, &runOptions{allFiles: true})
	if code != 0 || !strings.Contains(out, "nothing to do") {
		t.Fatalf("expected no-op, got %d\n%s", code, out)
	}
}

func TestRunPreCommitConfigFallback(t *testing.T) {
	logPath := filepath.ToSlash(filepath.Join(t.TempDir(), "hooks.log"))
	dir := newRepo(t, map[string]string{
		"lib/.pre-commit-config.yaml": fmt.Sprintf(`repos:
  - repo: local
    hooks:
      - id: from-pre-commit
        entry: %q
`, helperEntry(t, "@log="+logPath, "@name=pre-commit-config")),
		"lib/x.txt": "x\n",
	})

	if code, out := runInRepo(t, dir, &runOptions{files: []string{filepath.Join(dir, "lib", "x.txt")}}); code != 0 {
		t.Fatalf("run failed: %d\n%s", code, out)
	}
	assertLines(t, readLog(t, logPath), []string{"pre-commit-config|lib|x.txt"})
}
