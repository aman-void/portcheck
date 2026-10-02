//go:build !windows

package checker

import (
	"errors"
	"syscall"
)

func addressInUse(err error) bool {
	return errors.Is(err, syscall.EADDRINUSE)
}
