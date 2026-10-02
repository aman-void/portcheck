package portcheck

import "syscall"

func refusedTestError() error { return syscall.Errno(10061) }
