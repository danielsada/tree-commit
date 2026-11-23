package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSetAndUnsetEnv(t *testing.T) {
	env := []string{"A=1", "PATH=/bin", "B=2=3", "A=dup"}

	got := unsetEnv(env, "A")
	if strings.Join(got, ";") != "PATH=/bin;B=2=3" {
		t.Errorf("unsetEnv = %q", got)
	}
	if env[0] != "A=1" {
		t.Error("unsetEnv must not modify its input")
	}

	got = setEnv(env, "B", "new")
	if strings.Join(got, ";") != "A=1;PATH=/bin;A=dup;B=new" {
		t.Errorf("setEnv = %q", got)
	}

	// Environment variable names are case-insensitive on Windows only.
	got = setEnv([]string{"Path=x"}, "PATH", "y")
	want := "Path=x;PATH=y"
	if runtime.GOOS == windows {
		want = "PATH=y"
	}
	if strings.Join(got, ";") != want {
		t.Errorf("setEnv(Path) = %q, want %q", got, want)
	}
}

// fakeEnvs creates python environments without python.
type fakeEnvs struct {
	dir   string
	err   error
	calls []string
}

func (f *fakeEnvs) pythonEnv(repoDir string, deps []string) (string, error) {
	f.calls = append(f.calls, repoDir+"|"+strings.Join(deps, ","))
	return f.dir, f.err
}

// writeExecutable creates an empty executable named name in dir and returns its path.
func writeExecutable(t *testing.T, dir, name string) string {
	t.Helper()
	if runtime.GOOS == windows {
		name += ".exe"
	}
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, nil, 0o700); err != nil { //nolint:gosec // test executable
		t.Fatal(err)
	}
	return p
}

func TestExecutorCommandPythonEnvironment(t *testing.T) {
	envDir := t.TempDir()
	tool := writeExecutable(t, venvBinDir(envDir), "venv-tool")
	t.Setenv("PYTHONHOME", "/should/be/removed")

	envs := &fakeEnvs{dir: envDir}
	root := t.TempDir()
	x := &executor{root: root, envs: envs}
	d := &hookDef{
		scope:   &scope{dir: rootDir},
		repoDir: "/cache/repo",
		hook:    &Hook{Entry: "venv-tool --flag", Language: langPython, AdditionalDependencies: []string{"b", "a"}},
	}
	argv, env, err := x.command(d)
	if err != nil {
		t.Fatal(err)
	}
	if len(argv) != 2 || argv[0] != tool || argv[1] != "--flag" {
		t.Errorf("argv = %q, want [%s --flag]", argv, tool)
	}
	if len(envs.calls) != 1 || envs.calls[0] != "/cache/repo|b,a" {
		t.Errorf("pythonEnv calls = %q", envs.calls)
	}

	vars := map[string]string{}
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		if k == "" {
			continue // Windows per-drive working directories ("=C:=C:\x")
		}
		if runtime.GOOS == windows {
			k = strings.ToUpper(k)
		}
		if _, dup := vars[k]; dup {
			t.Errorf("duplicate variable %s in hook environment", k)
		}
		vars[k] = v
	}
	if vars["VIRTUAL_ENV"] != envDir {
		t.Errorf("VIRTUAL_ENV = %q, want %q", vars["VIRTUAL_ENV"], envDir)
	}
	if !strings.HasPrefix(vars["PATH"], venvBinDir(envDir)+string(os.PathListSeparator)) {
		t.Errorf("PATH should start with the virtualenv: %q", vars["PATH"])
	}
	if _, ok := vars["PYTHONHOME"]; ok {
		t.Error("PYTHONHOME must be removed from the hook environment")
	}
}

func TestExecutorCommandErrors(t *testing.T) {
	root := t.TempDir()
	python := func(entry string) *Hook {
		return &Hook{Entry: entry, Language: langPython, AdditionalDependencies: []string{"dep"}}
	}
	tests := []struct {
		name string
		envs envProvider
		hook *Hook
		want string
	}{
		{"unterminated quote", nil, &Hook{Entry: `tool "arg`, Language: langSystem}, "invalid entry"},
		{"empty entry", nil, &Hook{Entry: "   ", Language: langSystem}, "hook entry is empty"},
		{"unknown executable", nil, &Hook{Entry: "tree-commit-no-such-tool", Language: langSystem}, `"tree-commit-no-such-tool" not found on PATH`},
		{"missing script", nil, &Hook{Entry: "scripts/check.sh", Language: langScript}, "not found"},
		{"python without environments", nil, python("tool"), "python environments are not available"},
		{"python environment failure", &fakeEnvs{err: errors.New("pip exploded")}, python("tool"), "pip exploded"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := &executor{root: root, envs: tt.envs}
			_, _, err := x.command(&hookDef{scope: &scope{dir: rootDir}, hook: tt.hook})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("command error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestExecutorCommandScriptRelativeToRepository(t *testing.T) {
	repoDir := t.TempDir()
	script := writeExecutable(t, filepath.Join(repoDir, "hooks"), "check")
	x := &executor{root: t.TempDir()}
	d := &hookDef{scope: &scope{dir: "sub"}, repoDir: repoDir, hook: &Hook{Entry: "hooks/check", Language: langScript}}
	argv, _, err := x.command(d)
	if err != nil || argv[0] != script {
		t.Fatalf("command = %q, %v; want script %s", argv, err, script)
	}
}

func TestPygrepGrep(t *testing.T) {
	data := []byte("first line\nfoo\r\nbar baz\nlast\n")
	tests := []struct {
		name    string
		args    []string
		entry   string
		matched bool
		out     string
	}{
		{"single line", nil, "ba", true, "f:3:bar baz\n"},
		{"CR is part of the line when matching", nil, "^foo$", false, ""},
		{"ignore case", []string{"--ignore-case"}, "FOO", true, "f:2:foo\n"},
		{"negate prints nothing", []string{"--negate"}, "foo", true, ""},
		{"multiline", []string{"--multiline"}, `foo\r\nbar`, true, "f:2:foo\r\nbar\n"},
		{"multiline dot matches newline", []string{"--multiline"}, `line.foo`, true, "f:1:line\nfoo\n"},
		{"multiline no match", []string{"--multiline"}, `nope`, false, ""},
		{"multiline negate", []string{"--multiline", "--negate"}, `last$`, true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := parsePygrep(&Hook{Entry: tt.entry, Args: tt.args})
			if err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			if got := opts.grep(&out, "f", data); got != tt.matched {
				t.Errorf("grep matched = %v, want %v", got, tt.matched)
			}
			if out.String() != tt.out {
				t.Errorf("grep output = %q, want %q", out.String(), tt.out)
			}
		})
	}

	if _, err := parsePygrep(&Hook{Entry: "("}); err == nil || !strings.Contains(err.Error(), "invalid pygrep pattern") {
		t.Errorf("expected invalid pattern error, got %v", err)
	}
}

func TestRunPygrepNegateAndUnreadableFiles(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"sub/has.txt":     "license: MIT\n",
		"sub/missing.txt": "nothing here\n",
	})
	x := &executor{root: root}
	run := func(entry string, args ...string) *hookResult {
		return x.runPygrep(&hookRun{
			def: &hookDef{scope: &scope{dir: "sub"}, hook: &Hook{Entry: entry, Language: langPygrep, Args: args}},
			files: []string{
				filepath.Join("sub", "has.txt"), filepath.Join("sub", "missing.txt"),
				filepath.Join("sub", "deleted.txt"), // unreadable files are ignored
			},
		})
	}

	res := run("license", "--negate")
	if res.status != statusFailed || res.exitCode != 1 || string(res.output) != "missing.txt\n" {
		t.Errorf("negate result = %+v (output %q)", res, res.output)
	}
	res = run("(")
	if res.status != statusFailed || !strings.Contains(res.note, "invalid pygrep pattern") {
		t.Errorf("invalid pattern result = %+v", res)
	}
}

func TestReportAndPaint(t *testing.T) {
	var out bytes.Buffer
	x := &executor{out: &out, color: true}
	d := &hookDef{id: "a-hook", scope: &scope{dir: rootDir}, hook: &Hook{Name: strings.Repeat("n", 100)}}
	x.report(&hookRun{def: d}, &hookResult{status: statusFailed, note: "could not start", output: []byte("details\n")})

	got := out.String()
	for _, want := range []string{
		strings.Repeat("n", 100) + "...\x1b[41mFailed\x1b[0m\n", // long names still get 3 dots
		"- hook id: a-hook\n",
		"- error: could not start\n",
		"\ndetails\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("report output missing %q:\n%q", want, got)
		}
	}

	out.Reset()
	x.color = false
	x.report(&hookRun{def: d}, &hookResult{status: statusSkipped, note: "meta hooks are not supported"})
	if got := out.String(); !strings.HasSuffix(got, "...Skipped\n- meta hooks are not supported\n") || strings.Contains(got, "\x1b[") {
		t.Errorf("unexpected skipped report:\n%q", got)
	}
}

func TestRunCommandStartFailure(t *testing.T) {
	root := t.TempDir()
	exe := writeExecutable(t, root, "not-really-executable")
	if err := os.WriteFile(exe, []byte("\x00\x01garbage"), 0o700); err != nil { //nolint:gosec // test file
		t.Fatal(err)
	}
	x := &executor{ctx: context.Background(), root: root}
	res := x.runCommand(&hookRun{def: &hookDef{
		scope: &scope{dir: rootDir},
		hook:  &Hook{Entry: filepath.ToSlash(exe), Language: langSystem},
	}})
	if res.status != statusFailed || res.note == "" || res.exitCode != 0 {
		t.Errorf("expected a start failure with a note, got %+v", res)
	}
}

func TestMaxCommandLength(t *testing.T) {
	budget := maxCommandLength(filepath.Join("bin", "tool"))
	if budget < 30000 {
		t.Errorf("maxCommandLength(tool) = %d, want at least 30000", budget)
	}
	if runtime.GOOS != windows {
		if got := maxCommandLength("tool.cmd"); got != budget {
			t.Errorf("batch files are not special outside Windows: %d != %d", got, budget)
		}
	}
}
