// Package tailnet reads Soda's appliance identity from the locally managed
// Tailscale node.
package tailnet

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strings"

	"github.com/LevitateOS/soda-os/internal/process"
)

const DefaultCLI = "/usr/bin/tailscale"

var (
	ErrUnavailable         = errors.New("Tailscale status is unavailable")
	ErrNotEnrolled         = errors.New("Tailscale is not enrolled")
	ErrIdentityUnavailable = errors.New("Tailscale did not report a MagicDNS identity")
	ErrIPv4Unavailable     = errors.New("Tailscale did not report an IPv4 address")
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
	BackendState    string
	Identity        string
	IPv4            string
	MagicDNSEnabled bool
	Expired         bool
	AuthPending     bool
}

// Endpoint identifies an enrolled appliance by a resolvable name or IPv4 address.
type Endpoint struct {
	Identity string
	IPv4     string
}

// EnrollmentState returns Enrolled only when the local Tailscale node is
// running and has supplied a usable MagicDNS FQDN.
func (s Status) EnrollmentState() EnrollmentState {
	if s.BackendState != "Running" || s.Expired {
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

// URLHost returns a usable Tailnet host without assuming MagicDNS is enabled.
func (s Status) URLHost() string {
	if s.MagicDNSEnabled && s.Identity != "" {
		return s.Identity
	}
	return s.IPv4
}

// Endpoint returns the advertised host and the IPv4 address used by Forgejo.
func (c *Client) Endpoint(ctx context.Context) (Endpoint, error) {
	status, err := c.Status(ctx)
	if err != nil {
		return Endpoint{}, err
	}
	if status.BackendState != "Running" || status.Expired {
		return Endpoint{}, ErrNotEnrolled
	}
	if status.IPv4 == "" {
		return Endpoint{}, ErrIPv4Unavailable
	}
	return Endpoint{Identity: status.URLHost(), IPv4: status.IPv4}, nil
}

type statusDocument struct {
	BackendState   string `json:"BackendState"`
	AuthURL        string `json:"AuthURL"`
	CurrentTailnet struct {
		MagicDNSEnabled bool `json:"MagicDNSEnabled"`
	} `json:"CurrentTailnet"`
	Self struct {
		DNSName      string   `json:"DNSName"`
		Expired      bool     `json:"Expired"`
		TailscaleIPs []string `json:"TailscaleIPs"`
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
	status := Status{BackendState: document.BackendState, MagicDNSEnabled: document.CurrentTailnet.MagicDNSEnabled, Expired: document.Self.Expired, AuthPending: document.AuthURL != ""}
	if document.Self.DNSName != "" {
		identity, err := CanonicalMagicDNSName(document.Self.DNSName)
		if err != nil {
			return Status{}, err
		}
		status.Identity = identity
	}
	for _, rawAddress := range document.Self.TailscaleIPs {
		address, parseErr := netip.ParseAddr(rawAddress)
		if parseErr == nil && address.Is4() {
			status.IPv4 = address.String()
			break
		}
	}
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
