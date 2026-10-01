//go:build !linux

package process

import (
	"context"
	"errors"
	"testing"
)

func TestUnsupported(t *testing.T) {
	owners, err := Inspect(context.Background(), "127.0.0.1", 8080)
	if len(owners) != 0 || !errors.Is(err, ErrUnsupported) {
		t.Fatalf("%v %v", owners, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Inspect(ctx, "127.0.0.1", 8080); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
