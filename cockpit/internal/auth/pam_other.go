//go:build !linux || !cgo

package auth

import "errors"

func (PAM) Authenticate(_, _ string) error {
	return errors.New("PAM authentication is available only in a Linux cgo build")
}
