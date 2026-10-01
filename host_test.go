package portcheck

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"

	"github.com/aman-void/portcheck/internal/checker"
)

func TestValidateHost(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "0.0.0.0", "::", "::1", "fe80::1%eth0", "localhost", "LOCALHOST.", "my-host.example", "xn--bcher-kva.example"} {
		if err := ValidateHost(host); err != nil {
			t.Errorf("%q: %v", host, err)
		}
	}
	for _, host := range []string{"", " ", " localhost", "localhost\n", "[::1]", "127.0.0.1:80", "localhost:80", "999.1.1.1", "127.1", "::gg", "fe80::1%", "fe80::1%eth\x1b0", "foo/bar", "foo\\bar", "a..b", "-a", "a-", "a_b", "https://localhost", "a\x1b", strings.Repeat("a", 64), strings.Repeat("a.", 127) + "ab"} {
		if err := ValidateHost(host); !errors.Is(err, ErrInvalidHost) {
			t.Errorf("%q: %v", host, err)
		}
	}
}

func TestHostValidationBeforeBind(t *testing.T) {
	ctx := context.Background()
	got, err := CheckHost(ctx, "", 8080)
	if !errors.Is(err, ErrInvalidHost) || got.Status != StatusError || got.Err != err {
		t.Fatalf("%+v %v", got, err)
	}
	for _, tc := range []struct {
		host  string
		ports []int
		want  error
	}{
		{"", []int{8080}, ErrInvalidHost},
		{"127.0.0.1", []int{8080, 0}, ErrInvalidPort},
	} {
		got, err := CheckPortsHost(ctx, tc.host, tc.ports)
		if len(got) != 0 || !errors.Is(err, tc.want) {
			t.Fatalf("%+v %v", got, err)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	got, err = CheckHost(canceled, "::1", 8080)
	if !errors.Is(err, context.Canceled) || got.Err != err {
		t.Fatalf("%+v %v", got, err)
	}
	results, err := CheckPortsHost(canceled, "::1", []int{8080})
	if len(results) != 0 || !errors.Is(err, context.Canceled) {
		t.Fatalf("%+v %v", results, err)
	}
	results, err = CheckPortsHost(ctx, "::1", nil)
	if results == nil || len(results) != 0 || err != nil {
		t.Fatalf("empty: %+v %v", results, err)
	}
}

func TestCheckHostRealListeners(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "::1"} {
		t.Run(host, func(t *testing.T) {
			listener, err := net.Listen("tcp", net.JoinHostPort(host, "0"))
			if err != nil {
				if host == "::1" {
					t.Skipf("IPv6 loopback unavailable: %v", err)
				}
				t.Fatal(err)
			}
			t.Cleanup(func() { listener.Close() })
			port := listener.Addr().(*net.TCPAddr).Port
			ctx := context.Background()
			got, err := CheckHost(ctx, host, port)
			if err != nil || got.Status != StatusInUse {
				t.Fatalf("occupied: %+v %v", got, err)
			}
			results, err := CheckPortsHost(ctx, host, []int{port, port})
			if err != nil || len(results) != 1 ||
				results[0].Port != got.Port || results[0].Status != got.Status ||
				results[0].Err != nil {
				t.Fatalf("batch: %+v %v", results, err)
			}
			if err := listener.Close(); err != nil {
				t.Fatal(err)
			}
			got, err = CheckHost(ctx, host, port)
			if err != nil || got.Status != StatusFree {
				t.Fatalf("free: %+v %v", got, err)
			}
			rebound, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
			if err != nil {
				t.Fatalf("listener leaked: %v", err)
			}
			rebound.Close()
		})
	}
}

func TestCheckHostnameRealListener(t *testing.T) {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	got, err := CheckHost(context.Background(), "localhost", port)
	if err != nil || got.Status != StatusInUse {
		t.Fatalf("hostname: %+v %v", got, err)
	}
}

func TestCheckHostPreservesFailureOnCancellation(t *testing.T) {
	failure := errors.New("listener close failed")
	for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
		for _, tc := range []struct {
			name   string
			status checker.Status
			err    error
		}{
			{"free", checker.StatusFree, nil},
			{"in use", checker.StatusInUse, nil},
			{"system", checker.StatusError, &net.OpError{Op: "close", Net: "tcp", Err: failure}},
			{"context", checker.StatusError, fmt.Errorf("listen: %w", cause)},
			{"joined", checker.StatusError, errors.Join(failure, cause)},
		} {
			t.Run(fmt.Sprintf("%v/%s", cause, tc.name), func(t *testing.T) {
				parent, cancel := context.WithCancel(context.Background())
				defer cancel()
				ctx := batchCancellationContext{Context: parent, cause: cause}
				got, err := checkHost(ctx, "::1", 8080, func(_ context.Context, host string, port int) checker.Result {
					if host != "::1" || port != 8080 {
						t.Fatalf("host=%q port=%d", host, port)
					}
					cancel()
					return checker.Result{Port: port, Status: tc.status, Err: tc.err}
				})
				if got.Port != 8080 || got.Status != StatusError || got.Err != err || !errors.Is(err, cause) {
					t.Fatalf("got=%+v err=%v", got, err)
				}
				if errors.Is(tc.err, failure) && !errors.Is(err, failure) {
					t.Fatalf("lost system cause: %v", err)
				}
				if tc.err != nil && !errors.Is(err, tc.err) {
					t.Fatalf("lost original error: %v", err)
				}
				var original, preserved *net.OpError
				if errors.As(tc.err, &original) && (!errors.As(err, &preserved) || preserved != original) {
					t.Fatalf("lost wrapped networking error: %v", err)
				}
				if strings.Count(err.Error(), cause.Error()) != 1 {
					t.Fatalf("duplicate context cause: %v", err)
				}
			})
		}
	}
}
