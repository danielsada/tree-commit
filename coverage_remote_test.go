package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// fakePython writes a stand-in for a python interpreter, so that environment
// handling can be tested without python or network access. It prints
// $FAKE_PY_VERSION for "-c <code>", and "-m venv DIR" creates an empty DIR (or
// fails when $FAKE_VENV_FAIL is 1). The environments it creates contain no
// interpreter, so installing packages into them fails. uv is disabled, since it
// would inspect the fake interpreter.
func fakePython(t *testing.T) string {
	t.Helper()
	t.Setenv("TREE_COMMIT_UV", "0")
	dir := t.TempDir()
	if runtime.GOOS == windows {
		p := filepath.Join(dir, "fakepython.cmd")
		script := strings.Join([]string{
			"@echo off",
			`if "%~1"=="-c" (`,
			"  echo %FAKE_PY_VERSION%",
			"  exit /b 0",
			")",
			`if "%FAKE_VENV_FAIL%"=="1" (`,
			"  echo venv exploded",
			"  exit /b 1",
			")",
			`mkdir "%~3"`,
			"",
		}, "\r\n")
		if err := os.WriteFile(p, []byte(script), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	p := filepath.Join(dir, "fakepython")
	script := `#!/bin/sh
if [ "$1" = "-c" ]; then echo "$FAKE_PY_VERSION"; exit 0; fi
if [ "$FAKE_VENV_FAIL" = "1" ]; then echo "venv exploded"; exit 1; fi
mkdir -p "$3"
`
	if err := os.WriteFile(p, []byte(script), 0o700); err != nil { //nolint:gosec // test executable
		t.Fatal(err)
	}
	return p
}

func TestFindPython(t *testing.T) {
	fake := fakePython(t)
	tests := []struct {
		name, override, version string
		want                    string // python path, or "" for an error
	}{
		{"override is used", fake, "3", fake},
		{"python 2 is rejected", fake, "2", ""},
		{"missing override", filepath.Join(t.TempDir(), "no-such-python"), "3", ""},
		{"nothing on PATH", "", "3", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TREE_COMMIT_PYTHON", tt.override)
			t.Setenv("FAKE_PY_VERSION", tt.version)
			t.Setenv("PATH", t.TempDir())
			c := &repoCache{}
			got, err := c.findPython()
			if tt.want == "" {
				if err == nil || !strings.Contains(err.Error(), "python 3 is required") {
					t.Fatalf("findPython = %q, %v; want an error", got, err)
				}
				return
			}
			if err != nil || !strings.EqualFold(got, tt.want) {
				t.Fatalf("findPython = %q, %v; want %q", got, err, tt.want)
			}
			// The interpreter is looked up once per cache.
			t.Setenv("TREE_COMMIT_PYTHON", "")
			if again, err := c.findPython(); err != nil || again != got {
				t.Errorf("second findPython = %q, %v; want cached %q", again, err, got)
			}
		})
	}
}

func TestPythonEnv(t *testing.T) {
	home := t.TempDir()
	t.Setenv("TREE_COMMIT_HOME", home)
	t.Setenv("TREE_COMMIT_PYTHON", fakePython(t))
	t.Setenv("FAKE_PY_VERSION", "3")
	t.Setenv("FAKE_VENV_FAIL", "")

	c, err := newRepoCache()
	if err != nil {
		t.Fatal(err)
	}

	// No hook repository and no dependencies: a bare virtualenv, without pip.
	envDir, err := c.pythonEnv("", nil)
	if err != nil {
		t.Fatalf("pythonEnv failed: %v", err)
	}
	if !isWithin(filepath.Join(home, "envs"), envDir) || !fileExists(filepath.Join(envDir, readyMarker)) {
		t.Fatalf("environment %s is not ready in the cache %s", envDir, home)
	}
	if again, err2 := c.pythonEnv("", nil); err2 != nil || again != envDir {
		t.Errorf("pythonEnv should reuse the environment, got %q, %v", again, err2)
	}

	// Ready environments are reused by other processes without python.
	t.Setenv("TREE_COMMIT_PYTHON", filepath.Join(t.TempDir(), "no-such-python"))
	c2, err := newRepoCache()
	if err != nil {
		t.Fatal(err)
	}
	if again, err2 := c2.pythonEnv("", nil); err2 != nil || again != envDir {
		t.Errorf("ready environment not reused: %q, %v", again, err2)
	}
	if _, err2 := c2.pythonEnv("", []string{"dep"}); err2 == nil || !strings.Contains(err2.Error(), "python 3 is required") {
		t.Errorf("expected missing python error, got %v", err2)
	}

	// Installing packages fails (the fake virtualenv has no interpreter): the
	// broken environment is removed so that the next run starts over.
	repoDir := t.TempDir()
	failed, err := c.pythonEnv(repoDir, []string{"b", "a"})
	if err == nil || !strings.Contains(err.Error(), "pip install failed") {
		t.Fatalf("expected pip failure, got %v", err)
	}
	if fileExists(failed) {
		t.Errorf("failed environment %s was not removed", failed)
	}
	// Dependencies are an unordered set; the result of the first attempt is reused.
	if again, err2 := c.pythonEnv(repoDir, []string{"a", "b"}); again != failed || err2 == nil || err2.Error() != err.Error() {
		t.Errorf("pythonEnv(a, b) = %q, %v; want the cached result for (b, a)", again, err2)
	}

	t.Setenv("FAKE_VENV_FAIL", "1")
	if _, err := c.pythonEnv("", []string{"other"}); err == nil ||
		!strings.Contains(err.Error(), "creating the virtualenv failed") || !strings.Contains(err.Error(), "venv exploded") {
		t.Errorf("expected virtualenv failure with its output, got %v", err)
	}

	// The cache directory cannot be created below a regular file.
	file := filepath.Join(t.TempDir(), "file")
	writeFiles(t, filepath.Dir(file), map[string]string{"file": "x"})
	broken := &repoCache{dir: file, envs: map[string]*cachedEnv{}}
	if _, err := broken.pythonEnv("", nil); err == nil || !strings.Contains(err.Error(), "failed to create cache directory") {
		t.Errorf("expected cache directory error, got %v", err)
	}
}

func TestRunPythonHookEnvironmentFailure(t *testing.T) {
	t.Setenv("TREE_COMMIT_HOME", t.TempDir())
	t.Setenv("TREE_COMMIT_PYTHON", fakePython(t))
	t.Setenv("FAKE_PY_VERSION", "3")
	t.Setenv("FAKE_VENV_FAIL", "1")
	dir := newRepo(t, map[string]string{
		".tree-commit-config.yaml": `repos:
  - repo: local
    hooks:
      - id: black
        name: Black
        entry: black
        language: python
        additional_dependencies: [black==24.1.0]
`,
		"a.py": "x = 1\n",
	})
	runGit(t, dir, "add", ".")

	code, out := runInRepo(t, dir, &runOptions{allFiles: true})
	if code != 1 || !strings.Contains(out, "- error: creating the virtualenv failed") {
		t.Fatalf("expected the environment error to be reported, got %d\n%s", code, out)
	}
}

// commitFile commits a file in repo and returns the commit id.
func commitFile(t *testing.T, repo, name, content string) string {
	t.Helper()
	writeFiles(t, repo, map[string]string{name: content})
	runGit(t, repo, "add", name)
	runGit(t, repo, "commit", "-q", "-m", content)
	out, err := gitOutput(repo, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(out)
}

func TestCloneAtRev(t *testing.T) {
	repo := newRepo(t, nil)
	first := commitFile(t, repo, "v.txt", "one\n")
	commitFile(t, repo, "v.txt", "two\n")
	runGit(t, repo, "tag", "v2")

	file := filepath.Join(t.TempDir(), "file")
	writeFiles(t, filepath.Dir(file), map[string]string{"file": "x"})

	tests := []struct {
		name, repo, rev, dest string
		content, wantErr      string
	}{
		{name: "tag", repo: repo, rev: "v2", content: "two\n"},
		// Abbreviated ids cannot be fetched directly: all refs are fetched instead.
		{name: "abbreviated commit", repo: repo, rev: first[:10], content: "one\n"},
		{name: "unknown revision", repo: repo, rev: "no-such-rev", wantErr: `revision "no-such-rev" not found`},
		{name: "unknown repository", repo: filepath.Join(t.TempDir(), "missing"), rev: "v1", wantErr: "failed to fetch"},
		{name: "destination below a file", repo: repo, rev: "v2", dest: filepath.Join(file, "dest"), wantErr: "failed to create"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dest := tt.dest
			if dest == "" {
				dest = filepath.Join(t.TempDir(), "clone")
			}
			err := cloneAtRev(filepath.ToSlash(tt.repo), tt.rev, dest)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("cloneAtRev error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.TrimSpace(readFile(t, filepath.Join(dest, "v.txt"))); got != strings.TrimSpace(tt.content) {
				t.Errorf("v.txt = %q, want %q", got, tt.content)
			}
		})
	}
}

func TestCloneAtRevWithSubmodules(t *testing.T) {
	// Local submodules are blocked by default since git 2.38.1.
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "protocol.file.allow")
	t.Setenv("GIT_CONFIG_VALUE_0", "always")

	lib := newRepo(t, nil)
	commitFile(t, lib, "lib.txt", "lib\n")
	hooks := newRepo(t, nil)
	commitFile(t, hooks, "main.txt", "main\n")
	runGit(t, hooks, "submodule", "add", "-q", filepath.ToSlash(lib), "vendor/lib")
	runGit(t, hooks, "commit", "-q", "-m", "add submodule")

	dest := filepath.Join(t.TempDir(), "clone")
	if err := cloneAtRev(filepath.ToSlash(hooks), "HEAD", dest); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(readFile(t, filepath.Join(dest, "vendor", "lib", "lib.txt"))); got != "lib" {
		t.Errorf("submodule not checked out, lib.txt = %q", got)
	}
}

func TestCheckoutErrors(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	writeFiles(t, filepath.Dir(file), map[string]string{"file": "x"})
	if _, err := (&repoCache{dir: file}).checkout("r", "v1"); err == nil || !strings.Contains(err.Error(), "failed to create cache directory") {
		t.Errorf("expected cache directory error, got %v", err)
	}

	c := &repoCache{dir: t.TempDir()}
	missing := filepath.ToSlash(filepath.Join(t.TempDir(), "missing"))
	if _, err := c.checkout(missing, "v1"); err == nil || !strings.Contains(err.Error(), "failed to fetch") {
		t.Fatalf("expected fetch error, got %v", err)
	}
	// Neither a partial clone nor the lock file is left behind.
	entries, err := os.ReadDir(filepath.Join(c.dir, "repos"))
	if err != nil || len(entries) != 0 {
		t.Errorf("cache should be empty after a failed clone, got %v, %v", entries, err)
	}
}

func TestAcquireLock(t *testing.T) {
	dir := t.TempDir()

	stale := filepath.Join(dir, "stale.lock")
	writeFiles(t, dir, map[string]string{"stale.lock": ""})
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}
	if err := acquireLock(stale, time.Minute); err != nil {
		t.Fatalf("stale lock was not taken over: %v", err)
	}
	if info, err := os.Stat(stale); err != nil || info.ModTime().Before(time.Now().Add(-time.Minute)) {
		t.Errorf("lock should have been recreated: %v, %v", info, err)
	}

	busy := filepath.Join(dir, "busy.lock")
	writeFiles(t, dir, map[string]string{"busy.lock": ""})
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(busy, future, future); err != nil {
		t.Fatal(err)
	}
	if err := acquireLock(busy, 50*time.Millisecond); err == nil || !strings.Contains(err.Error(), "timed out waiting for lock") {
		t.Errorf("expected a timeout, got %v", err)
	}

	called := false
	err := withFileLock(filepath.Join(dir, "missing", "x.lock"), func() error { called = true; return nil })
	if err == nil || !strings.Contains(err.Error(), "failed to acquire lock") || called {
		t.Errorf("withFileLock = %v (called: %v), want lock error without running fn", err, called)
	}
}

// unsetCacheDir makes the cache directory undeterminable.
func unsetCacheDir(t *testing.T) {
	t.Helper()
	t.Setenv("TREE_COMMIT_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")
	if runtime.GOOS == windows {
		t.Setenv("LocalAppData", "")
	} else {
		t.Setenv("HOME", "")
		_ = os.Unsetenv("HOME") // git copes with a missing HOME better than with an empty one
	}
	if dir, err := os.UserCacheDir(); err == nil {
		t.Skipf("cannot unset the user cache directory (%s)", dir)
	}
}

func TestCleanCache(t *testing.T) {
	home := filepath.Join(t.TempDir(), "cache")
	writeFiles(t, home, map[string]string{"repos/x/file": "x", "envs/py-1/file": "y"})
	t.Setenv("TREE_COMMIT_HOME", home)

	dir, err := cleanCache()
	if err != nil || dir != home {
		t.Fatalf("cleanCache = %q, %v; want %q", dir, err, home)
	}
	if fileExists(home) {
		t.Error("cache directory still exists")
	}

	unsetCacheDir(t)
	for name, fn := range map[string]func() error{
		"cleanCache":   func() error { _, err := cleanCache(); return err },
		"newRepoCache": func() error { _, err := newRepoCache(); return err },
		"runHooks": func() error {
			repo := newRepo(t, map[string]string{".tree-commit-config.yaml": "repos: []\n"})
			_, err := runHooks(context.Background(), &runOptions{dir: repo, allFiles: true}, &strings.Builder{})
			return err
		},
	} {
		if err := fn(); err == nil || !strings.Contains(err.Error(), "set TREE_COMMIT_HOME") {
			t.Errorf("%s error = %v, want a hint to set TREE_COMMIT_HOME", name, err)
		}
	}
}
