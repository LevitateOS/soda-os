package acceptance

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
)

// Fixtures describe credentials and connections, not copied Linux account state.
// Linux remains authoritative when a later check changes roles or deletes accounts.
type personFixture struct {
	Remote          Remote
	PublicKey       []byte
	LinuxPassword   []byte
	ForgejoPassword []byte
}

type workspaceFixture struct {
	Person    personFixture
	Remote    Remote
	ProjectID string
}

// These are the three concrete workspaces created by seedPreservationState.
type projectFixture struct {
	Admin workspaceFixture
	Alice workspaceFixture
	Bob   workspaceFixture
}

type runInputs struct {
	Admin             personFixture
	Keys              fixtureKeys
	OwnerPasswordFile string
}

// fixtureKeys owns only generated personal SSH keys and their redaction registration.
type fixtureKeys struct {
	Directory string
	Secrets   *[]Secret
}

type personalKey struct {
	PrivatePath string
	Public      []byte
}

func (keys fixtureKeys) generate(ctx context.Context, username string) (personalKey, error) {
	path := filepath.Join(keys.Directory, username)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err = RunCommand(ctx, CommandSpec{Name: "ssh-keygen", Args: []string{"-q", "-t", "ed25519", "-N", "", "-C", username + "@soda-acceptance", "-f", path}}); err != nil {
			return personalKey{}, err
		}
	}
	private, err := os.ReadFile(path)
	if err != nil {
		return personalKey{}, err
	}
	*keys.Secrets = append(*keys.Secrets, Secret{Label: username + "-private-key", Value: private})
	public, err := os.ReadFile(path + ".pub")
	if err != nil {
		return personalKey{}, err
	}
	return personalKey{PrivatePath: path, Public: public}, nil
}

func addNativePerson(ctx context.Context, admin personFixture, username string, keys fixtureKeys, evidence string) (personFixture, error) {
	key, err := keys.generate(ctx, username)
	if err != nil {
		return personFixture{}, err
	}
	password := admin.LinuxPassword
	password64 := base64.StdEncoding.EncodeToString(bytes.TrimSpace(password))
	key64 := base64.StdEncoding.EncodeToString(bytes.TrimSpace(key.Public))
	script := fmt.Sprintf(`username=%q
/usr/sbin/useradd --create-home --user-group --shell /bin/bash --home-dir "/home/$username" -- "$username"
printf '%%s' %q | base64 --decode | /usr/bin/passwd --stdin -- "$username"
/usr/bin/install -d -m 0700 -o "$username" -g "$username" "/home/$username/.ssh"
printf '%%s' %q | base64 --decode >"/home/$username/.ssh/authorized_keys"
/usr/bin/chown "$username:$username" "/home/$username/.ssh/authorized_keys"
/usr/bin/chmod 0600 "/home/$username/.ssh/authorized_keys"
/usr/sbin/restorecon -RF "/home/$username/.ssh"
`, username, password64, key64)
	if err = admin.Remote.Sudo(ctx, password, script, evidence+"-linux"); err != nil {
		return personFixture{}, err
	}
	person := personFixture{Remote: admin.Remote.As(username, key.PrivatePath), PublicKey: key.Public, LinuxPassword: password, ForgejoPassword: password}
	user, err := forgejoAuthenticatedUser(ctx, person.Remote, username, person.ForgejoPassword)
	if err != nil {
		return personFixture{}, err
	}
	if user.Login != username || user.IsAdmin {
		return personFixture{}, fmt.Errorf("native Forgejo PAM created unexpected person %q", user.Login)
	}
	return person, nil
}
