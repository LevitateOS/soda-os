//go:build linux && cgo

package auth

import (
	"errors"
	"fmt"
	"strings"

	pam "github.com/msteinert/pam/v2"
)

func (PAM) Authenticate(username, password string) (Result, error) {
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
		return "", err
	}
	defer transaction.End()
	if err := transaction.Authenticate(pam.DisallowNullAuthtok); err != nil {
		return "", err
	}
	if err := transaction.AcctMgmt(pam.Silent); err != nil {
		if errors.Is(err, pam.ErrNewAuthtokReqd) {
			return PasswordChangeRequired, nil
		}
		return "", err
	}
	return Authenticated, nil
}

func (PAM) ChangePassword(username, currentPassword, newPassword string) error {
	conversation := passwordChangeConversation{username: username, currentPassword: currentPassword, newPassword: newPassword}
	transaction, err := pam.StartFunc("soda-cockpit", username, conversation.respond)
	if err != nil {
		return err
	}
	defer transaction.End()
	if err = transaction.Authenticate(pam.DisallowNullAuthtok); err != nil {
		return err
	}
	flags, err := passwordChangeFlags(transaction.AcctMgmt(pam.Silent))
	if err != nil {
		return err
	}
	conversation.changing = true
	return transaction.ChangeAuthTok(flags)
}

type passwordChangeConversation struct {
	username, currentPassword, newPassword string
	changing                               bool
}

func (c *passwordChangeConversation) respond(style pam.Style, prompt string) (string, error) {
	switch style {
	case pam.PromptEchoOff:
		if !c.changing {
			return c.currentPassword, nil
		}
		lower := strings.ToLower(prompt)
		if strings.Contains(lower, "current") || strings.Contains(lower, "old") {
			return c.currentPassword, nil
		}
		return c.newPassword, nil
	case pam.PromptEchoOn:
		return c.username, nil
	case pam.ErrorMsg, pam.TextInfo:
		return "", nil
	default:
		return "", fmt.Errorf("unsupported PAM conversation style %d", style)
	}
}

func passwordChangeFlags(accountErr error) (pam.Flags, error) {
	switch {
	case accountErr == nil:
		return 0, nil
	case errors.Is(accountErr, pam.ErrNewAuthtokReqd):
		return pam.ChangeExpiredAuthtok, nil
	default:
		return 0, accountErr
	}
}
