# Portcheck

A tiny Go CLI for local TCP availability and explicit TCP connectivity tests.

Check ports and ranges, find an available port, watch state changes, or wait
for a desired state. Optionally identify visible Linux listening processes.
Test TCP connectivity with `--connect`, or combine factual local observations
with `doctor`. Consume results through JSON and a small Go API.

```sh
curl -fsSL https://github.com/aman-void/portcheck/releases/latest/download/install.sh | sh
```

It uses only Go's standard library, defaults to IPv4 loopback
(`127.0.0.1`), and supports explicit local IPv4/IPv6 addresses and hostnames.

The current release candidate is **`v1.0.0-rc.2`**. Stable `v1.0.0` is not
released yet. Untagged source builds report `1.0.0-dev`; a build from a tagged
commit reports that tag, and release builds inject their version. Final `v1.0.0`
publication is pending candidate validation and owner review.
See the [compatibility/API/JSON contract](docs/contracts.md),
[platform matrix](docs/platforms.md), and [release checklist](docs/releasing.md).

```text
PORT    STATUS
3000    FREE
8080    IN USE
```

## Small by design

| | |
| --- | --- |
| Download | 1.26–1.42 MB per archive |
| Installed binary | 3.05–3.29 MB per executable |
| Runtime dependencies | none, on every platform |
| Third-party Go modules | zero, so there is no `go.sum` to audit |
| Non-test source | ~2,000 lines of Go across 23 files |
| Installer | 3.7 KB of POSIX shell |
| Startup | no config file, no daemon, no database |

Every feature is standard library only: `net`, `net/netip`, `os/signal`,
`encoding/json`, and `runtime/debug`. Nothing is fetched at runtime. Because
there is no dependency graph, there is no supply-chain surface to patch, and
`go list -m all` prints exactly one line.

Binary size is dominated by the Go runtime, not by this program; the compiled
code is a small part of it. The installer is under 4 KB and verifies SHA-256
before installing anything.

Those figures are measured, not estimated. Every `make release` prints the
actual byte size of each executable and archive for all six targets, so the
ranges above can be checked against real artifacts:

```text
TARGET                       BINARY      ARCHIVE
---------------------- ------------ ------------
linux/amd64                 3383456      1464082
linux/arm64                 3276960      1320014
darwin/amd64                3422032      1469920
darwin/arm64                3246594      1356146
windows/amd64               3453440      1484727
windows/arm64               3202048      1327165
```

An archive is larger than the executable it contains because it also carries
`LICENSE` and `BUILDINFO.txt`.

## Quick start

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

The version command prints `portcheck version 1.0.0-dev`. Port statuses depend
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

### Install a published version

**Prebuilt binary, no Go required.** This is the recommended path. The installer
picks the right archive for your platform, verifies its SHA-256 against the
published `SHA256SUMS`, and installs to `~/.local/bin` without needing `sudo`:

```sh
curl -fsSL https://github.com/aman-void/portcheck/releases/latest/download/install.sh | sh
portcheck --version
```

Pin a specific version instead of tracking the latest. Both `v1.0.0` and
`1.0.0` work:

```sh
PORTCHECK_VERSION=v1.0.0 \
  curl -fsSL https://github.com/aman-void/portcheck/releases/latest/download/install.sh | sh
```

GitHub excludes prereleases from `releases/latest`, so pin the tag explicitly
to install a release candidate:

```sh
PORTCHECK_VERSION=v1.0.0-rc.2 \
  curl -fsSL https://github.com/aman-void/portcheck/releases/download/v1.0.0-rc.2/install.sh | sh
```

The installer needs `curl`, `tar` (or `unzip` on Windows), and a POSIX shell; it
refuses to install anything whose checksum does not match. Two optional variables:

| Variable | Default | Purpose |
| --- | --- | --- |
| `PORTCHECK_VERSION` | latest stable release | Install a specific version; accepts `v1.0.0` or `1.0.0` |
| `PORTCHECK_INSTALL` | `$HOME/.local/bin` | Install elsewhere, e.g. a system-wide directory |

If `$HOME/.local/bin` is not on your `PATH`, the installer prints the line to
add. It never uses `sudo`; installing system-wide is an explicit choice:

```sh
curl -fsSL https://github.com/aman-void/portcheck/releases/latest/download/install.sh \
  | PORTCHECK_INSTALL=/usr/local/bin sudo sh
```

**From source**, if you already have Go 1.27.1 or newer:

```sh
go install github.com/aman-void/portcheck/cmd/portcheck@latest
```

Versioned Go installations read their version from Go build information. An
untagged checkout builds report `1.0.0-dev`; a checkout sitting on a tag
reports that tag, and linker-injected versions take precedence over both.

### Install a release binary manually

If you would rather download and unpack an archive yourself, get the archive for
your platform and `SHA256SUMS` from
[GitHub Releases](https://github.com/aman-void/portcheck/releases). When available,
an example Linux amd64 installation is:

```sh
# Run in the directory containing the downloaded files.
sha256sum --ignore-missing -c SHA256SUMS
tar -xzf portcheck_1.0.0_linux_amd64.tar.gz
mkdir -p "$HOME/.local/bin"
install -m 755 portcheck_1.0.0_linux_amd64/portcheck "$HOME/.local/bin/portcheck"
portcheck --version
```

Add `$HOME/.local/bin` to `PATH`. On macOS, compare `shasum -a 256 <archive>`
with its entry in `SHA256SUMS`. On Windows, use `Get-FileHash <archive> -Algorithm SHA256`,
then `Expand-Archive` and place `portcheck.exe` in a directory on `PATH`.
Use the actual candidate version in filenames when installing an RC. Archives
include the binary, MIT license, and build provenance. See the platform matrix
before treating an architecture as runtime-validated.

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
| `--process` | Identify visible local listening processes (Linux only) |
| `--connect <host:port>` | Test TCP establishment; no application payload |
| `doctor <port>` | Combine local bind, process inspection, and connectivity |
| `--watch` | Emit initial states and subsequent changes until interrupted |
| `--wait` | Wait until all requested ports are free |
| `--wait-in-use` | Wait until all requested ports are occupied |
| `--interval <duration>` | Watch/wait polling interval (default `1s`, minimum `100ms`) |
| `--timeout <duration>` | Positive timeout (wait: unbounded; connect dial/whole doctor run: `5s`) |
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

When `--host` is followed by a long flag, its value is treated as omitted and
the default `127.0.0.1` is used: `--host --quiet 8080` and
`--host --json 8080` both check port 8080 on the default host. Short aliases are
not covered — `--host -q 8080` remains a missing-host error. Supply a value with
`--host 127.0.0.1 --json 8080` when an explicit host is intended.

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
On a transition from `FREE` or `ERROR` to `IN USE`, human watch output appends
`observed since HH:MM:SS`, identifying when that busy state was first observed,
**not the actual time the port became busy**. The marker is not
repeated for unchanged polls or process-only changes. `FREE` and `ERROR` reset
the observation. An initially occupied port has no known transition and retains
its existing initial output. Quiet watch and all other modes are unchanged.
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

`--find`, `--watch`, `--wait`, `--wait-in-use`, `--connect`, and `doctor` are
mutually exclusive. `--interval` requires watch or either wait mode.
`--timeout` requires wait, connect, or doctor. `--quiet --json` remains invalid.
These conflicts fail before networking.

### Inspect listening processes

```sh
./bin/portcheck --process 8080
./bin/portcheck --process 3000 8080-8083
./bin/portcheck --process --json 8080
./bin/portcheck --process --watch 8080
./bin/portcheck --process --wait-in-use --timeout 30s 8080
```

Process inspection is optional and read-only. Only occupied ports are inspected;
free ports and bind errors have no process details. Without `--process`, no
process files are read and existing output is unchanged. Multiple visible owners
are supported, deduplicated by PID, and displayed in ascending PID order under
their port, while port order and first-occurrence deduplication remain unchanged:

```text
PORT    STATUS
8080    IN USE
        PID: 18242  NAME: api  EXECUTABLE: /usr/local/bin/api
```

Process-aware JSON is still an array. Occupied rows may add `processes`, with an
integer `pid` and optional string `name` and `executable`. Absent metadata is
omitted; the PID alone can identify a visible owner. A lookup failure adds the
string `process_error` instead of changing the bind `status` or its `error` field:

```json
[
  {"port": 8080, "status": "in_use", "processes": [{"pid": 18242, "name": "api", "executable": "/usr/local/bin/api"}]},
  {"port": 8081, "status": "in_use", "process_error": "process inspection permission denied"},
  {"port": 8082, "status": "free"}
]
```

Process failures also appear as `warning:` diagnostics on stderr and as
`PROCESS unavailable:` in human output. **They do not alter exit codes**: an
occupied one-shot check still exits `1`, satisfied wait-in-use exits `0`, and
watch interruption exits `0` unless a bind or output error occurred. Quiet mode
still prints only status tokens, though lookup warnings remain on stderr.

Process-aware watch inspects each occupied observation and emits changes in
owner PID sets, names, executable paths, or lookup errors, even if the port stays
occupied. Identical observations and repeated warnings are suppressed. Quiet
watch can therefore repeat `IN_USE` for an ownership change. No caching is used.
Wait inspects only occupied rows of the final completed cycle; ownership is not
a wait condition. `--find --process` performs no process lookups and preserves
find's numeric/JSON output. `--watch --json` remains invalid.

#### Platform support and limitations

- **Linux:** runtime-tested without root. Reads listening socket tables from
  `/proc/net/tcp` and `/proc/net/tcp6`, maps socket inodes through visible
  `/proc/<pid>/fd` entries, and reads only owner metadata (`stat`, `comm`, `exe`).
  It does not run `ss`, `lsof`, or any other command. Process names and paths are
  sanitized for terminal output; JSON preserves the original escaped strings.
- **macOS, Windows, other platforms:** process inspection is explicitly
  unsupported. An occupied `--process` result reports that limitation,
  while normal checks and free-port checks still work. Cross-compilation does
  not establish native runtime support.
- Permissions, `hidepid`, containers, and PID/network namespaces can hide owners.
  Only discovered visible owners are returned; this is not a completeness
  guarantee. If none can be identified, the port remains `IN USE` with a lookup
  warning. No privilege escalation, process control, or indefinite retry occurs.
- Inspection follows the requested local address within its address family,
  including same-family wildcard conflicts. `/proc` lacks the IPv6 socket's
  `IPV6_V6ONLY` setting, so cross-family wildcard conflicts are not attributed;
  e.g. an IPv4 bind blocked solely by a dual-stack IPv6 listener can report
  process unavailable. Go's generic TCP wildcard listener may use an IPv6
  dual-stack socket even when configured with `0.0.0.0`; use `--host ::` to inspect
  those IPv6 listeners. Scoped IPv6 inspection is explicitly unavailable because
  these tables lack interface scope. Bind checks themselves remain supported.
- Hostnames are independently resolved using Go's IPv4 preference; DNS can change
  between binding and inspection. Use literal addresses when family matters.
  Socket/PID identity is rechecked, but all results remain live, non-atomic
  observations. A bind conflict need not be a listening socket (e.g. an active
  connection or bound-but-not-listening socket), so ownership may be unavailable.
- Scanning `/proc` costs more than binding; large occupied ranges and watch cycles
  may take longer than the requested interval. Inspection remains sequential.

The existing bind API and `portcheck.Result` are unchanged. Process inspection and
the combined CLI records remain internal; no public process API is exposed.

### Test TCP connectivity

```sh
./bin/portcheck --connect localhost:8080
./bin/portcheck --connect 127.0.0.1:8080 --quiet
./bin/portcheck --connect '[::1]:8080' --timeout 2s --json
```

`--connect` is a **dial**, not a local availability check. It accepts exactly
one endpoint: an ASCII DNS hostname or IP address and a decimal port in
`1-65535`. IPv6 endpoints require brackets; zones are accepted, e.g.
`[fe80::1%eth0]:8080`. Empty hosts, service names, URLs, ranges, embedded
whitespace, and malformed endpoints are rejected before any networking.

Go's `net.Dialer.DialContext` handles hostname resolution and IPv4/IPv6.
There is one user-directed dial operation, with no Portcheck retries or scanning;
Go may try multiple resolved addresses as part of its normal dialing behavior.
The timeout covers resolution and TCP establishment, defaults to **5 seconds**,
and must be positive. An earlier caller deadline wins. Successful connections
are closed immediately, with **no application data sent**. Normal bind checks
do not dial. Connectivity does not inspect local or remote processes.

| Status | Observation | Exit |
| --- | --- | --- |
| `REACHABLE` | TCP establishment succeeded and connection cleanup succeeded | `0` |
| `REFUSED` | The connection attempt was explicitly refused | `1` |
| `TIMEOUT` | The attempt timed out or its context deadline expired | `1` |
| `ERROR` | Other failure, such as DNS, network, cancellation, or cleanup error | `3` |

Human output contains address, status, and elapsed dial time in milliseconds.
Quiet output is exactly one `REACHABLE`, `REFUSED`, `TIMEOUT`, or `ERROR` token.
Diagnostics go to stderr. Ctrl+C/SIGTERM cancels the active dial and produces
`ERROR` with exit `3`; a completed dial is not reclassified by a later signal.
These new-mode cancellation semantics do not change existing watch/wait behavior.

Connectivity JSON is a **single-element array**, preserving the repository's
array convention (not an object or NDJSON):

```json
[{"address":"localhost:8080","status":"reachable","duration_ms":0.42}]
```

Fields are `address` (the supplied string), `status` (lowercase), and
`duration_ms` (a nonnegative fractional number). Failures add `error` with
human-readable diagnostic text, including refusal and timeout. Dial duration
includes resolution but not validation or cleanup; a pre-canceled operation
has duration `0`. Do not match OS-specific error wording in scripts.

`--connect` rejects positional ports, repeated `--connect`, `--host`,
`--process`, `--interval`, and other operation modes. `--quiet --json` remains
invalid. There is no connectivity watch/wait mode.

Known flags may intervene before the endpoint, including
`--connect --quiet localhost:8080`, `--connect --json localhost:8080`, and
`--connect --timeout 1s localhost:8080`. The next non-option endpoint supplies
the deferred value; timeout and interval values still immediately follow their
own flags. Unknown options and missing endpoints remain errors.

### Diagnose a local endpoint

```sh
./bin/portcheck doctor 8080
./bin/portcheck doctor --host ::1 8080 --timeout 2s
./bin/portcheck doctor 8080 --json
```

Doctor accepts exactly **one single port**, not a range. It sequentially:

1. Checks local binding on `--host` (default `127.0.0.1`).
2. Inspects visible processes automatically, only if the bind result is `IN USE`.
3. Tests TCP connectivity to that same host and port.

The report separates endpoint (`host`, `port`, TCP), local bind status,
visible owners or unavailable inspection, and connectivity status/latency.
`IN USE` plus `REACHABLE`, or `FREE` plus `REFUSED`, are both valid.
Process failures are warnings and never overwrite either network observation.
Bind errors are reported separately; doctor still attempts the requested dial.
Free ports are not inspected, and a successful bind does not prove that no
process exists elsewhere. Observations may change between stages.

Doctor uses **one finite timeout budget for the complete operation**: local
binding (including resolution), process inspection (including resolution), and
dialing. The default is `5s`; `--timeout` overrides it. The connector uses the
remaining budget rather than getting an extra timeout after inspection. On
expiry, stderr explicitly reports a doctor timeout; completed observations
remain intact, and unfinished stages report their own deadline errors. As with
existing cancellation support, checks stop between OS calls; Go cannot promise
to interrupt every synchronous filesystem/socket call instantaneously.
Doctor uses the existing Linux-only inspector and its permission,
namespace, scoped-address, and dual-stack limitations. `--process` is accepted
but redundant. Quiet, find, watch, wait, wait-in-use, connect, and interval
combinations are rejected. `--host` retains local bind semantics; a remote-only
host can yield a local bind error even if the dial succeeds. Wildcard hosts
retain OS-defined bind/dial behavior; prefer a literal loopback address for
an unambiguous local connectivity target.

Doctor exits `0` for reachable connectivity, `1` for refusal/timeout, and `3`
for any local bind or connectivity system error (including cancellation).
Process lookup warnings alone do not change the exit. Invalid input exits `2`.
Human reports include errors in their corresponding sections and diagnostics
on stderr. JSON stdout is a single-element array:

```json
[{
  "endpoint":{"host":"127.0.0.1","port":8080,"protocol":"tcp"},
  "local":{"status":"in_use"},
  "processes":[{"pid":18242,"name":"api","executable":"/usr/local/bin/api"}],
  "connectivity":{"address":"127.0.0.1:8080","status":"reachable","duration_ms":0.42}
}]
```

`local.status` uses `free`, `in_use`, or `error`; `local.error` appears on bind
failure. `processes` is an optional PID-sorted array with the existing process
schema, supporting multiple owners. A lookup failure adds `process_error`;
free/error local rows omit process fields. `connectivity` uses the exact fields
described above. Optional errors and absent owners are omitted, not `null`.

TCP establishment does **not** prove application health, HTTP health, TLS
correctness, firewall configuration, or packet routing. Portcheck does not
test UDP, identify remote processes, send protocol probes, or control services.

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
| 0 | All ports free, find succeeds, wait satisfied, clean watch stop, connect/doctor reachable, or help/version |
| 1 | Ports in use, find exhausted, wait timeout/cancellation, one-shot cancellation between checks, or connect/doctor refused/timeout |
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

This version is TCP-only. It does not inspect every interface, check UDP,
control processes, or scan networks. Remote connectivity is tested only when
explicitly requested with `--connect` (or the host selected for doctor).
The checker uses portable Go networking APIs. Linux is runtime-tested;
Linux, macOS, and Windows compile for amd64 and arm64. Successful compilation
alone is not a claim of functional support on an untested OS.

## Go library

The public package lives at the module root:
`github.com/aman-void/portcheck`. Once the version is published, add it to
your Go project with `go get github.com/aman-void/portcheck@v1.0.0`.
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

#### Connectivity API

```go
result, err := portcheck.Connect(ctx, "127.0.0.1:8080", portcheck.DefaultConnectTimeout)
switch result.Status {
case portcheck.ConnectivityReachable:
    // TCP establishment succeeded; no application health claim.
case portcheck.ConnectivityRefused, portcheck.ConnectivityTimeout:
    // Expected negative observations; err retains diagnostic details.
case portcheck.ConnectivityError:
    // Inspect err with errors.Is/errors.As.
}
_ = err
```

`Connect(ctx, address, timeout) (ConnectivityResult, error)` takes a non-nil
context and a positive explicit timeout. `DefaultConnectTimeout` is `5s`.
The result has `Address`, `Status`, `Duration`, and `Err`; every unsuccessful
operation returns the same error in `Err` and the error return, **including
refusal and timeout**. This differs from a normal occupied bind result, which
has no library error. Wrapped causes remain inspectable; classification uses
typed errors, not text. `ValidateEndpoint(address)` validates syntax without
DNS/networking; invalid endpoints wrap `ErrInvalidEndpoint` (and invalid
numeric port bounds also wrap `ErrInvalidPort`). Nonpositive timeouts wrap
`ErrInvalidTimeout`. Cancellation yields `ConnectivityError`; deadlines yield
`ConnectivityTimeout`. There is no public doctor or process API.

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
| `make build VERSION=1.0.0-rc.2` | Inject a candidate version into `bin/portcheck` |
| `make release VERSION=1.0.0-rc.2 OUT=dist/rc.2` | Build six archives and `SHA256SUMS` from a clean checkout |

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
go test -race ./...
go vet ./...
go build ./cmd/portcheck
go build ./...
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
go test ./internal/process -count=1
go test ./internal/cli -run '^TestProcess' -count=1
go test . -run '^TestConnect' -count=1
go test ./internal/cli -run '^Test(Connect|Doctor)' -count=1
```

Tests allocate local TCP listeners dynamically; they need loopback socket
access but no internet services or root privileges.

Pure parsing and orchestration/formatting benchmarks (including the full legal range):

```sh
go test ./internal/cli -run '^$' -bench '^BenchmarkRangeExpansion$' -benchmem
go test ./internal/cli -run '^$' -bench '^BenchmarkCLIContracts$' -benchmem
```

### Architecture

`cmd/portcheck` handles signals and process exit; `internal/cli` owns parsing,
find, watch/wait orchestration, output, and exit-code selection. The CLI calls the public root package,
which owns library validation and batch semantics; `internal/checker` owns
binding and error classification. `internal/process` independently provides
optional platform-specific ownership inspection, composed by the CLI.
The root `connectivity.go` owns endpoint validation, dialing, classification,
and structured results independently of bind checking. CLI doctor orchestration
composes the public bind/connect APIs and existing process inspector.
The library never imports the CLI or process inspector.
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
