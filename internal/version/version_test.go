package version

import "testing"

func TestReleaseIdentity(t *testing.T) {
	if Version != "0.3.1" {
		t.Fatalf("Version = %q, want 0.3.1", Version)
	}
}
