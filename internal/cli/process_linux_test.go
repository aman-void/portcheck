package cli

import (
	"bytes"
	"encoding/json"
	"net"
	"os"
	"strconv"
	"testing"

	"github.com/aman-void/portcheck/internal/process"
)

func TestProcessRealListener(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	port := l.Addr().(*net.TCPAddr).Port
	var out, diag bytes.Buffer
	code := Run([]string{"--process", "--json", strconv.Itoa(port)}, &out, &diag)
	var rows []struct {
		Port      int            `json:"port"`
		Status    string         `json:"status"`
		Processes []process.Info `json:"processes"`
	}
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	if code != 1 || diag.Len() != 0 || len(rows) != 1 || rows[0].Port != port || rows[0].Status != "in_use" {
		t.Fatalf("%d %+v %q", code, rows, &diag)
	}
	found := false
	for _, p := range rows[0].Processes {
		if p.PID == os.Getpid() {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing current PID: %+v", rows)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	diag.Reset()
	code = Run([]string{"--process", "--json", strconv.Itoa(port)}, &out, &diag)
	var free []map[string]any
	if err := json.Unmarshal(out.Bytes(), &free); err != nil {
		t.Fatal(err)
	}
	if code != 0 || diag.Len() != 0 || free[0]["status"] != "free" || free[0]["processes"] != nil || free[0]["process_error"] != nil {
		t.Fatalf("%d %+v %q", code, free, &diag)
	}
}
