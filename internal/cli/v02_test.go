package cli

import (
	"bytes"
	"encoding/json"
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

func TestParseRanges(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want []int
	}{
		{[]string{"1-1"}, []int{1}},
		{[]string{"1-2"}, []int{1, 2}},
		{[]string{"65534-65535"}, []int{65534, 65535}},
		{[]string{"3000", "3001-3003", "3002"}, []int{3000, 3001, 3002, 3003}},
		{[]string{"8080-8082", "3000", "8081-8083"}, []int{8080, 8081, 8082, 3000, 8083}},
		{[]string{"03000-03002", "3001"}, []int{3000, 3001, 3002}},
		{[]string{"--", "3000-3002"}, []int{3000, 3001, 3002}},
	} {
		got, err := parse(tc.args)
		if err != nil || !reflect.DeepEqual(got.ports, tc.want) {
			t.Fatalf("%v: %+v, %v; want %v", tc.args, got, err, tc.want)
		}
	}
	got, err := parse([]string{"1-65535", "1-65535", "65535"})
	if err != nil || len(got.ports) != 65535 {
		t.Fatalf("full range: len=%d, %v", len(got.ports), err)
	}
	for i, port := range got.ports {
		if port != i+1 {
			t.Fatalf("full range: index %d = %d", i, port)
		}
	}
}

func TestInvalidRangesAndFindArguments(t *testing.T) {
	var cases [][]string
	for _, token := range []string{
		"0-10", "-1-10", "10-0", "65530-65536", "abc-100", "100-abc", "3000-",
		"-3000", "3000--3010", "3000-3010-3020", "3000-1000", "abc-def",
		"1-0", "0-1", "65535-65536", " 1-2", "1-2 ", "1 -2", "1-+2",
		"1-99999999999999999999999",
	} {
		cases = append(cases, []string{"8080", token})
	}
	cases = append(cases,
		[]string{"--find"}, []string{"--find", "1", "1"}, []string{"--find", "1-1"},
		[]string{"--find", "0"}, []string{"--find", "65536"}, []string{"--find", "abc"},
		[]string{"--find", "--quiet", "--json", "3000"}, []string{"-q", "--json", "3000"},
		[]string{"--json"}, []string{"--find=3000"},
	)
	for _, args := range cases {
		t.Run(fmt.Sprint(args), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(args, &stdout, &stderr, func(int) portcheck.Result {
				t.Fatal("invalid input performed a network check")
				return portcheck.Result{}
			})
			if code != 2 || stdout.Len() != 0 || stderr.Len() == 0 {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, &stdout, &stderr)
			}
		})
	}
}

func TestFindArgumentErrorMessages(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"--find"}, "error: --find requires exactly one starting port\n"},
		{[]string{"--find", "3000", "4000"}, "error: --find requires exactly one starting port\n"},
		{[]string{"--find", "3000-3010"}, "error: --find requires a single starting port, not a range\n"},
		{[]string{"--find", "3000-3000"}, "error: --find requires a single starting port, not a range\n"},
	} {
		t.Run(fmt.Sprint(tc.args), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tc.args, &stdout, &stderr, func(int) portcheck.Result {
				t.Fatal("invalid find performed a network check")
				return portcheck.Result{}
			})
			if code != 2 || stdout.Len() != 0 || stderr.String() != tc.want {
				t.Fatalf("code=%d stdout=%q stderr=%q; want %q", code, &stdout, &stderr, tc.want)
			}
		})
	}
}

func TestRangeOutputAndOrder(t *testing.T) {
	for _, quiet := range []bool{false, true} {
		args := []string{"8081", "8080-8082", "8081"}
		if quiet {
			args = append(args, "--quiet")
		}
		var checked []int
		var stdout, stderr bytes.Buffer
		code := run(args, &stdout, &stderr, func(port int) portcheck.Result {
			checked = append(checked, port)
			return portcheck.Result{Port: port, Status: portcheck.StatusFree}
		})
		want := "PORT    STATUS\n8081    FREE\n8080    FREE\n8082    FREE\n"
		if quiet {
			want = "FREE\nFREE\nFREE\n"
		}
		if code != 0 || stderr.Len() != 0 || stdout.String() != want || !reflect.DeepEqual(checked, []int{8081, 8080, 8082}) {
			t.Fatalf("code=%d checked=%v stdout=%q stderr=%q", code, checked, &stdout, &stderr)
		}
	}
}

type jsonRow struct {
	Port   int    `json:"port"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

func decodeResults(t *testing.T, data string) []jsonRow {
	t.Helper()
	var rows []jsonRow
	decoder := json.NewDecoder(strings.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&rows); err != nil {
		t.Fatalf("invalid JSON %q: %v", data, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		t.Fatalf("unexpected trailing output: %v", err)
	}
	return rows
}

func TestJSONResults(t *testing.T) {
	for _, fail := range []bool{false, true} {
		var stdout, stderr bytes.Buffer
		code := run([]string{"--json", "8081", "8080-8082", "8081"}, &stdout, &stderr, func(port int) portcheck.Result {
			if port == 8081 {
				return portcheck.Result{Port: port, Status: portcheck.StatusInUse}
			}
			if fail && port == 8080 {
				return portcheck.Result{Port: port, Status: portcheck.StatusError, Err: errors.New("permission denied")}
			}
			return portcheck.Result{Port: port, Status: portcheck.StatusFree}
		})
		want := []jsonRow{{8081, "in_use", ""}, {8080, "free", ""}, {8082, "free", ""}}
		wantCode := 1
		if fail {
			want[1] = jsonRow{8080, "error", "permission denied"}
			wantCode = 3
		}
		if got := decodeResults(t, stdout.String()); !reflect.DeepEqual(got, want) || code != wantCode || (stderr.Len() > 0) != fail {
			t.Fatalf("code=%d rows=%+v stderr=%q", code, got, &stderr)
		}
		var raw []map[string]any
		if err := json.Unmarshal(stdout.Bytes(), &raw); err != nil {
			t.Fatal(err)
		}
		if _, exists := raw[0]["error"]; exists {
			t.Fatal("successful result includes error field")
		}
	}
	var stdout bytes.Buffer
	if err := writeJSON(&stdout, nil); err != nil || stdout.String() != "[]\n" {
		t.Fatalf("empty JSON: %q, %v", &stdout, err)
	}
}

func TestFindResults(t *testing.T) {
	for _, args := range [][]string{{"--find", "3000"}, {"3000", "--find", "--quiet"}, {"--json", "--find", "3000"}} {
		for _, busy := range []int{0, 1, 3} {
			var stdout, stderr bytes.Buffer
			var checked []int
			code := run(args, &stdout, &stderr, func(port int) portcheck.Result {
				checked = append(checked, port)
				status := portcheck.StatusFree
				if port < 3000+busy {
					status = portcheck.StatusInUse
				}
				return portcheck.Result{Port: port, Status: status}
			})
			if code != 0 || stderr.Len() != 0 || len(checked) != busy+1 || checked[0] != 3000 || checked[busy] != 3000+busy {
				t.Fatalf("args=%v busy=%d code=%d checked=%v stderr=%q", args, busy, code, checked, &stderr)
			}
			if args[0] == "--json" {
				want := []jsonRow{{3000 + busy, "free", ""}}
				if got := decodeResults(t, stdout.String()); !reflect.DeepEqual(got, want) {
					t.Fatalf("find JSON = %+v, want %+v", got, want)
				}
			} else if stdout.String() != fmt.Sprintf("%d\n", 3000+busy) {
				t.Fatalf("find output = %q", &stdout)
			}
		}
	}
}

func TestFindUpperBoundAndErrors(t *testing.T) {
	for _, status := range []portcheck.Status{portcheck.StatusFree, portcheck.StatusInUse, portcheck.StatusError} {
		for _, asJSON := range []bool{false, true} {
			args := []string{"--find", "65535"}
			if asJSON {
				args = append(args, "--json")
			}
			var stdout, stderr bytes.Buffer
			count := 0
			code := run(args, &stdout, &stderr, func(port int) portcheck.Result {
				count++
				if port != 65535 {
					t.Fatalf("find wrapped or exceeded limit: %d", port)
				}
				result := portcheck.Result{Port: port, Status: status}
				if status == portcheck.StatusError {
					result.Err = errors.New("permission denied")
				}
				return result
			})
			wantCode := map[portcheck.Status]int{portcheck.StatusFree: 0, portcheck.StatusInUse: 1, portcheck.StatusError: 3}[status]
			if code != wantCode || count != 1 || (stderr.Len() > 0) != (status != portcheck.StatusFree) {
				t.Fatalf("status=%s code=%d count=%d stderr=%q", status, code, count, &stderr)
			}
			if status == portcheck.StatusInUse || (!asJSON && status == portcheck.StatusError) {
				if stdout.Len() != 0 {
					t.Fatalf("failed find emitted a port: %q", &stdout)
				}
			} else if asJSON {
				rows := decodeResults(t, stdout.String())
				if len(rows) != 1 || rows[0].Port != 65535 || rows[0].Status != string(status) {
					t.Fatalf("find JSON: %+v", rows)
				}
			}
		}
	}
}

func TestFindStopsOnSystemError(t *testing.T) {
	for _, asJSON := range []bool{false, true} {
		args := []string{"--find", "3000"}
		if asJSON {
			args = append(args, "--json")
		}
		var stdout, stderr bytes.Buffer
		count := 0
		code := run(args, &stdout, &stderr, func(port int) portcheck.Result {
			count++
			if port != 3000 {
				t.Fatalf("find continued after error: %d", port)
			}
			return portcheck.Result{Port: port, Status: portcheck.StatusError, Err: errors.New("resource exhaustion")}
		})
		if code != 3 || count != 1 || !strings.Contains(stderr.String(), "resource exhaustion") {
			t.Fatalf("code=%d count=%d stderr=%q", code, count, &stderr)
		}
	}
}

type limitedWriter struct {
	remaining int
}

func (w *limitedWriter) Write(data []byte) (int, error) {
	if len(data) > w.remaining {
		n := w.remaining
		w.remaining = 0
		return n, io.ErrClosedPipe
	}
	w.remaining -= len(data)
	return len(data), nil
}

func TestJSONPartialWriteFailures(t *testing.T) {
	results := []portcheck.Result{{Port: 3000, Status: portcheck.StatusFree}, {Port: 8080, Status: portcheck.StatusInUse}}
	var complete bytes.Buffer
	if err := writeJSON(&complete, results); err != nil {
		t.Fatal(err)
	}
	for available := 0; available < complete.Len(); available++ {
		if err := writeJSON(&limitedWriter{remaining: available}, results); !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("write limit %d: error = %v", available, err)
		}
	}
	var stderr bytes.Buffer
	code := run([]string{"--find", "--json", "3000"}, failingWriter{}, &stderr, func(port int) portcheck.Result {
		return portcheck.Result{Port: port, Status: portcheck.StatusError, Err: errors.New("permission denied")}
	})
	if code != 3 || !strings.Contains(stderr.String(), "write output") {
		t.Fatalf("code=%d stderr=%q", code, &stderr)
	}
}

// Allocate a controlled consecutive block without assuming a fixed free port.
func consecutiveListeners(t *testing.T) (int, []net.Listener) {
	t.Helper()
	for range 32 {
		first, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		start := first.Addr().(*net.TCPAddr).Port
		listeners := []net.Listener{first}
		if start <= 65533 {
			for port := start + 1; port <= start+2; port++ {
				listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
				if err != nil {
					break
				}
				listeners = append(listeners, listener)
			}
		}
		if len(listeners) == 3 {
			t.Cleanup(func() {
				for _, listener := range listeners {
					listener.Close()
				}
			})
			return start, listeners
		}
		for _, listener := range listeners {
			listener.Close()
		}
	}
	t.Skip("could not allocate three consecutive local TCP ports")
	return 0, nil
}

func TestRangeAndFindRealListeners(t *testing.T) {
	start, listeners := consecutiveListeners(t)
	if err := listeners[2].Close(); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	args := []string{"--json", fmt.Sprintf("%d-%d", start, start+2), strconv.Itoa(start)}
	code := Run(args, &stdout, &stderr)
	want := []jsonRow{{start, "in_use", ""}, {start + 1, "in_use", ""}, {start + 2, "free", ""}}
	if got := decodeResults(t, stdout.String()); code != 1 || stderr.Len() != 0 || !reflect.DeepEqual(got, want) {
		t.Fatalf("range: code=%d rows=%+v stderr=%q", code, got, &stderr)
	}
	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"--find", strconv.Itoa(start)}, &stdout, &stderr); code != 0 || stdout.String() != fmt.Sprintf("%d\n", start+2) || stderr.Len() != 0 {
		t.Fatalf("find: code=%d stdout=%q stderr=%q", code, &stdout, &stderr)
	}
}

func BenchmarkRangeExpansion(b *testing.B) {
	for _, size := range []int{1, 100, 1000, 65535} {
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			tokens := []string{fmt.Sprintf("1-%d", size)}
			b.ReportAllocs()
			for b.Loop() {
				if _, err := expandPorts(tokens); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
