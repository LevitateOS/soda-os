// Package runners implements Soda's focused local CI runner composition.
// Providers remain authoritative for registration, labels, workflows,
// scheduling, results, and history.
package runners

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"runtime"
	"strings"
)

const (
	DefaultRootPath   = "/var/lib/soda/runners"
	DefaultLockPath   = "/run/lock/soda/runners.lock"
	RunnerGroup       = "soda-runners"
	RunnerShell       = "/usr/sbin/nologin"
	RunnerCapacity    = 1
	BundledForgejoURL = "http://127.0.0.1:30000"

	ProviderForgejo Provider = "forgejo"
	ProviderGitHub  Provider = "github"
)

var (
	runnerIDPattern     = regexp.MustCompile(`^[a-z][a-z0-9-]{0,15}$`)
	forgejoUUIDPattern  = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	githubLabelPattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
	forgejoLabelPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}:host$`)
)

type Provider string

type CreateRequest struct {
	ID                string   `json:"id"`
	Provider          Provider `json:"provider"`
	RegistrationURL   string   `json:"registration_url"`
	RegistrationID    string   `json:"registration_id"`
	Labels            string   `json:"labels"`
	RegistrationToken string   `json:"registration_token"`
}

type RunnerRequest struct {
	ID string `json:"id"`
}

type EmptyRequest struct{}

type Descriptor struct {
	ID              string   `json:"id"`
	Provider        Provider `json:"provider"`
	RegistrationURL string   `json:"registration_url"`
	Account         string   `json:"account"`
	Architecture    string   `json:"architecture"`
}

type ServiceState struct {
	Load    string `json:"load"`
	Active  string `json:"active"`
	Sub     string `json:"sub"`
	Enabled string `json:"enabled"`
}

type RunnerView struct {
	Descriptor
	Version  string       `json:"version"`
	Capacity int          `json:"capacity"`
	Service  ServiceState `json:"service"`
}

type ListResponse struct {
	Runners         []RunnerView `json:"runners"`
	RunnerCount     int          `json:"runner_count"`
	ActiveListeners int          `json:"active_listeners"`
	TotalCapacity   int          `json:"total_capacity"`
}

type MutationResponse struct {
	OK bool `json:"ok"`
}

func (request CreateRequest) Validate() error {
	if err := ValidateID(request.ID); err != nil {
		return err
	}
	if err := validateRegistrationToken(request.RegistrationToken); err != nil {
		return err
	}
	switch request.Provider {
	case ProviderForgejo:
		return request.validateForgejo()
	case ProviderGitHub:
		return request.validateGitHub()
	default:
		return errors.New("provider must be forgejo or github")
	}
}

func validateRegistrationToken(token string) error {
	if token == "" || strings.IndexFunc(token, func(r rune) bool { return r == '\n' || r == '\r' || r == 0 }) >= 0 {
		return errors.New("registration token must be non-empty and contain no line breaks")
	}
	return nil
}

func (request CreateRequest) validateForgejo() error {
	if err := validateForgejoURL(request.RegistrationURL); err != nil {
		return fmt.Errorf("Forgejo URL: %w", err)
	}
	if !forgejoUUIDPattern.MatchString(request.RegistrationID) {
		return errors.New("Forgejo runner ID must be a lowercase UUID")
	}
	return requireLabels(request.Labels, forgejoLabelPattern, "Forgejo labels must use name:host syntax")
}

func (request CreateRequest) validateGitHub() error {
	if request.RegistrationID != "" {
		return errors.New("GitHub registration does not accept a Forgejo runner ID")
	}
	if err := validateGitHubURL(request.RegistrationURL); err != nil {
		return fmt.Errorf("GitHub URL: %w", err)
	}
	return validateLabels(request.Labels, githubLabelPattern, "GitHub labels must contain only letters, digits, dot, underscore, and hyphen")
}

func ValidateID(id string) error {
	if !runnerIDPattern.MatchString(id) {
		return errors.New("runner id must match [a-z][a-z0-9-]{0,15}")
	}
	return nil
}

func AccountName(id string) (string, error) {
	if err := ValidateID(id); err != nil {
		return "", err
	}
	return "soda-runner-" + id, nil
}

func NativeArchitecture() (string, error) {
	switch runtime.GOARCH {
	case "amd64":
		return "x86-64", nil
	case "arm64":
		return "AArch64", nil
	default:
		return "", fmt.Errorf("unsupported runner architecture %q", runtime.GOARCH)
	}
}

func requireLabels(labels string, pattern *regexp.Regexp, message string) error {
	if labels == "" {
		return errors.New(message)
	}
	return validateLabels(labels, pattern, message)
}

func validateLabels(labels string, pattern *regexp.Regexp, message string) error {
	if labels == "" {
		return nil
	}
	for _, label := range strings.Split(labels, ",") {
		if !pattern.MatchString(label) {
			return errors.New(message)
		}
	}
	return nil
}

func validateForgejoURL(raw string) error {
	parsed, err := validateProviderURL(raw)
	if err != nil {
		return err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("must use HTTP or HTTPS")
	}
	return nil
}

func validateGitHubURL(raw string) error {
	parsed, err := validateProviderURL(raw)
	if err != nil {
		return err
	}
	if parsed.Scheme != "https" || !strings.EqualFold(parsed.Hostname(), "github.com") || parsed.Port() != "" {
		return errors.New("must be an HTTPS github.com repository, organization, or enterprise URL")
	}
	if parsed.Path == "" || parsed.Path == "/" {
		return errors.New("must include a repository, organization, or enterprise path")
	}
	return nil
}

func validateProviderURL(raw string) (*url.URL, error) {
	if raw == "" || strings.IndexFunc(raw, func(r rune) bool { return r <= 0x20 || r == 0x7f }) >= 0 {
		return nil, errors.New("must be non-empty and contain no whitespace or control characters")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, errors.New("is not a valid URL")
	}
	if parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("must contain a host and no credentials, query, or fragment")
	}
	return parsed, nil
}
