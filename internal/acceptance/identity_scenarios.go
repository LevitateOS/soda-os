package acceptance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"
)

type forgejoKey struct {
	Key string `json:"key"`
}

func verifyWorkspaceGitKeys(ctx context.Context, project projectFixture) error {
	for _, workspace := range []workspaceFixture{project.Admin, project.Alice, project.Bob} {
		if err := verifyForgejoKeys(ctx, workspace); err != nil {
			return err
		}
	}
	return nil
}

func verifyForgejoKeys(ctx context.Context, workspace workspaceFixture) error {
	workspacePublic, err := workspace.Remote.Output(ctx, nil, "cat", ".ssh/id_ed25519_soda.pub")
	if err != nil {
		return err
	}
	person := workspace.Person
	keys, err := forgejoKeys(ctx, person.Remote, person.Remote.Username, person.ForgejoPassword)
	if err != nil {
		return err
	}
	if err = requireForgejoKey(keys, workspacePublic, "manually registered workspace", person.Remote.Username); err != nil {
		return err
	}
	return rejectForgejoKey(keys, person.PublicKey, "Linux", person.Remote.Username)
}

func requireForgejoKey(keys []forgejoKey, expected []byte, label, username string) error {
	found, err := containsPublicKey(keys, expected)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("Forgejo account %s does not contain its %s public key", username, label)
	}
	return nil
}

func rejectForgejoKey(keys []forgejoKey, rejected []byte, label, username string) error {
	found, err := containsPublicKey(keys, rejected)
	if err != nil {
		return err
	}
	if found {
		return fmt.Errorf("PAM-created Forgejo account %s unexpectedly contains its %s public key", username, label)
	}
	return nil
}

const forgejoLoopbackEndpoint = "http://127.0.0.1:30000"

func registerForgejoKey(ctx context.Context, person personFixture, publicKey []byte, evidence string) error {
	remote, password := person.Remote, person.ForgejoPassword
	payload, err := json.Marshal(map[string]string{
		"key":   strings.TrimSpace(string(publicKey)),
		"title": "Soda OS acceptance " + evidence,
	})
	if err != nil {
		return err
	}
	config := fmt.Sprintf(
		"user = %s\nsilent\nshow-error\nfail-with-body\nurl = %s\n",
		curlConfigQuote(remote.Username+":"+string(bytes.TrimSpace(password))),
		curlConfigQuote(forgejoLoopbackEndpoint+"/api/v1/user/keys"),
	)
	_, err = remote.CaptureOutput(ctx, evidence, []byte(config), "curl", "--config", "-", "--json", string(payload))
	return err
}

func workspacePublicKeyFromDiagnostic(diagnostic []byte) ([]byte, error) {
	const algorithm = "ssh-ed25519 "
	start := bytes.Index(diagnostic, []byte(algorithm))
	if start < 0 {
		return nil, errors.New("workspace setup failure did not report its outbound Git public key")
	}
	material := diagnostic[start+len(algorithm):]
	end := 0
	for end < len(material) && isBase64KeyByte(material[end]) {
		end++
	}
	if end == 0 {
		return nil, errors.New("workspace setup failure reported an invalid outbound Git public key")
	}
	key := []byte(algorithm + string(material[:end]))
	canonical, err := canonicalPublicKey(key)
	if err != nil {
		return nil, fmt.Errorf("validate reported workspace public key: %w", err)
	}
	return []byte(canonical), nil
}

func isBase64KeyByte(value byte) bool {
	return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z' || value >= '0' && value <= '9' || value == '+' || value == '/' || value == '='
}

func containsPublicKey(keys []forgejoKey, expected []byte) (bool, error) {
	material, err := canonicalPublicKey(expected)
	if err != nil {
		return false, err
	}
	for _, key := range keys {
		actual, keyErr := canonicalPublicKey([]byte(key.Key))
		if keyErr == nil && actual == material {
			return true, nil
		}
	}
	return false, nil
}

func forgejoKeys(ctx context.Context, remote Remote, username string, password []byte) ([]forgejoKey, error) {
	config := fmt.Sprintf("user = %s\nsilent\nshow-error\nfail-with-body\nurl = %s\n", curlConfigQuote(username+":"+string(bytes.TrimSpace(password))), curlConfigQuote(forgejoLoopbackEndpoint+"/api/v1/user/keys"))
	output, err := remote.CaptureOutput(ctx, "product/"+username+"-forgejo-keys", []byte(config), "curl", "--config", "-")
	if err != nil {
		return nil, err
	}
	var keys []forgejoKey
	if err := json.Unmarshal(output, &keys); err != nil {
		return nil, err
	}
	return keys, nil
}

func canonicalPublicKey(contents []byte) (string, error) {
	key, _, options, rest, err := ssh.ParseAuthorizedKey(contents)
	if err != nil || len(options) != 0 || len(bytes.TrimSpace(rest)) != 0 {
		return "", errors.New("expected one ordinary OpenSSH public key")
	}
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key))), nil
}

func verifyOneTimeAuthorizedKeys(ctx context.Context, alice workspaceFixture, keys fixtureKeys) error {
	key, err := keys.generate(ctx, "alice-later")
	if err != nil {
		return err
	}
	if err = alice.Person.Remote.Capture(ctx, "product/alice-new-authorized-key", key.Public, "tee", "-a", ".ssh/authorized_keys"); err != nil {
		return err
	}
	script := `if grep --fixed-strings --line-regexp --file=- "$HOME/.ssh/authorized_keys"; then exit 1; fi`
	return alice.Remote.Capture(ctx, "product/workspace-key-copy-once", key.Public, "/bin/bash", "-c", script)
}
