#!/bin/sh
set -eu

repository_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repository_root"

identity_value() {
  key=$1
  awk -F '"' -v key="$key" '
    /^\[identity\]$/ { identity = 1; next }
    /^\[/ { identity = 0 }
    identity && $1 ~ "^" key "[[:space:]]*=" { print $2; exit }
  ' distro/soda.toml
}

name=$(identity_value name)
id=$(identity_value id)
version=$(identity_value version)
case "$version" in
  [0-9]*.[0-9]*.[0-9]*) ;;
  *) echo "distro/soda.toml has no release version" >&2; exit 1 ;;
esac
[ "$name" = 'Soda OS' ] || { echo 'distro/soda.toml has no Soda product name' >&2; exit 1; }
[ "$id" = sodaos ] || { echo 'distro/soda.toml has no Soda product ID' >&2; exit 1; }
minor=${version%.*}

contains() {
  path=$1
  value=$2
  if ! grep -Fqx "$value" "$path" && ! grep -Fq "$value" "$path"; then
    echo "$path does not match distro/soda.toml version $version" >&2
    exit 1
  fi
}

exact() {
  path=$1
  value=$2
  if ! grep -Fqx "$value" "$path"; then
    echo "$path does not match distro/soda.toml identity" >&2
    exit 1
  fi
}

contains cockpit/soda-projects/manifest.json "\"version\": \"$version\""
contains packaging/installer/branding/buildstamp "Version=$version"
contains packaging/installer/branding/buildstamp "UUID=SodaOS-$version"
contains packaging/installer/branding/os-release "VERSION=\"$minor\""
contains packaging/installer/branding/os-release "PRETTY_NAME=\"Soda OS $version\""
for lock in \
  distro/locks/runtime-packages-aarch64.toml \
  distro/locks/runtime-packages-x86_64.toml; do
  for package in soda-release soda-runtime soda-projects; do
    contains "$lock" "$package-0:$version-"
  done
done

for spec in \
  packaging/rpm/release/soda-release.spec \
  packaging/rpm/runtime/soda-runtime.spec \
  packaging/rpm/projects/soda-projects.spec; do
  exact "$spec" 'Version:        %{soda_version}'
done
contains internal/build/image/rpm.go 'internal/version.Version=" + b.Spec.Identity.Version'
contains internal/build/image/rpm.go '"soda_version "+b.Spec.Identity.Version'
contains internal/build/image/rpm.go '"soda_os_release_version "+osReleaseVersion'
contains internal/version/version.go 'DefaultVersion = "development"'

contains packaging/bootc/Containerfile 'org.opencontainers.image.version="${SODA_VERSION}"'
contains internal/build/image/builder.go '"--tag", b.Spec.Image.Registry + ":" + b.Spec.Identity.Version'
contains internal/build/image/builder.go '"--build-arg", "SODA_VERSION=" + b.Spec.Identity.Version'
contains internal/build/release/image_publication.go 'return Repository + ":" + p.version + "-" + spec.Platform.Architecture.Artifact'
contains internal/build/installer/builder.go 'outputName := "SodaOS-" + b.Spec.Identity.Version'
contains internal/build/installer/qcow2.go 'outputName := "SodaOS-" + b.Spec.Identity.Version'
contains internal/build/release/publisher.go '"soda-os-"+record.SodaVersion+"-"+record.Channel+".release.json"'
contains internal/build/release/github_validation.go 'record := "soda-os-" + spec.Identity.Version'
contains scripts/soda-release-executor 'release_version() {'
contains scripts/soda-release-executor 'distro/soda.toml'

contains internal/build/release/github.go 'version:          aarch64.Identity.Version'
exact internal/build/release/github.go 'func (p *Publication) tag() string { return "v" + p.version }'
contains internal/build/release/github.go '"--title", "Soda OS "+p.version'
contains internal/build/release/github.go 'release notes omit the %s exact GHCR digest'
if grep -Fq "$version" .github/workflows/release.yml; then
  echo '.github/workflows/release.yml must derive Soda version from distro/soda.toml' >&2
  exit 1
fi
if [ "$(grep -Fc 'distro/soda.toml' .github/workflows/release.yml)" -lt 4 ]; then
  echo '.github/workflows/release.yml does not derive every release identity from distro/soda.toml' >&2
  exit 1
fi
