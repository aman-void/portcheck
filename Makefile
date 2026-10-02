.DEFAULT_GOAL := help

# Enable color only in a terminal. Override with COLOR=1 or disable with NO_COLOR=1.
COLOR ?= auto

# The source default is 1.0.0-dev. Only explicit builds inject a release version.
VERSION ?=
LDFLAGS = $(if $(VERSION),-X github.com/aman-void/portcheck/internal/cli.version=$(VERSION))

# Detect the recipe's stdout, not $(shell ...), whose stdout Make captures.
define colors
cyan=; green=; bold=; reset=; \
if [ -z "$(NO_COLOR)" ] && { [ "$(COLOR)" = 1 ] || \
    { [ "$(COLOR)" = auto ] && [ -t 1 ] && [ "$$TERM" != dumb ]; }; }; then \
    cyan='\033[36m'; green='\033[32m'; bold='\033[1m'; reset='\033[0m'; \
fi;
endef

announce = @$(colors) printf '%b%s%b\n' "$$cyan" '==> $(1)' "$$reset"
success = @$(colors) printf '%b%s%b\n' "$$green" ' OK $(1)' "$$reset"

.PHONY: help fmt test vet build run clean release

help:
	@$(colors) printf '\n%b%s%b\n' "$$bold$$cyan" 'Portcheck · development commands' "$$reset"
	@$(colors) printf '\n  %b%-12s%b %s\n' "$$green" 'make fmt' "$$reset" 'Format Go source'
	@$(colors) printf '  %b%-12s%b %s\n' "$$green" 'make test' "$$reset" 'Run all tests'
	@$(colors) printf '  %b%-12s%b %s\n' "$$green" 'make vet' "$$reset" 'Run Go static checks'
	@$(colors) printf '  %b%-12s%b %s\n' "$$green" 'make build' "$$reset" 'Build bin/portcheck'
	@$(colors) printf '  %b%-12s%b %s\n' "$$green" 'make run' "$$reset" 'Run from source with ARGS="..."'
	@$(colors) printf '  %b%-12s%b %s\n' "$$green" 'make clean' "$$reset" 'Remove known binary outputs'
	@$(colors) printf '  %b%-12s%b %s\n' "$$green" 'make release' "$$reset" 'Build archives/checksums; VERSION and OUT required'
	@$(colors) printf '\n  %b%s%b\n' "$$bold" 'Examples' "$$reset"
	@printf '    make fmt test vet build\n    make run ARGS="--help"\n    ./bin/portcheck 3000 8080\n\n'
	@printf '  Color: automatic; COLOR=1 forces it, NO_COLOR=1 disables it.\n\n'

fmt:
	$(call announce,Formatting Go source)
	@gofmt -w .
	$(call success,Formatting complete)

test:
	$(call announce,Running tests)
	@go test ./...
	$(call success,All tests passed)

vet:
	$(call announce,Checking Go source)
	@go vet ./...
	$(call success,Static checks passed)

build:
	$(call announce,Building Portcheck)
	@go build -trimpath -ldflags '$(LDFLAGS)' -o bin/portcheck ./cmd/portcheck
	$(call success,Binary ready: ./bin/portcheck)

run:
	$(call announce,Running Portcheck from source)
	@go run ./cmd/portcheck $(ARGS)

# Packaging is a developer-only Bash script, never invoked by the CLI/library.
release:
	@bash scripts/release.sh '$(VERSION)' '$(OUT)'

clean:
	$(call announce,Removing binary outputs)
	@rm -f bin/portcheck bin/portcheck.exe portcheck portcheck.exe
	$(call success,Binary outputs removed)
