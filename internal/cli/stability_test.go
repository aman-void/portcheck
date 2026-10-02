package cli

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"testing"
	"time"

	"github.com/aman-void/portcheck"
	"github.com/aman-void/portcheck/internal/process"
)

func FuzzPortBounds(f *testing.F) {
	for _, seed := range []string{"8080", "1-65535", "65535-65535", "3000--3001", "", "0", "\x1b[31m", "9999999999999999999999999"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, token string) {
		start, end, err := parseBounds(token)
		if err == nil && (start < 1 || end > 65535 || start > end) {
			t.Fatalf("invalid accepted bounds: %q -> %d,%d", token, start, end)
		}
	})
}

// Pure orchestration/formatting benchmarks: no fixed ports or OS socket costs.
func BenchmarkCLIContracts(b *testing.B) {
	check := func(_ context.Context, _ string, port int) portcheck.Result {
		return portcheck.Result{Port: port, Status: portcheck.StatusFree}
	}
	for _, count := range []int{1, 10, 100, 1000, 65535} {
		for _, output := range []string{"human", "json"} {
			b.Run(strconv.Itoa(count)+"/"+output, func(b *testing.B) {
				args := []string{fmt.Sprintf("1-%d", count)}
				if output == "json" {
					args = append(args, "--json")
				}
				b.ReportAllocs()
				for b.Loop() {
					if code := runContext(context.Background(), args, io.Discard, io.Discard, check); code != 0 {
						b.Fatal(code)
					}
				}
			})
		}
	}
	b.Run("find/1000-occupied", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			code := runContext(context.Background(), []string{"--find", "3000"}, io.Discard, io.Discard,
				func(_ context.Context, _ string, port int) portcheck.Result {
					status := portcheck.StatusInUse
					if port == 4000 {
						status = portcheck.StatusFree
					}
					return portcheck.Result{Port: port, Status: status}
				})
			if code != 0 {
				b.Fatal(code)
			}
		}
	})
	inspect := func(context.Context, string, int) ([]process.Info, error) {
		return []process.Info{{PID: 1, Name: "api", Executable: "/bin/api"}}, nil
	}
	connect := func(_ context.Context, address string, _ time.Duration) (portcheck.ConnectivityResult, error) {
		return portcheck.ConnectivityResult{Address: address, Status: portcheck.ConnectivityReachable}, nil
	}
	for _, mode := range []string{"process", "connect", "doctor"} {
		b.Run(mode, func(b *testing.B) {
			args := []string{"--process", "--json", "8080"}
			if mode == "connect" {
				args = []string{"--connect", "127.0.0.1:8080", "--json"}
			}
			if mode == "doctor" {
				args = []string{"doctor", "--json", "8080"}
			}
			b.ReportAllocs()
			for b.Loop() {
				code := runContextWithConnector(context.Background(), args, io.Discard, io.Discard,
					func(_ context.Context, _ string, port int) portcheck.Result {
						return portcheck.Result{Port: port, Status: portcheck.StatusInUse}
					}, inspect, connect)
				want := 0
				if mode == "process" {
					want = 1
				}
				if code != want {
					b.Fatal(code)
				}
			}
		})
	}
}
