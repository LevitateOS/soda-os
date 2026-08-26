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
	if parsed.Host == "" {
		return fmt.Errorf("Git remote URL must include a host")
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		if parsed.User != nil {
			return fmt.Errorf("HTTP Git remote URL must not contain user information")
		}
	case "ssh":
		if parsed.User != nil {
			if parsed.User.Username() == "" {
				return fmt.Errorf("SSH Git remote URL contains empty user information")
			}
			if _, hasPassword := parsed.User.Password(); hasPassword {
				return fmt.Errorf("SSH Git remote URL must not contain a password")
			}
		}
	default:
		return fmt.Errorf("unsupported Git remote URL scheme %q", parsed.Scheme)
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
	hostPath := remote
	if at := strings.IndexByte(remote, '@'); at >= 0 {
		username := remote[:at]
		if username == "" || strings.ContainsAny(username, ":/") || strings.Contains(remote[at+1:], "@") {
			return false
		}
		hostPath = remote[at+1:]
	}

	if strings.HasPrefix(hostPath, "[") {
		closeBracket := strings.IndexByte(hostPath, ']')
		return closeBracket > 1 && len(hostPath) > closeBracket+2 && hostPath[closeBracket+1] == ':'
	}
	separator := strings.IndexByte(hostPath, ':')
	return separator > 0 && !strings.Contains(hostPath[:separator], "/") && len(hostPath) > separator+1
}
