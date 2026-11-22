package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// gitOutput runs git in dir and returns its stdout.
func gitOutput(dir string, args ...string) (string, error) {
	return gitOutputEnv(dir, nil, args...)
}

func gitOutputEnv(dir string, env []string, args ...string) (string, error) {
	cmd := exec.CommandContext(context.Background(), "git", args...)
	cmd.Dir = dir
	cmd.Env = env
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}

// repoRoot returns the absolute path of the top level of the work tree containing dir.
func repoRoot(dir string) (string, error) {
	out, err := gitOutput(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("not in a git repository: %w", err)
	}
	return filepath.FromSlash(strings.TrimSpace(out)), nil
}

// stagedFiles returns the added, copied, modified and renamed files in the index,
// as slash-separated paths relative to the repository root.
func stagedFiles(root string) ([]string, error) {
	out, err := gitOutput(root, "diff", "--cached", "--name-only", "--diff-filter=ACMR", "--no-ext-diff", "-z")
	if err != nil {
		return nil, err
	}
	return splitNull(out), nil
}

// trackedFiles returns every file tracked by git, relative to the repository root.
func trackedFiles(root string) ([]string, error) {
	out, err := gitOutput(root, "ls-files", "-z")
	if err != nil {
		return nil, err
	}
	return splitNull(out), nil
}

func splitNull(s string) []string {
	parts := strings.Split(s, "\x00")
	files := parts[:0]
	for _, p := range parts {
		if p != "" {
			files = append(files, p)
		}
	}
	return files
}

// gitEnvAllowlist lists GIT_* variables that are safe to pass to git when
// operating on a repository other than the one running the hook.
var gitEnvAllowlist = map[string]bool{
	"GIT_EXEC_PATH":             true,
	"GIT_SSH":                   true,
	"GIT_SSH_COMMAND":           true,
	"GIT_SSL_CAINFO":            true,
	"GIT_SSL_NO_VERIFY":         true,
	"GIT_CONFIG_COUNT":          true,
	"GIT_HTTP_PROXY_AUTHMETHOD": true,
	"GIT_ALLOW_PROTOCOL":        true,
	"GIT_ASKPASS":               true,
}

// noGitEnv returns the current environment without the GIT_* variables that git
// sets while running hooks (GIT_DIR, GIT_INDEX_FILE, ...). Without this, cloning a
// hook repository from inside a pre-commit hook would operate on the wrong repo.
func noGitEnv() []string {
	env := os.Environ()
	filtered := make([]string, 0, len(env))
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		if !strings.HasPrefix(strings.ToUpper(name), "GIT_") ||
			gitEnvAllowlist[name] ||
			strings.HasPrefix(name, "GIT_CONFIG_KEY_") ||
			strings.HasPrefix(name, "GIT_CONFIG_VALUE_") {
			filtered = append(filtered, kv)
		}
	}
	return append(filtered, "GIT_TERMINAL_PROMPT=0")
}
