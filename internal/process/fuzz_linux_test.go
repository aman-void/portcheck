package process

import (
	"context"
	"net"
	"strings"
	"testing"
)

func FuzzListenerTable(f *testing.F) {
	for _, seed := range []string{"", "header\nshort\n", "header\n" + tcpRow(net.ParseIP("127.0.0.1").To4(), 8080, "0A", "123")} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, table string) {
		_, _ = listenerInodes(context.Background(), strings.NewReader(table), net.ParseIP("127.0.0.1"), 8080)
	})
}
