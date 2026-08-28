package domain

import "testing"

func TestParseSSHKeyNormalizesAndIgnoresComments(t *testing.T) {
	firstKey, first, err := ParseSSHKey("ssh-ed25519 AAAA alice@thin-client")
	if err != nil {
		t.Fatal(err)
	}
	secondKey, second, err := ParseSSHKey("ssh-ed25519 AAAA different-comment")
	if err != nil {
		t.Fatal(err)
	}
	if firstKey != "ssh-ed25519 AAAA" || firstKey != secondKey || first != second || first == "" {
		t.Fatalf("fingerprints differ: %q != %q", first, second)
	}
}

func TestParseSSHKeyRejectsMalformedMaterial(t *testing.T) {
	for _, key := range []string{"", "ssh-ed25519 !!!", "not-a-key AAAA", "ssh-ed25519 key\ncommand", "ecdsa-sha2-nistp256 AAAA"} {
		if _, _, err := ParseSSHKey(key); err == nil {
			t.Fatalf("accepted %q", key)
		}
	}
}
