package main

import (
	"time"

	"github.com/dlclark/regexp2"
)

// patternTimeout bounds a single regular expression match. regexp2 backtracks
// (like Python's re), so a pathological pattern could otherwise hang a commit.
const patternTimeout = 2 * time.Second

// pattern is a regular expression with Python-compatible syntax: lookarounds,
// backreferences, named groups and the verbose flag (?x) work as in pre-commit,
// whose patterns are evaluated with Python's re module.
type pattern struct {
	re *regexp2.Regexp
}

// compilePattern compiles a pre-commit style regular expression. Patterns are
// matched with search semantics (unanchored), like Python's re.search. An empty
// pattern returns nil, which matches nothing to filter on.
func compilePattern(expr string) (*pattern, error) {
	if expr == "" {
		return nil, nil
	}
	re, err := compileRegexp(expr, regexp2.None)
	if err != nil {
		return nil, err
	}
	return &pattern{re: re}, nil
}

func compileRegexp(expr string, opts regexp2.RegexOptions) (*regexp2.Regexp, error) {
	re, err := regexp2.Compile(expr, opts)
	if err != nil {
		return nil, err
	}
	re.MatchTimeout = patternTimeout
	return re, nil
}

// MatchString reports whether s contains a match. A match that times out counts
// as no match.
func (p *pattern) MatchString(s string) bool {
	ok, err := p.re.MatchString(s)
	return err == nil && ok
}
