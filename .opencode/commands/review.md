---
description: Review Portcheck changes against repo contracts
---

Review this change for bugs and contract violations.

!`git status --short && echo "--- STAGED ---" && git diff --cached && echo "--- UNSTAGED ---" && git diff && echo "--- UNTRACKED ---" && git ls-files --others --exclude-standard`

Target if given: $ARGUMENTS

If the diff is empty, review the most recent commit instead: !`git show --stat HEAD && git show HEAD`

---

## Project

Portcheck is a stdlib-only Go TCP utility. Module `github.com/aman-void/portcheck`.

```
portcheck.go  host.go  connectivity.go     public API (root package)
connectivity_unix.go  connectivity_windows.go  refusal errno per OS
cmd/portcheck/         executable: signals, exit
internal/checker/      bind + status classification
internal/process/      Linux /proc inspection + non-Linux unsupported stub
internal/cli/          parsing, output, find, watch/wait polling, doctor
plans/*.md             specifications — read-only
```

Public API: `Check`, `CheckPorts`, `CheckHost`, `CheckPortsHost`, `Connect`,
`ValidateHost`, `ValidateEndpoint`, `Result`, `Status`, `ConnectivityResult`,
`ConnectivityStatus`, `ErrInvalidPort`, `ErrInvalidHost`, `ErrInvalidEndpoint`,
`ErrInvalidTimeout`, `DefaultConnectTimeout`.

CLI flags: `-h/--help`, `-v/--version`, `-q/--quiet`, `--json`, `--find`,
`--host`, `--process`, `--connect`, `--watch`, `--wait`, `--wait-in-use`,
`--interval`, `--timeout`, `--`, plus the `doctor` subcommand.

Tests must use dynamically allocated local listeners. Root is never available.

---

## Verify before judging

Run these and report real output. Do not assume a prior green state.

```sh
gofmt -l . && go vet ./... && go test ./... -count=1
go test -race ./... -count=1 && go build ./... && go list -m all
```

- `gofmt -l .` must print nothing.
- `go list -m all` must be exactly `github.com/aman-void/portcheck`. Any other
  line is a blocking finding.
- Cross-compile check, which also typechecks test files:
  ```sh
  for os in windows darwin; do for p in . ./internal/cli ./internal/process \
    ./internal/checker ./cmd/portcheck; do
    GOOS=$os go test -c -o /dev/null $p || echo "FAIL $os $p"; done; done
  ```
  `go build` alone does not compile test files. A missing build tag shows up here.

A green suite does not prove the binary is correct. If a claim is about observable
behavior, build it and run it.

---

## Blocking contracts

Any violation here blocks.

**Specifications are read-only during implementation.** Never edit `plans/*.md` as
part of delivering a change. A plan states intended behavior before code exists;
appending implementation notes, validation results, or review status to it makes
the spec dictate the outcome of the review that validates it. Notes belong in the
matching `plans/<version>-report.md`. If a plan is modified, that is the finding.

**Never commit, tag, push, or publish** without explicit owner instruction.
`AGENTS.md` requires it.

**Zero third-party dependencies.** Standard library only. No CLI framework.

**No concurrency.** No goroutines, no worker pools, no channels. All iteration is
sequential. A new goroutine needs explicit justification in the review.

**OS-specific code lives in build-tagged files** — `*_linux.go`, `*_windows.go`,
`*_unix.go`. Flag `runtime.GOOS` branching inside shared files. Each platform
variant must compile; non-Linux process inspection reports `ErrUnsupported`
rather than faking data or shelling out.

**Error classification uses `errors.Is` / `errors.As` only.** Comparing error
strings is a finding. Walk `Unwrap()` leaves when a joined error may hide a system
cause — prior art is `cancellationOnly` in `internal/cli/poll.go`. OS-specific
refusal and in-use errnos differ per platform and need their own handling
(`syscall.ECONNREFUSED`, `syscall.Errno(10061)`, `syscall.EADDRINUSE`,
`syscall.Errno(10048)`).

**JSON is always an array**, in every mode including one-result ones. Optional
fields use `omitempty` and are omitted, never `null`. `processes` is an array
because a port can have multiple owners via `SO_REUSEPORT`.

**Process inspection never changes port status.** A failed or unsupported lookup
yields `IN USE` plus a separate `process_error`. Never `FREE`, never `ERROR`, never
a changed exit code. Doctor keeps local bind, process, and connectivity as three
independent observations for the same reason.

**Public API is a published contract.** `Result` and `ConnectivityResult` field
sets, sentinel error identity, and function signatures are frozen. Adding a field
is a deliberate design decision requiring documentation, not a refactor. CLI-only
features stay out of the library: process inspection, find, watch, wait, and
doctor are not public API.

**Exit codes:** `0` satisfied, `1` condition unmet, `2` invalid input, `3` system
error. System errors outrank occupied-port results. Results go to stdout,
diagnostics to stderr, and JSON stdout must never contain diagnostic text.

**Backward compatibility.** v0.1 through v0.5 output is frozen: the `PORT STATUS`
table, the `IN_USE` quiet token, range inclusivity, first-occurrence ordering with
deduplication, and the `processes` / `process_error` schema. `--find` prints a bare
port number even with `--quiet`.

---

## Bug classes to check

These have all shipped in this repo. Verify each explicitly.

**Flag value parsing.** A value-taking flag must never swallow the following flag,
and a following positional must never be silently consumed as a value. Test every
ordering of `--host`, `--connect`, `--timeout`, and `--interval` with its value on
both sides:

```sh
./bin/portcheck --connect --quiet localhost:8080     # must work
./bin/portcheck --quiet --connect localhost:8080     # must work
./bin/portcheck --host --json 8080                  # 8080 is a port, not a host
./bin/portcheck --host -q 8080                      # missing value, exit 2
```

Short aliases (`-q`, `-h`, `-v`) differ from long forms after `--host`; that
asymmetry is deliberate and documented.

**Test tables hide permutations.** A table that exercises one argument ordering
cannot catch the others — 30 connect tests once passed while the flag-before-value
ordering was broken. When adding a parser rule, add the mirrored case in the same
commit.

**Version metadata.** `internal/cli/version.go` must report `1.0.0-dev` for local
git builds. Go stamps pseudo-versions like `0.0.0-20261002042414-a1b05a4a6bf6+dirty`
into build info; only real `vX.Y.Z` tags may override. Verify by building and
running, not by testing `resolveVersion` with literals:

```sh
go build -o /tmp/pc ./cmd/portcheck && /tmp/pc --version
```

**Cancellation races.** A system failure landing in the same instant as
cancellation must still produce exit 3. Check that the failure is recorded before
any cancellation return, and that `errors.Is` over a joined error still finds the
system cause rather than only the context error.

**Watch change detection.** Watch emits on transitions only. A new marker must not
cause per-interval spam, and `previous` state must update on every emitted change
or the same line repeats forever. Quiet watch output must stay byte-identical.

**Watch timing markers.** A duration computed from the same `now()` seam twice in
one iteration is always zero. Polling can only know *since when it observed* the
state, never the true bind time — label it as an observed lower bound.

**`/proc` races.** Processes vanish mid-scan, PIDs get reused, and sockets close
between read and use. `internal/process` guards with start-time re-reads and fd
re-checks. Preserve those; do not weaken them for readability.

**Address family matching.** `/proc` cannot reveal `IPV6_V6ONLY` or interface
scope. Cross-family conflicts must not be attributed, and scoped IPv6 must report
unavailable rather than guess.

**Endpoint parsing.** IPv6 requires brackets via `net.JoinHostPort`. Never
concatenate host and port manually. Reject empty hosts, embedded ports, service
names, URLs, and whitespace before any networking.

**Doctor budget.** One finite timeout covers the whole run — binding, inspection,
and dialing. A fresh dial budget after inspection silently doubles the wait. On
expiry, completed observations must survive and get an explicit diagnostic.

**Resource leaks.** A dial returning both a connection and an error must still
close the connection. Every acquired listener must be closed. Verify with
`go test -race` and by reading the error paths, not just the happy path.

**Deterministic tests.** No sleeps as primary synchronization. No hardcoded ports.
No public internet, no public DNS, no external commands (`ping`, `nc`, `curl`,
`ss`, `lsof`, `traceroute`). Timeout tests inject a dial function; never rely on an
unroutable IP like `10.255.255.1` to time out.

**Untrusted output.** Process names and executable paths are OS data. Human output
must not emit terminal control codes — `safeText` in `internal/cli/process.go`
handles this. JSON relies on `encoding/json` escaping; do not "fix" that path by
sanitizing, which would corrupt legitimate values.

---

## Reporting

Order by severity: **blocking**, **worth fixing**, **optional**. For each finding
give the exact command that reproduces it, the file and line, and a one-line fix.

Distinguish clearly between a confirmed defect and a suspicion. If you could not
reproduce something, say so. If you are unsure whether an OS behavior holds, say
that rather than asserting.

Do not restate correct code. Do not pad with praise. If nothing is wrong, say that
plainly and list exactly what you verified and how.
