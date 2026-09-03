package version

import "testing"

func TestReleaseIdentity(t *testing.T) {
	if Version != "development" {
		t.Fatalf("Version = %q, want development", Version)
	}
}
