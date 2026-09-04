package projects

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"
)

type preparedPerson struct {
	username   string
	password   string
	publicKey  string
	forgejoURL string
}

func (coordinator Coordinator) addPerson(ctx context.Context, request AddPersonRequest) (MutationResponse, error) {
	person, err := coordinator.preparePerson(ctx, request)
	if err != nil {
		return MutationResponse{}, err
	}
	if err = coordinator.createPersonLinuxAccount(ctx, person); err != nil {
		return MutationResponse{}, err
	}
	if err = coordinator.publishPerson(ctx, person); err != nil {
		return MutationResponse{}, err
	}
	if err = coordinator.Forgejo.RegisterPublicKey(ctx, ForgejoKeyRequest{
		BaseURL: person.forgejoURL, Username: person.username, Password: person.password, PublicKey: person.publicKey,
	}); err != nil {
		return MutationResponse{}, fmt.Errorf("Linux account %s and its SSH key were retained; Forgejo account or key may be incomplete: %w", person.username, err)
	}
	person.password = ""
	request.Password = ""
	return MutationResponse{OK: true}, nil
}

func (coordinator Coordinator) preparePerson(ctx context.Context, request AddPersonRequest) (preparedPerson, error) {
	if err := ValidatePrimaryUsername(request.Username); err != nil {
		return preparedPerson{}, err
	}
	if err := validateHumanPassword(request.Password); err != nil {
		return preparedPerson{}, err
	}
	key, err := CanonicalAuthorizedKey(request.AuthorizedKey)
	if err != nil {
		return preparedPerson{}, err
	}
	return preparedPerson{request.Username, request.Password, key, coordinator.ForgejoAPIURL}, nil
}

func (coordinator Coordinator) createPersonLinuxAccount(ctx context.Context, person preparedPerson) error {
	err := coordinator.Privileged.HumanCreate(ctx, HelperHumanCreateRequest{
		Username: person.username, Password: person.password,
	})
	return err
}

func (coordinator Coordinator) publishPerson(ctx context.Context, person preparedPerson) error {
	if err := coordinator.Privileged.HumanPublish(ctx, HelperHumanPublishRequest{
		Username: person.username, AuthorizedKey: person.publicKey,
	}); err != nil {
		return fmt.Errorf("Linux account %s was retained without its SSH key: %w", person.username, err)
	}
	return nil
}

func validateHumanPassword(password string) error {
	if password == "" || len(password) > 4096 {
		return errors.New("password must contain between 1 and 4096 bytes")
	}
	if strings.ContainsAny(password, "\x00\r\n") {
		return errors.New("password must not contain NUL, CR, or LF")
	}
	return nil
}

func CanonicalAuthorizedKey(input string) (string, error) {
	publicKey, _, options, rest, err := ssh.ParseAuthorizedKey([]byte(input))
	if err != nil || len(options) != 0 || len(bytes.TrimSpace(rest)) != 0 {
		return "", errors.New("authorized key must contain exactly one valid OpenSSH public key")
	}
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(publicKey))), nil
}
