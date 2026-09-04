package runners

import (
	"context"
	"errors"

	"github.com/LevitateOS/soda-os/internal/projects"
)

type Authorizer interface {
	RequireAdministrator(context.Context, string) error
}

type LinuxAuthorizer struct {
	Lifecycle projects.Lifecycle
}

func (authorizer LinuxAuthorizer) RequireAdministrator(ctx context.Context, username string) error {
	account, uidMin, err := authorizer.Lifecycle.AuthorizePrimary(ctx, username)
	if err != nil {
		return err
	}
	if !account.IsAdministrator(uidMin) {
		return errors.New("administrator status is required")
	}
	return nil
}
