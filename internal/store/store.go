package store

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

const SchemaVersion = 2

var (
	ErrNotFound           = errors.New("resource not found")
	ErrAlreadyExists      = errors.New("resource already exists")
	ErrFailedPrecondition = errors.New("resource state does not allow operation")
	ErrUnsupportedSchema  = errors.New("unsupported database schema")
)

type Person struct {
	ID          string `gorm:"primaryKey;size:36"`
	Username    string `gorm:"not null;uniqueIndex;size:24"`
	DisplayName string `gorm:"not null"`
	Email       string `gorm:"not null"`
	Role        string `gorm:"not null;check:role IN ('admin','developer')"`
}

type SSHDeviceKey struct {
	ID               string `gorm:"primaryKey;size:36"`
	PersonID         string `gorm:"not null;uniqueIndex:ssh_device_label;size:36"`
	Label            string `gorm:"not null;uniqueIndex:ssh_device_label;size:40"`
	PublicKey        string `gorm:"not null"`
	Fingerprint      string `gorm:"not null;uniqueIndex"`
	IdentityFileHint string `gorm:"not null"`
	CreatedAt        int64  `gorm:"autoCreateTime:nano;index"`
	Person           Person `gorm:"constraint:OnDelete:RESTRICT;foreignKey:PersonID"`
}

type Project struct {
	ID              string `gorm:"primaryKey;size:36"`
	Slug            string `gorm:"not null;uniqueIndex;size:24"`
	Name            string `gorm:"not null"`
	UnixUser        string `gorm:"not null;uniqueIndex"`
	Profile         string `gorm:"not null;check:profile IN ('web','python','rust','go')"`
	SourceKind      string `gorm:"not null;check:source_kind IN ('empty','git');check:project_source_valid,(source_kind = 'empty' AND source_remote_url IS NULL) OR (source_kind = 'git' AND source_remote_url IS NOT NULL)"`
	SourceRemoteURL *string
}

type Membership struct {
	ProjectID string  `gorm:"primaryKey;size:36"`
	PersonID  string  `gorm:"primaryKey;size:36"`
	Project   Project `gorm:"constraint:OnDelete:RESTRICT;foreignKey:ProjectID"`
	Person    Person  `gorm:"constraint:OnDelete:RESTRICT;foreignKey:PersonID"`
}

type Worktree struct {
	ID        string  `gorm:"primaryKey;size:36"`
	ProjectID string  `gorm:"not null;uniqueIndex:worktree_person;uniqueIndex:worktree_branch;size:36"`
	PersonID  string  `gorm:"not null;uniqueIndex:worktree_person;size:36"`
	Name      string  `gorm:"not null"`
	Branch    string  `gorm:"not null;uniqueIndex:worktree_branch"`
	Path      string  `gorm:"not null;uniqueIndex"`
	Project   Project `gorm:"constraint:OnDelete:RESTRICT;foreignKey:ProjectID"`
	Person    Person  `gorm:"constraint:OnDelete:RESTRICT;foreignKey:PersonID"`
}

type ToolchainInstallation struct {
	ID       string `gorm:"primaryKey;size:36"`
	Profile  string `gorm:"not null;uniqueIndex:toolchain_version;check:profile IN ('web','python','rust','go')"`
	Version  string `gorm:"not null;uniqueIndex:toolchain_version"`
	Path     string `gorm:"not null"`
	Checksum string `gorm:"not null"`
	State    string `gorm:"not null;check:state IN ('installing','ready','failed')"`
}

type ProjectToolchainResolution struct {
	ProjectID               string                `gorm:"primaryKey;size:36"`
	ToolchainInstallationID string                `gorm:"not null;size:36"`
	Project                 Project               `gorm:"constraint:OnDelete:RESTRICT;foreignKey:ProjectID"`
	Installation            ToolchainInstallation `gorm:"constraint:OnDelete:RESTRICT;foreignKey:ToolchainInstallationID"`
}

type ProvisioningJob struct {
	ID        string `gorm:"primaryKey;size:36"`
	ProjectID string `gorm:"not null;index;size:36"`
	State     string `gorm:"not null;check:state IN ('installing','ready','failed')"`
	Error     *string
	CreatedAt int64   `gorm:"autoCreateTime:nano;index"`
	Project   Project `gorm:"constraint:OnDelete:RESTRICT;foreignKey:ProjectID"`
}

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
