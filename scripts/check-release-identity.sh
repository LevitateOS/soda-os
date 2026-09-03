#!/bin/sh
set -eu

repository_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repository_root"

version=$(awk -F '"' '/^version = / { print $2; exit }' distro/soda.toml)
case "$version" in
  [0-9]*.[0-9]*.[0-9]*) ;;
  *) echo "distro/soda.toml has no release version" >&2; exit 1 ;;
esac
minor=${version%.*}

contains() {
  path=$1
  value=$2
  if ! grep -Fqx "$value" "$path" && ! grep -Fq "$value" "$path"; then
    echo "$path does not match distro/soda.toml version $version" >&2
    exit 1
  fi
}

contains cockpit/soda-projects/manifest.json "\"version\": \"$version\""
contains packaging/installer/branding/buildstamp "Version=$version"
contains packaging/installer/branding/buildstamp "UUID=SodaOS-$version"
contains packaging/installer/branding/os-release "VERSION=\"$minor\""
contains packaging/installer/branding/os-release "PRETTY_NAME=\"Soda OS $version\""
for package in soda-release soda-runtime soda-projects; do
  contains distro/locks/runtime-packages-x86_64.toml "$package-0:$version-"
done
