package domain

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
)

// ParseSSHKey normalizes an authorized key and calculates its OpenSSH SHA256
// fingerprint. Comments are excluded because they are not key identity.
func ParseSSHKey(key string) (string, string, error) {
	if strings.ContainsAny(key, "\r\n\x00") {
		return "", "", errors.New("SSH public key is not a supported single-line key")
	}
	fields := strings.Fields(key)
	if len(fields) < 2 || (fields[0] != "ssh-ed25519" && fields[0] != "ssh-rsa") {
		return "", "", errors.New("SSH public key is not a supported key")
	}
	decoded, err := base64.StdEncoding.DecodeString(fields[1])
	if err != nil {
		return "", "", errors.New("SSH public key payload is invalid")
	}
	digest := sha256.Sum256(decoded)
	return strings.Join(fields[:2], " "), "SHA256:" + base64.RawStdEncoding.EncodeToString(digest[:]), nil
}
