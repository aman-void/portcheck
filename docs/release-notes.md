# Portcheck v1.0 release candidate

This candidate stabilizes the existing TCP developer-tool feature set:
local bind availability, ranges, upward free-port discovery, IPv4/IPv6 host
selection, watch/wait, optional Linux listener ownership, explicit TCP connectivity,
doctor composition, ordered JSON arrays, quiet tokens, and predictable exit codes.

## Changes

- Preserve the v0.1–v0.5 CLI/public Go API/JSON contracts.
- Correct Windows Winsock address-in-use classification without treating
  permission or address errors as occupied ports.
- Return the documented exit 3 on Unix broken stdout pipes instead of being
  terminated by SIGPIPE before the CLI can report the write failure.
- Supply accurate version metadata for release archives and versioned Go installs.
- Add native/cross-platform CI, built-binary regression tests, input fuzz targets,
  focused benchmarks, reproducible archives, provenance, and SHA-256 checksums.

## Installation

Select a reviewed version from `github.com/aman-void/portcheck/releases`, download
its platform archive and `SHA256SUMS`, verify the archive, extract the binary,
and place it on `PATH`. Or, once the tag is published:

```sh
go install github.com/aman-void/portcheck/cmd/portcheck@v1.0.0-rc.1
portcheck --version
```

Before publishing this draft, replace the candidate version with the actual tag
and record successful native CI evidence. Six target archives are generated:
Linux/macOS/Windows, each amd64 and arm64. Locally only Linux amd64 is
runtime-validated; other targets must be labeled build-only until tested.

## Compatibility and limitations

JSON remains an array; watch JSON and quiet+JSON remain invalid. Process lookup
warnings do not alter network statuses or exit codes. Only Linux process lookup
is implemented, subject to permissions, namespaces, and socket/PID races.
TCP success is not application/HTTP/TLS health, firewall diagnosis, or a port
reservation. No UDP, scanning, payloads, process control, retries, or external
Go dependencies were added. See the README and platform/contract documentation.

Final 1.0.0 stability commitments begin only after the candidate is accepted
and the owner approves the final release.
