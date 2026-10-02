package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aman-void/portcheck"
	"github.com/aman-void/portcheck/internal/process"
)

func TestConnectivityModeConflictsBeforeNetworking(t *testing.T) {
	for _, args := range [][]string{
		{"--connect"}, {"--connect", ""}, {"--connect", "localhost"},
		{"--connect", "-x", "--help"}, {"--help", "--connect", "--unknown"},
		{"--connect", "--quiet"}, {"--connect", "--quiet", "--unknown", "localhost:80"},
		{"--connect", "--json", "--connect", "localhost:80"},
		{"--connect", "--process", "localhost:80"}, {"--connect", "--watch", "localhost:80"},
		{"--connect", "--timeout", "1s", "localhost:80", "3000"},
		{"--host", "--quiet"}, {"--host", "-q", "8080"}, {"--host", "--json"},
		{"--connect", "localhost:80", "3000"}, {"--connect", "localhost:80", "--watch"},
		{"--connect", "localhost:80", "--wait"}, {"--connect", "localhost:80", "--wait-in-use"},
		{"--connect", "localhost:80", "--find"}, {"--connect", "localhost:80", "--process"},
		{"--connect", "localhost:80", "--host", "127.0.0.1"},
		{"--connect", "localhost:80", "--interval", "1s"},
		{"--connect", "localhost:80", "--quiet", "--json"},
		{"--connect", "localhost:80", "--timeout", "0s"},
		{"--connect", "localhost:80", "--timeout", "-1s"},
		{"--connect", "localhost:80", "--timeout", "garbage"},
		{"--connect", "localhost:80", "--connect", "localhost:81"},
		{"doctor"}, {"doctor", "8080", "8081"}, {"doctor", "8080-8080"},
		{"doctor", "0"}, {"doctor", "8080", "--quiet"},
		{"doctor", "8080", "--find"}, {"doctor", "8080", "--watch"},
		{"doctor", "8080", "--wait"}, {"doctor", "8080", "--wait-in-use"},
		{"doctor", "8080", "--connect", "localhost:80"},
		{"doctor", "8080", "--interval", "1s"},
		{"doctor", "8080", "--host", "bad host"},
		{"--", "doctor", "8080"},
	} {
		t.Run(fmt.Sprint(args), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := runContextWithConnector(context.Background(), args, &stdout, &stderr,
				func(context.Context, string, int) portcheck.Result {
					t.Fatal("unexpected bind")
					return portcheck.Result{}
				},
				func(context.Context, string, int) ([]process.Info, error) {
					t.Fatal("unexpected inspection")
					return nil, nil
				},
				func(context.Context, string, time.Duration) (portcheck.ConnectivityResult, error) {
					t.Fatal("unexpected dial")
					return portcheck.ConnectivityResult{}, nil
				})
			if code != 2 || stdout.Len() != 0 || stderr.Len() == 0 {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, &stdout, &stderr)
			}
		})
	}
}

func TestConnectCLIStatusesAndOutput(t *testing.T) {
	for _, status := range []portcheck.ConnectivityStatus{portcheck.ConnectivityReachable, portcheck.ConnectivityRefused, portcheck.ConnectivityTimeout, portcheck.ConnectivityError} {
		for _, mode := range []string{"", "--quiet", "--json"} {
			t.Run(string(status)+mode, func(t *testing.T) {
				args := []string{"--connect", "localhost:8080"}
				if mode != "" {
					args = append(args, mode)
				}
				var stdout, stderr bytes.Buffer
				calls := 0
				code := runContextWithConnector(context.Background(), args, &stdout, &stderr,
					func(context.Context, string, int) portcheck.Result {
						t.Fatal("connect bound a port")
						return portcheck.Result{}
					},
					func(context.Context, string, int) ([]process.Info, error) {
						t.Fatal("connect inspected process")
						return nil, nil
					},
					func(_ context.Context, address string, timeout time.Duration) (portcheck.ConnectivityResult, error) {
						calls++
						if address != "localhost:8080" || timeout != portcheck.DefaultConnectTimeout {
							t.Fatalf("address=%s timeout=%s", address, timeout)
						}
						var err error
						if status != portcheck.ConnectivityReachable {
							err = errors.New("controlled diagnostic")
						}
						return portcheck.ConnectivityResult{Address: address, Status: status, Duration: 420 * time.Microsecond, Err: err}, err
					})
				wantCode := 1
				if status == portcheck.ConnectivityReachable {
					wantCode = 0
				}
				if status == portcheck.ConnectivityError {
					wantCode = 3
				}
				if code != wantCode || calls != 1 {
					t.Fatalf("code=%d calls=%d", code, calls)
				}
				if status == portcheck.ConnectivityReachable && stderr.Len() != 0 {
					t.Fatalf("stderr=%q", &stderr)
				}
				if status != portcheck.ConnectivityReachable && !strings.Contains(stderr.String(), "controlled diagnostic") {
					t.Fatalf("stderr=%q", &stderr)
				}
				switch mode {
				case "--quiet":
					if stdout.String() != strings.ToUpper(string(status))+"\n" {
						t.Fatalf("stdout=%q", &stdout)
					}
				case "--json":
					var records []connectivityRecord
					if err := json.Unmarshal(stdout.Bytes(), &records); err != nil || len(records) != 1 {
						t.Fatalf("stdout=%q err=%v", &stdout, err)
					}
					if records[0].Address != "localhost:8080" || records[0].Status != status || records[0].DurationMS != 0.42 {
						t.Fatalf("record=%+v", records[0])
					}
					if status == portcheck.ConnectivityReachable && strings.Contains(stdout.String(), "\"error\"") {
						t.Fatal("success includes error")
					}
				default:
					if !strings.Contains(stdout.String(), strings.ToUpper(string(status))) || !strings.Contains(stdout.String(), "0.42ms") {
						t.Fatalf("stdout=%q", &stdout)
					}
				}
			})
		}
	}
}

func TestConnectTimeoutParsing(t *testing.T) {
	for _, args := range [][]string{{"--connect", "[::1]:8080", "--timeout", "2s"}, {"--timeout", "2s", "doctor", "--host", "::1", "8080"}} {
		opts, err := parse(args)
		if err != nil || opts.timeout != 2*time.Second {
			t.Fatalf("opts=%+v err=%v", opts, err)
		}
	}
}

func TestKnownFlagsBetweenConnectAndEndpoint(t *testing.T) {
	for _, tc := range []struct {
		args    []string
		want    string
		timeout time.Duration
	}{
		{[]string{"--connect", "--quiet", "localhost:8080"}, "REACHABLE\n", portcheck.DefaultConnectTimeout},
		{[]string{"--quiet", "--connect", "localhost:8080"}, "REACHABLE\n", portcheck.DefaultConnectTimeout},
		{[]string{"--connect", "localhost:8080", "--quiet"}, "REACHABLE\n", portcheck.DefaultConnectTimeout},
		{[]string{"--connect", "-q", "localhost:8080"}, "REACHABLE\n", portcheck.DefaultConnectTimeout},
		{[]string{"--connect", "--json", "localhost:8080"}, "[{\"address\":\"localhost:8080\",\"status\":\"reachable\",\"duration_ms\":0.42}]\n", portcheck.DefaultConnectTimeout},
		{[]string{"--connect", "--timeout", "1s", "localhost:8080"}, "ADDRESS  STATUS  LATENCY\nlocalhost:8080  REACHABLE  0.42ms\n", time.Second},
		{[]string{"--connect", "--timeout", "1s", "--quiet", "localhost:8080"}, "REACHABLE\n", time.Second},
		{[]string{"--connect", "--json", "--timeout", "1s", "localhost:8080"}, "[{\"address\":\"localhost:8080\",\"status\":\"reachable\",\"duration_ms\":0.42}]\n", time.Second},
	} {
		t.Run(fmt.Sprint(tc.args), func(t *testing.T) {
			var out, diag bytes.Buffer
			calls := 0
			code := runContextWithConnector(context.Background(), tc.args, &out, &diag,
				func(context.Context, string, int) portcheck.Result {
					t.Fatal("unexpected bind")
					return portcheck.Result{}
				},
				func(context.Context, string, int) ([]process.Info, error) {
					t.Fatal("unexpected inspection")
					return nil, nil
				},
				func(_ context.Context, address string, timeout time.Duration) (portcheck.ConnectivityResult, error) {
					calls++
					if address != "localhost:8080" || timeout != tc.timeout {
						t.Fatalf("address=%s timeout=%s", address, timeout)
					}
					return portcheck.ConnectivityResult{Address: address, Status: portcheck.ConnectivityReachable, Duration: 420 * time.Microsecond}, nil
				})
			if code != 0 || calls != 1 || out.String() != tc.want || diag.Len() != 0 {
				t.Fatalf("code=%d calls=%d out=%q diag=%q", code, calls, &out, &diag)
			}
		})
	}
}

func TestHostDefaultWhenFollowedByLongFlag(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"--host", "--quiet", "8080"}, "FREE\n"},
		{[]string{"--host", "--json", "8080"}, "[{\"port\":8080,\"status\":\"free\"}]\n"},
		{[]string{"--host", "--process", "8080"}, "PORT    STATUS\n8080    FREE\n"},
	} {
		var out, diag bytes.Buffer
		calls := 0
		code := runContext(context.Background(), tc.args, &out, &diag, func(_ context.Context, host string, port int) portcheck.Result {
			calls++
			if host != "127.0.0.1" || port != 8080 {
				t.Fatalf("host=%s port=%d", host, port)
			}
			return portcheck.Result{Port: port, Status: portcheck.StatusFree}
		})
		if code != 0 || calls != 1 || out.String() != tc.want || diag.Len() != 0 {
			t.Fatalf("%v: code=%d calls=%d out=%q diag=%q", tc.args, code, calls, &out, &diag)
		}
	}
}

func TestDoctorUsesSameBudgetAcrossStages(t *testing.T) {
	parentDeadline := time.Now().Add(time.Hour)
	ctx, cancel := context.WithDeadline(context.Background(), parentDeadline)
	defer cancel()
	var firstContext context.Context
	verify := func(ctx context.Context) {
		t.Helper()
		deadline, ok := ctx.Deadline()
		if !ok || !deadline.Equal(parentDeadline) {
			t.Fatalf("deadline=%v, %v; want %v", deadline, ok, parentDeadline)
		}
		if firstContext == nil {
			firstContext = ctx
		} else if firstContext != ctx {
			t.Fatal("stage got a different context budget")
		}
	}
	var out, diag bytes.Buffer
	code := runContextWithConnector(ctx, []string{"doctor", "8080", "--timeout", "2h", "--json"}, &out, &diag,
		func(ctx context.Context, _ string, port int) portcheck.Result {
			verify(ctx)
			return portcheck.Result{Port: port, Status: portcheck.StatusInUse}
		}, func(ctx context.Context, _ string, _ int) ([]process.Info, error) {
			verify(ctx)
			return []process.Info{{PID: 1}}, nil
		}, func(ctx context.Context, address string, _ time.Duration) (portcheck.ConnectivityResult, error) {
			verify(ctx)
			return portcheck.ConnectivityResult{Address: address, Status: portcheck.ConnectivityReachable}, nil
		})
	if code != 0 || diag.Len() != 0 {
		t.Fatalf("code=%d diag=%q", code, &diag)
	}
}

func TestDoctorBudgetExpiresDuringBindOrInspection(t *testing.T) {
	for _, stage := range []string{"bind", "inspection"} {
		t.Run(stage, func(t *testing.T) {
			var out, diag bytes.Buffer
			var sharedContext context.Context
			code := runContextWithConnector(context.Background(), []string{"doctor", "8080", "--timeout", "5ms", "--json"}, &out, &diag,
				func(ctx context.Context, _ string, port int) portcheck.Result {
					sharedContext = ctx
					if _, ok := ctx.Deadline(); !ok {
						t.Fatal("bind has no deadline")
					}
					if stage == "bind" {
						<-ctx.Done()
						return portcheck.Result{Port: port, Status: portcheck.StatusError, Err: ctx.Err()}
					}
					return portcheck.Result{Port: port, Status: portcheck.StatusInUse}
				}, func(ctx context.Context, _ string, _ int) ([]process.Info, error) {
					if stage != "inspection" || ctx != sharedContext {
						t.Fatal("unexpected inspection or budget")
					}
					<-ctx.Done()
					return nil, ctx.Err()
				}, func(ctx context.Context, address string, _ time.Duration) (portcheck.ConnectivityResult, error) {
					if ctx != sharedContext || !errors.Is(ctx.Err(), context.DeadlineExceeded) {
						t.Fatal("dial got a fresh budget")
					}
					return portcheck.ConnectivityResult{Address: address, Status: portcheck.ConnectivityTimeout, Err: ctx.Err()}, ctx.Err()
				})
			wantExit := 1
			if stage == "bind" {
				wantExit = 3
			}
			var records []doctorRecord
			if code != wantExit || json.Unmarshal(out.Bytes(), &records) != nil || len(records) != 1 || !strings.Contains(diag.String(), "doctor timeout (budget 5ms") {
				t.Fatalf("code=%d out=%q diag=%q", code, &out, &diag)
			}
			if records[0].Connectivity.Status != portcheck.ConnectivityTimeout {
				t.Fatalf("record=%+v", records[0])
			}
			if stage == "inspection" && (records[0].Local.Status != portcheck.StatusInUse || records[0].ProcessError == "") {
				t.Fatalf("observations overwritten: %+v", records[0])
			}
		})
	}
}

func TestConnectivityOutputFailures(t *testing.T) {
	for _, args := range [][]string{{"--connect", "localhost:80"}, {"--connect", "localhost:80", "--quiet"}, {"--connect", "localhost:80", "--json"}, {"doctor", "80"}, {"doctor", "80", "--json"}} {
		var stderr bytes.Buffer
		code := runContextWithConnector(context.Background(), args, failingWriter{}, &stderr,
			func(_ context.Context, _ string, port int) portcheck.Result {
				return portcheck.Result{Port: port, Status: portcheck.StatusFree}
			},
			func(context.Context, string, int) ([]process.Info, error) {
				t.Fatal("inspected free port")
				return nil, nil
			},
			func(_ context.Context, address string, _ time.Duration) (portcheck.ConnectivityResult, error) {
				return portcheck.ConnectivityResult{Address: address, Status: portcheck.ConnectivityReachable}, nil
			})
		if code != 3 || !strings.Contains(stderr.String(), "write output") {
			t.Fatalf("%v: code=%d stderr=%q", args, code, &stderr)
		}
	}
}

func TestDoctorComposition(t *testing.T) {
	for _, tc := range []struct {
		name        string
		local       portcheck.Status
		connection  portcheck.ConnectivityStatus
		processErr  error
		emptyOwners bool
		want        int
	}{
		{"free", portcheck.StatusFree, portcheck.ConnectivityRefused, nil, false, 1},
		{"occupied", portcheck.StatusInUse, portcheck.ConnectivityReachable, nil, false, 0},
		{"permission", portcheck.StatusInUse, portcheck.ConnectivityReachable, process.ErrPermission, false, 0},
		{"unsupported", portcheck.StatusInUse, portcheck.ConnectivityReachable, process.ErrUnsupported, false, 0},
		{"disappeared", portcheck.StatusInUse, portcheck.ConnectivityRefused, nil, true, 1},
		{"bind error", portcheck.StatusError, portcheck.ConnectivityReachable, nil, false, 3},
		{"dial error", portcheck.StatusFree, portcheck.ConnectivityError, nil, false, 3},
		{"timeout", portcheck.StatusInUse, portcheck.ConnectivityTimeout, nil, false, 1},
	} {
		for _, asJSON := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/json=%v", tc.name, asJSON), func(t *testing.T) {
				var stdout, stderr bytes.Buffer
				var operations []string
				args := []string{"doctor", "--host", "::1", "8080"}
				if asJSON {
					args = append(args, "--json")
				}
				code := runContextWithConnector(context.Background(), args, &stdout, &stderr,
					func(_ context.Context, host string, port int) portcheck.Result {
						operations = append(operations, "bind")
						if host != "::1" || port != 8080 {
							t.Fatalf("host=%s port=%d", host, port)
						}
						var err error
						if tc.local == portcheck.StatusError {
							err = errors.New("bind failure")
						}
						return portcheck.Result{Port: port, Status: tc.local, Err: err}
					},
					func(context.Context, string, int) ([]process.Info, error) {
						operations = append(operations, "process")
						if tc.emptyOwners || tc.processErr != nil {
							return nil, tc.processErr
						}
						return []process.Info{{PID: 20, Name: "api\x1b", Executable: "/bin/api"}, {PID: 10}}, nil
					},
					func(_ context.Context, address string, timeout time.Duration) (portcheck.ConnectivityResult, error) {
						operations = append(operations, "dial")
						if address != "[::1]:8080" || timeout != portcheck.DefaultConnectTimeout {
							t.Fatalf("address=%s timeout=%s", address, timeout)
						}
						var err error
						if tc.connection != portcheck.ConnectivityReachable {
							err = errors.New("dial observation")
						}
						return portcheck.ConnectivityResult{Address: address, Status: tc.connection, Err: err}, err
					})
				if code != tc.want {
					t.Fatalf("code=%d stdout=%q stderr=%q", code, &stdout, &stderr)
				}
				wantOps := "bind,dial"
				if tc.local == portcheck.StatusInUse {
					wantOps = "bind,process,dial"
				}
				if strings.Join(operations, ",") != wantOps {
					t.Fatalf("operations=%v", operations)
				}
				if asJSON {
					var records []doctorRecord
					if err := json.Unmarshal(stdout.Bytes(), &records); err != nil || len(records) != 1 {
						t.Fatalf("stdout=%q err=%v", &stdout, err)
					}
					record := records[0]
					if record.Endpoint.Host != "::1" || record.Endpoint.Port != 8080 || record.Endpoint.Protocol != "tcp" || record.Local.Status != tc.local || record.Connectivity.Status != tc.connection {
						t.Fatalf("record=%+v", record)
					}
					if tc.local == portcheck.StatusInUse && tc.processErr == nil && !tc.emptyOwners {
						if len(record.Processes) != 2 || record.Processes[0].PID != 10 || record.Processes[1].Name != "api\x1b" {
							t.Fatalf("processes=%+v", record.Processes)
						}
					}
					if (tc.processErr != nil || tc.emptyOwners) && record.ProcessError == "" {
						t.Fatal("missing process error")
					}
				} else {
					if strings.Contains(stdout.String(), "\x1b") {
						t.Fatal("unsafe metadata rendered")
					}
					if !strings.Contains(stdout.String(), "Portcheck Doctor") || !strings.Contains(stdout.String(), "Connectivity") {
						t.Fatalf("stdout=%q", &stdout)
					}
					if tc.local == portcheck.StatusFree && !strings.Contains(stdout.String(), "Status: none") {
						t.Fatalf("stdout=%q", &stdout)
					}
				}
			})
		}
	}
}

func TestConnectivityCLIRealListener(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	address := listener.Addr().String()
	port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
	for _, args := range [][]string{{"--connect", address, "--quiet"}, {"doctor", port, "--json"}} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr); code != 0 {
			t.Fatalf("%v: code=%d stdout=%q stderr=%q", args, code, &stdout, &stderr)
		}
		if strings.Contains(stdout.String(), "Doctor") {
			t.Fatal("JSON mixed with human output")
		}
	}
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"--connect", address, "--json"}, {"doctor", port, "--json"}} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr); code != 1 || !strings.Contains(stdout.String(), "\"status\":\"refused\"") {
			t.Fatalf("%v: code=%d stdout=%q stderr=%q", args, code, &stdout, &stderr)
		}
	}
}

func TestConnectivityCLICanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, args := range [][]string{{"--connect", "127.0.0.1:80", "--json"}, {"doctor", "80", "--json"}} {
		var stdout, stderr bytes.Buffer
		if code := RunContext(ctx, args, &stdout, &stderr); code != 3 || !strings.Contains(stdout.String(), "\"status\":\"error\"") || !strings.Contains(stderr.String(), "canceled") {
			t.Fatalf("code=%d stdout=%q stderr=%q", code, &stdout, &stderr)
		}
	}
}

func TestConnectivityHelpDoesNotDial(t *testing.T) {
	for _, args := range [][]string{{"--help", "--connect", "bad"}, {"doctor", "bad", "--help"}, {"--version", "--connect", "bad"}} {
		var stdout, stderr bytes.Buffer
		code := runContextWithConnector(context.Background(), args, &stdout, &stderr, nil, nil,
			func(context.Context, string, time.Duration) (portcheck.ConnectivityResult, error) {
				t.Fatal("help dialed")
				return portcheck.ConnectivityResult{}, io.EOF
			})
		if code != 0 || stdout.Len() == 0 || stderr.Len() != 0 {
			t.Fatalf("code=%d stdout=%q stderr=%q", code, &stdout, &stderr)
		}
	}
}
