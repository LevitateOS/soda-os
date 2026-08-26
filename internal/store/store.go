package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/LevitateOS/soda-os/internal/domain"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const SchemaVersion = 1

var (
	ErrNotFound          = errors.New("resource not found")
	ErrAlreadyExists     = errors.New("resource already exists")
	ErrUnsupportedSchema = errors.New("unsupported database schema")
)

type Person struct {
	ID           string `gorm:"primaryKey;size:36"`
	Username     string `gorm:"not null;uniqueIndex;size:24"`
	DisplayName  string `gorm:"not null"`
	Email        string `gorm:"not null"`
	Role         string `gorm:"not null;check:role IN ('admin','developer')"`
	SSHPublicKey string `gorm:"not null"`
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
	ProjectID string  `gorm:"not null;uniqueIndex:worktree_identity;size:36"`
	PersonID  string  `gorm:"not null;uniqueIndex:worktree_identity;size:36"`
	Name      string  `gorm:"not null;uniqueIndex:worktree_identity"`
	Branch    string  `gorm:"not null"`
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

func Open(path string) (*Store, error) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent), TranslateError: true})
	if err != nil {
		return nil, fmt.Errorf("open Soda database: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("open Soda SQL connection: %w", err)
	}
	// A single connection keeps SQLite PRAGMAs and concurrent daemon access
	// deterministic. Calls still queue safely through database/sql.
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	if err := db.Exec("PRAGMA busy_timeout = 5000").Error; err != nil {
		return nil, fmt.Errorf("configure SQLite busy timeout: %w", err)
	}
	if err := db.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
		return nil, fmt.Errorf("enable SQLite foreign keys: %w", err)
	}
	var version int
	if err := db.Raw("PRAGMA user_version").Scan(&version).Error; err != nil {
		return nil, fmt.Errorf("read Soda schema version: %w", err)
	}
	var tables int64
	if err := db.Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Scan(&tables).Error; err != nil {
		return nil, fmt.Errorf("inspect Soda schema: %w", err)
	}
	if version == 0 && tables != 0 {
		return nil, fmt.Errorf("%w: unversioned database; Soda OS 0.2 requires a fresh installation", ErrUnsupportedSchema)
	}
	if version != 0 && version != SchemaVersion {
		return nil, fmt.Errorf("%w: found version %d, expected %d; Soda OS 0.2 requires a fresh installation", ErrUnsupportedSchema, version, SchemaVersion)
	}
	if version == 0 {
		if err := db.AutoMigrate(&Person{}, &Project{}, &Membership{}, &Worktree{}, &ToolchainInstallation{}, &ProjectToolchainResolution{}, &ProvisioningJob{}); err != nil {
			return nil, fmt.Errorf("create Soda schema: %w", err)
		}
		if err := db.Exec(fmt.Sprintf("PRAGMA user_version = %d", SchemaVersion)).Error; err != nil {
			return nil, fmt.Errorf("record Soda schema version: %w", err)
		}
	}
	return &Store{db: db}, nil
}

func (s *Store) DB() *gorm.DB { return s.db }

func (s *Store) CreatePerson(ctx context.Context, value domain.Person) error {
	return classify(s.db.WithContext(ctx).Create(&Person{value.ID, value.Username, value.DisplayName, value.Email, string(value.Role), value.SSHPublicKey}).Error)
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

func (s *Store) PersonByUsername(ctx context.Context, username string) (domain.Person, error) {
	var row Person
	if err := s.db.WithContext(ctx).First(&row, "username = ?", username).Error; err != nil {
		return domain.Person{}, classify(err)
	}
	return personDomain(row), nil
}

func (s *Store) CreateProject(ctx context.Context, value domain.Project) error {
	kind, remote, err := sourceColumns(value.Source)
	if err != nil {
		return err
	}
	return classify(s.db.WithContext(ctx).Create(&Project{ID: value.ID, Slug: value.Slug, Name: value.Name, UnixUser: value.UnixUser, Profile: string(value.Profile), SourceKind: kind, SourceRemoteURL: remote}).Error)
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

// ListProjects implements observe.ProjectStore without exposing GORM models.
func (s *Store) ListProjects(ctx context.Context) ([]domain.Project, error) { return s.Projects(ctx) }

// ListPeople implements observe.ProjectStore without exposing GORM models.
func (s *Store) ListPeople(ctx context.Context) ([]domain.Person, error) { return s.People(ctx) }

func (s *Store) Project(ctx context.Context, id string) (domain.Project, error) {
	var row Project
	if err := s.db.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		return domain.Project{}, classify(err)
	}
	return projectDomain(row)
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

// ListWorktrees implements observe.ProjectStore without exposing GORM models.
func (s *Store) ListWorktrees(ctx context.Context, projectID string) ([]domain.Worktree, error) {
	return s.Worktrees(ctx, projectID)
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

func (s *Store) CreateJob(ctx context.Context, value domain.ProvisioningJob) error {
	return classify(s.db.WithContext(ctx).Create(jobRow(value)).Error)
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
		row := installationRow(value)
		if err := tx.Where("profile = ? AND version = ?", row.Profile, row.Version).Assign(map[string]any{"path": row.Path, "checksum": row.Checksum, "state": row.State}).FirstOrCreate(&row).Error; err != nil {
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
func personDomain(r Person) domain.Person {
	return domain.Person{ID: r.ID, Username: r.Username, DisplayName: r.DisplayName, Email: r.Email, Role: domain.Role(r.Role), SSHPublicKey: r.SSHPublicKey}
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
	return domain.Project{ID: r.ID, Slug: r.Slug, Name: r.Name, UnixUser: r.UnixUser, Profile: domain.ToolchainProfile(r.Profile), Source: source}, nil
}
func worktreeRow(v domain.Worktree) *Worktree {
	return &Worktree{ID: v.ID, ProjectID: v.ProjectID, PersonID: v.PersonID, Name: v.Name, Branch: v.Branch, Path: v.Path}
}
func worktreeDomain(r Worktree) domain.Worktree {
	return domain.Worktree{ID: r.ID, ProjectID: r.ProjectID, PersonID: r.PersonID, Name: r.Name, Branch: r.Branch, Path: r.Path}
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
