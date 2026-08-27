package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/LevitateOS/soda-os/internal/domain"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
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
		return nil, fmt.Errorf("%w: unversioned database; this Soda OS release requires a fresh installation", ErrUnsupportedSchema)
	}
	if version != 0 && version != SchemaVersion {
		return nil, fmt.Errorf("%w: found version %d, expected %d; this Soda OS release requires a fresh installation", ErrUnsupportedSchema, version, SchemaVersion)
	}
	if version == 0 {
		if err := initializeSchema(db, nil); err != nil {
			return nil, err
		}
	}
	return &Store{db: db}, nil
}

func initializeSchema(db *gorm.DB, beforeVersion func(*gorm.DB) error) error {
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.AutoMigrate(&Person{}, &SSHDeviceKey{}, &Project{}, &Membership{}, &Worktree{}, &ToolchainInstallation{}, &ProjectToolchainResolution{}, &ProvisioningJob{}); err != nil {
			return fmt.Errorf("create Soda schema: %w", err)
		}
		if beforeVersion != nil {
			if err := beforeVersion(tx); err != nil {
				return err
			}
		}
		if err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", SchemaVersion)).Error; err != nil {
			return fmt.Errorf("record Soda schema version: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("initialize Soda schema: %w", err)
	}
	return nil
}

func (s *Store) DB() *gorm.DB { return s.db }

func (s *Store) CreatePerson(ctx context.Context, value domain.Person) error {
	return classify(s.db.WithContext(ctx).Create(&Person{ID: value.ID, Username: value.Username, DisplayName: value.DisplayName, Email: value.Email, Role: string(value.Role)}).Error)
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

func (s *Store) SSHDeviceKey(ctx context.Context, personID, keyID string) (domain.SSHDeviceKey, error) {
	var row SSHDeviceKey
	if err := s.db.WithContext(ctx).First(&row, "id = ? AND person_id = ?", keyID, personID).Error; err != nil {
		return domain.SSHDeviceKey{}, classify(err)
	}
	return sshDeviceKeyDomain(row), nil
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
		if err := tx.Delete(&row).Error; err != nil {
			return err
		}
		removed = sshDeviceKeyDomain(row)
		return nil
	})
	return removed, classify(err)
}

func (s *Store) CreateProject(ctx context.Context, value domain.Project) error {
	return s.CreateProjectWithMemberships(ctx, value, nil)
}

func (s *Store) CreateProjectWithMemberships(ctx context.Context, value domain.Project, personIDs []string) error {
	kind, remote, err := sourceColumns(value.Source)
	if err != nil {
		return err
	}
	return classify(s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&Project{ID: value.ID, Slug: value.Slug, Name: value.Name, UnixUser: value.UnixUser, Profile: string(value.Profile), SourceKind: kind, SourceRemoteURL: remote}).Error; err != nil {
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

func (s *Store) Memberships(ctx context.Context, projectID string) ([]domain.Membership, error) {
	var rows []Membership
	if err := s.db.WithContext(ctx).Where("project_id = ?", projectID).Order("person_id").Find(&rows).Error; err != nil {
		return nil, err
	}
	values := make([]domain.Membership, 0, len(rows))
	for _, row := range rows {
		values = append(values, domain.Membership{ProjectID: row.ProjectID, PersonID: row.PersonID})
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

func (s *Store) CreateJob(ctx context.Context, value domain.ProvisioningJob) error {
	return classify(s.db.WithContext(ctx).Create(jobRow(value)).Error)
}

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
