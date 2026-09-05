#!/bin/sh
# Explicit build-host setup; never executed by an appliance RPM.
set -eu
installer=$(mktemp)
trap 'rm -f "$installer"' EXIT HUP INT TERM
curl --fail --silent --show-error --location \
  https://raw.githubusercontent.com/voidzero-dev/vite-plus/b2d15e3899dcc8adedfd45d98de9d30046a624f4/packages/cli/install.sh \
  --output "$installer"
VP_VERSION=0.3.0 VP_NODE_MANAGER=yes CI=true bash "$installer"
