package installer

import (
	"context"
	"errors"
	"regexp"
	"runtime"

	"github.com/LevitateOS/soda-os/internal/config"
	"github.com/LevitateOS/soda-os/internal/process"
)

const installerInputKickstartPrefix = `%include /usr/share/anaconda/interactive-defaults.ks
`

const installerInputUnattendedCommands = `# Fixed destructive storage automation for disposable acceptance VMs only.
cmdline
lang en_US.UTF-8
keyboard us
timezone UTC --utc
network --bootproto=dhcp --device=link --activate --onboot=on --hostname=soda-acceptance
zerombr
clearpart --all --initlabel
autopart --type=plain --fstype=ext4
eula --agreed
reboot
`

const installerInputKickstartScripts = `%pre --erroronfail
/usr/libexec/soda/soda-installer-input
%end
%include /run/soda-installer/account.ks
%post --nochroot --erroronfail
/usr/libexec/soda/soda-installer-finalize
%end
`

var installerInputUsernamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,23}$`)

type InstallerInputOptions struct {
	ISOPath              string
	ReleaseRecordPath    string
	Username             string
	SSHPublicKeyPath     string
	TailscaleAuthKeyPath string
	PasswordPath         string
	OutputPath           string
	Unattended           bool
}

type PasswordReader func(string) ([]byte, error)

type InstallerInputBuilder struct {
	Spec             config.DistroSpec
	hostArchitecture string
	runner           process.Runner
	passwordReader   PasswordReader
}

func NewInstallerInputBuilder(spec config.DistroSpec, runner process.Runner, passwordReader PasswordReader) *InstallerInputBuilder {
	if runner == nil {
		runner = process.OSRunner{}
	}
	if passwordReader == nil {
		passwordReader = readPasswordFromTerminal
	}
	return &InstallerInputBuilder{
		Spec:             spec,
		hostArchitecture: runtime.GOARCH,
		runner:           runner,
		passwordReader:   passwordReader,
	}
}

func (b *InstallerInputBuilder) Build(ctx context.Context, options InstallerInputOptions) (string, error) {
	if err := b.validateArchitecture(); err != nil {
		return "", err
	}
	if err := validateInstallerInputOutput(options.OutputPath); err != nil {
		return "", err
	}
	if err := validateInstallerInputArtifact(options.ISOPath, options.ReleaseRecordPath, b.Spec.Platform.Architecture.Platform); err != nil {
		return "", err
	}
	if !installerInputUsernamePattern.MatchString(options.Username) {
		return "", errors.New("administrator username does not match the Soda account contract")
	}
	publicKey, err := b.readCanonicalPublicKey(ctx, options.SSHPublicKeyPath)
	if err != nil {
		return "", err
	}
	tailscaleAuthKey, err := readProtectedSecret(options.TailscaleAuthKeyPath, "Tailscale auth key")
	if err != nil {
		return "", err
	}
	password, err := b.administratorPassword(options.PasswordPath)
	if err != nil {
		return "", err
	}

	files := map[string][]byte{
		"ks.cfg":                            []byte(options.kickstart()),
		"soda/administrator-username":       []byte(options.Username),
		"soda/administrator-password":       password,
		"soda/administrator-authorized-key": publicKey,
		"soda/tailscale-auth-key":           tailscaleAuthKey,
	}
	if err := b.createMedia(ctx, options.OutputPath, files); err != nil {
		return "", err
	}
	return options.OutputPath, nil
}

func (o InstallerInputOptions) kickstart() string {
	kickstart := installerInputKickstartPrefix
	if o.Unattended {
		kickstart += installerInputUnattendedCommands
	}
	return kickstart + installerInputKickstartScripts
}

func (b *InstallerInputBuilder) validateArchitecture() error {
	architecture := b.Spec.Platform.Architecture
	if b.Spec.Identity.Architecture != architecture.Name || b.Spec.Base.Platform != architecture.Platform {
		return errors.New("installer input architecture differs from the selected Soda platform")
	}
	hostArchitecture := b.hostArchitecture
	if hostArchitecture == "" {
		hostArchitecture = runtime.GOARCH
	}
	return config.RequireNativeHostArchitecture(architecture.Name, hostArchitecture)
}
