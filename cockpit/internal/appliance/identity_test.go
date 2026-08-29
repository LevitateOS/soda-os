package appliance

import "testing"

func TestFromHostname(t *testing.T) {
	for name, test := range map[string]struct {
		hostname string
		want     Identity
	}{
		"default":         {hostname: "soda", want: Identity{Label: "soda", Address: "soda.local"}},
		"normalizes case": {hostname: "Atlas", want: Identity{Label: "atlas", Address: "atlas.local"}},
		"allows hyphen":   {hostname: "atlas-1", want: Identity{Label: "atlas-1", Address: "atlas-1.local"}},
	} {
		t.Run(name, func(t *testing.T) {
			actual, err := FromHostname(test.hostname)
			if err != nil || actual != test.want {
				t.Fatalf("FromHostname(%q) = %#v, %v; want %#v, nil", test.hostname, actual, err, test.want)
			}
		})
	}
}

func TestFromHostnameRejectsInvalidLabels(t *testing.T) {
	for _, hostname := range []string{"atlas.local", "-atlas", "atlas-", "atlas_name", ""} {
		if _, err := FromHostname(hostname); err == nil {
			t.Errorf("FromHostname(%q) succeeded; want validation error", hostname)
		}
	}
}
