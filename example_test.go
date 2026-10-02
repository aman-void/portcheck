package portcheck_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/aman-void/portcheck"
)

func ExampleCheck() {
	result, err := portcheck.Check(context.Background(), 0)
	fmt.Println(result.Status)
	fmt.Println(errors.Is(err, portcheck.ErrInvalidPort))
	// Output:
	// error
	// true
}

func ExampleCheckPorts() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	results, err := portcheck.CheckPorts(ctx, []int{3000, 8080})
	fmt.Println(len(results))
	fmt.Println(errors.Is(err, context.Canceled))
	// Output:
	// 0
	// true
}

func ExampleConnect() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := portcheck.Connect(ctx, "127.0.0.1:8080", portcheck.DefaultConnectTimeout)
	fmt.Println(result.Status)
	fmt.Println(errors.Is(err, context.Canceled))
	// Output:
	// error
	// true
}

func ExampleValidateEndpoint() {
	err := portcheck.ValidateEndpoint("[::1]:8080")
	fmt.Println(err)
	// Output:
	// <nil>
}
