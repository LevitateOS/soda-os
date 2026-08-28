package auth

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
)

type fixedAuthenticator struct {
	result    Result
	authErr   error
	changeErr error
}

func (a fixedAuthenticator) Authenticate(_, _ string) (Result, error) {
	return a.result, a.authErr
}

func (a fixedAuthenticator) ChangePassword(_, _, _ string) error {
	return a.changeErr
}

func TestSocketClientAuthenticationResults(t *testing.T) {
	tests := []struct {
		name          string
		authenticator fixedAuthenticator
		want          Result
		wantError     bool
	}{
		{name: "authenticated", authenticator: fixedAuthenticator{result: Authenticated}, want: Authenticated},
		{name: "password change required", authenticator: fixedAuthenticator{result: PasswordChangeRequired}, want: PasswordChangeRequired},
		{name: "invalid credentials", authenticator: fixedAuthenticator{authErr: errors.New("rejected")}, wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := socketClient(t, test.authenticator).Authenticate("alice", "password")
			if (err != nil) != test.wantError || result != test.want {
				t.Fatalf("authentication result = %q, %v", result, err)
			}
		})
	}
}

func TestSocketClientPasswordChange(t *testing.T) {
	for name, changeError := range map[string]error{"accepted": nil, "rejected": errors.New("rejected")} {
		t.Run(name, func(t *testing.T) {
			err := socketClient(t, fixedAuthenticator{changeErr: changeError}).ChangePassword("alice", "temporary", "simple")
			if (err == nil) != (changeError == nil) {
				t.Fatalf("unexpected password change result: %v", err)
			}
		})
	}
}

func socketClient(t *testing.T, authenticator fixedAuthenticator) Client {
	t.Helper()
	directory, err := os.MkdirTemp("/tmp", "soda-auth-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(directory) })
	listener, err := net.Listen("unix", filepath.Join(directory, "pam.sock"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() { _ = Serve(listener, authenticator) }()
	return NewClient(listener.Addr().String())
}
