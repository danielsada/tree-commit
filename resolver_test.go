package main

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSplitCommand(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"eslint --fix", []string{"eslint", "--fix"}},
		{`black  --line-length 79 `, []string{"black", "--line-length", "79"}},
		{`'C:/Program Files/tool.exe' -x`, []string{"C:/Program Files/tool.exe", "-x"}},
		{`sh -c "echo \"hi\" \n"`, []string{"sh", "-c", `echo "hi" \n`}},
		{`a\ b c`, []string{"a b", "c"}},
		{`x ''`, []string{"x", ""}},
	}
	for _, tt := range tests {
		got, err := splitCommand(tt.in)
		if err != nil {
			t.Errorf("splitCommand(%q) error: %v", tt.in, err)
			continue
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("splitCommand(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
	if _, err := splitCommand(`echo "unterminated`); err == nil {
		t.Error("expected error for unterminated quote")
	}
}

func TestFolderFilters(t *testing.T) {
	tests := []struct {
		rel              string
		include, exclude []string
		want             bool
	}{
		{"src/a.js", nil, nil, true},
		{"src/a.js", []string{"src/"}, nil, true},
		{"lib/a.js", []string{"src/", "components"}, nil, false},
		{"lib/a.js", []string{"."}, nil, true},
		{"src/a.js", []string{"./src"}, nil, true},
		{"node_modules/x/a.js", nil, []string{"node_modules/"}, false},
		{"src/__pycache__/a.py", []string{"."}, []string{"__pycache__/"}, false},
		{"helpers/a.py", []string{"."}, []string{"helpers/"}, false},
		{"src/helpers.py", []string{"."}, []string{"helpers/"}, true},
		{"a/rebuild/x.js", nil, []string{"build"}, true},
		{"a/gen/sub/x.go", nil, []string{"/gen/sub/"}, false},
		{"srcx/a.js", []string{"src"}, nil, false},
	}
	for _, tt := range tests {
		rel := filepath.FromSlash(tt.rel)
		got := folderIncluded(rel, tt.include) && !folderExcluded(rel, tt.exclude)
		if got != tt.want {
			t.Errorf("%s include=%v exclude=%v: got %v, want %v", tt.rel, tt.include, tt.exclude, got, tt.want)
		}
	}
}

func TestCompilePatternVerbose(t *testing.T) {
	re, err := compilePattern(`(?x)^(
    vendor/.*|   # vendored code
    gen/[a-z ]+\.go
)$`)
	if err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]bool{"vendor/x.go": true, "gen/a b.go": true, "src/x.go": false} {
		if got := re.MatchString(path); got != want {
			t.Errorf("match(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestHookDependencies(t *testing.T) {
	root := &scope{dir: rootDir}
	sub := &scope{dir: "go_lib", depth: 1}
	no := false
	mk := func(sc *scope, passFiles bool, files ...string) *hookRun {
		h := &Hook{}
		if !passFiles {
			h.PassFilenames = &no
		}
		for i, f := range files {
			files[i] = filepath.FromSlash(f)
		}
		return &hookRun{def: &hookDef{scope: sc, hook: h}, files: files}
	}

	runs := []*hookRun{
		mk(sub, true, "go_lib/a.go"),               // 0: gofmt
		mk(sub, false, "go_lib/a.go"),              // 1: go test ./... (whole directory)
		mk(root, true, "docs/x.md"),                // 2: unrelated
		mk(root, true, "go_lib/a.go", "docs/y.md"), // 3: shares a file with 0
	}
	got := hookDependencies(runs)
	want := [][]int{nil, {0}, nil, {0, 1}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("hookDependencies = %v, want %v", got, want)
	}
}

func TestBuildPlanOrdersDeeperConfigsFirst(t *testing.T) {
	cfg := func(ids ...string) *Config {
		repo := RepoConfig{Repo: repoLocal}
		for _, id := range ids {
			repo.Hooks = append(repo.Hooks, map[string]any{"id": id, "entry": "true"})
		}
		return &Config{Repos: []RepoConfig{repo}}
	}
	scopes := []*scope{
		{dir: rootDir, path: ".tree-commit-config.yaml", config: cfg("validator")},
		{dir: "frontend", path: filepath.Join("frontend", ".tree-commit-config.yaml"), depth: 1, config: cfg("formatter", "linter")},
	}
	file := filepath.Join("frontend", "src", "App.js")
	plan := buildPlan(t.TempDir(), scopes, []string{file}, nil, planOptions{stage: "pre-commit"})

	var ids []string
	for _, r := range plan {
		ids = append(ids, r.def.id)
	}
	if got := strings.Join(ids, ","); got != "formatter,linter,validator" {
		t.Errorf("execution order = %s, want formatter,linter,validator", got)
	}
}
