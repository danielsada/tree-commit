package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigErrors(t *testing.T) {
	tests := []struct {
		name, content, want string
	}{
		{"invalid yaml", "repos: [broken\n", "failed to parse config file"},
		{"wrong type", "repos: 42\n", "failed to parse config file"},
		{"missing repo", "repos:\n  - rev: v1\n    hooks: []\n", "repos[0]: missing required field 'repo'"},
		{"missing rev", "repos:\n  - repo: https://example.com/hooks\n    hooks: []\n", "(https://example.com/hooks): missing required field 'rev'"},
		{"option-like repo", "repos:\n  - repo: --upload-pack=x\n    rev: v1\n", "must not start with '-'"},
		{"missing id", "repos:\n  - repo: local\n    hooks:\n      - id: ok\n        entry: x\n      - name: no id\n", "repos[0].hooks[1]: missing required field 'id'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), treeCommitConfigFile)
			if err := os.WriteFile(path, []byte(tt.content), 0o600); err != nil {
				t.Fatal(err)
			}
			cfg, err := loadConfig(path)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("loadConfig = %v, %v; want error containing %q", cfg, err, tt.want)
			}
			if !strings.Contains(err.Error(), path) {
				t.Errorf("error should name the config file: %v", err)
			}
		})
	}

	_, err := loadConfig(filepath.Join(t.TempDir(), "missing.yaml"))
	if err == nil || !strings.Contains(err.Error(), "failed to read config file") || !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected read error for a missing file, got %v", err)
	}
}

func TestLoadConfigAcceptsLocalAndMetaWithoutRev(t *testing.T) {
	path := filepath.Join(t.TempDir(), treeCommitConfigFile)
	content := "repos:\n  - repo: local\n    hooks: [{id: a, entry: x}]\n  - repo: meta\n    hooks: [{id: check-useless-excludes}]\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(path)
	if err != nil || len(cfg.Repos) != 2 {
		t.Fatalf("loadConfig = %+v, %v", cfg, err)
	}
}

func TestDecodeHookErrors(t *testing.T) {
	if _, err := decodeHook(map[string]any{"id": "x", "args": "not-a-list"}); err == nil || !strings.Contains(err.Error(), "failed to decode hook") {
		t.Errorf("expected decode error, got %v", err)
	}

	tests := []struct {
		name string
		raw  map[string]any
		want string
	}{
		{"missing entry", map[string]any{"id": "x"}, "missing required field 'entry'"},
		{"bad field type", map[string]any{"id": "x", "entry": "e", "stages": "pre-commit"}, "failed to decode hook"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := decodeLocalHook(tt.raw); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("decodeLocalHook = %v, want %q", err, tt.want)
			}
		})
	}

	// A disabled local hook needs no entry, and gets the default name and language.
	h, err := decodeLocalHook(map[string]any{"id": "x", "disable": true})
	if err != nil || h.Name != "x" || h.Language != langSystem {
		t.Errorf("decodeLocalHook(disabled) = %+v, %v", h, err)
	}
}

func TestLoadManifest(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}

	hooks, err := loadManifest(write("ok.yaml", "- id: a\n  entry: x\n- name: no id\n- id: b\n  entry: y\n"))
	if err != nil || len(hooks) != 2 || hooks["a"]["entry"] != "x" || hooks["b"]["entry"] != "y" {
		t.Errorf("loadManifest = %v, %v; want hooks a and b only", hooks, err)
	}

	if _, err := loadManifest(write("bad.yaml", "id: not-a-list\n")); err == nil || !strings.Contains(err.Error(), "failed to parse hook manifest") {
		t.Errorf("expected parse error, got %v", err)
	}
	if _, err := loadManifest(filepath.Join(dir, "missing.yaml")); err == nil || !strings.Contains(err.Error(), "failed to read hook manifest") {
		t.Errorf("expected read error, got %v", err)
	}
}

// fakeManifests serves remote hook manifests from memory.
type fakeManifests struct {
	dir   string
	hooks map[string]map[string]any
	err   error
}

func (f *fakeManifests) manifest(_, _ string) (dir string, hooks map[string]map[string]any, err error) {
	return f.dir, f.hooks, f.err
}

func TestResolveHookErrors(t *testing.T) {
	remote := &fakeManifests{dir: "/cache/hooks", hooks: map[string]map[string]any{
		"good":       {"id": "good", "name": "Good", "entry": "good-tool", "language": "system"},
		"no-entry":   {"id": "no-entry", "language": "system"},
		"no-lang":    {"id": "no-lang", "entry": "x"},
		"bad-fields": {"id": "bad-fields", "entry": "x", "language": "system", "args": "oops"},
	}}
	tests := []struct {
		name      string
		source    manifestSource
		repo      RepoConfig
		raw       map[string]any
		wantErr   string
		wantSkip  string
		wantName  string
		wantRepoD string
	}{
		{name: "local hook without entry", repo: RepoConfig{Repo: repoLocal},
			raw: map[string]any{"id": "x", "name": "Pretty"}, wantErr: "missing required field 'entry'", wantName: "Pretty"},
		{name: "invalid files pattern", repo: RepoConfig{Repo: repoLocal},
			raw: map[string]any{"id": "x", "entry": "e", "files": "("}, wantErr: "invalid 'files' pattern", wantName: "x"},
		{name: "invalid exclude pattern", repo: RepoConfig{Repo: repoLocal},
			raw: map[string]any{"id": "x", "entry": "e", "exclude": "[a-"}, wantErr: "invalid 'exclude' pattern", wantName: "x"},
		{name: "meta hook", repo: RepoConfig{Repo: repoMeta},
			raw: map[string]any{"id": "check-hooks-apply"}, wantSkip: "meta hooks are not supported", wantName: "check-hooks-apply"},
		{name: "remote without source", repo: RepoConfig{Repo: "https://example.com/hooks", Rev: "v1"},
			raw: map[string]any{"id": "good"}, wantErr: "remote repositories are not available"},
		{name: "disabled remote hook is not fetched", repo: RepoConfig{Repo: "https://example.com/hooks", Rev: "v1"},
			raw: map[string]any{"id": "good", "disable": true}, wantName: "good"},
		{name: "manifest error", source: &fakeManifests{err: errors.New("clone failed")}, repo: RepoConfig{Repo: "r", Rev: "v1"},
			raw: map[string]any{"id": "good"}, wantErr: "clone failed"},
		{name: "manifest missing entry", source: remote, repo: RepoConfig{Repo: "r", Rev: "v1"},
			raw: map[string]any{"id": "no-entry"}, wantErr: "missing 'entry' or 'language'"},
		{name: "manifest missing language", source: remote, repo: RepoConfig{Repo: "r", Rev: "v1"},
			raw: map[string]any{"id": "no-lang"}, wantErr: "missing 'entry' or 'language'"},
		{name: "manifest invalid field", source: remote, repo: RepoConfig{Repo: "r", Rev: "v1"},
			raw: map[string]any{"id": "bad-fields"}, wantErr: "failed to decode hook"},
		{name: "remote hook", source: remote, repo: RepoConfig{Repo: "r", Rev: "v1"},
			raw: map[string]any{"id": "good"}, wantName: "Good", wantRepoD: "/cache/hooks"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := &scope{dir: rootDir, path: treeCommitConfigFile, config: &Config{}}
			r := &resolver{source: tt.source}
			d := r.resolveHook(sc, &tt.repo, tt.raw)
			switch {
			case tt.wantErr == "" && d.err != nil:
				t.Fatalf("unexpected error: %v", d.err)
			case tt.wantErr != "" && (d.err == nil || !strings.Contains(d.err.Error(), tt.wantErr)):
				t.Fatalf("error = %v, want %q", d.err, tt.wantErr)
			case tt.wantErr != "" && !strings.Contains(d.err.Error(), treeCommitConfigFile):
				t.Errorf("error should name the config file: %v", d.err)
			}
			if d.skipReason != tt.wantSkip {
				t.Errorf("skipReason = %q, want %q", d.skipReason, tt.wantSkip)
			}
			if d.hook == nil {
				t.Fatal("hook must always be set so that it can be reported")
			}
			if tt.wantName != "" && d.hook.Name != tt.wantName {
				t.Errorf("name = %q, want %q", d.hook.Name, tt.wantName)
			}
			if d.repoDir != tt.wantRepoD {
				t.Errorf("repoDir = %q, want %q", d.repoDir, tt.wantRepoD)
			}
		})
	}
}

func TestBuildPlanReportsBrokenHooksAndFiltersFiles(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"src/a.py":        "print(1)\n",
		"src/gen/b.py":    "print(2)\n",
		"vendor/c.py":     "print(3)\n",
		"docs/readme.txt": "text\n",
	})
	sc := &scope{dir: rootDir, path: treeCommitConfigFile, config: &Config{Repos: []RepoConfig{{
		Repo: repoLocal,
		Hooks: []map[string]any{
			{"id": "broken", "entry": "x", "files": "("},
			{"id": "py", "entry": "x", "files": `\.py$`, "exclude": "^src/gen/"},
			{"id": "text-only", "entry": "x", "types_or": []any{"markdown", "plain-text"}},
			{"id": "no-python", "entry": "x", "exclude_types": []any{"python"}},
			{"id": "dirs-only", "entry": "x", "types": []any{"directory"}},
		},
	}}}}
	var err error
	if sc.filesRe, err = compilePattern("^(src|vendor|docs)/"); err != nil {
		t.Fatal(err)
	}
	if sc.excludeRe, err = compilePattern("^vendor/"); err != nil {
		t.Fatal(err)
	}
	files := []string{
		filepath.Join("src", "a.py"), filepath.Join("src", "gen", "b.py"),
		filepath.Join("vendor", "c.py"), filepath.Join("docs", "readme.txt"),
		"deleted.txt", // tags cannot be computed for missing files
	}
	plan := buildPlan(root, []*scope{sc}, files, nil, planOptions{stage: "pre-commit"})

	got := map[string]string{}
	for _, r := range plan {
		if r.noFiles {
			continue
		}
		got[r.def.id] = filepath.ToSlash(strings.Join(r.files, ","))
		if r.def.id == "broken" && (r.def.err == nil || !strings.Contains(r.def.err.Error(), "invalid 'files' pattern")) {
			t.Errorf("broken hook should carry its error, got %v", r.def.err)
		}
	}
	want := map[string]string{
		// Broken hooks get every file so that they are reported as failed.
		"broken":    "deleted.txt,docs/readme.txt,src/a.py,src/gen/b.py,vendor/c.py",
		"py":        "src/a.py",
		"text-only": "docs/readme.txt",
		"no-python": "docs/readme.txt",
	}
	if len(got) != len(want) {
		t.Errorf("plan = %v, want %v", got, want)
	}
	for id, files := range want {
		if got[id] != files {
			t.Errorf("hook %s runs on %q, want %q", id, got[id], files)
		}
	}
}

func TestScopesFromTreeRejectsInvalidTopLevelPatterns(t *testing.T) {
	for _, field := range []string{"files", "exclude"} {
		t.Run(field, func(t *testing.T) {
			cfg := &Config{Files: `\.py$`}
			if field == "files" {
				cfg.Files = "("
			} else {
				cfg.Exclude = "(?P<"
			}
			tree := &ConfigTree{Root: ".", Tree: &ConfigNode{Path: treeCommitConfigFile, Config: cfg}}
			_, err := scopesFromTree(tree)
			if err == nil || !strings.Contains(err.Error(), "invalid top-level '"+field+"'") {
				t.Fatalf("scopesFromTree = %v, want invalid top-level %s error", err, field)
			}
		})
	}
}

func TestMatchesTypes(t *testing.T) {
	tags := map[string]bool{"file": true, "text": true, "python": true}
	tests := []struct {
		name                      string
		types, typesOr, excludeTs []string
		want                      bool
	}{
		{"no filters", nil, nil, nil, true},
		{"all types present", []string{"file", "python"}, nil, nil, true},
		{"missing type", []string{"file", "go"}, nil, nil, false},
		{"types_or one present", nil, []string{"go", "python"}, nil, true},
		{"types_or none present", nil, []string{"go", "rust"}, nil, false},
		{"excluded type", []string{"file"}, nil, []string{"python"}, false},
		{"exclude absent type", []string{"file"}, []string{"python"}, []string{"binary"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesTypes(tags, tt.types, tt.typesOr, tt.excludeTs); got != tt.want {
				t.Errorf("matchesTypes = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFileTags(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, data []byte, perm os.FileMode) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, data, perm); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(p, perm); err != nil {
			t.Fatal(err)
		}
		return p
	}
	tests := []struct {
		name      string
		path      string
		want, not []string
	}{
		{"binary without extension", write("blob", []byte{0x7f, 'E', 'L', 'F', 0, 1, 2}, 0o644),
			[]string{"file", "binary", "non-executable"}, []string{"text", "executable"}},
		{"text without extension", write("NOTES", []byte("hello\tworld\r\n\x1b[0m"), 0o644),
			[]string{"file", "text"}, []string{"binary"}},
		{"executable script with shebang", write("run", []byte("#!/usr/bin/env -S python3 -u\nprint(1)\n"), 0o755),
			[]string{"file", "text", "executable", "python", "python3"}, []string{"non-executable"}},
		{"well-known name", write("Makefile", []byte("all:\n"), 0o644),
			[]string{"text", "makefile"}, nil},
		{"binary extension", write("logo.PNG", []byte("not really a png"), 0o644),
			[]string{"binary", "image", "png"}, []string{"text"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tags, err := fileTags(tt.path)
			if err != nil {
				t.Fatal(err)
			}
			for _, w := range tt.want {
				if !tags[w] {
					t.Errorf("missing tag %q in %v", w, tags)
				}
			}
			for _, n := range tt.not {
				if tags[n] {
					t.Errorf("unexpected tag %q in %v", n, tags)
				}
			}
		})
	}

	if tags, err := fileTags(dir); err != nil || !tags["directory"] || tags["file"] {
		t.Errorf("fileTags(dir) = %v, %v", tags, err)
	}
	if _, err := fileTags(filepath.Join(dir, "missing")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("fileTags(missing) error = %v, want not exist", err)
	}
}

func TestShebangInterpreter(t *testing.T) {
	tests := map[string]string{
		"#!/bin/bash\necho":        "bash",
		"#!/usr/bin/env node":      "node",
		"#!/usr/bin/env -S ruby":   "ruby",
		"#!\n":                     "",
		"no shebang":               "",
		"#! /usr/bin/python3 -u\n": "python3",
	}
	for head, want := range tests {
		if got := shebangInterpreter([]byte(head)); got != want {
			t.Errorf("shebangInterpreter(%q) = %q, want %q", head, got, want)
		}
	}
}

func TestGitCommandsOutsideRepository(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))

	if _, err := repoRoot(dir); err == nil || !strings.Contains(err.Error(), "not in a git repository") {
		t.Errorf("repoRoot error = %v", err)
	}
	if _, err := stagedFiles(dir); err == nil || !strings.Contains(err.Error(), "git diff") {
		t.Errorf("stagedFiles error = %v", err)
	}
	if _, err := trackedFiles(dir); err == nil || !strings.Contains(err.Error(), "git ls-files") {
		t.Errorf("trackedFiles error = %v", err)
	}
}

func TestPathHelpersWithMixedPaths(t *testing.T) {
	abs, err := filepath.Abs("x")
	if err != nil {
		t.Fatal(err)
	}
	if isWithin("rel", abs) {
		t.Error("a relative dir cannot contain an absolute path")
	}
	if got := relToScope("rel", abs); got != abs {
		t.Errorf("relToScope = %q, want the path unchanged", got)
	}
}

func TestNilConfigTreeRendering(t *testing.T) {
	var node *ConfigNode
	var tree *ConfigTree
	if got := node.String(); got != "<nil>" {
		t.Errorf("nil node String() = %q", got)
	}
	if got := tree.String(); got != "<nil ConfigTree>" {
		t.Errorf("nil tree String() = %q", got)
	}
	if got := (&ConfigTree{Root: "r"}).PrettyPrint(); got != "<empty tree>" {
		t.Errorf("empty tree PrettyPrint() = %q", got)
	}
}
