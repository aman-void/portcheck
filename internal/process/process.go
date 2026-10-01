// Package process provides optional, read-only local TCP listener inspection.
// It is separate from binding availability and never changes a port's status.
package process

import (
	"context"
	"errors"
)

var (
	ErrUnsupported = errors.New("process inspection is not supported on this platform")
	ErrNotFound    = errors.New("no visible listening process found (socket may have changed or be isolated)")
	ErrPermission  = errors.New("process inspection permission denied")
)

// Info identifies a visible owner. Name and Executable are optional metadata.
type Info struct {
	PID        int    `json:"pid"`
	Name       string `json:"name,omitempty"`
	Executable string `json:"executable,omitempty"`
}

// Inspect returns all discovered owners in PID order, without caching or retries.
// The caller supplies a validated local host and port and a non-nil context.
func Inspect(ctx context.Context, host string, port int) ([]Info, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return inspect(ctx, host, port)
}
