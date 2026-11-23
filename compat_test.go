package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// Regression tests for incompatibilities found by running tree-commit and
// pre-commit side by side on real-world repositories.

func TestFileTagsMatchIdentify(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"ext.pyx":           "cdef int x = 1\n",
		"Dockerfile.dev":    "FROM scratch\n",
		"notes.ipynb":       "{}\n",
		"data.unknownext":   "plain\n",
		"script":            "#!/usr/bin/env python3.12\nprint(1)\n",
		"script.sh":         "#!/usr/bin/env python3\n",
		"blob.unknownext":   "\x00\x01\x02",
		"pyproject.toml":    "[project]\n",
		".pre-commit-hooks": "",
	})
	if runtime.GOOS != windows {
		for _, name := range []string{"script", "script.sh"} {
			if err := os.Chmod(filepath.Join(dir, name), 0o700); err != nil { //nolint:gosec // test script
				t.Fatal(err)
			}
		}
	}

	tests := []struct {
		file      string
		want, not []string
	}{
		{"ext.pyx", []string{"file", "text", "cython"}, []string{"python"}},
		{"Dockerfile.dev", []string{"text", "dockerfile"}, nil},
		{"notes.ipynb", []string{"text", "jupyter", "json"}, nil},
		{"data.unknownext", []string{"text"}, []string{"binary"}},
		{"blob.unknownext", []string{"binary"}, []string{"text"}},
		{"pyproject.toml", []string{"toml"}, nil},
		// Interpreter versions are stripped until one is known.
		{"script", []string{"executable", "python", "text"}, nil},
		// The extension wins over the shebang.
		{"script.sh", []string{"shell"}, []string{"python"}},
	}
	for _, tt := range tests {
		tags, err := fileTags(filepath.Join(dir, tt.file))
		if err != nil {
			t.Fatal(err)
		}
		for _, w := range tt.want {
			if !tags[w] {
				t.Errorf("%s: missing tag %q in %v", tt.file, w, tags)
			}
		}
		for _, n := range tt.not {
			if tags[n] {
				t.Errorf("%s: unexpected tag %q in %v", tt.file, n, tags)
			}
		}
	}
}

func TestBuiltinRepoUsesPreCommitHooks(t *testing.T) {
	var requested []string
	source := manifestRecorder(func(repo, rev string) {
		requested = append(requested, repo+"@"+rev)
	})
	cfg := &Config{Repos: []RepoConfig{{Repo: repoBuiltin, Hooks: []map[string]any{{"id": "check-yaml"}}}}}
	if err := cfg.validate(); err != nil {
		t.Fatalf("builtin repos do not need a rev: %v", err)
	}
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"a.yaml": "a: 1\n"})
	plan := buildPlan(root, []*scope{{dir: rootDir, path: ".tree-commit-config.yaml", config: cfg}},
		[]string{"a.yaml"}, source, planOptions{stage: "pre-commit"})

	if len(plan) != 1 || plan[0].def.err != nil {
		t.Fatalf("expected check-yaml to resolve, got %+v", plan)
	}
	want := builtinHooksRepo + "@" + builtinHooksRev
	if !slices.Equal(requested, []string{want}) {
		t.Errorf("requested manifests %v, want [%s]", requested, want)
	}
}

// manifestRecorder is a manifestSource that records requests and defines any hook id.
type manifestRecorder func(repo, rev string)

func (m manifestRecorder) manifest(repo, rev string) (dir string, hooks map[string]map[string]any, err error) {
	m(repo, rev)
	return "", map[string]map[string]any{
		"check-yaml": {"id": "check-yaml", "name": "check yaml", "entry": "check-yaml", "language": "python", "types": []any{"yaml"}},
	}, nil
}

func TestUVExecutable(t *testing.T) {
	fake := filepath.Join(t.TempDir(), "my-uv")
	for _, tt := range []struct{ setting, want string }{
		{"0", ""}, {"off", ""}, {"FALSE", ""}, {fake, fake},
	} {
		t.Setenv("TREE_COMMIT_UV", tt.setting)
		if got := uvExecutable(); got != tt.want {
			t.Errorf("TREE_COMMIT_UV=%q: uvExecutable() = %q, want %q", tt.setting, got, tt.want)
		}
	}
	t.Setenv("TREE_COMMIT_UV", "")
	t.Setenv("PATH", t.TempDir())
	if got := uvExecutable(); got != "" {
		t.Errorf("without uv on PATH, uvExecutable() = %q", got)
	}
}

// TestPythonEnvWithUV creates a real environment with uv, offline: without a
// hook repository and dependencies nothing needs to be downloaded.
func TestPythonEnvWithUV(t *testing.T) {
	uv, err := exec.LookPath("uv")
	if err != nil {
		t.Skip("uv is not installed")
	}
	c := &repoCache{dir: t.TempDir(), manifests: map[string]*cachedManifest{}, envs: map[string]*cachedEnv{}}
	if _, pyErr := c.findPython(); pyErr != nil {
		t.Skip("python 3 is not installed")
	}
	t.Setenv("TREE_COMMIT_UV", uv)
	t.Setenv("UV_OFFLINE", "1")

	envDir, err := c.pythonEnv("", nil)
	if err != nil {
		t.Fatalf("pythonEnv with uv failed: %v", err)
	}
	if !fileExists(venvPython(envDir)) {
		t.Fatalf("no interpreter in %s", envDir)
	}

	// Installing from a missing directory reports uv's error.
	err = installPackages(uv, envDir, []string{filepath.Join(t.TempDir(), "missing")})
	if err == nil || !strings.Contains(err.Error(), "uv pip install failed") {
		t.Fatalf("expected a uv pip install error, got %v", err)
	}
}

func TestCreateVirtualenvErrorSuggestsUV(t *testing.T) {
	python := fakePython(t)
	t.Setenv("FAKE_VENV_FAIL", "1")
	err := createVirtualenv("", python, filepath.Join(t.TempDir(), "env"))
	if err == nil || !strings.Contains(err.Error(), "Install uv") {
		t.Fatalf("expected a hint to install uv, got %v", err)
	}
}
