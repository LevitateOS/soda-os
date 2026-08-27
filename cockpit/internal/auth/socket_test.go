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
	for name, authenticator := range map[string]fixedAuthenticator{
		"accepted":                 {result: Authenticated},
		"password change required": {result: PasswordChangeRequired},
		"rejected":                 {authErr: errors.New("rejected")},
	} {
		t.Run(name, func(t *testing.T) {
			socket := shortSocket(t)
			listener, err := net.Listen("unix", socket)
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			go func() { _ = Serve(listener, authenticator) }()

			result, authErr := NewClient(socket).Authenticate("alice", "password")
			if authErr == nil && result != authenticator.result {
				t.Fatalf("authentication result = %q, want %q", result, authenticator.result)
			}
			if (authErr == nil) != (authenticator.authErr == nil) {
				t.Fatalf("unexpected authentication error: %v", authErr)
			}
		})
	}
}

func TestSocketClientPasswordChange(t *testing.T) {
	for name, changeError := range map[string]error{"accepted": nil, "rejected": errors.New("rejected")} {
		t.Run(name, func(t *testing.T) {
			socket := shortSocket(t)
			listener, err := net.Listen("unix", socket)
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			go func() { _ = Serve(listener, fixedAuthenticator{changeErr: changeError}) }()
			err = NewClient(socket).ChangePassword("alice", "temporary", "simple")
			if (err == nil) != (changeError == nil) {
				t.Fatalf("unexpected password change result: %v", err)
			}
		})
	}
}

func shortSocket(t *testing.T) string {
	t.Helper()
	directory, err := os.MkdirTemp("/tmp", "soda-auth-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(directory) })
	return filepath.Join(directory, "pam.sock")
}
