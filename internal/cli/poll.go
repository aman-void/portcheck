package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aman-void/portcheck"
	"github.com/aman-void/portcheck/internal/process"
)

// ticks and now allow deterministic loop tests without a public clock API.
// Production owns one ticker and stops it on every exit path.
func runPolling(ctx context.Context, opts options, stdout, stderr io.Writer,
	check func(context.Context, string, int) portcheck.Result,
	ticks <-chan time.Time, now func() time.Time) int {
	return runPollingWithInspector(ctx, opts, stdout, stderr, check, ticks, now, process.Inspect)
}

func runPollingWithInspector(ctx context.Context, opts options, stdout, stderr io.Writer,
	check func(context.Context, string, int) portcheck.Result,
	ticks <-chan time.Time, now func() time.Time, inspect inspectFunc) int {
	if opts.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.timeout)
		defer cancel()
	}
	if ticks == nil {
		ticker := time.NewTicker(opts.interval)
		defer ticker.Stop()
		ticks = ticker.C
	}
	previous := make(map[int]portcheck.Result)
	busySince := make(map[int]time.Time)
	previousProcesses := make(map[int]processDetail)
	exit := 0
	for {
		if ctx.Err() != nil {
			return pollingCanceled(ctx, opts, stderr, exit)
		}
		results := make([]portcheck.Result, 0, len(opts.ports))
		details := make(map[int]processDetail)
		allMatch, failed := true, false
		for _, port := range opts.ports {
			if ctx.Err() != nil {
				return pollingCanceled(ctx, opts, stderr, exit)
			}
			result := check(ctx, opts.host, port)
			canceled := ctx.Err() != nil
			old, seen := previous[port]
			changed := !seen || old.Status != result.Status || errorText(old.Err) != errorText(result.Err)
			// Record genuine failures before honoring cancellation. A context-only
			// failure from this canceled check is normal termination, not exit 3.
			if result.Status == portcheck.StatusError && (!canceled || !cancellationOnly(result.Err)) {
				failed = true
				exit = 3
				if !opts.watch || changed {
					fmt.Fprintf(stderr, "error: failed to check %s port %d: %v\n", opts.host, port, result.Err)
				}
			}
			if canceled {
				return pollingCanceled(ctx, opts, stderr, exit)
			}
			enteredBusy := seen && old.Status != portcheck.StatusInUse && result.Status == portcheck.StatusInUse
			if opts.watch && !opts.quiet {
				if result.Status != portcheck.StatusInUse {
					delete(busySince, port)
				} else if !seen || old.Status != portcheck.StatusInUse {
					// This is the first busy observation, not the actual bind time.
					busySince[port] = now()
				}
			}
			if opts.watch && opts.process {
				if result.Status == portcheck.StatusInUse {
					details[port] = inspectDetail(ctx, opts.host, port, inspect)
				}
				if ctx.Err() != nil {
					return pollingCanceled(ctx, opts, stderr, exit)
				}
				changed = changed || !sameProcesses(previousProcesses[port], details[port])
				if changed {
					warnProcess(stderr, opts.host, port, details[port])
				}
			}
			results = append(results, result)
			desired := portcheck.StatusFree
			if opts.inUse {
				desired = portcheck.StatusInUse
			}
			if result.Status != desired {
				allMatch = false
			}
			if opts.watch && changed {
				timestamp := now()
				var observedSince *time.Time
				if enteredBusy && !opts.quiet {
					instant := busySince[port]
					observedSince = &instant
				}
				if err := writeWatchWithProcesses(stdout, opts, result, timestamp, details[port], observedSince); err != nil {
					fmt.Fprintf(stderr, "error: write output: %v\n", err)
					return 3
				}
				previous[port] = result
				previousProcesses[port] = details[port]
			}
		}
		// This is the completion boundary: cancellation observed before final
		// output wins. Once rendering starts, finish the result (especially JSON).
		if ctx.Err() != nil {
			return pollingCanceled(ctx, opts, stderr, exit)
		}
		if !opts.watch && (failed || allMatch) {
			if opts.process {
				for _, result := range results {
					if ctx.Err() != nil {
						return pollingCanceled(ctx, opts, stderr, exit)
					}
					if result.Status == portcheck.StatusInUse {
						details[result.Port] = inspectDetail(ctx, opts.host, result.Port, inspect)
						warnProcess(stderr, opts.host, result.Port, details[result.Port])
					}
				}
				if ctx.Err() != nil {
					return pollingCanceled(ctx, opts, stderr, exit)
				}
			}
			if err := renderWithProcesses(stdout, results, opts, details); err != nil {
				fmt.Fprintf(stderr, "error: write output: %v\n", err)
				return 3
			}
			if failed {
				return 3
			}
			return 0 // IN USE is success when it is the requested condition.
		}
		select {
		case <-ctx.Done():
			return pollingCanceled(ctx, opts, stderr, exit)
		case <-ticks:
		}
	}
}

// errors.Is alone cannot distinguish a context-only error from an errors.Join
// containing both a system failure and cancellation. Inspect each wrapped leaf
// so a joined system cause retains exit-code precedence.
func cancellationOnly(err error) bool {
	if err == nil {
		return false
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		causes := joined.Unwrap()
		if len(causes) == 0 {
			return false
		}
		for _, cause := range causes {
			if !cancellationOnly(cause) {
				return false
			}
		}
		return true
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		if cause := wrapped.Unwrap(); cause != nil {
			return cancellationOnly(cause)
		}
	}
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func pollingCanceled(ctx context.Context, opts options, stderr io.Writer, exit int) int {
	if opts.watch || exit == 3 {
		return exit
	}
	if ctx.Err() == context.DeadlineExceeded {
		state := "FREE"
		if opts.inUse {
			state = "IN USE"
		}
		fmt.Fprintf(stderr, "error: timeout waiting for all requested ports on %s to become %s\n", opts.host, state)
	}
	return 1
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func writeWatchWithProcesses(w io.Writer, opts options, result portcheck.Result, timestamp time.Time, detail processDetail, observedSince *time.Time) error {
	if opts.quiet {
		return writeResults(w, []portcheck.Result{result}, true)
	}
	status := "ERROR"
	if result.Status == portcheck.StatusFree {
		status = "FREE"
	}
	if result.Status == portcheck.StatusInUse {
		status = "IN USE"
	}
	busyMarker := ""
	if result.Status == portcheck.StatusInUse && observedSince != nil {
		busyMarker = "  observed since " + observedSince.Format("15:04:05")
	}
	_, err := fmt.Fprintf(w, "%s  %s  %d %s%s\n", timestamp.Format("2006-01-02 15:04:05"), opts.host, result.Port, status, busyMarker)
	if err != nil {
		return err
	}
	return writeProcess(w, detail)
}
