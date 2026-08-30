package domain

import (
	"fmt"
	"regexp"
	"time"
)

var unixIdentifier = regexp.MustCompile(`^[a-z][a-z0-9-]{0,23}$`)

func ValidateUnixIdentifier(value string) error {
	if unixIdentifier.MatchString(value) {
		return nil
	}
	return fmt.Errorf("must start with a lowercase letter and contain at most 24 lowercase letters, digits, or hyphens")
}

type Role string

const (
	RoleAdmin     Role = "admin"
	RoleDeveloper Role = "developer"
)

type Person struct {
	ID          string
	Username    string
	DisplayName string
	Email       string
	Role        Role
}

type GitIdentity struct {
	PersonID    string
	PublicKey   string
	Fingerprint string
}

type SSHDeviceKey struct {
	ID               string
	PersonID         string
	Label            string
	PublicKey        string
	Fingerprint      string
	IdentityFileHint string
	CreatedAt        time.Time
}
