package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// stash holds the unstaged changes of the work tree while hooks run, so that
// hooks only see what will be committed. It works like pre-commit's
// staged_files_only: the changes are saved as a patch, the work tree is reset to
// the index, and the patch is applied again afterwards.
type stash struct {
	root        string
	out         io.Writer
	patch       string   // patch file with the unstaged changes, "" if there were none
	intentToAdd []string // files added with `git add -N`, removed from the index meanwhile
}

// stashForRun stashes the unstaged changes when hooks run on staged files, so
// that they only see the staged content (like pre-commit). Otherwise it returns
// an empty stash.
func stashForRun(root string, opts *runOptions, out io.Writer) (*stash, error) {
	if opts.allFiles || len(opts.files) > 0 || opts.dryRun {
		return &stash{root: root, out: out}, nil
	}
	patches, err := patchDir()
	if err != nil {
		return nil, err
	}
	return stashUnstaged(root, patches, out)
}

// stashUnstaged saves and removes the unstaged changes of root. Call restore
// when the hooks are done, also when they fail.
func stashUnstaged(root, patchDir string, out io.Writer) (*stash, error) {
	s := &stash{root: root, out: out}
	if err := s.clearIntentToAdd(); err != nil {
		return nil, err
	}

	tree, err := gitOutput(root, "write-tree")
	if err != nil {
		return nil, s.fail(err)
	}
	diff, err := gitOutput(root, "diff-index", "--ignore-submodules", "--binary", "--no-color",
		"--no-ext-diff", strings.TrimSpace(tree), "--")
	if err != nil {
		return nil, s.fail(err)
	}
	if diff == "" {
		return s, nil
	}

	if err := os.MkdirAll(patchDir, 0o750); err != nil {
		return nil, s.fail(fmt.Errorf("failed to create %s: %w", patchDir, err))
	}
	s.patch = filepath.Join(patchDir, fmt.Sprintf("patch%d-%d", time.Now().UnixNano(), os.Getpid()))
	if err := os.WriteFile(s.patch, []byte(diff), 0o600); err != nil {
		return nil, s.fail(fmt.Errorf("failed to save unstaged changes: %w", err))
	}
	fmt.Fprintln(out, "[WARNING] Unstaged files detected.")
	fmt.Fprintf(out, "[INFO] Stashing unstaged files to %s.\n", s.patch)
	if _, err := gitOutput(root, "-c", "submodule.recurse=0", "checkout", "--", "."); err != nil {
		return nil, s.fail(fmt.Errorf("failed to stash unstaged changes: %w", err))
	}
	return s, nil
}

// fail restores what was changed so far and returns err.
func (s *stash) fail(err error) error {
	if restoreErr := s.restore(); restoreErr != nil {
		return errors.Join(err, restoreErr)
	}
	return err
}

// clearIntentToAdd removes `git add -N` files from the index: they have no
// staged content, and `git write-tree` cannot write them.
func (s *stash) clearIntentToAdd() error {
	out, err := gitOutput(s.root, "diff", "--no-ext-diff", "--ignore-submodules", "--diff-filter=A", "--name-only", "-z")
	if err != nil {
		return err
	}
	// --diff-filter=A on the index compared with the work tree lists exactly the
	// intent-to-add entries.
	s.intentToAdd = splitNull(out)
	if len(s.intentToAdd) == 0 {
		return nil
	}
	args := append([]string{"rm", "--cached", "-q", "--"}, s.intentToAdd...)
	if _, err := gitOutput(s.root, args...); err != nil {
		s.intentToAdd = nil
		return fmt.Errorf("failed to clear intent-to-add files: %w", err)
	}
	return nil
}

// restore applies the stashed changes again. If they conflict with changes made
// by hooks, the hook changes are rolled back: the unstaged work always wins.
func (s *stash) restore() error {
	var errs []error
	if s.patch != "" {
		if err := s.restorePatch(); err != nil {
			errs = append(errs, err)
		}
		s.patch = ""
	}
	if len(s.intentToAdd) > 0 {
		args := append([]string{"add", "--intent-to-add", "--"}, s.intentToAdd...)
		if _, err := gitOutput(s.root, args...); err != nil {
			errs = append(errs, fmt.Errorf("failed to restore intent-to-add files: %w", err))
		}
		s.intentToAdd = nil
	}
	return errors.Join(errs...)
}

// restorePatch applies the stashed patch, rolling back hook changes if needed.
func (s *stash) restorePatch() error {
	if err := s.applyPatch(); err != nil {
		fmt.Fprintln(s.out, "[WARNING] Stashed changes conflicted with hook auto-fixes... Rolling back fixes...")
		_, checkoutErr := gitOutput(s.root, "-c", "submodule.recurse=0", "checkout", "--", ".")
		if err := s.applyPatch(); err != nil || checkoutErr != nil {
			return fmt.Errorf("failed to restore unstaged changes, they are saved in %s: %w",
				s.patch, errors.Join(checkoutErr, err))
		}
	}
	fmt.Fprintf(s.out, "[INFO] Restored changes from %s.\n", s.patch)
	// The patch is only kept when it could not be applied.
	_ = os.Remove(s.patch)
	return nil
}

// applyPatch applies the stashed patch to the work tree. Like pre-commit, it
// retries without line ending conversion, which can make a clean patch fail.
func (s *stash) applyPatch() error {
	_, err := gitOutput(s.root, "apply", "--whitespace=nowarn", s.patch)
	if err == nil {
		return nil
	}
	_, retryErr := gitOutput(s.root, "-c", "core.autocrlf=false", "apply", "--whitespace=nowarn", s.patch)
	if retryErr == nil {
		return nil
	}
	return err
}

// patchDir returns the directory for stashed patches in the cache.
func patchDir() (string, error) {
	dir, err := cacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "patches"), nil
}
