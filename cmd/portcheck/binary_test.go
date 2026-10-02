package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// These tests execute a built binary: go run does not preserve CLI exit codes.
// A release-version override is exercised without mutating source/global state.
func TestBuiltBinaryContracts(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "portcheck")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	buildCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	build := exec.CommandContext(buildCtx, "go", "build", "-trimpath", "-buildvcs=false",
		"-ldflags", "-X github.com/aman-void/portcheck/internal/cli.version=1.0.0-rc.1", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	listen := func() net.Listener {
		t.Helper()
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { listener.Close() })
		return listener
	}
	busy := listen()
	port := strconv.Itoa(busy.Addr().(*net.TCPAddr).Port)
	free := listen()
	freePort := strconv.Itoa(free.Addr().(*net.TCPAddr).Port)
	if err := free.Close(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		args []string
		code int
		out  string
		json bool
	}{
		{"version", []string{"--version"}, 0, "portcheck version 1.0.0-rc.1\n", false},
		{"version alias", []string{"-v"}, 0, "portcheck version 1.0.0-rc.1\n", false},
		{"v01 single", []string{port}, 1, "PORT    STATUS\n" + port + strings.Repeat(" ", 8-len(port)) + "IN USE\n", false},
		{"v01 quiet", []string{"-q", freePort}, 0, "FREE\n", false},
		{"v01 order/dedup", []string{"--quiet", port, freePort, port}, 1, "IN_USE\nFREE\n", false},
		{"v01 invalid", []string{"70000"}, 2, "", false},
		{"v02 range/json", []string{"--json", port + "-" + port, port}, 1, "", true},
		{"v02 find", []string{"--find", "--quiet", freePort}, 0, freePort + "\n", false},
		{"v03 host shorthand", []string{"--host", "--quiet", port}, 1, "IN_USE\n", false},
		{"v03 wait", []string{"--wait", "--json", freePort}, 0, "", true},
		{"v03 wait-in-use", []string{"--wait-in-use", "--quiet", port}, 0, "IN_USE\n", false},
		{"v03 timeout", []string{"--wait", "--timeout", "50ms", "--json", port}, 1, "", false},
		{"v04 process", []string{"--process", "--json", port}, 1, "", true},
		{"v05 connect", []string{"--connect", busy.Addr().String(), "--quiet"}, 0, "REACHABLE\n", false},
		{"v05 doctor", []string{"doctor", port, "--json"}, 0, "", true},
		{"invalid combination", []string{"--watch", "--json", port}, 2, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, binary, tc.args...)
			var out, diag bytes.Buffer
			command.Stdout, command.Stderr = &out, &diag
			err := command.Run()
			code := 0
			if err != nil {
				if failure, ok := err.(*exec.ExitError); ok {
					code = failure.ExitCode()
				} else {
					t.Fatal(err)
				}
			}
			if code != tc.code {
				t.Fatalf("exit=%d want=%d stdout=%q stderr=%q", code, tc.code, &out, &diag)
			}
			if tc.json {
				var rows []map[string]any
				if err := json.Unmarshal(out.Bytes(), &rows); err != nil || len(rows) != 1 {
					t.Fatalf("JSON=%q error=%v", &out, err)
				}
				if tc.name == "v05 doctor" {
					if rows[0]["local"].(map[string]any)["status"] != "in_use" || rows[0]["connectivity"].(map[string]any)["status"] != "reachable" {
						t.Fatalf("doctor=%v", rows)
					}
				} else {
					status := "in_use"
					if tc.name == "v03 wait" {
						status = "free"
					}
					if rows[0]["status"] != status {
						t.Fatalf("rows=%v", rows)
					}
				}
			} else if out.String() != tc.out {
				t.Fatalf("stdout=%q want=%q", &out, tc.out)
			}
			if tc.code == 2 || tc.name == "v03 timeout" {
				if diag.Len() == 0 {
					t.Fatal("missing diagnostic")
				}
			} else if tc.name != "v04 process" && tc.name != "v05 doctor" && diag.Len() != 0 {
				t.Fatalf("unexpected stderr=%q", &diag)
			}
		})
	}
	t.Run("broken stdout pipe", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Unix SIGPIPE regression; Windows write failures covered by CLI tests")
		}
		reader, writer, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		reader.Close()
		defer writer.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, binary, "--version")
		var diag bytes.Buffer
		command.Stdout, command.Stderr = writer, &diag
		err = command.Run()
		failure, ok := err.(*exec.ExitError)
		if !ok || failure.ExitCode() != 3 || !strings.Contains(diag.String(), "write output") {
			t.Fatalf("exit=%v stderr=%q; want exit 3, not SIGPIPE", err, &diag)
		}
	})
}
