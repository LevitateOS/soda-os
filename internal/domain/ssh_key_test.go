package domain

import "testing"

func TestSSHKeyFingerprintIgnoresComments(t *testing.T) {
	first, err := SSHKeyFingerprint("ssh-ed25519 AAAA alice@thin-client")
	if err != nil {
		t.Fatal(err)
	}
	second, err := SSHKeyFingerprint("ssh-ed25519 AAAA different-comment")
	if err != nil {
		t.Fatal(err)
	}
	if first != second || first == "" {
		t.Fatalf("fingerprints differ: %q != %q", first, second)
	}
}

func TestSSHKeyFingerprintRejectsMalformedMaterial(t *testing.T) {
	for _, key := range []string{"", "ssh-ed25519 !!!", "not-a-key AAAA"} {
		if _, err := SSHKeyFingerprint(key); err == nil {
			t.Fatalf("accepted %q", key)
		}
	}
}
