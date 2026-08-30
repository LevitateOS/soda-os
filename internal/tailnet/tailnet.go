// Package tailnet reads Soda's appliance identity from the locally managed
// Tailscale node.
package tailnet

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/LevitateOS/soda-os/internal/process"
)

const DefaultCLI = "/usr/bin/tailscale"

var (
	ErrUnavailable         = errors.New("Tailscale status is unavailable")
	ErrNotEnrolled         = errors.New("Tailscale is not enrolled")
	ErrIdentityUnavailable = errors.New("Tailscale did not report a MagicDNS identity")
	ErrInvalidMagicDNSName = errors.New("invalid Tailscale MagicDNS identity")
)

// EnrollmentState reports whether Soda can use a Tailnet connection identity.
type EnrollmentState string

const (
	NeedsEnrollment     EnrollmentState = "needs-enrollment"
	IdentityUnavailable EnrollmentState = "identity-unavailable"
	Enrolled            EnrollmentState = "enrolled"
)

// Status is the stable Soda interpretation of `tailscale status --json`.
type Status struct {
	BackendState string
	Identity     string
}

// EnrollmentState returns Enrolled only when the local Tailscale node is
// running and has supplied a usable MagicDNS FQDN.
func (s Status) EnrollmentState() EnrollmentState {
	if s.BackendState != "Running" {
		return NeedsEnrollment
	}
	if s.Identity == "" {
		return IdentityUnavailable
	}
	return Enrolled
}

// Client queries the Tailscale CLI supplied by Soda's locked runtime package.
type Client struct {
	runner process.Runner
	cli    string
}

type Options struct {
	Runner process.Runner
	CLI    string
}

func New(options Options) *Client {
	if options.Runner == nil {
		options.Runner = process.OSRunner{}
	}
	if options.CLI == "" {
		options.CLI = DefaultCLI
	}
	return &Client{runner: options.Runner, cli: options.CLI}
}

// Status reads the local node's authoritative Tailscale status.
func (c *Client) Status(ctx context.Context) (Status, error) {
	output, err := c.runner.Output(ctx, process.Command{Name: c.cli, Args: []string{"status", "--json"}})
	if err != nil {
		return Status{}, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	status, err := parseStatus([]byte(output))
	if err != nil {
		return Status{}, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	return status, nil
}

// Identity returns the canonical MagicDNS FQDN only after enrollment.
func (c *Client) Identity(ctx context.Context) (string, error) {
	status, err := c.Status(ctx)
	if err != nil {
		return "", err
	}
	switch status.EnrollmentState() {
	case Enrolled:
		return status.Identity, nil
	case IdentityUnavailable:
		return "", ErrIdentityUnavailable
	default:
		return "", ErrNotEnrolled
	}
}

type statusDocument struct {
	BackendState string `json:"BackendState"`
	Self         struct {
		DNSName string `json:"DNSName"`
	} `json:"Self"`
}

func parseStatus(contents []byte) (Status, error) {
	var document statusDocument
	if err := json.Unmarshal(contents, &document); err != nil {
		return Status{}, fmt.Errorf("parse Tailscale status: %w", err)
	}
	if document.BackendState == "" {
		return Status{}, errors.New("Tailscale status did not include a backend state")
	}
	status := Status{BackendState: document.BackendState}
	if document.Self.DNSName == "" {
		return status, nil
	}
	identity, err := CanonicalMagicDNSName(document.Self.DNSName)
	if err != nil {
		return Status{}, err
	}
	status.Identity = identity
	return status, nil
}

// CanonicalMagicDNSName normalizes a Tailscale-supplied MagicDNS FQDN for use
// in connection instructions and certificate names.
func CanonicalMagicDNSName(value string) (string, error) {
	name := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(value), "."))
	if len(name) > 253 || !strings.Contains(name, ".") || strings.HasSuffix(name, ".local") {
		return "", ErrInvalidMagicDNSName
	}
	for _, label := range strings.Split(name, ".") {
		if !validLabel(label) {
			return "", ErrInvalidMagicDNSName
		}
	}
	return name, nil
}

func validLabel(label string) bool {
	switch {
	case len(label) == 0:
		return false
	case len(label) > 63:
		return false
	case label[0] == '-':
		return false
	case label[len(label)-1] == '-':
		return false
	}
	return strings.IndexFunc(label, invalidLabelCharacter) == -1
}

func invalidLabelCharacter(character rune) bool {
	switch {
	case character == '-':
		return false
	case character >= 'a' && character <= 'z':
		return false
	case character >= '0' && character <= '9':
		return false
	default:
		return true
	}
}
