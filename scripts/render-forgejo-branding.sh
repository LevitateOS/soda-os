#!/bin/sh
# Render Forgejo's square PNG assets directly from the approved SVG master.
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_root"

mode=${1:-write}
if [ "$#" -gt 1 ]; then
    echo "usage: $0 [--check]" >&2
    exit 2
fi
case "$mode" in
write|--check) ;;
*)
    echo "usage: $0 [--check]" >&2
    exit 2
    ;;
esac

command -v rsvg-convert >/dev/null 2>&1 || {
    echo "rsvg-convert is required to render Forgejo branding" >&2
    exit 1
}

workdir=$(mktemp -d)
trap 'rm -rf "$workdir"' EXIT HUP INT TERM

tab=$(printf '\t')
while IFS="$tab" read -r source output size background; do
    case "$source" in
    \#*|'') continue ;;
    esac
    generated="$workdir/$(basename "$output")"
    rsvg-convert \
        --format=png \
        --width="$size" \
        --height="$size" \
        --keep-aspect-ratio \
        --background-color="$background" \
        --output="$generated" \
        "$source"
    if [ "$mode" = "--check" ]; then
        if ! go run ./tools/png-equal "$generated" "$output"; then
            echo "$output is missing or stale; run scripts/render-forgejo-branding.sh" >&2
            exit 1
        fi
    else
        install -m 0644 "$generated" "$output"
    fi
done < assets/branding/forgejo/manifest.tsv
