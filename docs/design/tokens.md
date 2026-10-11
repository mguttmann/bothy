# Token-Referenz (ohne Farben)

> GENERIERT von tools/design-tokens/tokens.py aus design/tokens. Nicht bearbeiten; Quelle ändern und neu erzeugen.

Alle Werte außer Farben, wie sie die Apps und `tokens.css` bekommen. Farben: [palette.md](palette.md).
Hand geschriebene Seiten nennen nur Token-Namen; die Zahlen stehen hier und sind durch
`tokens.py check` an die Quelle gebunden.

## Basis (in allen Kontexten gleich)

| Token | Typ | Wert |
|---|---|---|
| `border-width.focus` | dimension | 2px |
| `border-width.hairline` | dimension | 1px |
| `border-width.spine` | dimension | 3px |
| `font.family.mono` | fontFamily | "Atkinson Hyperlegible Mono", ui-monospace, "SF Mono", "Cascadia Mono", "Consolas", monospace |
| `font.family.sans` | fontFamily | "Atkinson Hyperlegible Next", system-ui, -apple-system, "Segoe UI", "Roboto", sans-serif |
| `font.size.12` | dimension | 12px |
| `font.size.13` | dimension | 13px |
| `font.size.14` | dimension | 14px |
| `font.size.15` | dimension | 15px |
| `font.size.16` | dimension | 16px |
| `font.size.18` | dimension | 18px |
| `font.size.20` | dimension | 20px |
| `font.size.24` | dimension | 24px |
| `font.size.30` | dimension | 30px |
| `font.tracking.normal` | dimension | 0px |
| `font.tracking.snug` | dimension | -0.15px |
| `font.tracking.tight` | dimension | -0.4px |
| `font.tracking.wide` | dimension | 0.3px |
| `font.weight.bold` | fontWeight | 700 |
| `font.weight.extrabold` | fontWeight | 800 |
| `font.weight.medium` | fontWeight | 500 |
| `font.weight.regular` | fontWeight | 400 |
| `font.weight.semibold` | fontWeight | 600 |
| `icon.size.lg` | dimension | 24px |
| `icon.size.md` | dimension | 20px |
| `icon.size.sm` | dimension | 16px |
| `layout.column-min` | dimension | 200px |
| `layout.list-width` | dimension | 340px |
| `layout.message-max-width` | dimension | 880px |
| `layout.overlay-width` | dimension | 560px |
| `layout.session-width` | dimension | 296px |
| `layout.sidebar-width` | dimension | 248px |
| `radius.lg` | dimension | 8px |
| `radius.md` | dimension | 6px |
| `radius.none` | dimension | 0px |
| `radius.pill` | dimension | 999px |
| `radius.sm` | dimension | 4px |
| `radius.xl` | dimension | 12px |
| `radius.xs` | dimension | 2px |
| `size.avatar-lg` | dimension | 40px |
| `size.avatar-md` | dimension | 32px |
| `size.avatar-sm` | dimension | 20px |
| `size.badge-height` | dimension | 20px |
| `size.status-dot` | dimension | 8px |
| `space.0` | dimension | 0px |
| `space.12` | dimension | 12px |
| `space.16` | dimension | 16px |
| `space.2` | dimension | 2px |
| `space.20` | dimension | 20px |
| `space.24` | dimension | 24px |
| `space.32` | dimension | 32px |
| `space.4` | dimension | 4px |
| `space.40` | dimension | 40px |
| `space.48` | dimension | 48px |
| `space.6` | dimension | 6px |
| `space.64` | dimension | 64px |
| `space.8` | dimension | 8px |
| `type.body` | typography | font-family "Atkinson Hyperlegible Next", system-ui, -apple-system, "Segoe UI", "Roboto", sans-serif, font-size 15px, font-weight 400, line-height 1.467, letter-spacing 0px |
| `type.body-sm` | typography | font-family "Atkinson Hyperlegible Next", system-ui, -apple-system, "Segoe UI", "Roboto", sans-serif, font-size 14px, font-weight 400, line-height 1.43, letter-spacing 0px |
| `type.body-strong` | typography | font-family "Atkinson Hyperlegible Next", system-ui, -apple-system, "Segoe UI", "Roboto", sans-serif, font-size 15px, font-weight 600, line-height 1.467, letter-spacing 0px |
| `type.caption` | typography | font-family "Atkinson Hyperlegible Next", system-ui, -apple-system, "Segoe UI", "Roboto", sans-serif, font-size 12px, font-weight 400, line-height 1.333, letter-spacing 0px |
| `type.code` | typography | font-family "Atkinson Hyperlegible Mono", ui-monospace, "SF Mono", "Cascadia Mono", "Consolas", monospace, font-size 14px, font-weight 400, line-height 1.43, letter-spacing 0px |
| `type.code-sm` | typography | font-family "Atkinson Hyperlegible Mono", ui-monospace, "SF Mono", "Cascadia Mono", "Consolas", monospace, font-size 12px, font-weight 500, line-height 1.333, letter-spacing 0px |
| `type.display` | typography | font-family "Atkinson Hyperlegible Next", system-ui, -apple-system, "Segoe UI", "Roboto", sans-serif, font-size 30px, font-weight 800, line-height 1.2, letter-spacing -0.4px |
| `type.heading` | typography | font-family "Atkinson Hyperlegible Next", system-ui, -apple-system, "Segoe UI", "Roboto", sans-serif, font-size 16px, font-weight 600, line-height 1.375, letter-spacing 0px |
| `type.label` | typography | font-family "Atkinson Hyperlegible Next", system-ui, -apple-system, "Segoe UI", "Roboto", sans-serif, font-size 13px, font-weight 600, line-height 1.385, letter-spacing 0px |
| `type.section` | typography | font-family "Atkinson Hyperlegible Next", system-ui, -apple-system, "Segoe UI", "Roboto", sans-serif, font-size 12px, font-weight 700, line-height 1.333, letter-spacing 0.3px |
| `type.title` | typography | font-family "Atkinson Hyperlegible Next", system-ui, -apple-system, "Segoe UI", "Roboto", sans-serif, font-size 20px, font-weight 700, line-height 1.3, letter-spacing -0.15px |

## Modifikator `theme` (Standard: `dark`)

| Token | Typ | `light` | `dark` |
|---|---|---|---|
| `elevation.dialog` | shadow | 0px 4px 12px 0px rgb(8 25 28 / 0.16), 0px 24px 48px -8px rgb(8 25 28 / 0.16) | 0px 4px 12px 0px rgb(6 20 22 / 0.5), 0px 24px 48px -8px rgb(6 20 22 / 0.5) |
| `elevation.menu` | shadow | 0px 2px 6px 0px rgb(8 25 28 / 0.16), 0px 8px 20px -4px rgb(8 25 28 / 0.16) | 0px 2px 6px 0px rgb(6 20 22 / 0.5), 0px 8px 20px -4px rgb(6 20 22 / 0.5) |

## Modifikator `density` (Standard: `comfortable`)

| Token | Typ | `comfortable` | `compact` | `touch` |
|---|---|---|---|---|
| `density.composer-min-height` | dimension | 44px | 40px | 48px |
| `density.control-height` | dimension | 32px | 28px | 44px |
| `density.livebox-pad` | dimension | 12px | 8px | 12px |
| `density.message-font-size` | dimension | 15px | 15px | 16px |
| `density.message-gap` | dimension | 16px | 8px | 16px |
| `density.message-pad-y` | dimension | 4px | 2px | 6px |
| `density.row-channel` | dimension | 32px | 28px | 44px |
| `density.row-post` | dimension | 52px | 44px | 64px |
| `density.row-suggestion` | dimension | 36px | 30px | 48px |
| `density.target-min` | dimension | 24px | 24px | 48px |

## Modifikator `motion` (Standard: `full`)

| Token | Typ | `full` | `reduced` |
|---|---|---|---|
| `motion.duration.fade` | duration | 120ms | 80ms |
| `motion.duration.move` | duration | 180ms | 0ms |
| `motion.duration.move-fast` | duration | 120ms | 0ms |
| `motion.duration.move-slow` | duration | 260ms | 0ms |
| `motion.duration.pulse` | duration | 1400ms | 0ms |
| `motion.easing.enter` | cubicBezier | cubic-bezier(0, 0, 0.2, 1) | cubic-bezier(0, 0, 1, 1) |
| `motion.easing.exit` | cubicBezier | cubic-bezier(0.4, 0, 1, 1) | cubic-bezier(0, 0, 1, 1) |
| `motion.easing.standard` | cubicBezier | cubic-bezier(0.2, 0, 0, 1) | cubic-bezier(0, 0, 1, 1) |
