package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aman-void/portcheck"
)

func TestV03Parse(t *testing.T) {
	opts, err := parse([]string{"8080-8081", "--host", "::1", "--wait-in-use", "--interval", "500ms", "--timeout", "30s", "8080"})
	if err != nil || opts.host != "::1" || !opts.inUse || opts.interval != 500*time.Millisecond || opts.timeout != 30*time.Second || !reflect.DeepEqual(opts.ports, []int{8080, 8081}) {
		t.Fatalf("%+v %v", opts, err)
	}
	opts, err = parse([]string{"8080"})
	if err != nil || opts.host != "127.0.0.1" || opts.interval != time.Second || opts.timeout != 0 {
		t.Fatalf("%+v %v", opts, err)
	}
}

func TestV03InvalidInputBeforeCheck(t *testing.T) {
	cases := [][]string{
		{"--host"}, {"--host", "", "8080"}, {"--host", "[::1]", "8080"},
		{"--host", "999.1.1.1", "8080"}, {"--host", "localhost:80", "8080"},
		{"--watch", "--wait", "8080"}, {"--watch", "--wait-in-use", "8080"},
		{"--wait", "--wait-in-use", "8080"}, {"--watch", "--find", "8080"},
		{"--wait", "--find", "8080"}, {"--wait-in-use", "--find", "8080"},
		{"--watch", "--json", "8080"}, {"--wait", "--quiet", "--json", "8080"},
		{"--interval", "1s", "8080"}, {"--find", "--interval", "1s", "8080"},
		{"--timeout", "1s", "8080"}, {"--watch", "--timeout", "1s", "8080"},
		{"--wait", "--timeout"}, {"--watch", "--interval"}, {"--wait"},
		{"--wait", "8080", "bad"}, {"--watch", "--host", "-q", "8080"},
	}
	for _, value := range []string{"0", "0s", "-1s", "bad", "999999999999999999999s"} {
		cases = append(cases, []string{"--wait", "--timeout", value, "8080"}, []string{"--watch", "--interval", value, "8080"})
	}
	cases = append(cases, []string{"--watch", "--interval", "1ns", "8080"})
	for _, args := range cases {
		t.Run(fmt.Sprint(args), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := runContext(context.Background(), args, &stdout, &stderr, func(context.Context, string, int) portcheck.Result {
				t.Fatal("invalid input checked")
				return portcheck.Result{}
			})
			if code != 2 || stdout.Len() != 0 || stderr.Len() == 0 {
				t.Fatalf("code=%d out=%q err=%q", code, &stdout, &stderr)
			}
		})
	}
}

func TestHostPassedToEveryMode(t *testing.T) {
	for _, mode := range []string{"", "--find", "--wait", "--wait-in-use"} {
		args := []string{"--host", "::1", "8080"}
		if mode != "" {
			args = append(args, mode)
		}
		var stdout, stderr bytes.Buffer
		calls := 0
		code := runContext(context.Background(), args, &stdout, &stderr, func(_ context.Context, host string, port int) portcheck.Result {
			calls++
			if host != "::1" || port != 8080 {
				t.Fatalf("host=%s port=%d", host, port)
			}
			status := portcheck.StatusFree
			if mode == "--wait-in-use" {
				status = portcheck.StatusInUse
			}
			return portcheck.Result{Port: port, Status: status}
		})
		if code != 0 || calls != 1 || stderr.Len() != 0 {
			t.Fatalf("%s: %d %d %q", mode, code, calls, &stderr)
		}
	}
}

func TestWatchExpandedOrder(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var checked []int
	var stderr bytes.Buffer
	stdout := &callbackWriter{after: func() {
		if len(checked) == 3 {
			cancel()
		}
	}}
	code := runContext(ctx, []string{"--watch", "--host", "::1", "3002", "3000-3002", "3001"}, stdout, &stderr, func(_ context.Context, host string, port int) portcheck.Result {
		if host != "::1" {
			t.Fatalf("host=%q", host)
		}
		checked = append(checked, port)
		return portcheck.Result{Port: port, Status: portcheck.StatusFree}
	})
	if code != 0 || stderr.Len() != 0 || !reflect.DeepEqual(checked, []int{3002, 3000, 3001}) || strings.Count(stdout.String(), "FREE\n") != 3 {
		t.Fatalf("code=%d checked=%v out=%q err=%q", code, checked, stdout.String(), &stderr)
	}
}

type callbackWriter struct {
	bytes.Buffer
	after func()
}

func (w *callbackWriter) Write(p []byte) (int, error) {
	n, err := w.Buffer.Write(p)
	if w.after != nil {
		w.after()
	}
	return n, err
}

func TestWatchChangesAndErrors(t *testing.T) {
	for _, quiet := range []bool{false, true} {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		opts := options{host: "::1", watch: true, quiet: quiet, ports: []int{8080, 3000}, interval: time.Second}
		ticks := make(chan time.Time, 5)
		for range 5 {
			ticks <- time.Time{}
		}
		cycle := 0
		states := []portcheck.Status{portcheck.StatusInUse, portcheck.StatusInUse, portcheck.StatusError, portcheck.StatusError, portcheck.StatusError, portcheck.StatusFree}
		var stderr bytes.Buffer
		writes := 0
		stdout := &callbackWriter{after: func() {
			writes++
			if writes == 5 {
				cancel()
			}
		}}
		code := runPolling(ctx, opts, stdout, &stderr, func(_ context.Context, host string, port int) portcheck.Result {
			if host != "::1" {
				t.Fatalf("host=%s", host)
			}
			result := portcheck.Result{Port: port, Status: portcheck.StatusFree}
			if port == 8080 {
				result.Status = states[cycle]
				if cycle == 2 || cycle == 3 {
					result.Err = errors.New("failure A")
				}
				if cycle == 4 {
					result.Err = errors.New("failure B")
				}
			} else {
				cycle++
			}
			return result
		}, ticks, func() time.Time { return time.Date(2026, 10, 1, 15, 4, 5, 0, time.UTC) })
		if code != 3 || writes != 5 || strings.Count(stderr.String(), "failure A") != 1 || strings.Count(stderr.String(), "failure B") != 1 {
			t.Fatalf("code=%d writes=%d out=%q err=%q", code, writes, stdout.String(), &stderr)
		}
		want := "2026-10-01 15:04:05  ::1  8080 IN USE\n2026-10-01 15:04:05  ::1  3000 FREE\n2026-10-01 15:04:05  ::1  8080 ERROR\n2026-10-01 15:04:05  ::1  8080 ERROR\n2026-10-01 15:04:05  ::1  8080 FREE\n"
		if quiet {
			want = "IN_USE\nFREE\nERROR\nERROR\nFREE\n"
		}
		if stdout.String() != want {
			t.Fatalf("out=%q want=%q", stdout.String(), want)
		}
	}
}

func TestWatchBusyObservedInstantTransitionsAndReset(t *testing.T) {
	for _, quiet := range []bool{false, true} {
		t.Run(fmt.Sprintf("quiet=%v", quiet), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			states := []portcheck.Status{
				portcheck.StatusFree, portcheck.StatusInUse, portcheck.StatusInUse,
				portcheck.StatusFree, portcheck.StatusInUse, portcheck.StatusError,
				portcheck.StatusInUse,
			}
			ticks := make(chan time.Time, len(states))
			for range len(states) {
				ticks <- time.Time{}
			}
			base := time.Date(2026, 10, 2, 8, 14, 0, 0, time.UTC)
			// Observation and emission are separately controlled. The long gaps
			// between transitions must not be included in the new busy period.
			offsets := []time.Duration{0, 10 * time.Second, 12900 * time.Millisecond, 100 * time.Second,
				200 * time.Second, 201900 * time.Millisecond, 300 * time.Second,
				400 * time.Second, 400700 * time.Millisecond}
			clockCalls, checks := 0, 0
			var out, diag bytes.Buffer
			code := runPolling(ctx, options{watch: true, quiet: quiet, host: "127.0.0.1", ports: []int{8080}}, &out, &diag,
				func(context.Context, string, int) portcheck.Result {
					if checks == len(states) {
						cancel()
						return portcheck.Result{Port: 8080, Status: portcheck.StatusError, Err: context.Canceled}
					}
					result := portcheck.Result{Port: 8080, Status: states[checks]}
					if result.Status == portcheck.StatusError {
						result.Err = errors.New("controlled failure")
					}
					checks++
					return result
				}, ticks, func() time.Time {
					if clockCalls == len(offsets) {
						t.Fatal("unexpected clock call")
					}
					instant := base.Add(offsets[clockCalls])
					clockCalls++
					return instant
				})
			want := "2026-10-02 08:14:00  127.0.0.1  8080 FREE\n" +
				"2026-10-02 08:14:12  127.0.0.1  8080 IN USE  observed since 08:14:10\n" +
				"2026-10-02 08:15:40  127.0.0.1  8080 FREE\n" +
				"2026-10-02 08:17:21  127.0.0.1  8080 IN USE  observed since 08:17:20\n" +
				"2026-10-02 08:19:00  127.0.0.1  8080 ERROR\n" +
				"2026-10-02 08:20:40  127.0.0.1  8080 IN USE  observed since 08:20:40\n"
			wantClockCalls := len(offsets)
			if quiet {
				want = "FREE\nIN_USE\nFREE\nIN_USE\nERROR\nIN_USE\n"
				wantClockCalls = 6
			}
			if code != 3 || checks != len(states) || out.String() != want || clockCalls != wantClockCalls || diag.String() != "error: failed to check 127.0.0.1 port 8080: controlled failure\n" {
				t.Fatalf("code=%d checks=%d clock=%d out=%q diag=%q", code, checks, clockCalls, &out, &diag)
			}
		})
	}
}

func TestBusyMarkerDoesNotChangeOneShotOutput(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"8080"}, "PORT    STATUS\n8080    IN USE\n"},
		{[]string{"--json", "8080"}, "[{\"port\":8080,\"status\":\"in_use\"}]\n"},
		{[]string{"--quiet", "8080"}, "IN_USE\n"},
		{[]string{"--wait-in-use", "--quiet", "8080"}, "IN_USE\n"},
	} {
		var out, diag bytes.Buffer
		code := runContext(context.Background(), tc.args, &out, &diag, func(context.Context, string, int) portcheck.Result {
			return portcheck.Result{Port: 8080, Status: portcheck.StatusInUse}
		})
		wantCode := 1
		if tc.args[0] == "--wait-in-use" {
			wantCode = 0
		}
		if code != wantCode || out.String() != tc.want || diag.Len() != 0 {
			t.Fatalf("%v: code=%d out=%q diag=%q", tc.args, code, &out, &diag)
		}
	}
}

func TestWaitAllPortsSameCycle(t *testing.T) {
	for _, inUse := range []bool{false, true} {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		opts := options{host: "127.0.0.1", wait: !inUse, inUse: inUse, json: true, ports: []int{3000, 3001}, interval: time.Second}
		ticks := make(chan time.Time, 2)
		ticks <- time.Time{}
		ticks <- time.Time{}
		cycle, calls := 0, 0
		var stdout, stderr bytes.Buffer
		code := runPolling(ctx, opts, &stdout, &stderr, func(_ context.Context, _ string, port int) portcheck.Result {
			calls++
			if stdout.Len() != 0 {
				t.Fatal("wait printed intermediate state")
			}
			match := cycle == 2 || (cycle == 0 && port == 3000) || (cycle == 1 && port == 3001)
			status := portcheck.StatusInUse
			if match != inUse {
				status = portcheck.StatusFree
			}
			if port == 3001 {
				cycle++
			}
			return portcheck.Result{Port: port, Status: status}
		}, ticks, time.Now)
		var records []struct {
			Port   int
			Status portcheck.Status
		}
		if code != 0 || calls != 6 || stderr.Len() != 0 || json.Unmarshal(stdout.Bytes(), &records) != nil || len(records) != 2 {
			t.Fatalf("code=%d calls=%d out=%q err=%q", code, calls, &stdout, &stderr)
		}
		want := portcheck.StatusFree
		if inUse {
			want = portcheck.StatusInUse
		}
		for _, record := range records {
			if record.Status != want {
				t.Fatalf("%+v", records)
			}
		}
	}
}

func TestPollingCancellationAndFailures(t *testing.T) {
	for _, watch := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		var stdout, stderr bytes.Buffer
		opts := options{watch: watch, wait: !watch, interval: time.Second, ports: []int{8080}}
		code := runPolling(ctx, opts, &stdout, &stderr, func(context.Context, string, int) portcheck.Result {
			t.Fatal("checked canceled context")
			return portcheck.Result{}
		}, nil, time.Now)
		want := 1
		if watch {
			want = 0
		}
		if code != want || stdout.Len() != 0 || stderr.Len() != 0 {
			t.Fatalf("%d %q %q", code, &stdout, &stderr)
		}
	}
	for _, args := range [][]string{{"--watch", "8080"}, {"--wait", "8080"}, {"--wait-in-use", "8080"}, {"--wait", "--json", "8080"}} {
		var stderr bytes.Buffer
		code := runContext(context.Background(), args, failingWriter{}, &stderr, func(_ context.Context, _ string, port int) portcheck.Result {
			status := portcheck.StatusFree
			if args[0] == "--wait-in-use" {
				status = portcheck.StatusInUse
			}
			return portcheck.Result{Port: port, Status: status}
		})
		if code != 3 || !strings.Contains(stderr.String(), "write output") {
			t.Fatalf("%v: %d %q", args, code, &stderr)
		}
	}
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), []string{"--wait", "--json", "3000-3001"}, &stdout, &stderr, func(_ context.Context, _ string, port int) portcheck.Result {
		return portcheck.Result{Port: port, Status: portcheck.StatusError, Err: errors.New("permission denied")}
	})
	if code != 3 || !json.Valid(stdout.Bytes()) || strings.Count(stderr.String(), "permission denied") != 2 {
		t.Fatalf("%d %q %q", code, &stdout, &stderr)
	}
}

func TestWaitRealListenerTransitions(t *testing.T) {
	for _, inUse := range []bool{false, true} {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		port := listener.Addr().(*net.TCPAddr).Port
		address := listener.Addr().String()
		if inUse {
			listener.Close()
		}
		defer func() { listener.Close() }()
		mode := "--wait"
		if inUse {
			mode = "--wait-in-use"
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		var stdout, stderr bytes.Buffer
		calls := 0
		code := runContext(ctx, []string{mode, "--quiet", "--interval", "100ms", strconv.Itoa(port)}, &stdout, &stderr, func(ctx context.Context, host string, port int) portcheck.Result {
			calls++
			result, _ := portcheck.CheckHost(ctx, host, port)
			if calls == 1 {
				if inUse {
					listener, err = net.Listen("tcp", address)
					if err != nil {
						t.Fatal(err)
					}
				} else {
					listener.Close()
				}
			}
			return result
		})
		want := "FREE\n"
		if inUse {
			want = "IN_USE\n"
		}
		if code != 0 || calls != 2 || stdout.String() != want || stderr.Len() != 0 {
			t.Fatalf("%d calls=%d %q %q", code, calls, &stdout, &stderr)
		}
	}
}

func TestWaitRealTimeout(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	var stdout, stderr bytes.Buffer
	start := time.Now()
	code := Run([]string{"--wait", "--json", "--timeout", "20ms", strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)}, &stdout, &stderr)
	if code != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "timeout waiting") || time.Since(start) > time.Second {
		t.Fatalf("%d %q %q elapsed=%v", code, &stdout, &stderr, time.Since(start))
	}
}

func TestWatchAndWaitCancellationBetweenPorts(t *testing.T) {
	for _, mode := range []string{"--watch", "--wait"} {
		ctx, cancel := context.WithCancel(context.Background())
		var stdout, stderr bytes.Buffer
		calls := 0
		code := runContext(ctx, []string{mode, "3000-3002"}, &stdout, &stderr, func(context.Context, string, int) portcheck.Result {
			calls++
			cancel()
			return portcheck.Result{Status: portcheck.StatusError, Err: context.Canceled}
		})
		want := 1
		if mode == "--watch" {
			want = 0
		}
		if code != want || calls != 1 || stderr.Len() != 0 {
			t.Fatalf("%s %d calls=%d %q", mode, code, calls, &stderr)
		}
	}
}

func TestPollingCancellationWithoutTick(t *testing.T) {
	for _, watch := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		var stdout, stderr bytes.Buffer
		checked := make(chan struct{}, 1)
		done := make(chan int, 1)
		go func() {
			done <- runPolling(ctx, options{watch: watch, wait: !watch, host: "127.0.0.1", ports: []int{8080}, interval: time.Hour}, &stdout, &stderr,
				func(context.Context, string, int) portcheck.Result {
					checked <- struct{}{}
					return portcheck.Result{Port: 8080, Status: portcheck.StatusInUse}
				}, make(chan time.Time), time.Now)
		}()
		select {
		case <-checked:
		case <-time.After(time.Second):
			cancel()
			t.Fatal("initial check did not run")
		}
		cancel()
		select {
		case code := <-done:
			want := 1
			if watch {
				want = 0
			}
			if code != want || stderr.Len() != 0 {
				t.Fatalf("code=%d stderr=%q", code, &stderr)
			}
		case <-time.After(time.Second):
			t.Fatal("cancellation blocked waiting for a tick")
		}
	}
}

type pollingCancellationContext struct {
	context.Context
	cause error
}

func (ctx pollingCancellationContext) Err() error {
	if ctx.Context.Err() != nil {
		return ctx.cause
	}
	return nil
}

func TestPollingCancellationPreservesSystemFailures(t *testing.T) {
	failure := errors.New("bind permission denied")
	for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
		for _, mode := range []string{"--watch", "--wait", "--wait-in-use"} {
			for _, tc := range []struct {
				name   string
				err    error
				system bool
			}{
				{"success", nil, false},
				{"context", fmt.Errorf("listen: %w", cause), false},
				{"joined contexts", fmt.Errorf("listen: %w", errors.Join(context.Canceled, context.DeadlineExceeded)), false},
				{"system", fmt.Errorf("listen: %w", failure), true},
				{"joined system and context", fmt.Errorf("listen: %w", errors.Join(failure, cause)), true},
			} {
				t.Run(fmt.Sprintf("%v/%s/%s", cause, mode, tc.name), func(t *testing.T) {
					parent, cancel := context.WithCancel(context.Background())
					defer cancel()
					ctx := pollingCancellationContext{Context: parent, cause: cause}
					var stdout, stderr bytes.Buffer
					calls := 0
					code := runContext(ctx, []string{mode, "--host", "::1", "--quiet", "3000-3001"}, &stdout, &stderr,
						func(_ context.Context, host string, port int) portcheck.Result {
							calls++
							if host != "::1" {
								t.Fatalf("host=%q", host)
							}
							cancel()
							status := portcheck.StatusFree
							if mode == "--wait-in-use" {
								status = portcheck.StatusInUse
							}
							if tc.err != nil {
								status = portcheck.StatusError
							}
							return portcheck.Result{Port: port, Status: status, Err: tc.err}
						})
					want := 1
					if mode == "--watch" {
						want = 0
					}
					if tc.system {
						want = 3
					}
					if code != want || calls != 1 || stdout.Len() != 0 {
						t.Fatalf("code=%d want=%d calls=%d out=%q err=%q", code, want, calls, &stdout, &stderr)
					}
					if tc.system {
						if !strings.Contains(stderr.String(), failure.Error()) {
							t.Fatalf("lost system diagnostic: %q", &stderr)
						}
					} else if cause == context.DeadlineExceeded && mode != "--watch" {
						if !strings.Contains(stderr.String(), "timeout waiting") {
							t.Fatalf("missing timeout: %q", &stderr)
						}
					} else if stderr.Len() != 0 {
						t.Fatalf("cancellation diagnostic: %q", &stderr)
					}
				})
			}
		}
	}
}

func TestWaitCancellationBeforeSuccessfulCommit(t *testing.T) {
	for _, mode := range []string{"--wait", "--wait-in-use"} {
		ctx, cancel := context.WithCancel(context.Background())
		var stdout, stderr bytes.Buffer
		code := runContext(ctx, []string{mode, "--json", "8080"}, &stdout, &stderr, func(context.Context, string, int) portcheck.Result {
			cancel()
			status := portcheck.StatusFree
			if mode == "--wait-in-use" {
				status = portcheck.StatusInUse
			}
			return portcheck.Result{Port: 8080, Status: status}
		})
		if code != 1 || stdout.Len() != 0 || stderr.Len() != 0 {
			t.Fatalf("%s: code=%d out=%q err=%q", mode, code, &stdout, &stderr)
		}
	}
}

func TestWaitCompletionOnceOutputStarts(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stdout := &callbackWriter{after: cancel}
	var stderr bytes.Buffer
	code := runContext(ctx, []string{"--wait", "--json", "8080"}, stdout, &stderr, func(context.Context, string, int) portcheck.Result {
		return portcheck.Result{Port: 8080, Status: portcheck.StatusFree}
	})
	if code != 0 || stderr.Len() != 0 || !json.Valid(stdout.Bytes()) {
		t.Fatalf("code=%d out=%q err=%q", code, stdout.String(), &stderr)
	}
}

// Arrange cancellation after the post-check observation, at the next context
// observation. This verifies the pre-output guard without a scheduler race.
type commitCancellationContext struct {
	context.Context
	cancel       context.CancelFunc
	observations int
}

func (ctx *commitCancellationContext) Err() error {
	ctx.observations++
	if ctx.observations == 4 {
		ctx.cancel()
	}
	return ctx.Context.Err()
}

func TestWaitChecksCancellationAgainBeforeOutput(t *testing.T) {
	for _, mode := range []string{"--wait", "--wait-in-use"} {
		parent, cancel := context.WithCancel(context.Background())
		defer cancel()
		ctx := &commitCancellationContext{Context: parent, cancel: cancel}
		var stdout, stderr bytes.Buffer
		code := runContext(ctx, []string{mode, "--json", "8080"}, &stdout, &stderr, func(context.Context, string, int) portcheck.Result {
			status := portcheck.StatusFree
			if mode == "--wait-in-use" {
				status = portcheck.StatusInUse
			}
			return portcheck.Result{Port: 8080, Status: status}
		})
		if code != 1 || stdout.Len() != 0 || stderr.Len() != 0 {
			t.Fatalf("%s: code=%d out=%q err=%q", mode, code, &stdout, &stderr)
		}
	}
}

func TestHostFlagValueDiagnostics(t *testing.T) {
	for _, flag := range []string{"-q", "-h", "-v", "--unknown", "--"} {
		var stdout, stderr bytes.Buffer
		code := runContext(context.Background(), []string{"--host", flag, "8080"}, &stdout, &stderr, func(context.Context, string, int) portcheck.Result {
			t.Fatal("missing host performed a check")
			return portcheck.Result{}
		})
		if code != 2 || stdout.Len() != 0 || stderr.String() != "error: --host requires a value\n" {
			t.Fatalf("%q: code=%d out=%q err=%q", flag, code, &stdout, &stderr)
		}
	}
}
