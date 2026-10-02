// Package cli implements the Portcheck command-line interface.
package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aman-void/portcheck"
	"github.com/aman-void/portcheck/internal/process"
)

const help = `portcheck - Check local TCP port availability

Usage:
  portcheck [options] <port>...
  portcheck [options] --find <starting-port>
  portcheck --connect <host:port> [options]
  portcheck doctor [options] <port>

Options:
  -h, --help       Show help
  -v, --version    Show version
  -q, --quiet      Output only the status
      --json       Output an ordered JSON array (conflicts with --quiet)
      --find       Print the first free port at or above the starting port
      --host HOST  Local IP address or DNS hostname (default 127.0.0.1)
      --process    Inspect visible local listening processes (Linux)
      --connect E  Test TCP connectivity to host:port (IPv6: [::1]:8080)
      --watch      Emit initial state and changes until interrupted
      --wait       Wait until all ports are free
      --wait-in-use Wait until all ports are in use
      --interval D Polling interval (default 1s, minimum 100ms)
      --timeout D  Positive timeout (wait: unbounded; connect/doctor: 5s)
  --              End option parsing

Normal checks test local TCP binding, not connectivity. Ports must be 1-65535.
Connect sends no payload and closes immediately; TCP success is not application health.
Doctor combines local binding, visible processes, and connectivity for one port.
Doctor rejects quiet/find/watch/wait; connect rejects host/process and local modes.
JSON is always an array, including connectivity and doctor reports.
Options may appear before or after ports. Duplicate ports are checked once.
Connect's endpoint may follow intervening long flags, e.g. --connect --quiet HOST:PORT.
--host followed by a long flag selects default 127.0.0.1; --host -q is a missing value.
Ranges such as 3000-3010 include both endpoints. --find takes one single port.
--find prints a port number, also in quiet mode; JSON contains one free result.
Host IPs are unbracketed, including ::1. Hostnames check one Go-selected address.
Watch, wait, wait-in-use, and find are mutually exclusive. --watch --json is invalid.
--interval requires watch/wait; --timeout requires wait/connect/doctor. Durations use Go syntax.
Wait emits only final results. Watch quiet mode emits only changed status tokens.
Interrupt: watch exits 0 (3 if checks failed); wait exits 1. Wait timeout exits 1.
Process lookup warnings do not change port status or exit codes. Quiet stays status-only.
Process watch emits ownership changes; wait inspects final results; find skips inspection.

Exit codes:
  0  All ports free, a free port found, wait satisfied, clean watch stop, or help/version
  1  Ports in use, find exhausted, or wait canceled/timed out
  2  Invalid input
  3  System error

Connect/doctor: reachable exits 0; refused/timeout exits 1; other errors exit 3.
Doctor local system errors exit 3; process warnings never change exit status.
Doctor's timeout bounds binding, inspection, and dialing together.

Examples:
  portcheck 3000
  portcheck 3000 8080 5432
  portcheck -q 8080
  portcheck 3000 8080-8083
  portcheck --find 3000
  portcheck --json 3000 8080
  portcheck --host ::1 8080
  portcheck --watch --interval 2s 8080
  portcheck --wait --timeout 30s 8080
  portcheck --process --json 8080
  portcheck --connect localhost:8080 --timeout 2s
  portcheck doctor --host ::1 8080 --json
`

type options struct {
	ports    []int
	quiet    bool
	help     bool
	version  bool
	json     bool
	find     bool
	host     string
	watch    bool
	wait     bool
	inUse    bool
	interval time.Duration
	timeout  time.Duration
	process  bool
	connect  string
	doctor   bool
}

// Run writes results and diagnostics to the supplied streams and returns an exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	return RunContext(context.Background(), args, stdout, stderr)
}

// RunContext supports cancellation; the executable owns signal handling.
func RunContext(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	return runContext(ctx, args, stdout, stderr, func(ctx context.Context, host string, port int) portcheck.Result {
		result, _ := portcheck.CheckHost(ctx, host, port)
		return result
	})
}

func runContext(ctx context.Context, args []string, stdout, stderr io.Writer, check func(context.Context, string, int) portcheck.Result) int {
	return runContextWithInspector(ctx, args, stdout, stderr, check, process.Inspect)
}

func runContextWithInspector(ctx context.Context, args []string, stdout, stderr io.Writer, check func(context.Context, string, int) portcheck.Result, inspect inspectFunc) int {
	return runContextWithConnector(ctx, args, stdout, stderr, check, inspect, portcheck.Connect)
}

func runContextWithConnector(ctx context.Context, args []string, stdout, stderr io.Writer, check func(context.Context, string, int) portcheck.Result, inspect inspectFunc, connect connectFunc) int {
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
		if _, err := fmt.Fprintf(stdout, "portcheck version %s\n", applicationVersion()); err != nil {
			fmt.Fprintf(stderr, "error: write output: %v\n", err)
			return 3
		}
		return 0
	}
	if opts.connect != "" {
		return runConnect(ctx, opts, stdout, stderr, connect)
	}
	if opts.doctor {
		return runDoctor(ctx, opts, stdout, stderr, check, inspect, connect)
	}
	if opts.watch || opts.wait || opts.inUse {
		return runPollingWithInspector(ctx, opts, stdout, stderr, check, nil, time.Now, inspect)
	}
	checkPort := func(port int) portcheck.Result {
		if err := ctx.Err(); err != nil {
			return portcheck.Result{Port: port, Status: portcheck.StatusError, Err: err}
		}
		return check(ctx, opts.host, port)
	}
	if opts.find {
		return runFind(opts, stdout, stderr, checkPort)
	}
	results := make([]portcheck.Result, 0, len(opts.ports))
	details := make(map[int]processDetail)
	exit := 0
	for _, port := range opts.ports {
		if ctx.Err() != nil {
			fmt.Fprintf(stderr, "error: check canceled: %v\n", ctx.Err())
			if exit == 3 {
				return 3
			}
			return 1
		}
		result := checkPort(port)
		if opts.process && result.Status == portcheck.StatusInUse {
			details[port] = inspectDetail(ctx, opts.host, port, inspect)
			warnProcess(stderr, opts.host, port, details[port])
		}
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
	if err := renderWithProcesses(stdout, results, opts, details); err != nil {
		fmt.Fprintf(stderr, "error: write output: %v\n", err)
		return 3
	}
	return exit
}

func parse(args []string) (options, error) {
	opts := options{host: "127.0.0.1", interval: time.Second}
	var tokens []string
	var intervalSet, timeoutSet, hostSet, connectSet bool
	var pendingValue string
	flags := true
	for i := 0; i < len(args); i++ {
		arg := args[i]
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
			case "--process":
				opts.process = true
				continue
			case "--watch":
				opts.watch = true
				continue
			case "--wait":
				opts.wait = true
				continue
			case "--wait-in-use":
				opts.inUse = true
				continue
			case "--host", "--connect":
				if pendingValue != "" {
					return opts, fmt.Errorf("%s requires a value before %s", pendingValue, arg)
				}
				if arg == "--connect" && connectSet {
					return opts, fmt.Errorf("--connect may be supplied only once")
				}
				// A following long flag means the host value was omitted, so keep the
				// default. Without this the next argument is consumed as the hostname,
				// which hides a forgotten value behind a confusing "invalid host" error.
				// Short aliases keep their missing-value diagnostic.
				if arg == "--host" && i+1 < len(args) && args[i+1] != "--" &&
					strings.HasPrefix(args[i+1], "--") && knownOption(args[i+1]) {
					hostSet = true
					continue
				}
				if i+1 == len(args) {
					return opts, fmt.Errorf("%s requires a value", arg)
				}
				next := args[i+1]
				if knownOption(next) {
					// Only --connect reaches here with a known option following it;
					// --host long flags were consumed above and its short aliases
					// must still report a missing value.
					if next == "--" || arg == "--host" {
						return opts, fmt.Errorf("%s requires a value", arg)
					}
					connectSet = true
					pendingValue = arg
					continue
				}
				if strings.HasPrefix(next, "-") {
					return opts, fmt.Errorf("%s requires a value", arg)
				}
				i++
				value := args[i]
				if arg == "--connect" {
					opts.connect, connectSet = value, true
				} else {
					opts.host, hostSet = value, true
				}
				continue
			case "--interval", "--timeout":
				if i+1 == len(args) || strings.HasPrefix(args[i+1], "--") {
					return opts, fmt.Errorf("%s requires a value", arg)
				}
				i++
				value := args[i]
				duration, err := time.ParseDuration(value)
				if err != nil || duration <= 0 {
					return opts, fmt.Errorf("%s requires a positive Go duration, got %q", arg, value)
				}
				if arg == "--interval" {
					if duration < 100*time.Millisecond {
						return opts, fmt.Errorf("--interval must be at least 100ms")
					}
					opts.interval, intervalSet = duration, true
				} else {
					opts.timeout, timeoutSet = duration, true
				}
				continue
			}
			if strings.HasPrefix(arg, "-") {
				return opts, fmt.Errorf("unknown option %q", arg)
			}
		}
		if pendingValue != "" {
			if pendingValue == "--connect" {
				opts.connect = arg
			} else {
				opts.host = arg
			}
			pendingValue = ""
			continue
		}
		if flags && arg == "doctor" && len(tokens) == 0 && !opts.doctor {
			opts.doctor = true
			continue
		}
		tokens = append(tokens, arg)
	}
	if pendingValue != "" {
		return opts, fmt.Errorf("%s requires a value", pendingValue)
	}
	// Known help/version flags bypass port validation, but unknown flags never do.
	if opts.help || opts.version {
		return opts, nil
	}
	if opts.quiet && opts.json {
		return opts, fmt.Errorf("--quiet and --json cannot be used together")
	}
	modes := 0
	for _, enabled := range []bool{opts.find, opts.watch, opts.wait, opts.inUse, connectSet, opts.doctor} {
		if enabled {
			modes++
		}
	}
	if modes > 1 {
		return opts, fmt.Errorf("--find, --watch, --wait, --wait-in-use, --connect, and doctor are mutually exclusive")
	}
	if opts.watch && opts.json {
		return opts, fmt.Errorf("--watch and --json cannot be used together; JSON output is a final result array")
	}
	if intervalSet && !(opts.watch || opts.wait || opts.inUse) {
		return opts, fmt.Errorf("--interval requires --watch, --wait, or --wait-in-use")
	}
	if timeoutSet && !(opts.wait || opts.inUse || connectSet || opts.doctor) {
		return opts, fmt.Errorf("--timeout requires --wait, --wait-in-use, --connect, or doctor")
	}
	if connectSet || opts.doctor {
		if !timeoutSet {
			opts.timeout = portcheck.DefaultConnectTimeout
		}
	}
	if connectSet {
		if hostSet || opts.process || len(tokens) != 0 {
			return opts, fmt.Errorf("--connect cannot combine with --host, --process, or positional ports")
		}
		return opts, portcheck.ValidateEndpoint(opts.connect)
	}
	if err := portcheck.ValidateHost(opts.host); err != nil {
		return opts, err
	}
	if opts.doctor {
		if opts.quiet {
			return opts, fmt.Errorf("doctor does not support --quiet; use --json for structured reports")
		}
		if len(tokens) != 1 {
			return opts, fmt.Errorf("doctor requires exactly one single port")
		}
		port, err := parsePort(tokens[0])
		opts.ports = []int{port}
		return opts, err
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

func knownOption(arg string) bool {
	switch arg {
	case "--", "-h", "--help", "-v", "--version", "-q", "--quiet",
		"--json", "--find", "--process", "--watch", "--wait", "--wait-in-use",
		"--host", "--interval", "--timeout", "--connect":
		return true
	}
	return false
}
