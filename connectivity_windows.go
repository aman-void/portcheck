package portcheck

import (
	"errors"
	"syscall"
)

func connectionRefused(err error) bool {
	// Winsock WSAECONNREFUSED (10061) differs from Go's synthetic POSIX errno.
	return errors.Is(err, syscall.Errno(10061)) || errors.Is(err, syscall.ECONNREFUSED)
}
