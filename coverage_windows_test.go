//go:build windows

package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBatchArg(t *testing.T) {
	tests := map[string]string{
		"plain":       "plain",
		"":            `""`,
		"a b":         `"a b"`,
		"a&calc.js":   `"a&calc.js"`,
		"100%":        `"100%"`,
		`dir&x\`:      `"dir&x\\"`,
		`C:\a\b.txt`:  `C:\a\b.txt`,
		`say "hi"&go`: `"say \"hi\"&go"`,
	}
	for in, want := range tests {
		if got := batchArg(in); got != want {
			t.Errorf("batchArg(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestMaxCommandLengthForBatchFiles(t *testing.T) {
	t.Setenv("ComSpec", "")
	short := maxCommandLength(`C:\tools\eslint.CMD`)
	t.Setenv("ComSpec", `C:\Windows\system32\cmd.exe`)
	long := maxCommandLength(`C:\tools\eslint.bat`)
	if short >= 8191 || long >= short {
		t.Errorf("batch budgets = %d (default ComSpec), %d (long ComSpec); want below cmd.exe's limit and shrinking with ComSpec", short, long)
	}
	if exe := maxCommandLength(`C:\tools\eslint.exe`); exe <= short {
		t.Errorf("executables should get a larger budget than batch files: %d <= %d", exe, short)
	}
}

func TestWrapShebangFallbacks(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"plain":      "echo hi\n",
		"needs-ruby": "#!/usr/bin/env ruby\nputs 1\n",
	})
	sh := writeExecutable(t, filepath.Join(dir, "shbin"), "sh")
	empty := t.TempDir()

	tests := []struct {
		name    string
		argv    []string
		pathEnv string
		want    []string
	}{
		{"executable extension", []string{filepath.Join(dir, "tool.exe"), "a"}, empty, []string{filepath.Join(dir, "tool.exe"), "a"}},
		{"unreadable file", []string{filepath.Join(dir, "missing"), "a"}, empty, []string{filepath.Join(dir, "missing"), "a"}},
		{"no shebang", []string{filepath.Join(dir, "plain")}, empty, []string{filepath.Join(dir, "plain")}},
		{"no interpreter available", []string{filepath.Join(dir, "needs-ruby")}, empty, []string{filepath.Join(dir, "needs-ruby")}},
		{"falls back to sh", []string{filepath.Join(dir, "needs-ruby"), "a"}, filepath.Join(dir, "shbin"),
			[]string{sh, filepath.ToSlash(filepath.Join(dir, "needs-ruby")), "a"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wrapShebang(tt.argv, dir, tt.pathEnv)
			if !strings.EqualFold(strings.Join(got, "|"), strings.Join(tt.want, "|")) {
				t.Errorf("wrapShebang = %q, want %q", got, tt.want)
			}
		})
	}
}
