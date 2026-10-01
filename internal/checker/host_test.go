package checker

import (
	"errors"
	"net"
	"syscall"
	"testing"
)

func TestHostAddressConstruction(t *testing.T) {
	for _, tc := range []struct{ host, address string }{
		{"127.0.0.1", "127.0.0.1:8080"}, {"0.0.0.0", "0.0.0.0:8080"},
		{"::1", "[::1]:8080"}, {"::", "[::]:8080"},
		{"fe80::1%eth0", "[fe80::1%eth0]:8080"}, {"localhost", "localhost:8080"},
	} {
		got := checkHost(tc.host, 8080, func(network, address string) (net.Listener, error) {
			if network != "tcp" || address != tc.address {
				t.Fatalf("%s %s", network, address)
			}
			return nil, &net.OpError{Err: syscall.EADDRINUSE}
		})
		if got.Status != StatusInUse || got.Err != nil {
			t.Fatalf("%+v", got)
		}
	}
	err := &net.OpError{Err: syscall.EADDRNOTAVAIL}
	got := checkHost("::1", 8080, func(string, string) (net.Listener, error) { return nil, err })
	if got.Status != StatusError || !errors.Is(got.Err, syscall.EADDRNOTAVAIL) {
		t.Fatalf("%+v", got)
	}
}
