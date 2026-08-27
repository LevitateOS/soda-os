package version

import "testing"

func TestReleaseIdentity(t *testing.T) {
	if Version != "0.3.0" {
		t.Fatalf("Version = %q, want 0.3.0", Version)
	}
}
