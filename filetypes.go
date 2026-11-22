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

// extensionTags maps file extensions to identify-style type tags. It covers the
// tags used by the most common pre-commit hook repositories.
var extensionTags = map[string][]string{
	"bash": {"text", "shell", "bash"}, "bat": {"text", "batch"},
	"c": {"text", "c"}, "cc": {"text", "c++"}, "cfg": {"text"}, "cpp": {"text", "c++"},
	"cs": {"text", "c#"}, "css": {"text", "css"}, "csv": {"text", "csv"},
	"go": {"text", "go"}, "gif": {"binary", "image", "gif"}, "gradle": {"text", "groovy"},
	"h": {"text", "header", "c"}, "hpp": {"text", "header", "c++"}, "html": {"text", "html"},
	"ini": {"text", "ini"}, "java": {"text", "java"}, "jpeg": {"binary", "image", "jpeg"},
	"jpg": {"binary", "image", "jpeg"}, "js": {"text", "javascript"}, "json": {"text", "json"},
	"jsx": {"text", "jsx"}, "kt": {"text", "kotlin"}, "lua": {"text", "lua"},
	"md": {"text", "markdown"}, "mjs": {"text", "javascript"}, "php": {"text", "php"},
	"png": {"binary", "image", "png"}, "proto": {"text", "proto"}, "ps1": {"text", "powershell"},
	"py": {"text", "python"}, "pyi": {"text", "pyi"}, "rb": {"text", "ruby"},
	"rs": {"text", "rust"}, "rst": {"text", "rst"}, "scss": {"text", "scss"},
	"sh": {"text", "shell", "sh"}, "sql": {"text", "sql"}, "svg": {"text", "image", "svg", "xml"},
	"swift": {"text", "swift"}, "tf": {"text", "terraform"}, "toml": {"text", "toml"},
	"ts": {"text", "ts"}, "tsx": {"text", "tsx"}, "txt": {"text", "plain-text"},
	"xml": {"text", "xml"}, "yaml": {"text", "yaml"}, "yml": {"text", "yaml"},
	"zip": {"binary", "zip"}, "zsh": {"text", "shell", "zsh"},
}

// nameTags maps well-known file names to type tags.
var nameTags = map[string][]string{
	"Dockerfile": {"text", "dockerfile"},
	"Makefile":   {"text", "makefile"},
	"go.mod":     {"text", "go-mod"},
	"go.sum":     {"text", "go-sum"},
}

// interpreterTags maps shebang interpreters to type tags.
var interpreterTags = map[string][]string{
	"bash": {"shell", "bash"}, "sh": {"shell", "sh"}, "zsh": {"shell", "zsh"},
	"python": {"python"}, "python3": {"python", "python3"},
	"node": {"javascript"}, "ruby": {"ruby"}, "perl": {"perl"},
}

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
		for _, t := range interpreterTags[shebangInterpreter(head)] {
			tags[t] = true
		}
	} else {
		tags["non-executable"] = true
	}

	base := filepath.Base(path)
	for _, t := range nameTags[base] {
		tags[t] = true
	}
	if ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(base), ".")); ext != "" {
		for _, t := range extensionTags[ext] {
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
