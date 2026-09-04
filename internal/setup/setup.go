// Package setup handles temporary post-login network configuration through
// native NetworkManager, Tailscale, systemd, and Cockpit boundaries.
package setup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
)

type Administrator struct {
	Username string `json:"username"`
}

type Connection struct {
	Name                string `json:"name"`
	LocalNetworkAllowed bool   `json:"local_network_allowed"`
}

type Status struct {
	Ready               bool            `json:"ready"`
	Administrators      []Administrator `json:"administrators"`
	TailscaleConnected  bool            `json:"tailscale_connected"`
	LocalNetworkAllowed bool            `json:"local_network_allowed"`
	Connections         []Connection    `json:"connections"`
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
}

type Network interface {
	Status(context.Context) ([]Connection, bool, error)
	AllowLocalNetwork(context.Context, string) error
	ConnectTailscale(context.Context, string) error
}

type Locker interface {
	Lock() (io.Closer, error)
}

type Service struct {
	Accounts Accounts
	Network  Network
	Locker   Locker
}

func (service Service) Status(ctx context.Context) (Status, error) {
	administrators, err := service.Accounts.Administrators(ctx)
	if err != nil {
		return Status{}, fmt.Errorf("inspect Linux administrators: %w", err)
	}
	sort.Slice(administrators, func(i, j int) bool { return administrators[i].Username < administrators[j].Username })
	connections, tailscaleConnected, err := service.Network.Status(ctx)
	if err != nil {
		return Status{}, fmt.Errorf("inspect network access: %w", err)
	}
	status := Status{
		Administrators:     administrators,
		TailscaleConnected: tailscaleConnected,
		Connections:        connections,
	}
	status.LocalNetworkAllowed = anyLocalNetworkAllowed(connections)
	status.Ready = status.TailscaleConnected || status.LocalNetworkAllowed
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
