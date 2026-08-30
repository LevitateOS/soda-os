//go:build !linux || !cgo

package auth

import "errors"

func (PAM) Authenticate(_, _ string) (Result, error) {
	return "", errors.New("PAM authentication is available only in a Linux cgo build")
}

func (PAM) AuthenticatePasswordless(_ string) (Result, error) {
	return "", errors.New("PAM authentication is available only in a Linux cgo build")
}

func (PAM) ChangePassword(_, _, _ string) error {
	return errors.New("PAM password changes are available only in a Linux cgo build")
}
