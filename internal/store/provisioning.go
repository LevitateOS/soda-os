package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/LevitateOS/soda-os/internal/domain"
	"gorm.io/gorm"
)

func (s *Store) BeginProvisioning(ctx context.Context, value domain.ProvisioningJob) error {
	return classify(s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var project Project
		if err := tx.First(&project, "id = ?", value.ProjectID).Error; err != nil {
			return err
		}
		var latest ProvisioningJob
		err := tx.Where("project_id = ?", value.ProjectID).Order("created_at DESC").First(&latest).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil && (latest.State == string(domain.JobInstalling) || latest.State == string(domain.JobReady)) {
			return fmt.Errorf("%w: project provisioning is %s", ErrFailedPrecondition, latest.State)
		}
		return tx.Create(jobRow(value)).Error
	}))
}
func (s *Store) UpdateJob(ctx context.Context, value domain.ProvisioningJob) error {
	r := s.db.WithContext(ctx).Model(&ProvisioningJob{}).Where("id = ?", value.ID).Updates(map[string]any{"state": string(value.State), "error": value.Error})
	if r.Error != nil {
		return classify(r.Error)
	}
	if r.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// FailInterruptedProvisioning makes jobs abandoned by an earlier sodad
// process explicitly retryable. It must run once during daemon startup before
// accepting provisioning requests.
func (s *Store) FailInterruptedProvisioning(ctx context.Context) (int64, error) {
	message := "provisioning interrupted by daemon restart; retry provisioning manually"
	result := s.db.WithContext(ctx).Model(&ProvisioningJob{}).
		Where("state = ?", string(domain.JobInstalling)).
		Updates(map[string]any{"state": string(domain.JobFailed), "error": message})
	if result.Error != nil {
		return 0, classify(result.Error)
	}
	return result.RowsAffected, nil
}

func (s *Store) Jobs(ctx context.Context, projectID string) ([]domain.ProvisioningJob, error) {
	var rows []ProvisioningJob
	if err := s.db.WithContext(ctx).Where("project_id = ?", projectID).Order("created_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	values := make([]domain.ProvisioningJob, 0, len(rows))
	for _, row := range rows {
		values = append(values, jobDomain(row))
	}
	return values, nil
}

func (s *Store) SaveInstallation(ctx context.Context, projectID string, value domain.ToolchainInstallation) (domain.ProjectToolchainResolution, error) {
	var resolution domain.ProjectToolchainResolution
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		candidate := installationRow(value)
		var row ToolchainInstallation
		err := tx.Where("profile = ? AND version = ?", candidate.Profile, candidate.Version).First(&row).Error
		switch {
		case err == nil:
			if err = tx.Model(&row).Updates(map[string]any{"path": candidate.Path, "checksum": candidate.Checksum, "state": candidate.State}).Error; err != nil {
				return err
			}
		case errors.Is(err, gorm.ErrRecordNotFound):
			row = candidate
			if err = tx.Create(&row).Error; err != nil {
				return err
			}
		default:
			return err
		}
		link := ProjectToolchainResolution{ProjectID: projectID, ToolchainInstallationID: row.ID}
		if err := tx.Save(&link).Error; err != nil {
			return err
		}
		resolution = domain.ProjectToolchainResolution{ProjectID: projectID, ToolchainInstallationID: row.ID}
		return nil
	})
	return resolution, classify(err)
}

func (s *Store) ProjectInstallation(ctx context.Context, projectID string) (domain.ToolchainInstallation, domain.ProjectToolchainResolution, error) {
	var row ToolchainInstallation
	var link ProjectToolchainResolution
	err := s.db.WithContext(ctx).First(&link, "project_id = ?", projectID).Error
	if err != nil {
		return domain.ToolchainInstallation{}, domain.ProjectToolchainResolution{}, classify(err)
	}
	if err = s.db.WithContext(ctx).First(&row, "id = ?", link.ToolchainInstallationID).Error; err != nil {
		return domain.ToolchainInstallation{}, domain.ProjectToolchainResolution{}, classify(err)
	}
	return installationDomain(row), domain.ProjectToolchainResolution{ProjectID: link.ProjectID, ToolchainInstallationID: link.ToolchainInstallationID}, nil
}

func jobRow(v domain.ProvisioningJob) *ProvisioningJob {
	return &ProvisioningJob{ID: v.ID, ProjectID: v.ProjectID, State: string(v.State), Error: v.Error}
}

func jobDomain(r ProvisioningJob) domain.ProvisioningJob {
	return domain.ProvisioningJob{ID: r.ID, ProjectID: r.ProjectID, State: domain.JobState(r.State), Error: r.Error}
}
func installationRow(v domain.ToolchainInstallation) ToolchainInstallation {
	return ToolchainInstallation{ID: v.ID, Profile: string(v.Profile), Version: v.Version, Path: v.Path, Checksum: v.Checksum, State: string(v.State)}
}
func installationDomain(r ToolchainInstallation) domain.ToolchainInstallation {
	return domain.ToolchainInstallation{ID: r.ID, Profile: domain.ToolchainProfile(r.Profile), Version: r.Version, Path: r.Path, Checksum: r.Checksum, State: domain.JobState(r.State)}
}
