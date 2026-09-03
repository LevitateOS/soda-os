package version

import "testing"

func TestReleaseIdentity(t *testing.T) {
	if Version != "0.5.0" {
		t.Fatalf("Version = %q, want 0.5.0", Version)
	}
}
