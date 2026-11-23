package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stashRepo creates a repository with a committed file and a grep hook that
// fails on "BAD", so hook results show which content the hook saw.
func stashRepo(t *testing.T, extraHooks string) string {
	t.Helper()
	t.Setenv("TREE_COMMIT_HOME", t.TempDir())
	dir := newRepo(t, map[string]string{
		".tree-commit-config.yaml": `repos:
  - repo: local
    hooks:
      - id: no-bad
        name: no BAD
        entry: BAD
        language: grep
        files: \.txt$
` + extraHooks,
		"a.txt": "one\n",
	})
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "init")
	return dir
}

func fileContent(t *testing.T, dir, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func gitStatus(t *testing.T, dir string) string {
	t.Helper()
	out, err := gitOutput(dir, "status", "--porcelain")
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestStashHidesUnstagedChanges(t *testing.T) {
	dir := stashRepo(t, "")
	writeFiles(t, dir, map[string]string{"a.txt": "one\ntwo\n"})
	runGit(t, dir, "add", "a.txt")
	// Unstaged: a violation that is not part of the commit, a binary file change
	// and the deletion of a staged file.
	writeFiles(t, dir, map[string]string{"a.txt": "one\ntwo\nBAD unstaged\n", "b.txt": "b\n"})
	runGit(t, dir, "add", "b.txt")
	if err := os.Remove(filepath.Join(dir, "b.txt")); err != nil {
		t.Fatal(err)
	}
	statusBefore := gitStatus(t, dir)

	code, out := runInRepo(t, dir, &runOptions{})
	if code != 0 {
		t.Fatalf("the hook must only see the staged content, got %d\n%s", code, out)
	}
	for _, want := range []string{"[WARNING] Unstaged files detected.", "[INFO] Stashing unstaged files to", "[INFO] Restored changes from"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	if got := fileContent(t, dir, "a.txt"); got != "one\ntwo\nBAD unstaged\n" {
		t.Errorf("unstaged changes were not restored: %q", got)
	}
	if fileExists(filepath.Join(dir, "b.txt")) {
		t.Error("the unstaged deletion of b.txt was not restored")
	}
	if entries, _ := os.ReadDir(filepath.Join(os.Getenv("TREE_COMMIT_HOME"), "patches")); len(entries) != 0 {
		t.Errorf("restored patches should be deleted, found %d", len(entries))
	}
	if got := gitStatus(t, dir); got != statusBefore {
		t.Errorf("git status changed:\nbefore:\n%s\nafter:\n%s", statusBefore, got)
	}

	// With the violation staged and an unstaged fix, the hook fails.
	runGit(t, dir, "add", "a.txt")
	writeFiles(t, dir, map[string]string{"a.txt": "one\ntwo\n"})
	if code, out := runInRepo(t, dir, &runOptions{}); code != 1 || !strings.Contains(out, "a.txt:3:BAD unstaged") {
		t.Fatalf("expected the staged violation to fail, got %d\n%s", code, out)
	}
	if got := fileContent(t, dir, "a.txt"); got != "one\ntwo\n" {
		t.Errorf("the unstaged fix was not restored: %q", got)
	}
}

func TestStashRollsBackConflictingFixes(t *testing.T) {
	dir := stashRepo(t, fmt.Sprintf(`      - id: fixer
        name: fixer
        entry: %q
        files: \.txt$
`, helperEntry(t, "@modify")))
	writeFiles(t, dir, map[string]string{"a.txt": "one\nstaged\n"})
	runGit(t, dir, "add", "a.txt")
	// The fixer appends a line, which conflicts with this unstaged change at the end of the file.
	writeFiles(t, dir, map[string]string{"a.txt": "one\nstaged\nunstaged\n"})

	code, out := runInRepo(t, dir, &runOptions{})
	if code != 1 || !strings.Contains(out, "- files were modified by this hook") {
		t.Fatalf("expected the fixer to modify files, got %d\n%s", code, out)
	}
	if !strings.Contains(out, "[WARNING] Stashed changes conflicted with hook auto-fixes... Rolling back fixes...") {
		t.Errorf("expected the conflict warning:\n%s", out)
	}
	if got := fileContent(t, dir, "a.txt"); got != "one\nstaged\nunstaged\n" {
		t.Errorf("unstaged work must win over hook fixes, got %q", got)
	}
}

func TestStashKeepsNonConflictingFixes(t *testing.T) {
	dir := stashRepo(t, fmt.Sprintf(`      - id: fixer
        name: fixer
        entry: %q
        files: ^fix\.txt$
`, helperEntry(t, "@modify")))
	writeFiles(t, dir, map[string]string{"fix.txt": "fix me\n"})
	runGit(t, dir, "add", "fix.txt")
	writeFiles(t, dir, map[string]string{"a.txt": "one\nunstaged\n"})

	if code, out := runInRepo(t, dir, &runOptions{}); code != 1 || strings.Contains(out, "Rolling back") {
		t.Fatalf("expected a fix without conflict, got %d\n%s", code, out)
	}
	if got := fileContent(t, dir, "fix.txt"); got != "fix me\nmodified\n" {
		t.Errorf("the hook fix was lost: %q", got)
	}
	if got := fileContent(t, dir, "a.txt"); got != "one\nunstaged\n" {
		t.Errorf("unstaged changes were not restored: %q", got)
	}
}

func TestStashIntentToAddFiles(t *testing.T) {
	dir := stashRepo(t, "")
	writeFiles(t, dir, map[string]string{"new.txt": "BAD but not staged\n", "a.txt": "one\nchanged\n"})
	runGit(t, dir, "add", "--intent-to-add", "new.txt")
	runGit(t, dir, "add", "a.txt")

	if code, out := runInRepo(t, dir, &runOptions{}); code != 0 {
		t.Fatalf("intent-to-add files have no staged content, got %d\n%s", code, out)
	}
	if got := gitStatus(t, dir); !strings.Contains(got, " A new.txt") {
		t.Errorf("new.txt should still be intent-to-add, status:\n%s", got)
	}
	if got := fileContent(t, dir, "new.txt"); got != "BAD but not staged\n" {
		t.Errorf("new.txt changed: %q", got)
	}
}

func TestNoStashForAllFilesOrWithoutChanges(t *testing.T) {
	dir := stashRepo(t, "")
	writeFiles(t, dir, map[string]string{"a.txt": "BAD unstaged\n"})

	// --all-files checks the work tree, like pre-commit.
	code, out := runInRepo(t, dir, &runOptions{allFiles: true})
	if code != 1 || strings.Contains(out, "Stashing") {
		t.Fatalf("--all-files must not stash, got %d\n%s", code, out)
	}
	if code, out := runInRepo(t, dir, &runOptions{dryRun: true}); code != 0 || strings.Contains(out, "Stashing") {
		t.Fatalf("--dry-run must not stash, got %d\n%s", code, out)
	}

	writeFiles(t, dir, map[string]string{"a.txt": "one\n"})
	if code, out := runInRepo(t, dir, &runOptions{}); code != 0 || strings.Contains(out, "Unstaged") {
		t.Fatalf("nothing to stash, got %d\n%s", code, out)
	}
}

func TestStashRestoreFailureKeepsPatch(t *testing.T) {
	dir := stashRepo(t, "")
	writeFiles(t, dir, map[string]string{"a.txt": "one\nunstaged\n"})
	s, err := stashUnstaged(dir, t.TempDir(), &strings.Builder{})
	if err != nil {
		t.Fatal(err)
	}
	patch := s.patch
	// Make the patch impossible to apply, even after rolling back the work tree.
	if err := os.WriteFile(patch, []byte("not a patch"), 0o600); err != nil {
		t.Fatal(err)
	}
	if restoreErr := s.restore(); restoreErr == nil || !strings.Contains(restoreErr.Error(), "they are saved in "+patch) {
		t.Fatalf("expected the patch location in the error, got %v", restoreErr)
	}
}
