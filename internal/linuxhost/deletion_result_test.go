package linuxhost

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type partialAccountDeletion struct {
	accounts map[string]bool
	calls    []string
}

func (host *partialAccountDeletion) DeleteAccount(_ context.Context, account Account) error {
	host.calls = append(host.calls, account.Username)
	delete(host.accounts, account.Username)
	if account.Username == "bob" {
		return errors.New("home removal failed after account deletion")
	}
	return nil
}

func TestDeletionReceiptDoesNotCallAFailedAccountIntact(t *testing.T) {
	host := &partialAccountDeletion{accounts: map[string]bool{"alice": true, "bob": true, "carol": true}}
	result := DeleteAccounts(context.Background(), host, []Account{{Username: "alice"}, {Username: "bob"}, {Username: "carol"}})
	require.Equal(t, []string{"alice"}, result.Removed)
	require.Equal(t, "bob", result.Uncertain)
	require.Equal(t, []string{"carol"}, result.NotAttempted)
	require.NotContains(t, host.accounts, "bob")
	require.Contains(t, host.accounts, "carol")
	require.Equal(t, []string{"alice", "bob"}, host.calls)
}
