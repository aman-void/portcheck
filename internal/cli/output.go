package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/aman-void/portcheck"
)

func render(w io.Writer, results []portcheck.Result, opts options) error {
	if opts.json {
		return writeJSON(w, results)
	}
	return writeResults(w, results, opts.quiet)
}

func writeResults(w io.Writer, results []portcheck.Result, quiet bool) error {
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
	}
	return nil
}

// Encode one record at a time rather than copying an entire large result set.
func writeJSON(w io.Writer, results []portcheck.Result) error {
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
			Port   int              `json:"port"`
			Status portcheck.Status `json:"status"`
			Error  string           `json:"error,omitempty"`
		}{Port: result.Port, Status: result.Status}
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
