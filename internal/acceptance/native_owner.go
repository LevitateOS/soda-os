package acceptance

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
)

// Signup stays in Forgejo's native UI. The runner only verifies its outcome.
func awaitNativeOwnerSignup(ctx context.Context, person personFixture, address, passwordPath string, output io.Writer) error {
	fmt.Fprintf(output, "Register the first Forgejo owner at %s/user/sign_up with username %q and the independent password in %s. Keep PAM active. Before teammates sign in, verify site administration is available, then press Enter here.\n", address, person.Remote.Username, passwordPath)
	// Own this descriptor so cancellation can interrupt the read without closing
	// the caller's os.Stdin or leaving an unbounded reader goroutine behind.
	input, err := os.Open("/dev/stdin")
	if err != nil {
		return err
	}
	return awaitEnter(ctx, input)
}

func awaitEnter(ctx context.Context, input io.ReadCloser) error {
	defer input.Close()
	done := make(chan error, 1)
	go func() {
		_, err := bufio.NewReader(input).ReadString('\n')
		done <- err
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return errors.Join(ctx.Err(), err)
	}
}

func verifyOwnerCredentials(ctx context.Context, person personFixture, evidence string) error {
	remote := person.Remote
	user, err := forgejoAuthenticatedUser(ctx, person, evidence+"-forgejo-password")
	if err != nil {
		return err
	}
	if user.Login != remote.Username || !user.IsAdmin {
		return errors.New("native first owner is not the expected Forgejo administrator")
	}
	// Do not use curl --fail: HTTP 401 is the observation, not just an exit code.
	config := fmt.Sprintf("user = %s\nsilent\nshow-error\nmax-time = 15\noutput = \"/dev/null\"\nwrite-out = \"%%{http_code}\"\nurl = %s\n", curlConfigQuote(remote.Username+":"+string(bytes.TrimRight(person.LinuxPassword, "\r\n"))), curlConfigQuote(forgejoLoopbackEndpoint+"/api/v1/user"))
	status, err := remote.CaptureOutput(ctx, evidence+"-linux-password-rejected", []byte(config), "curl", "--config", "-")
	if err != nil {
		return err
	}
	if string(status) != "401" {
		return fmt.Errorf("Forgejo owner must reject the independent Linux password with HTTP 401, got %q", status)
	}
	return nil
}
