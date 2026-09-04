package acceptance

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type Remote struct {
	Username    string
	Host        string
	Port        int
	CockpitPort int
	Key         string
	KnownHosts  string
	Evidence    Evidence
}

func (remote Remote) WaitReady(ctx context.Context) error {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		if err := remote.refreshHostKey(ctx); err == nil {
			if _, err = remote.Output(ctx, nil, "id; cat /proc/sys/kernel/random/boot_id"); err == nil {
				return remote.waitCockpit(ctx)
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for guest SSH: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func (remote Remote) refreshHostKey(ctx context.Context) error {
	contents, err := CommandOutput(ctx, CommandSpec{Name: "ssh-keyscan", Args: []string{
		"-T", "5", "-t", "ed25519", "-p", strconv.Itoa(remote.Port), remote.Host,
	}})
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(contents)) == 0 {
		return errors.New("ssh-keyscan returned no host key")
	}
	return os.WriteFile(remote.KnownHosts, contents, 0o600)
}

func (remote Remote) waitCockpit(ctx context.Context) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := RunCommand(requestCtx, CommandSpec{Name: "curl", Args: []string{
			"--fail", "--silent", "--show-error", "--insecure", "https://" + urlHost(remote.Host) + ":" + strconv.Itoa(remote.CockpitPort) + "/ping",
		}, Stdout: io.Discard, Stderr: io.Discard})
		cancel()
		if err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for Cockpit: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func (remote Remote) Output(ctx context.Context, input []byte, command ...string) ([]byte, error) {
	var stdin io.Reader
	if input != nil {
		stdin = bytes.NewReader(input)
	}
	return CommandOutput(ctx, CommandSpec{Name: "ssh", Args: append(remote.sshArgs(), command...), Stdin: stdin})
}

func (remote Remote) Capture(ctx context.Context, relative string, input []byte, command ...string) error {
	_, err := remote.Exchange(ctx, relative, input, command...)
	return err
}

func (remote Remote) Exchange(ctx context.Context, relative string, input []byte, command ...string) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	var stdin io.Reader
	if input != nil {
		stdin = bytes.NewReader(input)
	}
	spec := CommandSpec{Name: "ssh", Args: append(remote.sshArgs(), command...), Stdin: stdin, Stdout: &stdout, Stderr: &stderr}
	runErr := RunCommand(ctx, spec)
	writeErr := errors.Join(remote.Evidence.Write(relative+".stdout", stdout.Bytes()), remote.Evidence.Write(relative+".stderr", stderr.Bytes()))
	return stdout.Bytes(), errors.Join(runErr, writeErr)
}

func (remote Remote) Sudo(ctx context.Context, password []byte, script string, relative string) error {
	input := make([]byte, 0, len(password)+len(script)+2)
	input = append(input, bytes.TrimRight(password, "\r\n")...)
	input = append(input, '\n')
	input = append(input, script...)
	return remote.Capture(ctx, relative, input, "sudo", "-k", "-S", "-p", "", "/bin/bash", "-eu", "-o", "pipefail", "-s")
}

func (remote Remote) sshArgs() []string {
	return []string{
		"-T", "-o", "BatchMode=yes", "-o", "IdentitiesOnly=yes", "-o", "StrictHostKeyChecking=yes",
		"-o", "UserKnownHostsFile=" + remote.KnownHosts, "-i", remote.Key, "-p", strconv.Itoa(remote.Port),
		remote.Username + "@" + remote.Host,
	}
}

func (remote Remote) As(username, key string) Remote {
	remote.Username = username
	remote.Key = key
	return remote
}

func urlHost(host string) string {
	if strings.Contains(host, ":") && net.ParseIP(host) != nil {
		return "[" + host + "]"
	}
	return host
}
