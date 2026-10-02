# Releasing Portcheck

The master is `plans/plan.md`; v1.0 scope is `plans/v1.0.md`. Final release and
commit/tag/publication require owner approval. The first candidate is
`v1.0.0-rc.1`. A local candidate is not a released v1.0.0.

## Validate a candidate

Run on the intended source revision, using the Go version in `go.mod`:

```sh
gofmt -w .
test -z "$(gofmt -l .)"
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go build ./...
go build ./cmd/portcheck
go list -m all
go doc .
```

Run `govulncheck ./...` if installed; report unavailable tooling explicitly.
The application has no third-party Go dependencies. Review malformed input,
resources, cancellation, `/proc` races, and terminal safety, not just dependencies.
CI checks formatting without rewriting files, native behavior on three OSes,
and CLI/test compilation on all six target pairs.

Use focused socket tests from `AGENTS.md`, the built-binary compatibility suite
(`go test ./cmd/portcheck -run '^TestBuiltBinaryContracts$' -count=1`), and manual
smoke tests with dynamically allocated local listeners. Cover check/ranges/find/
JSON/quiet/host/watch/wait/process/connect/doctor and actual process exit codes.
Use channels/observed watch output to synchronize; don't assume fixed ports
are free or use public services for tests. Recheck installation and `--version`.

Benchmarks are pure parsing/orchestration/formatting, not claims about OS TCP
performance. Record representative local timings separately, without introducing
worker pools. See the README benchmark commands.

## Build artifacts

Packaging runs on Linux and requires Bash, Go, Git, GNU tar, gzip, zip,
sha256sum, and ordinary core utilities. These are developer-only tools, not
dependencies of the Portcheck executable or library. No application shells out.

```sh
make release VERSION=1.0.0-rc.1 OUT=dist/rc.1
(cd dist/rc.1 && sha256sum -c SHA256SUMS)
```

The output directory must not exist. The script validates stable `1.x.y` and
`1.x.y-rc.N` versions, refuses a dirty checkout, and builds Linux/macOS/Windows
amd64/arm64 with `CGO_ENABLED=0`, `-trimpath`, `-buildvcs=false`, stripping symbols
and injecting the version with `-X .../internal/cli.version=...`.
`go install module@version` uses Go module build information instead of linker
flags; explicit linker injection takes precedence. Development is `1.0.0-dev`.

Each archive contains exactly one directory:

```text
portcheck_1.0.0-rc.1_<os>_<arch>/
  portcheck[.exe]
  LICENSE
  BUILDINFO.txt
```

The build-info file records version, source commit, dirty state, toolchain,
target, and cgo setting. No timestamp, username, or local path is embedded.
Tar/gzip and ZIP metadata are normalized; `SHA256SUMS` is generated from the
actual six archives. Rebuild into a second fresh directory using identical
source, Go toolchain, version, and environment, and compare the checksum manifests.
Reproducibility is assessed for those inputs, not across different Go versions.
Checksum verification establishes integrity against the manifest, not a signature
or a separate authenticity guarantee.

For testing tooling **before** committing intentionally changed files, use:

```sh
ALLOW_DIRTY=1 make release VERSION=1.0.0-rc.1 OUT=dist/local-rc.1
```

Those archives record `dirty=true`, warn, and **must not be published**. Rebuild
from the approved clean commit before tagging/distributing. No commit or tag is
created by the packaging script.

## Owner-controlled publication

1. Review [contracts](contracts.md), [platform evidence](platforms.md), security/
   performance findings, README/help/Go docs, and release notes.
2. Approve and commit the candidate; verify clean `git status`.
3. With explicit approval, tag the intended commit `v1.0.0-rc.1` and push it.
4. The tag workflow reruns native/cross-platform CI, builds archives/checksums,
   and creates a **draft prerelease**. Review artifacts and update the draft's
   notes with the actual version, tested platforms, and CI evidence before publishing.
5. Install/extract and smoke-test the candidate on claimed native platforms.
6. Only after candidate acceptance, approve the final clean release commit and
   `v1.0.0` tag. Build from that exact commit, check `--version`, publish the
   reviewed draft with release notes/checksums, and verify Go installation.

The workflow uses GitHub-hosted Go setup/checkout actions; these are CI tooling,
not Go module dependencies. Publishing a draft does not satisfy native runtime
validation or the owner's final release review. An existing draft is not silently
replaced on rerun: investigate it rather than overwriting assets accidentally.

## Final checklist

- [ ] Full tests, race, vet, formatting, builds, dependency audit pass
- [ ] Vulnerability check executed or limitation explicitly accepted
- [ ] API/CLI/JSON/exit-code compatibility and documentation reviewed
- [ ] Security, cancellation, timeout, and resource review completed
- [ ] Native CI evidence recorded for every runtime support claim
- [ ] Six artifacts/checksums/archive layout validated; reproducibility compared
- [ ] Candidate installed and smoke-tested; owner acceptance recorded
- [ ] Intentional clean commit approved; exact release tag approved and created
- [ ] Release version is 1.0.0; artifacts built from the tagged commit
- [ ] User-focused notes and installation instructions published/verified

Unfinished owner/CI/publication steps remain blockers to calling v1.0 complete.
