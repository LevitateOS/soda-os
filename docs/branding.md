# Soda OS branding

Soda OS uses the approved terminal-and-bubbles symbol and lowercase `soda os`
wordmark. The source-of-truth artwork is SVG under `assets/branding/source`.
`assets/branding/soda-os-logo-concept-v3.png` is the approved visual reference.
Tracked PNGs under `assets/branding` and copies in Cockpit
consumers are deterministic derivatives; do not edit them by hand.

## Marks and variants

Use `soda-logo-horizontal.svg` as the primary mark on white or very light
backgrounds. Use `soda-logo-horizontal-dark.svg` on midnight-navy or other
dark backgrounds. `soda-logo-white.svg` is the reverse-wordmark variant;
`soda-logo-navy.svg` and `soda-logo-black.svg` are single-colour marks for
restricted reproduction. Use the approved full-colour `soda-symbol.svg`
whenever the name is already visible or space is square.

The symbol's white, navy, and black counterparts are reduced one-colour line
marks for flat, high-contrast backgrounds. They retain the outer circle,
terminal prompt, command line, and three bubbles while omitting the liquid wave
that cannot remain distinct in one colour. Use each only where it contrasts
clearly with the background.

There is intentionally no stacked lockup: no shipped Soda OS surface needs one
and the horizontal mark remains more legible at the available Cockpit widths.
The approved `soda-symbol.svg` master produces every icon and favicon size.

Keep clear space equal to one bubble diameter around every mark. Do not use the
horizontal wordmark below 114 px wide. Do not recolour, outline, stretch,
rotate, add effects, or place the full-colour mark on cyan or busy backgrounds.
Customer-facing branding must say Soda OS only.

## Outputs and regeneration

`assets/branding/icons` contains the hicolor installed-system icons at 16, 24,
32, 48, 64, 128, 256,
and 512 px. `assets/branding/web` contains favicon and touch-icon PNGs. Cockpit
embeds its synchronized copies from `cockpit/internal/web/static`.

`assets/branding/installer/manifest.tsv` maps every approved SVG master to a
managed Anaconda raster derivative. Horizontal lockups use the established
114x36 slot and symbols use a 256x256 canvas. The dark horizontal lockup is the
installer sidebar logo and the full-colour symbol is its square product mark;
the other seven derivatives remain managed variants without invented UI
placements. Navy sidebar and top-bar backgrounds are CSS colors taken from the
same artwork rather than duplicated raster images. Run
`scripts/render-installer-branding.sh` to regenerate the set, or pass `--check`
to verify that all tracked outputs are current.

Run the following from the repository root after changing an SVG:

```sh
scripts/render-branding.sh
just check
```

The renderer requires the macOS Swift toolchain/AppKit and ImageMagick. AppKit
rasterizes the SVGs and ImageMagick sizes its lossless TIFF output and
synchronizes direct consumers. Review every regenerated derivative before
shipping. Installer rendering separately requires `rsvg-convert` from librsvg.
