//go:build !linux

package process

import "context"

// Native macOS/Windows inspection is not implemented in v0.4. Other platforms
// also report unsupported rather than shelling out or inventing ownership.
func inspect(context.Context, string, int) ([]Info, error) {
	return nil, ErrUnsupported
}
