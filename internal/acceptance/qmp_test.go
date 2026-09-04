package acceptance

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQMPClientNegotiatesAndReturnsTypedResult(t *testing.T) {
	directory, err := os.MkdirTemp("/tmp", "soda-qmp-test-")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(directory)) })
	socket := filepath.Join(directory, "qmp.sock")
	listener, err := net.Listen("unix", socket)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, listener.Close()) })
	done := make(chan error, 1)
	go serveQMPFixture(listener, done)

	var result struct {
		Status string `json:"status"`
	}
	err = (QMPClient{Socket: socket}).Execute(context.Background(), "query-status", "status", nil, &result)
	require.NoError(t, err)
	require.Equal(t, "running", result.Status)
	require.NoError(t, <-done)
}

func serveQMPFixture(listener net.Listener, done chan<- error) {
	connection, err := listener.Accept()
	if err != nil {
		done <- err
		return
	}
	defer connection.Close()
	decoder := json.NewDecoder(bufio.NewReader(connection))
	encoder := json.NewEncoder(connection)
	if err = encoder.Encode(map[string]any{"QMP": map[string]any{"version": map[string]any{}}}); err != nil {
		done <- err
		return
	}
	var request qmpRequest
	if err = decoder.Decode(&request); err == nil {
		err = encoder.Encode(qmpResponse{Return: json.RawMessage(`{}`), ID: request.ID})
	}
	if err == nil {
		err = decoder.Decode(&request)
	}
	if err == nil {
		err = encoder.Encode(qmpResponse{Return: json.RawMessage(`{"status":"running"}`), ID: request.ID})
	}
	done <- err
}

func TestQMPClientReportsNativeError(t *testing.T) {
	client := QMPClient{Socket: "ignored", Dial: errorQMPDialer}
	err := client.Execute(context.Background(), "system_powerdown", "powerdown", nil, nil)
	require.ErrorContains(t, err, "GenericError: rejected")
}

func errorQMPDialer(_ context.Context, _, _ string) (net.Conn, error) {
	client, server := net.Pipe()
	go func() {
		defer server.Close()
		decoder := json.NewDecoder(server)
		encoder := json.NewEncoder(server)
		_ = encoder.Encode(map[string]any{"QMP": map[string]any{}})
		var request qmpRequest
		_ = decoder.Decode(&request)
		_ = encoder.Encode(qmpResponse{Return: json.RawMessage(`{}`), ID: request.ID})
		_ = decoder.Decode(&request)
		_ = encoder.Encode(qmpResponse{Error: &qmpError{Class: "GenericError", Description: "rejected"}, ID: request.ID})
	}()
	return client, nil
}
