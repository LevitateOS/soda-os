package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/LevitateOS/soda-os/internal/tailnet"
)

func execute(ctx context.Context, output io.Writer, endpoint func(context.Context) (tailnet.Endpoint, error)) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	current, err := endpoint(ctx)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(output, current.Identity, current.IPv4)
	return err
}
