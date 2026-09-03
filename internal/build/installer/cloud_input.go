package installer

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/LevitateOS/soda-os/internal/config"
	"github.com/LevitateOS/soda-os/internal/process"
)

type CloudDataSource string

const (
	CloudNoCloud     CloudDataSource = "nocloud"
	CloudConfigDrive CloudDataSource = "configdrive"
)

type CloudInputOptions struct {
	DataSource           CloudDataSource
	Username             string
	SSHPublicKeyPath     string
	TailscaleAuthKeyPath string
	PasswordPath         string
	OutputPath           string
}

// CloudInputBuilder writes one protected cloud-init seed ISO for one of the
// two supported upstream local datasources. It never contacts a cloud API.
type CloudInputBuilder struct {
	input *InstallerInputBuilder
}

func NewCloudInputBuilder(spec config.DistroSpec, runner process.Runner, passwordReader PasswordReader) *CloudInputBuilder {
	return &CloudInputBuilder{input: NewInstallerInputBuilder(spec, runner, passwordReader)}
}

func (b *CloudInputBuilder) Build(ctx context.Context, options CloudInputOptions) (string, error) {
	if err := b.input.validateArchitecture(); err != nil {
		return "", err
	}
	if err := validateCloudDataSource(options.DataSource); err != nil {
		return "", err
	}
	if err := validateInstallerInputOutput(options.OutputPath); err != nil {
		return "", err
	}
	if !installerInputUsernamePattern.MatchString(options.Username) {
		return "", errors.New("administrator username does not match the Soda account contract")
	}
	publicKey, err := b.input.readCanonicalPublicKey(ctx, options.SSHPublicKeyPath)
	if err != nil {
		return "", err
	}
	tailscaleKey, err := readProtectedSecret(options.TailscaleAuthKeyPath, "Tailscale auth key")
	if err != nil {
		return "", err
	}
	password, err := b.input.administratorPassword(options.PasswordPath)
	if err != nil {
		return "", err
	}
	files, label, err := cloudInputFiles(options.DataSource, options.Username, password, publicKey, tailscaleKey)
	if err != nil {
		return "", err
	}
	if err := b.input.createProtectedInputISO(ctx, options.OutputPath, label, files); err != nil {
		return "", err
	}
	return options.OutputPath, nil
}

func cloudInputFiles(source CloudDataSource, username string, password, publicKey, tailscaleKey []byte) (map[string][]byte, string, error) {
	instanceID, err := cloudInstanceID()
	if err != nil {
		return nil, "", err
	}
	userData := cloudUserData(username, string(password), string(publicKey), string(tailscaleKey))
	switch source {
	case CloudNoCloud:
		return map[string][]byte{
			"meta-data": []byte("instance-id: " + instanceID + "\nlocal-hostname: soda\n"),
			"user-data": userData,
		}, "CIDATA", nil
	case CloudConfigDrive:
		metadata := fmt.Sprintf(`{"uuid":%q,"hostname":"soda","dsmode":"local"}`+"\n", instanceID)
		return map[string][]byte{
			"openstack/latest/meta_data.json": []byte(metadata),
			"openstack/latest/user_data":      userData,
		}, "CONFIG-2", nil
	default:
		return nil, "", errors.New("cloud input datasource must be nocloud or configdrive")
	}
}

func validateCloudDataSource(source CloudDataSource) error {
	if source != CloudNoCloud && source != CloudConfigDrive {
		return errors.New("cloud input datasource must be nocloud or configdrive")
	}
	return nil
}

func cloudInstanceID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", errors.New("generate cloud instance identity")
	}
	return "soda-cloud-" + hex.EncodeToString(bytes), nil
}

func cloudUserData(username, password, publicKey, tailscaleKey string) []byte {
	return []byte(strings.Join([]string{
		"#cloud-config",
		"bootcmd:",
		"  - [ /usr/bin/install, -d, -o, root, -g, root, -m, '0700', /var/lib/soda-install/cloud ]",
		"users:",
		"  - name: " + yamlQuoted(username),
		"    groups: [wheel]",
		"    shell: /bin/bash",
		"    lock_passwd: false",
		"    ssh_authorized_keys:",
		"      - " + yamlQuoted(publicKey),
		"chpasswd:",
		"  expire: false",
		"  users:",
		"    - {name: " + yamlQuoted(username) + ", password: " + yamlQuoted(password) + ", type: text}",
		"write_files:",
		cloudWriteFile("administrator-username", username),
		cloudWriteFile("administrator-password", password),
		cloudWriteFile("administrator-authorized-key", publicKey),
		cloudWriteFile("tailscale-auth-key", tailscaleKey),
		"runcmd:",
		"  - [ /usr/libexec/soda/soda-cloud-finalize ]",
		"",
	}, "\n"))
}

func cloudWriteFile(name, value string) string {
	return strings.Join([]string{
		"  - path: /var/lib/soda-install/cloud/" + name,
		"    owner: root:root",
		"    permissions: '0600'",
		"    content: |",
		"      " + value,
	}, "\n")
}

func yamlQuoted(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
