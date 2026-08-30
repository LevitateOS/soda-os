package store

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

const SchemaVersion = 4

var (
	ErrNotFound           = errors.New("resource not found")
	ErrAlreadyExists      = errors.New("resource already exists")
	ErrFailedPrecondition = errors.New("resource state does not allow operation")
	ErrUnsupportedSchema  = errors.New("unsupported database schema")
)

type Store struct{ db *gorm.DB }

func (s *Store) DB() *gorm.DB { return s.db }
func classify(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("%w: %v", ErrNotFound, err)
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(strings.ToLower(err.Error()), "unique constraint") {
		return fmt.Errorf("%w: %v", ErrAlreadyExists, err)
	}
	return err
}
