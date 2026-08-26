//go:build linux && cgo

package auth

import (
	"fmt"

	pam "github.com/msteinert/pam/v2"
)

func (PAM) Authenticate(username, password string) error {
	transaction, err := pam.StartFunc("soda-cockpit", username, func(style pam.Style, _ string) (string, error) {
		switch style {
		case pam.PromptEchoOff:
			return password, nil
		case pam.PromptEchoOn:
			return username, nil
		case pam.ErrorMsg, pam.TextInfo:
			return "", nil
		default:
			return "", fmt.Errorf("unsupported PAM conversation style %d", style)
		}
	})
	if err != nil {
		return err
	}
	defer transaction.End()
	if err := transaction.Authenticate(pam.DisallowNullAuthtok); err != nil {
		return err
	}
	return transaction.AcctMgmt(pam.Silent)
}
