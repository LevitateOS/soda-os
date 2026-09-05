# Soda OS Cockpit identity kit

The Soda Projects RPM installs the palette, two login backgrounds, favicon and
Apple touch icon into `/usr/share/cockpit/branding/sodaos/`, together with the
canonical symbol and light/dark wordmarks. The four Soda pages bundle this same
palette and canonical symbol. The preview and individual favicon PNG proofs are
not installed. This describes image source composition, not deployment to an
existing server.

Open `preview.html` locally to review both themes at 100% browser zoom. Its login
panels are non-interactive placement studies, not stock Cockpit screenshots or
replacement login pages. The source is self-contained and makes no network requests.

## Artwork inventory

Paths below are relative to this directory. Reuse approved SVG masters directly;
do not create a second copy of the symbol or wordmark for Cockpit.

| Placement | Source / deliverable | Treatment |
| --- | --- | --- |
| Login badge or square page identity | `../source/soda-symbol.svg` | Approved full-colour SVG, unchanged |
| Horizontal identity on light surfaces | `../source/soda-logo-horizontal.svg` | Approved lowercase outlined wordmark, unchanged |
| Horizontal identity on dark surfaces | `../source/soda-logo-horizontal-dark.svg` | Approved outlined wordmark and white symbol keyline, unchanged |
| Browser favicon | `favicon.ico` | PNG-encoded 16, 32, 48 px frames, each rendered directly from the symbol |
| Native-size favicon proofs | `favicon-16.png`, `favicon-32.png`, `favicon-48.png` | Transparent PNGs; also inputs to the ICO bundle |
| Apple shortcut | `apple-touch-icon.png` | 180 x 180 px, opaque approved navy; no baked-in corner mask |
| Login background, light | `login-background-light.svg` | Static 1920 x 1080 vector canvas, cover-cropped |
| Login background, dark | `login-background-dark.svg` | Static 1920 x 1080 vector canvas, cover-cropped |
| Shared theme | `palette.css` | Soda tokens and PatternFly 6 brand-token mappings |
| Asset and placement proof | `preview.html`, `preview.css` | Offline responsive review sheet, not runtime UI |

The background waves and bubbles echo the existing symbol without modifying it.
Use these images decoratively, behind an opaque login card. There is no motion,
so no reduced-motion alternative is needed. A plain canvas color is a sufficient
fallback. The separate animated-wave experiment is unchanged and is not used here.

## Color and type

| Role | Daylight | Midnight |
| --- | --- | --- |
| Canvas | `#F3F7FB` | `#081526` |
| Surface | `#FFFFFF` | `#11243C` |
| Main text | `#142D4E` | `#F4F8FC` |
| Secondary text | `#50627A` | `#B3C4D8` |
| Input boundary | `#77889B` | `#788BA1` |
| Primary action / link / focus | `#007885` | `#10D7E8` |
| Hover | `#006571` | `#65E7F1` |
| Pressed | `#00535E` | `#A0F2F8` |
| Text / icon on primary action | `#FFFFFF` | `#06245B` |

`palette.css` owns the values; this table documents their intended roles.
Keep native warning, danger, success, and disabled-state semantics. Do not recolor
the approved logo to match control colors. The preview's system font is only for
offline review: integration should retain Cockpit's native PatternFly fonts.

Tests measure at least 4.5:1 for main/secondary text and all brand interaction
states on the theme's canvas and surface, and for button labels on all three
brand states. Input boundaries and focus colors are checked at 3:1 against both
surfaces. These are token-pair measurements, not a complete UI accessibility audit.
Use underlines for text links. Use a 2px focus outline with 3px offset; do not
replace native keyboard focus with color alone.

Retain the established one-bubble-diameter clear space. The horizontal mark must
not be used below 114px wide. The proof uses 204px login lockups and up to 380px
identity samples. Symbol sizes are placement-dependent; inspect the favicon at
actual 16px, not just enlarged. No new font files or duplicate logo masters.

## Proposed copy and documentation

- Product identity: **Soda OS** (the approved artwork uses lowercase lettering).
- Login tagline: **Your development home.**
- Login instruction: keep Cockpit's **Log in with your server user account.**
- Page names: keep **Projects**, **Runners**, **Tailscale**, **Soda Updates** for now.
- Review-sheet headline: **Your workspace. Your machine.** This is proposed copy,
  not a new product guarantee or an instruction to add it to every page.

No documentation URL has been invented. Selecting a stable published Soda
handbook destination and contextual page links remains an integration decision.
The preview's documentation link opens this local asset guide, not a purported
public support site. No optional empty-state, social, or About artwork is needed
for this kit; the stock About dialog should remain intact.

## Reproduction and verification

From the repository root, with Go and `rsvg-convert` (librsvg) installed:

```sh
go run ./tools/render-cockpit-branding
go test ./tools/render-cockpit-branding
```

The renderer writes only the four PNGs and the ICO. It does not edit SVG masters,
CSS, previews, packages, or system settings. Tests compare freshly rendered pixels
with tracked PNGs and check every ICO frame against its PNG. The PNG-encoded ICO
is intended for modern browsers, not legacy Windows icon consumers.

These are architecture-independent assets. No image build, installation, or
native architecture support is established by generating them. Before shipping,
validate real Cockpit 366 behavior in both themes, keyboard/error states, narrow
screens, and favicon caching. Many but not all native pages load branding CSS;
keep this distinction and preserve the stock authentication flow. See
[`docs/cockpit-development.md`](../../../docs/cockpit-development.md#branding-verification)
for reference-DOM and real installed-session browser checks.
