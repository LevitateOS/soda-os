package store

import (
	"context"
	"fmt"

	"github.com/LevitateOS/soda-os/internal/domain"
	"gorm.io/gorm"
)

func (s *Store) CreateProjectWithMemberships(ctx context.Context, value domain.Project, personIDs []string) error {
	kind, remote, err := sourceColumns(value.Source)
	if err != nil {
		return err
	}
	return classify(s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var bootstrap *string
		if value.BootstrapPersonID != "" {
			bootstrap = &value.BootstrapPersonID
		}
		if err := tx.Create(&Project{ID: value.ID, Slug: value.Slug, Name: value.Name, UnixUser: value.UnixUser, Profile: string(value.Profile), SourceKind: kind, SourceRemoteURL: remote, BootstrapPersonID: bootstrap}).Error; err != nil {
			return err
		}
		for _, personID := range personIDs {
			if err := tx.Create(&Membership{ProjectID: value.ID, PersonID: personID}).Error; err != nil {
				return err
			}
		}
		return nil
	}))
}

// DeleteFreshProject is internal failed-create compensation. Foreign-key
// restrictions ensure it cannot remove a project once related state exists.
func (s *Store) DeleteFreshProject(ctx context.Context, id string) error {
	return classify(s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("project_id = ?", id).Delete(&Membership{}).Error; err != nil {
			return err
		}
		result := tx.Where("id = ?", id).Delete(&Project{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrNotFound
		}
		return nil
	}))
}

func (s *Store) Projects(ctx context.Context) ([]domain.Project, error) {
	var rows []Project
	if err := s.db.WithContext(ctx).Order("slug").Find(&rows).Error; err != nil {
		return nil, err
	}
	values := make([]domain.Project, 0, len(rows))
	for _, row := range rows {
		value, err := projectDomain(row)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

func (s *Store) Project(ctx context.Context, id string) (domain.Project, error) {
	var row Project
	if err := s.db.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		return domain.Project{}, classify(err)
	}
	return projectDomain(row)
}

func (s *Store) PreflightProject(ctx context.Context, slug, unixUser string) error {
	var count int64
	if err := s.db.WithContext(ctx).Model(&Project{}).Where("slug = ? OR unix_user = ?", slug, unixUser).Count(&count).Error; err != nil {
		return err
	}
	if count != 0 {
		return fmt.Errorf("%w: project %s", ErrAlreadyExists, slug)
	}
	return nil
}

func (s *Store) ProjectsForPerson(ctx context.Context, personID string) ([]domain.Project, error) {
	var rows []Project
	err := s.db.WithContext(ctx).Table("projects").Joins("JOIN memberships ON memberships.project_id = projects.id").Where("memberships.person_id = ?", personID).Order("projects.slug").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	values := make([]domain.Project, 0, len(rows))
	for _, row := range rows {
		value, err := projectDomain(row)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

func (s *Store) AddMembershipAndWorktree(ctx context.Context, membership domain.Membership, worktree domain.Worktree) error {
	return classify(s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&Membership{ProjectID: membership.ProjectID, PersonID: membership.PersonID}).Error; err != nil {
			return err
		}
		return tx.Create(worktreeRow(worktree)).Error
	}))
}

func (s *Store) DeleteMembershipAndWorktree(ctx context.Context, projectID, personID string) error {
	return classify(s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("project_id = ? AND person_id = ?", projectID, personID).Delete(&Worktree{}).Error; err != nil {
			return err
		}
		result := tx.Where("project_id = ? AND person_id = ?", projectID, personID).Delete(&Membership{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrNotFound
		}
		return nil
	}))
}

func (s *Store) Membership(ctx context.Context, projectID, personID string) (domain.Membership, error) {
	var row Membership
	if err := s.db.WithContext(ctx).First(&row, "project_id = ? AND person_id = ?", projectID, personID).Error; err != nil {
		return domain.Membership{}, classify(err)
	}
	return domain.Membership{ProjectID: row.ProjectID, PersonID: row.PersonID}, nil
}

func (s *Store) Collaborators(ctx context.Context, projectID string) ([]domain.Person, error) {
	var rows []Person
	err := s.db.WithContext(ctx).Table("people").Joins("JOIN memberships ON memberships.person_id = people.id").Where("memberships.project_id = ?", projectID).Order("people.username").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	values := make([]domain.Person, 0, len(rows))
	for _, row := range rows {
		values = append(values, personDomain(row))
	}
	return values, nil
}

func (s *Store) CreateWorktree(ctx context.Context, value domain.Worktree) error {
	return classify(s.db.WithContext(ctx).Create(worktreeRow(value)).Error)
}

func (s *Store) PreflightWorktree(ctx context.Context, value domain.Worktree) error {
	var count int64
	err := s.db.WithContext(ctx).Model(&Worktree{}).
		Where("path = ? OR (project_id = ? AND branch = ?) OR (project_id = ? AND person_id = ?)", value.Path, value.ProjectID, value.Branch, value.ProjectID, value.PersonID).
		Count(&count).Error
	if err != nil {
		return err
	}
	if count != 0 {
		return fmt.Errorf("%w: worktree %s", ErrAlreadyExists, value.Name)
	}
	return nil
}

func (s *Store) Worktrees(ctx context.Context, projectID string) ([]domain.Worktree, error) {
	var rows []Worktree
	if err := s.db.WithContext(ctx).Where("project_id = ?", projectID).Order("path").Find(&rows).Error; err != nil {
		return nil, err
	}
	values := make([]domain.Worktree, 0, len(rows))
	for _, row := range rows {
		values = append(values, worktreeDomain(row))
	}
	return values, nil
}

func (s *Store) WorktreesForPerson(ctx context.Context, projectID, personID string) ([]domain.Worktree, error) {
	var rows []Worktree
	if err := s.db.WithContext(ctx).Where("project_id = ? AND person_id = ?", projectID, personID).Order("path").Find(&rows).Error; err != nil {
		return nil, err
	}
	values := make([]domain.Worktree, 0, len(rows))
	for _, row := range rows {
		values = append(values, worktreeDomain(row))
	}
	return values, nil
}
func sourceColumns(source domain.ProjectSource) (string, *string, error) {
	switch value := source.(type) {
	case domain.EmptyProjectSource:
		return "empty", nil, nil
	case domain.GitProjectSource:
		remote := value.RemoteURL
		return "git", &remote, nil
	default:
		return "", nil, fmt.Errorf("unknown project source %T", source)
	}
}
func projectDomain(r Project) (domain.Project, error) {
	var source domain.ProjectSource
	switch r.SourceKind {
	case "empty":
		source = domain.EmptyProjectSource{}
	case "git":
		if r.SourceRemoteURL == nil {
			return domain.Project{}, fmt.Errorf("git project %s has no remote URL", r.ID)
		}
		source = domain.GitProjectSource{RemoteURL: *r.SourceRemoteURL}
	default:
		return domain.Project{}, fmt.Errorf("project %s has unknown source %q", r.ID, r.SourceKind)
	}
	bootstrap := ""
	if r.BootstrapPersonID != nil {
		bootstrap = *r.BootstrapPersonID
	}
	return domain.Project{ID: r.ID, Slug: r.Slug, Name: r.Name, UnixUser: r.UnixUser, Profile: domain.ToolchainProfile(r.Profile), Source: source, BootstrapPersonID: bootstrap}, nil
}
func worktreeRow(v domain.Worktree) *Worktree {
	return &Worktree{ID: v.ID, ProjectID: v.ProjectID, PersonID: v.PersonID, Name: v.Name, Branch: v.Branch, Path: v.Path}
}
func worktreeDomain(r Worktree) domain.Worktree {
	return domain.Worktree{ID: r.ID, ProjectID: r.ProjectID, PersonID: r.PersonID, Name: r.Name, Branch: r.Branch, Path: r.Path}
}
