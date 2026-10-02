# v1 compatibility contract

This records the accepted v0.5 behavior retained for v1.0. It is a contract,
not permission to implement further roadmap features. Examples and full option
combinations are in [README](../README.md).

## Compatibility table

| Feature | v0.5 behavior | v1.0 candidate |
| --- | --- | --- |
| Single/multiple checks | Sequential local TCP bind; close immediately | Unchanged |
| Default address | `127.0.0.1` | Unchanged |
| Ranges | Inclusive, 1–65535, preserve first occurrence, no sorting | Unchanged |
| Find | One single starting port; upward to 65535; never wrap | Unchanged |
| Find output | Numeric even when quiet; JSON one-element array; exhaustion has no stdout, exit 1 | Unchanged |
| Quiet | `FREE`, `IN_USE`, `ERROR`; one token per result/event | Unchanged |
| JSON | Arrays, no diagnostic text on stdout, no quiet combination | Unchanged |
| Host | Syntax-only validation; one Go-selected bind address; owner-selected long-flag shorthand | Unchanged |
| Watch | Initial state, meaningful status/error/owner changes; human observed-since marker; no JSON | Unchanged |
| Watch errors | Continue checking; suppress unchanged errors; interruption exits 3 if any check failed, otherwise 0 | Unchanged |
| Wait/wait-in-use | Every requested port must match in the same cycle; print final results only | Unchanged |
| Polling | Sequential, 1s default, 100ms minimum; cancellation-aware ticker | Unchanged |
| Wait timeout/cancellation | Exit 1, no stdout; timeout diagnostic only | Unchanged |
| Process | Optional read-only Linux lookup; multiple PID-sorted owners; warnings do not change network status/exit | Unchanged |
| Connect | One user-directed Go TCP dial, no payload/retry/scanning; 5s default timeout | Unchanged |
| Doctor | One port; compose bind, optional owner inspection, dial under one 5s default budget | Unchanged |
| Exit codes | 0 satisfied, 1 negative, 2 invalid, 3 system failure | Unchanged |
| Version | `portcheck version 0.5.0` | Same format; dev/module/linker version source |
| Windows occupation | Actual Winsock bind failures could be misclassified as system errors | Bug fix: wrapped WSAEADDRINUSE is `IN USE`; other errors remain errors |
| Unix broken stdout pipe | Runtime SIGPIPE could bypass documented output-error exit 3 | Bug fix: executable ignores SIGPIPE so writes report exit 3 and stderr diagnostics |

The v1.0 plan's warning against infinite error retries does not silently replace
the established watch contract. Watch is continuous observation, while wait
terminates on unexpected check errors. No new retry/backoff policy is added.

### Completion and cancellation

Wait checks cancellation again before final output. Once output starts it
finishes, so a later cancellation does not truncate a successful JSON response.
An interrupted cycle produces no wait stdout. Genuine system failures retain
exit 3 even if cancellation races with them. Connect cancellation is `ERROR`,
exit 3; deadlines are `TIMEOUT`, exit 1. Doctor bind errors take precedence over
connectivity results; process lookup warnings never do.

### Environment and parsing

No configuration files or Portcheck runtime environment variables are read.
Go's resolver and OS networking/permissions still influence network behavior.
The executable uses stdout/stderr as supplied and never adds terminal colors.
`NO_COLOR`, `COLOR`, `VERSION`, `OUT`, and `ALLOW_DIRTY` affect developer tooling
only. `GOBIN`, `GOPATH`, and `PATH` affect installation/command discovery.
The CLI registers SIGINT/SIGTERM; Windows Unix-style signal delivery is not
claimed. Programmatic API cancellation works via context on all build targets.

Known options can occur before/after positional ports. Help wins over version
and bypasses positional validation, but unknown options and missing flag values
remain invalid. Boolean `--flag=true` and combined short flags are unsupported.
`--` ends option parsing. Modes find/watch/wait/wait-in-use/connect/doctor are
mutually exclusive; interval needs watch/wait and timeout needs wait/connect/doctor.

## Public Go API

The module is `github.com/aman-void/portcheck`. These signatures and exported
concepts are retained without additions, removals, or field/type changes:

```go
Check(context.Context, int) (Result, error)
CheckPorts(context.Context, []int) ([]Result, error)
CheckHost(context.Context, string, int) (Result, error)
CheckPortsHost(context.Context, string, []int) ([]Result, error)
ValidateHost(string) error
ValidateEndpoint(string) error
Connect(context.Context, string, time.Duration) (ConnectivityResult, error)
```

- `Status` is a string: `StatusFree="free"`, `StatusInUse="in_use"`,
  `StatusError="error"`. `Result` contains `Port int`, `Status Status`, `Err error`.
- `ConnectivityStatus` is a string: `ConnectivityReachable="reachable"`,
  `ConnectivityRefused="refused"`, `ConnectivityTimeout="timeout"`,
  `ConnectivityError="error"`. `ConnectivityResult` contains `Address string`,
  `Status ConnectivityStatus`, `Duration time.Duration`, `Err error`.
- `DefaultConnectTimeout` is 5 seconds. `Connect` requires a positive timeout.
- Validation sentinels: `ErrInvalidPort`, `ErrInvalidHost`, `ErrInvalidEndpoint`,
  `ErrInvalidTimeout`. Use `errors.Is`; an invalid endpoint port may wrap both
  endpoint and port sentinels. DNS failures are not syntax-validation failures.
- Bind occupation returns no error. Every unsuccessful dial (including expected
  refusal/timeout) returns an error. Individual operations return the same error
  in the result and error return. Wrapped causes support `errors.Is`/`errors.As`.
- Batch operations validate everything first, preserve order, deduplicate without
  changing input, and continue after system failures. Check all returned results
  **and** the joined error. Cancellation can return partial results and joined
  system/context causes. Empty valid input returns a non-nil empty slice.
- Contexts must be non-nil. Host validation performs no DNS lookup. Hostnames
  are resolved by the networking operation; bind selects one address, while Go's
  normal dial may try more than one resolved address. No Portcheck retries exist.
- Find/range parsing/watch/wait/process/doctor and renderers remain internal CLI
  concerns. Go structs are not the CLI JSON schema; do not marshal them expecting
  the CLI's field names/omission rules.

## JSON schemas

Every complete JSON response is one top-level array followed by a newline.
Connectivity and doctor arrays contain exactly one record. Port checks and
successful waits/find preserve deduplicated input order. Watch JSON is rejected.
An exhausted find or a timed-out/canceled wait has no stdout, not an empty array.
Invalid CLI input has no JSON stdout. An output-write failure may necessarily
leave incomplete output and exits 3.

### Port record

| Field | Type | Presence/meaning |
| --- | --- | --- |
| `port` | integer, 1–65535 | Required |
| `status` | `free`, `in_use`, `error` | Required, bind result only |
| `error` | string | Optional bind-failure diagnostic |
| `processes` | array of process records | Optional visible occupied-port owners, only when requested |
| `process_error` | string | Optional inspection failure; never changes bind status |

A process record contains required integer `pid` (>0), optional string `name`,
and optional string `executable`. Owners are PID-sorted; absent metadata is
omitted, not null. JSON escapes original process strings; human output replaces
nonprintable characters. Inspection is best effort, not a completeness guarantee.

### Connectivity record

| Field | Type | Presence/meaning |
| --- | --- | --- |
| `address` | string | Required, supplied endpoint (not resolved IP) |
| `status` | `reachable`, `refused`, `timeout`, `error` | Required |
| `duration_ms` | nonnegative number | Required; elapsed resolution/dial, fractional milliseconds |
| `error` | string | Required on unsuccessful operation, omitted on success |

Duration excludes validation and cleanup; a pre-canceled dial has duration 0.
Error text is diagnostic, OS-dependent, and **not** a programmatic contract.

### Doctor record

| Field | Type | Presence/meaning |
| --- | --- | --- |
| `endpoint` | object | Required: string `host`, integer `port`, string `protocol="tcp"` |
| `local` | object | Required: bind `status`; optional diagnostic `error` |
| `processes` | array | Optional, same process records as above |
| `process_error` | string | Optional inspection failure |
| `connectivity` | object | Required, exactly the connectivity record above |

Free/error local results omit process fields. Observations are sequential and
can disagree due to intervening state changes; no atomic snapshot is promised.
Example responses are in the README. No JSON key or value vocabulary changes
were made for v1.0.

## Versioning policy

Upon final v1.0.0 release: patches fix bugs without breaking these contracts;
minor releases add backward-compatible capabilities; breaking API/CLI/JSON
changes require a major version. Consumers should tolerate additive optional
JSON fields, but field names, types, status values, quiet tokens, and exit-code
meanings must not be casually changed. Internal packages and OS error wording
are not stable APIs. Version output keeps `portcheck version <semver>`.
