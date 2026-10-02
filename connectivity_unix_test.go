//go:build !windows

package portcheck

import "syscall"

func refusedTestError() error { return syscall.ECONNREFUSED }
