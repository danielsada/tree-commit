package main

import (
	"path/filepath"
	"strings"
)

// Paths inside tree-commit are OS-native file paths relative to the repository
// root (or to a config's directory), manipulated with path/filepath. Git always
// prints forward slashes, so its output is converted at the boundary
// (gitPaths), and regular expressions from config files, which are written with
// forward slashes like in pre-commit, are matched against filepath.ToSlash(p).

// rootDir is the relative path of the repository root itself.
const rootDir = "."

// gitPaths converts NUL-separated paths printed by git into native paths.
func gitPaths(out string) []string {
	files := splitNull(out)
	for i, f := range files {
		files[i] = filepath.FromSlash(f)
	}
	return files
}

// isWithin reports whether p is dir itself or below dir. Both paths must be
// relative (or both absolute).
func isWithin(dir, p string) bool {
	rel, err := filepath.Rel(dir, p)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// ancestorDirs returns the directories containing the relative path p, from
// the closest one up to rootDir.
func ancestorDirs(p string) []string {
	var dirs []string
	for d := filepath.Dir(p); ; d = filepath.Dir(d) {
		dirs = append(dirs, d)
		if d == rootDir || d == filepath.Dir(d) {
			return dirs
		}
	}
}

// pathDepth returns the number of components of a relative directory (0 for rootDir).
func pathDepth(dir string) int {
	return len(ancestorDirs(filepath.Join(dir, "x"))) - 1
}

// hasPathSuffix reports whether the trailing components of p are suffix,
// e.g. hasPathSuffix("a/build", "build") is true but ("a/rebuild", "build") is not.
func hasPathSuffix(p, suffix string) bool {
	for suffix != rootDir && suffix != filepath.Dir(suffix) {
		if p == rootDir || filepath.Base(p) != filepath.Base(suffix) {
			return false
		}
		p, suffix = filepath.Dir(p), filepath.Dir(suffix)
	}
	return true
}

// relToScope returns the repo-relative file relative to a scope directory.
func relToScope(dir, file string) string {
	rel, err := filepath.Rel(dir, file)
	if err != nil {
		return file
	}
	return rel
}

// scopeLabel renders a scope directory for humans ("frontend/" or "frontend\").
func scopeLabel(dir string) string {
	return dir + string(filepath.Separator)
}
