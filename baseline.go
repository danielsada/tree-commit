package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const (
	baselineLines = "lines"
	baselineCount = "count"

	baselineVersion = 1
)

// baselineDoc is the content of a baseline file. It is meant to be committed:
// paths use forward slashes and the JSON is sorted, so diffs stay readable.
type baselineDoc struct {
	Version int                      `json:"version"`
	Hooks   map[string]*hookBaseline `json:"hooks"`
}

// hookBaseline holds the accepted violations of one hook, by file relative to
// the directory of the hook's config file.
type hookBaseline struct {
	Mode  string                   `json:"mode"`
	Files map[string]*fileBaseline `json:"files"`
}

// fileBaseline is the accepted state of one file: the number of matching lines
// for each fingerprint (lines mode), or the number of violations (count mode).
type fileBaseline struct {
	Lines map[string]int `json:"lines,omitempty"`
	Count int            `json:"count,omitempty"`
}

func baselineMode(h *Hook) string {
	if h.BaselineMode == "" {
		return baselineLines
	}
	return h.BaselineMode
}

// baselinePath returns the absolute path of a hook's baseline file.
func (x *executor) baselinePath(d *hookDef) string {
	return filepath.Join(x.scopeDir(d), filepath.FromSlash(d.hook.Baseline))
}

func loadBaseline(path string) (*baselineDoc, error) {
	doc := &baselineDoc{Version: baselineVersion, Hooks: map[string]*hookBaseline{}}
	data, err := os.ReadFile(path) //nolint:gosec // baseline file named in a config file
	if errors.Is(err, os.ErrNotExist) {
		return doc, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read baseline: %w", err)
	}
	if err := json.Unmarshal(data, doc); err != nil {
		return nil, fmt.Errorf("invalid baseline %s: %w", path, err)
	}
	if doc.Version != baselineVersion {
		return nil, fmt.Errorf("baseline %s has unsupported version %d", path, doc.Version)
	}
	if doc.Hooks == nil {
		doc.Hooks = map[string]*hookBaseline{}
	}
	return doc, nil
}

func saveBaseline(path string, doc *baselineDoc) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644) //nolint:gosec // baselines are committed files
}

// hookEntry returns the baseline of a hook, creating it if needed. A baseline
// recorded in another mode must be recorded again.
func (doc *baselineDoc) hookEntry(id, mode string) (*hookBaseline, error) {
	hb := doc.Hooks[id]
	if hb == nil {
		hb = &hookBaseline{Mode: mode, Files: map[string]*fileBaseline{}}
		doc.Hooks[id] = hb
	}
	if hb.Mode != mode {
		return nil, fmt.Errorf("the baseline of %q uses mode %q, but the hook uses %q: run `tree-commit baseline %s`",
			id, hb.Mode, mode, id)
	}
	if hb.Files == nil {
		hb.Files = map[string]*fileBaseline{}
	}
	return hb, nil
}

// record makes found the accepted state of each file in it.
func (hb *hookBaseline) record(found map[string][]violation) {
	for rel, vs := range found {
		key := filepath.ToSlash(rel)
		fb := newFileBaseline(hb.Mode, vs)
		if fb == nil {
			delete(hb.Files, key)
		} else {
			hb.Files[key] = fb
		}
	}
}

func newFileBaseline(mode string, vs []violation) *fileBaseline {
	if len(vs) == 0 {
		return nil
	}
	if mode == baselineCount {
		return &fileBaseline{Count: len(vs)}
	}
	lines := map[string]int{}
	for _, v := range vs {
		lines[v.fingerprint()]++
	}
	return &fileBaseline{Lines: lines}
}

// check compares the violations of a file with its baseline. It returns the
// violations that are not accepted, and shrinks the baseline to what is still
// there. It reports whether the baseline changed.
func (hb *hookBaseline) check(rel string, vs []violation) (rejected []violation, changed bool) {
	key := filepath.ToSlash(rel)
	fb := hb.Files[key]
	if fb == nil {
		return vs, false
	}
	var kept *fileBaseline
	if hb.Mode == baselineCount {
		rejected, kept = checkCount(fb, vs)
	} else {
		rejected, kept = checkLines(fb, vs)
	}
	if kept == nil {
		delete(hb.Files, key)
	} else {
		hb.Files[key] = kept
	}
	return rejected, kept == nil || kept.size() != fb.size()
}

// checkCount accepts up to fb.Count violations. Without fingerprints the new
// violations are unknown, so all of them are reported when there are too many.
func checkCount(fb *fileBaseline, vs []violation) (rejected []violation, kept *fileBaseline) {
	if len(vs) > fb.Count {
		rejected = vs
	}
	if n := min(len(vs), fb.Count); n > 0 {
		kept = &fileBaseline{Count: n}
	}
	return rejected, kept
}

// checkLines accepts each baselined fingerprint as many times as it was recorded.
func checkLines(fb *fileBaseline, vs []violation) (rejected []violation, kept *fileBaseline) {
	remaining := make(map[string]int, len(fb.Lines))
	for k, n := range fb.Lines {
		remaining[k] = n
	}
	for _, v := range vs {
		if fp := v.fingerprint(); remaining[fp] > 0 {
			remaining[fp]--
		} else {
			rejected = append(rejected, v)
		}
	}
	lines := map[string]int{}
	for fp, n := range fb.Lines {
		if used := n - remaining[fp]; used > 0 {
			lines[fp] = used
		}
	}
	if len(lines) > 0 {
		kept = &fileBaseline{Lines: lines}
	}
	return rejected, kept
}

// size counts the violations a baseline accepts.
func (fb *fileBaseline) size() int {
	n := fb.Count
	for _, c := range fb.Lines {
		n += c
	}
	return n
}

// checkBaseline applies a hook's baseline to the violations it found. It
// returns the violations to report and how many accepted violations were fixed;
// the baseline file is rewritten when that number is not zero.
func (x *executor) checkBaseline(d *hookDef, found map[string][]violation) (rejected map[string][]violation, fixed int, err error) {
	path := x.baselinePath(d)
	x.baselineMu.Lock()
	defer x.baselineMu.Unlock()

	doc, err := loadBaseline(path)
	if err != nil {
		return nil, 0, err
	}
	hb, err := doc.hookEntry(d.id, baselineMode(d.hook))
	if err != nil {
		return nil, 0, err
	}
	before := 0
	for _, fb := range hb.Files {
		before += fb.size()
	}

	rejected = map[string][]violation{}
	changed := false
	for rel, vs := range found {
		bad, c := hb.check(rel, vs)
		if len(bad) > 0 {
			rejected[rel] = bad
		}
		changed = changed || c
	}
	// Files that no longer exist have no violations left.
	for key := range hb.Files {
		if !fileExists(filepath.Join(x.scopeDir(d), filepath.FromSlash(key))) {
			delete(hb.Files, key)
			changed = true
		}
	}
	if !changed {
		return rejected, 0, nil
	}

	after := 0
	for _, fb := range hb.Files {
		after += fb.size()
	}
	if len(hb.Files) == 0 {
		delete(doc.Hooks, d.id)
	}
	if err := saveBaseline(path, doc); err != nil {
		return nil, 0, fmt.Errorf("failed to update baseline: %w", err)
	}
	return rejected, before - after, nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
