package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/aman-void/portcheck"
	"github.com/aman-void/portcheck/internal/process"
)

func render(w io.Writer, results []portcheck.Result, opts options) error {
	return renderWithProcesses(w, results, opts, nil)
}

func renderWithProcesses(w io.Writer, results []portcheck.Result, opts options, details map[int]processDetail) error {
	if opts.json {
		return writeJSONWithProcesses(w, results, details)
	}
	return writeResultsWithProcesses(w, results, opts.quiet, details)
}

func writeResults(w io.Writer, results []portcheck.Result, quiet bool) error {
	return writeResultsWithProcesses(w, results, quiet, nil)
}

func writeResultsWithProcesses(w io.Writer, results []portcheck.Result, quiet bool, details map[int]processDetail) error {
	if !quiet {
		if _, err := fmt.Fprintln(w, "PORT    STATUS"); err != nil {
			return err
		}
	}
	for _, result := range results {
		status := "ERROR"
		switch result.Status {
		case portcheck.StatusFree:
			status = "FREE"
		case portcheck.StatusInUse:
			status = "IN USE"
			if quiet {
				status = "IN_USE"
			}
		}
		var err error
		if quiet {
			_, err = fmt.Fprintln(w, status)
		} else {
			_, err = fmt.Fprintf(w, "%-8d%s\n", result.Port, status)
		}
		if err != nil {
			return err
		}
		if !quiet {
			if err := writeProcess(w, details[result.Port]); err != nil {
				return err
			}
		}
	}
	return nil
}

// Encode one record at a time rather than copying an entire large result set.
func writeJSON(w io.Writer, results []portcheck.Result) error {
	return writeJSONWithProcesses(w, results, nil)
}

func writeJSONWithProcesses(w io.Writer, results []portcheck.Result, details map[int]processDetail) error {
	if _, err := io.WriteString(w, "["); err != nil {
		return err
	}
	for i, result := range results {
		if i > 0 {
			if _, err := io.WriteString(w, ","); err != nil {
				return err
			}
		}
		record := struct {
			Port         int              `json:"port"`
			Status       portcheck.Status `json:"status"`
			Error        string           `json:"error,omitempty"`
			Processes    []process.Info   `json:"processes,omitempty"`
			ProcessError string           `json:"process_error,omitempty"`
		}{Port: result.Port, Status: result.Status}
		detail := details[result.Port]
		record.Processes = detail.owners
		if detail.err != nil {
			// Raw text is intentional: encoding/json escapes control characters, so
			// process metadata stays byte-accurate in this machine-readable contract.
			record.ProcessError = detail.err.Error()
		}
		if result.Err != nil {
			record.Error = result.Err.Error()
		}
		data, err := json.Marshal(record)
		if err != nil {
			return err
		}
		if _, err := w.Write(data); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "]\n")
	return err
}
