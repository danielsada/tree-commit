package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const grepConfig = `repos:
  - repo: local
    hooks:
      - id: no-print
        name: no print
        entry: '^\s*print\('
        language: grep
        types: [python]
        baseline: .grep-baseline.json
      - id: todo-count
        name: TODO count
        entry: 'TODO(?!\(#\d+\))'
        language: grep
        exclude: \.yaml$
        baseline: .grep-baseline.json
        baseline_mode: count
`

func record(t *testing.T, dir string, ids ...string) string {
	t.Helper()
	var out strings.Builder
	if err := recordBaselines(dir, ids, &out); err != nil {
		t.Fatalf("recordBaselines failed: %v\n%s", err, out.String())
	}
	return out.String()
}

func readBaseline(t *testing.T, path string) *baselineDoc {
	t.Helper()
	doc, err := loadBaseline(path)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func mustRun(t *testing.T, dir string, wantCode int, wantOut ...string) string {
	t.Helper()
	code, out := runInRepo(t, dir, &runOptions{allFiles: true})
	if code != wantCode {
		t.Fatalf("exit code %d, want %d\n%s", code, wantCode, out)
	}
	for _, w := range wantOut {
		if !strings.Contains(out, w) {
			t.Errorf("output missing %q:\n%s", w, out)
		}
	}
	return out
}

func TestGrepBaselineRatchetLines(t *testing.T) {
	dir := newRepo(t, map[string]string{
		".tree-commit-config.yaml": grepConfig,
		"a.py":                     "print('a')\nx = 1\n    print('indented')\n",
		"b.py":                     "print('b')\n",
	})
	runGit(t, dir, "add", "-A")
	baseline := filepath.Join(dir, ".grep-baseline.json")

	// Without a baseline file every match is new.
	mustRun(t, dir, 1, "a.py:1:print('a')", "3 violation(s) are not in the baseline")

	out := record(t, dir, "no-print")
	if !strings.Contains(out, "no print: recorded 3 violation(s) in 2 file(s) to .grep-baseline.json") {
		t.Errorf("unexpected record output: %s", out)
	}
	doc := readBaseline(t, baseline)
	if got := doc.Hooks["no-print"].Files["a.py"].Lines; got["print('a')"] != 1 || got["print('indented')"] != 1 {
		t.Fatalf("unexpected baseline for a.py: %v", got)
	}
	mustRun(t, dir, 0)

	// Moving and reindenting baselined lines is fine.
	writeFiles(t, dir, map[string]string{"a.py": "x = 1\nprint('indented')\n\n  print('a')\n"})
	mustRun(t, dir, 0)

	// A new violation fails, and so does a second copy of a baselined line.
	writeFiles(t, dir, map[string]string{"a.py": "print('a')\nprint('a')\nprint('indented')\nprint('new')\n"})
	out = mustRun(t, dir, 1, "a.py:2:print('a')", "a.py:4:print('new')", "2 violation(s) are not in the baseline")
	if strings.Contains(out, "a.py:1:") || strings.Contains(out, "a.py:3:") {
		t.Errorf("baselined lines must not be reported:\n%s", out)
	}

	// Fixing a violation shrinks the baseline, like a formatter: fail once, then pass.
	writeFiles(t, dir, map[string]string{"a.py": "print('a')\nprint('indented')\n", "b.py": "pass\n"})
	mustRun(t, dir, 1, "Removed 1 fixed violation(s) from the baseline .grep-baseline.json", "- files were modified by this hook")
	if _, ok := readBaseline(t, baseline).Hooks["no-print"].Files["b.py"]; ok {
		t.Error("b.py should be removed from the baseline")
	}
	mustRun(t, dir, 0)

	// Deleted files are removed from the baseline.
	runGit(t, dir, "rm", "-qf", "a.py")
	mustRun(t, dir, 1, "Removed 2 fixed violation(s)")
	if _, ok := readBaseline(t, baseline).Hooks["no-print"]; ok {
		t.Error("the hook should have no baseline left")
	}
}

func TestGrepBaselineRatchetCount(t *testing.T) {
	dir := newRepo(t, map[string]string{
		".tree-commit-config.yaml": grepConfig,
		"notes.md":                 "TODO one\nTODO(#1) tracked\nTODO two\n",
	})
	runGit(t, dir, "add", "-A")
	record(t, dir, "todo-count")
	baseline := filepath.Join(dir, ".grep-baseline.json")
	if got := readBaseline(t, baseline).Hooks["todo-count"].Files["notes.md"].Count; got != 2 {
		t.Fatalf("count = %d, want 2", got)
	}
	// The baseline itself is never checked, although it contains "TODO".
	mustRun(t, dir, 0)

	// Count mode only looks at the number of violations per file.
	writeFiles(t, dir, map[string]string{"notes.md": "TODO three\nTODO four\n"})
	mustRun(t, dir, 0)

	writeFiles(t, dir, map[string]string{"notes.md": "TODO three\nTODO four\nTODO five\n"})
	mustRun(t, dir, 1, "notes.md:1:TODO three", "notes.md:3:TODO five")

	writeFiles(t, dir, map[string]string{"notes.md": "TODO three\n"})
	mustRun(t, dir, 1, "Removed 1 fixed violation(s)")
	if got := readBaseline(t, baseline).Hooks["todo-count"].Files["notes.md"].Count; got != 1 {
		t.Errorf("count = %d after the fix, want 1", got)
	}
}

func TestGrepBaselineSharedFileAndNestedScope(t *testing.T) {
	dir := newRepo(t, map[string]string{
		"svc/.tree-commit-config.yaml": grepConfig,
		"svc/app.py":                   "print('x')  # TODO later\n",
	})
	runGit(t, dir, "add", "-A")
	out := record(t, dir)
	if !strings.Contains(out, "to svc/.grep-baseline.json") || strings.Count(out, "recorded") != 2 {
		t.Fatalf("both hooks should be recorded next to their config file:\n%s", out)
	}
	doc := readBaseline(t, filepath.Join(dir, "svc", ".grep-baseline.json"))
	data, _ := json.Marshal(doc)
	if !strings.Contains(string(data), `"app.py"`) {
		t.Errorf("paths must be relative to the config directory: %s", data)
	}

	// Both hooks shrink the same file in one run; neither update is lost.
	writeFiles(t, dir, map[string]string{"svc/app.py": "pass\n"})
	mustRun(t, dir, 1, "Removed 1 fixed violation(s)")
	if doc := readBaseline(t, filepath.Join(dir, "svc", ".grep-baseline.json")); len(doc.Hooks) != 0 {
		t.Errorf("all violations are fixed, baseline still has %v", doc.Hooks)
	}
}

func TestGrepBaselineErrors(t *testing.T) {
	tests := []struct {
		name, hook, want string
	}{
		{"not a grep hook", "language: system\n        entry: tool\n        baseline: b.json", "only supported for grep and pygrep"},
		{"bad mode", "language: grep\n        entry: x\n        baseline: b.json\n        baseline_mode: words", `baseline_mode must be "lines" or "count"`},
		{"mode without file", "language: grep\n        entry: x\n        baseline_mode: count", "baseline_mode needs a baseline file"},
		{"outside the directory", "language: grep\n        entry: x\n        baseline: ../b.json", "inside the config file's directory"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := newRepo(t, map[string]string{
				".tree-commit-config.yaml": "repos:\n  - repo: local\n    hooks:\n      - id: h\n        " + tt.hook + "\n",
				"a.txt":                    "x\n",
			})
			runGit(t, dir, "add", "-A")
			mustRun(t, dir, 1, tt.want)
		})
	}

	dir := newRepo(t, map[string]string{".tree-commit-config.yaml": grepConfig, "a.py": "print(1)\n"})
	runGit(t, dir, "add", "-A")
	var out strings.Builder
	if err := recordBaselines(dir, []string{"nope"}, &out); err == nil || !strings.Contains(err.Error(), "nope: no grep hook with a baseline") {
		t.Errorf("expected an unknown id error, got %v", err)
	}

	// A baseline recorded in another mode must be recorded again.
	record(t, dir, "no-print")
	path := filepath.Join(dir, ".grep-baseline.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	writeFiles(t, dir, map[string]string{".grep-baseline.json": strings.Replace(string(data), `"mode": "lines"`, `"mode": "count"`, 1)})
	mustRun(t, dir, 1, "run `tree-commit baseline no-print`")

	writeFiles(t, dir, map[string]string{".grep-baseline.json": "{not json"})
	mustRun(t, dir, 1, "invalid baseline")
	writeFiles(t, dir, map[string]string{".grep-baseline.json": `{"version": 9}`})
	mustRun(t, dir, 1, "unsupported version 9")

	empty := newRepo(t, map[string]string{".tree-commit-config.yaml": "repos: []\n", "a.txt": "a\n"})
	runGit(t, empty, "add", "-A")
	if err := recordBaselines(empty, nil, &out); err == nil || !strings.Contains(err.Error(), "no grep or pygrep hook has a baseline") {
		t.Errorf("expected an error without baselines, got %v", err)
	}
}

func TestPythonRegexInFilePatterns(t *testing.T) {
	logPath := filepath.ToSlash(filepath.Join(t.TempDir(), "hooks.log"))
	dir := newRepo(t, map[string]string{
		".tree-commit-config.yaml": `repos:
  - repo: local
    hooks:
      - id: only-src
        entry: "` + strings.ReplaceAll(helperEntry(t, "@log="+logPath, "@name=src"), `"`, `\"`) + `"
        exclude: ^(?!src/)
`,
		"src/a.txt":  "a\n",
		"docs/b.txt": "b\n",
	})
	runGit(t, dir, "add", "-A")
	mustRun(t, dir, 0)
	assertLines(t, readLog(t, logPath), []string{"src|" + filepath.Base(dir) + "|src/a.txt"})
}
