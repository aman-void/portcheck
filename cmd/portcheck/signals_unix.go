//go:build !windows

package main

import (
	"os/signal"
	"syscall"
)

func prepareSignals() {
	// Let stdout writes return EPIPE so the CLI can report exit 3, rather than
	// having the Go runtime terminate the process with SIGPIPE (shell exit 141).
	// Keep this policy out of the reusable library and CLI package.
	signal.Ignore(syscall.SIGPIPE)
}
