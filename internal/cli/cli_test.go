package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/aman-void/portcheck"
)

func TestParseValidPorts(t *testing.T) {
	for _, token := range []string{"1", "80", "443", "3000", "8080", "65535", "08080"} {
		t.Run(token, func(t *testing.T) {
			got, err := parse([]string{token})
			want, _ := strconv.Atoi(token)
			if err != nil || !reflect.DeepEqual(got.ports, []int{want}) {
				t.Fatalf("parse = %+v, %v", got, err)
			}
		})
	}
}

func TestInvalidInputDoesNotCheck(t *testing.T) {
	cases := [][]string{
		nil, {"0"}, {"-1"}, {"65536"}, {"99999"}, {"abc"}, {"8080abc"},
		{"1.5"}, {""}, {" 8080"}, {"8080 "}, {"+8080"}, {"999999999999999999999999"},
		{"3000", "abc", "8080"}, {"--foobar", "8080"}, {"8080", "--foobar"},
		{"--help", "--foobar"}, {"3010-3000"}, {"--json", "--quiet", "8080"},
		{"--find", "3000", "4000"}, {"--host", "127.0.0.1", "8080"}, {"--watch", "8080"},
		{"--wait", "8080"}, {"--process", "8080"}, {"--connect", "localhost:8080"},
		{"doctor", "8080"}, {"--", "-q"},
	}
	for _, args := range cases {
		t.Run(fmt.Sprint(args), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(args, &stdout, &stderr, func(int) portcheck.Result {
				t.Fatal("network check on invalid invocation")
				return portcheck.Result{}
			})
			if code != 2 || stdout.Len() != 0 || !strings.HasPrefix(stderr.String(), "error: ") {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, &stdout, &stderr)
			}
		})
	}
}

func TestHelpAndVersion(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"-h"}, help},
		{[]string{"--help"}, help},
		{[]string{"--help", "abc"}, help},
		{[]string{"--help", "--version"}, help},
		{[]string{"-v"}, "portcheck version 0.2.0\n"},
		{[]string{"--version"}, "portcheck version 0.2.0\n"},
	} {
		var stdout, stderr bytes.Buffer
		code := run(tc.args, &stdout, &stderr, func(int) portcheck.Result {
			t.Fatal("help/version performed network operation")
			return portcheck.Result{}
		})
		if code != 0 || stdout.String() != tc.want || stderr.Len() != 0 {
			t.Fatalf("%v: code=%d stdout=%q stderr=%q", tc.args, code, &stdout, &stderr)
		}
	}
}

func TestOrderDeduplicationAndQuiet(t *testing.T) {
	for _, flag := range []string{"", "-q", "--quiet"} {
		args := []string{"8080", "3000", "08080", "5432", "3000"}
		if flag != "" {
			args = append(args, flag) // Options also work after positionals.
		}
		var checked []int
		var stdout, stderr bytes.Buffer
		code := run(args, &stdout, &stderr, func(port int) portcheck.Result {
			checked = append(checked, port)
			status := portcheck.StatusFree
			if port == 8080 {
				status = portcheck.StatusInUse
			}
			return portcheck.Result{Port: port, Status: status}
		})
		want := "PORT    STATUS\n8080    IN USE\n3000    FREE\n5432    FREE\n"
		if flag != "" {
			want = "IN_USE\nFREE\nFREE\n"
		}
		if code != 1 || stdout.String() != want || stderr.Len() != 0 || !reflect.DeepEqual(checked, []int{8080, 3000, 5432}) {
			t.Fatalf("flag=%q code=%d checked=%v stdout=%q stderr=%q", flag, code, checked, &stdout, &stderr)
		}
	}
}

func TestSystemErrorPrecedenceAndContinuation(t *testing.T) {
	for _, args := range [][]string{{"3000", "8080", "5432"}, {"8080", "3000", "5432"}} {
		var stdout, stderr bytes.Buffer
		count := 0
		code := run(args, &stdout, &stderr, func(port int) portcheck.Result {
			count++
			if port == 3000 {
				return portcheck.Result{Port: port, Status: portcheck.StatusError, Err: errors.New("permission denied")}
			}
			if port == 8080 {
				return portcheck.Result{Port: port, Status: portcheck.StatusInUse}
			}
			return portcheck.Result{Port: port, Status: portcheck.StatusFree}
		})
		if code != 3 || count != 3 || !strings.Contains(stderr.String(), "permission denied") || strings.Contains(stdout.String(), "permission denied") || !strings.Contains(stdout.String(), "5432    FREE") {
			t.Fatalf("code=%d checks=%d stdout=%q stderr=%q", code, count, &stdout, &stderr)
		}
	}
}

func TestRunRealListener(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	port := listener.Addr().(*net.TCPAddr).Port
	for _, flag := range []string{"-q", "--quiet"} {
		var stdout, stderr bytes.Buffer
		if code := Run([]string{flag, strconv.Itoa(port)}, &stdout, &stderr); code != 1 || stdout.String() != "IN_USE\n" || stderr.Len() != 0 {
			t.Fatalf("occupied: code=%d stdout=%q stderr=%q", code, &stdout, &stderr)
		}
	}
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"--quiet", "--", strconv.Itoa(port)}, &stdout, &stderr); code != 0 || stdout.String() != "FREE\n" || stderr.Len() != 0 {
		t.Fatalf("free: code=%d stdout=%q stderr=%q", code, &stdout, &stderr)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestOutputFailures(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"--version"}, {"8080"}, {"--quiet", "8080"}, {"--json", "8080"}, {"--find", "8080"}, {"--find", "--json", "8080"}} {
		var stderr bytes.Buffer
		code := run(args, failingWriter{}, &stderr, func(port int) portcheck.Result {
			return portcheck.Result{Port: port, Status: portcheck.StatusFree}
		})
		if code != 3 || !strings.Contains(stderr.String(), "write output") {
			t.Fatalf("%v: code=%d stderr=%q", args, code, &stderr)
		}
	}
}
