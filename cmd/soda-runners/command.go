package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

var errUsage = errors.New("usage: soda-runners <list|create|start|stop|restart|remove>")

func execute(ctx context.Context, args []string, input io.Reader, output io.Writer, request func(context.Context, string, io.Reader) (any, error)) error {
	if len(args) != 1 {
		return errUsage
	}
	response, err := request(ctx, args[0], input)
	if err != nil {
		return err
	}
	if err := json.NewEncoder(output).Encode(response); err != nil {
		return fmt.Errorf("encode result: %w", err)
	}
	return nil
}
