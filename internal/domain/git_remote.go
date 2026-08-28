package domain

import (
	"fmt"
	"net/url"
	"strings"
	"unicode"
)

// ValidateGitRemoteURL enforces the credential-safe remote URL policy shared
// by daemon input validation and outbound Project projection.
func ValidateGitRemoteURL(remote string) error {
	if remote == "" {
		return fmt.Errorf("Git remote URL is required")
	}
	if strings.IndexFunc(remote, unicode.IsSpace) >= 0 {
		return fmt.Errorf("Git remote URL must not contain whitespace")
	}
	if isSCPLikeRemote(remote) {
		return nil
	}

	parsed, err := url.Parse(remote)
	if err != nil {
		return fmt.Errorf("parse Git remote URL: %w", err)
	}
	return validateGitURL(parsed)
}

func validateGitURL(parsed *url.URL) error {
	if parsed.Host == "" {
		return fmt.Errorf("Git remote URL must include a host")
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" && scheme != "ssh" {
		return fmt.Errorf("unsupported Git remote URL scheme %q", parsed.Scheme)
	}
	if parsed.User == nil {
		return nil
	}
	if scheme == "http" || scheme == "https" {
		return fmt.Errorf("HTTP Git remote URL must not contain user information")
	}
	if parsed.User.Username() == "" {
		return fmt.Errorf("SSH Git remote URL contains empty user information")
	}
	if _, hasPassword := parsed.User.Password(); hasPassword {
		return fmt.Errorf("SSH Git remote URL must not contain a password")
	}
	return nil
}

func ValidateProjectSource(source ProjectSource) error {
	switch source := source.(type) {
	case EmptyProjectSource:
		return nil
	case GitProjectSource:
		return ValidateGitRemoteURL(source.RemoteURL)
	default:
		return fmt.Errorf("project source must be empty or Git")
	}
}

func isSCPLikeRemote(remote string) bool {
	if strings.Contains(remote, "://") {
		return false
	}
	_, hostPath, hasUser := strings.Cut(remote, "@")
	if !hasUser {
		hostPath = remote
	}
	if hasUser && !validSCPUsername(remote, hostPath) {
		return false
	}
	return validSCPHostPath(hostPath)
}

func validSCPUsername(remote, hostPath string) bool {
	username := strings.TrimSuffix(remote, "@"+hostPath)
	return username != "" && !strings.ContainsAny(username, ":/") && !strings.Contains(hostPath, "@")
}

func validSCPHostPath(hostPath string) bool {
	if strings.HasPrefix(hostPath, "[") {
		closeBracket := strings.IndexByte(hostPath, ']')
		return closeBracket > 1 && len(hostPath) > closeBracket+2 && hostPath[closeBracket+1] == ':'
	}
	separator := strings.IndexByte(hostPath, ':')
	return separator > 0 && !strings.Contains(hostPath[:separator], "/") && len(hostPath) > separator+1
}
