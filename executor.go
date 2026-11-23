package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"
)

type hookStatus int

const (
	statusPassed hookStatus = iota
	statusFailed
	statusSkipped
)

const statusLineWidth = 79

type hookResult struct {
	status   hookStatus
	exitCode int
	modified bool
	output   []byte
	note     string
	duration time.Duration
}

// envProvider builds language environments for hooks.
type envProvider interface {
	languageEnv(language, repoDir string, deps []string) (string, error)
}

type executor struct {
	ctx     context.Context
	root    string
	envs    envProvider
	jobs    int
	out     io.Writer
	color   bool
	verbose bool
}

// runAll executes the plan and prints one status line per hook, in plan order.
// Hooks that share files run one after the other in plan order; all other hooks
// run in parallel (up to x.jobs at a time). It returns the number of failed hooks.
func (x *executor) runAll(runs []*hookRun) int {
	deps := hookDependencies(runs)
	done := make([]chan struct{}, len(runs))
	for i := range done {
		done[i] = make(chan struct{})
	}
	results := make([]*hookResult, len(runs))
	jobs := x.jobs
	if jobs < 1 {
		jobs = 1
	}
	sem := make(chan struct{}, jobs)

	for i := range runs {
		go func(i int) {
			defer close(done[i])
			for _, d := range deps[i] {
				<-done[d]
			}
			sem <- struct{}{}
			defer func() { <-sem }()
			start := time.Now()
			results[i] = x.execute(runs[i])
			results[i].duration = time.Since(start)
		}(i)
	}

	failed := 0
	for i, r := range runs {
		<-done[i]
		if results[i].status == statusFailed {
			failed++
		}
		x.report(r, results[i])
	}
	return failed
}

// footprint describes which files a hook may read or write.
type footprint struct {
	files    map[string]bool
	dir      string
	wholeDir bool // the hook works on its whole directory (pass_filenames: false)
}

func footprintOf(r *hookRun) footprint {
	fp := footprint{files: make(map[string]bool, len(r.files)), dir: r.def.scope.dir}
	if r.noFiles {
		return fp
	}
	for _, f := range r.files {
		fp.files[f] = true
	}
	fp.wholeDir = !r.def.hook.passFilenames() || (r.def.hook.AlwaysRun && len(r.files) == 0)
	return fp
}

func (a footprint) conflicts(b footprint) bool {
	if a.wholeDir && b.wholeDir {
		return isWithin(a.dir, b.dir) || isWithin(b.dir, a.dir)
	}
	if a.wholeDir || b.wholeDir {
		whole, other := a, b
		if b.wholeDir {
			whole, other = b, a
		}
		for f := range other.files {
			if isWithin(whole.dir, f) {
				return true
			}
		}
		return false
	}
	small, large := a.files, b.files
	if len(small) > len(large) {
		small, large = large, small
	}
	for f := range small {
		if large[f] {
			return true
		}
	}
	return false
}

// hookDependencies returns, for every run, the earlier runs it must wait for.
func hookDependencies(runs []*hookRun) [][]int {
	fps := make([]footprint, len(runs))
	for i, r := range runs {
		fps[i] = footprintOf(r)
	}
	deps := make([][]int, len(runs))
	for j := range runs {
		for i := 0; i < j; i++ {
			if fps[i].conflicts(fps[j]) {
				deps[j] = append(deps[j], i)
			}
		}
	}
	return deps
}

func (x *executor) execute(r *hookRun) *hookResult {
	d := r.def
	if r.noFiles {
		return &hookResult{status: statusSkipped}
	}
	if d.err != nil {
		return &hookResult{status: statusFailed, note: d.err.Error()}
	}
	if d.skipReason != "" {
		return &hookResult{status: statusSkipped, note: d.skipReason}
	}

	before := x.snapshot(r)
	var res *hookResult
	switch d.hook.Language {
	case langFail:
		res = &hookResult{status: statusFailed, exitCode: 1, output: failOutput(d.hook.Entry, x.scopeFiles(r))}
	case langPygrep:
		res = x.runPygrep(r)
	default:
		res = x.runCommand(r)
	}
	if !bytes.Equal(before, x.snapshot(r)) {
		res.modified = true
		res.status = statusFailed
	}
	return res
}

// snapshot fingerprints what a hook may modify: its files and, for hooks that
// work on their whole directory, the unstaged changes in that directory (like
// pre-commit's whole-repository `git diff` check). The dependency graph
// guarantees no other hook touches that directory meanwhile.
func (x *executor) snapshot(r *hookRun) []byte {
	sum := hashFiles(x.root, r.files)
	if !footprintOf(r).wholeDir {
		return sum
	}
	pathspec := ":/"
	if r.def.scope.dir != rootDir {
		pathspec = ":(top,literal)" + filepath.ToSlash(r.def.scope.dir)
	}
	diff, err := gitOutput(x.root, "diff", "--no-ext-diff", "--binary", "--", pathspec)
	if err != nil {
		diff = "error: " + err.Error()
	}
	h := sha256.New()
	_, _ = h.Write(sum)
	_, _ = io.WriteString(h, diff)
	return h.Sum(nil)
}

func failOutput(entry string, files []string) []byte {
	var b bytes.Buffer
	b.WriteString(entry)
	b.WriteString("\n\n")
	for _, f := range files {
		b.WriteString(f)
		b.WriteString("\n")
	}
	return b.Bytes()
}

// scopeFiles returns the hook's files relative to its config directory, which is
// also the working directory the hook runs in.
func (x *executor) scopeFiles(r *hookRun) []string {
	files := make([]string, len(r.files))
	for i, f := range r.files {
		files[i] = relToScope(r.def.scope.dir, f)
	}
	return files
}

func (x *executor) scopeDir(d *hookDef) string {
	return filepath.Join(x.root, d.scope.dir)
}

func (x *executor) runCommand(r *hookRun) *hookResult {
	d := r.def
	argv, env, err := x.command(d)
	if err != nil {
		return &hookResult{status: statusFailed, note: err.Error()}
	}
	argv = append(argv, d.hook.Args...)

	batches := [][]string{nil}
	if d.hook.passFilenames() {
		batches = partitionFiles(x.scopeFiles(r), maxCommandLength(argv[0])-commandLength(argv))
	}

	res := &hookResult{status: statusPassed}
	var output bytes.Buffer
	for _, batch := range batches {
		args := append(append([]string{}, argv[1:]...), batch...)
		cmd := exec.CommandContext(x.ctx, argv[0], args...)
		cmd.Dir = x.scopeDir(d)
		cmd.Env = env
		quoteForBatch(cmd)
		out, err := cmd.CombinedOutput()
		output.Write(out)
		if err != nil {
			res.status = statusFailed
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				res.exitCode = exitErr.ExitCode()
			} else {
				res.note = err.Error()
			}
		}
	}
	res.output = output.Bytes()
	return res
}

// command builds the executable and environment for a hook.
func (x *executor) command(d *hookDef) (argv, env []string, err error) {
	h := d.hook
	argv, err = splitCommand(h.Entry)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid entry %q: %w", h.Entry, err)
	}
	if len(argv) == 0 {
		return nil, nil, errors.New("hook entry is empty")
	}

	env = os.Environ()
	pathEnv := os.Getenv("PATH")
	switch {
	case h.Language == langScript:
		base := d.repoDir
		if base == "" {
			base = x.scopeDir(d)
		}
		argv[0] = filepath.Join(base, filepath.FromSlash(argv[0]))
	case envKinds[h.Language] != "" && (d.repoDir != "" || len(h.AdditionalDependencies) > 0):
		// Remote hooks, and local hooks with additional_dependencies, run in an
		// environment. Other local hooks use the tools on PATH.
		if x.envs == nil {
			return nil, nil, fmt.Errorf("%s environments are not available", h.Language)
		}
		envDir, envErr := x.envs.languageEnv(h.Language, d.repoDir, h.AdditionalDependencies)
		if envErr != nil {
			return nil, nil, envErr
		}
		pathEnv = envBinDir(h.Language, envDir) + string(os.PathListSeparator) + pathEnv
		env = setEnv(env, "PATH", pathEnv)
		env = envVariables(h.Language, envDir, env)
	}

	exe, err := resolveExecutable(argv[0], x.scopeDir(d), pathEnv)
	if err != nil {
		return nil, nil, err
	}
	argv[0] = exe
	return wrapShebang(argv, x.scopeDir(d), pathEnv), env, nil
}

func envKeyEqual(a, b string) bool {
	if runtime.GOOS == windows {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func unsetEnv(env []string, key string) []string {
	out := env[:0:0]
	for _, kv := range env {
		k, _, _ := strings.Cut(kv, "=")
		if !envKeyEqual(k, key) {
			out = append(out, kv)
		}
	}
	return out
}

func setEnv(env []string, key, value string) []string {
	return append(unsetEnv(env, key), key+"="+value)
}

func executableExtensions() []string {
	if runtime.GOOS != windows {
		return nil
	}
	pathext := os.Getenv("PATHEXT")
	if pathext == "" {
		pathext = ".COM;.EXE;.BAT;.CMD"
	}
	var exts []string
	for _, e := range strings.Split(strings.ToLower(pathext), ";") {
		if e != "" {
			exts = append(exts, e)
		}
	}
	return exts
}

func hasExecutableExtension(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	for _, e := range executableExtensions() {
		if ext == e {
			return true
		}
	}
	return false
}

// candidates returns the file names to try for an executable name.
func candidates(name string) []string {
	if runtime.GOOS != windows || hasExecutableExtension(name) {
		return []string{name}
	}
	exts := executableExtensions()
	names := make([]string, 0, len(exts)+1)
	for _, e := range exts {
		names = append(names, name+e)
	}
	return append(names, name) // extension-less scripts are run through their shebang
}

func isExecutableFile(path string) bool {
	info, err := os.Stat(path) //nolint:gosec // probing candidate executables
	if err != nil || info.IsDir() {
		return false
	}
	return runtime.GOOS == windows || info.Mode()&0o111 != 0
}

// resolveExecutable finds name using pathEnv (not the parent's PATH, which is what
// exec.Command would use), or relative to dir when name contains a separator.
func resolveExecutable(name, dir, pathEnv string) (string, error) {
	if strings.ContainsAny(name, `/\`) {
		p := filepath.FromSlash(name)
		if !filepath.IsAbs(p) {
			p = filepath.Join(dir, p)
		}
		for _, c := range candidates(p) {
			if isExecutableFile(c) {
				return c, nil
			}
		}
		return "", fmt.Errorf("executable %s not found", p)
	}
	for _, d := range filepath.SplitList(pathEnv) {
		if d == "" {
			continue
		}
		// The hook runs in dir, so that is where relative PATH entries point.
		if !filepath.IsAbs(d) {
			d = filepath.Join(dir, d)
		}
		for _, c := range candidates(name) {
			if full := filepath.Join(d, c); isExecutableFile(full) {
				return full, nil
			}
		}
	}
	return "", fmt.Errorf("executable %q not found on PATH", name)
}

// wrapShebang makes "#!" scripts runnable on Windows by invoking their interpreter
// (or Git's sh) explicitly, looked up in the hook's PATH so that virtualenvs are
// honored. It is a no-op on other platforms.
func wrapShebang(argv []string, dir, pathEnv string) []string {
	if runtime.GOOS != windows || hasExecutableExtension(argv[0]) {
		return argv
	}
	head, err := readHead(argv[0], 256)
	if err != nil {
		return argv
	}
	interp := shebangInterpreter(head)
	if interp == "" {
		return argv
	}
	for _, name := range []string{interp, "sh"} {
		if p, err := resolveExecutable(name, dir, pathEnv); err == nil {
			return append([]string{p, filepath.ToSlash(argv[0])}, argv[1:]...)
		}
	}
	return argv
}

// isBatchFile reports whether exe is a Windows batch file, which runs through cmd.exe.
func isBatchFile(exe string) bool {
	ext := strings.ToLower(filepath.Ext(exe))
	return runtime.GOOS == windows && (ext == ".bat" || ext == ".cmd")
}

// maxCommandLength returns the command line budget for exe. cmd.exe, which runs
// batch files such as npm's eslint.cmd, has a much lower limit than CreateProcess.
func maxCommandLength(exe string) int {
	switch {
	case isBatchFile(exe):
		const cmdExeLimit = 8191
		comspec := os.Getenv("ComSpec")
		if comspec == "" {
			comspec = "cmd.exe"
		}
		return cmdExeLimit - len(comspec) - len(" /c ") - 1024
	case runtime.GOOS == windows:
		return 30000
	default:
		return 100000
	}
}

func commandLength(argv []string) int {
	n := 0
	for _, a := range argv {
		n += len(a) + 3
	}
	return n
}

// partitionFiles splits files into batches whose command line length stays under budget.
func partitionFiles(files []string, budget int) [][]string {
	if len(files) == 0 {
		return [][]string{nil}
	}
	var batches [][]string
	var cur []string
	size := 0
	for _, f := range files {
		n := len(f) + 3
		if len(cur) > 0 && size+n > budget {
			batches = append(batches, cur)
			cur, size = nil, 0
		}
		cur = append(cur, f)
		size += n
	}
	return append(batches, cur)
}

// hashFiles returns a digest of the content of files, used to detect hooks that
// modify files.
func hashFiles(root string, files []string) []byte {
	h := sha256.New()
	for _, f := range files {
		_, _ = io.WriteString(h, f+"\x00")
		data, err := os.ReadFile(filepath.Join(root, f)) //nolint:gosec // repo files
		if err != nil {
			_, _ = io.WriteString(h, "<missing>")
			continue
		}
		sum := sha256.Sum256(data)
		_, _ = h.Write(sum[:])
	}
	return h.Sum(nil)
}

// pygrepOptions are the flags understood by the pygrep language.
type pygrepOptions struct {
	re        *regexp.Regexp
	multiline bool
	negate    bool
}

func parsePygrep(h *Hook) (*pygrepOptions, error) {
	opts := &pygrepOptions{}
	flags := ""
	for _, a := range h.Args {
		switch a {
		case "-i", "--ignore-case":
			flags += "i"
		case "--multiline":
			opts.multiline = true
			flags += "ms"
		case "--negate":
			opts.negate = true
		}
	}
	pattern := h.Entry
	if flags != "" {
		pattern = "(?" + flags + ")" + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid pygrep pattern: %w", err)
	}
	opts.re = re
	return opts, nil
}

// grep writes the matches of data to out and reports whether there was a match.
func (o *pygrepOptions) grep(out io.Writer, rel string, data []byte) bool {
	if o.multiline {
		loc := o.re.FindIndex(data)
		if loc != nil && !o.negate {
			line := bytes.Count(data[:loc[0]], []byte("\n")) + 1
			fmt.Fprintf(out, "%s:%d:%s\n", rel, line, data[loc[0]:loc[1]])
		}
		return loc != nil
	}
	if len(data) == 0 {
		return false
	}
	matched := false
	// A trailing newline ends the last line; it does not start an empty one.
	for i, line := range bytes.Split(bytes.TrimSuffix(data, []byte("\n")), []byte("\n")) {
		if o.re.Match(line) {
			matched = true
			if !o.negate {
				fmt.Fprintf(out, "%s:%d:%s\n", rel, i+1, bytes.TrimRight(line, "\r"))
			}
		}
	}
	return matched
}

// runPygrep implements pre-commit's built-in "pygrep" language: the entry is a
// regular expression and the hook fails when it matches (or, with --negate,
// when a file has no match).
func (x *executor) runPygrep(r *hookRun) *hookResult {
	opts, err := parsePygrep(r.def.hook)
	if err != nil {
		return &hookResult{status: statusFailed, note: err.Error()}
	}

	var out bytes.Buffer
	res := &hookResult{status: statusPassed}
	for _, f := range r.files {
		data, err := os.ReadFile(filepath.Join(x.root, f)) //nolint:gosec // repo files
		if err != nil {
			continue
		}
		rel := relToScope(r.def.scope.dir, f)
		matched := opts.grep(&out, rel, data)
		if opts.negate && !matched {
			fmt.Fprintln(&out, rel)
		}
		if matched != opts.negate {
			res.status, res.exitCode = statusFailed, 1
		}
	}
	res.output = out.Bytes()
	return res
}

func (x *executor) paint(s, code string) string {
	if !x.color {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

func hookLabel(d *hookDef) string {
	if d.scope.dir == rootDir {
		return d.hook.Name
	}
	return fmt.Sprintf("%s (%s)", d.hook.Name, scopeLabel(d.scope.dir))
}

// statusLine pads label with dots so that the status lines up at statusLineWidth.
func statusLine(label, status string) string {
	dots := statusLineWidth - utf8.RuneCountInString(label) - len(status)
	if dots < 3 {
		dots = 3
	}
	return label + strings.Repeat(".", dots)
}

// report prints the result of a hook in pre-commit's format.
func (x *executor) report(r *hookRun, res *hookResult) {
	var status, code string
	switch res.status {
	case statusPassed:
		status, code = "Passed", "32"
	case statusFailed:
		status, code = "Failed", "41"
	default:
		status, code = "Skipped", "30;43"
	}
	reason := ""
	if r.noFiles {
		reason = "(no files to check)"
	}
	fmt.Fprintln(x.out, statusLine(hookLabel(r.def), reason+status)+reason+x.paint(status, code))

	details := res.status != statusPassed || x.verbose || r.def.hook.Verbose
	if !details || r.noFiles {
		return
	}
	if res.status == statusSkipped {
		fmt.Fprintf(x.out, "- %s\n", res.note)
		return
	}
	fmt.Fprintf(x.out, "- hook id: %s\n", r.def.id)
	if x.verbose || r.def.hook.Verbose {
		fmt.Fprintf(x.out, "- duration: %.2fs\n", res.duration.Seconds())
	}
	if res.exitCode != 0 {
		fmt.Fprintf(x.out, "- exit code: %d\n", res.exitCode)
	}
	if res.modified {
		fmt.Fprintln(x.out, "- files were modified by this hook")
	}
	if res.note != "" {
		fmt.Fprintf(x.out, "- error: %s\n", res.note)
	}
	if out := bytes.TrimRight(res.output, "\r\n \t"); len(out) > 0 {
		fmt.Fprintf(x.out, "\n%s\n\n", out)
	}
}
