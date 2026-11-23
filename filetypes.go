package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// fileTags returns the identify-style tags for a file (file/symlink, text/binary,
// executable/non-executable, plus language tags from the name, extension and shebang).
func fileTags(path string) (map[string]bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}

	tags := map[string]bool{}
	if info.Mode()&os.ModeSymlink != 0 {
		tags["symlink"] = true
		return tags, nil
	}
	if info.IsDir() {
		tags["directory"] = true
		return tags, nil
	}
	tags["file"] = true

	head, err := readHead(path, 1024)
	if err != nil {
		return nil, err
	}

	executable := info.Mode()&0o111 != 0
	if runtime.GOOS == windows {
		executable = bytes.HasPrefix(head, []byte("#!"))
	}
	if executable {
		tags["executable"] = true
	} else {
		tags["non-executable"] = true
	}

	// Like identify, only look at the shebang when the name says nothing.
	if nameTags := tagsFromFilename(filepath.Base(path)); len(nameTags) > 0 {
		for t := range nameTags {
			tags[t] = true
		}
	} else if executable {
		for _, t := range tagsFromInterpreter(shebangInterpreter(head)) {
			tags[t] = true
		}
	}

	if !tags["text"] && !tags["binary"] {
		if isText(head) {
			tags["text"] = true
		} else {
			tags["binary"] = true
		}
	}
	return tags, nil
}

// tagsFromFilename returns the tags implied by a file name: a well-known name
// (also as a dot-separated part, so "Dockerfile.dev" is a Dockerfile) and the
// extension.
func tagsFromFilename(name string) map[string]bool {
	tags := map[string]bool{}
	for _, part := range append([]string{name}, strings.Split(name, ".")...) {
		if known, ok := nameTags[part]; ok {
			for _, t := range known {
				tags[t] = true
			}
			break
		}
	}
	if ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), ".")); ext != "" {
		known, ok := extensionTags[ext]
		if !ok {
			known = extensionTagsNeedBinaryCheck[ext]
		}
		for _, t := range known {
			tags[t] = true
		}
	}
	return tags
}

// tagsFromInterpreter returns the tags for a shebang interpreter, dropping
// version suffixes until one is known: python3.12 -> python3 -> python.
func tagsFromInterpreter(interp string) []string {
	for interp != "" {
		if tags, ok := interpreterTags[interp]; ok {
			return tags
		}
		i := strings.LastIndex(interp, ".")
		if i < 0 {
			break
		}
		interp = interp[:i]
	}
	return nil
}

func readHead(path string, n int) ([]byte, error) {
	f, err := os.Open(path) //nolint:gosec // reading files selected from the git index
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	buf := make([]byte, n)
	read, err := io.ReadFull(f, buf)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, err
	}
	return buf[:read], nil
}

// isText mirrors identify's heuristic: text files contain only printable
// characters and common control characters.
func isText(data []byte) bool {
	for _, b := range data {
		switch {
		case b >= 0x20 && b != 0x7f:
		case b == '\a' || b == '\b' || b == '\t' || b == '\n' || b == '\v' || b == '\f' || b == '\r' || b == 0x1b:
		default:
			return false
		}
	}
	return true
}

// shebangInterpreter returns the interpreter name from a "#!" line, resolving "/usr/bin/env".
func shebangInterpreter(head []byte) string {
	if !bytes.HasPrefix(head, []byte("#!")) {
		return ""
	}
	line, _, _ := bytes.Cut(head[2:], []byte("\n"))
	fields := strings.Fields(strings.TrimSpace(string(line)))
	if len(fields) == 0 {
		return ""
	}
	interp := filepath.Base(filepath.ToSlash(fields[0]))
	if interp == "env" {
		for _, f := range fields[1:] {
			if !strings.HasPrefix(f, "-") {
				interp = f
				break
			}
		}
	}
	return interp
}

// matchesTypes applies pre-commit's types / types_or / exclude_types semantics.
func matchesTypes(tags map[string]bool, types, typesOr, excludeTypes []string) bool {
	for _, t := range types {
		if !tags[t] {
			return false
		}
	}
	if len(typesOr) > 0 {
		found := false
		for _, t := range typesOr {
			if tags[t] {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	for _, t := range excludeTypes {
		if tags[t] {
			return false
		}
	}
	return true
}
