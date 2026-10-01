package cli

import (
	"fmt"
	"io"

	"github.com/aman-void/portcheck"
)

func runFind(opts options, stdout, stderr io.Writer, check func(int) portcheck.Result) int {
	start := opts.ports[0]
	for port := start; port <= 65535; port++ {
		result := check(port)
		switch result.Status {
		case portcheck.StatusInUse:
			continue
		case portcheck.StatusError:
			fmt.Fprintf(stderr, "error: failed to check port %d: %v\n", port, result.Err)
			if opts.json {
				if err := writeJSON(stdout, []portcheck.Result{result}); err != nil {
					fmt.Fprintf(stderr, "error: write output: %v\n", err)
				}
			}
			return 3
		}
		var err error
		if opts.json {
			err = writeJSON(stdout, []portcheck.Result{result})
		} else {
			_, err = fmt.Fprintf(stdout, "%d\n", result.Port)
		}
		if err != nil {
			fmt.Fprintf(stderr, "error: write output: %v\n", err)
			return 3
		}
		return 0
	}
	fmt.Fprintf(stderr, "error: no free port found between %d and 65535\n", start)
	return 1
}
