package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aman-void/portcheck"
)

type connectFunc func(context.Context, string, time.Duration) (portcheck.ConnectivityResult, error)

type connectivityRecord struct {
	Address    string                       `json:"address"`
	Status     portcheck.ConnectivityStatus `json:"status"`
	DurationMS float64                      `json:"duration_ms"`
	Error      string                       `json:"error,omitempty"`
}

func connectionRecord(result portcheck.ConnectivityResult) connectivityRecord {
	record := connectivityRecord{Address: result.Address, Status: result.Status, DurationMS: float64(result.Duration) / float64(time.Millisecond)}
	if result.Err != nil {
		record.Error = result.Err.Error()
	}
	return record
}

func connectivityExit(result portcheck.ConnectivityResult) int {
	switch result.Status {
	case portcheck.ConnectivityReachable:
		return 0
	case portcheck.ConnectivityRefused, portcheck.ConnectivityTimeout:
		return 1
	default:
		return 3
	}
}

func runConnect(ctx context.Context, opts options, stdout, stderr io.Writer, connect connectFunc) int {
	result, err := connect(ctx, opts.connect, opts.timeout)
	if result.Err == nil {
		result.Err = err
	}
	if result.Err != nil {
		fmt.Fprintf(stderr, "error: connect %s: %s\n", opts.connect, safeText(result.Err.Error()))
	}
	if err := renderConnection(stdout, result, opts); err != nil {
		fmt.Fprintf(stderr, "error: write output: %v\n", err)
		return 3
	}
	return connectivityExit(result)
}

func renderConnection(w io.Writer, result portcheck.ConnectivityResult, opts options) error {
	if opts.json {
		return json.NewEncoder(w).Encode([]connectivityRecord{connectionRecord(result)})
	}
	status := strings.ToUpper(string(result.Status))
	if opts.quiet {
		_, err := fmt.Fprintln(w, status)
		return err
	}
	if _, err := fmt.Fprintln(w, "ADDRESS  STATUS  LATENCY"); err != nil {
		return err
	}
	_, err := fmt.Fprintf(w, "%s  %s  %.2fms\n", result.Address, status, float64(result.Duration)/float64(time.Millisecond))
	return err
}
