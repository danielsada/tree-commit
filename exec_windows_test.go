//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBatchHooksOnWindows(t *testing.T) {
	files := map[string]string{
		// Would run "mkdir pwned" if passed to cmd.exe unquoted.
		"x&mkdir pwned": "x\n",
	}
	// Enough file names to exceed cmd.exe's 8191 character limit in one batch.
	for i := 0; i < 300; i++ {
		files[fmt.Sprintf("files/a-rather-long-file-name-number-%04d.txt", i)] = "x\n"
	}
	dir := newRepo(t, files)
	hook := filepath.Join(dir, "tools", "check.cmd")
	writeFiles(t, dir, map[string]string{
		"tools/check.cmd": "@exit /b 0\r\n",
		".tree-commit-config.yaml": fmt.Sprintf(`repos:
  - repo: local
    hooks:
      - id: batch
        entry: %q
        files: (\.txt|pwned)$
`, filepath.ToSlash(hook)),
	})
	runGit(t, dir, "add", ".")

	code, out := runInRepo(t, dir, &runOptions{allFiles: true})
	if code != 0 {
		t.Fatalf("batch hook failed: %d\n%s", code, out)
	}
	if fileExists(filepath.Join(dir, "pwned")) {
		t.Fatal("file name was interpreted by cmd.exe")
	}
}

func TestWrapShebangUsesHookPath(t *testing.T) {
	dir := t.TempDir()
	venv := filepath.Join(dir, "venv")
	interp := filepath.Join(venv, "fakepython.exe")
	script := filepath.Join(dir, "check")
	writeFiles(t, dir, map[string]string{
		"venv/fakepython.exe": "",
		"check":               "#!/usr/bin/env fakepython\nprint('hi')\n",
	})

	argv := wrapShebang([]string{script, "a.py"}, dir, venv+string(os.PathListSeparator)+os.Getenv("PATH"))
	if len(argv) != 3 || !strings.EqualFold(argv[0], interp) {
		t.Fatalf("wrapShebang = %q, want interpreter %q", argv, interp)
	}
}
