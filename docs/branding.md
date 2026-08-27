# Soda OS branding

Soda OS uses the approved terminal-and-bubbles symbol and lowercase `soda os`
wordmark. The source-of-truth artwork is SVG under `assets/branding/source`.
`assets/branding/soda-os-logo-concept-v3.png` is the approved visual reference;
the earlier concept PNGs are retained only as historical exploration.
Tracked PNGs under `assets/branding` and copies in Cockpit
consumers are deterministic derivatives; do not edit them by hand.

## Marks and variants

Use `soda-logo-horizontal.svg` as the primary mark on white or very light
backgrounds. Use `soda-logo-horizontal-dark.svg` on midnight-navy or other
dark backgrounds. `soda-logo-white.svg` is the reverse-wordmark variant;
`soda-logo-navy.svg` and `soda-logo-black.svg` are single-colour marks for
restricted reproduction. Use `soda-symbol.svg` whenever the name is already
visible or space is square. Its white, navy, and black counterparts have
matching filenames. `soda-symbol-monochrome.svg` is retained as the earlier
black-symbol filename for compatibility.

There is intentionally no stacked lockup: no shipped Soda OS surface needs one
and the horizontal mark remains more legible at the available Cockpit widths.
`soda-symbol-small.svg` is the approved master symbol for 16,
24, and 32 px icons and favicons; it retains the same geometry as larger uses.

Keep clear space equal to one bubble diameter around every mark. Do not use the
horizontal wordmark below 114 px wide or the standard symbol below 32 px. The
small symbol is the only approved mark below 32 px. Do not recolour, outline,
stretch, rotate, add effects, or place the full-colour mark on cyan or busy
backgrounds. Customer-facing branding must say Soda OS only; Rocky attribution
belongs in technical documentation.

## Outputs and regeneration

`assets/branding/icons` contains the hicolor installed-system icons at 16, 24,
32, 48, 64, 128, 256,
and 512 px. `assets/branding/web` contains favicon and touch-icon PNGs. Cockpit
embeds its synchronized copies from `cockpit/internal/server/static`.

Run the following from the repository root after changing an SVG:

```sh
scripts/render-branding.sh
just check
```

The renderer requires the macOS Swift toolchain/AppKit and ImageMagick. AppKit
rasterizes the SVGs and ImageMagick sizes its lossless TIFF output and
synchronizes direct consumers. Review every regenerated derivative before
shipping.
