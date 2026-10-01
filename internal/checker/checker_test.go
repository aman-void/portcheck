package checker

import (
	"errors"
	"fmt"
	"net"
	"syscall"
	"testing"
)

func TestCheckListenerLifecycle(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	port := listener.Addr().(*net.TCPAddr).Port
	if got := Check(port); got.Status != StatusInUse || got.Err != nil || got.Port != port {
		t.Fatalf("occupied: %+v", got)
	}
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	if got := Check(port); got.Status != StatusFree || got.Err != nil {
		t.Fatalf("free: %+v", got)
	}
	// Check must have released its listener, permitting a subsequent bind.
	rebound, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatalf("checker did not release port: %v", err)
	}
	rebound.Close()
}

func TestCheckInvalidPort(t *testing.T) {
	for _, port := range []int{-1, 0, 65536} {
		got := check(port, func(string, string) (net.Listener, error) {
			t.Fatal("invalid port reached network operation")
			return nil, nil
		})
		if got.Status != StatusError || got.Err == nil {
			t.Fatalf("port %d: %+v", port, got)
		}
	}
}

func TestCheckErrorClassification(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status Status
	}{
		{"occupied", syscall.EADDRINUSE, StatusInUse},
		{"permission", syscall.EACCES, StatusError},
		{"resources", syscall.EMFILE, StatusError},
		{"other", errors.New("unexpected failure"), StatusError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := check(8080, func(network, address string) (net.Listener, error) {
				if network != "tcp" || address != "127.0.0.1:8080" {
					t.Fatalf("unexpected listen: %s %s", network, address)
				}
				return nil, &net.OpError{Op: "listen", Net: "tcp", Err: fmt.Errorf("wrapped: %w", tc.err)}
			})
			if got.Status != tc.status {
				t.Fatalf("status = %v, want %v", got.Status, tc.status)
			}
			if tc.status == StatusError && !errors.Is(got.Err, tc.err) {
				t.Fatalf("lost error cause: %v", got.Err)
			}
		})
	}
}

type closeListener struct {
	net.Listener
	closed bool
	err    error
}

func (l *closeListener) Close() error {
	l.closed = true
	return l.err
}

func TestCheckClosesListener(t *testing.T) {
	for _, closeErr := range []error{nil, errors.New("close failed")} {
		listener := &closeListener{err: closeErr}
		got := check(8080, func(string, string) (net.Listener, error) { return listener, nil })
		if !listener.closed {
			t.Fatal("listener not closed")
		}
		if closeErr != nil && (got.Status != StatusError || !errors.Is(got.Err, closeErr)) {
			t.Fatalf("close error lost: %+v", got)
		}
	}
}
