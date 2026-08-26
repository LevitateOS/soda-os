package auth

import (
	"errors"
	"net"
	"path/filepath"
	"testing"
)

type fixedAuthenticator struct {
	err error
}

func (a fixedAuthenticator) Authenticate(_, _ string) error {
	return a.err
}

func TestSocketClient(t *testing.T) {
	for name, authError := range map[string]error{
		"accepted": nil,
		"rejected": errors.New("rejected"),
	} {
		t.Run(name, func(t *testing.T) {
			socket := filepath.Join(t.TempDir(), "pam.sock")
			listener, err := net.Listen("unix", socket)
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			go func() { _ = Serve(listener, fixedAuthenticator{err: authError}) }()

			err = NewClient(socket).Authenticate("alice", "password")
			if (err == nil) != (authError == nil) {
				t.Fatalf("unexpected authentication result: %v", err)
			}
		})
	}
}
