#!/bin/sh
# Render the Anaconda bitmap assets from the approved Soda OS SVG masters.
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_root"

mode=${1:-write}
case "$mode" in
write|--check)
    ;;
--list)
    sed '/^#/d; /^$/d' assets/branding/installer/manifest.tsv
    exit 0
    ;;
*)
    echo "usage: $0 [--check|--list]" >&2
    exit 2
    ;;
esac

command -v rsvg-convert >/dev/null 2>&1 || {
    echo "rsvg-convert is required to render installer branding" >&2
    exit 1
}

workdir=$(mktemp -d)
trap 'rm -rf "$workdir"' EXIT HUP INT TERM

install_asset() {
    generated=$1
    tracked=$2
    if [ "$mode" = "--check" ]; then
        if ! cmp -s "$generated" "$tracked"; then
            echo "$tracked is missing or stale; run scripts/render-installer-branding.sh" >&2
            return 1
        fi
        return 0
    fi
    install -m 0644 "$generated" "$tracked"
}

tab=$(printf '\t')
while IFS="$tab" read -r kind source output role; do
    case "$kind" in
    \#*|'')
        continue
        ;;
    horizontal)
        generated="$workdir/$(basename "$output")"
        # Preserve each 4:1 lockup in the established 114x36 Anaconda slot.
        rsvg-convert \
            --format=png \
            --width=114 \
            --height=28.5 \
            --page-width=114 \
            --page-height=36 \
            --top=3.75 \
            --output="$generated" \
            "$source"
        ;;
    symbol)
        generated="$workdir/$(basename "$output")"
        rsvg-convert \
            --format=png \
            --width=256 \
            --height=256 \
            --output="$generated" \
            "$source"
        ;;
    *)
        echo "unknown installer branding asset kind $kind for $source" >&2
        exit 1
        ;;
    esac
    install_asset "$generated" "$output"
done < assets/branding/installer/manifest.tsv
