package main

import (
	"bufio"
	"bytes"
	"context"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Run the real entrypoint in a subprocess, so os.Exit and signal registration
// are tested without interfering with the test runner's signal handlers.
func TestEntrypointHelper(t *testing.T) {
	if os.Getenv("PORTCHECK_ENTRYPOINT_HELPER") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{"portcheck"}, os.Args[i+1:]...)
			main()
			return
		}
	}
	t.Fatal("missing helper arguments")
}

func TestWatchEntrypointSignals(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix process signals are not available on Windows")
	}
	for _, sig := range []os.Signal{os.Interrupt, syscall.SIGTERM} {
		t.Run(sig.String(), func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			port := listener.Addr().(*net.TCPAddr).Port
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, executable, "-test.run=^TestEntrypointHelper$", "--", "--watch", "--interval", "100ms", strconv.Itoa(port))
			command.Env = append(os.Environ(), "PORTCHECK_ENTRYPOINT_HELPER=1")
			stdout, err := command.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			var stderr bytes.Buffer
			command.Stderr = &stderr
			if err := command.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if command.ProcessState == nil {
					command.Process.Kill()
					command.Wait()
				}
			}()
			scanner := bufio.NewScanner(stdout)
			if !scanner.Scan() || !strings.HasSuffix(scanner.Text(), "  "+strconv.Itoa(port)+" IN USE") {
				t.Fatalf("initial event: %q (%v)", scanner.Text(), scanner.Err())
			}
			if err := listener.Close(); err != nil {
				t.Fatal(err)
			}
			if !scanner.Scan() || !strings.HasSuffix(scanner.Text(), "  "+strconv.Itoa(port)+" FREE") {
				t.Fatalf("transition: %q (%v)", scanner.Text(), scanner.Err())
			}
			if err := command.Process.Signal(sig); err != nil {
				t.Fatal(err)
			}
			if err := command.Wait(); err != nil {
				t.Fatalf("exit: %v, stderr=%q", err, &stderr)
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr=%q", &stderr)
			}
		})
	}
}
