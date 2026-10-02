package cli

import (
	"runtime/debug"
	"strings"
)

const developmentVersion = "1.0.0-dev"

// Release tooling overrides this with -ldflags -X. A versioned go install uses
// the module version from Go build information. No dates/commits clutter stdout.
var version = developmentVersion

func applicationVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return version
	}
	return resolveVersion(version, info.Main.Version)
}

func resolveVersion(injected, module string) string {
	if injected != developmentVersion || !isReleaseVersion(module) {
		return injected
	}
	return strings.TrimPrefix(module, "v")
}

// isReleaseVersion reports whether module is a published release reference such
// as v1.0.0 or v1.0.0-rc.1. Go stamps local builds with a pseudo-version such as
// v0.0.0-20261002042414-a1b05a4a6bf6+dirty, which embeds a 14-digit timestamp
// and optional VCS build metadata. That is a build detail, not a release
// version, so such builds report the development version instead.
func isReleaseVersion(module string) bool {
	if !strings.HasPrefix(module, "v") {
		return false
	}
	for start := 0; start+14 <= len(module); start++ {
		if isDigits(module[start : start+14]) {
			return false
		}
	}
	return true
}

func isDigits(value string) bool {
	for _, c := range value {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
