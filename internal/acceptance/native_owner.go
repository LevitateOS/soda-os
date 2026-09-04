package acceptance

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func (state *runnerState) forgejoPassword(username string, linuxPassword []byte) []byte {
	if username == state.options.Administrator.Username {
		return state.secret("forgejo-owner-password")
	}
	return linuxPassword
}

// Signup stays in Forgejo's native UI. The runner only verifies its outcome.
func (state *runnerState) verifyNativeOwner(ctx context.Context, remote Remote, address string) error {
	fmt.Fprintf(state.output, "Register the first Forgejo owner at %s/user/sign_up with username %q and the independent password in %s. Keep PAM active. Before teammates sign in, verify site administration is available, then press Enter here.\n", address, state.options.Administrator.Username, filepath.Join(state.paths.work, "forgejo-owner-password"))
	if _, err := bufio.NewReader(os.Stdin).ReadString('\n'); err != nil {
		return err
	}
	user, err := forgejoAuthenticatedUser(ctx, remote, remote.Username, state.secret("forgejo-owner-password"))
	if err != nil {
		return err
	}
	if user.Login != remote.Username || !user.IsAdmin {
		return errors.New("native first owner is not the expected Forgejo administrator")
	}
	if _, err = forgejoAuthenticatedUser(ctx, remote, remote.Username, state.secret("administrator-password")); err == nil {
		return errors.New("Forgejo owner unexpectedly accepts the independent Linux password")
	}
	return nil
}
