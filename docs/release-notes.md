# Portcheck {{VERSION}}

This release stabilizes the existing TCP developer-tool feature set: local bind
availability, ranges, upward free-port discovery, IPv4/IPv6 host selection,
watch/wait, optional Linux listener ownership, explicit TCP connectivity, doctor
composition, ordered JSON arrays, quiet tokens, and predictable exit codes.

## Changes

- Preserve the v0.1–v0.5 CLI, public Go API, and JSON contracts.
- Correct Windows Winsock address-in-use classification without treating
  permission or address errors as occupied ports.
- Report the documented exit 3 on Unix broken stdout pipes instead of being
  terminated by SIGPIPE before the CLI can report the write failure.
- Supply accurate version metadata for release archives and versioned Go installs.
- Add native/cross-platform CI, built-binary regression tests, input fuzz targets,
  focused benchmarks, reproducible archives, provenance, and SHA-256 checksums.

## Installation

Download this release's platform archive and `SHA256SUMS` from
[the releases page](https://github.com/aman-void/portcheck/releases), verify the
archive, extract the binary, and place it on `PATH`. Or install the same tag with
the Go toolchain:

```sh
go install github.com/aman-void/portcheck/cmd/portcheck@v{{VERSION}}
portcheck --version
```

Six target archives are published: Linux, macOS, and Windows, each amd64 and
arm64. `docs/platforms.md` records which platforms are runtime-validated and
which are build-only.

## Compatibility and limitations

JSON remains an array; watch JSON and quiet+JSON remain invalid. Process lookup
warnings do not alter network statuses or exit codes. Only Linux process lookup
is implemented, subject to permissions, namespaces, and socket/PID races. TCP
success is not application/HTTP/TLS health, firewall diagnosis, or a port
reservation. No UDP, scanning, payloads, process control, retries, or external Go
dependencies were added. See the README and `docs/platforms.md`.