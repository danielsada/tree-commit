package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// newHookRepo creates a committed hook repository with the given files and tag v1.
func newHookRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := newRepo(t, files)
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "hooks")
	runGit(t, dir, "tag", "v1")
	return dir
}

// runRemoteHook runs every hook of hooksRepo@v1 on a.txt in a new repository.
func runRemoteHook(t *testing.T, hooksRepo string, hookIDs ...string) (code int, output string) {
	t.Helper()
	t.Setenv("TREE_COMMIT_HOME", t.TempDir())
	var hooks strings.Builder
	for _, id := range hookIDs {
		fmt.Fprintf(&hooks, "      - id: %s\n        verbose: true\n", id)
	}
	dir := newRepo(t, map[string]string{
		".tree-commit-config.yaml": fmt.Sprintf("repos:\n  - repo: %s\n    rev: v1\n    hooks:\n%s",
			filepath.ToSlash(hooksRepo), hooks.String()),
		"a.txt": "a\n",
	})
	runGit(t, dir, "add", "a.txt")
	return runInRepo(t, dir, &runOptions{})
}

func TestGolangHookRepository(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go is not installed")
	}
	hooksRepo := newHookRepo(t, map[string]string{
		".pre-commit-hooks.yaml": `- id: go-echo
  name: go echo
  entry: go-echo
  language: golang
  files: \.txt$
`,
		"go.mod": "module example.com/hooks\n\ngo 1.21\n",
		"cmd/go-echo/main.go": `package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	fmt.Println("go-echo got " + strings.Join(os.Args[1:], ","))
}
`,
	})

	code, out := runRemoteHook(t, hooksRepo, "go-echo")
	if code != 0 || !strings.Contains(out, "go-echo got a.txt") {
		t.Fatalf("expected the Go hook to run, got %d\n%s", code, out)
	}
}

func TestNodeHookRepository(t *testing.T) {
	for _, tool := range []string{"node", "npm"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not installed", tool)
		}
	}
	hooksRepo := newHookRepo(t, map[string]string{
		".pre-commit-hooks.yaml": `- id: node-echo
  name: node echo
  entry: node-echo
  language: node
  files: \.txt$
`,
		"package.json": `{"name": "node-echo", "version": "1.0.0", "bin": {"node-echo": "bin.js"}}
`,
		"bin.js": "#!/usr/bin/env node\nconsole.log('node-echo got ' + process.argv.slice(2).join(','));\n",
	})

	code, out := runRemoteHook(t, hooksRepo, "node-echo")
	if code != 0 || !strings.Contains(out, "node-echo got a.txt") {
		t.Fatalf("expected the Node.js hook to run, got %d\n%s", code, out)
	}
}

func TestLanguageEnvErrors(t *testing.T) {
	c := &repoCache{dir: t.TempDir(), manifests: map[string]*cachedManifest{}, envs: map[string]*cachedEnv{}}
	if _, err := c.languageEnv("rust", "", nil); err == nil || !strings.Contains(err.Error(), "cannot create rust environments") {
		t.Errorf("expected an error for rust, got %v", err)
	}

	t.Setenv("PATH", t.TempDir())
	for lang, want := range map[string]string{
		langNode:   "need node and npm",
		langGolang: "need go",
	} {
		if _, err := c.languageEnv(lang, "", []string{"dep"}); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s without tools: got %v, want %q", lang, err, want)
		}
	}
}

func TestEnvBinDirAndVariables(t *testing.T) {
	env := envVariables(langNode, "/envs/node", []string{"PATH=/bin"})
	joined := strings.Join(env, " ")
	if !strings.Contains(joined, "NPM_CONFIG_PREFIX=/envs/node") || !strings.Contains(joined, "NODE_VIRTUAL_ENV=/envs/node") {
		t.Errorf("node variables missing: %v", env)
	}
	if got := envBinDir(langGolang, "envs"); got != filepath.Join("envs", "bin") {
		t.Errorf("golang bin dir = %q", got)
	}
	if got := strings.Join(envVariables(langGolang, "x", []string{"A=1"}), " "); got != "A=1" {
		t.Errorf("golang should not change the environment, got %q", got)
	}
}
