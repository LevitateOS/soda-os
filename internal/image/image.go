// Package image implements Soda's deterministic installer and RPM build pipeline.
package image

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/LevitateOS/soda-os/internal/config"
)

var (
	targetRPMs               = []string{"soda-release", "soda-runtime", "soda-cockpit"}
	baseOSMirrorlist         = "https://mirrors.rockylinux.org/mirrorlist?arch=aarch64&repo=BaseOS-10"
	appStreamMirrorlist      = "https://mirrors.rockylinux.org/mirrorlist?arch=aarch64&repo=AppStream-10"
	payloadPackages          = []string{"avahi", "git", "openssh-server", "soda-release", "soda-runtime", "soda-cockpit", "sudo"}
	automatedExtraPackages   = []string{"curl"}
	anacondaRequiredPackages = []string{"kernel", "grub2", "grub2-tools", "grub2-efi-aa64", "grub2-efi-aa64-cdboot", "shim-aa64"}
	requiredFirmwarePackages = []string{"linux-firmware", "amd-gpu-firmware", "intel-gpu-firmware", "nvidia-gpu-firmware"}
	isoRootAllowlist         = []string{".discinfo", ".treeinfo", "COMMUNITY-CHARTER", "EFI", "EULA", "LICENSE", "RPM-GPG-KEY-Rocky-10", "boot.catalog", "images", "ks.cfg", "soda"}
)

type brandingManifest struct {
	SchemaVersion uint32       `toml:"schema_version"`
	Asset         []brandAsset `toml:"asset"`
}

type brandAsset struct {
	Source string `toml:"source"`
	Output string `toml:"output"`
	Width  uint32 `toml:"width"`
	Height uint32 `toml:"height"`
	SHA256 string `toml:"sha256"`
}

type upstreamManifest struct {
	SchemaVersion    uint32        `toml:"schema_version"`
	AnacondaGUINEVRA string        `toml:"anaconda_gui_nevra"`
	AnacondaGUIRPM   string        `toml:"anaconda_gui_rpm"`
	Spokes           spokeContract `toml:"spokes"`
	Glade            []gladeSpec   `toml:"glade"`
}

type spokeContract struct {
	Visible []string `toml:"visible"`
	Hidden  []string `toml:"hidden"`
}

type gladeSpec struct {
	Path      string          `toml:"path"`
	SHA256    string          `toml:"sha256"`
	Overrides []gladeOverride `toml:"override"`
}

type gladeOverride struct {
	ObjectID string `toml:"object_id"`
	Property string `toml:"property"`
	Value    string `toml:"value"`
}

// Builder owns no system state. It only writes disposable artifacts beneath Root.
type Builder struct {
	Root   string
	Spec   config.DistroSpec
	runner Runner
}

func NewBuilderFromWorkingDirectory(specPath string, runner Runner) (*Builder, error) {
	root, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}
	return NewBuilder(root, specPath, runner)
}

func NewBuilder(root, specPath string, runner Runner) (*Builder, error) {
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("canonicalize workspace: %w", err)
	}
	if !filepath.IsAbs(specPath) {
		specPath = filepath.Join(canonicalRoot, specPath)
	}
	spec, err := config.LoadDistro(specPath)
	if err != nil {
		return nil, err
	}
	if runner == nil {
		runner = OSRunner{}
	}
	return &Builder{Root: canonicalRoot, Spec: spec, runner: runner}, nil
}

func (b *Builder) imageName() string {
	return "soda-os-builder:" + b.Spec.Identity.Version
}

func (b *Builder) artifactPath(parts ...string) string {
	return filepath.Join(append([]string{b.Root, ".artifacts"}, parts...)...)
}

// Check validates every input which does not require Docker, the source ISO, or network access.
func (b *Builder) Check(_ context.Context) error {
	spec := b.Spec
	if spec.Identity.Architecture != "aarch64" {
		return errors.New("only AArch64 image builds are supported")
	}
	if spec.Base.Distribution != "rocky" || spec.Base.InstallerSourceVersion != "10.2" || spec.Base.PackageStream != "10" {
		return errors.New("Soda OS requires the Rocky 10.2 installer runtime and Rocky 10 package stream")
	}
	if spec.Installer.ProfileID != "sodaos" {
		return errors.New("installer profile must be sodaos")
	}
	if spec.Installer.VolumeID != expectedVolumeID(spec.Identity.Version) {
		return fmt.Errorf("unexpected installer volume ID %q; expected %q", spec.Installer.VolumeID, expectedVolumeID(spec.Identity.Version))
	}
	if spec.Installer.BootTimeoutSeconds != 10 {
		return errors.New("installer boot timeout must be 10 seconds")
	}
	payload := spec.Installer.Payload
	if payload.Mode != "network" || payload.BaseOSMirrorlist != baseOSMirrorlist || payload.AppStreamMirrorlist != appStreamMirrorlist {
		return errors.New("Rocky network sources differ from the approved mirrorlists")
	}
	if payload.InstallWeakDependencies {
		return errors.New("weak RPM dependencies must remain disabled")
	}
	if payload.MaxISOSizeBytes != 1342177280 {
		return errors.New("compact ISO size limit must be 1.25 GiB")
	}
	if payload.Environment != "minimal-environment" || !sameStrings(payload.Packages, payloadPackages) || !sameStrings(payload.AutomatedExtraPackages, automatedExtraPackages) || !sameStrings(payload.AnacondaRequiredPackages, anacondaRequiredPackages) {
		return errors.New("network payload roots differ from the approved package contract")
	}
	for _, path := range []string{spec.Base.SourceISO, spec.Base.ChecksumFile, spec.Base.SignatureFile, spec.Installer.BrandingManifest, spec.Installer.UpstreamManifest} {
		if !isFile(b.path(path)) {
			return fmt.Errorf("required input %s is missing", path)
		}
	}
	branding, err := b.brandingManifest()
	if err != nil {
		return err
	}
	if branding.SchemaVersion != 1 {
		return errors.New("unsupported branding manifest")
	}
	for _, asset := range branding.Asset {
		if !isFile(b.path(asset.Source)) || !isFile(b.path(asset.Output)) {
			return fmt.Errorf("branding asset %s or %s is missing", asset.Source, asset.Output)
		}
		width, height, err := pngDimensions(b.path(asset.Output))
		if err != nil {
			return err
		}
		if width != asset.Width || height != asset.Height {
			return fmt.Errorf("%s is %dx%d; expected %dx%d", asset.Output, width, height, asset.Width, asset.Height)
		}
		digest, err := sha256File(b.path(asset.Output))
		if err != nil {
			return err
		}
		if digest != asset.SHA256 {
			return fmt.Errorf("%s does not match its recorded SHA-256", asset.Output)
		}
	}
	upstream, err := b.upstreamManifest()
	if err != nil {
		return err
	}
	if err := validateUpstreamContract(spec, upstream); err != nil {
		return err
	}
	profile, err := os.ReadFile(b.path("packaging/anaconda/product/etc/anaconda/profile.d/sodaos.conf"))
	if err != nil {
		return err
	}
	profileText := string(profile)
	for _, required := range []string{"profile_id = sodaos", "base_profile = rocky", "efi_dir = rocky", "custom_stylesheet = /usr/share/anaconda/pixmaps/soda.css", "user (quality 1, length 6)"} {
		if !strings.Contains(profileText, required) {
			return errors.New("Soda Anaconda profile is incomplete")
		}
	}
	if strings.Contains(profileText, "strict") {
		return errors.New("Soda Anaconda profile contains strict password policy")
	}
	for _, spoke := range upstream.Spokes.Hidden {
		if !strings.Contains(profileText, spoke) {
			return fmt.Errorf("profile does not hide %s", spoke)
		}
	}
	for _, path := range []string{"packaging/anaconda/product/.buildstamp", "packaging/anaconda/product/etc/os-release", "packaging/anaconda/product/usr/lib/os-release", "packaging/anaconda/product/usr/share/anaconda/pixmaps/soda.css", "packaging/anaconda/grub.cfg", "packaging/rpm/soda-installer-branding.spec"} {
		if !isFile(b.path(path)) {
			return fmt.Errorf("required overlay %s is missing", path)
		}
	}
	for path, mode := range map[string]string{"packaging/kickstart/interactive.ks": "graphical", "packaging/kickstart/automated.ks": "text"} {
		kickstart, err := os.ReadFile(b.path(path))
		if err != nil {
			return err
		}
		if !validKickstart(string(kickstart), mode, payload) {
			return fmt.Errorf("%s does not match the compact network payload contract", path)
		}
	}
	fmt.Printf("%s %s installer contract is valid\n", spec.Identity.Name, spec.Identity.Version)
	return nil
}

func validateUpstreamContract(spec config.DistroSpec, upstream upstreamManifest) error {
	if upstream.SchemaVersion != 1 || upstream.AnacondaGUINEVRA != spec.Installer.AnacondaGUINEVRA {
		return errors.New("Anaconda package pin differs between distro and upstream manifests")
	}
	visible := []string{"WelcomeLanguageSpoke", "KeyboardSpoke", "DatetimeSpoke", "StorageSpoke", "NetworkSpoke", "UserSpoke"}
	hidden := []string{"LangsupportSpoke", "SourceSpoke", "SoftwareSelectionSpoke", "KdumpSpoke", "PasswordSpoke", "CustomPartitioningSpoke", "BlivetGuiSpoke", "FilterSpoke"}
	if !sameStrings(upstream.Spokes.Visible, visible) || !sameStrings(upstream.Spokes.Hidden, hidden) {
		return errors.New("Anaconda spoke contract differs from the approved allowlist")
	}
	if len(upstream.Glade) != 4 {
		return errors.New("expected four Glade overlays")
	}
	for _, glade := range upstream.Glade {
		if !strings.HasPrefix(glade.Path, "usr/share/anaconda/ui/spokes/") || filepath.Ext(glade.Path) != ".glade" || len(glade.SHA256) != 64 || !hexDigest(glade.SHA256) || len(glade.Overrides) == 0 {
			return fmt.Errorf("invalid Glade contract for %s", glade.Path)
		}
	}
	return nil
}

func (b *Builder) Verify(ctx context.Context) error {
	if err := b.Check(ctx); err != nil {
		return err
	}
	if err := b.buildContainer(ctx); err != nil {
		return err
	}
	checksum := b.containerPath(b.path(b.Spec.Base.ChecksumFile))
	signature := b.containerPath(b.path(b.Spec.Base.SignatureFile))
	if err := b.docker(ctx, false, nil, "sq", "verify", "--signer-file", "/etc/pki/rpm-gpg/RPM-GPG-KEY-Rocky-10", "--signature-file", signature, checksum); err != nil {
		return err
	}
	signed, err := os.ReadFile(b.path(b.Spec.Base.ChecksumFile))
	if err != nil {
		return err
	}
	if !strings.Contains(string(signed), b.Spec.Base.SourceISOSHA256) {
		return errors.New("signed checksum file does not contain the configured ISO digest")
	}
	output, err := b.dockerOutput(ctx, false, nil, "sha256sum", b.containerPath(b.path(b.Spec.Base.SourceISO)))
	if err != nil {
		return err
	}
	if fields := strings.Fields(output); len(fields) == 0 || fields[0] != b.Spec.Base.SourceISOSHA256 {
		return fmt.Errorf("source ISO checksum mismatch: expected %s, got %s", b.Spec.Base.SourceISOSHA256, strings.TrimSpace(output))
	}
	fmt.Printf("Verified Rocky %s %s source ISO and release signature\n", b.Spec.Base.InstallerSourceVersion, b.Spec.Identity.Architecture)
	return nil
}

// BuildRPMs creates the target repository and the build-only installer branding RPM.
func (b *Builder) BuildRPMs(ctx context.Context) error {
	if err := b.Verify(ctx); err != nil {
		return err
	}
	build := b.artifactPath("build")
	topdir := b.artifactPath("rpmbuild")
	repo := b.artifactPath("soda")
	installer := b.artifactPath("installer")
	if err := os.MkdirAll(build, 0o755); err != nil {
		return err
	}
	for _, path := range []string{topdir, repo, installer} {
		if err := recreate(path); err != nil {
			return err
		}
	}
	for _, directory := range []string{"BUILD", "BUILDROOT", "RPMS", "SOURCES", "SPECS", "SRPMS"} {
		if err := os.MkdirAll(filepath.Join(topdir, directory), 0o755); err != nil {
			return err
		}
	}
	productRoot, err := b.prepareProductImage(ctx, installer)
	if err != nil {
		return err
	}
	if err := b.buildGoBinaries(ctx); err != nil {
		return err
	}
	sources := filepath.Join(topdir, "SOURCES")
	if err := b.stageRPMSources(build, sources, productRoot); err != nil {
		return err
	}
	for _, name := range targetRPMs {
		if err := b.rpmbuild(ctx, name); err != nil {
			return err
		}
		rpm, err := findSingleRPM(filepath.Join(topdir, "RPMS"), name)
		if err != nil {
			return err
		}
		if err := copyFile(rpm, filepath.Join(repo, filepath.Base(rpm))); err != nil {
			return err
		}
	}
	if err := b.docker(ctx, false, nil, "createrepo_c", "--update", "/src/.artifacts/soda"); err != nil {
		return err
	}
	if err := b.validateTargetRPMs(ctx, repo); err != nil {
		return err
	}
	if err := b.rpmbuild(ctx, "soda-installer-branding"); err != nil {
		return err
	}
	brandingRPM, err := findSingleRPM(filepath.Join(topdir, "RPMS"), "soda-installer-branding")
	if err != nil {
		return err
	}
	brandingDir := filepath.Join(installer, "rpms")
	if err := os.MkdirAll(brandingDir, 0o755); err != nil {
		return err
	}
	brandingOutput := filepath.Join(brandingDir, filepath.Base(brandingRPM))
	if err := copyFile(brandingRPM, brandingOutput); err != nil {
		return err
	}
	listing, err := b.dockerOutput(ctx, false, nil, "rpm", "-qpl", b.containerPath(brandingOutput))
	if err != nil {
		return err
	}
	if !strings.Contains(listing, "/usr/share/soda-installer/product/etc/anaconda/profile.d/sodaos.conf") || !strings.Contains(listing, "/usr/share/soda-installer/manifests/upstream.toml") {
		return errors.New("installer-branding RPM payload is incomplete")
	}
	entries, err := os.ReadDir(repo)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "soda-installer-branding-") {
			return errors.New("build-only installer RPM leaked into the target repository")
		}
	}
	fmt.Printf("Built Soda target RPM repository at %s\n", repo)
	fmt.Printf("Built build-only installer RPM at %s\n", brandingOutput)
	return nil
}

func (b *Builder) buildGoBinaries(ctx context.Context) error {
	linkerFlags := "-s -w -buildid= -X github.com/LevitateOS/soda-os/internal/version.Version=" + b.Spec.Identity.Version
	for _, target := range []struct{ output, pkg string }{
		{"sodad", "./cmd/sodad"},
		{"sodactl", "./cmd/sodactl"},
		{"soda-ssh", "./cmd/soda-ssh"},
		{"soda-image", "./cmd/soda-image"},
		{"soda-cockpit", "./cockpit/cmd/soda-cockpit"},
		{"soda-authd", "./cockpit/cmd/soda-authd"},
	} {
		if err := b.docker(ctx, false, []string{"CGO_ENABLED=1"}, "go", "build", "-buildvcs=false", "-trimpath", "-ldflags="+linkerFlags, "-o", "/src/.artifacts/build/"+target.output, target.pkg); err != nil {
			return err
		}
	}
	return nil
}

func (b *Builder) stageRPMSources(build, sources, productRoot string) error {
	files := [][2]string{
		{filepath.Join(build, "sodad"), filepath.Join(sources, "sodad")},
		{filepath.Join(build, "sodactl"), filepath.Join(sources, "sodactl")},
		{filepath.Join(build, "soda-ssh"), filepath.Join(sources, "soda-ssh")},
		{filepath.Join(build, "soda-cockpit"), filepath.Join(sources, "soda-cockpit")},
		{filepath.Join(build, "soda-authd"), filepath.Join(sources, "soda-authd")},
		{b.path("packaging/systemd/sodad.service"), filepath.Join(sources, "sodad.service")},
		{b.path("packaging/sshd/40-soda-observability.conf"), filepath.Join(sources, "40-soda-observability.conf")},
		{b.path("packaging/systemd/soda-cockpit.service"), filepath.Join(sources, "soda-cockpit.service")},
		{b.path("packaging/systemd/soda-authd.service"), filepath.Join(sources, "soda-authd.service")},
		{b.path("packaging/avahi/soda-cockpit.service"), filepath.Join(sources, "soda-cockpit.avahi.service")},
		{b.path("packaging/pam/soda-cockpit"), filepath.Join(sources, "soda-cockpit.pam")},
		{b.path("packaging/anaconda/product/usr/share/doc/soda-installer/BASE_SYSTEM.md"), filepath.Join(sources, "BASE_SYSTEM.md")},
		{b.path("assets/branding/source/soda-symbol.svg"), filepath.Join(sources, "soda-symbol.svg")},
		{b.path("assets/branding/installer/soda-symbol-256.png"), filepath.Join(sources, "soda-symbol-256.png")},
	}
	for _, size := range []string{"16", "24", "32", "48", "64", "128", "256", "512"} {
		files = append(files, [2]string{b.path("assets/branding/icons/hicolor/" + size + "x" + size + "/apps/soda-os.png"), filepath.Join(sources, "soda-os-"+size+".png")})
	}
	for _, pair := range files {
		if err := copyFile(pair[0], pair[1]); err != nil {
			return err
		}
	}
	if err := copyTree(productRoot, filepath.Join(sources, "soda-installer-product")); err != nil {
		return err
	}
	for _, pair := range [][2]string{{b.path(b.Spec.Installer.BrandingManifest), filepath.Join(sources, "branding.toml")}, {b.path(b.Spec.Installer.UpstreamManifest), filepath.Join(sources, "upstream.toml")}} {
		if err := copyFile(pair[0], pair[1]); err != nil {
			return err
		}
	}
	return nil
}

// BuildISO creates either the interactive or disposable automated installer image.
func (b *Builder) BuildISO(ctx context.Context, automated bool) error {
	if err := b.BuildRPMs(ctx); err != nil {
		return err
	}
	images := b.artifactPath("images")
	overlay := b.artifactPath("iso-overlay")
	if err := os.MkdirAll(images, 0o755); err != nil {
		return err
	}
	if err := recreate(overlay); err != nil {
		return err
	}
	for _, path := range []string{filepath.Join(overlay, "EFI", "BOOT"), filepath.Join(overlay, "images")} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return err
		}
	}
	suffix := ""
	kickstart := "packaging/kickstart/interactive.ks"
	if automated {
		suffix, kickstart = "-test", "packaging/kickstart/automated.ks"
	}
	output := filepath.Join(images, "SodaOS-"+b.Spec.Identity.Version+"-aarch64"+suffix+".iso")
	if err := os.Remove(output); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := b.docker(ctx, false, nil, "ksvalidator", "-v", "RHEL10", "/src/"+kickstart); err != nil {
		return err
	}
	if err := b.resolveNetworkPayload(ctx, automated); err != nil {
		return err
	}
	return b.assembleISO(ctx, overlay, output, kickstart, automated)
}

func (b *Builder) assembleISO(ctx context.Context, overlay, output, kickstart string, automated bool) error {
	sourceISO := b.containerPath(b.path(b.Spec.Base.SourceISO))
	metadata := b.artifactPath("source-metadata")
	if err := recreate(metadata); err != nil {
		return err
	}
	if err := b.extractISOFile(ctx, sourceISO, "/.treeinfo", filepath.Join(metadata, "treeinfo")); err != nil {
		return err
	}
	if err := b.extractISOFile(ctx, sourceISO, "/.discinfo", filepath.Join(metadata, "discinfo")); err != nil {
		return err
	}
	efiStage := b.artifactPath("efi-stage")
	if err := recreate(efiStage); err != nil {
		return err
	}
	if err := b.docker(ctx, false, nil, "xorriso", "-osirrox", "on", "-indev", sourceISO, "-extract", "/EFI", b.containerPath(filepath.Join(efiStage, "EFI"))); err != nil {
		return err
	}
	grub, err := b.renderGrub()
	if err != nil {
		return err
	}
	for _, path := range []string{filepath.Join(efiStage, "EFI", "BOOT", "grub.cfg"), filepath.Join(overlay, "EFI", "BOOT", "grub.cfg")} {
		if err := replaceFile(path, []byte(grub), 0o644); err != nil {
			return err
		}
	}
	background := b.path("assets/branding/installer/grub-background.png")
	for _, destination := range []string{filepath.Join(efiStage, "EFI", "BOOT", "soda-grub-background.png"), filepath.Join(overlay, "EFI", "BOOT", "soda-grub-background.png")} {
		if err := copyFile(background, destination); err != nil {
			return err
		}
	}
	efiBoot := filepath.Join(overlay, "images", "efiboot.img")
	if err := b.docker(ctx, true, nil, "mkefiboot", "--label=SODAOS", b.containerPath(filepath.Join(efiStage, "EFI", "BOOT")), b.containerPath(efiBoot)); err != nil {
		return err
	}
	if err := copyFile(b.artifactPath("installer", "product.img"), filepath.Join(overlay, "images", "product.img")); err != nil {
		return err
	}
	if err := copyFile(b.path(kickstart), filepath.Join(overlay, "ks.cfg")); err != nil {
		return err
	}
	if err := copyTree(b.artifactPath("soda"), filepath.Join(overlay, "soda")); err != nil {
		return err
	}
	efiDigest, err := sha256File(efiBoot)
	if err != nil {
		return err
	}
	productDigest, err := sha256File(filepath.Join(overlay, "images", "product.img"))
	if err != nil {
		return err
	}
	treeinfo, err := os.ReadFile(filepath.Join(metadata, "treeinfo"))
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(overlay, ".treeinfo"), []byte(rewriteTreeinfo(string(treeinfo), efiDigest, productDigest, b.Spec.Identity.Version)), 0o644); err != nil {
		return err
	}
	discinfo, err := os.ReadFile(filepath.Join(metadata, "discinfo"))
	if err != nil {
		return err
	}
	timestamp := "0"
	if lines := strings.Split(string(discinfo), "\n"); len(lines) > 0 && lines[0] != "" {
		timestamp = lines[0]
	}
	if err := os.WriteFile(filepath.Join(overlay, ".discinfo"), []byte(fmt.Sprintf("%s\nSoda OS %s\naarch64\nALL\n", timestamp, b.Spec.Identity.Version)), 0o644); err != nil {
		return err
	}
	args := []string{"xorriso", "-indev", sourceISO, "-outdev", b.containerPath(output), "-boot_image", "any", "replay", "-volid", b.Spec.Installer.VolumeID, "-rm_r", "/BaseOS", "/AppStream", "--", "-rm", "/media.repo", "/extra_files.json", "/RPM-GPG-KEY-Rocky-10-Testing", "--"}
	for _, update := range [][2]string{{filepath.Join(overlay, ".treeinfo"), "/.treeinfo"}, {filepath.Join(overlay, ".discinfo"), "/.discinfo"}, {filepath.Join(overlay, "EFI", "BOOT", "grub.cfg"), "/EFI/BOOT/grub.cfg"}, {efiBoot, "/images/efiboot.img"}} {
		args = append(args, "-update", b.containerPath(update[0]), update[1])
	}
	for _, mapped := range [][2]string{{filepath.Join(overlay, "EFI", "BOOT", "soda-grub-background.png"), "/EFI/BOOT/soda-grub-background.png"}, {filepath.Join(overlay, "images", "product.img"), "/images/product.img"}, {filepath.Join(overlay, "ks.cfg"), "/ks.cfg"}, {filepath.Join(overlay, "soda"), "/soda"}} {
		args = append(args, "-map", b.containerPath(mapped[0]), mapped[1])
	}
	if err := b.docker(ctx, true, nil, args[0], args[1:]...); err != nil {
		return err
	}
	if err := b.docker(ctx, true, nil, "implantisomd5", "--force", b.containerPath(output)); err != nil {
		return err
	}
	info, err := os.Stat(output)
	if err != nil {
		return err
	}
	if uint64(info.Size()) > b.Spec.Installer.Payload.MaxISOSizeBytes {
		return fmt.Errorf("compact ISO is %d bytes; maximum is %d bytes", info.Size(), b.Spec.Installer.Payload.MaxISOSizeBytes)
	}
	if err := b.inspectISO(ctx, output, automated); err != nil {
		return err
	}
	digest, err := b.dockerOutput(ctx, false, nil, "sha256sum", b.containerPath(output))
	if err != nil {
		return err
	}
	fields := strings.Fields(digest)
	if len(fields) == 0 {
		return errors.New("sha256sum did not return a digest")
	}
	if err := os.WriteFile(strings.TrimSuffix(output, ".iso")+".iso.sha256", []byte(fields[0]+"\n"), 0o644); err != nil {
		return err
	}
	fmt.Printf("Built %s (%s)\n", output, fields[0])
	return nil
}

func (b *Builder) prepareProductImage(ctx context.Context, installer string) (string, error) {
	productRoot := filepath.Join(installer, "product-root")
	upstreamRoot := filepath.Join(installer, "upstream-root")
	for _, path := range []string{productRoot, upstreamRoot} {
		if err := recreate(path); err != nil {
			return "", err
		}
	}
	if err := copyTree(b.path("packaging/anaconda/product"), productRoot); err != nil {
		return "", err
	}
	manifest, err := b.upstreamManifest()
	if err != nil {
		return "", err
	}
	upstreamRPM := filepath.Join(installer, "anaconda-gui.rpm")
	if err := b.extractISOFile(ctx, b.containerPath(b.path(b.Spec.Base.SourceISO)), "/"+manifest.AnacondaGUIRPM, upstreamRPM); err != nil {
		return "", err
	}
	nevra, err := b.dockerOutput(ctx, false, nil, "rpm", "-qp", "--qf", "%{NAME}-%{VERSION}-%{RELEASE}.%{ARCH}", b.containerPath(upstreamRPM))
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(nevra) != manifest.AnacondaGUINEVRA {
		return "", fmt.Errorf("expected %s, extracted %s", manifest.AnacondaGUINEVRA, strings.TrimSpace(nevra))
	}
	if err := b.docker(ctx, false, nil, "bash", "-c", "cd \"$1\" && rpm2cpio \"$2\" | cpio -idm --quiet", "soda-extract", b.containerPath(upstreamRoot), b.containerPath(upstreamRPM)); err != nil {
		return "", err
	}
	for _, contract := range manifest.Glade {
		source := filepath.Join(upstreamRoot, contract.Path)
		digest, err := sha256File(source)
		if err != nil {
			return "", fmt.Errorf("upstream Glade %s is missing: %w", contract.Path, err)
		}
		if digest != contract.SHA256 {
			return "", fmt.Errorf("upstream Glade %s changed; refusing to apply an unreviewed overlay", contract.Path)
		}
		xml, err := os.ReadFile(source)
		if err != nil {
			return "", err
		}
		updated := string(xml)
		for _, change := range contract.Overrides {
			updated, err = applyGladeOverride(updated, change)
			if err != nil {
				return "", fmt.Errorf("apply %s.%s in %s: %w", change.ObjectID, change.Property, contract.Path, err)
			}
		}
		destination := filepath.Join(productRoot, contract.Path)
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(destination, []byte(updated), 0o644); err != nil {
			return "", err
		}
		if err := b.docker(ctx, false, nil, "xmllint", "--noout", b.containerPath(destination)); err != nil {
			return "", err
		}
	}
	productImage := filepath.Join(installer, "product.img")
	if err := b.docker(ctx, false, nil, "bash", "-c", "cd \"$1\" && find . -exec touch -h -d @0 {} + && find . -print0 | LC_ALL=C sort -z | cpio --null -o --format=newc --owner=0:0 --reproducible --quiet | xz -9e > \"$2\"", "soda-product-image", b.containerPath(productRoot), b.containerPath(productImage)); err != nil {
		return "", err
	}
	listing, err := b.dockerOutput(ctx, false, nil, "bash", "-c", "xz -dc \"$1\" | cpio -it --quiet", "soda-product-inspect", b.containerPath(productImage))
	if err != nil {
		return "", err
	}
	for _, required := range []string{".buildstamp", "etc/anaconda/profile.d/sodaos.conf", "etc/os-release", "usr/lib/os-release", "usr/share/anaconda/pixmaps/soda.css", "usr/share/anaconda/ui/spokes/welcome.glade", "usr/share/anaconda/ui/spokes/storage.glade", "usr/share/anaconda/ui/spokes/network.glade", "usr/share/anaconda/ui/spokes/user.glade"} {
		if !strings.Contains(listing, required) {
			return "", fmt.Errorf("product.img is missing %s", required)
		}
	}
	fmt.Printf("Built and inspected %s\n", productImage)
	return productRoot, nil
}

func (b *Builder) renderGrub() (string, error) {
	template, err := os.ReadFile(b.path("packaging/anaconda/grub.cfg"))
	if err != nil {
		return "", err
	}
	rendered := strings.NewReplacer(
		"@BOOT_TIMEOUT@", fmt.Sprint(b.Spec.Installer.BootTimeoutSeconds),
		"@VOLUME_ID@", b.Spec.Installer.VolumeID,
		"@VOLUME_ID_ESCAPED@", b.Spec.Installer.VolumeID,
		"@PROFILE_ID@", b.Spec.Installer.ProfileID,
		"@VERSION@", b.Spec.Identity.Version,
	).Replace(string(template))
	if strings.Contains(rendered, "@") {
		return "", errors.New("unresolved GRUB template marker")
	}
	if strings.Contains(rendered, "Rocky") || strings.Contains(rendered, "FIPS") {
		return "", errors.New("forbidden boot menu identity")
	}
	return rendered, nil
}

func (b *Builder) inspectISO(ctx context.Context, iso string, automated bool) error {
	inspect := b.artifactPath("iso-inspect")
	if err := recreate(inspect); err != nil {
		return err
	}
	isoPath := b.containerPath(iso)
	for _, extraction := range [][2]string{{"/.treeinfo", "treeinfo"}, {"/.discinfo", "discinfo"}, {"/EFI/BOOT/grub.cfg", "grub.cfg"}, {"/ks.cfg", "ks.cfg"}, {"/images/product.img", "product.img"}} {
		if err := b.extractISOFile(ctx, isoPath, extraction[0], filepath.Join(inspect, extraction[1])); err != nil {
			return err
		}
	}
	report, err := b.dockerCombinedOutput(ctx, false, nil, "xorriso", "-indev", isoPath, "-pvd_info", "-report_el_torito", "plain", "-report_system_area", "plain")
	if err != nil {
		return err
	}
	if !strings.Contains(report, "Volume id    : '"+b.Spec.Installer.VolumeID+"'") {
		return errors.New("ISO volume ID differs from the installer contract")
	}
	if !strings.Contains(report, "EFI") && !strings.Contains(report, "UEFI") {
		return errors.New("ISO does not report an EFI boot image")
	}
	grub, err := os.ReadFile(filepath.Join(inspect, "grub.cfg"))
	if err != nil {
		return err
	}
	grubText := string(grub)
	if strings.Count(grubText, "menuentry '") != 4 || !strings.Contains(grubText, "inst.profile=sodaos") || !strings.Contains(grubText, "Install Soda OS "+b.Spec.Identity.Version) || strings.Contains(grubText, "Rocky") || strings.Contains(grubText, "FIPS") {
		return errors.New("boot menu contract failed")
	}
	treeinfo, err := os.ReadFile(filepath.Join(inspect, "treeinfo"))
	if err != nil {
		return err
	}
	if strings.Contains(string(treeinfo), "BaseOS") || strings.Contains(string(treeinfo), "AppStream") || strings.Contains(string(treeinfo), "[variant-") {
		return errors.New("boot-only treeinfo still advertises a local Rocky package payload")
	}
	visible, err := joinFiles(filepath.Join(inspect, "treeinfo"), filepath.Join(inspect, "discinfo"), filepath.Join(inspect, "grub.cfg"))
	if err != nil {
		return err
	}
	if strings.Contains(visible, "Rocky") {
		return errors.New("customer-visible ISO metadata still contains Rocky branding")
	}
	kickstart, err := os.ReadFile(filepath.Join(inspect, "ks.cfg"))
	if err != nil {
		return err
	}
	mode := "graphical"
	if automated {
		mode = "text"
	}
	if !validKickstart(string(kickstart), mode, b.Spec.Installer.Payload) {
		return errors.New("ISO contains the wrong Kickstart mode")
	}
	productListing, err := b.dockerOutput(ctx, false, nil, "bash", "-c", "xz -dc \"$1\" | cpio -it --quiet", "soda-product-inspect", b.containerPath(filepath.Join(inspect, "product.img")))
	if err != nil {
		return err
	}
	if !strings.Contains(productListing, "etc/anaconda/profile.d/sodaos.conf") {
		return errors.New("ISO product image is missing the Soda profile")
	}
	rootReport, err := b.dockerCombinedOutput(ctx, false, nil, "xorriso", "-indev", isoPath, "-ls", "/")
	if err != nil {
		return err
	}
	roots := quotedListingEntries(rootReport)
	sort.Strings(roots)
	expected := append([]string(nil), isoRootAllowlist...)
	sort.Strings(expected)
	if !sameStrings(roots, expected) {
		return fmt.Errorf("compact ISO root differs from the allowlist: found %v", roots)
	}
	rpmReport, err := b.dockerCombinedOutput(ctx, false, nil, "xorriso", "-indev", isoPath, "-find", "/soda", "-type", "f", "-name", "*.rpm")
	if err != nil {
		return err
	}
	rpms := quotedListingEntries(rpmReport)
	if len(rpms) != len(targetRPMs) {
		return fmt.Errorf("compact ISO does not contain exactly the three target Soda RPMs: %v", rpms)
	}
	for _, name := range targetRPMs {
		found := false
		for _, rpm := range rpms {
			if strings.HasPrefix(filepath.Base(rpm), name+"-") {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("compact ISO does not contain %s", name)
		}
	}
	info, err := os.Stat(iso)
	if err != nil {
		return err
	}
	if uint64(info.Size()) > b.Spec.Installer.Payload.MaxISOSizeBytes {
		return errors.New("compact ISO exceeds its size contract")
	}
	if err := b.docker(ctx, false, nil, "checkisomd5", isoPath); err != nil {
		return err
	}
	fmt.Printf("Inspected UEFI layout and Soda identity in %s\n", iso)
	return nil
}

func (b *Builder) resolveNetworkPayload(ctx context.Context, automated bool) error {
	repoDir := b.artifactPath("network-repos")
	manifestDir := b.artifactPath("manifests")
	if err := recreate(repoDir); err != nil {
		return err
	}
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		return err
	}
	payload := b.Spec.Installer.Payload
	repo := fmt.Sprintf("[soda-baseos]\nname=Rocky Linux 10 BaseOS\nmirrorlist=%s\nenabled=1\ngpgcheck=0\n\n[soda-appstream]\nname=Rocky Linux 10 AppStream\nmirrorlist=%s\nenabled=1\ngpgcheck=0\n\n[soda-local]\nname=Soda OS local packages\nbaseurl=file:///src/.artifacts/soda\nenabled=1\ngpgcheck=0\n", payload.BaseOSMirrorlist, payload.AppStreamMirrorlist)
	if err := os.WriteFile(filepath.Join(repoDir, "soda-network.repo"), []byte(repo), 0o644); err != nil {
		return err
	}
	roots := []string{"@" + payload.Environment}
	roots = append(roots, payload.Packages...)
	if automated {
		roots = append(roots, payload.AutomatedExtraPackages...)
	}
	roots = append(roots, payload.AnacondaRequiredPackages...)
	script := "set -euo pipefail; rm -rf /tmp/soda-network-root /tmp/soda-network-payload; mkdir -p /tmp/soda-network-root /tmp/soda-network-payload; dnf -q -y --installroot /tmp/soda-network-root --releasever \"$2\" --setopt=\"reposdir=$1\" --setopt=install_weak_deps=False --downloadonly --destdir /tmp/soda-network-payload install \"${@:3}\" >/dev/null; find /tmp/soda-network-payload -type f -name '*.rpm' -print0 | xargs -0 rpm -qp --qf '%{NAME}-%{VERSION}-%{RELEASE}.%{ARCH}\\n' | LC_ALL=C sort -u"
	args := []string{"bash", "-c", script, "soda-network-resolve", b.containerPath(repoDir), b.Spec.Base.PackageStream}
	args = append(args, roots...)
	resolved, err := b.dockerOutput(ctx, false, nil, args[0], args[1:]...)
	if err != nil {
		return err
	}
	if strings.TrimSpace(resolved) == "" {
		return errors.New("Rocky network payload resolved no RPMs")
	}
	for _, packageName := range append(append(append([]string{}, anacondaRequiredPackages...), requiredFirmwarePackages...), targetRPMs...) {
		if !manifestContainsPackage(resolved, packageName) {
			return fmt.Errorf("resolved network payload is missing %s", packageName)
		}
	}
	suffix := ""
	if automated {
		suffix = "-test"
	}
	manifest := fmt.Sprintf("baseos_mirrorlist=%s\nappstream_mirrorlist=%s\npackage_stream=%s\nweak_dependencies=false\n\n%s", payload.BaseOSMirrorlist, payload.AppStreamMirrorlist, b.Spec.Base.PackageStream, resolved)
	path := filepath.Join(manifestDir, "rocky-network-payload"+suffix+".txt")
	if err := os.WriteFile(path, []byte(manifest), 0o644); err != nil {
		return err
	}
	fmt.Printf("Resolved current Rocky %s network payload at %s\n", b.Spec.Base.PackageStream, path)
	return nil
}

func (b *Builder) validateTargetRPMs(ctx context.Context, repo string) error {
	entries, err := os.ReadDir(repo)
	if err != nil {
		return err
	}
	var rpms []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".rpm") {
			rpms = append(rpms, filepath.Join(repo, entry.Name()))
		}
	}
	sort.Strings(rpms)
	if len(rpms) != len(targetRPMs) {
		return fmt.Errorf("expected three target Soda RPMs, found %d", len(rpms))
	}
	args := []string{"dnf", "-y", "install"}
	for _, rpm := range rpms {
		args = append(args, b.containerPath(rpm))
	}
	return b.docker(ctx, false, nil, args[0], args[1:]...)
}

func (b *Builder) rpmbuild(ctx context.Context, name string) error {
	return b.docker(ctx, false, nil, "rpmbuild", "-bb", "--define", "_topdir /src/.artifacts/rpmbuild", "packaging/rpm/"+name+".spec")
}

func (b *Builder) extractISOFile(ctx context.Context, iso, source, destination string) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	if err := os.Remove(destination); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return b.docker(ctx, false, nil, "xorriso", "-osirrox", "on", "-indev", iso, "-extract", source, b.containerPath(destination))
}

func (b *Builder) brandingManifest() (brandingManifest, error) {
	var manifest brandingManifest
	return manifest, decodeTOML(b.path(b.Spec.Installer.BrandingManifest), &manifest)
}

func (b *Builder) upstreamManifest() (upstreamManifest, error) {
	var manifest upstreamManifest
	return manifest, decodeTOML(b.path(b.Spec.Installer.UpstreamManifest), &manifest)
}

func (b *Builder) buildContainer(ctx context.Context) error {
	return b.runner.Run(ctx, Command{Dir: b.Root, Name: "docker", Args: []string{"build", "--quiet", "--platform", "linux/arm64", "--file", "packaging/builder/Containerfile", "--tag", b.imageName(), "."}})
}

func (b *Builder) docker(ctx context.Context, privileged bool, environment []string, name string, args ...string) error {
	command := b.dockerCommand(privileged, environment, name, args...)
	return b.runner.Run(ctx, command)
}

func (b *Builder) dockerOutput(ctx context.Context, privileged bool, environment []string, name string, args ...string) (string, error) {
	return b.runner.Output(ctx, b.dockerCommand(privileged, environment, name, args...))
}

func (b *Builder) dockerCombinedOutput(ctx context.Context, privileged bool, environment []string, name string, args ...string) (string, error) {
	return b.runner.CombinedOutput(ctx, b.dockerCommand(privileged, environment, name, args...))
}

func (b *Builder) dockerCommand(privileged bool, environment []string, name string, args ...string) Command {
	dockerArgs := []string{"run", "--rm", "--platform", "linux/arm64", "--volume", b.Root + ":/src", "--workdir", "/src"}
	if privileged {
		dockerArgs = append(dockerArgs, "--privileged")
	}
	for _, pair := range environment {
		dockerArgs = append(dockerArgs, "--env", pair)
	}
	dockerArgs = append(dockerArgs, b.imageName(), name)
	dockerArgs = append(dockerArgs, args...)
	return Command{Dir: b.Root, Name: "docker", Args: dockerArgs, Privileged: privileged}
}

func (b *Builder) path(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(b.Root, path)
}

func (b *Builder) containerPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	relative, err := filepath.Rel(b.Root, abs)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return path
	}
	return "/src/" + filepath.ToSlash(relative)
}

func expectedVolumeID(version string) string {
	return "SodaOS-" + strings.ReplaceAll(version, ".", "-") + "-aarch64"
}

func validKickstart(kickstart, mode string, payload config.NetworkPayloadSpec) bool {
	return strings.Contains(kickstart, mode) && strings.Contains(kickstart, "url --mirrorlist=\""+payload.BaseOSMirrorlist+"\"") && strings.Contains(kickstart, "repo --name=AppStream --mirrorlist=\""+payload.AppStreamMirrorlist+"\"") && strings.Contains(kickstart, "%packages --exclude-weakdeps") && strings.Contains(kickstart, "file:///run/install/repo/soda/") && !containsLine(kickstart, "cdrom")
}

func containsLine(text, target string) bool {
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == target {
			return true
		}
	}
	return false
}

func sameStrings(actual, expected []string) bool {
	if len(actual) != len(expected) {
		return false
	}
	for i := range actual {
		if actual[i] != expected[i] {
			return false
		}
	}
	return true
}

func hexDigest(value string) bool {
	for _, character := range value {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f') || (character >= 'A' && character <= 'F')) {
			return false
		}
	}
	return true
}

func decodeTOML(path string, destination any) error {
	if _, err := toml.DecodeFile(path, destination); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func pngDimensions(path string) (uint32, uint32, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()
	var header [24]byte
	if _, err := io.ReadFull(file, header[:]); err != nil {
		return 0, 0, err
	}
	if string(header[:8]) != "\x89PNG\r\n\x1a\n" || string(header[12:16]) != "IHDR" {
		return 0, 0, fmt.Errorf("%s is not a PNG", path)
	}
	return uint32(header[16])<<24 | uint32(header[17])<<16 | uint32(header[18])<<8 | uint32(header[19]), uint32(header[20])<<24 | uint32(header[21])<<16 | uint32(header[22])<<8 | uint32(header[23]), nil
}

func sha256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func applyGladeOverride(xml string, change gladeOverride) (string, error) {
	idDouble := `id="` + change.ObjectID + `"`
	idSingle := "id='" + change.ObjectID + "'"
	id := strings.Index(xml, idDouble)
	if id < 0 {
		id = strings.Index(xml, idSingle)
	}
	if id < 0 {
		return "", fmt.Errorf("object %s is missing", change.ObjectID)
	}
	objectStart := strings.LastIndex(xml[:id], "<object")
	if objectStart < 0 {
		return "", fmt.Errorf("object %s has no opening tag", change.ObjectID)
	}
	openingRelative := strings.Index(xml[id:], ">")
	if openingRelative < 0 {
		return "", errors.New("object opening tag is incomplete")
	}
	openingEnd := id + openingRelative + 1
	objectEnd, err := matchingObjectEnd(xml, objectStart)
	if err != nil {
		return "", err
	}
	firstChild := objectEnd
	if relative := strings.Index(xml[openingEnd:objectEnd], "<child"); relative >= 0 {
		firstChild = openingEnd + relative
	}
	direct := xml[openingEnd:firstChild]
	propertyDouble := `<property name="` + change.Property + `"`
	propertySingle := "<property name='" + change.Property + "'"
	property := strings.Index(direct, propertyDouble)
	if property < 0 {
		property = strings.Index(direct, propertySingle)
	}
	if property >= 0 {
		propertyStart := openingEnd + property
		valueOpen := strings.Index(xml[propertyStart:], ">")
		if valueOpen < 0 {
			return "", errors.New("property opening tag is incomplete")
		}
		valueStart := propertyStart + valueOpen + 1
		valueEndRelative := strings.Index(xml[valueStart:], "</property>")
		if valueEndRelative < 0 {
			return "", errors.New("property closing tag is missing")
		}
		valueEnd := valueStart + valueEndRelative
		return xml[:valueStart] + escapeXML(change.Value) + xml[valueEnd:], nil
	}
	indentation := lineIndentation(xml, openingEnd)
	propertyText := "\n" + indentation + "  <property name=\"" + change.Property + "\">" + escapeXML(change.Value) + "</property>"
	return xml[:openingEnd] + propertyText + xml[openingEnd:], nil
}

func matchingObjectEnd(xml string, objectStart int) (int, error) {
	position, depth := objectStart, 0
	for {
		open := strings.Index(xml[position:], "<object")
		close := strings.Index(xml[position:], "</object>")
		if open >= 0 {
			open += position
		}
		if close >= 0 {
			close += position
		}
		switch {
		case open >= 0 && close >= 0 && open < close:
			endRelative := strings.Index(xml[open:], ">")
			if endRelative < 0 {
				return 0, errors.New("object opening tag is incomplete")
			}
			end := open + endRelative
			if !strings.HasSuffix(strings.TrimSpace(xml[open:end+1]), "/>") {
				depth++
			}
			position = end + 1
		case close >= 0:
			if depth == 0 {
				return 0, errors.New("unexpected object closing tag")
			}
			depth--
			if depth == 0 {
				return close, nil
			}
			position = close + len("</object>")
		default:
			return 0, errors.New("object closing tag is missing")
		}
	}
}

func lineIndentation(text string, position int) string {
	lineStart := strings.LastIndex(text[:position], "\n") + 1
	line := text[lineStart:position]
	return line[:len(line)-len(strings.TrimLeft(line, " \t"))]
}

func escapeXML(value string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(value)
}

func rewriteTreeinfo(source, efiSHA256, productSHA256, version string) string {
	section := ""
	var output []string
	productAdded, omitSection := false, false
	for _, line := range strings.Split(source, "\n") {
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			if section == "checksums" && !productAdded && !omitSection {
				output = append(output, "images/product.img = sha256:"+productSHA256)
				productAdded = true
			}
			section = strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")
			omitSection = strings.HasPrefix(section, "variant-")
			if !omitSection {
				output = append(output, line)
			}
			continue
		}
		if omitSection {
			continue
		}
		key := ""
		if split := strings.SplitN(line, "=", 2); len(split) == 2 {
			key = strings.TrimSpace(split[0])
		}
		switch {
		case section == "checksums" && key == "images/efiboot.img":
			output = append(output, "images/efiboot.img = sha256:"+efiSHA256)
		case section == "general" && key == "family":
			output = append(output, "family = Soda OS")
		case section == "general" && key == "name":
			output = append(output, "name = Soda OS "+version)
		case (section == "general" || section == "release") && key == "version":
			output = append(output, "version = "+version)
		case section == "release" && key == "name":
			output = append(output, "name = Soda OS")
		case section == "release" && key == "short":
			output = append(output, "short = SodaOS")
		case section == "general" && (key == "packagedir" || key == "repository" || key == "variant" || key == "variants"):
		case section == "tree" && key == "variants":
		default:
			output = append(output, line)
		}
	}
	if section == "checksums" && !productAdded {
		output = append(output, "images/product.img = sha256:"+productSHA256)
	}
	return strings.Join(output, "\n") + "\n"
}

func manifestContainsPackage(manifest, packageName string) bool {
	for _, line := range strings.Split(manifest, "\n") {
		if strings.HasPrefix(line, packageName+"-") {
			return true
		}
	}
	return false
}

func quotedListingEntries(report string) []string {
	var entries []string
	for _, line := range strings.Split(report, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "'") && strings.HasSuffix(line, "'") {
			entries = append(entries, strings.TrimSuffix(strings.TrimPrefix(line, "'"), "'"))
		}
	}
	return entries
}

func recreate(path string) error {
	if err := os.RemoveAll(path); err != nil {
		return err
	}
	return os.MkdirAll(path, 0o755)
}

func replaceFile(path string, data []byte, mode fs.FileMode) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return os.WriteFile(path, data, mode)
}

func copyFile(source, destination string) error {
	in, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("copy %s: %w", source, err)
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func copyTree(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("unsupported non-regular artifact %s", path)
		}
		return copyFile(path, target)
	})
}

func findSingleRPM(root, name string) (string, error) {
	var matches []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".rpm") && strings.HasPrefix(entry.Name(), name+"-") {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(matches) != 1 {
		return "", fmt.Errorf("expected one %s RPM, found %d", name, len(matches))
	}
	return matches[0], nil
}

func joinFiles(paths ...string) (string, error) {
	var pieces []string
	for _, path := range paths {
		contents, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		pieces = append(pieces, string(contents))
	}
	return strings.Join(pieces, "\n"), nil
}
