package cert

import (
	"crypto/tls"
	"path/filepath"
	"testing"
)

func TestEnsureCreatesTLSKeyPair(t *testing.T) {
	directory := t.TempDir()
	certificate := filepath.Join(directory, "cockpit.crt")
	key := filepath.Join(directory, "cockpit.key")
	if err := Ensure(certificate, key); err != nil {
		t.Fatal(err)
	}
	if _, err := tls.LoadX509KeyPair(certificate, key); err != nil {
		t.Fatalf("load generated key pair: %v", err)
	}
}
