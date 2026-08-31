package image

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/LevitateOS/soda-os/internal/process"
)

func (b *Builder) BuildRPMs(ctx context.Context) error {
	if err := b.requireNativeHost(); err != nil {
		return err
	}
	if err := b.Check(ctx); err != nil {
		return err
	}
	revision, err := b.sourceRevision(ctx)
	if err != nil {
		return err
	}
	if err := b.buildContainer(ctx); err != nil {
		return err
	}
	workspace, err := b.prepareRPMWorkspace()
	if err != nil {
		return err
	}
	return b.buildLockedRPMs(ctx, workspace, revision)
}

type rpmWorkspace struct{ build, topdir, rpms string }

func (b *Builder) prepareRPMWorkspace() (rpmWorkspace, error) {
	workspace := rpmWorkspace{b.artifactPath("build"), b.artifactPath("rpmbuild"), b.artifactPath("rpms")}
	for _, path := range []string{workspace.build, workspace.topdir, workspace.rpms} {
		if err := recreate(path); err != nil {
			return rpmWorkspace{}, err
		}
	}
	for _, directory := range []string{"BUILD", "BUILDROOT", "RPMS", "SOURCES", "SPECS", "SRPMS"} {
		if err := os.MkdirAll(filepath.Join(workspace.topdir, directory), 0o755); err != nil {
			return rpmWorkspace{}, err
		}
	}
	return workspace, nil
}

func (b *Builder) buildLockedRPMs(ctx context.Context, workspace rpmWorkspace, revision string) error {
	if err := b.buildProductBinaries(ctx, revision); err != nil {
		return err
	}
	if err := b.stageRPMSources(workspace.build, filepath.Join(workspace.topdir, "SOURCES")); err != nil {
		return err
	}
	for _, name := range targetRPMs {
		if err := b.rpmbuild(ctx, name); err != nil {
			return err
		}
		rpm, err := findSingleRPM(filepath.Join(workspace.topdir, "RPMS"), name)
		if err != nil {
			return err
		}
		if err := copyFile(rpm, filepath.Join(workspace.rpms, filepath.Base(rpm))); err != nil {
			return err
		}
	}
	if err := b.writeLockedInstallInputs(workspace.rpms); err != nil {
		return err
	}
	fmt.Printf("Built locked Soda RPM inputs at %s\n", workspace.rpms)
	return nil
}

func (b *Builder) buildProductBinaries(ctx context.Context, revision string) error {
	if err := b.buildGoBinaries(ctx, revision); err != nil {
		return err
	}
	return b.buildForgejo(ctx)
}

const (
	forgejoVersion      = "15.0.7"
	forgejoSourceSHA256 = "e11490f52542104651d81cfa7a23376a4c005397499e6dc1a7850e2fb8176ad6"
)

func (b *Builder) buildForgejo(ctx context.Context) error {
	archive := b.artifactPath("tools", "forgejo-src-"+forgejoVersion+".tar.gz")
	contents, err := os.ReadFile(archive)
	if err != nil {
		return fmt.Errorf("pinned Forgejo source is missing; run just forgejo-source: %w", err)
	}
	hash := sha256.Sum256(contents)
	if hex.EncodeToString(hash[:]) != forgejoSourceSHA256 {
		return errors.New("Forgejo source archive checksum differs from the distribution contract")
	}
	script := strings.Join([]string{
		"set -eu",
		"rm -rf /src/.artifacts/build/forgejo-source",
		"mkdir -p /src/.artifacts/build/forgejo-source",
		"tar -xzf /src/.artifacts/tools/forgejo-src-" + forgejoVersion + ".tar.gz -C /src/.artifacts/build/forgejo-source --strip-components=1",
		"cd /src/.artifacts/build/forgejo-source",
		"TAGS='bindata timetzdata sqlite sqlite_unlock_notify pam' make backend",
		"install -m 0755 gitea /src/.artifacts/build/forgejo",
		"/src/.artifacts/build/forgejo --version | grep -F ': bindata, timetzdata, sqlite, sqlite_unlock_notify, pam'",
	}, "\n")
	return b.docker(ctx, []string{"CGO_ENABLED=1", "EXTRA_GOFLAGS=-buildvcs=false", "SOURCE_DATE_EPOCH=" + fmt.Sprint(b.Spec.Build.SourceDateEpoch)}, "sh", "-c", script)
}

func (b *Builder) buildGoBinaries(ctx context.Context, revision string) error {
	buildDate := time.Unix(b.Spec.Build.SourceDateEpoch, 0).UTC().Format(time.RFC3339)
	linkerFlags := strings.Join([]string{
		"-s", "-w", "-buildid=",
		"-X github.com/LevitateOS/soda-os/internal/version.Version=" + b.Spec.Identity.Version,
		"-X github.com/LevitateOS/soda-os/internal/version.Commit=" + revision,
		"-X github.com/LevitateOS/soda-os/internal/version.BuildDate=" + buildDate,
	}, " ")
	for _, target := range []struct{ output, pkg string }{
		{"sodad", "./cmd/sodad"},
		{"sodactl", "./cmd/sodactl"},
		{"soda-ssh", "./cmd/soda-ssh"},
		{"soda-tailnet", "./cmd/soda-tailnet"},
		{"soda-forgejo-tailnet", "./cmd/soda-forgejo-tailnet"},
		{"soda-cockpit", "./cockpit/cmd/soda-cockpit"},
		{"soda-authd", "./cockpit/cmd/soda-authd"},
	} {
		if err := b.docker(ctx, []string{"CGO_ENABLED=1", "SOURCE_DATE_EPOCH=" + fmt.Sprint(b.Spec.Build.SourceDateEpoch)}, "go", "build", "-buildvcs=false", "-trimpath", "-ldflags="+linkerFlags, "-o", "/src/.artifacts/build/"+target.output, target.pkg); err != nil {
			return err
		}
	}
	return nil
}

func (b *Builder) stageRPMSources(build, sources string) error {
	files := [][2]string{
		{filepath.Join(build, "sodad"), filepath.Join(sources, "sodad")},
		{filepath.Join(build, "sodactl"), filepath.Join(sources, "sodactl")},
		{filepath.Join(build, "soda-ssh"), filepath.Join(sources, "soda-ssh")},
		{filepath.Join(build, "soda-tailnet"), filepath.Join(sources, "soda-tailnet")},
		{filepath.Join(build, "soda-cockpit"), filepath.Join(sources, "soda-cockpit")},
		{filepath.Join(build, "soda-authd"), filepath.Join(sources, "soda-authd")},
		{filepath.Join(build, "forgejo"), filepath.Join(sources, "forgejo")},
		{b.path("packaging/rpm/runtime/sources/systemd/sodad.service"), filepath.Join(sources, "sodad.service")},
		{b.path("packaging/rpm/runtime/sources/systemd/soda-tailscale-enroll.service"), filepath.Join(sources, "soda-tailscale-enroll.service")},
		{b.path("packaging/rpm/runtime/sources/systemd/soda-state-directories.service"), filepath.Join(sources, "soda-state-directories.service")},
		{b.path("packaging/rpm/runtime/sources/systemd/var-srv-soda-projects.mount"), filepath.Join(sources, "var-srv-soda-projects.mount")},
		{b.path("packaging/rpm/runtime/sources/systemd/opt-soda-toolchains.mount"), filepath.Join(sources, "opt-soda-toolchains.mount")},
		{b.path("packaging/rpm/runtime/sources/systemd/90-soda.preset"), filepath.Join(sources, "90-soda.preset")},
		{b.path("packaging/rpm/runtime/sources/nftables/soda-ingress.nft"), filepath.Join(sources, "soda-ingress.nft")},
		{b.path("packaging/rpm/runtime/sources/systemd/nftables.service.d/10-soda-ingress.conf"), filepath.Join(sources, "10-soda-ingress.conf")},
		{b.path("packaging/rpm/runtime/sources/systemd/getty@tty1.service.d/10-soda-console.conf"), filepath.Join(sources, "10-soda-console.conf")},
		{b.path("packaging/rpm/runtime/sources/tmpfiles/soda.conf"), filepath.Join(sources, "soda.conf")},
		{b.path("packaging/rpm/runtime/sources/sysctl/60-soda-console.conf"), filepath.Join(sources, "60-soda-console.conf")},
		{b.path("packaging/rpm/runtime/sources/sysusers/soda.conf"), filepath.Join(sources, "soda.sysusers")},
		{b.path("packaging/rpm/runtime/sources/sshd/41-soda-project-accounts.conf"), filepath.Join(sources, "41-soda-project-accounts.conf")},
		{b.path("packaging/rpm/runtime/sources/console/soda-console-welcome"), filepath.Join(sources, "soda-console-welcome")},
		{b.path("packaging/rpm/runtime/sources/profile.d/soda-console-welcome.sh"), filepath.Join(sources, "soda-console-welcome.sh")},
		{b.path("packaging/rpm/cockpit/sources/systemd/soda-cockpit.service"), filepath.Join(sources, "soda-cockpit.service")},
		{b.path("packaging/rpm/cockpit/sources/systemd/soda-authd.service"), filepath.Join(sources, "soda-authd.service")},
		{b.path("packaging/rpm/cockpit/sources/avahi/soda-cockpit.service"), filepath.Join(sources, "soda-cockpit.avahi.service")},
		{b.path("packaging/rpm/cockpit/sources/pam/soda-cockpit"), filepath.Join(sources, "soda-cockpit.pam")},
		{b.path("packaging/rpm/forgejo/sources/systemd/forgejo.service"), filepath.Join(sources, "forgejo.service")},
		{b.path("packaging/rpm/forgejo/sources/systemd/forgejo-init.service"), filepath.Join(sources, "forgejo-init.service")},
		{b.path("packaging/rpm/forgejo/sources/forgejo-init"), filepath.Join(sources, "forgejo-init")},
		{filepath.Join(build, "soda-forgejo-tailnet"), filepath.Join(sources, "forgejo-tailnet")},
		{b.path("packaging/rpm/forgejo/sources/app.ini.tmpl"), filepath.Join(sources, "forgejo-app.ini.tmpl")},
		{b.path("packaging/rpm/forgejo/sources/sysusers/forgejo.conf"), filepath.Join(sources, "forgejo.sysusers")},
		{b.path("packaging/rpm/forgejo/sources/tmpfiles/forgejo.conf"), filepath.Join(sources, "forgejo.tmpfiles")},
		{b.path("packaging/rpm/forgejo/sources/pam/soda-forgejo"), filepath.Join(sources, "soda-forgejo.pam")},
		{b.path("packaging/rpm/release/sources/BASE_SYSTEM.md"), filepath.Join(sources, "BASE_SYSTEM.md")},
		{b.path("assets/branding/source/soda-symbol.svg"), filepath.Join(sources, "soda-symbol.svg")},
	}
	for _, size := range []string{"16", "24", "32", "48", "64", "128", "256", "512"} {
		files = append(files, [2]string{b.path("assets/branding/icons/hicolor/" + size + "x" + size + "/apps/soda-os.png"), filepath.Join(sources, "soda-os-"+size+".png")})
	}
	for _, pair := range files {
		if err := copyFile(pair[0], pair[1]); err != nil {
			return err
		}
	}
	return nil
}

func (b *Builder) writeLockedInstallInputs(rpms string) error {
	lock, err := b.packageLock()
	if err != nil {
		return err
	}
	directory := b.artifactPath("bootc")
	if err := recreate(directory); err != nil {
		return err
	}
	var fedora, installed []string
	for _, item := range lock.Package {
		installed = append(installed, item.NEVRA)
		if item.Source == "fedora" {
			fedora = append(fedora, item.NEVRA)
			continue
		}
		if !isFile(filepath.Join(rpms, item.File)) {
			return fmt.Errorf("locked local RPM %s is missing", item.File)
		}
	}
	for path, lines := range map[string][]string{
		"fedora-packages.txt":   fedora,
		"expected-packages.txt": installed,
	} {
		if err := os.WriteFile(filepath.Join(directory, path), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func (b *Builder) buildContainer(ctx context.Context) error {
	lockDestination := b.artifactPath("builder", "packages.lock")
	if err := copyFile(b.path(b.Spec.Platform.Builder.PackageLock), lockDestination); err != nil {
		return fmt.Errorf("stage builder package lock: %w", err)
	}
	goArchive := b.path(b.Spec.Platform.Builder.GoArchive)
	contents, err := os.ReadFile(goArchive)
	if err != nil {
		return fmt.Errorf("pinned Go 1.27 builder input is missing; run just builder-tools: %w", err)
	}
	hash := sha256.Sum256(contents)
	if hex.EncodeToString(hash[:]) != b.Spec.Platform.Builder.GoArchiveSHA256 {
		return errors.New("Go 1.27 builder archive checksum differs from the selected platform contract")
	}
	if err := copyFile(goArchive, b.artifactPath("builder", "go.tar.gz")); err != nil {
		return fmt.Errorf("stage Go 1.27 builder toolchain: %w", err)
	}
	return b.runner.Run(ctx, process.Command{Dir: b.Root, Name: "docker", Args: []string{"build", "--quiet", "--platform", b.Spec.Base.Platform, "--build-arg", "BUILDER_BASE_REFERENCE=" + b.Spec.Platform.Builder.BaseReference, "--file", "packaging/builder/Containerfile", "--tag", b.builderTag(), "."}})
}

func (b *Builder) docker(ctx context.Context, environment []string, name string, args ...string) error {
	return b.runner.Run(ctx, b.dockerCommand(environment, name, args...))
}

func (b *Builder) dockerCommand(environment []string, name string, args ...string) process.Command {
	dockerArgs := []string{"run", "--rm", "--platform", b.Spec.Base.Platform, "--volume", b.Root + ":/src", "--workdir", "/src"}
	for _, pair := range environment {
		dockerArgs = append(dockerArgs, "--env", pair)
	}
	dockerArgs = append(dockerArgs, b.builderTag(), name)
	dockerArgs = append(dockerArgs, args...)
	return process.Command{Dir: b.Root, Name: "docker", Args: dockerArgs}
}

func (b *Builder) builderTag() string {
	return "soda-os-rpm-builder:" + b.Spec.Identity.Version + "-" + b.Spec.Platform.Architecture.Artifact
}

func (b *Builder) rpmbuild(ctx context.Context, name string) error {
	epoch := fmt.Sprint(b.Spec.Build.SourceDateEpoch)
	spec := "packaging/rpm/" + strings.TrimPrefix(name, "soda-") + "/" + name + ".spec"
	return b.docker(ctx, []string{"SOURCE_DATE_EPOCH=" + epoch}, "rpmbuild", "-bb",
		"--define", "_topdir /src/.artifacts/rpmbuild",
		"--define", "_source_date_epoch "+epoch,
		"--define", "use_source_date_epoch_as_buildtime 1",
		"--define", "_buildhost soda-builder",
		spec)
}
