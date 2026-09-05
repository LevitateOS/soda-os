package workspace

import (
	"context"
	"path/filepath"

	"github.com/LevitateOS/soda-os/internal/linuxhost"
	"github.com/LevitateOS/soda-os/internal/projects/catalog"
)

// Inspection contains current Linux and local Git facts, not setup history.
// A problem means that fact could not be established; it is not an assertion
// that a file is absent. Inspection never creates an account, key, or checkout.
type Inspection struct {
	Username            string `json:"username"`
	Exists              bool   `json:"exists"`
	CheckoutPath        string `json:"checkout_path"`
	CheckoutReady       bool   `json:"checkout_ready"`
	PublicKey           string `json:"public_key"`
	PrimaryKeyProblem   string `json:"primary_key_problem"`
	WorkspaceKeyProblem string `json:"workspace_key_problem"`
	CheckoutProblem     string `json:"checkout_problem"`
	GitKeyProblem       string `json:"git_key_problem"`
}

func (accounts Accounts) Inspect(ctx context.Context, repository Repository, primary linuxhost.Account, entry catalog.Entry) (Inspection, error) {
	target, err := accounts.target(primary, entry)
	if err != nil {
		return Inspection{}, err
	}
	account, found, err := accounts.existing(ctx, target)
	if err != nil {
		return Inspection{}, err
	}
	username, _ := DerivedUsername(primary.Username, entry.ID)
	result := Inspection{Username: username, Exists: found}
	if _, err = accounts.keys.ReadAuthorizedKeys(primary); err != nil {
		result.PrimaryKeyProblem = err.Error()
	}
	if !found {
		return result, nil
	}
	result.CheckoutPath = filepath.Join(account.Home, "Projects", entry.ID)
	if _, err = accounts.keys.ReadAuthorizedKeys(account); err != nil {
		result.WorkspaceKeyProblem = err.Error()
	}
	result.CheckoutReady, err = repository.CloneExists(ctx, account, entry)
	if err != nil {
		result.CheckoutProblem = err.Error()
	}
	result.PublicKey, err = repository.ReadOutboundKey(ctx, account)
	if err != nil {
		result.GitKeyProblem = err.Error()
	}
	return result, nil
}
