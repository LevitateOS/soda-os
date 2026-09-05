package linuxhost

import "context"

// AccountDeleter revalidates each selected account immediately before mutation.
type AccountDeleter interface {
	DeleteAccount(context.Context, Account) error
}

// DeletionResult is a receipt, not an inventory. A failed native command can
// already have terminated processes or removed some account/home data.
type DeletionResult struct {
	Removed      []string `json:"removed"`
	Uncertain    string   `json:"uncertain"`
	NotAttempted []string `json:"not_attempted"`
	Diagnostic   string   `json:"diagnostic"`
}

func DeleteAccounts(ctx context.Context, host AccountDeleter, targets []Account) DeletionResult {
	result := DeletionResult{Removed: []string{}, NotAttempted: []string{}}
	for index, account := range targets {
		if err := host.DeleteAccount(ctx, account); err != nil {
			result.Uncertain = account.Username
			result.Diagnostic = err.Error()
			for _, remaining := range targets[index+1:] {
				result.NotAttempted = append(result.NotAttempted, remaining.Username)
			}
			return result
		}
		result.Removed = append(result.Removed, account.Username)
	}
	return result
}
