// Package projects implements Soda's minimal project catalog and synchronous
// Linux workspace lifecycle. It deliberately has no daemon, database, RPC, or
// durable operation state.
package projects

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	DefaultCatalogPath = "/var/lib/soda/catalog/projects.json"
	DefaultLockPath    = "/run/lock/soda/projects.lock"
	WorkspaceGroup     = "soda-workspaces"
	WorkspaceShell     = "/bin/bash"
	workspaceMarkerKey = "soda-workspace="
)

var projectIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,23}$`)

func ValidatePrimaryUsername(username string) error {
	if !projectIDPattern.MatchString(username) {
		return errors.New("username must match [a-z][a-z0-9-]{0,23}")
	}
	return nil
}

// CatalogEntry is the complete durable Soda project representation.
type CatalogEntry struct {
	ID           string                     `json:"id"`
	DisplayName  string                     `json:"display_name"`
	CanonicalURL string                     `json:"canonical_url"`
	Additional   map[string]json.RawMessage `json:"-"`
}

func (entry CatalogEntry) Validate() error {
	if err := validateCatalogRequiredFields(entry); err != nil {
		return err
	}
	return validateCatalogAdditionalFields(entry.Additional)
}

func validateCatalogRequiredFields(entry CatalogEntry) error {
	if !projectIDPattern.MatchString(entry.ID) {
		return errors.New("project id must match [a-z][a-z0-9-]{0,23}")
	}
	if entry.DisplayName == "" || !utf8.ValidString(entry.DisplayName) {
		return errors.New("display name must be non-empty UTF-8")
	}
	if strings.IndexFunc(entry.DisplayName, unicode.IsControl) >= 0 {
		return errors.New("display name must not contain control characters")
	}
	if err := ValidateCanonicalURL(entry.CanonicalURL); err != nil {
		return fmt.Errorf("canonical URL: %w", err)
	}
	return nil
}

func validateCatalogAdditionalFields(additional map[string]json.RawMessage) error {
	for field, value := range additional {
		if field == "" || !utf8.ValidString(field) {
			return errors.New("additional catalog field names must be non-empty UTF-8")
		}
		if field == "id" || field == "display_name" || field == "canonical_url" {
			return fmt.Errorf("additional catalog field %q conflicts with a required field", field)
		}
		if !json.Valid(value) {
			return fmt.Errorf("additional catalog field %q must contain valid JSON", field)
		}
	}
	return nil
}

func (entry CatalogEntry) jsonObject() map[string]json.RawMessage {
	object := make(map[string]json.RawMessage, len(entry.Additional)+3)
	for field, value := range entry.Additional {
		object[field] = append(json.RawMessage(nil), value...)
	}
	object["id"], _ = json.Marshal(entry.ID)
	object["display_name"], _ = json.Marshal(entry.DisplayName)
	object["canonical_url"], _ = json.Marshal(entry.CanonicalURL)
	return object
}

func ValidateCanonicalURL(remote string) error {
	if err := validateRemoteText(remote); err != nil {
		return err
	}
	if validSCPLikeRemote(remote) {
		return nil
	}
	parsed, err := url.Parse(remote)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	return validateStructuredRemote(parsed)
}

func sameHost(left, right string) bool {
	return normalizeHost(left) != "" && normalizeHost(left) == normalizeHost(right)
}

func normalizeHost(value string) string {
	return strings.TrimSuffix(strings.ToLower(value), ".")
}

func sshRemoteHost(remote string) (string, error) {
	if err := ValidateCanonicalURL(remote); err != nil {
		return "", err
	}
	if validSCPLikeRemote(remote) {
		_, hostPath, found := strings.Cut(remote, "@")
		if !found {
			hostPath = remote
		}
		if strings.HasPrefix(hostPath, "[") {
			closing := strings.IndexByte(hostPath, ']')
			return hostPath[1:closing], nil
		}
		host, _, _ := strings.Cut(hostPath, ":")
		return host, nil
	}
	parsed, err := url.Parse(remote)
	if err != nil {
		return "", fmt.Errorf("parse: %w", err)
	}
	return parsed.Hostname(), nil
}

func ValidateToolSelections(tools []string) error {
	seen := map[string]bool{}
	for _, tool := range tools {
		if tool == "" || strings.HasPrefix(tool, "-") || !utf8.ValidString(tool) || strings.IndexFunc(tool, unicode.IsSpace) >= 0 || strings.IndexFunc(tool, unicode.IsControl) >= 0 {
			return fmt.Errorf("invalid mise tool selection %q", tool)
		}
		if seen[tool] {
			return fmt.Errorf("duplicate mise tool selection %q", tool)
		}
		seen[tool] = true
	}
	return nil
}

func validateRemoteText(remote string) error {
	switch {
	case remote == "":
		return errors.New("is required")
	case strings.HasPrefix(strings.ToLower(remote), "file:"):
		return errors.New("must not use a file URL or local file syntax")
	case driveLetterPath(remote):
		return errors.New("must not use a local drive path")
	case strings.ContainsAny(remote, "?#"):
		return errors.New("must not contain a query or fragment")
	case strings.IndexFunc(remote, unicode.IsSpace) >= 0:
		return errors.New("must not contain whitespace")
	case strings.IndexFunc(remote, unicode.IsControl) >= 0:
		return errors.New("must not contain control characters")
	default:
		return nil
	}
}

func driveLetterPath(remote string) bool {
	if len(remote) < 3 || remote[1] != ':' {
		return false
	}
	letter := remote[0]
	return ((letter >= 'a' && letter <= 'z') || (letter >= 'A' && letter <= 'Z')) && (remote[2] == '/' || remote[2] == '\\')
}

func validateStructuredRemote(parsed *url.URL) error {
	if parsed.Host == "" || parsed.Path == "" || parsed.Path == "/" {
		return errors.New("must include a host and repository path")
	}
	switch strings.ToLower(parsed.Scheme) {
	case "ssh":
		return validateSSHRemote(parsed)
	default:
		return errors.New("must use SSH or SCP syntax")
	}
}

func validateSSHRemote(parsed *url.URL) error {
	if parsed.User == nil {
		return nil
	}
	if parsed.User.Username() == "" {
		return errors.New("SSH URL must not contain an empty user")
	}
	if _, present := parsed.User.Password(); present {
		return errors.New("must not contain a password")
	}
	return nil
}

func validSCPLikeRemote(remote string) bool {
	if strings.Contains(remote, "://") || strings.ContainsAny(remote, "?#") {
		return false
	}
	user, hostPath, found := strings.Cut(remote, "@")
	if !found {
		hostPath = user
	} else if !validSCPUser(user) || strings.Contains(hostPath, "@") {
		return false
	}
	return validSCPHostPath(hostPath)
}

func validSCPUser(user string) bool {
	return user != "" && !strings.ContainsAny(user, "/:@")
}

func validSCPHostPath(hostPath string) bool {
	if strings.HasPrefix(hostPath, "[") {
		return validBracketedSCPHostPath(hostPath)
	}
	host, path, found := strings.Cut(hostPath, ":")
	return found && host != "" && path != "" && !strings.ContainsAny(host, "/@")
}

func validBracketedSCPHostPath(hostPath string) bool {
	closing := strings.IndexByte(hostPath, ']')
	return closing > 1 && len(hostPath) > closing+2 && hostPath[closing+1] == ':' && hostPath[closing+2:] != ""
}

// DerivedUsername returns the deterministic Linux workspace account name.
func DerivedUsername(primaryUsername, projectID string) (string, error) {
	if !projectIDPattern.MatchString(primaryUsername) {
		return "", errors.New("primary username must match [a-z][a-z0-9-]{0,23}")
	}
	if !projectIDPattern.MatchString(projectID) {
		return "", errors.New("project id must match [a-z][a-z0-9-]{0,23}")
	}
	digest := sha256.Sum256([]byte(primaryUsername + "\x00" + projectID))
	return "soda-w-" + fmt.Sprintf("%x", digest[:12]), nil
}

func WorkspaceMarker(primaryUsername, projectID string) (string, error) {
	if _, err := DerivedUsername(primaryUsername, projectID); err != nil {
		return "", err
	}
	return workspaceMarkerKey + primaryUsername + "/" + projectID, nil
}

func ParseWorkspaceMarker(marker string) (primaryUsername, projectID string, err error) {
	association, found := strings.CutPrefix(marker, workspaceMarkerKey)
	if !found || strings.Count(association, "/") != 1 {
		return "", "", errors.New("invalid workspace account marker")
	}
	primaryUsername, projectID, _ = strings.Cut(association, "/")
	if _, err := DerivedUsername(primaryUsername, projectID); err != nil {
		return "", "", fmt.Errorf("invalid workspace account marker: %w", err)
	}
	return primaryUsername, projectID, nil
}

// Account is the Linux-native account evidence required by Soda operations.
type Account struct {
	Username     string
	UID          int
	GID          int
	PrimaryGroup string
	GECOS        string
	Home         string
	Shell        string
	Groups       map[string]bool
}

func (account Account) IsPrimary(uidMin int) bool {
	return account.UID >= uidMin && projectIDPattern.MatchString(account.Username) && interactiveShell(account.Shell) && !account.Groups[WorkspaceGroup]
}

func (account Account) IsAdministrator(uidMin int) bool {
	return account.IsPrimary(uidMin) && account.Groups["wheel"]
}

func (account Account) ValidateWorkspace(primaryUsername, projectID string, uidMin int) error {
	expectedUsername, err := DerivedUsername(primaryUsername, projectID)
	if err != nil {
		return err
	}
	expectedMarker, _ := WorkspaceMarker(primaryUsername, projectID)
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
	case account.Shell != WorkspaceShell:
		return errors.New("workspace account shell does not match the Soda convention")
	case !account.Groups[WorkspaceGroup]:
		return errors.New("workspace account is not in the workspace group")
	case account.Groups["wheel"]:
		return errors.New("workspace account must not be an administrator")
	}
	return nil
}

func interactiveShell(shell string) bool {
	if shell == "" {
		return false
	}
	base := shell[strings.LastIndex(shell, "/")+1:]
	return base != "false" && base != "nologin"
}

func parseInt(field, value string) (int, error) {
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid %s", field)
	}
	return n, nil
}
