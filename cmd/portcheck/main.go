package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/aman-void/portcheck/internal/cli"
)

func main() {
	prepareSignals()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := cli.RunContext(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}
