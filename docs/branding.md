# Soda OS branding

Soda OS uses the approved terminal-and-bubbles symbol and lowercase `soda os`
wordmark. The source-of-truth artwork is SVG under `assets/branding/source`.
Tracked PNGs under `assets/branding` and copies in installer or Cockpit
consumers are deterministic derivatives; do not edit them by hand.

## Marks and variants

Use `soda-logo-horizontal.svg` as the primary mark on white or very light
backgrounds. Use `soda-logo-horizontal-dark.svg` on midnight-navy or other
dark backgrounds. `soda-logo-white.svg` is the all-white reverse mark;
`soda-logo-navy.svg` and `soda-logo-black.svg` are single-colour marks for
restricted reproduction. Use `soda-symbol.svg` whenever the name is already
visible or space is square. Its white, navy, and black counterparts have
matching filenames. `soda-symbol-monochrome.svg` is retained as the earlier
black-symbol filename for compatibility.

`soda-logo-sidebar.svg` is an installer-only optical lockup. Its filled,
compact letterforms preserve the approved wordmark at Anaconda's fixed
114 × 36 px allocation; do not use it at larger sizes.

There is intentionally no stacked lockup: no shipped Soda OS surface needs one
and the horizontal mark remains more legible at the available installer and
Cockpit widths. `soda-symbol-small.svg` is only for 16, 24, and 32 px icons and
favicons. It omits the lower liquid wave so the terminal prompt and bubbles
remain distinct; all larger uses retain the approved geometry.

Keep clear space equal to one bubble diameter around every mark. Do not use the
horizontal wordmark below 114 px wide or the standard symbol below 32 px. The
small symbol is the only approved mark below 32 px. Do not recolour, outline,
stretch, rotate, add effects, or place the full-colour mark on cyan or busy
backgrounds. Customer-facing branding must say Soda OS only; Rocky attribution
belongs in technical documentation.

## Outputs and regeneration

`assets/branding/installer` supplies Anaconda and GRUB. `assets/branding/icons`
contains the hicolor installed-system icons at 16, 24, 32, 48, 64, 128, 256,
and 512 px. `assets/branding/web` contains favicon and touch-icon PNGs. Cockpit
embeds its synchronized copies from `cockpit/internal/server/static`.

Run the following from the repository root after changing an SVG:

```sh
scripts/render-branding.sh
just check
```

The renderer requires the macOS Swift toolchain/AppKit, ImageMagick, and
`shasum`. AppKit rasterizes the SVGs and ImageMagick sizes its lossless TIFF
output, synchronizes direct consumers, and rewrites
`packaging/anaconda/branding.toml` with the rendered dimensions and SHA-256
values. Review every regenerated derivative and manifest entry before shipping.
