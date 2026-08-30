package store

import (
	"context"
	"fmt"
	"time"

	"github.com/LevitateOS/soda-os/internal/domain"
	"gorm.io/gorm"
)

func (s *Store) CreatePerson(ctx context.Context, value domain.Person) error {
	return s.CreatePersonWithGitIdentity(ctx, value, domain.GitIdentity{})
}

func (s *Store) CreatePersonWithGitIdentity(ctx context.Context, value domain.Person, identity domain.GitIdentity) error {
	return classify(s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&Person{ID: value.ID, Username: value.Username, DisplayName: value.DisplayName, Email: value.Email, Role: string(value.Role)}).Error; err != nil {
			return err
		}
		if identity.PersonID == "" {
			return nil
		}
		return tx.Create(&GitIdentity{PersonID: identity.PersonID, PublicKey: identity.PublicKey, Fingerprint: identity.Fingerprint}).Error
	}))
}

func (s *Store) DeleteFreshPerson(ctx context.Context, id string) error {
	return classify(s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("person_id = ?", id).Delete(&GitIdentity{}).Error; err != nil {
			return err
		}
		result := tx.Where("id = ?", id).Delete(&Person{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrNotFound
		}
		return nil
	}))
}

func (s *Store) GitIdentity(ctx context.Context, personID string) (domain.GitIdentity, error) {
	var row GitIdentity
	if err := s.db.WithContext(ctx).First(&row, "person_id = ?", personID).Error; err != nil {
		return domain.GitIdentity{}, classify(err)
	}
	return domain.GitIdentity{PersonID: row.PersonID, PublicKey: row.PublicKey, Fingerprint: row.Fingerprint}, nil
}

func (s *Store) People(ctx context.Context) ([]domain.Person, error) {
	var rows []Person
	if err := s.db.WithContext(ctx).Order("username").Find(&rows).Error; err != nil {
		return nil, err
	}
	values := make([]domain.Person, 0, len(rows))
	for _, row := range rows {
		values = append(values, personDomain(row))
	}
	return values, nil
}

func (s *Store) Person(ctx context.Context, id string) (domain.Person, error) {
	var row Person
	if err := s.db.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		return domain.Person{}, classify(err)
	}
	return personDomain(row), nil
}

func (s *Store) PreflightPerson(ctx context.Context, username string) error {
	var count int64
	if err := s.db.WithContext(ctx).Model(&Person{}).Where("username = ?", username).Count(&count).Error; err != nil {
		return err
	}
	if count != 0 {
		return fmt.Errorf("%w: person %s", ErrAlreadyExists, username)
	}
	return nil
}

func (s *Store) CreateSSHDeviceKey(ctx context.Context, value domain.SSHDeviceKey) error {
	return classify(s.db.WithContext(ctx).Create(sshDeviceKeyRow(value)).Error)
}

func (s *Store) SSHDeviceKeys(ctx context.Context, personID string) ([]domain.SSHDeviceKey, error) {
	var rows []SSHDeviceKey
	if err := s.db.WithContext(ctx).Where("person_id = ?", personID).Order("label, id").Find(&rows).Error; err != nil {
		return nil, err
	}
	values := make([]domain.SSHDeviceKey, 0, len(rows))
	for _, row := range rows {
		values = append(values, sshDeviceKeyDomain(row))
	}
	return values, nil
}

func (s *Store) DeleteSSHDeviceKey(ctx context.Context, personID, keyID string) (domain.SSHDeviceKey, error) {
	var removed domain.SSHDeviceKey
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row SSHDeviceKey
		if err := tx.First(&row, "id = ? AND person_id = ?", keyID, personID).Error; err != nil {
			return err
		}
		if err := tx.Where("ssh_device_key_id = ?", row.ID).Delete(&BuiltInGitKey{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&row).Error; err != nil {
			return err
		}
		removed = sshDeviceKeyDomain(row)
		return nil
	})
	return removed, classify(err)
}
func personDomain(r Person) domain.Person {
	return domain.Person{ID: r.ID, Username: r.Username, DisplayName: r.DisplayName, Email: r.Email, Role: domain.Role(r.Role)}
}

func sshDeviceKeyRow(v domain.SSHDeviceKey) *SSHDeviceKey {
	createdAt := v.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	return &SSHDeviceKey{ID: v.ID, PersonID: v.PersonID, Label: v.Label, PublicKey: v.PublicKey, Fingerprint: v.Fingerprint, IdentityFileHint: v.IdentityFileHint, CreatedAt: createdAt.UnixNano()}
}

func sshDeviceKeyDomain(r SSHDeviceKey) domain.SSHDeviceKey {
	return domain.SSHDeviceKey{ID: r.ID, PersonID: r.PersonID, Label: r.Label, PublicKey: r.PublicKey, Fingerprint: r.Fingerprint, IdentityFileHint: r.IdentityFileHint, CreatedAt: time.Unix(0, r.CreatedAt)}
}
