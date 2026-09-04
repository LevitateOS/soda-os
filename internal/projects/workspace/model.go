// Package workspace owns Soda's derived Linux workspace convention and the
// synchronous native operations that establish and remove those workspaces.
package workspace

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"github.com/LevitateOS/soda-os/internal/projects/catalog"
)

const (
	Group        = "soda-workspaces"
	Shell        = "/bin/bash"
	MarkerPrefix = "soda-workspace="
)

type Association struct {
	PrimaryUsername string
	ProjectID       string
}

// DerivedUsername returns the deterministic Linux account name for one
// human-project pair.
func DerivedUsername(primaryUsername, projectID string) (string, error) {
	if primaryUsername == "" || strings.ContainsAny(primaryUsername, "/\x00\r\n") {
		return "", errors.New("primary username cannot be represented in a workspace marker")
	}
	if err := validateProjectID(projectID); err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(primaryUsername + "\x00" + projectID))
	return "soda-w-" + fmt.Sprintf("%x", digest[:12]), nil
}

// Marker returns the Linux GECOS marker for one workspace association.
func Marker(primaryUsername, projectID string) (string, error) {
	if _, err := DerivedUsername(primaryUsername, projectID); err != nil {
		return "", err
	}
	return MarkerPrefix + primaryUsername + "/" + projectID, nil
}

// ParseMarker reads a workspace association from Linux-native GECOS evidence.
func ParseMarker(marker string) (primaryUsername, projectID string, err error) {
	association, found := strings.CutPrefix(marker, MarkerPrefix)
	if !found || strings.Count(association, "/") != 1 {
		return "", "", errors.New("invalid workspace account marker")
	}
	primaryUsername, projectID, _ = strings.Cut(association, "/")
	if _, err := DerivedUsername(primaryUsername, projectID); err != nil {
		return "", "", fmt.Errorf("invalid workspace account marker: %w", err)
	}
	return primaryUsername, projectID, nil
}

// ValidateAccount verifies that Linux evidence exactly represents the expected
// Soda workspace association.
func ValidateAccount(account linuxhost.Account, primaryUsername, projectID string, uidMin int) error {
	expectedUsername, err := DerivedUsername(primaryUsername, projectID)
	if err != nil {
		return err
	}
	expectedMarker, _ := Marker(primaryUsername, projectID)
	expectedHome := "/home/" + expectedUsername
	switch {
	case account.Username != expectedUsername:
		return errors.New("workspace account name does not match its association")
	case account.UID < uidMin:
		return errors.New("workspace account does not have a regular UID")
	case account.PrimaryGroup != expectedUsername:
		return errors.New("workspace account does not have its private primary group")
	case account.GECOS != expectedMarker:
		return errors.New("workspace account marker does not match its association")
	case account.Home != expectedHome:
		return errors.New("workspace account home does not match its association")
	case account.Shell != Shell:
		return errors.New("workspace account shell does not match the Soda convention")
	case !account.HasGroup(Group):
		return errors.New("workspace account is not in the workspace group")
	case account.IsAdministrator():
		return errors.New("workspace account must not be an administrator")
	}
	return nil
}

func validateProjectID(projectID string) error {
	return catalog.ValidateID(projectID)
}
