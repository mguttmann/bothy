# Typography

Source: `design/tokens/typography.tokens.json`. Values in px; the apps scale them with the
system setting (iOS Dynamic Type, Android font size, Windows text size) through
`MediaQuery.textScaler`. Images: [Components](previews/components-dark.png).

## Font choice

| Role | Font | License | Axes |
|---|---|---|---|
| Interface, messages | Atkinson Hyperlegible Next | SIL Open Font License 1.1 | variable, weight 200 to 800, with italic |
| Code, tool names, IDs, metrics | Atkinson Hyperlegible Mono | SIL Open Font License 1.1 | variable, weight 200 to 800, with italic |

Evidence: package data of `@fontsource-variable/atkinson-hyperlegible-next@5.3.0` and
`@fontsource-variable/atkinson-hyperlegible-mono@5.3.0` (license OFL-1.1, axis `wght` 200 to
800; https://fontsource.org/fonts/atkinson-hyperlegible-next, retrieved 2026-10-10 [verified]).
Source repositories: https://github.com/googlefonts/atkinson-hyperlegible-next and
https://github.com/googlefonts/atkinson-hyperlegible-next-mono (retrieved 2026-10-10 [verified]).
The font family comes from the Braille Institute and is designed for people with low vision;
the "Next" version appeared in 2025 with seven weights and a mono variant [verified through the
Fontsource page and press reports, primary page of the Braille Institute not read].

Reasons:

1. **Readability as a property of the font.** Confusable characters (I, l, 1; 0, O) are clearly
   distinct. In a chat with IDs, paths, commands and code, this is a working advantage, not
   decoration.
2. **A face of its own.** The font is recognizably different from the usual grotesques (Inter,
   system fonts) of the generic look and different from Discord's house font.
3. **One family for text and code.** Next and Mono share shapes and letter spacing; metrics in
   the live box (mono) sit calmly next to running text.
4. **Variable.** One font file per style covers all weights; this keeps app bundles small.
5. **Free license.** OFL allows embedding in apps and redistribution; the license text is included
   (`tools/design-tokens/vendor/fonts/*.OFL.txt`, likewise in the app package).

Study B recommended system fonts (SF, Segoe UI). The requirement "unmistakable" and
readability weigh more; the system fonts remain as fallback in `font.family.*`.

Rejected: Inter (generic look), system font alone (no face of its own, different per
platform), Bricolage Grotesque (strong, but too idiosyncratic for dense lists), fonts close to
Material or Discord.

## Roles

Components use only these roles, never `font.size.*` directly.

| Role | Character | Use |
|---|---|---|
| `type.display` | very large, extra bold, set tighter | empty states, setup, metric tiles |
| `type.title` | large, bold | title of a view, dialog |
| `type.heading` | medium, semibold | channel and thread header, sections |
| `type.body` | base size, regular | message text on desktop |
| `type.body-strong` | base size, semibold | author name |
| `type.body-sm` | one step smaller | list rows, channel names, live box |
| `type.label` | small, semibold | buttons, chips, badges |
| `type.section` | small, bold, set wider | space titles in the sidebar |
| `type.caption` | small, regular | timestamps, help texts |
| `type.code` | mono, one step below base size | code blocks, inline code |
| `type.code-sm` | mono, small, medium | tool chips, costs, token counts |

Sizes, weights, line heights and letter spacing are in the generated
[Token reference](tokens.md); this page gives only names and intent.

Message text on touch devices: `density.message-font-size` (context `touch`) instead of the size
from `type.body`, so that the iPhone does not read smaller than the system default.

## Rules

- **No uppercase by transformation.** Long German words ("Benachrichtigungen")
  become unreadable in capitals. Space titles differ through weight and letter spacing.
- **Numbers in mono where they change.** Run time, cost, tokens, context in the live box use
  `type.code-sm`, so that the line does not jump while counting up.
- **Dynamic type size up to 200 percent** without loss of content (WCAG 2.2 SC 1.4.4,
  https://www.w3.org/TR/WCAG22/, retrieved 2026-10-10 [verified]). Rows grow in height;
  columns collapse into the navigation stack on narrow windows (see 01-product.md).
- **Markdown.** Headings in messages use at most `type.heading`; an agent cannot fill the
  history with display sizes.
- **Fallback.** If a character is missing in Atkinson (for example CJK), the fallback list from
  `font.family.*` applies; the apps bundle no further fonts.

## Integration

- Flutter: `packages/design_system` bundles the variable TTF files of both families (upright
  and italic, axis `wght`) from the source repositories at a recorded commit, with the OFL text of
  each family next to them, under `packages/design_system/fonts/`; `fonts/MANIFEST.json` records
  source URL, commit, SHA-256 and license of every file, and `test/fonts_test.dart` fails on a
  changed, missing or extra file or a file without license text (docs/DECISIONS.md D-M4-04-4).
  The files carry the Google Fonts download names (`<Family>-VariableFont_wght.ttf`), not the
  bracket names of the repositories. `pubspec.yaml` declares the families
  `Atkinson Hyperlegible Next` and `Atkinson Hyperlegible Mono`. `DsType` (generated, copied into
  `lib/src/generated/` by `tokens.py build`) supplies the `TextStyle` constants; `dsTextStyle`
  adds the package font name `packages/design_system/<family>` in front of the fallback list
  (apps that depend on the package register the fonts under that name) and drives the `wght`
  axis from the weight. The theme (`buildDsTheme`) takes its text theme from the roles through
  `dsTextStyle`; the fallback list of `font.family.*` stays behind it.
- Web console and docs site: WOFF2 from the same version; `tokens.css` provides the variables
  `--ds-type-*`.
- Preview: the WOFF2 files are under `tools/design-tokens/vendor/fonts/` and are
  embedded.
