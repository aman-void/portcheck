package cli

import (
	"fmt"
	"strconv"
	"strings"
)

func parsePort(token string) (int, error) {
	if token == "" || strings.IndexFunc(token, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
		return 0, fmt.Errorf("invalid port %q: must be a decimal integer", token)
	}
	port, err := strconv.Atoi(token)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("invalid port %q: must be between 1 and 65535", token)
	}
	return port, nil
}

func expandPorts(tokens []string) ([]int, error) {
	ports := make([]int, 0)
	// Port numbers are bounded, so membership needs no growing map.
	var seen [65536]bool
	for _, token := range tokens {
		start, end, err := parseBounds(token)
		if err != nil {
			return nil, err
		}
		for port := start; port <= end; port++ {
			if !seen[port] {
				seen[port] = true
				ports = append(ports, port)
			}
		}
	}
	return ports, nil
}

func parseBounds(token string) (int, int, error) {
	if !strings.Contains(token, "-") {
		port, err := parsePort(token)
		return port, port, err
	}
	left, right, _ := strings.Cut(token, "-")
	start, err := parsePort(left)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid port range %q: %w", token, err)
	}
	end, err := parsePort(right)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid port range %q: %w", token, err)
	}
	if start > end {
		return 0, 0, fmt.Errorf("invalid port range %q: start port must not exceed end port", token)
	}
	return start, end, nil
}
