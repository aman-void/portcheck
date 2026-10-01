package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/aman-void/portcheck"
	"github.com/aman-void/portcheck/internal/process"
)

func TestProcessCLI(t *testing.T) {
	for _, mode := range []string{"human", "json", "quiet"} {
		t.Run(mode, func(t *testing.T) {
			args := []string{"--process", "3002", "3000-3002", "3001"}
			if mode == "json" {
				args = append(args, "--json")
			}
			if mode == "quiet" {
				args = append(args, "--quiet")
			}
			var out, diag bytes.Buffer
			var checked, inspected []int
			code := runContextWithInspector(context.Background(), args, &out, &diag,
				func(_ context.Context, _ string, p int) portcheck.Result {
					checked = append(checked, p)
					s := portcheck.StatusInUse
					if p == 3000 {
						s = portcheck.StatusFree
					}
					return portcheck.Result{Port: p, Status: s}
				}, func(_ context.Context, h string, p int) ([]process.Info, error) {
					if h != "127.0.0.1" {
						t.Fatal(h)
					}
					inspected = append(inspected, p)
					if p == 3001 {
						return nil, process.ErrPermission
					}
					return []process.Info{{PID: 20, Name: "server", Executable: "/bin/server"}, {PID: 10, Name: "shared"}}, nil
				})
			if code != 1 || !reflect.DeepEqual(checked, []int{3002, 3000, 3001}) || !reflect.DeepEqual(inspected, []int{3002, 3001}) || !strings.Contains(diag.String(), "warning: process unavailable") {
				t.Fatalf("%d %v %v %q", code, checked, inspected, &diag)
			}
			switch mode {
			case "quiet":
				if out.String() != "IN_USE\nFREE\nIN_USE\n" {
					t.Fatal(out.String())
				}
			case "human":
				if !strings.Contains(out.String(), "PID: 10") || !strings.Contains(out.String(), "EXECUTABLE: /bin/server") || !strings.Contains(out.String(), "PROCESS unavailable:") {
					t.Fatal(out.String())
				}
			case "json":
				var rows []struct {
					Port         int            `json:"port"`
					Status       string         `json:"status"`
					Processes    []process.Info `json:"processes"`
					ProcessError string         `json:"process_error"`
					Error        string         `json:"error"`
				}
				if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
					t.Fatal(err)
				}
				if len(rows) != 3 || rows[0].Port != 3002 || len(rows[0].Processes) != 2 || rows[0].Processes[0].PID != 10 || rows[0].Processes[1].Name != "server" || rows[0].Processes[1].Executable != "/bin/server" || rows[1].Processes != nil || rows[1].ProcessError != "" || rows[2].Status != "in_use" || rows[2].ProcessError == "" || rows[2].Error != "" {
					t.Fatalf("%+v", rows)
				}
			}
		})
	}
}

func TestNoUnrequestedProcessInspection(t *testing.T) {
	for _, args := range [][]string{{"8080"}, {"--process", "--help"}, {"--process", "--version"}, {"--process", "--find", "8080"}, {"--process", "--find", "--quiet", "8080"}, {"--process", "--find", "--json", "8080"}, {"--process", "--wait", "8080"}, {"--process", "8080", "bad"}, {"--process", "--quiet", "--json", "8080"}, {"--process", "--watch", "--json", "8080"}} {
		t.Run(fmt.Sprint(args), func(t *testing.T) {
			var out, diag bytes.Buffer
			code := runContextWithInspector(context.Background(), args, &out, &diag,
				func(_ context.Context, _ string, p int) portcheck.Result {
					return portcheck.Result{Port: p, Status: portcheck.StatusFree}
				},
				func(context.Context, string, int) ([]process.Info, error) {
					t.Fatal("unnecessary process inspection")
					return nil, nil
				})
			if code == 3 {
				t.Fatalf("%d %q %q", code, &out, &diag)
			}
		})
	}
}

func TestProcessLookupWarningsPreserveExit(t *testing.T) {
	for _, err := range []error{process.ErrUnsupported, process.ErrNotFound, process.ErrPermission, errors.New("broken proc")} {
		for _, wait := range []bool{false, true} {
			args := []string{"--process", "--json", "8080"}
			want := 1
			if wait {
				args = append(args, "--wait-in-use")
				want = 0
			}
			var out, diag bytes.Buffer
			code := runContextWithInspector(context.Background(), args, &out, &diag, func(_ context.Context, _ string, p int) portcheck.Result {
				return portcheck.Result{Port: p, Status: portcheck.StatusInUse}
			}, func(context.Context, string, int) ([]process.Info, error) { return nil, err })
			var rows []map[string]any
			if e := json.Unmarshal(out.Bytes(), &rows); e != nil {
				t.Fatal(e)
			}
			if code != want || rows[0]["status"] != "in_use" || rows[0]["process_error"] != err.Error() || rows[0]["error"] != nil || diag.Len() == 0 {
				t.Fatalf("%d %+v %q", code, rows, &diag)
			}
		}
	}
}

func TestProcessWatchChanges(t *testing.T) {
	for _, quiet := range []bool{false, true} {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		ticks := make(chan time.Time, 8)
		for range 8 {
			ticks <- time.Time{}
		}
		var out, diag bytes.Buffer
		cycle := 0
		code := runPollingWithInspector(ctx, options{watch: true, process: true, quiet: quiet, host: "::1", ports: []int{8080}}, &out, &diag,
			func(context.Context, string, int) portcheck.Result {
				cycle++
				if cycle == 9 {
					cancel()
				}
				return portcheck.Result{Port: 8080, Status: portcheck.StatusInUse}
			}, ticks, time.Now, func(_ context.Context, h string, p int) ([]process.Info, error) {
				if h != "::1" || p != 8080 {
					t.Fatalf("%s %d", h, p)
				}
				switch cycle {
				case 1, 2:
					return []process.Info{{PID: 10, Name: "A"}}, nil
				case 3:
					return []process.Info{{PID: 20, Name: "B"}, {PID: 10, Name: "A"}}, nil
				case 4:
					return []process.Info{{PID: 10, Name: "A"}, {PID: 20, Name: "B"}}, nil // same set, reordered
				case 5, 6:
					return nil, process.ErrPermission
				case 7:
					return nil, process.ErrNotFound
				default:
					return []process.Info{{PID: 30, Name: "C"}}, nil
				}
			})
		if code != 0 || strings.Count(diag.String(), "warning:") != 2 {
			t.Fatalf("%d %q", code, &diag)
		}
		if quiet {
			if out.String() != strings.Repeat("IN_USE\n", 5) {
				t.Fatal(out.String())
			}
		} else {
			if strings.Count(out.String(), "8080 IN USE\n") != 5 || strings.Count(out.String(), "PID: 20") != 1 || strings.Count(out.String(), "PID: 30") != 1 || strings.Count(out.String(), "PROCESS unavailable:") != 2 {
				t.Fatal(out.String())
			}
		}
	}
}

func TestProcessWaitInspectsOnlyFinalCycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ticks := make(chan time.Time, 1)
	ticks <- time.Time{}
	checks, lookups := 0, 0
	var out, diag bytes.Buffer
	code := runPollingWithInspector(ctx, options{inUse: true, process: true, json: true, host: "127.0.0.1", ports: []int{8080, 8081}}, &out, &diag,
		func(_ context.Context, _ string, p int) portcheck.Result {
			checks++
			s := portcheck.StatusInUse
			if checks == 2 {
				s = portcheck.StatusFree
			}
			return portcheck.Result{Port: p, Status: s}
		}, ticks, time.Now, func(context.Context, string, int) ([]process.Info, error) {
			lookups++
			if checks != 4 {
				t.Fatal("inspected intermediate cycle")
			}
			return []process.Info{{PID: 10}}, nil
		})
	if code != 0 || lookups != 2 || checks != 4 || diag.Len() != 0 || !json.Valid(out.Bytes()) {
		t.Fatalf("%d %d %d %q %q", code, lookups, checks, &out, &diag)
	}
}

func TestProcessCancellationAndOutputFailure(t *testing.T) {
	for _, mode := range []string{"--watch", "--wait-in-use"} {
		ctx, cancel := context.WithCancel(context.Background())
		var out, diag bytes.Buffer
		code := runContextWithInspector(ctx, []string{"--process", mode, "8080"}, &out, &diag,
			func(_ context.Context, _ string, p int) portcheck.Result {
				return portcheck.Result{Port: p, Status: portcheck.StatusInUse}
			},
			func(context.Context, string, int) ([]process.Info, error) { cancel(); return nil, context.Canceled })
		want := 1
		if mode == "--watch" {
			want = 0
		}
		if code != want || out.Len() != 0 || diag.Len() != 0 {
			t.Fatalf("%s %d %q %q", mode, code, &out, &diag)
		}
	}
	var diag bytes.Buffer
	code := runContextWithInspector(context.Background(), []string{"--process", "8080"}, &failProcessWriter{}, &diag,
		func(_ context.Context, _ string, p int) portcheck.Result {
			return portcheck.Result{Port: p, Status: portcheck.StatusInUse}
		},
		func(context.Context, string, int) ([]process.Info, error) { return []process.Info{{PID: 10}}, nil })
	if code != 3 || !strings.Contains(diag.String(), "write output") {
		t.Fatalf("%d %q", code, &diag)
	}
}

type failProcessWriter struct{ writes int }

func (w *failProcessWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.writes >= 3 {
		return 0, io.ErrClosedPipe
	}
	return len(p), nil
}

func TestProcessMetadataEscaping(t *testing.T) {
	detail := processDetail{owners: []process.Info{{PID: 10, Name: "bad\x1b[31m\nname", Executable: "/bin/\tbad"}}, err: errors.New("bad\x1b\nerror")}
	var out bytes.Buffer
	if err := writeProcess(&out, detail); err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(out.String(), "\x1b\t") || strings.Contains(out.String(), "\nname") {
		t.Fatal(out.String())
	}
	var encoded bytes.Buffer
	if err := writeJSONWithProcesses(&encoded, []portcheck.Result{{Port: 8080, Status: portcheck.StatusInUse}}, map[int]processDetail{8080: detail}); err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Processes []process.Info `json:"processes"`
	}
	if err := json.Unmarshal(encoded.Bytes(), &rows); err != nil || rows[0].Processes[0].Name != detail.owners[0].Name {
		t.Fatalf("%+v %v", rows, err)
	}
}
