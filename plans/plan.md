# Portcheck

> A tiny, fast, dependency-free developer utility for inspecting TCP ports, local services, and network connectivity.

---

## 1. Project Overview

Portcheck is a developer-focused command-line utility written in Go.

Its initial purpose is simple:

> Determine whether a TCP port is available or currently in use.

The project should gradually evolve into a small, reliable local network diagnostic utility while maintaining a minimal core, predictable CLI behavior, and zero external Go dependencies.

The project is also an experiment in AI-driven software development.

The project will be developed incrementally by AI coding agents. Each version has an explicit specification, implementation scope, tests, and definition of done.

Humans review each completed version before the next version is implemented.

The project should therefore prioritize:

- Correctness
- Simplicity
- Maintainability
- Testability
- Backward compatibility
- Clear interfaces
- Small, focused changes
- Cross-platform considerations
- Good CLI design
- Minimal dependencies

---

# 2. Project Goals

Portcheck should eventually provide developers with a convenient way to:

- Check whether a TCP port is available.
- Determine whether a local TCP port is in use.
- Check multiple ports.
- Check port ranges.
- Find an available port.
- Select a specific network address/interface.
- Watch port state changes.
- Wait for a port to become available or occupied.
- Inspect the process using a local port where the operating system permits it.
- Test TCP connectivity to remote hosts.
- Produce human-readable output.
- Produce machine-readable JSON output.
- Provide predictable exit codes.
- Work well in shell scripts and CI environments.
- Provide a reusable Go library where appropriate.
- Eventually be distributed through multiple ecosystems.

The project must remain useful as a simple CLI even as advanced features are added.

---

# 3. Non-Goals

Portcheck is not intended to become:

- A full network scanner.
- A packet analyzer.
- A firewall configuration tool.
- A vulnerability scanner.
- A service manager.
- A replacement for `netstat`, `ss`, `lsof`, or `nmap`.
- An AI assistant.
- A general-purpose network monitoring platform.

Features should be added only when they directly support the project's developer-focused purpose.

Avoid feature creep.

---

# 4. Core Design Philosophy

## 4.1 Keep the core small

The fundamental operation should remain simple:

```text
Input
  ↓
Validate
  ↓
Check
  ↓
Result
  ↓
Present
```

The basic port-checking engine should not know about:

- terminal formatting
- CLI argument parsing
- JSON formatting
- process inspection
- shell commands
- logging presentation

Those concerns belong in higher layers.

---

## 4.2 Separate detection from presentation

The detection engine should produce structured results.

For example:

```go
type Result struct {
    Port   int
    Status Status
    Err    error
}
```

The CLI should decide how that result is displayed.

This allows the same detection logic to support:

```text
Human output
Quiet output
JSON output
Library consumers
Tests
Future interfaces
```

---

## 4.3 Standard library first

The Go implementation should use only the Go standard library unless an external dependency is explicitly approved later.

The initial project must have:

```text
No third-party Go dependencies
```

This is intentional.

Portcheck should demonstrate how far a small production-quality developer utility can go using the Go standard library.

---

## 4.4 Do not over-engineer

The project should not introduce abstractions without a concrete reason.

Avoid:

- unnecessary interfaces
- unnecessary dependency injection
- generic frameworks
- CLI frameworks
- worker pools for trivial workloads
- configuration systems that solve problems the project does not have
- excessive package fragmentation
- unnecessary concurrency
- premature optimization

Architecture should follow actual requirements.

---

# 5. Product Architecture

The long-term conceptual architecture is:

```text
                         ┌──────────────────────┐
                         │        CLI           │
                         │ argument parsing     │
                         │ commands/options     │
                         └──────────┬───────────┘
                                    │
                                    ▼
                         ┌──────────────────────┐
                         │      Application     │
                         │ orchestration/usecase│
                         └──────────┬───────────┘
                                    │
                    ┌───────────────┼────────────────┐
                    │               │                │
                    ▼               ▼                ▼
             ┌────────────┐ ┌─────────────┐ ┌──────────────┐
             │ TCP Checker│ │ Process Info│ │ Connectivity │
             └────────────┘ └─────────────┘ └──────────────┘
                    │               │                │
                    └───────────────┼────────────────┘
                                    ▼
                              Structured Results
                                    │
                         ┌──────────┴──────────┐
                         ▼                     ▼
                    Human Output           JSON Output
```

This is a conceptual architecture.

Do not create every component immediately.

Only introduce a component when its version actually requires it.

---

# 6. Core Concepts

## 6.1 Port

A TCP port is represented as an integer in:

```text
1 - 65535
```

Port `0` is not accepted as user input for normal checking.

Port `0` may still be used internally in tests when asking the operating system to allocate an ephemeral port.

---

## 6.2 Address

The initial implementation checks:

```text
127.0.0.1:<port>
```

This is intentional because it provides predictable local behavior.

Later versions may support:

```text
127.0.0.1
0.0.0.0
::
::1
specific interfaces
specific host addresses
```

The semantics of address selection must always be documented.

---

## 6.3 Protocol

The initial protocol is:

```text
TCP
```

UDP should not be mixed into the initial port availability semantics.

If UDP support is ever added, it should have explicitly separate behavior because UDP does not have the same connection/listening semantics as TCP.

---

# 7. Version Roadmap

The project is divided into incremental versions.

```text
v0.1
Foundation
  │
  ▼
v0.2
Ranges + JSON + Go library
  │
  ▼
v0.3
Hosts + Watch + Wait
  │
  ▼
v0.4
Process inspection
  │
  ▼
v0.5
Connectivity + Diagnostics
  │
  ▼
v1.0
Stable production release
```

Each version must have its own specification:

```text
plans/
├── v0.1.md
├── v0.2.md
├── v0.3.md
├── v0.4.md
├── v0.5.md
└── v1.0.md
```

The master `plan.md` defines the project.

The version-specific files define the implementation scope for each release.

---

# 8. Version 0.1: Foundation

The first release establishes the basic product.

Expected capabilities:

- Go module
- CLI
- Port validation
- Single port checking
- Multiple ports
- TCP localhost checking
- Help
- Version
- Quiet mode
- Exit codes
- Deterministic output
- Duplicate handling
- Error handling
- Tests
- Documentation

The implementation should remain extremely small.

The detailed implementation specification belongs in:

```text
plans/v0.1.md
```

---

# 9. Version 0.2: Expansion

The second release should make Portcheck more useful as a developer utility.

Potential capabilities:

- Port ranges
- Finding available ports
- JSON output
- Stable structured result model
- Reusable Go library API
- Better shell/automation support

Example:

```bash
portcheck 3000-3010
```

And:

```bash
portcheck --json 3000 8080
```

The exact API and CLI contract must be defined in:

```text
plans/v0.2.md
```

---

# 10. Version 0.3: Monitoring and Address Selection

The third release should expand Portcheck from one-shot checking into lightweight local monitoring.

Potential capabilities:

- Explicit host/address selection
- Watch mode
- State-change reporting
- Wait until available
- Wait until occupied
- Improved timeout/cancellation behavior

Examples:

```bash
portcheck --host 127.0.0.1 8080
```

```bash
portcheck --watch 8080
```

```bash
portcheck --wait 8080
```

Detailed behavior belongs in:

```text
plans/v0.3.md
```

---

# 11. Version 0.4: Process Inspection

Portcheck should eventually answer:

> What process is using this port?

Example:

```bash
portcheck --process 8080
```

Potential output:

```text
PORT    STATUS
8080    IN USE

PROCESS
Name:   yardly-api
PID:    18242
```

Process inspection is inherently platform-specific.

Possible implementations:

```text
Linux
macOS
Windows
```

The platform-specific code must be isolated from the platform-independent core.

Potential conceptual structure:

```text
internal/
└── process/
    ├── process.go
    ├── linux.go
    ├── darwin.go
    └── windows.go
```

Do not assume that process information available on one operating system exists identically on another.

Detailed requirements belong in:

```text
plans/v0.4.md
```

---

# 12. Version 0.5: Connectivity and Diagnostics

Portcheck can eventually distinguish between:

```text
Can I bind this local port?
```

and:

```text
Can I connect to this TCP endpoint?
```

These are different operations and must remain separate concepts.

Potential functionality:

```bash
portcheck --connect localhost:8080
```

or:

```bash
portcheck --connect example.com:443
```

Potential output:

```text
ADDRESS        STATUS
localhost:8080 REACHABLE
```

The version may also introduce richer diagnostic output.

For example:

```bash
portcheck doctor 8080
```

could combine:

- port status
- address
- protocol
- process
- PID
- connectivity information
- diagnostic explanation

Detailed requirements belong in:

```text
plans/v0.5.md
```

---

# 13. Version 1.0: Stable Release

Version 1.0 represents a stable public release.

The focus should shift from adding features to:

- API stability
- CLI stability
- documentation
- cross-platform reliability
- release automation
- packaging
- backward compatibility
- performance
- security
- maintainability

Potential distribution targets:

```text
Go module
GitHub Releases
npm wrapper
Homebrew
Linux packages
AUR
```

Distribution mechanisms should be added only when they can be maintained properly.

Detailed requirements belong in:

```text
plans/v1.0.md
```

---

# 14. CLI Design Principles

The CLI should follow familiar Unix conventions.

Prefer:

```bash
portcheck [options] <ports...>
```

Options should have predictable short and long forms where appropriate.

For example:

```text
-h, --help
-v, --version
-q, --quiet
```

Later options may include:

```text
--json
--host
--watch
--wait
--process
--connect
--verbose
```

Do not add an option merely because it is technically possible.

Every option must have:

- clear semantics
- documentation
- tests
- predictable exit behavior

---

# 15. Output Design

There are three primary output modes.

## Human-readable

Optimized for developers reading terminal output.

Example:

```text
PORT    STATUS
3000    FREE
8080    IN USE
```

---

## Quiet

Optimized for shell scripts.

Example:

```text
FREE
```

or:

```text
IN_USE
```

Quiet output must remain stable.

---

## JSON

Optimized for programs.

Example:

```json
[
  {
    "port": 3000,
    "status": "free"
  },
  {
    "port": 8080,
    "status": "in_use"
  }
]
```

JSON schemas should be treated as public interfaces once released.

Avoid casually changing field names or types after v1.0.

---

# 16. Exit Code Design

Exit codes should communicate the overall command result.

Proposed semantics:

```text
0   All requested checks succeeded with desired state
1   One or more ports are in use / requested condition not met
2   Invalid command-line input
3   Unexpected/system error
```

Exact semantics must be documented before v1.0.

Do not encode detailed information into arbitrary exit codes.

Detailed information belongs in stdout/JSON.

---

# 17. Error Handling

Errors should be:

- concise
- actionable
- specific
- written to stderr
- non-panicking for normal user errors

Bad:

```text
error: something went wrong
```

Better:

```text
error: invalid port "70000": must be between 1 and 65535
```

System errors should retain useful context:

```text
error: failed to check port 8080: permission denied
```

Never classify every `net.Listen` failure as "port in use".

Possible causes include:

- address already in use
- permission failure
- invalid address
- resource exhaustion
- operating-system-specific failures

The implementation must distinguish these where practical.

---

# 18. Concurrency

Concurrency is not a feature by itself.

The project should not introduce goroutines merely to make the code appear sophisticated.

For a small number of ports, sequential checking may be sufficient.

Concurrency becomes relevant when:

- checking large port ranges
- performing independent network operations
- benchmarking demonstrates a meaningful benefit

If concurrency is introduced:

- preserve deterministic output
- support cancellation
- avoid unbounded goroutine creation
- avoid unnecessary worker pools
- benchmark before and after

A range such as:

```bash
portcheck 1-65535
```

is a more meaningful performance scenario than checking five ports concurrently.

---

# 19. Cancellation and Timeouts

As network operations become more complex, operations should support cancellation where appropriate.

The reusable library should prefer:

```go
context.Context
```

when operations can block or wait.

CLI commands should terminate cleanly on:

```text
SIGINT
SIGTERM
```

where appropriate.

Do not introduce context everywhere just because it exists.

Use it where cancellation and deadlines actually matter.

---

# 20. Cross-Platform Strategy

The project should aim to support:

```text
Linux
macOS
Windows
```

The core TCP checking functionality should use portable Go APIs whenever possible.

OS-specific functionality must be isolated.

Examples:

```text
Process inspection
Interface details
Socket metadata
Operating-system diagnostics
```

Potential structure:

```text
internal/process/
    process.go
    linux.go
    darwin.go
    windows.go
```

Use Go build constraints only when necessary.

Do not make the entire application platform-specific because one advanced feature requires OS APIs.

---

# 21. Go Library Design

The CLI should not necessarily be the only interface.

Where useful, Portcheck should provide a small reusable Go API.

The library should expose the smallest useful public surface.

Prefer:

```go
result, err := portcheck.Check(ctx, 8080)
```

over exposing internal implementation details.

Avoid exporting:

- internal CLI structures
- terminal formatting types
- internal worker implementations
- unnecessary interfaces
- unstable implementation details

The public API becomes a compatibility commitment.

Therefore, do not rush to export types simply because they currently exist.

---

# 22. Package Structure

Start with the smallest structure necessary.

A possible long-term structure:

```text
portcheck/
├── cmd/
│   └── portcheck/
│       └── main.go
│
├── internal/
│   ├── checker/
│   ├── cli/
│   ├── process/
│   └── connectivity/
│
├── portcheck.go
│
├── plans/
│   ├── v0.1.md
│   ├── v0.2.md
│   ├── v0.3.md
│   ├── v0.4.md
│   ├── v0.5.md
│   └── v1.0.md
│
├── go.mod
├── README.md
├── LICENSE
├── Makefile
└── .gitignore
```

This is a possible destination, not a requirement for the first version.

Do not create directories until they represent real responsibilities.

---

# 23. Testing Strategy

Testing is a first-class part of the project.

Tests should focus primarily on behavior rather than implementation details.

Important categories:

## Unit tests

Test:

- port validation
- result construction
- parsing
- formatting
- option handling
- range parsing
- state transitions

---

## Integration tests

Use real TCP listeners where appropriate.

Example:

```go
listener, err := net.Listen("tcp", "127.0.0.1:0")
```

Retrieve the dynamically assigned port.

Verify:

```text
listener open → IN USE
listener closed → FREE
```

Never hardcode a port in tests that might already be occupied.

---

## CLI tests

Test:

- no arguments
- invalid arguments
- valid ports
- multiple ports
- duplicate ports
- help
- version
- quiet mode
- JSON mode
- exit codes
- stdout
- stderr

---

## Platform tests

OS-specific features must have platform-appropriate tests.

Do not assume Linux behavior is identical to macOS or Windows.

---

# 24. Performance

Performance matters only after correctness.

The project should eventually benchmark:

```text
single port
multiple ports
large port ranges
full 1-65535 range
JSON output
watch mode
process inspection
```

Potential benchmark:

```text
portcheck 1-65535
```

Metrics may include:

- execution time
- allocations
- memory usage
- goroutine count
- system calls where useful

Do not optimize based on assumptions.

Measure first.

---

# 25. Security Considerations

Portcheck is a diagnostic tool and should avoid introducing unnecessary security risks.

Consider:

- untrusted command-line input
- malicious hostnames
- excessive ranges
- resource exhaustion
- unbounded concurrency
- command execution
- platform-specific process inspection
- output injection
- terminal escape sequences

Do not execute shell commands merely to obtain information that can be retrieved safely through APIs.

If OS commands become necessary for a platform-specific feature, isolate and validate them carefully.

---

# 26. Dependency Policy

Initial policy:

```text
Go dependencies: 0
```

Standard library is preferred.

Adding an external dependency requires a concrete reason.

Before introducing one, evaluate:

- Can the standard library solve it?
- Does the dependency significantly reduce complexity?
- Is it actively maintained?
- Does it introduce security or licensing concerns?
- Does it meaningfully improve the project?
- Is the dependency worth the long-term maintenance cost?

Dependency count should not become a vanity metric either.

The goal is appropriate simplicity.

---

# 27. Distribution Strategy

The primary Go installation method should eventually be:

```bash
go install github.com/<owner>/portcheck/cmd/portcheck@latest
```

Binary releases should eventually support common platforms and architectures.

Potential targets:

```text
Linux amd64
Linux arm64
macOS amd64
macOS arm64
Windows amd64
Windows arm64
```

Exact release targets should be decided during v1.0.

---

# 28. npm Distribution

The core implementation remains Go.

If npm distribution is added, npm should act as a distribution wrapper around the compiled Go binary.

Conceptually:

```text
npm package
     │
     ▼
Detect OS / architecture
     │
     ▼
Download appropriate binary
     │
     ▼
Expose portcheck CLI
```

Example:

```bash
npm install -g @<scope>/portcheck
```

The npm package should not reimplement the port-checking engine in JavaScript.

There should be one authoritative implementation.

---

# 29. Documentation

The README should eventually document:

- Project purpose
- Installation
- Usage
- Examples
- Supported platforms
- CLI options
- Output formats
- Exit codes
- Limitations
- Library usage
- Development
- Testing
- Building
- Release process

Advanced features should have examples.

Documentation should describe actual behavior, not intended behavior.

---

# 30. Compatibility Policy

Before v1.0:

Breaking changes are acceptable when justified.

However:

- document the change
- update tests
- update documentation
- avoid unnecessary churn

After v1.0:

CLI behavior and public Go APIs should be treated as stable contracts.

Breaking changes should require deliberate versioning.

---

# 31. Git and Change Discipline

AI agents must make focused changes.

Avoid:

- unrelated refactoring
- mass formatting unrelated files
- renaming things without reason
- changing architecture while implementing a feature
- modifying future-version functionality
- adding dependencies without approval

Each version should result in a coherent project state.

Commit messages should describe meaningful changes, for example:

```text
feat: add TCP port availability checking
test: add port checker integration tests
feat: add JSON output
fix: preserve port order in concurrent checks
```

---

# 32. AI Agent Development Rules

These rules apply to every coding agent working on Portcheck.

## Rule 1

Read the entire `plan.md` before implementing anything.

## Rule 2

Read the current version specification:

```text
plans/vX.Y.md
```

## Rule 3

Implement only the current version.

Do not implement future versions.

## Rule 4

Preserve existing behavior unless the current version explicitly changes it.

## Rule 5

Do not add external dependencies without explicit approval.

## Rule 6

Do not over-engineer.

## Rule 7

Do not introduce abstractions without a concrete requirement.

## Rule 8

Write tests for new behavior.

## Rule 9

Do not remove existing tests merely because implementation changed.

## Rule 10

Run:

```bash
gofmt
go test ./...
go vet ./...
```

before declaring the work complete.

## Rule 11

If a requirement is ambiguous, inspect the version specification and existing implementation before making assumptions.

## Rule 12

Do not silently change public behavior.

## Rule 13

If the requested feature conflicts with existing architecture, explain the conflict before performing a large refactor.

## Rule 14

Do not implement "nice-to-have" features outside the current version.

## Rule 15

Do not add AI functionality to the Portcheck core.

---

# 33. Version Boundary Rule

The project follows strict version boundaries.

For example:

```text
v0.1 agent
    ↓
May implement v0.1
    ↓
Must not implement v0.2+

v0.2 agent
    ↓
May implement v0.2
    ↓
Must preserve v0.1

v0.3 agent
    ↓
May implement v0.3
    ↓
Must preserve v0.1 + v0.2
```

The existence of a feature in this master plan does not authorize its implementation.

Only the active version specification authorizes implementation.

---

# 34. Human Review Gate

Every version must pass a human review gate before the next version begins.

Process:

```text
Read master plan
      ↓
Read version plan
      ↓
AI implementation
      ↓
Automated tests
      ↓
Human review
      ↓
Accept / reject / revise
      ↓
Next version
```

The next version must not begin merely because the previous agent claims it is complete.

The implementation must satisfy the version's Definition of Done.

---

# 35. Definition of Done

Every version must define its own Definition of Done.

At minimum:

```text
[ ] Required functionality implemented
[ ] Existing functionality preserved
[ ] Tests added
[ ] Tests pass
[ ] gofmt clean
[ ] go vet clean
[ ] Build succeeds
[ ] Documentation updated
[ ] CLI behavior verified
[ ] No unauthorized dependencies
[ ] No future-version features implemented
```

Additional version-specific requirements belong in the corresponding version plan.

---

# 36. Regression Policy

Every new version must preserve the behavior of previous versions unless the current version explicitly changes it.

Before completing a version:

```bash
go test ./...
```

must pass.

If a previous behavior must change:

1. Identify the behavior.
2. Explain why it must change.
3. Update the specification.
4. Update tests.
5. Update documentation.

Never silently break existing functionality.

---

# 37. Architecture Evolution

The architecture should evolve according to actual requirements.

Expected evolution:

```text
v0.1
Minimal CLI + checker
        │
        ▼
v0.2
Structured results + reusable library
        │
        ▼
v0.3
Address abstraction + monitoring
        │
        ▼
v0.4
Platform-specific process layer
        │
        ▼
v0.5
Connectivity + diagnostics
        │
        ▼
v1.0
Stable public architecture
```

Do not build the v1.0 architecture during v0.1.

Build the simplest architecture capable of correctly implementing the current requirements.

---

# 38. Future Possibilities

These features are intentionally not committed to a specific version unless explicitly included in its version plan.

Possible future capabilities include:

### Port ranges

```bash
portcheck 3000-3010
```

### Find available port

```bash
portcheck --find 3000
```

### JSON

```bash
portcheck --json 3000 8080
```

### Host selection

```bash
portcheck --host 0.0.0.0 8080
```

### Watch mode

```bash
portcheck --watch 8080
```

### Wait mode

```bash
portcheck --wait 8080
```

### Process inspection

```bash
portcheck --process 8080
```

### TCP connectivity

```bash
portcheck --connect example.com:443
```

### Diagnostics

```bash
portcheck doctor 8080
```

These are possibilities, not automatic implementation requirements.

---

# 39. What Portcheck Should Ultimately Feel Like

The final product should feel like a tool that follows this philosophy:

```text
Simple when you need simple.
Detailed when you need detailed.
Scriptable when automation needs it.
Fast when checking many things.
Portable where the operating system allows it.
Predictable everywhere.
```

A developer should be able to type:

```bash
portcheck 8080
```

and immediately understand the answer.

A script should be able to consume:

```bash
portcheck --json 8080
```

A developer debugging a service should eventually be able to use:

```bash
portcheck doctor 8080
```

without needing five different system utilities and a small ritual involving Google.

---

# 40. Master Success Criteria

Portcheck is successful when it demonstrates that a small Go developer utility can be:

- genuinely useful
- dependency-free
- cross-platform where practical
- easy to install
- easy to understand
- easy to script
- easy to test
- easy to extend
- stable over time
- maintainable by humans
- incrementally maintainable by AI agents

The project should favor engineering quality over feature count.

A small tool that developers trust is more valuable than a giant tool that does everything badly.

---

# 41. Project File Contract

The repository should maintain this planning structure:

```text
plan.md
plans/
├── v0.1.md
├── v0.2.md
├── v0.3.md
├── v0.4.md
├── v0.5.md
└── v1.0.md
```

`plan.md` is the authoritative master project specification.

Each version file is the authoritative implementation specification for that version.

If a version file conflicts with the master plan, resolve the conflict explicitly before implementation.

Do not silently choose one.

---

# 42. Agent Invocation Pattern

When starting a new version, provide the coding agent with instructions equivalent to:

```text
Read plan.md completely.

Read plans/vX.Y.md completely.

The current implementation target is vX.Y.

Implement ONLY the functionality specified for vX.Y.

Preserve all functionality from previous versions.

Do not implement features belonging to future versions.

Follow the engineering rules in plan.md.

Write or update tests for all changed behavior.

Run:

gofmt
go test ./...
go vet ./...

Build the project.

Before finishing, verify every item in the vX.Y Definition of Done.

Report:
1. What was implemented.
2. What files changed.
3. What tests were added.
4. Commands executed and their results.
5. Any deviations from the plan.
6. Any unresolved issues.

Do not claim completion if the Definition of Done is not satisfied.
```

---

# 43. Final Principle

The project is intentionally built in stages.

The goal is not:

```text
Give AI a giant prompt
        ↓
Generate everything
        ↓
Hope it works
```

The goal is:

```text
Master specification
        ↓
Version specification
        ↓
AI implementation
        ↓
Automated verification
        ↓
Human review
        ↓
Accepted version
        ↓
Next version
```

Each version should leave the repository in a usable state.

Each version should make the project more capable without making it unnecessarily more complicated.

The ultimate experiment is not whether AI can generate Portcheck.

The experiment is whether AI can **incrementally engineer, test, evolve, and maintain a real developer tool under explicit architectural constraints.**
