//go:build windows

package main

import (
	"os/exec"
	"strings"
	"syscall"
)

// cmdMetaChars are characters cmd.exe interprets outside double quotes.
const cmdMetaChars = " \t&|<>^()%!,;="

// quoteForBatch builds the command line of a .bat/.cmd hook explicitly. Go only
// quotes arguments containing spaces or quotes, but batch files run through
// cmd.exe, which would otherwise interpret characters like & or | in file names
// (e.g. "a&calc.js") as commands.
func quoteForBatch(cmd *exec.Cmd) {
	if !isBatchFile(cmd.Path) {
		return
	}
	args := make([]string, len(cmd.Args))
	for i, a := range cmd.Args {
		if i == 0 {
			a = cmd.Path
		}
		args[i] = batchArg(a)
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CmdLine = strings.Join(args, " ")
}

func batchArg(a string) string {
	escaped := syscall.EscapeArg(a)
	if strings.HasPrefix(escaped, `"`) || (a != "" && !strings.ContainsAny(a, cmdMetaChars)) {
		return escaped
	}
	// Backslashes before the closing quote must be doubled.
	trailing := len(escaped) - len(strings.TrimRight(escaped, `\`))
	return `"` + escaped + strings.Repeat(`\`, trailing) + `"`
}
