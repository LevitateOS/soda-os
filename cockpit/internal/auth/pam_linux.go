//go:build linux && cgo

package auth

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

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
	if err := validateNewPassword(newPassword); err != nil {
		return err
	}
	changing := false
	transaction, err := pam.StartFunc("soda-cockpit", username, func(style pam.Style, prompt string) (string, error) {
		switch style {
		case pam.PromptEchoOff:
			if !changing {
				return currentPassword, nil
			}
			lower := strings.ToLower(prompt)
			if strings.Contains(lower, "current") || strings.Contains(lower, "old") {
				return currentPassword, nil
			}
			return newPassword, nil
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
	if err = transaction.Authenticate(pam.DisallowNullAuthtok); err != nil {
		return err
	}
	accountErr := transaction.AcctMgmt(pam.Silent)
	if accountErr != nil && !errors.Is(accountErr, pam.ErrNewAuthtokReqd) {
		return accountErr
	}
	changing = true
	flags := pam.Flags(0)
	if errors.Is(accountErr, pam.ErrNewAuthtokReqd) {
		flags = pam.ChangeExpiredAuthtok
	}
	return transaction.ChangeAuthTok(flags)
}

func validateNewPassword(password string) error {
	if utf8.RuneCountInString(password) < 6 {
		return errors.New("password must contain at least 6 characters")
	}
	if strings.ContainsAny(password, "\r\n\x00") {
		return errors.New("password contains a line or NUL delimiter")
	}
	return nil
}
