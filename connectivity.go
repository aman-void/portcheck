package portcheck

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// DefaultConnectTimeout is the CLI's finite default TCP connection deadline.
const DefaultConnectTimeout = 5 * time.Second

// ConnectivityStatus describes a TCP dial, independently of local bind status.
type ConnectivityStatus string

const (
	// ConnectivityReachable means TCP establishment and cleanup succeeded.
	ConnectivityReachable ConnectivityStatus = "reachable"
	// ConnectivityRefused means the connection attempt was explicitly refused.
	ConnectivityRefused ConnectivityStatus = "refused"
	// ConnectivityTimeout means the dial timed out or its deadline expired.
	ConnectivityTimeout ConnectivityStatus = "timeout"
	// ConnectivityError means another failure, including cancellation or cleanup.
	ConnectivityError ConnectivityStatus = "error"
)

// ErrInvalidEndpoint identifies malformed host:port input (no DNS is performed).
var ErrInvalidEndpoint = errors.New("invalid endpoint")

// ErrInvalidTimeout identifies a nonpositive connectivity timeout.
var ErrInvalidTimeout = errors.New("invalid connectivity timeout")

// ConnectivityResult records one TCP connection attempt. Err retains the cause
// of any unsuccessful operation, including expected refusal and timeout results.
// Duration measures the dial, not validation or connection cleanup.
type ConnectivityResult struct {
	Address  string
	Status   ConnectivityStatus
	Duration time.Duration
	Err      error
}

// ValidateEndpoint validates host:port syntax without resolution or networking.
// Ports must be decimal digits in 1-65535; IPv6 endpoints require brackets.
// Host syntax follows ValidateHost, including ASCII hostnames and IPv6 zones.
func ValidateEndpoint(address string) error {
	_, err := endpointAddress(address)
	return err
}

func endpointAddress(address string) (string, error) {
	invalid := func(err error) (string, error) {
		return "", fmt.Errorf("%w %q: %w", ErrInvalidEndpoint, address, err)
	}
	host, text, err := net.SplitHostPort(address)
	if err != nil {
		return invalid(err)
	}
	if err := ValidateHost(host); err != nil {
		return invalid(err)
	}
	if strings.HasPrefix(address, "[") && !strings.Contains(host, ":") {
		return invalid(errors.New("brackets require an IPv6 address"))
	}
	if text == "" {
		return invalid(errors.New("a decimal port is required"))
	}
	for _, c := range text {
		if c < '0' || c > '9' {
			return invalid(errors.New("port must contain only decimal digits"))
		}
	}
	port, err := strconv.Atoi(text)
	if err != nil {
		return invalid(fmt.Errorf("%w: port must be between 1 and 65535", ErrInvalidPort))
	}
	if err := validatePort(port); err != nil {
		return invalid(err)
	}
	return net.JoinHostPort(host, strconv.Itoa(port)), nil
}

// Connect attempts TCP establishment on address, sends no payload, and closes
// a successful connection before returning. timeout must be positive and ctx
// non-nil. The deadline covers resolution and dialing; an earlier context
// deadline wins. Go may try resolved addresses as part of its normal dialing;
// Portcheck adds no retries. All unsuccessful results return the same error in
// Err and the error return, with causes preserved for errors.Is/errors.As.
// Refusal and timeout are expected negative observations, not bind statuses.
func Connect(ctx context.Context, address string, timeout time.Duration) (ConnectivityResult, error) {
	dialer := &net.Dialer{}
	return connect(ctx, address, timeout, dialer.DialContext)
}

func connect(ctx context.Context, address string, timeout time.Duration,
	dial func(context.Context, string, string) (net.Conn, error)) (ConnectivityResult, error) {
	result := ConnectivityResult{Address: address, Status: ConnectivityError}
	target, err := endpointAddress(address)
	if err != nil {
		result.Err = err
		return result, err
	}
	if timeout <= 0 {
		result.Err = fmt.Errorf("%w: must be positive", ErrInvalidTimeout)
		return result, result.Err
	}
	if err := ctx.Err(); err != nil {
		result.Err = err
		result.Status = connectivityStatus(err)
		return result, err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	start := time.Now()
	conn, err := dial(ctx, "tcp", target)
	result.Duration = time.Since(start)
	if conn != nil {
		// Even a dial implementation returning both a connection and an error
		// must not leak the connection. Cleanup failure remains a system error.
		if closeErr := conn.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close TCP connection: %w", closeErr))
			result.Err = err
			return result, err
		}
	}
	if err == nil && conn == nil {
		err = errors.New("TCP dial returned no connection")
	}
	result.Status = connectivityStatus(err)
	result.Err = err
	return result, err
}

func connectivityStatus(err error) ConnectivityStatus {
	if err == nil {
		return ConnectivityReachable
	}
	if errors.Is(err, context.Canceled) {
		return ConnectivityError
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ConnectivityTimeout
	}
	var networkError net.Error
	if errors.As(err, &networkError) && networkError.Timeout() {
		return ConnectivityTimeout
	}
	if connectionRefused(err) {
		return ConnectivityRefused
	}
	return ConnectivityError
}
