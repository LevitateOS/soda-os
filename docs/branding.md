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

`assets/branding/forgejo` contains the web raster assets shipped by
`soda-forgejo`. Open `assets/branding/forgejo/preview.html`
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
The shipped homepage uses the square symbol, the configured instance name,
and the configured meta description. The 440px horizontal layout remains an
alternative for review, not a replacement for the square navigation icon.

The RPM installs the canonical symbol as both `logo.svg` and `favicon.svg`;
there are no duplicate SVG masters or raster copies of the SVG-only
navigation/homepage slots. The Apple icon's background uses the
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

## Cockpit integration

The Soda Projects RPM owns `/usr/share/cockpit/branding/sodaos/`: the runtime
branding stylesheet, shared palette, Cockpit adapter, canonical symbol and
horizontal marks, light/dark login backgrounds, favicon ICO, and opaque Apple
touch icon.
`internal/build/image/rpm.go` stages the approved sources directly. Individual
favicon PNG proofs and the offline preview are not installed. Installer and
Forgejo branding are unchanged.

Cockpit discovers this directory through `ID=sodaos`. Administrator overrides in
`/etc/cockpit/branding/` retain precedence; Soda does not write there or replace
files in Cockpit's vendor `static` or `shell` packages. There is no custom login
page, shell, authentication script, banner, or login-title override. Hostname
identification, authentication conversations, password visibility, error and
warning states remain native.

`packaging/rpm/projects/sources/branding/sodaos/branding.css` imports the palette
and adapts Cockpit 366's standalone login color variables. Its stock `#brand`
hook supplies accessible wordmark images and the tagline; theme-specific CSS
selects the image, including when the system preference changes. The login card
uses a centered responsive grid and static decorative background. Inline/remote
login behavior still follows Cockpit's own visibility rules. The version-bound
selectors require browser revalidation when Cockpit changes.

`assets/branding/theme/palette.css` is the single color-value source for Forgejo,
the Cockpit shell, all shipped stock pages, and the four Soda packages.
`cockpit/src/cockpit/theme.css` maps those roles to PatternFly, with distinct
filled-action and link colors for dark-mode contrast. The native branding entry
imports installed copies of both files. Soda pages bundle those same sources via
`cockpit/src/cockpit/soda.css` and use the canonical symbol in their shared heading.
Artwork, backgrounds, native fonts and layout are unchanged. See the
[shared palette contract](../assets/branding/theme/README.md).

Cockpit 366 omits the branding stylesheet in Logs, Services, Terminal, hardware
information, and the standalone firewall
entry. The image build adds the same native `../../static/branding.css` link to
those five HTML entry points, after their own styles. This is a small maintained
vendor HTML adaptation, not a replacement page, script injection, or runtime
DOM patch. Recheck the entry points when upgrading Cockpit. Existing stylesheet
links are retained; missing expected HTML fails the image build.
Danger, warning, success and disabled-state colors remain native. Cockpit's
Default/Light/Dark selection and host-specific accent overrides are preserved.

Open `assets/branding/cockpit/preview.html` for the offline responsive placement
sheet. Its login panels are non-interactive design studies, not screenshots of
stock Cockpit or a replacement authentication UI. See
[`assets/branding/cockpit/README.md`](../assets/branding/cockpit/README.md) for the
asset inventory, palette, copy, usage rules, outstanding documentation-link
decision, and integration boundaries. These preview panels are not the installed layout;
the runtime adaptation uses Cockpit's own controls and fonts. No optional social
or About artwork is required for this scope.

`go run ./tools/render-cockpit-branding` requires Go and librsvg and regenerates
the four icon PNGs and the PNG-encoded ICO directly from the canonical symbol.
`go test ./tools/render-cockpit-branding` checks pixel freshness, dimensions,
opacity, ICO entries, and the intended text/control contrast pairs. Browser
review at desktop and narrow widths is asset evidence, not installed-product
acceptance. These assets are architecture-independent. See
[branding verification](cockpit-development.md#branding-verification) for the
browser commands and the distinction between simulated and installed evidence.
Documentation/help links remain deferred until a published destination is chosen.

## Forgejo themes

The Soda themes in `packaging/rpm/forgejo/sources/custom/public/assets/css`
extend Forgejo 15's native styles and map the shared palette's mode-qualified
values to native keys. The extraction preserved all 208 existing light/dark
color assignments. The palette reference is
[`soda-os-website` at `b9e37c7`](https://github.com/LevitateOS/soda-os-website/tree/b9e37c7e4bccb450c75574bd57a7c981e2f098f3),
particularly `src/app/styles/tokens.css` and `theme.css`. There is no runtime or
build-time dependency on the website repository, Tailwind, or an external CDN.

Light mode uses the website's warm `#fffdf8` canvas, white surfaces, `#f6f3ee`
panels, `#1c1917` ink, and `#155eef` primary blue. Dark mode uses `#0c1017`,
`#141a24`, and `#1b2332` surfaces with `#f0f4f8` ink. Neutral ramps fill the
intermediate roles required by native Forgejo controls. Mint, amber, and blue
map to success, warning, and information messages. Git diffs, error colors,
syntax highlighting, and Actions ANSI colors retain their native meanings and
palettes. The logo's cyan is not used for small text on white.

For contrast, dark links use the website's brighter `#60a5fa` blue, while filled
primary buttons use `#2563eb` with white text and darker hover/active states.
`soda-controls.css` binds native button variables locally, replaces Fomantic's
literal selected-primary blue, and provides visible keyboard focus, including
Fomantic fields that otherwise suppress outlines.
It does not introduce new control behavior, fonts, animations, or layouts.
`theme-soda-auto.css` follows the browser's color-scheme preference through
conditional CSS imports; explicit light/dark choices ignore that preference.
Stock and accessibility themes do not load these overrides.

### Theme review

`assets/branding/forgejo/theme-preview.html` is a review-only component sheet
using **actual native Forgejo CSS and the same Soda stylesheets that ship**.
Unlike the standalone artwork preview, it must be served from a disposable
matching-native Forgejo instance: copy it to
`$FORGEJO_CUSTOM/public/assets/soda-theme-preview.html` after staging the Soda
custom assets. It is deliberately excluded from the RPM. Open
`/assets/soda-theme-preview.html` to compare light, dark, automatic, and stock
modes without changing any user's saved preference.

With the repository's pinned Playwright/browser available, run:

```sh
node scripts/check-forgejo-branding.mjs \
  http://127.0.0.1:PORT/assets/soda-theme-preview.html \
  .artifacts/branding/themes
```

This read-only check exercises native primary button families and their
normal/hover/active/selected contrast, disabled controls, keyboard focus, preservation
of native diff/error/ANSI colors, automatic preference changes without reload,
explicit themes under the opposite OS preference, narrow screens, and 2x
screenshots. Source tests additionally check text and field/focus contrast.
Neither the component sheet nor these source checks establish installed-system
acceptance or complete WCAG conformance.

## Installed Forgejo branding

`soda-forgejo` owns `/usr/share/soda/forgejo/custom`. Its systemd service sets
`FORGEJO_CUSTOM` to that directory while keeping the explicit
`--config /etc/forgejo/app.ini`, native `git` service account, and writable data
under `/var/lib/forgejo`. Assets are read directly from the image; startup does
not copy them into mutable state. The two template overrides are `home.tmpl`
and `custom/header.tmpl` (the Apple touch-icon link). Native navigation, login,
registration, user/repository avatars, footer attribution, license links, and
repository functionality are not replaced.

New configurations set `APP_NAME = Soda OS`, the shared description
`Your team's repositories and collaboration.`, and `DEFAULT_THEME = soda-auto`.
`THEMES` adds the three Soda modes to **all** native Forgejo/Gitea and
color-vision accessibility choices. Users continue to select their own theme
through Forgejo's native Appearance settings. No saved preference is reset.

### Existing installations and administrator choices

The initializer still seeds `app.ini` only when it is absent. An image update
ships the artwork/templates but does **not** rewrite an existing instance name,
meta description, theme list/default, static-cache setting, or secrets. To
adopt the new defaults, merge the relevant settings from
`/usr/share/soda/forgejo/app.ini.tmpl` into the corresponding existing sections:

- Set the top-level `APP_NAME` to `Soda OS` if desired.
- In `[ui]`, add `soda-auto,soda-light,soda-dark` to the existing `THEMES` list,
  preserving custom choices, and set `DEFAULT_THEME = soda-auto`. If no list was
  explicitly configured, start from the complete list in the shipped template;
  do not accidentally replace native/accessibility choices with only Soda.
- In `[ui.meta]`, adopt the shipped author, description, and keywords if desired.
- In `[server]`, adopt `STATIC_CACHE_TIME = 0` for native cache revalidation.

Do not replace the entire file, duplicate INI sections, regenerate secrets, or
modify the database to change user themes. Restart `forgejo.service` through
native systemd after an approved configuration edit, then hard-refresh the
browser once to clear assets cached before the change.

**Operator custom files:** the previous default custom path was
`/var/lib/forgejo/custom`. Before updating an instance with local files there,
inspect its effective custom path. Retain the operator-owned path with a native
systemd drop-in (`[Service]` and
`Environment=FORGEJO_CUSTOM=/var/lib/forgejo/custom`) if needed. Existing
explicit drop-in overrides remain authoritative. Such installations can adopt
selected bundled files through native Forgejo customization; Soda does not
merge, overwrite, or delete operator templates. There is only one active
custom path, not a Soda-managed overlay system.

### Asset caching and upgrade checks

Forgejo's asset cache key uses its upstream version, which does not change
when only the Soda RPM release changes. New configurations therefore use native
`STATIC_CACHE_TIME = 0`: browsers revalidate assets, unchanged files can return
304, and changed local files are served without waiting for the upstream
six-hour default. This includes imported theme CSS. There is no custom cache
service or asset-version state.

Forgejo explicitly does not guarantee compatibility for custom templates,
artwork overrides, or CSS internals. On every Forgejo upgrade:

1. Recheck the custom paths, native theme imports/variables, template helpers,
   and complete native/accessibility theme list.
2. Run source tests and the component review on the matching-native binary.
3. Review the actual logged-out homepage, signup/login, repository/code pages,
   issues, pull requests/diffs, Actions, settings, and confirmation dialogs in
   light/dark/automatic modes, mobile widths, and high-density displays.
4. Verify keyboard focus, readable state colors, native saved theme selection,
   manifest/icon responses, and same-URL asset revalidation after an update.
5. On each architecture, verify the built RPM payload and installed read-only
   custom path under systemd/SELinux, new initialization, and update/reboot of
   an existing configuration. Keep native PAM/SSH clone/push smoke tests in the
   installed-system validation; branding tests are not substitutes.

The integration's source tests execute the spec's install section in a
scratch directory and compare installed assets/templates/CSS with their
sources; this is not an RPM build. Linux x86-64 validation exercised a disposable
Forgejo 15.0.7 process, native login and saved theme selection, repository/code,
issue/PR/diff/Actions/settings pages and dialogs, mobile/2x views, HTTP asset bytes
and MIME types, the manifest, and unchanged/updated cache responses. A process
restart retained the native account/repository and saved theme despite a
changed site default; the homepage respected operator text and escaped markup.
This is a process restart, not an OS update/reboot test. Actions
page coverage does not establish runner execution. No installed-system,
PAM/OpenSSH, or Apple-device result is implied. AArch64 must reproduce the native
browser checks; both architectures still require RPM/image and installed
fresh/update/reboot validation on matching hardware. Prerequisites are the
matching-native Forgejo/build inputs, browser, and disposable installed guest.

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
