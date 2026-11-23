package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// scope is a discovered config file together with the directory it governs.
type scope struct {
	dir    string // directory relative to the repo root (rootDir for the root)
	path   string // config file path relative to the repo root
	depth  int
	config *Config

	filesRe   *regexp.Regexp
	excludeRe *regexp.Regexp
	defs      []*hookDef
	resolved  bool
}

// hookDef is one hook entry of one config file, resolved against its manifest.
type hookDef struct {
	id      string
	scope   *scope
	repo    string
	rev     string
	index   int // definition order within the config file
	hook    *Hook
	repoDir string // checkout of the hook repository (remote hooks only)

	filesRe    *regexp.Regexp
	excludeRe  *regexp.Regexp
	err        error  // resolution error, reported as a failed hook
	skipReason string // set when the hook cannot be run by tree-commit
	won        bool   // the hook won closest-wins for at least one file
}

// hookRun is a hook that will be executed, with the files it applies to.
type hookRun struct {
	def   *hookDef
	files []string // paths relative to the repo root
}

// manifestSource provides hook manifests for remote repositories.
type manifestSource interface {
	manifest(repo, rev string) (dir string, hooks map[string]map[string]any, err error)
}

type planOptions struct {
	stage  string          // git hook stage, e.g. "pre-commit"
	hookID string          // run only this hook id when set
	skip   map[string]bool // hook ids to skip (SKIP env var)
}

type resolver struct {
	root    string
	scopes  map[string]*scope
	source  manifestSource
	opts    planOptions
	tagMemo map[string]map[string]bool
}

// supportedRemoteLanguages are the languages tree-commit can install and run for
// hooks that come from a remote repository manifest.
var supportedRemoteLanguages = map[string]bool{
	langSystem: true, langScript: true, langPython: true, langFail: true, langPygrep: true,
}

// scopesFromTree converts a discovered config tree into scopes.
func scopesFromTree(tree *ConfigTree) ([]*scope, error) {
	nodes := tree.FlattenConfigs()
	scopes := make([]*scope, 0, len(nodes))
	for _, n := range nodes {
		p := filepath.Clean(n.Path)
		dir := filepath.Dir(p)
		sc := &scope{dir: dir, path: p, depth: pathDepth(dir), config: n.Config}
		var err error
		if sc.filesRe, err = compilePattern(n.Config.Files); err != nil {
			return nil, fmt.Errorf("%s: invalid top-level 'files': %w", p, err)
		}
		if sc.excludeRe, err = compilePattern(n.Config.Exclude); err != nil {
			return nil, fmt.Errorf("%s: invalid top-level 'exclude': %w", p, err)
		}
		scopes = append(scopes, sc)
	}
	sort.Slice(scopes, func(i, j int) bool { return scopes[i].path < scopes[j].path })
	return scopes, nil
}

// buildPlan decides which hooks run on which files:
//   - a config only applies to files below its directory (scope-based execution),
//   - patterns are matched against paths relative to the config's directory,
//   - when several ancestor configs define the same hook id, the closest one wins.
func buildPlan(root string, scopes []*scope, files []string, source manifestSource, opts planOptions) []*hookRun {
	r := &resolver{
		root:    root,
		scopes:  make(map[string]*scope, len(scopes)),
		source:  source,
		opts:    opts,
		tagMemo: make(map[string]map[string]bool),
	}
	for _, sc := range scopes {
		r.scopes[sc.dir] = sc
	}

	runs := make(map[*hookDef]*hookRun)
	inScope := make(map[*scope]bool)
	for _, f := range files {
		for _, sc := range r.ancestors(f) {
			inScope[sc] = true
		}
		for _, d := range r.winningHooks(f) {
			if d.err != nil || r.matches(d, f) {
				if runs[d] == nil {
					runs[d] = &hookRun{def: d}
				}
				runs[d].files = append(runs[d].files, f)
			}
		}
	}

	addAlwaysRunHooks(inScope, runs)

	plan := make([]*hookRun, 0, len(runs))
	for _, run := range runs {
		sort.Strings(run.files)
		plan = append(plan, run)
	}
	sortRuns(plan)
	return plan
}

// addAlwaysRunHooks adds always_run hooks of in-scope configs, even without matching files.
func addAlwaysRunHooks(inScope map[*scope]bool, runs map[*hookDef]*hookRun) {
	for sc := range inScope {
		for _, d := range sc.defs {
			if d.won && d.hook.AlwaysRun && runs[d] == nil {
				runs[d] = &hookRun{def: d}
			}
		}
	}
}

// winningHooks returns the active hooks that may apply to file after closest-wins:
// configs are visited from the closest to the farthest, and once a config
// defines a hook id, that id is ignored in every config further up. The same id
// may appear several times in a single config (e.g. with different args).
func (r *resolver) winningHooks(file string) []*hookDef {
	var winners []*hookDef
	claimed := make(map[string]bool)
	for _, sc := range r.ancestors(file) {
		defs := r.defsFor(sc)
		for _, d := range defs {
			if !claimed[d.id] && r.active(d) {
				d.won = true
				winners = append(winners, d)
			}
		}
		for _, d := range defs {
			claimed[d.id] = true
		}
	}
	return winners
}

// sortRuns orders hooks the way they execute when they touch the same files:
// deeper configs first (closest to the code), then config path, then definition order.
func sortRuns(runs []*hookRun) {
	sort.SliceStable(runs, func(i, j int) bool {
		a, b := runs[i].def, runs[j].def
		if a.scope.depth != b.scope.depth {
			return a.scope.depth > b.scope.depth
		}
		if a.scope.path != b.scope.path {
			return a.scope.path < b.scope.path
		}
		return a.index < b.index
	})
}

// ancestors returns the scopes whose directory contains file, deepest first.
func (r *resolver) ancestors(file string) []*scope {
	var chain []*scope
	for _, dir := range ancestorDirs(file) {
		if sc := r.scopes[dir]; sc != nil {
			chain = append(chain, sc)
		}
	}
	return chain
}

func (r *resolver) active(d *hookDef) bool {
	if d.hook.Disable || r.opts.skip[d.id] {
		return false
	}
	if r.opts.hookID != "" && r.opts.hookID != d.id {
		return false
	}
	return d.hook.runsInStage(r.opts.stage, d.scope.config.DefaultStages)
}

// defsFor resolves (once) every hook defined in a scope's config file.
func (r *resolver) defsFor(sc *scope) []*hookDef {
	if sc.resolved {
		return sc.defs
	}
	sc.resolved = true
	index := 0
	for i := range sc.config.Repos {
		repo := &sc.config.Repos[i]
		for _, raw := range repo.Hooks {
			d := r.resolveHook(sc, repo, raw)
			d.index = index
			index++
			sc.defs = append(sc.defs, d)
		}
	}
	return sc.defs
}

func (r *resolver) resolveHook(sc *scope, repo *RepoConfig, raw map[string]any) *hookDef {
	id, _ := raw["id"].(string)
	d := &hookDef{id: id, scope: sc, repo: repo.Repo, rev: repo.Rev}
	fallback := func(err error) *hookDef {
		name, _ := raw["name"].(string)
		if name == "" {
			name = id
		}
		d.hook = &Hook{ID: id, Name: name}
		d.err = err
		return d
	}

	var err error
	switch {
	case repo.Repo == repoLocal:
		d.hook, err = decodeLocalHook(raw)
	case repo.Repo == repoMeta:
		d.hook, err = decodeHook(raw)
		d.skipReason = "meta hooks are not supported"
	case raw["disable"] == true:
		d.hook, err = decodeHook(raw)
	default:
		d.hook, err = r.resolveRemoteHook(d, raw)
	}
	if err != nil {
		return fallback(fmt.Errorf("%s: hook %q: %w", sc.path, id, err))
	}
	if d.hook.Name == "" {
		d.hook.Name = id
	}
	// Like pre-commit, hooks only receive regular files (not symlinks) by default.
	if len(d.hook.Types) == 0 {
		d.hook.Types = []string{"file"}
	}

	if d.filesRe, err = compilePattern(d.hook.Files); err != nil {
		return fallback(fmt.Errorf("%s: hook %q: invalid 'files' pattern: %w", sc.path, id, err))
	}
	if d.excludeRe, err = compilePattern(d.hook.Exclude); err != nil {
		return fallback(fmt.Errorf("%s: hook %q: invalid 'exclude' pattern: %w", sc.path, id, err))
	}
	return d
}

func (r *resolver) resolveRemoteHook(d *hookDef, raw map[string]any) (*Hook, error) {
	if r.source == nil {
		return nil, fmt.Errorf("cannot fetch %s: remote repositories are not available", d.repo)
	}
	dir, manifest, err := r.source.manifest(d.repo, d.rev)
	if err != nil {
		return nil, err
	}
	base, ok := manifest[d.id]
	if !ok {
		return nil, fmt.Errorf("hook id not found in %s at %s", d.repo, d.rev)
	}
	hook, err := decodeHook(mergeHook(base, raw))
	if err != nil {
		return nil, err
	}
	if hook.Entry == "" || hook.Language == "" {
		return nil, fmt.Errorf("manifest of %s is missing 'entry' or 'language'", d.repo)
	}
	if !supportedRemoteLanguages[hook.Language] {
		d.skipReason = fmt.Sprintf("language %q is not supported yet", hook.Language)
	}
	d.repoDir = dir
	return hook, nil
}

// matches reports whether a hook applies to a file (a repo-relative path).
func (r *resolver) matches(d *hookDef, file string) bool {
	rel := relToScope(d.scope.dir, file)
	sc, h := d.scope, d.hook

	if !folderIncluded(rel, h.IncludeFolders) || folderExcluded(rel, h.ExcludeFolders) {
		return false
	}
	// Patterns are written with forward slashes, like in pre-commit.
	pattern := filepath.ToSlash(rel)
	if sc.filesRe != nil && !sc.filesRe.MatchString(pattern) {
		return false
	}
	if sc.excludeRe != nil && sc.excludeRe.MatchString(pattern) {
		return false
	}
	if d.filesRe != nil && !d.filesRe.MatchString(pattern) {
		return false
	}
	if d.excludeRe != nil && d.excludeRe.MatchString(pattern) {
		return false
	}
	tags := r.tags(file)
	return tags != nil && matchesTypes(tags, h.Types, h.TypesOr, h.ExcludeTypes)
}

func (r *resolver) tags(file string) map[string]bool {
	if tags, ok := r.tagMemo[file]; ok {
		return tags
	}
	tags, err := fileTags(filepath.Join(r.root, file))
	if err != nil {
		tags = nil
	}
	r.tagMemo[file] = tags
	return tags
}

// normalizeFolder converts a folder from a config file ("src/", "./a/b", "/build")
// into a clean relative path, rootDir meaning "everything".
func normalizeFolder(folder string) string {
	f := filepath.FromSlash(strings.TrimSpace(folder))
	f = strings.TrimLeft(f, string(filepath.Separator))
	return filepath.Clean(rootDir + string(filepath.Separator) + f)
}

// folderIncluded reports whether rel lives in one of the include folders.
// An empty list, or ".", includes everything.
func folderIncluded(rel string, folders []string) bool {
	if len(folders) == 0 {
		return true
	}
	for _, folder := range folders {
		if isWithin(normalizeFolder(folder), filepath.Dir(rel)) {
			return true
		}
	}
	return false
}

// folderExcluded reports whether rel is inside an exclude folder, at any depth:
// "build/" excludes build/x and also nested a/build/x.
func folderExcluded(rel string, folders []string) bool {
	for _, folder := range folders {
		f := normalizeFolder(folder)
		if f == rootDir {
			continue
		}
		for _, dir := range ancestorDirs(rel) {
			if hasPathSuffix(dir, f) {
				return true
			}
		}
	}
	return false
}

// compilePattern compiles a pre-commit style regular expression. Patterns are
// matched with search semantics (unanchored), like Python's re.search. Python's
// verbose flag "(?x)" is not supported by RE2, so it is expanded here.
func compilePattern(pattern string) (*regexp.Regexp, error) {
	if pattern == "" {
		return nil, nil
	}
	trimmed := strings.TrimSpace(pattern)
	if strings.HasPrefix(trimmed, "(?x)") {
		pattern = stripVerbose(strings.TrimPrefix(trimmed, "(?x)"))
	}
	return regexp.Compile(pattern)
}

// stripVerbose removes unescaped whitespace and # comments outside character classes.
func stripVerbose(p string) string {
	var b strings.Builder
	inClass, inComment, escaped := false, false, false
	for _, r := range p {
		switch {
		case inComment:
			if r == '\n' {
				inComment = false
			}
		case escaped:
			b.WriteRune(r)
			escaped = false
		case r == '\\':
			b.WriteRune(r)
			escaped = true
		case inClass:
			b.WriteRune(r)
			if r == ']' {
				inClass = false
			}
		case r == '[':
			b.WriteRune(r)
			inClass = true
		case r == '#':
			inComment = true
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
