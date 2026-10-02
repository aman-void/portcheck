package checker

import (
	"errors"
	"syscall"
)

func addressInUse(err error) bool {
	// Winsock WSAEADDRINUSE differs from Go's synthetic POSIX errno.
	return errors.Is(err, syscall.Errno(10048)) || errors.Is(err, syscall.EADDRINUSE)
}
