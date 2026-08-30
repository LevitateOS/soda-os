package store

import (
	"context"

	"github.com/LevitateOS/soda-os/internal/domain"
)

func (s *Store) SaveBuiltInGitUser(ctx context.Context, value domain.BuiltInGitUser) error {
	return classify(s.db.WithContext(ctx).Create(&BuiltInGitUser{PersonID: value.PersonID, UserID: value.UserID}).Error)
}

func (s *Store) BuiltInGitUser(ctx context.Context, personID string) (domain.BuiltInGitUser, error) {
	var row BuiltInGitUser
	if err := s.db.WithContext(ctx).First(&row, "person_id = ?", personID).Error; err != nil {
		return domain.BuiltInGitUser{}, classify(err)
	}
	return domain.BuiltInGitUser{PersonID: row.PersonID, UserID: row.UserID}, nil
}

func (s *Store) SaveBuiltInGitKey(ctx context.Context, value domain.BuiltInGitKey) error {
	return classify(s.db.WithContext(ctx).Create(&BuiltInGitKey{SSHDeviceKeyID: value.SSHDeviceKeyID, PersonID: value.PersonID, KeyID: value.KeyID}).Error)
}

func (s *Store) BuiltInGitKey(ctx context.Context, keyID string) (domain.BuiltInGitKey, error) {
	var row BuiltInGitKey
	if err := s.db.WithContext(ctx).First(&row, "ssh_device_key_id = ?", keyID).Error; err != nil {
		return domain.BuiltInGitKey{}, classify(err)
	}
	return domain.BuiltInGitKey{SSHDeviceKeyID: row.SSHDeviceKeyID, PersonID: row.PersonID, KeyID: row.KeyID}, nil
}

func (s *Store) SaveBuiltInGitIdentity(ctx context.Context, value domain.BuiltInGitIdentity) error {
	return classify(s.db.WithContext(ctx).Create(&BuiltInGitIdentity{PersonID: value.PersonID, KeyID: value.KeyID}).Error)
}

func (s *Store) BuiltInGitIdentity(ctx context.Context, personID string) (domain.BuiltInGitIdentity, error) {
	var row BuiltInGitIdentity
	if err := s.db.WithContext(ctx).First(&row, "person_id = ?", personID).Error; err != nil {
		return domain.BuiltInGitIdentity{}, classify(err)
	}
	return domain.BuiltInGitIdentity{PersonID: row.PersonID, KeyID: row.KeyID}, nil
}

func (s *Store) SaveBuiltInGitRepository(ctx context.Context, value domain.BuiltInGitRepository) error {
	return classify(s.db.WithContext(ctx).Create(&BuiltInGitRepository{ProjectID: value.ProjectID, RepositoryID: value.RepositoryID, DeployKeyID: value.DeployKeyID, WebURL: value.WebURL, SSHURL: value.SSHURL}).Error)
}

func (s *Store) BuiltInGitRepository(ctx context.Context, projectID string) (domain.BuiltInGitRepository, error) {
	var row BuiltInGitRepository
	if err := s.db.WithContext(ctx).First(&row, "project_id = ?", projectID).Error; err != nil {
		return domain.BuiltInGitRepository{}, classify(err)
	}
	return domain.BuiltInGitRepository{ProjectID: row.ProjectID, RepositoryID: row.RepositoryID, DeployKeyID: row.DeployKeyID, WebURL: row.WebURL, SSHURL: row.SSHURL}, nil
}
