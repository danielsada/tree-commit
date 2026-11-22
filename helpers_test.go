package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const helperArg = "__tree_commit_hook_helper__"

// TestMain lets the test binary double as a fake hook executable, so end-to-end
// tests work the same on every platform without external tools.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == helperArg {
		os.Exit(hookHelper(os.Args[2:]))
	}
	os.Exit(m.Run())
}

// hookHelper understands "@name=<n>", "@log=<path>", "@fail" and "@modify";
// every other argument is a file name.
func hookHelper(args []string) int {
	var name, logPath string
	var fail, modify bool
	var files []string
	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "@name="):
			name = strings.TrimPrefix(a, "@name=")
		case strings.HasPrefix(a, "@log="):
			logPath = strings.TrimPrefix(a, "@log=")
		case a == "@fail":
			fail = true
		case a == "@modify":
			modify = true
		default:
			files = append(files, a)
		}
	}
	cwd, _ := os.Getwd()
	if logPath != "" {
		f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return 3
		}
		fmt.Fprintf(f, "%s|%s|%s\n", name, filepath.Base(cwd), strings.Join(files, ","))
		_ = f.Close()
	}
	if modify {
		for _, file := range files {
			data, _ := os.ReadFile(file)
			_ = os.WriteFile(file, append(data, []byte("modified\n")...), 0o600)
		}
	}
	if fail {
		fmt.Println("boom: something is wrong")
		return 1
	}
	return 0
}

// helperEntry returns a hook entry that runs the fake hook.
func helperEntry(t *testing.T, opts ...string) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	parts := []string{quote(filepath.ToSlash(exe)), helperArg}
	for _, o := range opts {
		parts = append(parts, quote(o))
	}
	return strings.Join(parts, " ")
}

func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(noGitEnv(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

// newRepo creates a git repository with the given files (slash-separated paths).
func newRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "commit.gpgsign", "false")
	runGit(t, dir, "config", "core.autocrlf", "false")
	writeFiles(t, dir, files)
	return dir
}

func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func runInRepo(t *testing.T, dir string, opts *runOptions) (code int, output string) {
	t.Helper()
	opts.dir = dir
	if opts.stage == "" {
		opts.stage = "pre-commit"
	}
	opts.jobs = 4
	opts.color = "never"
	var out bytes.Buffer
	code, err := runHooks(context.Background(), opts, &out)
	if err != nil {
		t.Fatalf("runHooks failed: %v\noutput:\n%s", err, out.String())
	}
	return code, out.String()
}

// readLog returns the fake hook log lines, sorted.
func readLog(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	sort.Strings(lines)
	return lines
}

func assertLines(t *testing.T, got, want []string) {
	t.Helper()
	sort.Strings(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("unexpected hook invocations\n got: %q\nwant: %q", got, want)
	}
}
