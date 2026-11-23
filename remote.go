package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const readyMarker = ".tree-commit-ready"

// repoCache clones hook repositories and builds language environments under the
// tree-commit cache directory ($TREE_COMMIT_HOME, or the user cache dir).
type repoCache struct {
	dir string

	mu        sync.Mutex
	manifests map[string]*cachedManifest
	envs      map[string]*cachedEnv
	python    string
}

type cachedManifest struct {
	once  sync.Once
	dir   string
	hooks map[string]map[string]any
	err   error
}

type cachedEnv struct {
	once sync.Once
	dir  string
	err  error
}

func cacheDir() (string, error) {
	if dir := os.Getenv("TREE_COMMIT_HOME"); dir != "" {
		return dir, nil
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine cache directory (set TREE_COMMIT_HOME): %w", err)
	}
	return filepath.Join(base, "tree-commit"), nil
}

func newRepoCache() (*repoCache, error) {
	dir, err := cacheDir()
	if err != nil {
		return nil, err
	}
	return &repoCache{
		dir:       dir,
		manifests: make(map[string]*cachedManifest),
		envs:      make(map[string]*cachedEnv),
	}, nil
}

func shortHash(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])[:16]
}

// manifest implements manifestSource.
func (c *repoCache) manifest(repo, rev string) (dir string, hooks map[string]map[string]any, err error) {
	key := repo + "@" + rev
	c.mu.Lock()
	m := c.manifests[key]
	if m == nil {
		m = &cachedManifest{}
		c.manifests[key] = m
	}
	c.mu.Unlock()

	m.once.Do(func() {
		m.dir, m.err = c.checkout(repo, rev)
		if m.err == nil {
			m.hooks, m.err = loadManifest(filepath.Join(m.dir, manifestFile))
		}
	})
	return m.dir, m.hooks, m.err
}

// checkout returns a directory containing repo at rev, cloning it on first use.
func (c *repoCache) checkout(repo, rev string) (string, error) {
	reposDir := filepath.Join(c.dir, "repos")
	dest := filepath.Join(reposDir, shortHash(repo, rev))
	if fileExists(filepath.Join(dest, readyMarker)) {
		return dest, nil
	}
	if err := os.MkdirAll(reposDir, 0o750); err != nil {
		return "", fmt.Errorf("failed to create cache directory: %w", err)
	}

	err := withFileLock(dest+".lock", func() error {
		if fileExists(filepath.Join(dest, readyMarker)) {
			return nil
		}
		if err := os.RemoveAll(dest); err != nil {
			return fmt.Errorf("failed to clean %s: %w", dest, err)
		}
		fmt.Fprintf(os.Stderr, "[tree-commit] Initializing hook repository %s@%s\n", repo, rev)
		if err := cloneAtRev(repo, rev, dest); err != nil {
			_ = os.RemoveAll(dest)
			return err
		}
		return os.WriteFile(filepath.Join(dest, readyMarker), []byte(repo+"@"+rev+"\n"), 0o600)
	})
	if err != nil {
		return "", err
	}
	return dest, nil
}

// cloneAtRev fetches a single revision (shallow when the server allows it).
func cloneAtRev(repo, rev, dest string) error {
	env := noGitEnv()
	git := func(args ...string) error {
		_, err := gitOutputEnv(dest, env, args...)
		return err
	}
	if err := os.MkdirAll(dest, 0o750); err != nil {
		return fmt.Errorf("failed to create %s: %w", dest, err)
	}
	if err := git("init", "-q"); err != nil {
		return err
	}
	if err := git("remote", "add", "--end-of-options", "origin", repo); err != nil {
		return err
	}
	if err := git("fetch", "-q", "--depth", "1", "--end-of-options", "origin", rev); err == nil {
		if err := git("-c", "advice.detachedHead=false", "checkout", "-q", "FETCH_HEAD"); err != nil {
			return err
		}
	} else {
		if err := git("fetch", "-q", "--tags", "origin"); err != nil {
			return fmt.Errorf("failed to fetch %s: %w", repo, err)
		}
		if err := git("-c", "advice.detachedHead=false", "checkout", "-q", rev); err != nil {
			return fmt.Errorf("revision %q not found in %s: %w", rev, repo, err)
		}
	}
	if fileExists(filepath.Join(dest, ".gitmodules")) {
		if err := git("submodule", "update", "-q", "--init", "--recursive", "--depth", "1"); err != nil {
			return err
		}
	}
	return nil
}

// envBuilder installs a hook repository (if any) and dependencies into envDir.
type envBuilder func(envDir, repoDir string, deps []string) error

// envKinds are the cache directory prefixes of the languages with environments.
var envKinds = map[string]string{langPython: "py", langNode: "node", langGolang: "go"}

// languageEnv returns an environment for language with the hook repository (if
// any) and the additional dependencies installed.
func (c *repoCache) languageEnv(language, repoDir string, deps []string) (string, error) {
	switch language {
	case langPython:
		return c.pythonEnv(repoDir, deps)
	case langNode:
		return c.cachedEnv(language, repoDir, deps, buildNodeEnv)
	case langGolang:
		return c.cachedEnv(language, repoDir, deps, buildGoEnv)
	default:
		return "", fmt.Errorf("tree-commit cannot create %s environments", language)
	}
}

// pythonEnv returns a virtualenv with the hook repository (if any) and the
// additional dependencies installed.
func (c *repoCache) pythonEnv(repoDir string, deps []string) (string, error) {
	return c.cachedEnv(langPython, repoDir, deps, c.buildPythonEnv)
}

// cachedEnv builds an environment once per process and reuses ready
// environments from the cache directory.
func (c *repoCache) cachedEnv(language, repoDir string, deps []string, build envBuilder) (string, error) {
	sorted := append([]string(nil), deps...)
	sort.Strings(sorted)
	key := envKinds[language] + "-" + shortHash(append([]string{repoDir}, sorted...)...)

	c.mu.Lock()
	e := c.envs[key]
	if e == nil {
		e = &cachedEnv{}
		c.envs[key] = e
	}
	c.mu.Unlock()

	e.once.Do(func() {
		e.dir = filepath.Join(c.dir, "envs", key)
		e.err = buildEnv(language, e.dir, func(envDir string) error { return build(envDir, repoDir, sorted) })
	})
	return e.dir, e.err
}

// buildEnv builds envDir with build, unless another process already did.
func buildEnv(language, envDir string, build func(envDir string) error) error {
	if fileExists(filepath.Join(envDir, readyMarker)) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(envDir), 0o750); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}
	return withFileLock(envDir+".lock", func() error {
		if fileExists(filepath.Join(envDir, readyMarker)) {
			return nil
		}
		if err := os.RemoveAll(envDir); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "[tree-commit] Creating %s environment %s\n", language, filepath.Base(envDir))
		if err := build(envDir); err != nil {
			_ = os.RemoveAll(envDir)
			return err
		}
		return os.WriteFile(filepath.Join(envDir, readyMarker), nil, 0o600)
	})
}

func (c *repoCache) buildPythonEnv(envDir, repoDir string, deps []string) error {
	uv := uvExecutable()
	python, err := c.findPython()
	if err != nil && uv == "" {
		return err
	}
	if err := createVirtualenv(uv, python, envDir); err != nil {
		return err
	}
	if repoDir == "" && len(deps) == 0 {
		return nil
	}
	packages := append([]string(nil), deps...)
	if repoDir != "" {
		packages = append([]string{repoDir}, packages...)
	}
	return installPackages(uv, envDir, packages)
}

// buildNodeEnv installs the hook repository and dependencies as global npm
// packages under envDir, like pre-commit: the repository is built with
// `npm install`, packed with `npm pack` and installed from the tarball.
func buildNodeEnv(envDir, repoDir string, deps []string) error {
	npm, lookErr := exec.LookPath("npm")
	if lookErr != nil {
		return errors.New("node hooks need node and npm, which were not found on PATH")
	}
	if err := os.MkdirAll(envDir, 0o750); err != nil {
		return err
	}
	packages := append([]string(nil), deps...)
	if repoDir != "" {
		tarball, packErr := npmPack(npm, repoDir, envDir)
		if packErr != nil {
			return packErr
		}
		packages = append([]string{tarball}, packages...)
	}
	if len(packages) == 0 {
		return nil
	}
	args := append([]string{"install", "--global", "--prefix", envDir,
		"--no-audit", "--no-fund", "--no-progress", "--loglevel=error"}, packages...)
	_, err := runSetupIn("", nil, "npm install", npm, args...)
	return err
}

// npmPack builds the package in repoDir and packs it into destDir. The lock
// keeps concurrent environments for the same repository from sharing node_modules.
func npmPack(npm, repoDir, destDir string) (string, error) {
	var tarball string
	err := withFileLock(repoDir+".npm-lock", func() error {
		defer func() { _ = os.RemoveAll(filepath.Join(repoDir, "node_modules")) }()
		if _, err := runSetupIn(repoDir, nil, "npm install", npm,
			"install", "--include=dev", "--include=prod", "--no-save",
			"--no-audit", "--no-fund", "--no-progress", "--loglevel=error"); err != nil {
			return err
		}
		out, err := runSetupIn(repoDir, nil, "npm pack", npm,
			"pack", "--silent", "--pack-destination", destDir)
		if err != nil {
			return err
		}
		lines := strings.Fields(out)
		if len(lines) == 0 {
			return errors.New("npm pack did not report a package file")
		}
		tarball = filepath.Join(destDir, lines[len(lines)-1])
		return nil
	})
	return tarball, err
}

// buildGoEnv installs the commands of the hook repository and the
// dependencies (module@version) into envDir/bin with `go install`.
func buildGoEnv(envDir, repoDir string, deps []string) error {
	goExe, err := exec.LookPath("go")
	if err != nil {
		return errors.New("golang hooks need go, which was not found on PATH")
	}
	bin := filepath.Join(envDir, "bin")
	if err := os.MkdirAll(bin, 0o750); err != nil {
		return err
	}
	env := setEnv(noGitEnv(), "GOBIN", bin)
	if repoDir != "" {
		if _, err := runSetupIn(repoDir, env, "go install", goExe, "install", "./..."); err != nil {
			return err
		}
	}
	for _, dep := range deps {
		if _, err := runSetupIn(envDir, env, "go install", goExe, "install", dep); err != nil {
			return err
		}
	}
	return nil
}

// uvExecutable returns the uv executable used to build Python environments, or
// "" to use python's own venv and pip. $TREE_COMMIT_UV selects a uv binary, or
// disables uv with 0/false/no/off. uv is much faster and does not need the
// ensurepip module, which Debian and Ubuntu only ship in python3-venv.
func uvExecutable() string {
	setting := strings.TrimSpace(os.Getenv("TREE_COMMIT_UV"))
	switch strings.ToLower(setting) {
	case "0", "false", "no", "off":
		return ""
	case "":
		if p, err := exec.LookPath("uv"); err == nil {
			return p
		}
		return ""
	default:
		return setting
	}
}

// runSetup runs an environment setup command and reports its output on failure.
func runSetup(what, name string, args ...string) error {
	_, err := runSetupIn("", nil, what, name, args...)
	return err
}

// runSetupIn runs a setup command in dir (or the current directory) with env
// (or the environment without git's hook variables) and returns its stdout.
func runSetupIn(dir string, env []string, what, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(context.Background(), name, args...)
	cmd.Dir = dir
	cmd.Env = env
	if env == nil {
		cmd.Env = noGitEnv()
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s failed: %w\n%s%s", what, err, stdout.String(), stderr.String())
	}
	return stdout.String(), nil
}

// createVirtualenv creates a virtual environment with uv when available
// (python may then be "": uv finds or downloads an interpreter), otherwise with
// python's venv module, falling back to the virtualenv package.
func createVirtualenv(uv, python, envDir string) error {
	if uv != "" {
		args := []string{"venv", "--quiet"}
		if python != "" {
			args = append(args, "--python", python)
		}
		return runSetup("creating the virtualenv with uv", uv, append(args, envDir)...)
	}
	err := runSetup("creating the virtualenv", python, "-m", "venv", envDir)
	if err == nil {
		return nil
	}
	if runSetup("creating the virtualenv", python, "-m", "virtualenv", envDir) == nil {
		return nil
	}
	return fmt.Errorf("%w\nInstall uv (https://docs.astral.sh/uv/), or the venv module "+
		"(the python3-venv package on Debian and Ubuntu)", err)
}

// installPackages installs packages (paths or requirement specifiers) into a virtualenv.
func installPackages(uv, envDir string, packages []string) error {
	const install = "install"
	if uv != "" {
		args := append([]string{"pip", install, "--quiet", "--python", venvPython(envDir)}, packages...)
		return runSetup("uv pip install", uv, args...)
	}
	args := append([]string{"-m", "pip", install, "--disable-pip-version-check", "-q"}, packages...)
	return runSetup("pip install", venvPython(envDir), args...)
}

// findPython locates a working Python 3 interpreter ($TREE_COMMIT_PYTHON overrides).
func (c *repoCache) findPython() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.python != "" {
		return c.python, nil
	}

	candidates := []string{langPython + "3", langPython}
	if runtime.GOOS == windows {
		candidates = []string{langPython, "py", langPython + "3"}
	}
	if p := os.Getenv("TREE_COMMIT_PYTHON"); p != "" {
		candidates = []string{p}
	}
	for _, name := range candidates {
		path, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		// Skip stubs (e.g. the Windows Store alias) that exist but cannot run.
		out, err := exec.CommandContext(context.Background(), path, "-c", "import sys; print(sys.version_info[0])").Output() //nolint:gosec // interpreter chosen by the user
		if err == nil && bytes.Equal(bytes.TrimSpace(out), []byte("3")) {
			c.python = path
			return path, nil
		}
	}
	return "", errors.New("python 3 is required for python hooks but was not found on PATH (set TREE_COMMIT_PYTHON)")
}

func venvBinDir(envDir string) string {
	if runtime.GOOS == windows {
		return filepath.Join(envDir, "Scripts")
	}
	return filepath.Join(envDir, "bin")
}

// envBinDir returns the directory with the executables of an environment.
func envBinDir(language, envDir string) string {
	switch {
	case language == langPython:
		return venvBinDir(envDir)
	case language == langNode && runtime.GOOS == windows:
		return envDir
	default:
		return filepath.Join(envDir, "bin")
	}
}

// envVariables sets the variables that activate an environment.
func envVariables(language, envDir string, env []string) []string {
	switch language {
	case langPython:
		env = setEnv(env, "VIRTUAL_ENV", envDir)
		return unsetEnv(env, "PYTHONHOME")
	case langNode:
		env = setEnv(env, "NODE_VIRTUAL_ENV", envDir)
		return setEnv(env, "NPM_CONFIG_PREFIX", envDir)
	default:
		return env
	}
}

func venvPython(envDir string) string {
	if runtime.GOOS == windows {
		return filepath.Join(venvBinDir(envDir), "python.exe")
	}
	return filepath.Join(venvBinDir(envDir), "python")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// withFileLock serializes fn across tree-commit processes using an exclusive lock file.
func withFileLock(lockPath string, fn func() error) error {
	const staleAfter = 10 * time.Minute
	if err := acquireLock(lockPath, staleAfter); err != nil {
		return err
	}
	defer func() { _ = os.Remove(lockPath) }()
	return fn()
}

func acquireLock(lockPath string, staleAfter time.Duration) error {
	deadline := time.Now().Add(staleAfter)
	for {
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600) //nolint:gosec // lock in cache dir
		if err == nil {
			return f.Close()
		}
		if !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("failed to acquire lock %s: %w", lockPath, err)
		}
		if info, statErr := os.Stat(lockPath); statErr == nil && time.Since(info.ModTime()) > staleAfter {
			_ = os.Remove(lockPath)
			continue
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for lock %s", lockPath)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// cleanCache removes every cloned repository and environment.
func cleanCache() (string, error) {
	dir, err := cacheDir()
	if err != nil {
		return "", err
	}
	if err := os.RemoveAll(dir); err != nil {
		return "", fmt.Errorf("failed to remove %s: %w", dir, err)
	}
	return dir, nil
}
