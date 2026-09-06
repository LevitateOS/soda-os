package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/LevitateOS/soda-os/internal/projects"
)

var errUsage = errors.New("usage: soda-projects <list|add-existing|edit|inspect|removal-inspect|setup|remove-workspace|remove|delete-human>")
var errIncompleteRemoval = errors.New("removal did not complete; see the structured result on stdout")

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
	if removal, ok := response.(projects.RemovalResponse); ok && !removal.OK {
		return errIncompleteRemoval
	}
	return nil
}
