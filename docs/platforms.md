# Platform support

Compilation is not functional validation. This table records evidence available
on the development machine, not a promise that remote CI has already run.

| Target | Candidate artifact | Native evidence here | Process inspection |
| --- | --- | --- | --- |
| Linux amd64 | `.tar.gz` | Native tests, sockets, process ownership, signals, race detector | Supported, best effort via `/proc` |
| Linux arm64 | `.tar.gz` | Build-only | Linux implementation compiled, not runtime-validated here |
| macOS amd64 | `.tar.gz` | Build-only | Explicitly unsupported |
| macOS arm64 | `.tar.gz` | Build-only | Explicitly unsupported |
| Windows amd64 | `.zip` | Build-only | Explicitly unsupported |
| Windows arm64 | `.zip` | Build-only | Explicitly unsupported |

CI runs native tests/race/vet/build on Linux, macOS, and Windows runners and
cross-compiles the CLI and test suites for all six targets. Record the exact
runner architectures and successful run URLs before promoting any build-only
target to runtime-validated support. Windows Winsock occupation classification
has a targeted platform test; its execution still needs native Windows CI.

Linux inspection uses visible listener tables and PID fd/metadata files without
root, subprocesses, process modification, or command-line collection. Names and
executable paths are optional; permission failures, namespaces, containers,
`hidepid`, PID/socket races, and non-listening bind conflicts can hide ownership.
Same-family wildcard conflicts are matched. Cross-family dual-stack ownership
and scoped IPv6 ownership cannot be determined safely from `/proc` and may be
unavailable. Details are in the README; network status is never changed by lookup
failure. Unsupported platforms report an explicit inspection warning rather
than pretending there is no owner.

Basic bind/dial operations use Go networking APIs. IPv6 tests are skipped when
the environment lacks IPv6. Low ports may require permissions. Wildcard and
dual-stack behavior remains OS-defined. Context cancellation is cooperative;
not every synchronous socket/filesystem syscall can be interrupted instantly.
Tests of Unix process signals are explicitly skipped on Windows, where Unix
signal-delivery equivalence is not claimed. No Windows/macOS process backend
was added as part of stabilization.
