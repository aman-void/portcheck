// Package portcheck checks TCP binding availability on 127.0.0.1.
// A free result is an observation, not a reservation of the port.
package portcheck

import (
	"context"
	"errors"
	"fmt"

	"github.com/aman-void/portcheck/internal/checker"
)

// Status describes the outcome of a local TCP bind check.
type Status string

const (
	// StatusFree means a listener was successfully created and closed.
	StatusFree Status = "free"
	// StatusInUse means the address could not be bound because it was in use.
	StatusInUse Status = "in_use"
	// StatusError means the check failed for another reason.
	StatusError Status = "error"
)

// ErrInvalidPort identifies ports outside the inclusive range 1-65535.
var ErrInvalidPort = errors.New("invalid port")

// Result describes one check. Err is non-nil only when Status is StatusError.
// Errors retain their underlying causes for errors.Is and errors.As.
type Result struct {
	Port   int
	Status Status
	Err    error
}

// Check briefly binds 127.0.0.1:port and closes the listener before returning.
// Occupied ports return StatusInUse with no error. Invalid ports, cancellation,
// and system failures return StatusError and the same error in Result.Err and
// the error return value. The context must be non-nil.
func Check(ctx context.Context, port int) (Result, error) {
	result := Result{Port: port, Status: StatusError}
	if err := validatePort(port); err != nil {
		result.Err = err
		return result, err
	}
	if err := ctx.Err(); err != nil {
		result.Err = err
		return result, err
	}
	checked := checker.CheckContext(ctx, port)
	result.Err = checked.Err
	switch checked.Status {
	case checker.StatusFree:
		result.Status = StatusFree
	case checker.StatusInUse:
		result.Status = StatusInUse
	}
	if err := ctx.Err(); err != nil {
		result.Status = StatusError
		result.Err = err
	}
	return result, result.Err
}

// CheckPorts validates all input before checking any port, checks sequentially,
// and removes duplicates while preserving first-occurrence order.
// Empty input with an active context returns an empty slice and no error.
// Invalid input returns no results and an error wrapping ErrInvalidPort.
// System failures are recorded
// in individual results and joined in the returned error; other checks continue.
// Cancellation stops checking and returns any results already collected plus
// an error matching the context error. The context must be non-nil.
func CheckPorts(ctx context.Context, ports []int) ([]Result, error) {
	return checkPorts(ctx, ports, Check)
}

func checkPorts(ctx context.Context, ports []int, check func(context.Context, int) (Result, error)) ([]Result, error) {
	for _, port := range ports {
		if err := validatePort(port); err != nil {
			return nil, err
		}
	}
	results := make([]Result, 0, min(len(ports), 65535))
	seen := make(map[int]bool)
	var failures []error
	if err := ctx.Err(); err != nil {
		return results, err
	}
	for _, port := range ports {
		if err := ctx.Err(); err != nil {
			return results, joinFailures(failures, err)
		}
		if seen[port] {
			continue
		}
		seen[port] = true
		result, err := check(ctx, port)
		results = append(results, result)
		if err != nil {
			failures = append(failures, err)
		}
	}
	return results, joinFailures(failures, ctx.Err())
}

func joinFailures(failures []error, contextErr error) error {
	joined := errors.Join(failures...)
	// A per-port failure may already wrap the cancellation or deadline error.
	if contextErr == nil || errors.Is(joined, contextErr) {
		return joined
	}
	return errors.Join(joined, contextErr)
}

func validatePort(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("%w %d: must be between 1 and 65535", ErrInvalidPort, port)
	}
	return nil
}
