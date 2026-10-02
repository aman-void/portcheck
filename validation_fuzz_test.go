package portcheck

import (
	"errors"
	"testing"
)

func FuzzValidateHost(f *testing.F) {
	for _, seed := range []string{"127.0.0.1", "::1", "fe80::1%eth0", "localhost", "", "999.1.1.1", "a\x1b", "a..b"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, host string) {
		if err := ValidateHost(host); err != nil && !errors.Is(err, ErrInvalidHost) {
			t.Fatalf("validation lost sentinel: %v", err)
		}
	})
}

func FuzzValidateEndpoint(f *testing.F) {
	for _, seed := range []string{"localhost:8080", "[::1]:80", "[fe80::1%eth0]:65535", "localhost", ":80", "a:999999999999999999999", ""} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, address string) {
		if err := ValidateEndpoint(address); err != nil && !errors.Is(err, ErrInvalidEndpoint) {
			t.Fatalf("validation lost sentinel: %v", err)
		}
	})
}
