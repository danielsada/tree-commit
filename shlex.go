package main

import (
	"errors"
	"strings"
	"unicode"
)

// shellLexer splits a command line using POSIX shell quoting rules.
type shellLexer struct {
	args    []string
	cur     strings.Builder
	inWord  bool
	quote   rune
	escaped bool
}

// splitCommand splits a hook entry into arguments using POSIX shell rules
// (like Python's shlex.split, which pre-commit uses): whitespace separates
// words, single quotes are literal, double quotes allow backslash escapes.
func splitCommand(s string) ([]string, error) {
	l := &shellLexer{}
	for _, r := range s {
		l.feed(r)
	}
	if l.escaped || l.quote != 0 {
		return nil, errors.New("unterminated quote or escape in command")
	}
	if l.inWord {
		l.args = append(l.args, l.cur.String())
	}
	return l.args, nil
}

func (l *shellLexer) feed(r rune) {
	switch {
	case l.escaped:
		// Inside double quotes a backslash only escapes a few characters.
		if l.quote == '"' && !strings.ContainsRune("$`\"\\\n", r) {
			l.cur.WriteRune('\\')
		}
		l.cur.WriteRune(r)
		l.escaped = false
	case l.quote == '\'':
		l.feedQuoted(r)
	case r == '\\':
		l.escaped = true
		l.inWord = true
	case l.quote == '"':
		l.feedQuoted(r)
	default:
		l.feedUnquoted(r)
	}
}

func (l *shellLexer) feedQuoted(r rune) {
	if r == l.quote {
		l.quote = 0
		return
	}
	l.cur.WriteRune(r)
}

func (l *shellLexer) feedUnquoted(r rune) {
	switch {
	case r == '\'' || r == '"':
		l.quote = r
		l.inWord = true
	case unicode.IsSpace(r):
		if l.inWord {
			l.args = append(l.args, l.cur.String())
			l.cur.Reset()
			l.inWord = false
		}
	default:
		l.cur.WriteRune(r)
		l.inWord = true
	}
}
