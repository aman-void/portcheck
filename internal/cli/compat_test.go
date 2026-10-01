package cli

import (
	"context"
	"io"

	"github.com/aman-void/portcheck"
)

// Retain the port-only checker adapter for v0.1/v0.2 regression tests.
// Host-aware v0.3 tests use runContext directly, as production does.
func run(args []string, stdout, stderr io.Writer, check func(int) portcheck.Result) int {
	return runContext(context.Background(), args, stdout, stderr, func(_ context.Context, _ string, port int) portcheck.Result {
		return check(port)
	})
}
