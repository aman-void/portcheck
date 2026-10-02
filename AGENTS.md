# Portcheck agent guidance

## Specifications and scope
- The master specification is `plans/plan.md`, not root `plan.md` despite references in the plans. Read it completely before implementation, then read the target version's complete specification.
- Plans describe intended behavior, not implemented features. Inspect the actual code before assuming any version is complete.
- Implement only the explicitly assigned version. Roadmap features do not authorize implementation; each subsequent version requires human acceptance of its predecessor.
- Resolve conflicts between master and version specifications explicitly before coding. Preserve earlier behavior unless the target specification explicitly changes it.
- Owner-approved module path: `github.com/aman-void/portcheck`; license: MIT.
- Never commit without explicit user permission. Plans are version-controlled alongside the implementation.

## Design constraints
- Use Go's standard library only; external dependencies require explicit approval. Do not introduce CLI frameworks or shell out for networking/process inspection.
- Availability means successfully binding the selected local TCP address, not connecting to it. Default to `127.0.0.1`; close successful listeners immediately. Only address-in-use failures mean `IN USE`.
- Keep detection and structured results independent of CLI parsing and rendering. Watch/wait orchestration stays outside the stateless checker; doctor composes existing operations.
- Preserve input order and deduplicate by first occurrence. Range endpoints are inclusive; find searches upward through 65535 without wrapping.
- The CLI calls the root Go API, not `internal/checker` directly. `CheckPorts` validates all input before binding, continues after system errors, and returns partial results on cancellation; inspect both results and the joined error.
- `--find` requires one single starting port and prints a port number even with `--quiet`. JSON is always an array; exhausted find emits no stdout and exits 1. `--quiet --json` is invalid.
- v0.1 forbids concurrency. Later versions do not justify worker pools merely by adding ranges or polling.
- From v0.3, use IPv6-safe address construction and cancellation-aware polling. Watch emits initial state and meaningful changes; wait requires every requested port to satisfy the condition.
- From v0.4, process inspection is optional, read-only, and isolated in platform-specific files. Multiple owners are possible; lookup failures must not change port status.
- From v0.5, connectivity is a separate TCP dial operation: no payloads, retries, or scanning. Classify wrapped errors rather than matching strings; do not infer firewall or application health.

## Release and version discipline
- A release version is one coordinated change across the Git tag, the injected application version, release archive filenames, README current-release references, and release notes. Never update only the README, or only the version constant.
- Inspect `.github/workflows/release.yml`, `scripts/release.sh`, `scripts/check-version.sh`, `Makefile`, `README.md`, `docs/release-notes.md`, and `internal/cli/version.go` before changing any release information.
- `bash scripts/check-version.sh <1.x.y[-rc.N]>` is the enforcement point. It builds the binary, asserts `--version`, checks archive names, and scans the current-release documentation. Do not create or push a release tag until it passes.
- Release notes use the `{{VERSION}}` placeholder substituted by the workflow. Never hardcode a version there; one file must serve both prerelease and final tags.
- Distinguish the three versions deliberately: `1.0.0-dev` (untagged source builds), the current candidate, and a future stable release. Do not claim a stable release exists before it is published.
- Version reporting depends on build flags. `go build` without `-buildvcs=false` on a tagged commit reports that tag, because Go stamps the module version; with the flag, or on an untagged checkout, it reports `1.0.0-dev`. `-X` injection always wins. `release.sh` and CI use `-buildvcs=false` plus `-X` so release artifacts are reproducible and tag-independent.
- Do not commit unrelated changes as part of release preparation. Keep version bumps reviewable on their own.
- Sizes documented in the README come from the table `scripts/release.sh` prints. Re-measure rather than estimating, and do not simplify the six-target platform matrix to make size reporting easier.

## Order of operations
- Distinguish these five steps; a successful local edit proves none of the later ones:
  1. modify files
  2. validate locally (`gofmt -l .`, `go test ./...`, `go vet ./...`, `bash -n scripts/*.sh`, `scripts/check-version.sh <version>`)
  3. commit
  4. push
  5. create and push the release tag
- Never commit, tag, or push without explicit user permission.
- Let CI be the final enforcement layer. If CI reports a version mismatch, stop the release rather than publishing inconsistent artifacts; fix the repository and release again.

## Verification and contracts
- `make build` writes `bin/portcheck`; `go build ./cmd/portcheck` writes the root binary. Use the built binary when checking exit codes: `go run` does not preserve nonzero application exit codes.
- Focused real-socket checks: `go test ./internal/checker -run '^TestCheckListenerLifecycle$' -count=1` and `go test ./internal/cli -run '^TestRunRealListener$' -count=1`.
- Verify the existing baseline before advancing versions. Before completion run `gofmt -w .`, `go test ./...`, `go vet ./...`, and `go build ./cmd/portcheck`; v0.5+ also requires `go build ./...` and `go list -m all`.
- v1.0 adds formatting checks with `gofmt -l .`, race testing where supported, cross-platform builds, and `govulncheck ./...` if available.
- Network tests use dynamically allocated local listeners, not assumed-free ports or public internet services. Control dialing for timeout tests rather than assuming an arbitrary IP will time out; process tests must not require root.
- Keep diagnostics on stderr and results on stdout, especially JSON. Preserve documented quiet tokens, JSON schemas, and exit codes: 0 success, 1 unmet condition, 2 invalid input, 3 system failure.
- Check the target version's full Definition of Done; report actual commands/results, deviations, and limitations. Cross-compilation alone is not proof of functional platform support.
