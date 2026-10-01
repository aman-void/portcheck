// Package checker checks local TCP binding availability.
package checker

import (
	"context"
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
// Retained as a background-context entrypoint for the checker regression tests.
func Check(port int) Result {
	return check(port, net.Listen)
}

// CheckContext uses the same bind classification with a context-aware listener.
func CheckContext(ctx context.Context, port int) Result {
	return CheckHost(ctx, "127.0.0.1", port)
}

// CheckHost binds the requested host using the Go networking layer.
func CheckHost(ctx context.Context, host string, port int) Result {
	var config net.ListenConfig
	return checkHost(host, port, func(network, address string) (net.Listener, error) {
		return config.Listen(ctx, network, address)
	})
}

func check(port int, listen func(string, string) (net.Listener, error)) Result {
	return checkHost("127.0.0.1", port, listen)
}

func checkHost(host string, port int, listen func(string, string) (net.Listener, error)) Result {
	result := Result{Port: port}
	if port < 1 || port > 65535 {
		result.Status = StatusError
		result.Err = fmt.Errorf("port %d must be between 1 and 65535", port)
		return result
	}
	address := net.JoinHostPort(host, strconv.Itoa(port))
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
