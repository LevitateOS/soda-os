# Shared Soda palette

`palette.css` is the source of color values for Soda's frontends. Its light and
dark roles preserve the approved Forgejo themes, based on the Soda website at
`b9e37c7`. It contains values, not application selectors, native variable names,
layout, theme switching, or asset URLs.

- Canvas, surface, panel, navigation, text, muted text, border, focus, hover and
  selection are separate roles.
- `action` / `on-action` describe filled primary buttons. `link` / `on-link`
  describe link/accent treatments. Dark links are brighter than filled buttons;
  do not collapse these roles because light mode happens to use the same blue.
- Mode-qualified values let each application keep its own native theme selection.
- Additional ramps and alpha values preserve existing Forgejo control colors.
  Only distinct extra ramp values are named; adapters reuse semantic values where
  those already supply a ramp step. Alpha bytes remain exact, not rounded mixes.
- Forgejo's existing success/warning roles are retained. Cockpit continues to use
  its native status palette. Git diff, syntax and terminal ANSI colors stay native.

Application adapters map these values to native keys and own control fixes.
Forgejo's theme entries import the palette alongside their native base theme.
Cockpit's frontend entry imports the palette and its PatternFly adapter; the
native branding entry imports installed copies of those same sources.

Vite bundles CSS for the four Soda pages. Existing RPM staging copies the same
palette into each application's own public assets. Independent build outputs are
intentional; never maintain a second source copy or fetch CSS from another service.
There is no npm package, generator, runtime theme service, or extra build tool.

When changing colors, run the palette contrast and packaging tests, then render
both applications in light, dark and automatic modes. Test actual controls and
focus states, not just custom-property declarations. Preserve explicit theme
choices under the opposite system preference. Test staging/import paths separately
from rendered colors; their failure modes are different.
