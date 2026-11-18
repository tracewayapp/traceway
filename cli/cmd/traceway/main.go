package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/tracewayapp/traceway/cli/v2/internal/exitcode"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	root := newRootCmd()
	if err := root.ExecuteContext(ctx); err != nil {
		var ce *cliError
		if errors.As(err, &ce) {
			os.Exit(ce.code)
		}
		if os.Getenv("TRACEWAY_DEBUG") == "1" {
			fmt.Fprintln(os.Stderr, "debug:", err)
		}
		os.Exit(exitcode.Usage)
	}
	os.Exit(exitcode.Success)
}
