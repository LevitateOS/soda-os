package projects

import (
	"errors"
	"strings"
)

// Validate rejects incomplete receipts rather than treating transport success
// as proof of completed removal.
func (response RemovalResponse) Validate() error {
	if response.Result.Removed == nil || response.Result.NotAttempted == nil {
		return errors.New("removal receipt is incomplete")
	}
	catalogStates := map[string]bool{"unchanged": true, "not_attempted": true, "removed": true, "uncertain": true}
	if !catalogStates[response.Catalog] {
		return errors.New("invalid catalog removal outcome")
	}
	problems := strings.Join([]string{response.Problem, response.Result.Diagnostic, response.Result.Uncertain}, "")
	if response.OK {
		if problems != "" || len(response.Result.NotAttempted) != 0 || response.Catalog == "uncertain" {
			return errors.New("successful removal receipt contains unresolved outcomes")
		}
	} else if problems == "" {
		return errors.New("failed removal receipt has no diagnostic")
	}
	return nil
}
