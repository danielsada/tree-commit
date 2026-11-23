package main

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const exampleDir = "example_repos/example_monorepo"

// exampleManifests stands in for the remote hook repositories used by the
// example (pre-commit-hooks and ruff-pre-commit), so it resolves offline.
type exampleManifests struct{}

func (exampleManifests) manifest(_, _ string) (dir string, hooks map[string]map[string]any, err error) {
	hook := func(id string, filters map[string]any) map[string]any {
		h := map[string]any{"id": id, "name": id, "entry": id, "language": "system"}
		for k, v := range filters {
			h[k] = v
		}
		return h
	}
	text := map[string]any{"types": []any{"text"}}
	python := map[string]any{"types_or": []any{"python", "pyi"}}
	return "", map[string]map[string]any{
		"trailing-whitespace":             hook("trailing-whitespace", text),
		"end-of-file-fixer":               hook("end-of-file-fixer", text),
		"check-yaml":                      hook("check-yaml", map[string]any{"types": []any{"yaml"}}),
		"check-toml":                      hook("check-toml", map[string]any{"types": []any{"toml"}}),
		"check-merge-conflict":            hook("check-merge-conflict", text),
		"check-added-large-files":         hook("check-added-large-files", nil),
		"check-executables-have-shebangs": hook("check-executables-have-shebangs", text),
		"ruff-check":                      hook("ruff-check", python),
		"ruff-format":                     hook("ruff-format", python),
	}, nil
}

// examplePlan resolves the example monorepo for the given files and returns,
// for each file, the hooks that apply to it as "id@config-dir".
func examplePlan(t *testing.T, files ...string) map[string][]string {
	t.Helper()
	tree, err := DiscoverConfigs(exampleDir)
	if err != nil {
		t.Fatal(err)
	}
	scopes, err := scopesFromTree(tree)
	if err != nil {
		t.Fatal(err)
	}
	native := make([]string, len(files))
	for i, f := range files {
		native[i] = filepath.FromSlash(f)
	}
	plan := buildPlan(exampleDir, scopes, native, exampleManifests{}, planOptions{stage: "pre-commit"})

	got := make(map[string][]string)
	for _, r := range plan {
		if r.def.err != nil {
			t.Fatalf("hook %s in %s failed to resolve: %v", r.def.id, r.def.scope.path, r.def.err)
		}
		for _, f := range r.files {
			key := filepath.ToSlash(f)
			got[key] = append(got[key], r.def.id+"@"+filepath.ToSlash(r.def.scope.dir))
		}
	}
	return got
}

func assertHooks(t *testing.T, plan map[string][]string, file string, want, notWant []string) {
	t.Helper()
	got := plan[file]
	for _, w := range want {
		if !slices.Contains(got, w) {
			t.Errorf("%s: expected hook %s, got %v", file, w, got)
		}
	}
	for _, n := range notWant {
		if slices.Contains(got, n) {
			t.Errorf("%s: hook %s should not apply, got %v", file, n, got)
		}
	}
}

// TestExampleMonorepoResolution checks the behavior each example config is
// meant to demonstrate (see example_repos/example_monorepo/README.md).
func TestExampleMonorepoResolution(t *testing.T) {
	const (
		engine  = "python_lib/src/calc/core/engine.py"
		format  = "python_lib/src/calc/utils/formatting.py"
		parsing = "python_lib/src/calc/utils/helpers/parsing.py"
		invoice = "legacy/billing_v1/invoice.py"
		k8s     = "infra/k8s/api.yaml"
		app     = "services/api/src/api/app.py"
		gomain  = "go_lib/cmd/calc/main.go"
		gopkg   = "go_lib/pkg/mathutil/mathutil.go"
		script  = "scripts/release.sh"
	)
	plan := examplePlan(t, engine, format, parsing, invoice, k8s, app, gomain, gopkg, script)

	// core/ replaces the root ruff-check (closest wins) and adds a pygrep hook;
	// it still inherits ruff-format, pytest and the repo-wide checks.
	assertHooks(t, plan, engine,
		[]string{"ruff-check@python_lib/src/calc/core", "no-print@python_lib/src/calc/core",
			"ruff-format@.", "pytest@python_lib", "trailing-whitespace@.", "codespell@."},
		[]string{"ruff-check@.", "complexity@python_lib/src/calc/utils"})

	// utils/ adds a complexity check that excludes helpers/ (exclude_folders).
	assertHooks(t, plan, format, []string{"complexity@python_lib/src/calc/utils", "ruff-check@."}, nil)
	assertHooks(t, plan, parsing,
		[]string{"ruff-format@python_lib/src/calc/utils/helpers", "ruff-check@."},
		[]string{"ruff-format@.", "complexity@python_lib/src/calc/utils"})

	// legacy/ disables ruff and codespell but keeps the whitespace fixers.
	assertHooks(t, plan, invoice,
		[]string{"trailing-whitespace@.", "end-of-file-fixer@.", "todo-needs-ticket@."},
		[]string{"ruff-check@.", "ruff-format@.", "codespell@."})

	// infra/k8s/ overrides the remote check-yaml hook with extra args.
	assertHooks(t, plan, k8s, []string{"check-yaml@infra/k8s"}, []string{"check-yaml@."})

	// services/api/ uses the remote ruff-pre-commit hooks instead of the root ones.
	assertHooks(t, plan, app,
		[]string{"ruff-check@services/api", "ruff-format@services/api", "pytest@services/api"},
		[]string{"ruff-check@.", "pytest@python_lib"})

	// go-test is skipped for changes that only touch cmd/ (exclude_folders).
	assertHooks(t, plan, gomain, []string{"gofmt@go_lib", "go-vet@go_lib"}, []string{"go-test@go_lib"})
	assertHooks(t, plan, gopkg, []string{"gofmt@go_lib", "go-vet@go_lib", "go-test@go_lib"}, nil)

	// Shell scripts get the shell hooks (types: [shell]) but not ruff.
	assertHooks(t, plan, script, []string{"shellcheck@scripts", "shfmt@scripts"}, []string{"ruff-check@."})
}

// copyTree copies the example into dst, skipping local environments and caches.
func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	skip := map[string]bool{".venv": true, "__pycache__": true, ".pytest_cache": true, ".ruff_cache": true, "node_modules": true}
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && skip[d.Name()] {
			return filepath.SkipDir
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o750)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o600)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestExampleMonorepoEndToEnd runs every hook of the example for real. It needs
// network access, uv, python and go, so it only runs with TREE_COMMIT_E2E=1.
func TestExampleMonorepoEndToEnd(t *testing.T) {
	if os.Getenv("TREE_COMMIT_E2E") == "" {
		t.Skip("set TREE_COMMIT_E2E=1 to run (needs network, uv, python and go)")
	}
	for _, tool := range []string{"uv", "uvx", "go"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Fatalf("%s is required: %v", tool, err)
		}
	}
	// Node.js tooling is not part of this test.
	t.Setenv("SKIP", "prettier,eslint,tsc")

	dir := t.TempDir()
	copyTree(t, exampleDir, dir)
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "add", "-A")

	code, out := runInRepo(t, dir, &runOptions{allFiles: true})
	if code != 0 {
		t.Fatalf("expected the example to pass, got %d\n%s", code, out)
	}
	if n := strings.Count(out, "Passed"); n < 20 {
		t.Errorf("expected at least 20 passing hooks, got %d\n%s", n, out)
	}

	// A public function without docstring or annotations, using print, in core/.
	engine := filepath.Join(dir, "python_lib", "src", "calc", "core", "engine.py")
	data, err := os.ReadFile(engine)
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, "\n\ndef helper(x):\n    print(x)  # TODO remove\n    return x\n"...)
	if err := os.WriteFile(engine, data, 0o600); err != nil {
		t.Fatal(err)
	}

	code, out = runInRepo(t, dir, &runOptions{files: []string{engine}})
	if code != 1 {
		t.Fatalf("expected the strict core checks to fail, got %d\n%s", code, out)
	}
	// The new TODO fails; the baselined TODO in legacy/ does not.
	for _, want := range []string{"ANN201", "D103", "no print() in library code", "TODO needs a ticket", "not in the baseline"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}
