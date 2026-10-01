package cli

import (
	"fmt"
	"io"

	"github.com/aman-void/portcheck/internal/checker"
)

func writeResults(w io.Writer, results []checker.Result, quiet bool) error {
	if !quiet {
		if _, err := fmt.Fprintln(w, "PORT    STATUS"); err != nil {
			return err
		}
	}
	for _, result := range results {
		status := "ERROR"
		switch result.Status {
		case checker.StatusFree:
			status = "FREE"
		case checker.StatusInUse:
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
