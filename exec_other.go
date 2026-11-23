//go:build !windows

package main

import "os/exec"

// quoteForBatch is only needed for Windows batch files.
func quoteForBatch(*exec.Cmd) {}
