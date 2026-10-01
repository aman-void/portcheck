# Portcheck

A tiny Go CLI that answers one question: **can I bind this local TCP port?**

Check ports and ranges, find an available port, watch state changes, or wait
for a desired state. Consume results through JSON and a small Go API.
**v0.3.0** uses only Go's standard library, defaults to IPv4 loopback
(`127.0.0.1`), and supports explicit local IPv4/IPv6 addresses and hostnames.

```text
PORT    STATUS
3000    FREE
8080    IN USE
```

## Quick start

You need **Go 1.27.1 or newer** (see `go.mod`). Make is optional; Python is
only needed for the hands-on listener example below.

Run these commands from the repository directory:

```sh
make build
./bin/portcheck 3000 8080
./bin/portcheck --help
./bin/portcheck --version
```

The version command prints `portcheck version 0.3.0`. Port statuses depend
on what is running on your machine; the example output is illustrative.

**Without Make:**

```sh
go build -o bin/portcheck ./cmd/portcheck
./bin/portcheck 8080
```

On Windows, build with `go build -o bin/portcheck.exe ./cmd/portcheck`
and run `.\bin\portcheck.exe` in PowerShell. Shell examples below use a
POSIX shell; native runtime behavior has been tested on Linux only.

### Install the command locally

From this checkout:

```sh
go install ./cmd/portcheck
portcheck --version
```

The command is installed into `GOBIN`, or `$(go env GOPATH)/bin` when unset.
Add that directory to your `PATH` if `portcheck` is not found.
Remote installation depends on the repository being published at
`github.com/aman-void/portcheck`; these checkout-based commands do not.

## Usage examples

Examples use the built binary; after local installation, you can replace
`./bin/portcheck` with `portcheck`.

```sh
# Check one port.
./bin/portcheck 8080

# Check several ports in the order supplied.
./bin/portcheck 3000 8080 5432

# Repeated ports are checked once, at their first occurrence.
./bin/portcheck 8080 3000 8080

# Print status tokens only, without the table header.
./bin/portcheck --quiet 3000 8080

# Options can also appear after ports.
./bin/portcheck 8080 -q
```

Quiet output uses one `FREE`, `IN_USE`, or `ERROR` token per unique port:

```text
FREE
IN_USE
```

| Option | Purpose |
| --- | --- |
| `-h`, `--help` | Show usage and exit successfully |
| `-v`, `--version` | Show version and exit successfully |
| `-q`, `--quiet` | Print only status tokens |
| `--json` | Print an ordered JSON array; cannot combine with quiet mode |
| `--find` | Find the first free port, starting at one supplied port |
| `--host <host>` | Select the local bind address (default `127.0.0.1`) |
| `--watch` | Emit initial states and subsequent changes until interrupted |
| `--wait` | Wait until all requested ports are free |
| `--wait-in-use` | Wait until all requested ports are occupied |
| `--interval <duration>` | Watch/wait polling interval (default `1s`, minimum `100ms`) |
| `--timeout <duration>` | Positive wait timeout (default: no timeout) |
| `--` | End option parsing |

### Check port ranges

```sh
# Include every port from 3000 through 3010.
./bin/portcheck 3000-3010

# Mix individual ports and ranges.
./bin/portcheck 3000 8080-8083 5432

# Overlapping ranges still check each port only once.
./bin/portcheck 3000-3003 3002-3005
```

Both range endpoints are inclusive. Expanded ports retain their first
occurrence in the input; Portcheck does not sort them. `3000-3000` is valid.
Reversed or malformed ranges such as `3010-3000` and `3000-` are rejected
before any checks run. The maximum unique set is `1-65535`, checked
sequentially without spawning a worker pool.

### Find an available port

```sh
./bin/portcheck --find 3000
```

If 3000 and 3001 are occupied but 3002 is available, stdout contains:

```text
3002
```

Search starts at the supplied port and stops at the first free port or 65535.
It never wraps to port 1. Exactly one single starting port is required;
`--find 3000 4000` and `--find 3000-3010` are invalid, even for a one-port range.
`--find --quiet 3000` also prints the selected **port number**, not `FREE`.
System errors stop the search rather than being treated as occupied ports.

When no free port remains, stdout is empty, stderr explains the exhausted
search, and the exit code is `1`. A found port is not reserved.

### Get JSON output

```sh
./bin/portcheck --json 3000 8080-8082
./bin/portcheck --find --json 3000
```

JSON always uses an array, including one-port checks and successful find
results. Example (formatted here for readability):

```json
[
  {"port": 3000, "status": "free"},
  {"port": 8080, "status": "in_use"}
]
```

The schema is `port` (integer), `status` (`free`, `in_use`, or `error`), and an
optional `error` string for failed checks. Successful rows omit `error`.
For example:

```json
[{"port": 8080, "status": "error", "error": "check 127.0.0.1:8080: permission denied"}]
```

JSON comes from the same checks as human output and preserves the same
deduplicated order. Diagnostics stay on stderr; stdout contains only JSON.
JSON mode preserves exit codes—an occupied port still produces exit `1`.
`--json --quiet` is rejected with exit `2`. An exhausted find emits no JSON;
a system failure during JSON find emits the failed result and exits `3`.

### Select a local host/address

```sh
./bin/portcheck --host 127.0.0.1 8080
./bin/portcheck --host 0.0.0.0 8080
./bin/portcheck --host ::1 8080
./bin/portcheck --host localhost --find 3000
```

`--host` applies to every check, including find, watch, and wait. Supply an
unbracketed IPv4/IPv6 address (IPv6 zones such as `fe80::1%eth0` are accepted)
or an ASCII DNS hostname. Empty values, embedded ports, malformed IPs, and
invalid hostname syntax are rejected with exit `2`, without binding or DNS
lookups. Internationalized names must use their ASCII/punycode form.

For a hostname, Go resolves it during binding and selects **one** address;
Portcheck does not check all resolved addresses or silently fall back to
loopback. `localhost` is not guaranteed to select IPv4; use a literal IP when
the address family matters. Resolution failures, unavailable local addresses,
and unsupported IPv6 are system errors (exit `3`), not occupied ports.

This is still a local **bind**, not a connection test. A remote-only address
normally cannot be bound. Wildcards (`0.0.0.0` and `::`) select wildcard bind
semantics; IPv6 dual-stack behavior and conflicts depend on the OS. A free
`127.0.0.1` port does not prove that the same wildcard or IPv6 port is free.
One-shot human output and JSON retain their existing schemas; the selected
host is supplied by the invocation, not an added JSON field.

### Watch state changes

```sh
./bin/portcheck --watch 8080
./bin/portcheck --host ::1 --watch --interval 2s 3000-3003
```

Watch immediately emits each unique port's initial state, then only changes:

```text
2026-10-01 15:04:05  127.0.0.1  8080 IN USE
2026-10-01 15:04:07  127.0.0.1  8080 FREE
```

Human events contain a local-time timestamp (`YYYY-MM-DD HH:MM:SS`), requested
host, port, and status. Each port keeps its own previous state, in input order.
An error's appearance, changed diagnostic, or recovery is also a state change.
Watch continues after check errors; unchanged errors do not spam stderr.
Diagnostics stay on stderr and events on stdout. Output-write failures stop
immediately with exit `3`.

`--quiet` emits only the existing `FREE`, `IN_USE`, or `ERROR` tokens for initial
states and changes (without host, port, or timestamp). For multiple ports this
token stream does not identify the port; use normal output when identity matters.
**`--watch --json` is rejected with exit `2`**: JSON remains a final array, not
a streaming/NDJSON interface in this release.

Ctrl+C (`SIGINT`) and `SIGTERM` stop watch cleanly without a cancellation error.
The exit code is `0`, or `3` if any check failed during the watch, even if it
later recovered. No watch timeout is supported.

### Wait for a desired state

```sh
./bin/portcheck --wait 8080
./bin/portcheck --wait-in-use 8080
./bin/portcheck --wait --interval 500ms --timeout 30s 3000-3003
./bin/portcheck --host ::1 --wait-in-use --json 8080
```

Wait checks immediately, then polls until **all** requested ports match in
the same cycle. A match in an earlier cycle is not remembered as success.
Checks are sequential observations, not an atomic snapshot or port reservation.
Only the final results are printed, in the normal table, quiet tokens, or JSON
array. Both successful wait modes exit `0`, including when the requested state
is `IN USE`. No progress banners or intermediate results are printed.

A system error finishes the current cycle, emits its results/diagnostics, and
stops with exit `3`; unknown errors are not retried forever. A timeout exits
`1`, with a diagnostic on stderr and **no stdout**. Ctrl+C or `SIGTERM` cancels
wait cleanly with exit `1` and no cancellation diagnostic or stdout.

Cancellation observed before final output takes precedence over successful
wait completion. Once final output starts, it finishes (including the JSON
array) and successful completion exits `0`. If a genuine check failure races
with cancellation, its diagnostic and exit `3` are preserved; context-only
cancellation is not a system failure. An interrupted cycle emits no final
wait results, even when it also contains a system failure.

Watch/wait default to a `1s` polling interval; `--interval` accepts Go durations
such as `100ms`, `500ms`, and `2s`, with a minimum of `100ms`. `--timeout` must
be positive (`500ms`, `30s`, `5m`); omit it for an unbounded wait. Polling uses
a cancellation-aware ticker, with sequential checks and no worker pool.
Long cycles can take longer than the requested interval. Transitions between
observations may be missed; wait-in-use indicates bind conflict, not service
readiness or application health.

`--find`, `--watch`, `--wait`, and `--wait-in-use` are mutually exclusive.
`--interval` requires watch or either wait mode. `--timeout` requires a wait
mode. `--quiet --json` remains invalid. These conflicts fail before networking.

### Try it against a real listener

In **terminal 1**, start a local HTTP server (Python 3 required):

```sh
python3 -m http.server 8080 --bind 127.0.0.1
```

If the server cannot bind, pick another free port and use it in both terminals.
In **terminal 2**:

```sh
./bin/portcheck 8080
echo $?
```

Expected while the server is running:

```text
PORT    STATUS
8080    IN USE
1
```

Stop the server with **Ctrl+C** in terminal 1, then repeat the check.
Expected: `FREE` and exit code `0`, unless another process acquires the port.

### Check invalid input

```sh
./bin/portcheck 70000
echo $?
```

Expected:

```text
error: invalid port "70000": must be between 1 and 65535
2
```

Individual ports and range endpoints must contain only decimal digits and
be between **1 and 65535**. Leading zeroes are allowed; whitespace, signs,
and partial numbers are rejected. All arguments are validated before any
network operation.

### Use it in a shell script

```sh
if ./bin/portcheck --quiet 8080 > /dev/null; then
    echo "Port 8080 is available."
else
    status=$?
    case "$status" in
        1) echo "Port 8080 is already in use." ;;
        2) echo "Check the command arguments." >&2; exit "$status" ;;
        3) echo "The check failed; see the diagnostic above." >&2; exit "$status" ;;
        *) echo "Unexpected exit code: $status" >&2; exit "$status" ;;
    esac
fi
```

This is a check, **not a reservation**: a different process may take the port
before your application starts.

## Exit codes

| Code | Meaning |
| --- | --- |
| 0 | All ports free, find succeeds, wait condition satisfied, clean watch stop, or help/version |
| 1 | Ports in use, find exhausted, wait timeout/cancellation, or one-shot cancellation between checks |
| 2 | Invalid CLI input, including missing ports |
| 3 | System error, including failed output writes |

Results go to stdout; diagnostics go to stderr. System errors take precedence
over occupied-port results and do not prevent subsequent ports from being
checked. Affected rows contain `ERROR`.

Use the binary directly in scripts: `go run` and Make do **not** preserve
the application's exact nonzero exit code. `make run` also adds development
messages, so it is not a machine-readable output interface.

## What a check means

Portcheck attempts a TCP bind to the selected local host (default
`127.0.0.1:<port>`), then immediately closes
the listener. Only an address-in-use error becomes `IN USE`; permission and
other failures remain errors. A `FREE` result is an observation, not a port
reservation: another process can acquire the port immediately afterward.

Some operating systems restrict low ports (commonly below 1024). A large
range such as `1-65535` may therefore include permission errors and exit `3`
under a normal user. That is not evidence that those ports are occupied;
Portcheck does not request elevated privileges.

This version is TCP-only. It does not inspect every interface, identify
processes, check UDP, test remote connectivity, or scan networks.
The checker uses portable Go networking APIs. Linux is runtime-tested;
Linux, macOS, and Windows compile for amd64 and arm64. Successful compilation
alone is not a claim of functional support on an untested OS.

## Go library

The public package lives at the module root:
`github.com/aman-void/portcheck`. Once the version is published, add it to
your Go project with `go get github.com/aman-void/portcheck@v0.3.0`.
For unpublished local changes, use a local `replace` directive pointing to
this checkout instead of expecting the remote version to exist.

```go
package main

import (
    "context"
    "errors"
    "fmt"
    "log"

    "github.com/aman-void/portcheck"
)

func main() {
    ctx := context.Background()
    result, err := portcheck.Check(ctx, 8080)
    if err != nil {
        if errors.Is(err, portcheck.ErrInvalidPort) {
            log.Fatal("port must be between 1 and 65535")
        }
        log.Fatal(err)
    }
    fmt.Printf("%d: %s\n", result.Port, result.Status)

    results, err := portcheck.CheckPorts(ctx, []int{3000, 8080, 3000})
    for _, result := range results {
        fmt.Printf("%d: %s\n", result.Port, result.Status)
        if result.Err != nil {
            log.Printf("check failed: %v", result.Err)
        }
    }
    if err != nil {
        log.Printf("one or more checks failed: %v", err)
    }
}
```

- `Check(ctx, port) (Result, error)` returns `StatusFree`, `StatusInUse`, or
  `StatusError`. An occupied port is a normal result, **not** a library error.
- On failure, `Check` returns the same error in `Result.Err` and its error
  return value. Use `errors.Is(err, portcheck.ErrInvalidPort)` for validation
  failures; wrapped system errors retain their causes.
- `CheckPorts(ctx, ports) ([]Result, error)` validates all ports before binding,
  preserves order, and removes duplicates without modifying the input slice.
  Empty input with an active context returns an empty slice and no error.
- System failures appear in each affected `Result.Err` and are joined in the
  batch error; checks for other ports continue. Cancellation stops the batch
  with partial results and an error matching the context error.
- Supply a non-nil context. Cancellation/deadlines are checked between
  operations and passed to the listener; they do not reserve ports or promise
  to interrupt every operating-system socket call instantaneously.
- `CheckHost(ctx, host, port)` and `CheckPortsHost(ctx, host, ports)` add explicit
  host selection with the same result and batch semantics. Existing `Check`
  and `CheckPorts` remain compatible and default to `127.0.0.1`.
- If a system failure races with cancellation, the error retains both causes;
  inspect results and use `errors.Is`/`errors.As` rather than assuming that an
  error matching cancellation contains no system failure.
- `ValidateHost(host)` performs syntax-only validation, with no DNS/networking.
  Use `errors.Is(err, portcheck.ErrInvalidHost)` for invalid syntax. Valid syntax
  does not guarantee successful resolution or a local bind.
- `Result` and the one-shot JSON schema are unchanged; host-aware callers know
  the requested host from their input. For example:

  ```go
  result, err := portcheck.CheckHost(ctx, "::1", 8080)
  results, batchErr := portcheck.CheckPortsHost(ctx, "127.0.0.1", []int{3000, 8080})
  // Inspect each result and the returned errors, as with Check/CheckPorts.
  ```

- There is no public range parser, find, watch, or wait API; those are CLI concerns.

## Development

Run `make` or `make help` for the command menu.

| Command | Action |
| --- | --- |
| `make fmt` | Format Go source |
| `make test` | Run all tests |
| `make vet` | Run Go static checks |
| `make build` | Build `bin/portcheck` |
| `make run ARGS="--help"` | Run the CLI from source |
| `make clean` | Remove only known binary outputs |

Before finishing a change:

```sh
make fmt test vet build
```

Targets print progress and success messages without hiding tool diagnostics.
Success messages appear only after the command succeeds. Color is automatic
in terminals and disabled for redirected output or `TERM=dumb`.

```sh
NO_COLOR=1 make build  # Plain output.
make help COLOR=1     # Force ANSI colors.
make help COLOR=0     # Disable ANSI colors.
```

`NO_COLOR=1` takes precedence over `COLOR=1`. These colors belong to the
Makefile only; the Portcheck CLI output stays plain and script-friendly.

Direct verification:

```sh
gofmt -l .
go test ./...
go vet ./...
go build ./cmd/portcheck
go list -m all
```

Focused tests:

```sh
go test ./internal/checker -run '^TestCheckListenerLifecycle$' -count=1
go test ./internal/cli -run '^TestRunRealListener$' -count=1
go test . -run '^TestCheckRealListener$' -count=1
go test ./internal/cli -run '^TestRangeAndFindRealListeners$' -count=1
go test . -run '^TestCheckHostRealListeners$' -count=1
go test ./internal/cli -run '^TestWaitReal' -count=1
```

Tests allocate local TCP listeners dynamically; they need loopback socket
access but no internet services or root privileges.

Range-expansion benchmarks (including the full legal range):

```sh
go test ./internal/cli -run '^$' -bench '^BenchmarkRangeExpansion$' -benchmem
```

### Architecture

`cmd/portcheck` handles signals and process exit; `internal/cli` owns parsing,
find, watch/wait orchestration, output, and exit-code selection. The CLI calls the public root package,
which owns library validation and batch semantics; `internal/checker` owns
binding and error classification. The library never imports the CLI.
The checker remains stateless. Tests use internal function parameters to control failures and timing,
without mutable global hooks. Plans live under `plans/` and describe future
scope, not currently available features. Each version requires human review
before the next begins.

### Parsing details

Unknown options are rejected, including when help/version is requested.
Help takes precedence over version; both bypass positional port validation
and perform no network checks. Combined short flags and `--quiet=true`
syntax are not supported.

## License

MIT; see [LICENSE](LICENSE).
