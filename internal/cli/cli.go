// Package cli implements the Portcheck command-line interface.
package cli

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/aman-void/portcheck/internal/checker"
)

const version = "0.1.0"

const help = `portcheck - Check local TCP port availability

Usage:
  portcheck [options] <port>...

Options:
  -h, --help       Show help
  -v, --version    Show version
  -q, --quiet      Output only the status
  --              End option parsing

Checks TCP binding availability on 127.0.0.1. Ports must be 1-65535.
Options may appear before or after ports. Duplicate ports are checked once.

Exit codes:
  0  All ports free (or help/version)
  1  One or more ports in use
  2  Invalid input
  3  System error

Examples:
  portcheck 3000
  portcheck 3000 8080 5432
  portcheck -q 8080
`

type options struct {
	ports   []int
	quiet   bool
	help    bool
	version bool
}

// Run writes results and diagnostics to the supplied streams and returns an exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	return run(args, stdout, stderr, checker.Check)
}

func run(args []string, stdout, stderr io.Writer, check func(int) checker.Result) int {
	opts, err := parse(args)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 2
	}
	if opts.help {
		if _, err := io.WriteString(stdout, help); err != nil {
			fmt.Fprintf(stderr, "error: write output: %v\n", err)
			return 3
		}
		return 0
	}
	if opts.version {
		if _, err := fmt.Fprintf(stdout, "portcheck version %s\n", version); err != nil {
			fmt.Fprintf(stderr, "error: write output: %v\n", err)
			return 3
		}
		return 0
	}
	results := make([]checker.Result, 0, len(opts.ports))
	exit := 0
	for _, port := range opts.ports {
		result := check(port)
		results = append(results, result)
		switch result.Status {
		case checker.StatusInUse:
			if exit == 0 {
				exit = 1
			}
		case checker.StatusError:
			fmt.Fprintf(stderr, "error: failed to check port %d: %v\n", port, result.Err)
			exit = 3
		}
	}
	if err := writeResults(stdout, results, opts.quiet); err != nil {
		fmt.Fprintf(stderr, "error: write output: %v\n", err)
		return 3
	}
	return exit
}

func parse(args []string) (options, error) {
	var opts options
	var tokens []string
	flags := true
	for _, arg := range args {
		if flags {
			switch arg {
			case "--":
				flags = false
				continue
			case "-h", "--help":
				opts.help = true
				continue
			case "-v", "--version":
				opts.version = true
				continue
			case "-q", "--quiet":
				opts.quiet = true
				continue
			}
			if strings.HasPrefix(arg, "-") {
				return opts, fmt.Errorf("unknown option %q", arg)
			}
		}
		tokens = append(tokens, arg)
	}
	// Known help/version flags bypass port validation, but unknown flags never do.
	if opts.help || opts.version {
		return opts, nil
	}
	if len(tokens) == 0 {
		return opts, fmt.Errorf("at least one port is required; use --help for usage")
	}
	seen := make(map[int]bool)
	for _, token := range tokens {
		if token == "" || strings.IndexFunc(token, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
			return opts, fmt.Errorf("invalid port %q: must be a decimal integer", token)
		}
		port, err := strconv.Atoi(token)
		if err != nil || port < 1 || port > 65535 {
			return opts, fmt.Errorf("invalid port %q: must be between 1 and 65535", token)
		}
		if !seen[port] {
			seen[port] = true
			opts.ports = append(opts.ports, port)
		}
	}
	return opts, nil
}
