package acceptance

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
)

// Signup stays in Forgejo's native UI. The runner only verifies its outcome.
func (state *runnerState) verifyNativeOwner(ctx context.Context, person personFixture, address, passwordPath string) error {
	fmt.Fprintf(state.output, "Register the first Forgejo owner at %s/user/sign_up with username %q and the independent password in %s. Keep PAM active. Before teammates sign in, verify site administration is available, then press Enter here.\n", address, person.Remote.Username, passwordPath)
	if _, err := bufio.NewReader(os.Stdin).ReadString('\n'); err != nil {
		return err
	}
	return verifyOwnerCredentials(ctx, person)
}

func verifyOwnerCredentials(ctx context.Context, person personFixture) error {
	remote := person.Remote
	user, err := forgejoAuthenticatedUser(ctx, remote, remote.Username, person.ForgejoPassword)
	if err != nil {
		return err
	}
	if user.Login != remote.Username || !user.IsAdmin {
		return errors.New("native first owner is not the expected Forgejo administrator")
	}
	result, err := requestForgejoUser(ctx, remote, remote.Username, person.LinuxPassword)
	if err != nil {
		return err
	}
	if result.Err == nil {
		return errors.New("Forgejo owner unexpectedly accepts the independent Linux password")
	}
	return nil
}
