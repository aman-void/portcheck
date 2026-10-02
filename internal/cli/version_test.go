package cli

import "testing"

func TestVersionSources(t *testing.T) {
	for _, tc := range []struct{ injected, module, want string }{
		{developmentVersion, "", developmentVersion},
		{developmentVersion, "(devel)", developmentVersion},
		{developmentVersion, "v1.0.0", "1.0.0"},
		{developmentVersion, "v1.0.0-rc.1", "1.0.0-rc.1"},
		{developmentVersion, "v1.2.3", "1.2.3"},
		{"1.0.0-rc.2", "v1.0.0-rc.1", "1.0.0-rc.2"},
		{"1.0.0", "(devel)", "1.0.0"},
		// Local VCS-stamped builds are not release versions.
		{developmentVersion, "v0.0.0-20261002042414-a1b05a4a6bf6+dirty", developmentVersion},
		{developmentVersion, "v0.0.0-20261002042414-a1b05a4a6bf6", developmentVersion},
		{developmentVersion, "v1.2.3-0.20261002042414-a1b05a4a6bf6+dirty", developmentVersion},
		{developmentVersion, "1.0.0", developmentVersion},
		{"1.0.0", "v0.0.0-20261002042414-a1b05a4a6bf6+dirty", "1.0.0"},
	} {
		if got := resolveVersion(tc.injected, tc.module); got != tc.want {
			t.Fatalf("%+v: got %q", tc, got)
		}
	}
}

// The 14-digit rule must not reject ordinary short numeric prerelease segments.
func TestIsReleaseVersion(t *testing.T) {
	for _, tc := range []struct {
		module string
		want   bool
	}{
		{"v1.0.0", true},
		{"v1.0.0-rc.1", true},
		{"v10.20.30", true},
		{"v1.0.0-20261002", true},
		{"v0.0.0-20261002042414-abcdefabcdef", false},
		{"v1.0.0+dirty", true},
		{"", false},
		{"1.0.0", false},
	} {
		if got := isReleaseVersion(tc.module); got != tc.want {
			t.Fatalf("%q: got %v, want %v", tc.module, got, tc.want)
		}
	}
}
