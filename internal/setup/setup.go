// Package setup composes Soda's one interactive post-install setup from native
// Linux, Forgejo, NetworkManager, Tailscale, systemd, and Cockpit boundaries.
package setup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/LevitateOS/soda-os/internal/projects"
)

const DefaultCompletionPath = "/var/lib/soda/setup-complete"

type Administrator struct {
	Username     string `json:"username"`
	PasswordSet  bool   `json:"password_set"`
	SSHPublicKey bool   `json:"ssh_public_key"`
	ForgejoReady bool   `json:"forgejo_ready"`
}

func (administrator Administrator) complete() bool {
	return administrator.Username != "" && administrator.PasswordSet && administrator.SSHPublicKey && administrator.ForgejoReady
}

type Connection struct {
	Name                string `json:"name"`
	LocalNetworkAllowed bool   `json:"local_network_allowed"`
}

type Status struct {
	Dismissed           bool            `json:"dismissed"`
	CanDismiss          bool            `json:"can_dismiss"`
	Administrators      []Administrator `json:"administrators"`
	TailscaleConnected  bool            `json:"tailscale_connected"`
	LocalNetworkAllowed bool            `json:"local_network_allowed"`
	Connections         []Connection    `json:"connections"`
}

type AdministratorRequest struct {
	Username      string `json:"username"`
	Password      string `json:"password"`
	AuthorizedKey string `json:"authorized_key"`
}

type LocalNetworkRequest struct {
	Connection string `json:"connection"`
}

type TailscaleRequest struct {
	AuthKey string `json:"auth_key"`
}

type Response struct {
	Status Status `json:"status"`
	Error  string `json:"error,omitempty"`
}

type Accounts interface {
	Administrators(context.Context) ([]Administrator, error)
	Prepare(context.Context, AdministratorRequest) error
	Promote(context.Context, string) error
}

type Forgejo interface {
	Ready(context.Context, string) bool
	PrepareAdministrator(context.Context, AdministratorRequest) error
}

type Network interface {
	Status(context.Context) ([]Connection, bool, error)
	AllowLocalNetwork(context.Context, string) error
	ConnectTailscale(context.Context, string) error
}

type Completion interface {
	Dismissed() (bool, error)
	Mark() error
}

type Locker interface {
	Lock() (io.Closer, error)
}

type Service struct {
	Accounts   Accounts
	Forgejo    Forgejo
	Network    Network
	Completion Completion
	Locker     Locker
}

func (service Service) Status(ctx context.Context) (Status, error) {
	administrators, err := service.Accounts.Administrators(ctx)
	if err != nil {
		return Status{}, fmt.Errorf("inspect Linux administrators: %w", err)
	}
	for index := range administrators {
		administrators[index].ForgejoReady = service.Forgejo.Ready(ctx, administrators[index].Username)
	}
	sort.Slice(administrators, func(i, j int) bool { return administrators[i].Username < administrators[j].Username })
	connections, tailscaleConnected, err := service.Network.Status(ctx)
	if err != nil {
		return Status{}, fmt.Errorf("inspect network access: %w", err)
	}
	dismissed, err := service.Completion.Dismissed()
	if err != nil {
		return Status{}, fmt.Errorf("inspect Soda Setup dismissal: %w", err)
	}
	status := Status{
		Dismissed:          dismissed,
		Administrators:     administrators,
		TailscaleConnected: tailscaleConnected,
		Connections:        connections,
	}
	status.LocalNetworkAllowed = anyLocalNetworkAllowed(connections)
	status.CanDismiss = anyCompleteAdministrator(administrators) && (status.TailscaleConnected || status.LocalNetworkAllowed)
	return status, nil
}

func anyLocalNetworkAllowed(connections []Connection) bool {
	for _, connection := range connections {
		if connection.LocalNetworkAllowed {
			return true
		}
	}
	return false
}

func anyCompleteAdministrator(administrators []Administrator) bool {
	for _, administrator := range administrators {
		if administrator.complete() {
			return true
		}
	}
	return false
}

func (service Service) CreateAdministrator(ctx context.Context, request AdministratorRequest) (Status, error) {
	unlock, err := service.lock()
	if err != nil {
		return Status{}, err
	}
	defer unlock.Close()
	status, err := service.Status(ctx)
	if err != nil {
		return Status{}, err
	}
	if len(status.Administrators) != 0 {
		return status, errors.New("an ordinary Linux administrator already exists")
	}
	key, err := projects.CanonicalAuthorizedKey(request.AuthorizedKey)
	if err != nil {
		return status, err
	}
	request.AuthorizedKey = key
	if err = service.Accounts.Prepare(ctx, request); err == nil {
		err = service.Forgejo.PrepareAdministrator(ctx, request)
	}
	if err == nil {
		err = service.Accounts.Promote(ctx, request.Username)
	}
	return service.statusAfter(ctx, err)
}

func (service Service) AllowLocalNetwork(ctx context.Context, connection string) (Status, error) {
	unlock, err := service.lock()
	if err != nil {
		return Status{}, err
	}
	defer unlock.Close()
	err = service.Network.AllowLocalNetwork(ctx, connection)
	return service.statusAfter(ctx, err)
}

func (service Service) ConnectTailscale(ctx context.Context, authKey string) (Status, error) {
	unlock, err := service.lock()
	if err != nil {
		return Status{}, err
	}
	defer unlock.Close()
	err = service.Network.ConnectTailscale(ctx, authKey)
	return service.statusAfter(ctx, err)
}

func (service Service) Dismiss(ctx context.Context) (Status, error) {
	unlock, err := service.lock()
	if err != nil {
		return Status{}, err
	}
	defer unlock.Close()
	status, err := service.Status(ctx)
	if err != nil {
		return Status{}, err
	}
	if !status.CanDismiss {
		return status, errors.New("Soda Setup cannot be dismissed until every required fact is complete")
	}
	if err = service.Completion.Mark(); err != nil {
		return status, fmt.Errorf("record Soda Setup dismissal: %w", err)
	}
	return service.Status(ctx)
}

func (service Service) lock() (io.Closer, error) {
	if service.Locker == nil {
		return io.NopCloser(nil), nil
	}
	lock, err := service.Locker.Lock()
	if err != nil {
		return nil, fmt.Errorf("lock Soda Setup operation: %w", err)
	}
	return lock, nil
}

func (service Service) statusAfter(ctx context.Context, operationErr error) (Status, error) {
	status, statusErr := service.Status(ctx)
	return status, errors.Join(operationErr, statusErr)
}
