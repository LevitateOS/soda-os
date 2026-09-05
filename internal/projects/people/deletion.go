package people

import (
	"context"
	"errors"
	"sort"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"github.com/LevitateOS/soda-os/internal/projects/workspace"
)

type DeletionHost interface {
	LookupAccount(context.Context, string) (linuxhost.Account, error)
	CandidateAccounts(context.Context, string, string) ([]linuxhost.Account, error)
	workspace.DeletionPreflight
}

type Deletion struct{ Host DeletionHost }

// Targets reads and preflights the complete selection, with the primary account
// last. An absent primary account does not authorize cascading orphan cleanup.
func (deletion Deletion) Targets(ctx context.Context, actor linuxhost.Account, uidMin int, targetUsername string) ([]linuxhost.Account, error) {
	target, err := deletion.authorizeTarget(ctx, actor, uidMin, targetUsername)
	if err != nil {
		return nil, err
	}
	accounts, err := deletion.Host.CandidateAccounts(ctx, workspace.Group, workspace.MarkerPrefix)
	if err != nil {
		return nil, err
	}
	if err = deletion.Host.PreflightDeleteAccount(ctx, target); err != nil {
		return nil, err
	}
	workspaces, err := deletion.targets(ctx, accounts, targetUsername, uidMin)
	if err != nil {
		return nil, err
	}
	return append(workspaces, target), nil
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
