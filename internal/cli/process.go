package cli

import (
	"context"
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/aman-void/portcheck/internal/process"
)

type inspectFunc func(context.Context, string, int) ([]process.Info, error)

// Separate from portcheck.Result: lookup errors are not bind errors, and the
// public Go API stays source-compatible (including unkeyed Result literals).
type processDetail struct {
	owners []process.Info
	err    error
}

func inspectDetail(ctx context.Context, host string, port int, inspect inspectFunc) processDetail {
	owners, err := inspect(ctx, host, port)
	owners = slices.Clone(owners)
	sort.Slice(owners, func(i, j int) bool { return owners[i].PID < owners[j].PID })
	if len(owners) == 0 && err == nil {
		err = process.ErrNotFound
	}
	return processDetail{owners: owners, err: err}
}

func sameProcesses(a, b processDetail) bool {
	return slices.Equal(a.owners, b.owners) && errorText(a.err) == errorText(b.err)
}

func warnProcess(w io.Writer, host string, port int, detail processDetail) {
	if detail.err != nil && !cancellationOnly(detail.err) {
		fmt.Fprintf(w, "warning: process unavailable for %s port %d: %s\n", host, port, safeText(detail.err.Error()))
	}
}

// Process metadata is untrusted OS data; never render terminal control codes.
// JSON retains the original strings, escaped by encoding/json.
func safeText(value string) string {
	return strings.Map(func(r rune) rune {
		if !unicode.IsPrint(r) {
			return '\uFFFD'
		}
		return r
	}, value)
}

func writeProcess(w io.Writer, detail processDetail) error {
	for _, owner := range detail.owners {
		if _, err := fmt.Fprintf(w, "        PID: %d", owner.PID); err != nil {
			return err
		}
		if owner.Name != "" {
			if _, err := fmt.Fprintf(w, "  NAME: %s", safeText(owner.Name)); err != nil {
				return err
			}
		}
		if owner.Executable != "" {
			if _, err := fmt.Fprintf(w, "  EXECUTABLE: %s", safeText(owner.Executable)); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}
	if detail.err != nil {
		_, err := fmt.Fprintf(w, "        PROCESS unavailable: %s\n", safeText(detail.err.Error()))
		return err
	}
	return nil
}
