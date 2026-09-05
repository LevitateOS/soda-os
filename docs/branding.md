# Soda OS branding

Soda OS uses the approved terminal-and-bubbles symbol and lowercase `soda os`
wordmark. The source-of-truth artwork is SVG under `assets/branding/source`.
`assets/branding/soda-os-logo-concept-v3.png` is the approved visual reference.
Tracked PNGs under `assets/branding` are deterministic derivatives; do not edit
them by hand. Stock Cockpit consumes the canonical symbol directly through the
Soda branding package.

## Marks and variants

Use `soda-logo-horizontal.svg` as the primary mark on white or very light
backgrounds. Use `soda-logo-horizontal-dark.svg` on midnight-navy or other
dark backgrounds. Its built-in white keyline keeps the full-colour symbol
distinct when the background matches the symbol's navy fill.
`soda-logo-white.svg`, `soda-logo-navy.svg`, and `soda-logo-black.svg` are
single-colour marks for restricted reproduction. Use the approved full-colour
`soda-symbol.svg` whenever the name is already visible or space is square; its
white keyline protects the symbol boundary on dark backgrounds.

The symbol's white, navy, and black counterparts are reduced one-colour line
marks for flat, high-contrast backgrounds. They retain the outer circle,
terminal prompt, command line, and three bubbles while omitting the liquid wave
that cannot remain distinct in one colour. Use each only where it contrasts
clearly with the background.

There is intentionally no stacked lockup: no shipped Soda OS surface needs one
and the horizontal mark remains more legible at the available Cockpit widths.
The approved `soda-symbol.svg` master produces every icon and favicon size.

Keep clear space equal to one bubble diameter around every mark. Do not use the
horizontal wordmark below 114 px wide. Do not recolour, add another outline,
stretch, rotate, add effects, or place the full-colour mark on cyan or busy
backgrounds. Never place a one-colour mark on the same colour background.
Customer-facing branding must say Soda OS only.

## Outputs and regeneration

`assets/branding/icons` contains the hicolor installed-system icons at 16, 24,
32, 48, 64, 128, 256,
and 512 px. `assets/branding/web` contains favicon and touch-icon PNGs for
consumers that require raster web artwork. Stock Cockpit branding is shipped
from `packaging/rpm/projects/sources/branding/sodaos` and the canonical symbol.

`assets/branding/installer/manifest.tsv` maps every approved SVG master to a
managed Anaconda raster derivative. Horizontal lockups use the established
114x36 slot and symbols use a 256x256 canvas. The dark horizontal lockup is the
installer sidebar logo and the full-colour symbol is its square product mark;
the other seven derivatives remain managed variants without invented UI
placements. Navy sidebar and top-bar backgrounds are CSS colors taken from the
same artwork rather than duplicated raster images. Run
`scripts/render-installer-branding.sh` to regenerate the set, or pass `--check`
to verify that all tracked outputs are current.

## Forgejo asset preparation

`assets/branding/forgejo` contains the prepared web raster assets, not an
installed Forgejo customization. Open `assets/branding/forgejo/preview.html`
directly in a browser at 100% zoom to review the light/dark placements without
a server or network connection. It compares 1x and 2x favicon samples and two
homepage layouts; it is not an installed-product screenshot or visual approval.

| Placement | Artwork | Size and treatment |
| --- | --- | --- |
| Navigation and native square homepage slot | `source/soda-symbol.svg` | SVG at 30x30 and 220x220 CSS px respectively |
| SVG favicon | `source/soda-symbol.svg` | SVG; review at 16 CSS px on normal and high-density displays |
| Small favicon proof / optional 16px PNG consumer | `forgejo/favicon-16.png` | 16x16, transparent |
| Forgejo PNG favicon fallback | `forgejo/favicon.png` | 32x32, transparent; also a 2x sample at 16 CSS px |
| Apple touch icon | `forgejo/apple-touch-icon.png` | 180x180, opaque midnight-navy canvas; the platform owns corner masking |
| Native web-app manifest and default social image | `forgejo/logo.png` | 512x512, transparent; not a user or repository avatar |
| Alternative homepage lockup | `source/soda-logo-horizontal.svg` and `source/soda-logo-horizontal-dark.svg` | SVG at 440x110 CSS px, scaling proportionally on narrow screens |

Paths in this table are relative to `assets/branding`. The native 30px and
220px placements come from Forgejo 15.0.7, not permanent Soda layout rules.
The 440px horizontal layout is an alternative for review, not a replacement
for the square navigation icon. Homepage copy in the preview is proposed copy.

Use the canonical symbol as both `logo.svg` and `favicon.svg` when integrating;
do not maintain duplicate SVG masters or manufacture raster copies of the
SVG-only navigation/homepage slots. The Apple icon's background uses the
approved navy without recolouring the mark or adding baked-in rounded corners.
The existing Cockpit, installer, and general web exports are unchanged.

`assets/branding/forgejo/manifest.tsv` records each PNG's source, output, square
size, and background. `scripts/render-forgejo-branding.sh` renders directly from
the SVG with `rsvg-convert` (librsvg), without scaling an intermediate PNG.
Pass `--check` to compare decoded pixels with freshly rendered outputs using
the existing Go `tools/png-equal` command. `go test ./scripts` includes this
freshness check and verifies dimensions and transparency. These assets are
architecture-independent; no native RPM or image support is established by
rendering them.

## Cockpit asset preparation

`assets/branding/cockpit` contains the prepared light/dark login backgrounds,
PatternFly brand-token palette, native-size favicon PNGs, a multi-resolution ICO,
and an opaque Apple touch icon. It references the existing approved symbol and
horizontal SVG masters without duplicating or redrawing them. Nothing in this
kit is installed into Cockpit yet; existing Cockpit, installer, and Forgejo
branding is unchanged.

Open `assets/branding/cockpit/preview.html` for the offline responsive placement
sheet. Its login panels are non-interactive design studies, not screenshots of
stock Cockpit or a replacement authentication UI. See
[`assets/branding/cockpit/README.md`](../assets/branding/cockpit/README.md) for the
asset inventory, palette, copy, usage rules, outstanding documentation-link
decision, and integration boundaries. No optional social or About artwork is
required for this scope.

`go run ./tools/render-cockpit-branding` requires Go and librsvg and regenerates
the four icon PNGs and the PNG-encoded ICO directly from the canonical symbol.
`go test ./tools/render-cockpit-branding` checks pixel freshness, dimensions,
opacity, ICO entries, and the intended text/control contrast pairs. Browser
review at desktop and narrow widths is asset evidence, not installed-product
acceptance. These assets are architecture-independent.

## Regenerating all derivatives

Run the following from the repository root after changing an SVG:

```sh
scripts/render-branding.sh
scripts/render-installer-branding.sh
scripts/render-forgejo-branding.sh
go run ./tools/render-cockpit-branding
just check
```

The general web/system renderer requires the macOS Swift toolchain/AppKit and
ImageMagick. AppKit rasterizes the SVGs and ImageMagick sizes its lossless TIFF
output. Installer and Forgejo rendering separately require `rsvg-convert` from
librsvg. Review every regenerated derivative before shipping.
