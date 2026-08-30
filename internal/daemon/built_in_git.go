package daemon

import (
	"context"
	"errors"
	"sort"

	"github.com/LevitateOS/soda-os/internal/builtingit"
	"github.com/LevitateOS/soda-os/internal/domain"
	"github.com/LevitateOS/soda-os/internal/store"
)

func (s *Service) ensureBuiltInGitPerson(ctx context.Context, person domain.Person, kind builtingit.PersonKind) error {
	if s.builtInGit == nil {
		return nil
	}
	if _, err := s.store.BuiltInGitUser(ctx, person.ID); err == nil {
		return nil
	} else if !errors.Is(err, store.ErrNotFound) {
		return err
	}
	user, err := s.builtInGit.EnsurePerson(ctx, person, kind)
	if err != nil {
		return err
	}
	return s.store.SaveBuiltInGitUser(ctx, domain.BuiltInGitUser{PersonID: person.ID, UserID: user.ID})
}

func (s *Service) ensureBuiltInGitKey(ctx context.Context, person domain.Person, key domain.SSHDeviceKey) error {
	if s.builtInGit == nil {
		return nil
	}
	if _, err := s.store.BuiltInGitKey(ctx, key.ID); err == nil {
		return nil
	} else if !errors.Is(err, store.ErrNotFound) {
		return err
	}
	remote, err := s.builtInGit.EnsureKey(ctx, person, key)
	if err != nil {
		return err
	}
	return s.store.SaveBuiltInGitKey(ctx, domain.BuiltInGitKey{SSHDeviceKeyID: key.ID, PersonID: person.ID, KeyID: remote.ID})
}

func (s *Service) ensureBuiltInGitIdentity(ctx context.Context, person domain.Person, identity domain.GitIdentity) error {
	if s.builtInGit == nil {
		return nil
	}
	if _, err := s.store.BuiltInGitIdentity(ctx, person.ID); err == nil {
		return nil
	} else if !errors.Is(err, store.ErrNotFound) {
		return err
	}
	remote, err := s.builtInGit.EnsureGitIdentity(ctx, person, identity)
	if err != nil {
		return err
	}
	return s.store.SaveBuiltInGitIdentity(ctx, domain.BuiltInGitIdentity{PersonID: person.ID, KeyID: remote.ID})
}

func (s *Service) ensureBuiltInGitProject(ctx context.Context, project domain.Project) error {
	if s.builtInGit == nil {
		return nil
	}
	if _, builtIn := project.Source.(domain.EmptyProjectSource); !builtIn {
		return nil
	}
	members, err := s.store.Collaborators(ctx, project.ID)
	if err != nil {
		return err
	}
	if err = s.ensureBuiltInGitMembers(ctx, members); err != nil {
		return err
	}
	deployKey, err := s.host.DeployPublicKey(ctx, project)
	if err != nil {
		return err
	}
	repository, err := s.builtInGit.EnsureRepository(ctx, project, members, deployKey)
	if err != nil {
		return err
	}
	if err = s.saveBuiltInGitRepository(ctx, project.ID, repository); err != nil {
		return err
	}
	return s.host.ConnectBuiltInRepository(ctx, project, repository.SSHURL)
}

func (s *Service) ensureBuiltInGitMembers(ctx context.Context, members []domain.Person) error {
	for _, person := range members {
		if err := s.reconcileBuiltInGitPerson(ctx, person, builtingit.PersonMember); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) reconcileBuiltInGitPerson(ctx context.Context, person domain.Person, kind builtingit.PersonKind) error {
	if err := s.ensureBuiltInGitPerson(ctx, person, kind); err != nil {
		return err
	}
	identity, err := s.store.GitIdentity(ctx, person.ID)
	if err != nil {
		return err
	}
	if err = s.ensureBuiltInGitIdentity(ctx, person, identity); err != nil {
		return err
	}
	keys, err := s.store.SSHDeviceKeys(ctx, person.ID)
	if err != nil {
		return err
	}
	for _, key := range keys {
		if err = s.ensureBuiltInGitKey(ctx, person, key); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) saveBuiltInGitRepository(ctx context.Context, projectID string, repository builtingit.Repository) error {
	if _, err := s.store.BuiltInGitRepository(ctx, projectID); err == nil {
		return nil
	} else if !errors.Is(err, store.ErrNotFound) {
		return err
	}
	return s.store.SaveBuiltInGitRepository(ctx, domain.BuiltInGitRepository{ProjectID: projectID, RepositoryID: repository.ID, DeployKeyID: repository.DeployKeyID, WebURL: repository.WebURL, SSHURL: repository.SSHURL})
}

func (s *Service) ReconcileAllBuiltInGit(ctx context.Context) error {
	if s.builtInGit == nil {
		return nil
	}
	people, err := s.store.People(ctx)
	if err != nil {
		return err
	}
	sort.SliceStable(people, func(left, right int) bool {
		return people[left].Role == domain.RoleAdmin && people[right].Role != domain.RoleAdmin
	})
	for _, person := range people {
		if err = s.reconcileBuiltInGitPerson(ctx, person, builtInGitPersonKind(person)); err != nil {
			return err
		}
	}
	return s.reconcileBuiltInGitProjects(ctx)
}

func (s *Service) reconcileBuiltInGitProjects(ctx context.Context) error {
	projects, err := s.store.Projects(ctx)
	if err != nil {
		return err
	}
	for _, project := range projects {
		if err = s.ensureBuiltInGitProject(ctx, project); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ReconcileAllAccess(ctx context.Context) error {
	if err := s.ReconcileAllAuthorizedKeys(ctx); err != nil {
		return err
	}
	if err := s.ReconcileAllBuiltInGit(ctx); err != nil {
		s.logger.Warn("Built-in Git reconciliation failed; local and external project access remains available", "error", err)
	}
	return nil
}
