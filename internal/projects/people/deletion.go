package people

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"github.com/LevitateOS/soda-os/internal/projects/workspace"
)

type DeletionHost interface {
	LookupAccount(context.Context, string) (linuxhost.Account, error)
	CandidateAccounts(context.Context, string, string) ([]linuxhost.Account, error)
	workspace.DeletionHost
}

type ForgejoAccounts interface {
	DeleteUser(context.Context, string) error
}

type Deletion struct {
	Host    DeletionHost
	Forgejo ForgejoAccounts
}

func (deletion Deletion) Delete(ctx context.Context, actor linuxhost.Account, uidMin int, targetUsername string) error {
	target, err := deletion.authorizeTarget(ctx, actor, uidMin, targetUsername)
	if err != nil {
		return err
	}
	accounts, err := deletion.Host.CandidateAccounts(ctx, workspace.Group, workspace.MarkerPrefix)
	if err != nil {
		return err
	}
	if err = deletion.Host.PreflightDeleteAccount(ctx, target); err != nil {
		return err
	}
	workspaces, err := deletion.targets(ctx, accounts, targetUsername, uidMin)
	if err != nil {
		return err
	}
	removed := make([]string, 0, len(workspaces))
	for index, account := range workspaces {
		if err = deletion.Host.DeleteAccount(ctx, account); err != nil {
			return fmt.Errorf("%s; %s, Forgejo account, and primary Linux account remain: delete workspace: %w", removedWorkspaceDescription(removed), retainedWorkspaceDescription(workspaces[index:]), err)
		}
		removed = append(removed, account.Username)
	}
	if err = deletion.Forgejo.DeleteUser(ctx, target.Username); err != nil && !errors.Is(err, ErrForgejoUserNotFound) {
		return fmt.Errorf("%s; Forgejo account and primary Linux account %s remain: delete Forgejo account: %w", removedWorkspaceDescription(removed), target.Username, err)
	}
	if err = deletion.Host.DeleteAccount(ctx, target); err != nil {
		return fmt.Errorf("%s and Forgejo account %s; primary Linux account remains: %w", removedWorkspaceDescription(removed), target.Username, err)
	}
	return nil
}

func (deletion Deletion) authorizeTarget(ctx context.Context, actor linuxhost.Account, uidMin int, targetUsername string) (linuxhost.Account, error) {
	if !IsAdministrator(actor, uidMin) {
		return linuxhost.Account{}, errors.New("administrator status is required")
	}
	target, err := deletion.Host.LookupAccount(ctx, targetUsername)
	if err != nil {
		return linuxhost.Account{}, err
	}
	if !IsPrimary(target, uidMin) {
		return linuxhost.Account{}, errors.New("target is not a supported primary Linux account")
	}
	return target, nil
}

func (deletion Deletion) targets(ctx context.Context, accounts []linuxhost.Account, targetUsername string, uidMin int) ([]linuxhost.Account, error) {
	targets := []linuxhost.Account{}
	for _, account := range accounts {
		primary, projectID, err := workspace.ParseMarker(account.GECOS)
		if err != nil {
			return nil, err
		}
		if primary != targetUsername {
			continue
		}
		association := workspace.Association{PrimaryUsername: primary, ProjectID: projectID}
		if err = workspace.PreflightDeletion(ctx, deletion.Host, account, association, uidMin); err != nil {
			return nil, err
		}
		targets = append(targets, account)
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].Username < targets[j].Username })
	return targets, nil
}

func removedWorkspaceDescription(workspaces []string) string {
	if len(workspaces) == 0 {
		return "no Soda workspaces were removed"
	}
	return "removed Soda workspaces " + strings.Join(workspaces, ", ")
}

func retainedWorkspaceDescription(accounts []linuxhost.Account) string {
	names := make([]string, 0, len(accounts))
	for _, account := range accounts {
		names = append(names, account.Username)
	}
	if len(names) == 1 {
		return "workspace " + names[0]
	}
	return "workspaces " + strings.Join(names, ", ")
}
