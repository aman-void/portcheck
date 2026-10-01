// Package checker checks TCP binding availability on IPv4 loopback.
package checker

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"syscall"
)

type Status int

const (
	StatusFree Status = iota
	StatusInUse
	StatusError
)

type Result struct {
	Port   int
	Status Status
	Err    error
}

// Check briefly binds 127.0.0.1:port and closes the listener before returning.
func Check(port int) Result {
	return check(port, net.Listen)
}

func check(port int, listen func(string, string) (net.Listener, error)) Result {
	result := Result{Port: port}
	if port < 1 || port > 65535 {
		result.Status = StatusError
		result.Err = fmt.Errorf("port %d must be between 1 and 65535", port)
		return result
	}
	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	listener, err := listen("tcp", address)
	if err != nil {
		if errors.Is(err, syscall.EADDRINUSE) {
			result.Status = StatusInUse
		} else {
			result.Status = StatusError
			result.Err = fmt.Errorf("check %s: %w", address, err)
		}
		return result
	}
	if err := listener.Close(); err != nil {
		result.Status = StatusError
		result.Err = fmt.Errorf("close listener for %s: %w", address, err)
	}
	return result
}
