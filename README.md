# Portcheck

A tiny Go CLI that answers one question: **can I bind this local TCP port?**

Check one port or several, get readable output, and use predictable exit
codes in your scripts. **v0.1.0** uses only Go's standard library and checks
IPv4 loopback (`127.0.0.1`).

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

The version command prints `portcheck version 0.1.0`. Port statuses depend
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
| `--` | End option parsing |

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

Ports must contain only decimal digits and be between **1 and 65535**.
Leading zeroes are allowed; whitespace, signs, partial numbers, and ranges
are rejected. All arguments are validated before any network operation.

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
| 0 | All requested ports are free; also help/version success |
| 1 | One or more ports are in use |
| 2 | Invalid CLI input, including missing ports |
| 3 | System error, including failed output writes |

Results go to stdout; diagnostics go to stderr. System errors take precedence
over occupied-port results and do not prevent subsequent ports from being
checked. Affected rows contain `ERROR`.

Use the binary directly in scripts: `go run` and Make do **not** preserve
the application's exact nonzero exit code. `make run` also adds development
messages, so it is not a machine-readable output interface.

## What a check means

Portcheck attempts a TCP bind to `127.0.0.1:<port>`, then immediately closes
the listener. Only an address-in-use error becomes `IN USE`; permission and
other failures remain errors. A `FREE` result is an observation, not a port
reservation: another process can acquire the port immediately afterward.

This version is TCP-only and IPv4-loopback-only. It does not inspect every
interface, identify processes, check UDP, test remote connectivity, or scan
networks. Ranges, JSON, and a public Go API are not part of v0.1.
The checker uses portable Go networking APIs. Linux is runtime-tested;
Linux, macOS, and Windows compile for amd64 and arm64. Successful compilation
alone is not a claim of functional support on an untested OS.

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
```

Tests allocate local TCP listeners dynamically; they need loopback socket
access but no internet services or root privileges.

### Architecture

`cmd/portcheck` handles process exit; `internal/cli` owns parsing, output,
and exit-code selection; `internal/checker` owns binding and error
classification. Tests use internal function parameters to control failures,
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
