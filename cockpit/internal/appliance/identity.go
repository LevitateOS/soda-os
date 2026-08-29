package appliance

import (
	"fmt"
	"strings"
)

// Identity is the installed appliance name used for trusted-LAN connections.
type Identity struct {
	Label   string
	Address string
}

// FromHostname derives Soda's mDNS address from one installed DNS label.
func FromHostname(hostname string) (Identity, error) {
	if !validLabel(hostname) {
		return Identity{}, fmt.Errorf("invalid appliance hostname %q: expected one DNS label", hostname)
	}
	label := strings.ToLower(hostname)
	return Identity{Label: label, Address: label + ".local"}, nil
}

func validLabel(value string) bool {
	if len(value) == 0 || len(value) > 63 {
		return false
	}
	for index := range len(value) {
		character := value[index]
		if character == '-' && index > 0 && index < len(value)-1 {
			continue
		}
		if !isAlphaNumeric(character) {
			return false
		}
	}
	return true
}

func isAlphaNumeric(character byte) bool {
	return (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9')
}
