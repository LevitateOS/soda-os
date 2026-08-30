package appliance

import (
	"fmt"
	"strings"

	"github.com/LevitateOS/soda-os/internal/tailnet"
)

// Identity is the installed appliance name used for trusted-LAN connections.
type Identity struct {
	Label        string
	Address      string
	LocalAddress string
}

// FromHostname derives Soda's mDNS address from one installed DNS label.
func FromHostname(hostname string) (Identity, error) {
	if !validLabel(hostname) {
		return Identity{}, fmt.Errorf("invalid appliance hostname %q: expected one DNS label", hostname)
	}
	label := strings.ToLower(hostname)
	localAddress := label + ".local"
	return Identity{Label: label, Address: localAddress, LocalAddress: localAddress}, nil
}

// FromTailnet makes Tailscale's MagicDNS name the appliance's normal
// connection identity while retaining the installed host label for local
// console access.
func FromTailnet(hostname, magicDNSName string) (Identity, error) {
	identity, err := FromHostname(hostname)
	if err != nil {
		return Identity{}, err
	}
	address, err := tailnet.CanonicalMagicDNSName(magicDNSName)
	if err != nil {
		return Identity{}, fmt.Errorf("invalid Tailnet appliance identity: %w", err)
	}
	identity.Address = address
	return identity, nil
}

// CertificateNames includes the normal Tailnet identity and local-console
// names that are already established for an installed appliance.
func (i Identity) CertificateNames() []string {
	if i.Address == i.LocalAddress {
		return []string{i.Address, i.Label}
	}
	return []string{i.Address, i.LocalAddress, i.Label}
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
