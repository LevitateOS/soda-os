package runners

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type PKExecInvoker struct {
	Binary     string
	HelperPath string
}

func (invoker PKExecInvoker) Create(ctx context.Context, request CreateRequest) error {
	return invoker.mutate(ctx, "create", request)
}

func (invoker PKExecInvoker) Start(ctx context.Context, request RunnerRequest) error {
	return invoker.mutate(ctx, "start", request)
}

func (invoker PKExecInvoker) Stop(ctx context.Context, request RunnerRequest) error {
	return invoker.mutate(ctx, "stop", request)
}

func (invoker PKExecInvoker) Restart(ctx context.Context, request RunnerRequest) error {
	return invoker.mutate(ctx, "restart", request)
}

func (invoker PKExecInvoker) Remove(ctx context.Context, request RunnerRequest) error {
	return invoker.mutate(ctx, "remove", request)
}

func (invoker PKExecInvoker) List(ctx context.Context) ([]RunnerView, error) {
	var response []RunnerView
	err := invoker.invoke(ctx, "list", EmptyRequest{}, &response)
	return response, err
}

func (invoker PKExecInvoker) mutate(ctx context.Context, action string, request any) error {
	var response MutationResponse
	if err := invoker.invoke(ctx, action, request, &response); err != nil {
		return err
	}
	if !response.OK {
		return fmt.Errorf("privileged runner %s did not complete", action)
	}
	return nil
}

func (invoker PKExecInvoker) invoke(ctx context.Context, action string, request, response any) error {
	binary := invoker.Binary
	if binary == "" {
		binary = "/usr/bin/pkexec"
	}
	helper := invoker.HelperPath
	if helper == "" {
		helper = "/usr/libexec/soda/soda-runner-helper"
	}
	contents, err := json.Marshal(request)
	if err != nil {
		return err
	}
	command := exec.CommandContext(ctx, binary, "--disable-internal-agent", helper, action)
	command.Stdin = bytes.NewReader(contents)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err = command.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("privileged runner %s: %s", action, message)
	}
	decoder := json.NewDecoder(&stdout)
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(response); err != nil {
		return fmt.Errorf("decode privileged runner %s result: %w", action, err)
	}
	return nil
}
