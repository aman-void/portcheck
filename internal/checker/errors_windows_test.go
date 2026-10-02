package checker

import (
	"errors"
	"fmt"
	"net"
	"os"
	"syscall"
	"testing"
)

func TestWinsockBindErrors(t *testing.T) {
	for _, tc := range []struct {
		code   syscall.Errno
		status Status
	}{
		{10048, StatusInUse}, // WSAEADDRINUSE
		{10013, StatusError}, // WSAEACCES is not evidence of occupation.
		{10049, StatusError}, // WSAEADDRNOTAVAIL
	} {
		t.Run(fmt.Sprint(tc.code), func(t *testing.T) {
			got := check(8080, func(string, string) (net.Listener, error) {
				return nil, &net.OpError{Op: "listen", Net: "tcp", Err: &os.SyscallError{Syscall: "bind", Err: tc.code}}
			})
			if got.Status != tc.status {
				t.Fatalf("got %+v, want %v", got, tc.status)
			}
			if tc.status == StatusInUse && got.Err != nil {
				t.Fatal(got.Err)
			}
			if tc.status == StatusError && !errors.Is(got.Err, tc.code) {
				t.Fatalf("lost cause: %v", got.Err)
			}
		})
	}
}
