package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

var errUsage = errors.New("usage: soda-workspace-helper <catalog-add|catalog-edit|removal-inspect|workspace-inspect|workspace-prepare|workspace-publish|workspace-remove|project-remove|human-delete>")

func execute(ctx context.Context, args []string, input io.Reader, output io.Writer, request func(context.Context, string, io.Reader) (any, error)) error {
	if len(args) != 1 {
		return errUsage
	}
	response, err := request(ctx, args[0], input)
	if err != nil {
		return err
	}
	// Completion belongs to the coordinator; transport success preserves the receipt.
	if err := json.NewEncoder(output).Encode(response); err != nil {
		return fmt.Errorf("encode result: %w", err)
	}
	return nil
}
