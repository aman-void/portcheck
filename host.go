package portcheck

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"
)

// ErrInvalidHost identifies invalid local host syntax, not DNS or bind failures.
var ErrInvalidHost = errors.New("invalid host")

// ValidateHost validates an unbracketed IP address or ASCII DNS hostname without
// resolving it. Empty hosts, embedded ports, and malformed IPs are rejected.
// IPv6 zone identifiers and fully qualified hostnames are accepted.
func ValidateHost(host string) error {
	invalid := func() error {
		return fmt.Errorf("%w %q: expected an unbracketed IP address or DNS hostname", ErrInvalidHost, host)
	}
	if host == "" || strings.ContainsAny(host, " \t\r\n/\\[]") {
		return invalid()
	}
	// Hosts appear in human output: reject control bytes, including in IPv6 zones.
	for _, c := range host {
		if c < 33 || c > 126 {
			return invalid()
		}
	}
	if _, err := netip.ParseAddr(host); err == nil {
		return nil
	}
	if strings.ContainsAny(host, ":%") {
		return invalid()
	}
	name := strings.TrimSuffix(host, ".")
	if len(name) == 0 || len(name) > 253 {
		return invalid()
	}
	// Do not reinterpret an invalid dotted IPv4 literal as a hostname.
	numeric := true
	for _, c := range name {
		if (c < '0' || c > '9') && c != '.' {
			numeric = false
		}
	}
	if numeric {
		return invalid()
	}
	for _, label := range strings.Split(name, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return invalid()
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-') {
				return invalid()
			}
		}
	}
	return nil
}
