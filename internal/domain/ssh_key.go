package domain

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
)

// SSHKeyFingerprint returns the OpenSSH SHA256 fingerprint for an authorized
// key. Comments are deliberately excluded because they are not key identity.
func SSHKeyFingerprint(key string) (string, error) {
	fields := strings.Fields(key)
	if len(fields) < 2 || (!strings.HasPrefix(fields[0], "ssh-") && !strings.HasPrefix(fields[0], "ecdsa-")) {
		return "", errors.New("SSH public key is not a supported key")
	}
	decoded, err := base64.StdEncoding.DecodeString(fields[1])
	if err != nil {
		return "", errors.New("SSH public key payload is invalid")
	}
	digest := sha256.Sum256(decoded)
	return "SHA256:" + base64.RawStdEncoding.EncodeToString(digest[:]), nil
}
