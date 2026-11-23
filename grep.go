package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dlclark/regexp2"
)

// grepOptions are the flags of the grep and pygrep languages, given in args.
type grepOptions struct {
	re        *regexp2.Regexp
	multiline bool
	negate    bool
}

// violation is a match of a grep hook (or, with --negate, a file without one).
type violation struct {
	line int    // 1-based line number, 0 for a whole file
	text string // the matching line, or the match itself with --multiline
}

// fingerprint identifies a violation in a lines-mode baseline: its text without
// surrounding whitespace, so that reindenting or moving code keeps it baselined.
func (v violation) fingerprint() string {
	return strings.TrimSpace(v.text)
}

func parseGrep(h *Hook) (*grepOptions, error) {
	opts := &grepOptions{}
	flags := regexp2.None
	for _, a := range h.Args {
		switch a {
		case "-i", "--ignore-case":
			flags |= regexp2.IgnoreCase
		case "--multiline":
			opts.multiline = true
			flags |= regexp2.Multiline | regexp2.Singleline
		case "--negate":
			opts.negate = true
		}
	}
	re, err := compileRegexp(h.Entry, flags)
	if err != nil {
		return nil, fmt.Errorf("invalid %s pattern: %w", h.Language, err)
	}
	opts.re = re
	return opts, nil
}

// find returns the violations in a file's content.
func (o *grepOptions) find(data []byte) ([]violation, error) {
	matches, err := o.matches(data)
	if err != nil {
		return nil, err
	}
	if !o.negate {
		return matches, nil
	}
	if len(matches) > 0 {
		return nil, nil
	}
	return []violation{{}}, nil
}

func (o *grepOptions) matches(data []byte) ([]violation, error) {
	if o.multiline {
		// Like pygrep, only the first match of the whole content is reported.
		text := []rune(string(data))
		m, err := o.re.FindRunesMatch(text)
		if err != nil || m == nil {
			return nil, err
		}
		line := strings.Count(string(text[:m.Index]), "\n") + 1
		return []violation{{line: line, text: m.String()}}, nil
	}
	if len(data) == 0 {
		return nil, nil
	}
	var found []violation
	// A trailing newline ends the last line; it does not start an empty one.
	for i, line := range bytes.Split(bytes.TrimSuffix(data, []byte("\n")), []byte("\n")) {
		// Like pygrep, a line is matched with its "\r": `^foo$` does not match "foo\r\n".
		ok, err := o.re.MatchString(string(line))
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}
		if ok {
			found = append(found, violation{line: i + 1, text: string(bytes.TrimRight(line, "\r"))})
		}
	}
	return found, nil
}

// scanGrep returns the violations of a grep hook, by file relative to the
// hook's config directory. Files without violations are present with no entries.
func (x *executor) scanGrep(r *hookRun, opts *grepOptions) (map[string][]violation, error) {
	found := make(map[string][]violation, len(r.files))
	for _, f := range r.files {
		data, err := os.ReadFile(filepath.Join(x.root, f)) //nolint:gosec // repo files
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		v, err := opts.find(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		found[relToScope(r.def.scope.dir, f)] = v
	}
	return found, nil
}

func formatViolation(rel string, v violation) string {
	if v.line == 0 {
		return rel
	}
	return fmt.Sprintf("%s:%d:%s", rel, v.line, v.text)
}

// runGrep implements the grep and pygrep languages: the entry is a regular
// expression and the hook fails when it matches (or, with --negate, when a file
// has no match). With a baseline, only violations not in the baseline fail, and
// fixed violations are removed from it (the baseline is a ratchet).
func (x *executor) runGrep(r *hookRun) *hookResult {
	h := r.def.hook
	opts, err := parseGrep(h)
	if err != nil {
		return &hookResult{status: statusFailed, note: err.Error()}
	}
	found, err := x.scanGrep(r, opts)
	if err != nil {
		return &hookResult{status: statusFailed, note: err.Error()}
	}

	res := &hookResult{status: statusPassed}
	var out bytes.Buffer
	newViolations := found
	if h.Baseline != "" {
		var shrunk int
		newViolations, shrunk, err = x.checkBaseline(r.def, found)
		if err != nil {
			return &hookResult{status: statusFailed, note: err.Error()}
		}
		if shrunk > 0 {
			res.modified = true
			res.status = statusFailed
			fmt.Fprintf(&out, "Removed %d fixed violation(s) from the baseline %s. Stage it to keep the improvement.\n",
				shrunk, filepath.ToSlash(h.Baseline))
		}
	}

	count := 0
	for _, rel := range sortedKeys(newViolations) {
		for _, v := range newViolations[rel] {
			fmt.Fprintln(&out, formatViolation(rel, v))
			count++
		}
	}
	if count > 0 {
		res.status, res.exitCode = statusFailed, 1
		if h.Baseline != "" {
			fmt.Fprintf(&out, "%d violation(s) are not in the baseline %s.\n", count, filepath.ToSlash(h.Baseline))
		}
	}
	res.output = out.Bytes()
	return res
}
