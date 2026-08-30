//go:build linux && cgo

package auth

import (
	"os"
	"testing"
)

func TestLivePAMAuthentication(t *testing.T) {
	username := os.Getenv("SODA_TEST_PAM_USER")
	password := os.Getenv("SODA_TEST_PAM_PASSWORD")
	if username == "" || password == "" {
		t.Skip("set SODA_TEST_PAM_USER and SODA_TEST_PAM_PASSWORD for a live PAM test")
	}
	authenticator := NewPAM()
	if _, err := authenticator.Authenticate(username, password); err != nil {
		t.Fatalf("authenticate valid Linux account: %v", err)
	}
	if _, err := authenticator.Authenticate(username, password+"-wrong"); err == nil {
		t.Fatal("PAM accepted an invalid password")
	}
}

func TestLivePAMPasswordlessAuthentication(t *testing.T) {
	username := os.Getenv("SODA_TEST_PAM_PASSWORDLESS_USER")
	if username == "" {
		t.Skip("set SODA_TEST_PAM_PASSWORDLESS_USER to a Linux account with an empty password")
	}
	authenticator := NewPAM()
	if _, err := authenticator.AuthenticatePasswordless(username); err != nil {
		t.Fatalf("authenticate passwordless Linux account: %v", err)
	}
	if _, err := authenticator.Authenticate(username, ""); err == nil {
		t.Fatal("password authentication accepted an empty authentication token")
	}
}
