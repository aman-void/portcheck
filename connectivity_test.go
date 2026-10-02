package portcheck

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestEndpointValidation(t *testing.T) {
	for _, address := range []string{"localhost:8080", "127.0.0.1:1", "example.com:443", "[::1]:65535", "[fe80::1%eth0]:80", "localhost:08080"} {
		if err := ValidateEndpoint(address); err != nil {
			t.Errorf("%q: %v", address, err)
		}
	}
	for _, address := range []string{"localhost", "localhost:", ":8080", "localhost:abc", "localhost:0", "localhost:65536", "[::1]:99999", "::1:80", "[localhost]:80", "127.0.0.999:80", "a\n:80", "localhost:+80", "localhost:-80", "localhost: 80", "localhost:80 ", "localhost:999999999999999999999", "https://example.com:443", "localhost:80-81", "", "[::1]:"} {
		result, err := connect(context.Background(), address, time.Second, func(context.Context, string, string) (net.Conn, error) {
			t.Fatal("invalid endpoint dialed")
			return nil, nil
		})
		if !errors.Is(err, ErrInvalidEndpoint) || result.Status != ConnectivityError || result.Err != err || result.Duration != 0 {
			t.Errorf("%q: %+v, %v", address, result, err)
		}
	}
}

func TestConnectValidationAndCancellation(t *testing.T) {
	for _, timeout := range []time.Duration{0, -time.Second} {
		result, err := connect(context.Background(), "localhost:80", timeout, func(context.Context, string, string) (net.Conn, error) {
			t.Fatal("invalid timeout dialed")
			return nil, nil
		})
		if !errors.Is(err, ErrInvalidTimeout) || result.Status != ConnectivityError {
			t.Fatalf("result=%+v err=%v", result, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := connect(ctx, "localhost:80", time.Second, func(context.Context, string, string) (net.Conn, error) {
		t.Fatal("pre-canceled operation dialed")
		return nil, nil
	})
	if !errors.Is(err, context.Canceled) || result.Status != ConnectivityError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestConnectControlledDial(t *testing.T) {
	for _, tc := range []struct {
		name   string
		cause  error
		status ConnectivityStatus
	}{
		{"refused", &net.OpError{Op: "dial", Net: "tcp", Err: &os.SyscallError{Syscall: "connect", Err: refusedTestError()}}, ConnectivityRefused},
		{"deadline", fmt.Errorf("wrapped: %w", context.DeadlineExceeded), ConnectivityTimeout},
		{"DNS", &net.DNSError{Err: "no such host", Name: "controlled.invalid", IsNotFound: true}, ConnectivityError},
		{"network timeout", &net.DNSError{Err: "controlled timeout", Name: "controlled.invalid", IsTimeout: true}, ConnectivityTimeout},
		{"network", errors.New("network failure"), ConnectivityError},
		{"cancel", fmt.Errorf("wrapped: %w", context.Canceled), ConnectivityError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			result, err := connect(context.Background(), "localhost:08080", time.Second, func(ctx context.Context, network, address string) (net.Conn, error) {
				calls++
				if network != "tcp" || address != "localhost:8080" {
					t.Fatalf("dial(%q,%q)", network, address)
				}
				if _, ok := ctx.Deadline(); !ok {
					t.Fatal("missing deadline")
				}
				return nil, tc.cause
			})
			if calls != 1 || result.Address != "localhost:08080" || result.Status != tc.status || !errors.Is(err, tc.cause) || result.Err != err || result.Duration < 0 {
				t.Fatalf("result=%+v err=%v calls=%d", result, err, calls)
			}
		})
	}
}

func TestConnectActiveTimeoutAndCancellation(t *testing.T) {
	for _, cancelActive := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		result, err := connect(ctx, "localhost:80", 5*time.Millisecond, func(ctx context.Context, _, _ string) (net.Conn, error) {
			if cancelActive {
				cancel()
			}
			<-ctx.Done()
			return nil, fmt.Errorf("dial: %w", ctx.Err())
		})
		status, cause := ConnectivityTimeout, context.DeadlineExceeded
		if cancelActive {
			status, cause = ConnectivityError, context.Canceled
		}
		if result.Status != status || !errors.Is(err, cause) {
			t.Fatalf("result=%+v err=%v", result, err)
		}
	}
}

type closingConn struct {
	net.Conn
	closed   bool
	closeErr error
}

func (c *closingConn) Close() error { c.closed = true; return c.closeErr }

func TestConnectClosesConnections(t *testing.T) {
	for _, closeErr := range []error{nil, io.ErrClosedPipe} {
		conn := &closingConn{closeErr: closeErr}
		result, err := connect(context.Background(), "localhost:80", time.Second, func(context.Context, string, string) (net.Conn, error) {
			return conn, nil
		})
		if !conn.closed || result.Duration < 0 {
			t.Fatalf("not closed: %+v", result)
		}
		if closeErr == nil {
			if result.Status != ConnectivityReachable || err != nil {
				t.Fatalf("result=%+v err=%v", result, err)
			}
		} else if result.Status != ConnectivityError || !errors.Is(err, closeErr) {
			t.Fatalf("result=%+v err=%v", result, err)
		}
	}
}

func TestConnectInheritedDeadline(t *testing.T) {
	deadline := time.Now().Add(time.Hour)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	_, err := connect(ctx, "localhost:80", 2*time.Hour, func(ctx context.Context, _, _ string) (net.Conn, error) {
		if got, ok := ctx.Deadline(); !ok || !got.Equal(deadline) {
			t.Fatalf("deadline=%v, %v; want %v", got, ok, deadline)
		}
		return nil, context.DeadlineExceeded
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
}

func TestConnectCleanupOnDialFailure(t *testing.T) {
	conn := &closingConn{}
	cause := errors.New("dial failed")
	result, err := connect(context.Background(), "localhost:80", time.Second, func(context.Context, string, string) (net.Conn, error) {
		return conn, cause
	})
	if !conn.closed || result.Status != ConnectivityError || !errors.Is(err, cause) {
		t.Fatalf("closed=%v result=%+v err=%v", conn.closed, result, err)
	}
	result, err = connect(context.Background(), "localhost:80", time.Second, func(context.Context, string, string) (net.Conn, error) {
		return nil, nil
	})
	if result.Status != ConnectivityError || err == nil {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestConnectRealListeners(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "::1"} {
		t.Run(host, func(t *testing.T) {
			listener, err := net.Listen("tcp", net.JoinHostPort(host, "0"))
			if err != nil {
				t.Skipf("loopback unavailable: %v", err)
			}
			t.Cleanup(func() { listener.Close() })
			address := listener.Addr().String()
			result, err := Connect(context.Background(), address, time.Second)
			if err != nil || result.Status != ConnectivityReachable || result.Duration < 0 {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			// No payload and immediate client closure: the accepted peer sees EOF.
			conn, err := listener.Accept()
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
				t.Fatal(err)
			}
			var b [1]byte
			if n, err := conn.Read(b[:]); n != 0 || err != io.EOF {
				t.Fatalf("read=%d,%v; expected EOF without payload", n, err)
			}
			if host == "127.0.0.1" {
				local := net.JoinHostPort("localhost", strconv.Itoa(listener.Addr().(*net.TCPAddr).Port))
				if result, err := Connect(context.Background(), local, time.Second); err != nil || result.Status != ConnectivityReachable {
					t.Fatalf("localhost result=%+v err=%v", result, err)
				}
			}
			if err := listener.Close(); err != nil {
				t.Fatal(err)
			}
			result, err = Connect(context.Background(), address, time.Second)
			if result.Status != ConnectivityRefused || err == nil {
				t.Fatalf("closed result=%+v err=%v", result, err)
			}
		})
	}
}
