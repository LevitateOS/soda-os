package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/LevitateOS/soda-os/internal/setup"
)

const maximumRequestBytes = 16 << 10

var errReady = errors.New("network access is configured")

func main() {
	if os.Geteuid() != 0 && !(len(os.Args) == 2 && (os.Args[1] == "status" || os.Args[1] == "pending")) {
		fmt.Fprintln(os.Stderr, "soda-setup: root privileges are required")
		os.Exit(1)
	}
	if err := execute(context.Background(), setup.NewNativeService(), os.Args[1:], os.Stdin, os.Stdout); err != nil {
		if errors.Is(err, errReady) {
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "soda-setup:", err)
		os.Exit(2)
	}
}

func execute(ctx context.Context, service setup.Service, arguments []string, input io.Reader, output io.Writer) error {
	if len(arguments) != 1 {
		return errors.New("expected one of: status, pending, allow-local-network, connect-tailscale, console")
	}
	switch arguments[0] {
	case "status":
		status, err := service.Status(ctx)
		return writeJSON(output, status, err)
	case "pending":
		return setupPending(ctx, service)
	case "console":
		return runConsole(ctx, service, input, output)
	default:
		return executeMutation(ctx, service, arguments[0], input, output)
	}
}

func setupPending(ctx context.Context, service setup.Service) error {
	status, err := service.Status(ctx)
	if err != nil {
		return err
	}
	if status.Ready {
		return errReady
	}
	return nil
}

func executeMutation(ctx context.Context, service setup.Service, action string, input io.Reader, output io.Writer) error {
	switch action {
	case "allow-local-network":
		var request setup.LocalNetworkRequest
		if err := decodeRequest(input, &request); err != nil {
			return err
		}
		status, err := service.AllowLocalNetwork(ctx, request.Connection)
		return writeResponse(output, status, err)
	case "connect-tailscale":
		var request setup.TailscaleRequest
		if err := decodeRequest(input, &request); err != nil {
			return err
		}
		status, err := service.ConnectTailscale(ctx, request.AuthKey)
		request.AuthKey = ""
		return writeResponse(output, status, err)
	default:
		return fmt.Errorf("unsupported action %q", action)
	}
}

func decodeRequest(input io.Reader, target any) error {
	decoder := json.NewDecoder(io.LimitReader(input, maximumRequestBytes+1))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode request: %w", err)
	}
	if decoder.InputOffset() > maximumRequestBytes {
		return errors.New("request is too large")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("request must contain one JSON object")
	}
	return nil
}

func writeResponse(output io.Writer, status setup.Status, operationErr error) error {
	response := setup.Response{Status: status}
	if operationErr != nil {
		response.Error = operationErr.Error()
	}
	return writeJSON(output, response, nil)
}

func writeJSON(output io.Writer, value any, operationErr error) error {
	if operationErr != nil {
		return operationErr
	}
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(true)
	return encoder.Encode(value)
}

func readLine(reader lineReader, output io.Writer, prompt string) (string, error) {
	if _, err := fmt.Fprint(output, prompt); err != nil {
		return "", err
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}
