package image

import (
	"context"
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
	if err := b.verifyFetchedBuildInputs(); err != nil {
		return err
	}
	runtimeLock, _, err := b.snapshotRuntimePackageLock()
	if err != nil {
		return err
	}
	return b.buildRPMs(ctx, revision, runtimeLock)
}

func (b *Builder) buildRPMs(ctx context.Context, revision, runtimeLock string) error {
	if err := b.buildContainer(ctx); err != nil {
		return err
	}
	workspace, err := b.prepareRPMWorkspace()
	if err != nil {
		return err
	}
	return b.buildLockedRPMs(ctx, workspace, revision, runtimeLock)
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

func (b *Builder) buildLockedRPMs(ctx context.Context, workspace rpmWorkspace, revision, runtimeLock string) error {
	if err := b.stageLockedRPMs(ctx, workspace, revision); err != nil {
		return err
	}
	if err := b.writeLockedInstallInputs(workspace.rpms, runtimeLock); err != nil {
		return err
	}
	fmt.Printf("Built locked Soda RPM inputs at %s\n", workspace.rpms)
	return nil
}

func (b *Builder) stageLockedRPMs(ctx context.Context, workspace rpmWorkspace, revision string) error {
	if err := b.buildProductBinaries(ctx, revision); err != nil {
		return err
	}
	if err := b.stageRPMSources(workspace.build, filepath.Join(workspace.topdir, "SOURCES")); err != nil {
		return err
	}
	for _, name := range builtRPMs {
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
	if err := b.stageMiseRPM(workspace.rpms); err != nil {
		return err
	}
	return nil
}

func (b *Builder) buildProductBinaries(ctx context.Context, revision string) error {
	if err := b.buildGoBinaries(ctx, revision); err != nil {
		return err
	}
	if err := b.buildForgejo(ctx); err != nil {
		return err
	}
	return b.buildTea(ctx)
}

func (b *Builder) buildForgejo(ctx context.Context) error {
	lock, err := readForgejoSourceLock(b.path("distro/locks/forgejo-source.toml"))
	if err != nil {
		return err
	}
	archive := b.artifactPath("tools", lock.SourceArchive)
	if err := verifyFileSHA256(archive, lock.SHA256); err != nil {
		return fmt.Errorf("verify Forgejo source; run just forgejo-source: %w", err)
	}
	patch := b.path("packaging/rpm/forgejo/sources/patches/0001-pam-do-not-retain-password.patch")
	if err = verifyFileSHA256(patch, lock.PatchSHA256); err != nil {
		return fmt.Errorf("verify Forgejo PAM patch: %w", err)
	}
	script := strings.Join([]string{
		"set -eu",
		"rm -rf /src/.artifacts/build/forgejo-source",
		"mkdir -p /src/.artifacts/build/forgejo-source /src/.artifacts/build/forgejo-go-cache /src/.artifacts/build/forgejo-go-tmp",
		"tar -xzf /src/.artifacts/tools/" + lock.SourceArchive + " -C /src/.artifacts/build/forgejo-source --strip-components=1",
		"cd /src/.artifacts/build/forgejo-source",
		"patch --batch --forward --fuzz=0 --strip=1 --input=/src/packaging/rpm/forgejo/sources/patches/0001-pam-do-not-retain-password.patch",
		"if grep -F 'Passwd:      password' services/auth/source/pam/source_authenticate.go; then echo 'Forgejo PAM patch did not remove the copied password verifier' >&2; exit 1; fi",
		"go test ./services/auth/source/pam",
		"TAGS='" + lock.BuildTags + "' make backend",
		"install -m 0755 gitea /src/.artifacts/build/forgejo",
		"/src/.artifacts/build/forgejo --version | grep -F ': " + strings.ReplaceAll(lock.BuildTags, " ", ", ") + "'",
	}, "\n")
	return b.docker(ctx, []string{
		"CGO_ENABLED=1",
		"EXTRA_GOFLAGS=-buildvcs=false",
		"GOCACHE=/src/.artifacts/build/forgejo-go-cache",
		"GOTMPDIR=/src/.artifacts/build/forgejo-go-tmp",
		"SOURCE_DATE_EPOCH=" + fmt.Sprint(b.Spec.Build.SourceDateEpoch),
	}, "sh", "-c", script)
}

func (b *Builder) buildTea(ctx context.Context) error {
	lock, err := readTeaSourceLock(b.path("distro/locks/tea-source.toml"))
	if err != nil {
		return err
	}
	archive := b.artifactPath("tools", lock.SourceArchive)
	if err = verifyFileSHA256(archive, lock.SourceSHA256); err != nil {
		return fmt.Errorf("verify Tea source; run just tea-source: %w", err)
	}
	script := strings.Join([]string{
		"set -eu",
		"rm -rf /src/.artifacts/build/tea-source /src/.artifacts/build/tea-go-cache /src/.artifacts/build/tea-go-tmp",
		"mkdir -p /src/.artifacts/build/tea-source /src/.artifacts/build/tea-go-cache /src/.artifacts/build/tea-go-tmp",
		"tar -xzf /src/.artifacts/tools/" + lock.SourceArchive + " -C /src/.artifacts/build/tea-source --strip-components=1",
		"cd /src/.artifacts/build/tea-source",
		"go test ./cmd/login ./modules/task ./modules/config",
		"TEA_VERSION=" + lock.Version + " make BUILDMODE=-buildvcs=false build",
		"install -m 0755 tea /src/.artifacts/build/tea",
		"/src/.artifacts/build/tea --version | grep -F '" + lock.Version + "'",
	}, "\n")
	return b.docker(ctx, []string{
		"CGO_ENABLED=0",
		"GOCACHE=/src/.artifacts/build/tea-go-cache",
		"GOTMPDIR=/src/.artifacts/build/tea-go-tmp",
		"SOURCE_DATE_EPOCH=" + fmt.Sprint(b.Spec.Build.SourceDateEpoch),
	}, "sh", "-c", script)
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
		{"soda-projects", "./cmd/soda-projects"},
		{"soda-workspace-helper", "./cmd/soda-workspace-helper"},
		{"soda-runners", "./cmd/soda-runners"},
		{"soda-runner-helper", "./cmd/soda-runner-helper"},
		{"soda-runner-launch", "./cmd/soda-runner-launch"},
		{"soda-setup", "./cmd/soda-setup"},
		{"soda-tailnet", "./cmd/soda-tailnet"},
		{"soda-forgejo-tailnet", "./cmd/soda-forgejo-tailnet"},
	} {
		if err := b.docker(ctx, []string{"CGO_ENABLED=1", "SOURCE_DATE_EPOCH=" + fmt.Sprint(b.Spec.Build.SourceDateEpoch)}, "go", "build", "-buildvcs=false", "-trimpath", "-ldflags="+linkerFlags, "-o", "/src/.artifacts/build/"+target.output, target.pkg); err != nil {
			return err
		}
	}
	return nil
}

func (b *Builder) stageRPMSources(build, sources string) error {
	if err := b.stageTeaSource(sources); err != nil {
		return err
	}
	if err := b.stageGitHubRunnerSource(sources); err != nil {
		return err
	}
	return b.stageProductRPMSources(build, sources)
}

func (b *Builder) stageGitHubRunnerSource(sources string) error {
	lock, err := readGitHubRunnerSourceLock(b.path("distro/locks/github-runner-source.toml"))
	if err != nil {
		return err
	}
	asset, err := lock.asset(b.Spec.Platform.Architecture.Name)
	if err != nil {
		return err
	}
	archive := b.artifactPath("tools", asset.Archive)
	if err = verifyFileSHA256(archive, asset.SHA256); err != nil {
		return fmt.Errorf("verify GitHub runner source; run just github-runner %s: %w", asset.Architecture, err)
	}
	return copyFile(archive, filepath.Join(sources, "github-actions-runner.tar.gz"))
}

func (b *Builder) stageProductRPMSources(build, sources string) error {
	files := [][2]string{
		{filepath.Join(build, "soda-projects"), filepath.Join(sources, "soda-projects")},
		{filepath.Join(build, "soda-workspace-helper"), filepath.Join(sources, "soda-workspace-helper")},
		{filepath.Join(build, "soda-runners"), filepath.Join(sources, "soda-runners")},
		{filepath.Join(build, "soda-runner-helper"), filepath.Join(sources, "soda-runner-helper")},
		{filepath.Join(build, "soda-runner-launch"), filepath.Join(sources, "soda-runner-launch")},
		{filepath.Join(build, "soda-setup"), filepath.Join(sources, "soda-setup")},
		{filepath.Join(build, "soda-tailnet"), filepath.Join(sources, "soda-tailnet")},
		{filepath.Join(build, "forgejo"), filepath.Join(sources, "forgejo")},
		{b.path("packaging/rpm/runtime/sources/systemd/90-soda.preset"), filepath.Join(sources, "90-soda.preset")},
		{b.path("packaging/rpm/runtime/sources/systemd/soda-setup.service"), filepath.Join(sources, "soda-setup.service")},
		{b.path("packaging/rpm/runtime/sources/tmpfiles/soda-runtime.conf"), filepath.Join(sources, "soda-runtime.tmpfiles")},
		{b.path("packaging/rpm/runtime/sources/soda-local-access"), filepath.Join(sources, "soda-local-access")},
		{b.path("packaging/rpm/runtime/sources/firewalld/zones/soda-tailnet.xml"), filepath.Join(sources, "soda-tailnet.xml")},
		{b.path("packaging/rpm/runtime/sources/systemd/getty@tty1.service.d/10-soda-console.conf"), filepath.Join(sources, "10-soda-console.conf")},
		{b.path("packaging/rpm/runtime/sources/sysctl/60-soda-console.conf"), filepath.Join(sources, "60-soda-console.conf")},
		{b.path("packaging/rpm/runtime/sources/console/soda-console-welcome"), filepath.Join(sources, "soda-console-welcome")},
		{b.path("packaging/rpm/runtime/sources/profile.d/soda-console-welcome.sh"), filepath.Join(sources, "soda-console-welcome.sh")},
		{b.path("packaging/rpm/projects/sources/pam/cockpit-stock"), filepath.Join(sources, "cockpit-stock.pam")},
		{b.path("packaging/rpm/projects/sources/polkit/org.sodaos.projects.policy"), filepath.Join(sources, "org.sodaos.projects.policy")},
		{b.path("packaging/rpm/projects/sources/tmpfiles/soda-projects.conf"), filepath.Join(sources, "soda-projects.tmpfiles")},
		{b.path("packaging/rpm/projects/sources/sysusers/soda-projects.conf"), filepath.Join(sources, "soda-projects.sysusers")},
		{b.path("cockpit/soda-projects/manifest.json"), filepath.Join(sources, "soda-projects-manifest.json")},
		{b.path("cockpit/soda-projects/index.html"), filepath.Join(sources, "soda-projects-index.html")},
		{b.path("cockpit/soda-projects/app.mjs"), filepath.Join(sources, "soda-projects-app.mjs")},
		{b.path("cockpit/soda-projects/protocol.mjs"), filepath.Join(sources, "soda-projects-protocol.mjs")},
		{b.path("cockpit/soda-projects/ui.mjs"), filepath.Join(sources, "soda-projects-ui.mjs")},
		{b.path("cockpit/soda-projects/setup.mjs"), filepath.Join(sources, "soda-projects-setup.mjs")},
		{b.path("cockpit/soda-projects/setup-protocol.mjs"), filepath.Join(sources, "soda-projects-setup-protocol.mjs")},
		{b.path("cockpit/soda-projects/app.css"), filepath.Join(sources, "soda-projects-app.css")},
		{b.path("packaging/rpm/runners/sources/polkit/org.sodaos.runners.policy"), filepath.Join(sources, "org.sodaos.runners.policy")},
		{b.path("packaging/rpm/runners/sources/tmpfiles/soda-runners.conf"), filepath.Join(sources, "soda-runners.tmpfiles")},
		{b.path("packaging/rpm/runners/sources/sysusers/soda-runners.conf"), filepath.Join(sources, "soda-runners.sysusers")},
		{b.path("packaging/rpm/runners/sources/systemd/soda-runner@.service"), filepath.Join(sources, "soda-runner@.service")},
		{b.path("cockpit/soda-runners/manifest.json"), filepath.Join(sources, "soda-runners-manifest.json")},
		{b.path("cockpit/soda-runners/index.html"), filepath.Join(sources, "soda-runners-index.html")},
		{b.path("cockpit/soda-runners/app.mjs"), filepath.Join(sources, "soda-runners-app.mjs")},
		{b.path("cockpit/soda-runners/protocol.mjs"), filepath.Join(sources, "soda-runners-protocol.mjs")},
		{b.path("cockpit/soda-runners/ui.mjs"), filepath.Join(sources, "soda-runners-ui.mjs")},
		{b.path("cockpit/soda-runners/app.css"), filepath.Join(sources, "soda-runners-app.css")},
		{b.path("packaging/rpm/projects/sources/branding/sodaos/branding.css"), filepath.Join(sources, "soda-projects-branding.css")},
		{b.path("assets/branding/source/soda-symbol.svg"), filepath.Join(sources, "soda-projects-symbol.svg")},
		{b.path("packaging/rpm/forgejo/sources/systemd/forgejo.service"), filepath.Join(sources, "forgejo.service")},
		{b.path("packaging/rpm/forgejo/sources/systemd/forgejo-init.service"), filepath.Join(sources, "forgejo-init.service")},
		{b.path("packaging/rpm/forgejo/sources/forgejo-init"), filepath.Join(sources, "forgejo-init")},
		{filepath.Join(build, "soda-forgejo-tailnet"), filepath.Join(sources, "forgejo-tailnet")},
		{b.path("packaging/rpm/forgejo/sources/app.ini.tmpl"), filepath.Join(sources, "forgejo-app.ini.tmpl")},
		{b.path("packaging/rpm/forgejo/sources/sysusers/forgejo.conf"), filepath.Join(sources, "forgejo.sysusers")},
		{b.path("packaging/rpm/forgejo/sources/tmpfiles/forgejo.conf"), filepath.Join(sources, "forgejo.tmpfiles")},
		{b.path("packaging/rpm/forgejo/sources/pam/soda-forgejo"), filepath.Join(sources, "soda-forgejo.pam")},
		{b.path("packaging/rpm/forgejo/sources/selinux/soda-forgejo-shadow.te"), filepath.Join(sources, "soda-forgejo-shadow.te")},
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

func (b *Builder) writeLockedInstallInputs(rpms, runtimeLock string) error {
	lock, err := readPackageLock(runtimeLock)
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
			return fmt.Errorf("locked RPM %s is missing", item.File)
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
	if err := b.verifyGoBuilderArchive(); err != nil {
		return err
	}
	if err := copyFile(goArchive, b.artifactPath("builder", "go.tar.gz")); err != nil {
		return fmt.Errorf("stage Go builder toolchain: %w", err)
	}
	return b.runner.Run(ctx, process.Command{Dir: b.Root, Name: "docker", Args: []string{"build", "--quiet", "--platform", b.Spec.Base.Platform, "--build-arg", "BUILDER_BASE_REFERENCE=" + b.Spec.Platform.Builder.BaseReference, "--build-arg", "GO_VERSION=" + b.Spec.Platform.Builder.GoVersion, "--file", "packaging/builder/Containerfile", "--tag", b.builderTag(), "."}})
}

func (b *Builder) verifyGoBuilderArchive() error {
	if err := verifyFileSHA256(b.path(b.Spec.Platform.Builder.GoArchive), b.Spec.Platform.Builder.GoArchiveSHA256); err != nil {
		return fmt.Errorf("verify Go builder input; run just builder-tools %s: %w", b.Spec.Platform.Architecture.Name, err)
	}
	return nil
}

func (b *Builder) docker(ctx context.Context, environment []string, name string, args ...string) error {
	return b.runner.Run(ctx, b.dockerCommand(environment, name, args...))
}

func (b *Builder) dockerCommand(environment []string, name string, args ...string) process.Command {
	owner := fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())
	dockerArgs := []string{
		"run", "--rm", "--platform", b.Spec.Base.Platform,
		"--user", owner, "--env", "HOME=/tmp",
		"--volume", b.Root + ":/src", "--workdir", "/src",
	}
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
	osReleaseVersion, err := osReleaseVersionID(b.Spec.Identity.Version)
	if err != nil {
		return err
	}
	spec := "packaging/rpm/" + strings.TrimPrefix(name, "soda-") + "/" + name + ".spec"
	command := b.dockerCommand([]string{"SOURCE_DATE_EPOCH=" + epoch}, "rpmbuild", "-bb",
		"--define", "_topdir /src/.artifacts/rpmbuild",
		"--define", "soda_version "+b.Spec.Identity.Version,
		"--define", "soda_os_release_version "+osReleaseVersion,
		"--define", "_source_date_epoch "+epoch,
		"--define", "use_source_date_epoch_as_buildtime 1",
		"--define", "_buildhost soda-builder",
		spec)
	args := make([]string, 0, len(command.Args)+2)
	args = append(args, command.Args[0], "--network", "none")
	command.Args = append(args, command.Args[1:]...)
	return b.runner.Run(ctx, command)
}

func osReleaseVersionID(version string) (string, error) {
	parts := strings.Split(version, ".")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", fmt.Errorf("Soda version %q is not major.minor.patch", version)
	}
	for _, part := range parts {
		for _, rune := range part {
			if rune < '0' || rune > '9' {
				return "", fmt.Errorf("Soda version %q is not major.minor.patch", version)
			}
		}
	}
	return parts[0] + "." + parts[1], nil
}
