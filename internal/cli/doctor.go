package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/aman-void/portcheck"
	"github.com/aman-void/portcheck/internal/process"
)

// Doctor composes the same API and inspector used by ordinary commands.
// The observations are sequential, not an atomic snapshot or a diagnosis of
// application health. Process failures never replace bind/dial observations.
type doctorReport struct {
	host         string
	port         int
	local        portcheck.Result
	process      processDetail
	connectivity portcheck.ConnectivityResult
}

func diagnose(ctx context.Context, opts options, check func(context.Context, string, int) portcheck.Result,
	inspect inspectFunc, connect connectFunc) doctorReport {
	report := doctorReport{host: opts.host, port: opts.ports[0]}
	report.local = check(ctx, report.host, report.port)
	if report.local.Status == portcheck.StatusInUse {
		report.process = inspectDetail(ctx, report.host, report.port, inspect)
	}
	address := net.JoinHostPort(report.host, strconv.Itoa(report.port))
	var err error
	report.connectivity, err = connect(ctx, address, opts.timeout)
	if report.connectivity.Err == nil {
		report.connectivity.Err = err
	}
	return report
}

func runDoctor(ctx context.Context, opts options, stdout, stderr io.Writer,
	check func(context.Context, string, int) portcheck.Result, inspect inspectFunc, connect connectFunc) int {
	// A single budget covers resolution, local binding, process inspection,
	// and dialing. Each component already cooperates with context cancellation.
	ctx, cancel := context.WithTimeout(ctx, opts.timeout)
	defer cancel()
	report := diagnose(ctx, opts, check, inspect, connect)
	exit := connectivityExit(report.connectivity)
	if report.local.Status == portcheck.StatusError {
		fmt.Fprintf(stderr, "error: local check %s port %d: %s\n", report.host, report.port, safeText(errorText(report.local.Err)))
		exit = 3
	}
	warnProcess(stderr, report.host, report.port, report.process)
	if report.connectivity.Err != nil {
		fmt.Fprintf(stderr, "error: connect %s: %s\n", report.connectivity.Address, safeText(report.connectivity.Err.Error()))
	}
	if ctx.Err() == context.DeadlineExceeded {
		fmt.Fprintf(stderr, "error: doctor timeout (budget %s; includes local check, process inspection, and connectivity)\n", opts.timeout)
		if exit != 3 {
			exit = 1
		}
	}
	if err := renderDoctor(stdout, report, opts.json); err != nil {
		fmt.Fprintf(stderr, "error: write output: %v\n", err)
		return 3
	}
	return exit
}

type doctorRecord struct {
	Endpoint struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Protocol string `json:"protocol"`
	} `json:"endpoint"`
	Local struct {
		Status portcheck.Status `json:"status"`
		Error  string           `json:"error,omitempty"`
	} `json:"local"`
	Processes    []process.Info     `json:"processes,omitempty"`
	ProcessError string             `json:"process_error,omitempty"`
	Connectivity connectivityRecord `json:"connectivity"`
}

func renderDoctor(w io.Writer, report doctorReport, asJSON bool) error {
	if asJSON {
		var record doctorRecord
		record.Endpoint.Host, record.Endpoint.Port, record.Endpoint.Protocol = report.host, report.port, "tcp"
		record.Local.Status, record.Local.Error = report.local.Status, errorText(report.local.Err)
		record.Processes, record.ProcessError = report.process.owners, errorText(report.process.err)
		record.Connectivity = connectionRecord(report.connectivity)
		return json.NewEncoder(w).Encode([]doctorRecord{record})
	}
	localStatus := strings.ToUpper(strings.ReplaceAll(string(report.local.Status), "_", " "))
	if _, err := fmt.Fprintf(w, "Portcheck Doctor\n\nEndpoint\n  Host: %s\n  Port: %d\n  Protocol: TCP\n\nLocal Port\n  Status: %s\n", report.host, report.port, localStatus); err != nil {
		return err
	}
	if report.local.Err != nil {
		if _, err := fmt.Fprintf(w, "  Error: %s\n", safeText(report.local.Err.Error())); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w, "\nProcess"); err != nil {
		return err
	}
	if report.local.Status == portcheck.StatusFree {
		if _, err := fmt.Fprintln(w, "  Status: none (local bind succeeded)"); err != nil {
			return err
		}
	} else if report.local.Status == portcheck.StatusError {
		if _, err := fmt.Fprintln(w, "  Status: not inspected (local check failed)"); err != nil {
			return err
		}
	} else if err := writeProcess(w, report.process); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "\nConnectivity\n  Status: %s\n  Latency: %.2fms\n", strings.ToUpper(string(report.connectivity.Status)), float64(report.connectivity.Duration)/float64(time.Millisecond)); err != nil {
		return err
	}
	if report.connectivity.Err != nil {
		if _, err := fmt.Fprintf(w, "  Error: %s\n", safeText(report.connectivity.Err.Error())); err != nil {
			return err
		}
	}
	return nil
}
