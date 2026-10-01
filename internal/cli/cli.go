// Package cli implements the Portcheck command-line interface.
package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aman-void/portcheck"
)

const version = "0.2.0"

const help = `portcheck - Check local TCP port availability

Usage:
  portcheck [options] <port>...
  portcheck [options] --find <starting-port>

Options:
  -h, --help       Show help
  -v, --version    Show version
  -q, --quiet      Output only the status
      --json       Output an ordered JSON array (conflicts with --quiet)
      --find       Print the first free port at or above the starting port
  --              End option parsing

Checks TCP binding availability on 127.0.0.1. Ports must be 1-65535.
Options may appear before or after ports. Duplicate ports are checked once.
Ranges such as 3000-3010 include both endpoints. --find takes one single port.
--find prints a port number, also in quiet mode; JSON contains one free result.

Exit codes:
  0  All ports free, a free port found, or help/version
  1  One or more ports in use, or no free port found
  2  Invalid input
  3  System error

Examples:
  portcheck 3000
  portcheck 3000 8080 5432
  portcheck -q 8080
  portcheck 3000 8080-8083
  portcheck --find 3000
  portcheck --json 3000 8080
`

type options struct {
	ports   []int
	quiet   bool
	help    bool
	version bool
	json    bool
	find    bool
}

// Run writes results and diagnostics to the supplied streams and returns an exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	return run(args, stdout, stderr, func(port int) portcheck.Result {
		result, _ := portcheck.Check(context.Background(), port)
		return result
	})
}

func run(args []string, stdout, stderr io.Writer, check func(int) portcheck.Result) int {
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
	if opts.find {
		return runFind(opts, stdout, stderr, check)
	}
	results := make([]portcheck.Result, 0, len(opts.ports))
	exit := 0
	for _, port := range opts.ports {
		result := check(port)
		results = append(results, result)
		switch result.Status {
		case portcheck.StatusInUse:
			if exit == 0 {
				exit = 1
			}
		case portcheck.StatusError:
			fmt.Fprintf(stderr, "error: failed to check port %d: %v\n", port, result.Err)
			exit = 3
		}
	}
	if err := render(stdout, results, opts); err != nil {
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
			case "--json":
				opts.json = true
				continue
			case "--find":
				opts.find = true
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
	if opts.quiet && opts.json {
		return opts, fmt.Errorf("--quiet and --json cannot be used together")
	}
	if opts.find {
		if len(tokens) != 1 {
			return opts, fmt.Errorf("--find requires exactly one starting port")
		}
		if strings.Contains(tokens[0], "-") {
			return opts, fmt.Errorf("--find requires a single starting port, not a range")
		}
		port, err := parsePort(tokens[0])
		if err != nil {
			return opts, err
		}
		opts.ports = []int{port}
		return opts, nil
	}
	if len(tokens) == 0 {
		return opts, fmt.Errorf("at least one port is required; use --help for usage")
	}
	ports, err := expandPorts(tokens)
	opts.ports = ports
	return opts, err
}
