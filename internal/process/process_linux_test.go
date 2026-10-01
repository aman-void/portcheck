package process

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestInspectRealListeners(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "::1", "0.0.0.0", "::"} {
		t.Run(host, func(t *testing.T) {
			// Force the observed family: Go's generic tcp wildcard listener may
			// use a dual-stack IPv6 socket even for the address 0.0.0.0.
			network := "tcp4"
			if strings.Contains(host, ":") {
				network = "tcp6"
			}
			l, err := net.Listen(network, net.JoinHostPort(host, "0"))
			if err != nil {
				if strings.Contains(host, ":") {
					t.Skipf("IPv6 unavailable: %v", err)
				}
				t.Fatal(err)
			}
			defer l.Close()
			port := l.Addr().(*net.TCPAddr).Port
			owners, err := Inspect(context.Background(), host, port)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, owner := range owners {
				if owner.PID == os.Getpid() {
					found = true
					if owner.Name == "" || owner.Executable == "" {
						t.Fatalf("missing metadata: %+v", owner)
					}
				}
			}
			if !found {
				t.Fatalf("current PID %d not in %+v", os.Getpid(), owners)
			}
			if err := l.Close(); err != nil {
				t.Fatal(err)
			}
			owners, err = Inspect(context.Background(), host, port)
			if !errors.Is(err, ErrNotFound) || len(owners) != 0 {
				t.Fatalf("after close: %+v %v", owners, err)
			}
		})
	}
}

func procAddress(ip net.IP) string {
	data := append([]byte(nil), ip...)
	for i := 0; i < len(data); i += 4 {
		binary.NativeEndian.PutUint32(data[i:i+4], binary.BigEndian.Uint32(data[i:i+4]))
	}
	return strings.ToUpper(hex.EncodeToString(data))
}

func tcpRow(ip net.IP, port int, state, inode string) string {
	return fmt.Sprintf("0: %s:%04X 00000000:0000 %s 0:0 0:0 0 1000 0 %s\n", procAddress(ip), port, state, inode)
}

func TestListenerTable(t *testing.T) {
	ip := net.ParseIP("127.0.0.1").To4()
	table := "header\n" + tcpRow(ip, 8080, "0A", "10") +
		tcpRow(net.ParseIP("127.0.0.2").To4(), 8080, "0A", "11") +
		tcpRow(ip, 8080, "01", "12") + tcpRow(ip, 8081, "0A", "13") +
		tcpRow(net.IPv4zero.To4(), 8080, "0A", "14") + tcpRow(net.IPv6zero, 8080, "0A", "15")
	found, err := listenerInodes(context.Background(), strings.NewReader(table), ip, 8080)
	if err != nil || !reflect.DeepEqual(found, map[string]bool{"10": true, "14": true}) {
		t.Fatalf("%v %v", found, err)
	}
	for _, ip := range []net.IP{net.ParseIP("127.0.0.1").To4(), net.ParseIP("::1"), net.ParseIP("2001:db8::1234")} {
		got, err := procIP(procAddress(ip))
		if err != nil || !got.Equal(ip) {
			t.Fatalf("%s: %s %v", ip, got, err)
		}
	}
	for _, row := range []string{"", "header\nshort\n", "header\n0: BAD:ZZ 0 0A 0 0 0 0 0 1\n", "header\n0: BAD:1F90 0 0A 0 0 0 0 0 1\n", "header\n" + tcpRow(ip, 8080, "0A", "bad")} {
		if _, err := listenerInodes(context.Background(), strings.NewReader(row), ip, 8080); err == nil {
			t.Fatalf("accepted malformed %q", row)
		}
	}
}

func writeFixture(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
}

func fixtureOwner(t *testing.T, root string, pid int, inode, name string) string {
	t.Helper()
	dir := filepath.Join(root, strconv.Itoa(pid))
	stat := strconv.Itoa(pid) + " (name with ) spaces) S " + strings.Repeat("0 ", 18) + "42\n"
	writeFixture(t, filepath.Join(dir, "stat"), stat)
	writeFixture(t, filepath.Join(dir, "comm"), name+"\n")
	if err := os.MkdirAll(filepath.Join(dir, "fd"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("socket:["+inode+"]", filepath.Join(dir, "fd", "3")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/example/server", filepath.Join(dir, "exe")); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestInspectProcSharedOwnersAndRaces(t *testing.T) {
	root := t.TempDir()
	ip := net.ParseIP("127.0.0.1").To4()
	writeFixture(t, filepath.Join(root, "net", "tcp"), "header\n"+tcpRow(ip, 8080, "0A", "123"))
	a := fixtureOwner(t, root, 30, "123", "server")
	b := fixtureOwner(t, root, 20, "123", "other")
	if err := os.Symlink("socket:[123]", filepath.Join(a, "fd", "4")); err != nil {
		t.Fatal(err)
	}
	// A stale PID with missing fd directory must not stop visible owners.
	writeFixture(t, filepath.Join(root, "40", "stat"), "stale")
	owners, err := inspectProc(context.Background(), root, ip, 8080)
	if err != nil || len(owners) != 2 || owners[0].PID != 20 || owners[1].PID != 30 {
		t.Fatalf("%+v %v", owners, err)
	}
	// Optional metadata may disappear without losing the valid PID.
	if err := os.Remove(filepath.Join(a, "comm")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(a, "exe")); err != nil {
		t.Fatal(err)
	}
	owners, err = inspectProc(context.Background(), root, ip, 8080)
	if err != nil || owners[1].Name != "" || owners[1].Executable != "" {
		t.Fatalf("%+v %v", owners, err)
	}
	// Socket table remains stale after both owners disappear.
	for _, dir := range []string{a, b} {
		if err := os.Remove(filepath.Join(dir, "fd", "3")); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Remove(filepath.Join(a, "fd", "4")); err != nil {
		t.Fatal(err)
	}
	owners, err = inspectProc(context.Background(), root, ip, 8080)
	if !errors.Is(err, ErrNotFound) || len(owners) != 0 {
		t.Fatalf("%+v %v", owners, err)
	}
}

func TestProcPermissionsAndCancellation(t *testing.T) {
	wrapped := procError(&os.PathError{Op: "open", Path: "proc", Err: os.ErrPermission})
	if !errors.Is(wrapped, ErrPermission) || !errors.Is(wrapped, os.ErrPermission) {
		t.Fatal(wrapped)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Inspect(ctx, "127.0.0.1", 8080); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := listenerInodes(ctx, strings.NewReader("header\n"+tcpRow(net.IPv4zero.To4(), 8080, "0A", "1")), net.IPv4zero, 8080); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	root := t.TempDir()
	if _, err := inspectProc(context.Background(), root, net.IPv4zero, 8080); err == nil {
		t.Fatal("missing proc silently accepted")
	}
}

func TestProcPermissionDenied(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses fixture permissions")
	}
	root := t.TempDir()
	ip := net.ParseIP("127.0.0.1").To4()
	writeFixture(t, filepath.Join(root, "net", "tcp"), "header\n"+tcpRow(ip, 8080, "0A", "123"))
	dir := fixtureOwner(t, root, 20, "123", "server")
	fdDir := filepath.Join(dir, "fd")
	if err := os.Chmod(fdDir, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(fdDir, 0700) })
	owners, err := inspectProc(context.Background(), root, ip, 8080)
	if len(owners) != 0 || !errors.Is(err, ErrPermission) {
		t.Fatalf("%+v %v", owners, err)
	}
	// An inaccessible process must not prevent another visible shared owner.
	fixtureOwner(t, root, 30, "123", "visible")
	owners, err = inspectProc(context.Background(), root, ip, 8080)
	if err != nil || len(owners) != 1 || owners[0].PID != 30 {
		t.Fatalf("%+v %v", owners, err)
	}
}

func TestAddressMatching(t *testing.T) {
	for _, tc := range []struct {
		requested, listener string
		want                bool
	}{
		{"127.0.0.1", "127.0.0.1", true}, {"127.0.0.1", "127.0.0.2", false},
		{"127.0.0.1", "0.0.0.0", true}, {"0.0.0.0", "127.0.0.2", true},
		{"::1", "::", true}, {"::", "::1", true}, {"::1", "2001:db8::1", false},
		{"127.0.0.1", "::", false}, {"::", "127.0.0.1", false},
	} {
		if got := conflicts(net.ParseIP(tc.requested), net.ParseIP(tc.listener)); got != tc.want {
			t.Fatalf("%+v: %v", tc, got)
		}
	}
	if _, err := Inspect(context.Background(), "fe80::1%lo", 8080); err == nil {
		t.Fatal("scoped IPv6 ownership guessed")
	}
}

func TestProcessDisappearsDuringMetadata(t *testing.T) {
	for _, race := range []string{"socket_closed", "pid_reused"} {
		t.Run(race, func(t *testing.T) {
			root := t.TempDir()
			ip := net.ParseIP("127.0.0.1").To4()
			writeFixture(t, filepath.Join(root, "net", "tcp"), "header\n"+tcpRow(ip, 8080, "0A", "123"))
			dir := fixtureOwner(t, root, 20, "123", "server")
			comm := filepath.Join(dir, "comm")
			if err := os.Remove(comm); err != nil {
				t.Fatal(err)
			}
			if err := syscall.Mkfifo(comm, 0600); err != nil {
				t.Fatal(err)
			}
			type result struct {
				owners []Info
				err    error
			}
			done := make(chan result, 1)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			go func() { owners, err := inspectProc(ctx, root, ip, 8080); done <- result{owners, err} }()
			// Opening a FIFO writer synchronizes with the inspector's metadata
			// read. No sleeps or assumptions about scheduling are needed.
			type opened struct {
				file *os.File
				err  error
			}
			ready := make(chan opened, 1)
			go func() { file, err := os.OpenFile(comm, os.O_WRONLY, 0); ready <- opened{file, err} }()
			var writer *os.File
			select {
			case result := <-ready:
				if result.err != nil {
					t.Fatal(result.err)
				}
				writer = result.file
			case <-ctx.Done():
				// Unblock a pending writer before failing the test.
				unblock, err := os.OpenFile(comm, os.O_RDWR|syscall.O_NONBLOCK, 0)
				if err == nil {
					result := <-ready
					if result.file != nil {
						result.file.Close()
					}
					unblock.Close()
				}
				t.Fatal("inspector did not open metadata")
			}
			defer writer.Close()
			if race == "socket_closed" {
				if err := os.Remove(filepath.Join(dir, "fd", "3")); err != nil {
					t.Fatal(err)
				}
			} else {
				writeFixture(t, filepath.Join(dir, "stat"), "20 (replacement) S "+strings.Repeat("0 ", 18)+"43\n")
			}
			if _, err := writer.WriteString("replacement\n"); err != nil {
				t.Fatal(err)
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			select {
			case got := <-done:
				if len(got.owners) != 0 || !errors.Is(got.err, ErrNotFound) {
					t.Fatalf("%+v %v", got.owners, got.err)
				}
			case <-ctx.Done():
				t.Fatal("inspection failed to finish")
			}
		})
	}
}
