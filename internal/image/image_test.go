package image

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LevitateOS/soda-os/internal/config"
	"github.com/stretchr/testify/require"
)

func TestApplyGladeOverrideReplacesDirectProperty(t *testing.T) {
	xml := `<interface><object class="GtkWindow" id="window"><property name="title">Old</property><child><object class="GtkLabel" id="child"><property name="title">Nested</property></object></child></object></interface>`
	changed, err := applyGladeOverride(xml, gladeOverride{ObjectID: "window", Property: "title", Value: "Soda & More"})
	require.NoError(t, err)
	require.Contains(t, changed, ">Soda &amp; More</property>")
	require.Contains(t, changed, ">Nested</property>")
}

func TestApplyGladeOverrideAddsProperty(t *testing.T) {
	xml := "<interface>\n  <object class=\"GtkBox\" id=\"box\">\n  </object>\n</interface>"
	changed, err := applyGladeOverride(xml, gladeOverride{ObjectID: "box", Property: "visible", Value: "False"})
	require.NoError(t, err)
	require.Contains(t, changed, `<property name="visible">False</property>`)
}

func TestApplyGladeOverrideHandlesNestedSelfClosingObject(t *testing.T) {
	xml := `<interface><object class="GtkWindow" id="window"><property name="title">Old</property><child><object class="GtkSizeGroup" id="size"/></child></object></interface>`
	changed, err := applyGladeOverride(xml, gladeOverride{ObjectID: "window", Property: "title", Value: "Soda"})
	require.NoError(t, err)
	require.Contains(t, changed, ">Soda</property>")
}

func TestRewriteTreeinfoProducesBootOnlyIdentity(t *testing.T) {
	source := "[checksums]\nimages/efiboot.img = sha256:old\n\n[general]\nfamily = Rocky Linux\nname = Rocky Linux 10.2\npackagedir = AppStream/Packages\nrepository = AppStream\nvariants = AppStream,BaseOS\nversion = 10.2\n\n[release]\nname = Rocky Linux\nshort = Rocky\nversion = 10.2\n\n[tree]\narch = aarch64\nvariants = AppStream,BaseOS\n\n[variant-BaseOS]\nname = BaseOS\n"
	result := rewriteTreeinfo(source, "efi", "product", "0.2.0")
	require.Contains(t, result, "images/product.img = sha256:product")
	require.Contains(t, result, "family = Soda OS")
	require.Contains(t, result, "short = SodaOS")
	require.NotContains(t, result, "variant-BaseOS")
	require.NotContains(t, result, "AppStream")
	require.NotContains(t, result, "Rocky")
}

func TestManifestContainsExactPackage(t *testing.T) {
	manifest := "kernel-6.12.aarch64\nkernel-core-6.12.aarch64\n"
	require.True(t, manifestContainsPackage(manifest, "kernel"))
	require.True(t, manifestContainsPackage(manifest, "kernel-core"))
	require.False(t, manifestContainsPackage(manifest, "kern"))
}

func TestDockerCommandUsesVersionedImageAndPrivileges(t *testing.T) {
	builder := &Builder{Root: "/workspace/soda", Spec: config.DistroSpec{Identity: config.IdentitySpec{Version: "0.2.0"}}}
	command := builder.dockerCommand(true, []string{"CGO_ENABLED=1"}, "go", "build", "./cmd/sodad")
	require.Equal(t, "docker", command.Name)
	require.Equal(t, []string{"run", "--rm", "--platform", "linux/arm64", "--volume", "/workspace/soda:/src", "--workdir", "/src", "--privileged", "--env", "CGO_ENABLED=1", "soda-os-builder:0.2.0", "go", "build", "./cmd/sodad"}, command.Args)
}

func TestCheckUsesSpecDerivedVolumeID(t *testing.T) {
	root := t.TempDir()
	for _, file := range []string{"source.iso", "checksums", "signature", "branding.toml", "upstream.toml", "packaging/anaconda/product/etc/anaconda/profile.d/sodaos.conf", "packaging/anaconda/product/.buildstamp", "packaging/anaconda/product/etc/os-release", "packaging/anaconda/product/usr/lib/os-release", "packaging/anaconda/product/usr/share/anaconda/pixmaps/soda.css", "packaging/anaconda/grub.cfg", "packaging/rpm/soda-installer-branding.spec", "packaging/kickstart/interactive.ks", "packaging/kickstart/automated.ks"} {
		writeTestFile(t, root, file, "ok")
	}
	writeTestFile(t, root, "packaging/anaconda/product/etc/anaconda/profile.d/sodaos.conf", "profile_id = sodaos\nbase_profile = rocky\nefi_dir = rocky\ncustom_stylesheet = /usr/share/anaconda/pixmaps/soda.css\nuser (quality 1, length 6)\nLangsupportSpoke SourceSpoke SoftwareSelectionSpoke KdumpSpoke PasswordSpoke CustomPartitioningSpoke BlivetGuiSpoke FilterSpoke\n")
	writeTestFile(t, root, "packaging/kickstart/interactive.ks", validKickstartText("graphical"))
	writeTestFile(t, root, "packaging/kickstart/automated.ks", validKickstartText("text"))
	writeTestFile(t, root, "branding.toml", "schema_version = 1\n")
	writeTestFile(t, root, "upstream.toml", validUpstreamTOML())
	builder := &Builder{Root: root, runner: &RecordingRunner{}, Spec: testSpec("SodaOS-0-2-0-aarch64")}
	require.NoError(t, builder.Check(context.Background()))
	builder.Spec.Installer.VolumeID = "SodaOS-0-0-0-aarch64"
	require.ErrorContains(t, builder.Check(context.Background()), "unexpected installer volume ID")
}

func TestBuildGoBinariesUsesAllGoEntrypoints(t *testing.T) {
	runner := &RecordingRunner{}
	builder := &Builder{Root: "/workspace/soda", runner: runner, Spec: config.DistroSpec{Identity: config.IdentitySpec{Version: "0.2.0"}}}
	require.NoError(t, builder.buildGoBinaries(context.Background()))
	commands := make([]string, 0, len(runner.Commands))
	for _, command := range runner.Commands {
		commands = append(commands, command.String())
	}
	require.Len(t, commands, 6)
	require.Contains(t, strings.Join(commands, "\n"), "./cmd/sodad")
	require.Contains(t, strings.Join(commands, "\n"), "./cmd/sodactl")
	require.Contains(t, strings.Join(commands, "\n"), "./cmd/soda-ssh")
	require.Contains(t, strings.Join(commands, "\n"), "./cmd/soda-image")
	require.Contains(t, strings.Join(commands, "\n"), "./cockpit/cmd/soda-cockpit")
	require.Contains(t, strings.Join(commands, "\n"), "./cockpit/cmd/soda-authd")
	require.Contains(t, strings.Join(commands, "\n"), "-buildvcs=false")
	require.Contains(t, strings.Join(commands, "\n"), "internal/version.Version=0.2.0")
	require.NotContains(t, strings.Join(commands, "\n"), "cargo")
}

func TestRuntimePackagesRootOwnedProjectAuthorizedKeys(t *testing.T) {
	configContents, err := os.ReadFile(filepath.Join("..", "..", "packaging", "sshd", "41-soda-project-accounts.conf"))
	require.NoError(t, err)
	configuration := string(configContents)
	require.Contains(t, configuration, "Match User soda-p-*")
	require.Contains(t, configuration, "AuthorizedKeysFile /etc/soda/authorized_keys/%u")
	require.Contains(t, configuration, "Match all")

	specContents, err := os.ReadFile(filepath.Join("..", "..", "packaging", "rpm", "soda-runtime.spec"))
	require.NoError(t, err)
	spec := string(specContents)
	require.Contains(t, spec, "%attr(0755,root,root) %{_sysconfdir}/soda/authorized_keys")
	require.Contains(t, spec, "ssh_home_t '/etc/soda/authorized_keys(/.*)?'")
	require.Contains(t, spec, "restorecon -RF /etc/soda/authorized_keys")
}

func TestReplaceFileReplacesReadOnlyExtractedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "grub.cfg")
	require.NoError(t, os.WriteFile(path, []byte("upstream"), 0o444))

	require.NoError(t, replaceFile(path, []byte("soda"), 0o644))
	contents, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "soda", string(contents))
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o644), info.Mode().Perm())
}

func testSpec(volumeID string) config.DistroSpec {
	return config.DistroSpec{
		Identity:  config.IdentitySpec{Name: "Soda OS", Version: "0.2.0", Architecture: "aarch64"},
		Base:      config.BaseSpec{Distribution: "rocky", InstallerSourceVersion: "10.2", PackageStream: "10", SourceISO: "source.iso", ChecksumFile: "checksums", SignatureFile: "signature"},
		Installer: config.InstallerSpec{ProfileID: "sodaos", AnacondaGUINEVRA: "anaconda-gui-40.22.3.46-1.el10.rocky.0.6.aarch64", VolumeID: volumeID, BrandingManifest: "branding.toml", UpstreamManifest: "upstream.toml", BootTimeoutSeconds: 10, Payload: config.NetworkPayloadSpec{Mode: "network", BaseOSMirrorlist: baseOSMirrorlist, AppStreamMirrorlist: appStreamMirrorlist, MaxISOSizeBytes: 1342177280, Environment: "minimal-environment", Packages: payloadPackages, AutomatedExtraPackages: automatedExtraPackages, AnacondaRequiredPackages: anacondaRequiredPackages}},
	}
}

func validKickstartText(mode string) string {
	return mode + "\nurl --mirrorlist=\"" + baseOSMirrorlist + "\"\nrepo --name=AppStream --mirrorlist=\"" + appStreamMirrorlist + "\"\n%packages --exclude-weakdeps\nfile:///run/install/repo/soda/\n"
}

func validUpstreamTOML() string {
	return "schema_version = 1\nanaconda_gui_nevra = \"anaconda-gui-40.22.3.46-1.el10.rocky.0.6.aarch64\"\n\n[spokes]\nvisible = [\"WelcomeLanguageSpoke\", \"KeyboardSpoke\", \"DatetimeSpoke\", \"StorageSpoke\", \"NetworkSpoke\", \"UserSpoke\"]\nhidden = [\"LangsupportSpoke\", \"SourceSpoke\", \"SoftwareSelectionSpoke\", \"KdumpSpoke\", \"PasswordSpoke\", \"CustomPartitioningSpoke\", \"BlivetGuiSpoke\", \"FilterSpoke\"]\n\n[[glade]]\npath = \"usr/share/anaconda/ui/spokes/welcome.glade\"\nsha256 = \"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef\"\n\n[[glade.override]]\nobject_id = \"welcome\"\nproperty = \"label\"\nvalue = \"Soda\"\n\n[[glade]]\npath = \"usr/share/anaconda/ui/spokes/storage.glade\"\nsha256 = \"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef\"\n\n[[glade.override]]\nobject_id = \"storage\"\nproperty = \"label\"\nvalue = \"Soda\"\n\n[[glade]]\npath = \"usr/share/anaconda/ui/spokes/network.glade\"\nsha256 = \"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef\"\n\n[[glade.override]]\nobject_id = \"network\"\nproperty = \"label\"\nvalue = \"Soda\"\n\n[[glade]]\npath = \"usr/share/anaconda/ui/spokes/user.glade\"\nsha256 = \"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef\"\n\n[[glade.override]]\nobject_id = \"user\"\nproperty = \"label\"\nvalue = \"Soda\"\n"
}

func writeTestFile(t *testing.T, root, path, contents string) {
	t.Helper()
	fullPath := filepath.Join(root, path)
	require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0o755))
	require.NoError(t, os.WriteFile(fullPath, []byte(contents), 0o644))
}
