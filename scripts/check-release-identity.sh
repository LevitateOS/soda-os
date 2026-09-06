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
  for package in soda-release soda-runtime soda-projects soda-runners; do
    contains "$lock" "$package-0:$version-"
  done
done

for spec in \
  packaging/rpm/release/soda-release.spec \
  packaging/rpm/runtime/soda-runtime.spec \
  packaging/rpm/projects/soda-projects.spec \
  packaging/rpm/runners/soda-runners.spec; do
  exact "$spec" 'Version:        %{soda_version}'
done
contains internal/build/image/rpm.go 'internal/version.Version=" + b.Spec.Identity.Version'
contains internal/build/image/rpm.go '"soda_version "+b.Spec.Identity.Version'
contains internal/build/image/rpm.go '"soda_os_release_version "+osReleaseVersion'
contains internal/version/version.go 'DefaultVersion = "development"'

contains packaging/bootc/Containerfile 'org.opencontainers.image.version="${SODA_VERSION}"'
contains internal/build/image/builder.go '"--tag", b.Spec.Image.Registry + ":" + b.Spec.Identity.Version'
contains internal/build/image/builder.go '"--build-arg", "SODA_VERSION=" + b.Spec.Identity.Version'
contains packaging/bootc/Containerfile 'org.opencontainers.image.revision="${SODA_SOURCE_REVISION}"'
contains internal/build/image/builder.go '"--build-arg", "SODA_SOURCE_REVISION=" + inputs.revision'
contains internal/build/installer/builder.go 'outputName := "SodaOS-" + b.Spec.Identity.Version'
contains internal/build/installer/qcow2.go 'outputName := "SodaOS-" + b.Spec.Identity.Version'
