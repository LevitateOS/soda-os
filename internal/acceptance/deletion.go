package acceptance

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// A pre-deletion observation identifies the exact home and UID to check after
// Linux removes the account. It is not an account-existence or role cache.
type deletionTarget struct {
	Username string
	UID      int
	Home     string
}

func observeDeletionTarget(ctx context.Context, admin personFixture, username, evidence string) (deletionTarget, error) {
	output, err := admin.Remote.SudoOutput(ctx, admin.LinuxPassword, remoteCommand([]string{"getent", "passwd", username})+"\n", evidence)
	if err != nil {
		return deletionTarget{}, err
	}
	fields := strings.Split(strings.TrimSpace(string(output)), ":")
	if len(fields) != 7 || fields[0] != username || !strings.HasPrefix(fields[5], "/home/") {
		return deletionTarget{}, errors.New("deletion target is not the expected native account and home")
	}
	uid, err := strconv.Atoi(fields[2])
	if err != nil || uid < 1000 {
		return deletionTarget{}, errors.New("deletion target has no regular Linux UID")
	}
	return deletionTarget{Username: username, UID: uid, Home: fields[5]}, nil
}

func verifyAccountRemoved(ctx context.Context, admin personFixture, target deletionTarget, evidence string) error {
	script := fmt.Sprintf("set -- %s\n", remoteCommand([]string{target.Username, strconv.Itoa(target.UID), target.Home})) + accountRemovedScript
	return admin.Remote.Sudo(ctx, admin.LinuxPassword, script, evidence)
}

const accountRemovedScript = `if getent passwd "$1" >/dev/null; then exit 1; else test "$?" -eq 2; fi
test ! -e "$3"
test ! -L "$3"
if pgrep -u "$2"; then exit 1; else test "$?" -eq 1; fi
`
