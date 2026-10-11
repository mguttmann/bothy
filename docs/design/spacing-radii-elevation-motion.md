# Spacing, radii, elevation, motion, density

Sources: `design/tokens/space.tokens.json`, `elevation.tokens.json`, `motion.*.tokens.json`,
`density.*.tokens.json`. Generated values: `design/build/tokens.css`, `design/build/dart/`.
The JSON files are authoritative; the tables here show only the names and the intent.

## Spacing

Grid 4 px. In addition 2 px and 6 px for dense lists and chips. The names are the pixel values
(`space.4`, `space.12`), because the grid is primitive; spacing gets its meaning in the
component specifications.

| Token | Typical use |
|---|---|
| `space.2`, `space.4` | gap from icon to text in chips, row padding compact |
| `space.6`, `space.8` | chips next to each other, list rows, button groups |
| `space.12`, `space.16` | column inner margin, live box padding, gap from avatar to text |
| `space.20`, `space.24` | sections in settings and console |
| `space.32` to `space.64` | empty states, setup |

## Radii

Tight radii: the product is a tool. `radius.pill` only for counters and badges.

| Token | Use |
|---|---|
| `radius.xs` | inline code, keycaps |
| `radius.sm` | buttons, chips, channel rows, input fields |
| `radius.md` | live box, cards, agent avatars, code blocks |
| `radius.lg` | input, menus, quick switcher |
| `radius.xl` | phone frame in the preview, large dialogs |
| `radius.pill` | counters, badges |

Shape language: people have round avatars, agents rounded squares (`radius.md`). This makes it
clear at a glance who wrote, even without color.

## Borders

| Token | Value and purpose |
|---|---|
| `border-width.hairline` | column borders, cards, row separators |
| `border-width.focus` | focus ring, at least 2 px (target WCAG 2.2 SC 2.4.13) |
| `border-width.spine` | spine on the left of the live box and mentions, color from the state |

## Elevation

Surfaces lie flat and are separated by color (`color.surface.*`) and lines. Shadows exist
only for layers above the content:

| Token | Use |
|---|---|
| `elevation.menu` | context menu, suggestion list, tooltip |
| `elevation.dialog` | dialog, quick switcher, grant wizard |

The shadow color `color.shadow` is specific to each theme (darker and denser in the dark theme, so
that the shadow shows at all on dark surfaces). No glow, no colored shadows, no
blur behind layers.

## Motion

Motion shows a state change and never decorates. Two contexts in the resolver:

| Token | `full` | `reduced` | Use |
|---|---|---|---|
| `motion.duration.move-fast` | short | 0 | collapsing spaces, hover |
| `motion.duration.move` | medium | 0 | live box collapses, columns |
| `motion.duration.move-slow` | long | 0 | own transitions in the navigation stack (the platform transition takes precedence) |
| `motion.duration.fade` | short | shorter | cross-fade without movement |
| `motion.duration.pulse` | period | 0 | spinning of the run icon, pulse of the run dot; 0 means static |
| `motion.easing.standard`, `.enter`, `.exit` | curves | linear | |

Rules:

- `reduced` applies when the system setting "Reduce motion" is active or the person
  has chosen `appearance.reduce_motion = on` (`off` forces `full`). WCAG 2.2 SC 2.3.3 is the
  target (https://www.w3.org/TR/WCAG22/, retrieved 2026-10-10 [verified]).
- A running agent shows a static icon in the `reduced` context; the state stays recognizable
  through icon shape and caption.
- Live box patches animate nothing. New steps appear without flying in; only the pulse of the
  state icon shows "running".
- No parallax, no shaking on errors, no confetti effects.

## Density

Three contexts in the resolver; the app chooses the context, not the person alone:

| Context | Applies to | Choice |
|---|---|---|
| `comfortable` | desktop, web console | default |
| `compact` | desktop | setting `appearance.density = compact` |
| `touch` | iPhone, Android | fixed |

| Token | Meaning |
|---|---|
| `density.row-channel` | height of a channel row |
| `density.row-post` | height of a post row in the forum (two lines) |
| `density.row-suggestion` | row in the suggestion list and the quick switcher |
| `density.control-height` | buttons, select fields |
| `density.target-min` | smallest hit area: WCAG 2.2 SC 2.5.8 on desktop, 48 dp on touch |
| `density.message-gap`, `density.message-pad-y` | spacing in the history |
| `density.message-font-size` | message text |
| `density.livebox-pad` | padding of the live box |
| `density.composer-min-height` | minimum height of the input |

Evidence for hit areas: WCAG 2.2 SC 2.5.8 requires 24 by 24 CSS pixels
(https://www.w3.org/WAI/WCAG22/Understanding/target-size-minimum.html, retrieved 2026-10-10
[verified]); Android recommends 48 dp
(https://support.google.com/accessibility/android/answer/7101858, retrieved 2026-10-10 [verified]);
Apple names 44 pt [derived from secondary sources, primary page of the Human Interface Guidelines
not read]. `touch` uses the larger Material value so that both
platforms are satisfied. All numbers: [Token reference](tokens.md).

The visible area of an element may be smaller than `density.target-min`; the hit area must never be.
Flutter tests check this with `androidTapTargetGuideline` and `iOSTapTargetGuideline`
(https://api.flutter.dev/flutter/flutter_test/meetsGuideline.html, retrieved 2026-10-10 [verified]).
