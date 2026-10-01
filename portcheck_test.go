package portcheck

import (
	"context"
	"errors"
	"fmt"
	"net"
	"reflect"
	"strconv"
	"testing"
	"time"
)

func TestCheckRealListener(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	port := listener.Addr().(*net.TCPAddr).Port
	got, err := Check(context.Background(), port)
	if err != nil || got.Err != nil || got.Status != StatusInUse || got.Port != port {
		t.Fatalf("occupied: %+v, %v", got, err)
	}
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	got, err = Check(context.Background(), port)
	if err != nil || got.Err != nil || got.Status != StatusFree {
		t.Fatalf("free: %+v, %v", got, err)
	}
	// A successful library check must leave the port available for another bind.
	rebound, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		t.Fatal(err)
	}
	rebound.Close()
}

func TestCheckInvalidPorts(t *testing.T) {
	for _, port := range []int{-1, 0, 65536} {
		got, err := Check(context.Background(), port)
		if !errors.Is(err, ErrInvalidPort) || got.Err != err || got.Status != StatusError || got.Port != port {
			t.Fatalf("invalid: %+v, %v", got, err)
		}
	}
}

func TestCheckContext(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, stop := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer stop()
	for _, ctx := range []context.Context{canceled, expired} {
		got, err := Check(ctx, 8080)
		if !errors.Is(err, ctx.Err()) || got.Err != err || got.Status != StatusError {
			t.Fatalf("cancellation: %+v, %v", got, err)
		}
		results, err := CheckPorts(ctx, []int{8080})
		if !errors.Is(err, ctx.Err()) || len(results) != 0 {
			t.Fatalf("batch cancellation: %+v, %v", results, err)
		}
	}
}

func TestCheckPortsRealListeners(t *testing.T) {
	var listeners []net.Listener
	var ports []int
	for range 2 {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { listener.Close() })
		listeners = append(listeners, listener)
		ports = append(ports, listener.Addr().(*net.TCPAddr).Port)
	}
	if err := listeners[1].Close(); err != nil {
		t.Fatal(err)
	}
	input := []int{ports[1], ports[0], ports[1]}
	unchanged := append([]int(nil), input...)
	results, err := CheckPorts(context.Background(), input)
	want := []Result{{Port: ports[1], Status: StatusFree}, {Port: ports[0], Status: StatusInUse}}
	if err != nil || len(results) != len(want) || !reflect.DeepEqual(input, unchanged) {
		t.Fatalf("results: %+v, %v", results, err)
	}
	for i, result := range results {
		if result.Port != want[i].Port || result.Status != want[i].Status || result.Err != nil {
			t.Fatalf("result %d: %+v; want port=%d status=%s and no error", i, result, want[i].Port, want[i].Status)
		}
	}
}

func TestCheckPortsValidationAndEmptyInput(t *testing.T) {
	for _, ports := range [][]int{{1, 0}, {65536}, {-1}} {
		results, err := checkPorts(context.Background(), ports, func(context.Context, int) (Result, error) {
			t.Fatal("invalid input reached checker")
			return Result{}, nil
		})
		if results != nil || !errors.Is(err, ErrInvalidPort) {
			t.Fatalf("validation: %+v, %v", results, err)
		}
	}
	results, err := CheckPorts(context.Background(), nil)
	if results == nil || len(results) != 0 || err != nil {
		t.Fatalf("empty: %+v, %v", results, err)
	}
}

func TestCheckPortsFailuresAndOrder(t *testing.T) {
	first, second := errors.New("first failure"), errors.New("second failure")
	var checked []int
	results, err := checkPorts(context.Background(), []int{3, 1, 3, 2}, func(_ context.Context, port int) (Result, error) {
		checked = append(checked, port)
		failure := first
		if port == 2 {
			failure = second
		}
		if port == 1 {
			return Result{Port: port, Status: StatusFree}, nil
		}
		return Result{Port: port, Status: StatusError, Err: failure}, failure
	})
	if !reflect.DeepEqual(checked, []int{3, 1, 2}) || len(results) != 3 || !errors.Is(err, first) || !errors.Is(err, second) || results[0].Err != first || results[2].Err != second {
		t.Fatalf("checks=%v results=%+v error=%v", checked, results, err)
	}
}

func TestCheckPortsStopsAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	count := 0
	results, err := checkPorts(ctx, []int{1, 2}, func(_ context.Context, port int) (Result, error) {
		count++
		cancel()
		return Result{Port: port, Status: StatusFree}, nil
	})
	if count != 1 || len(results) != 1 || !errors.Is(err, context.Canceled) {
		t.Fatalf("count=%d results=%+v error=%v", count, results, err)
	}
}

func TestCheckPortsCancellationAfterLastCheck(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	results, err := checkPorts(ctx, []int{1}, func(_ context.Context, port int) (Result, error) {
		cancel()
		return Result{Port: port, Status: StatusFree}, nil
	})
	if len(results) != 1 || !errors.Is(err, context.Canceled) {
		t.Fatalf("results=%+v error=%v", results, err)
	}
}

// Model cancellation or deadline expiry without timers or real socket races.
// Done comes from the manually canceled parent; Err reports the chosen cause.
type batchCancellationContext struct {
	context.Context
	cause error
}

func (ctx batchCancellationContext) Err() error {
	if ctx.Context.Err() != nil {
		return ctx.cause
	}
	return nil
}

func TestCheckPortsDoesNotDuplicateCancellation(t *testing.T) {
	for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
		for _, last := range []bool{false, true} {
			for _, wrapped := range []bool{false, true} {
				for _, priorFailure := range []bool{false, true} {
					name := fmt.Sprintf("%v/last=%t/wrapped=%t/prior=%t", cause, last, wrapped, priorFailure)
					t.Run(name, func(t *testing.T) {
						parent, cancel := context.WithCancel(context.Background())
						defer cancel()
						ctx := batchCancellationContext{Context: parent, cause: cause}
						ports := []int{1, 2, 3}
						if last {
							ports = ports[:2]
						}
						failure := cause
						if wrapped {
							failure = fmt.Errorf("check port 2: %w", cause)
						}
						previous := errors.New("previous system failure")
						count := 0
						results, err := checkPorts(ctx, ports, func(_ context.Context, port int) (Result, error) {
							count++
							if port == 1 {
								if priorFailure {
									return Result{Port: port, Status: StatusError, Err: previous}, previous
								}
								return Result{Port: port, Status: StatusFree}, nil
							}
							if port != 2 {
								t.Fatal("batch continued after cancellation")
							}
							cancel()
							return Result{Port: port, Status: StatusError, Err: failure}, failure
						})
						wantError := failure.Error()
						if priorFailure {
							wantError = previous.Error() + "\n" + wantError
						}
						if count != 2 || len(results) != 2 || !errors.Is(err, cause) || err.Error() != wantError {
							t.Fatalf("checks=%d results=%+v error=%v; want %q", count, results, err, wantError)
						}
						if results[1].Port != 2 || results[1].Status != StatusError || !errors.Is(results[1].Err, cause) {
							t.Fatalf("canceled result: %+v", results[1])
						}
						if priorFailure && !errors.Is(err, previous) {
							t.Fatalf("lost previous failure: %v", err)
						}
					})
				}
			}
		}
	}
}
