package daemon

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/LevitateOS/soda-os/internal/builtingit"
	"github.com/LevitateOS/soda-os/internal/domain"
	sodav2 "github.com/LevitateOS/soda-os/internal/gen/soda/v2"
	"github.com/LevitateOS/soda-os/internal/host"
	"github.com/LevitateOS/soda-os/internal/store"
	"github.com/google/uuid"
)

func (s *Service) CreatePerson(ctx context.Context, request *sodav2.CreatePersonRequest) (*sodav2.CreatePersonResponse, error) {
	role, err := roleDomain(request.GetRole())
	if err != nil {
		return nil, rpcError(err)
	}
	if err = validatePerson(request.GetUsername(), request.GetDisplayName(), request.GetEmail()); err != nil {
		return nil, rpcError(err)
	}
	if request.GetPassword() == "" {
		return nil, rpcError(invalid("password is required"))
	}
	if utf8.RuneCountInString(request.GetPassword()) < 6 {
		return nil, rpcError(invalid("password must contain at least 6 characters"))
	}
	if strings.ContainsAny(request.GetPassword(), "\r\n\x00") {
		return nil, rpcError(invalid("password contains a line or NUL delimiter"))
	}
	if err = s.store.PreflightPerson(ctx, request.GetUsername()); err != nil {
		return nil, rpcError(err)
	}
	person := domain.Person{ID: uuid.NewString(), Username: request.GetUsername(), DisplayName: request.GetDisplayName(), Email: request.GetEmail(), Role: role}
	cleanup, err := s.host.CreatePerson(ctx, person, request.GetPassword())
	if err != nil {
		return nil, rpcError(err)
	}
	if err = s.persistPerson(ctx, person, cleanup); err != nil {
		return nil, rpcError(err)
	}
	return &sodav2.CreatePersonResponse{Person: personProto(person)}, nil
}
func (s *Service) ImportPerson(ctx context.Context, request *sodav2.ImportPersonRequest) (*sodav2.ImportPersonResponse, error) {
	role, err := roleDomain(request.GetRole())
	if err != nil {
		return nil, rpcError(err)
	}
	if err = validatePerson(request.GetUsername(), request.GetDisplayName(), request.GetEmail()); err != nil {
		return nil, rpcError(err)
	}
	if err = s.store.PreflightPerson(ctx, request.GetUsername()); err != nil {
		return nil, rpcError(err)
	}
	person := domain.Person{ID: uuid.NewString(), Username: request.GetUsername(), DisplayName: request.GetDisplayName(), Email: request.GetEmail(), Role: role}
	cleanup, err := s.host.ImportPerson(ctx, person)
	if err != nil {
		return nil, rpcError(err)
	}
	if err = s.persistPerson(ctx, person, cleanup); err != nil {
		return nil, rpcError(err)
	}
	return &sodav2.ImportPersonResponse{Person: personProto(person)}, nil
}

func builtInGitPersonKind(person domain.Person) builtingit.PersonKind {
	if person.Role == domain.RoleAdmin {
		return builtingit.PersonAdministrator
	}
	return builtingit.PersonMember
}

func (s *Service) persistPerson(ctx context.Context, person domain.Person, cleanup host.Cleanup) error {
	if err := s.store.CreatePerson(ctx, person); err != nil {
		return s.compensate(ctx, err, cleanup, "person", person.Username)
	}
	if err := s.ensureBuiltInGitPerson(ctx, person, builtInGitPersonKind(person)); err != nil {
		deleteErr := s.store.DeleteFreshPerson(context.WithoutCancel(ctx), person.ID)
		if deleteErr == nil {
			deleteErr = s.runCleanup(ctx, cleanup)
		}
		return errors.Join(err, deleteErr)
	}
	return nil
}
func (s *Service) ListPeople(ctx context.Context, _ *sodav2.ListPeopleRequest) (*sodav2.ListPeopleResponse, error) {
	values, err := s.store.People(ctx)
	if err != nil {
		return nil, rpcError(err)
	}
	response := &sodav2.ListPeopleResponse{People: make([]*sodav2.Person, 0, len(values))}
	for _, value := range values {
		response.People = append(response.People, personProto(value))
	}
	return response, nil
}

func (s *Service) CreateSshDeviceKey(ctx context.Context, request *sodav2.CreateSshDeviceKeyRequest) (*sodav2.CreateSshDeviceKeyResponse, error) {
	personID, err := parseID(request.GetPersonId(), "person")
	if err != nil {
		return nil, rpcError(err)
	}
	if _, err = s.store.Person(ctx, personID); err != nil {
		return nil, rpcError(err)
	}
	key, err := deviceKeyRegistration(personID, request)
	if err != nil {
		return nil, rpcError(err)
	}
	if err = s.store.CreateSSHDeviceKey(ctx, key); err != nil {
		return nil, rpcError(err)
	}
	person, err := s.store.Person(ctx, personID)
	if err == nil {
		err = s.ensureBuiltInGitKey(ctx, person, key)
	}
	if err != nil {
		_, rollbackErr := s.store.DeleteSSHDeviceKey(context.WithoutCancel(ctx), personID, key.ID)
		return nil, rpcError(errors.Join(err, rollbackErr))
	}
	if err = s.reconcilePersonAccess(ctx, personID); err != nil {
		_, rollbackErr := s.store.DeleteSSHDeviceKey(context.WithoutCancel(ctx), personID, key.ID)
		if rollbackErr == nil {
			rollbackErr = s.reconcilePersonAccess(context.WithoutCancel(ctx), personID)
		}
		err = errors.Join(err, rollbackErr)
		return nil, rpcError(err)
	}
	return &sodav2.CreateSshDeviceKeyResponse{Key: sshDeviceKeyProto(key)}, nil
}

func deviceKeyRegistration(personID string, request *sodav2.CreateSshDeviceKeyRequest) (domain.SSHDeviceKey, error) {
	label := strings.TrimSpace(request.GetLabel())
	if label == "" || len(label) > 40 || strings.ContainsAny(label, "\r\n\x00") {
		return domain.SSHDeviceKey{}, invalid("device label must contain 1 to 40 characters")
	}
	hint := strings.TrimSpace(request.GetIdentityFileHint())
	if hint == "" || len(hint) > 255 || strings.ContainsAny(hint, "\r\n\x00") {
		return domain.SSHDeviceKey{}, invalid("identity file path hint must be a single value of at most 255 characters")
	}
	publicKey, fingerprint, err := domain.ParseSSHKey(strings.TrimSpace(request.GetPublicKey()))
	if err != nil {
		return domain.SSHDeviceKey{}, invalid("%v", err)
	}
	return domain.SSHDeviceKey{ID: uuid.NewString(), PersonID: personID, Label: label, PublicKey: publicKey, Fingerprint: fingerprint, IdentityFileHint: hint, CreatedAt: time.Now().UTC()}, nil
}

func (s *Service) ListSshDeviceKeys(ctx context.Context, request *sodav2.ListSshDeviceKeysRequest) (*sodav2.ListSshDeviceKeysResponse, error) {
	personID, err := parseID(request.GetPersonId(), "person")
	if err != nil {
		return nil, rpcError(err)
	}
	if _, err = s.store.Person(ctx, personID); err != nil {
		return nil, rpcError(err)
	}
	keys, err := s.store.SSHDeviceKeys(ctx, personID)
	if err != nil {
		return nil, rpcError(err)
	}
	response := &sodav2.ListSshDeviceKeysResponse{Keys: make([]*sodav2.SshDeviceKey, 0, len(keys))}
	for _, key := range keys {
		response.Keys = append(response.Keys, sshDeviceKeyProto(key))
	}
	return response, nil
}

func (s *Service) RevokeSshDeviceKey(ctx context.Context, request *sodav2.RevokeSshDeviceKeyRequest) (*sodav2.RevokeSshDeviceKeyResponse, error) {
	personID, err := parseID(request.GetPersonId(), "person")
	if err != nil {
		return nil, rpcError(err)
	}
	keyID, err := parseID(request.GetKeyId(), "SSH device key")
	if err != nil {
		return nil, rpcError(err)
	}
	person, err := s.store.Person(ctx, personID)
	if err != nil {
		return nil, rpcError(err)
	}
	mapping, mappingErr := s.store.BuiltInGitKey(ctx, keyID)
	if mappingErr != nil && !errors.Is(mappingErr, store.ErrNotFound) {
		return nil, rpcError(mappingErr)
	}
	key, err := s.store.DeleteSSHDeviceKey(ctx, personID, keyID)
	if err != nil {
		return nil, rpcError(err)
	}
	if err = s.reconcilePersonAccess(ctx, personID); err != nil {
		rollbackErr := s.store.CreateSSHDeviceKey(context.WithoutCancel(ctx), key)
		if rollbackErr == nil {
			rollbackErr = s.reconcilePersonAccess(context.WithoutCancel(ctx), personID)
		}
		return nil, rpcError(errors.Join(err, rollbackErr))
	}
	if err = s.revokeBuiltInGitKey(ctx, person, key, mapping, mappingErr); err != nil {
		return nil, rpcError(err)
	}
	return &sodav2.RevokeSshDeviceKeyResponse{Key: sshDeviceKeyProto(key)}, nil
}

func (s *Service) revokeBuiltInGitKey(ctx context.Context, person domain.Person, key domain.SSHDeviceKey, mapping domain.BuiltInGitKey, mappingErr error) error {
	if errors.Is(mappingErr, store.ErrNotFound) {
		return nil
	}
	if mappingErr != nil {
		return mappingErr
	}
	if err := s.builtInGit.DeleteKey(ctx, person.Username, mapping.KeyID); err != nil {
		rollbackErr := s.store.CreateSSHDeviceKey(context.WithoutCancel(ctx), key)
		if rollbackErr == nil {
			rollbackErr = s.store.SaveBuiltInGitKey(context.WithoutCancel(ctx), mapping)
		}
		if rollbackErr == nil {
			rollbackErr = s.reconcilePersonAccess(context.WithoutCancel(ctx), person.ID)
		}
		return errors.Join(err, rollbackErr)
	}
	return nil
}
