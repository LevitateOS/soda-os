package image

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
)

func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func verifyFileSHA256(path, expected string) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(contents)
	if hex.EncodeToString(digest[:]) != expected {
		return errors.New("SHA-256 checksum mismatch")
	}
	return nil
}
