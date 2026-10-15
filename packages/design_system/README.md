# design_system

The one place where the apps and the console get their look: the generated design tokens, the
theme built from them, the resolver from the personal settings to the theme inputs, the bundled
fonts, the semantic icons, the core components and the shared golden test setup.

Status: tokens, theme, fonts and golden setup (M4-04), icons (M4-05), core components (M4-07). Kind: Flutter package, a member of the
Dart workspace at the repository root (`resolution: workspace`, `publish_to: none`).

## What the package provides

| Symbol | Library | Content |
|---|---|---|
| `DsColors`, `DsAccent`, `DsElevation`, `DsDensity`, `DsMotion` | `design_system.dart` | generated `ThemeExtension` classes with one instance per context |
| `DsPalette`, `DsSpace`, `DsRadius`, `DsBorderWidth`, `DsLayout`, `DsSize`, `DsIconSize`, `DsFonts`, `DsType`, `DsIcons` | `design_system.dart` | generated constants |
| `DsContext`, `DsThemeMode`, `DsDensityMode`, `DsMotionMode` | `design_system.dart` | the four theme inputs as a value type |
| `resolveDsContext`, `dsDefaultAccent` | `design_system.dart` | personal settings and system values to a `DsContext` (pure) |
| `buildDsTheme`, `dsTextStyle`, `DsNoTransitionsBuilder` | `design_system.dart` | `ThemeData` of a `DsContext`; a type role ready for rendering |
| `context.dsColors`, `dsAccent`, `dsDensity`, `dsMotion`, `dsElevation` | `design_system.dart` | accessors on `BuildContext`; a `StateError` naming the extension when the theme was not built with `buildDsTheme` |
| `DsThemeScope` | `design_system.dart` | applies a `DsContext` with `AnimatedTheme` (`motion.duration.fade`, zero under reduced motion) |
| `DsIcon`, `DsIconSizeRole`, `DsIconPlaceholder` | `design_system.dart` | a semantic icon of `DsIcons` at `icon.size.*`; see "Icons" |
| `DsIconAssets`, `dsSemanticIcons`, `dsIconPackage` | `design_system.dart` | generated asset lookup of the icons and the list of every semantic icon |
| `dsPumpIcons` | `testing.dart` | `pumpBeforeTest` of a golden with icons: pumps until every icon asset is loaded |
| `dsGoldenMatrix` | `testing.dart` | one alchemist `GoldenTestGroup` per theme, density, text scale and locale; import it from tests only |

Widgets read colors, type, spacing and motion only from these classes (ADR 0030); no literal
size, color, duration or curve in widget code except `0`. Type roles only, never
`font.size.*`: take the text styles from `Theme.of(context).textTheme` or wrap a role with
`dsTextStyle(DsType.<role>)`, so the bundled font also resolves inside an app (see "Fonts").

## Components

The core components of docs/design/components.md (M4-07). Each takes a small immutable view
model of `lib/src/components/view_models.dart` (enums carry the codes of docs/spec/03 and an
`unknown` member; docs/DECISIONS.md D-M4-07-1), reads every text from `package:l10n`, performs no
I/O and reports actions through plain callbacks. Values of configuration keys are parameters;
the apps pass them from the app configuration (M4-08), this package holds no number of them.

| Widget | View model | Configuration parameters | components.md |
|---|---|---|---|
| `DsMessage` | `DsMessageData`, `DsMessageAction` | `collapseLines` (`ui.message_collapse_lines`); `isContinuation` computed by the caller from `ui.message_group_window` | 1 |
| `DsMarkdown` | the Markdown source, `DsAttachmentResolver` | none | 1 |
| `DsLiveBox`, `.collapsed`, `.mobile` | `DsRunData`, `DsStepData` | `visibleSteps` (`ui.livebox_visible_steps`), `announceMinInterval` (`a11y.live_announce_min_interval`) | 2 |
| `LiveAnnouncer` | run state and question ID | `minInterval` (`a11y.live_announce_min_interval`) | 2.4 |
| `DsToolChip` | `DsStepData` | none | 3 |
| `DsStatusLine` | the text | none | 4 |
| `DsTriggerStateIcon` | `DsTriggerState` and reason code | none | 5 |
| `DsThreadRow` | `DsThreadRowData`, `DsTagData` | none | 6 |
| `DsSpaceRow`, `DsChannelRow` | `DsSpaceRowData`, `DsChannelRowData` | none | 7 |
| `DsMessageComponents`, `DsButton` | `DsComponentData`, `DsComponentItem`, `DsComponentHeader` | none | 8 |
| `DsComposer` | `DsComposerCommand`, `DsCommandOption` | `maxLines` (`ui.composer_max_lines`), `enterSends` (`composer.enter_sends`) | 9 |
| `DsSuggestionList` | `DsSuggestion` | none | 9 |
| `DsBillingBadge`, `DsCostText` | `DsBillingMode`, `DsCost` | none | 10 |
| `DsBanner` | `DsTone` and text | none | 2.1, 6.6 of 01 |
| `DsTappable`, `DsBadge`, `dsIsolate` | shared building blocks | none | "Common rules" |

Security rules held here: Markdown renders no HTML, loads no image and activates only `https`,
`http`, `mailto` and internal links (SEC-OUT-1, SEC-OUT-2; D-M4-07-2); foreign names are wrapped
in U+2068 and U+2069 for display, never in semantics labels (SEC-IN-4); origin badges for agent,
app, import and system (SEC-OUT-3); the card header comes only from `DsComponentHeader`
(SEC-OUT-4; D-M4-07-3). Tests: `test/components/` (one file per component plus
`guidelines_test.dart` for contrast, tap targets and labels), goldens in
`test/goldens/components/` built from the sheets in `sheets.dart`.

## Resolver

`resolveDsContext` (docs/spec/01-produkt.md 8 and 11; docs/DECISIONS.md D-M4-04-2):

| Input | Value | Result |
|---|---|---|
| `appearance.theme` | `light` / `dark` | that theme |
| | `system`, absent, unknown | `platformBrightness` |
| `appearance.density` | any, on a touch platform | `touch` |
| | `compact` (desktop) | `compact` |
| | `comfortable`, absent, unknown (desktop) | `comfortable` |
| `appearance.reduce_motion` | `on` | `reduced` |
| | `off` | `full` |
| | `system`, absent, unknown | `reduced` when the platform asks for reduced motion, else `full` |
| accent (`branding.accent_color`) | a key of `DsAccent` | that key |
| | absent, unknown | `spruce` |

## Color scheme mapping

`buildDsTheme` maps the tokens into the Material `ColorScheme` (docs/DECISIONS.md D-M4-04-3);
accent roles come from `DsAccent` of the context's accent and brightness, all others from
`DsColors` of the theme.

| `ColorScheme` | Token |
|---|---|
| `primary` | `color.accent.bg` (`DsAccent.accentBg`) |
| `onPrimary` | `color.text.on-accent` |
| `primaryContainer` | `color.accent.subtle` (`DsAccent.accentSubtle`) |
| `onPrimaryContainer`, `secondary` | `color.accent.fg` (`DsAccent.accentFg`) |
| `onSecondary` | `color.surface.canvas` |
| `secondaryContainer` | `color.surface.selected` |
| `onSecondaryContainer`, `onSurface` | `color.text.primary` |
| `tertiary` / `onTertiary` | `color.info.bg` / `color.info.on-bg` |
| `error` / `onError` | `color.danger.bg` / `color.danger.on-bg` |
| `errorContainer` / `onErrorContainer` | `color.danger.subtle` / `color.danger.fg` |
| `surface`, `surfaceTint` | `color.surface.canvas` |
| `onSurfaceVariant` | `color.text.secondary` |
| `surfaceContainerLowest` | `color.surface.sunken` |
| `surfaceContainerLow` | `color.surface.list` |
| `surfaceContainer`, `surfaceContainerHigh` | `color.surface.raised` |
| `surfaceContainerHighest` | `color.surface.overlay` |
| `outline` / `outlineVariant` | `color.border.default` / `color.border.subtle` |
| `shadow` / `scrim` | `color.shadow` / `color.scrim` |
| `inverseSurface` / `onInverseSurface` | `color.text.primary` / `color.surface.canvas` |
| `inversePrimary` | `color.accent.subtle` |

Further theme fields: `scaffoldBackgroundColor` and `canvasColor` `color.surface.canvas`,
`dividerColor` `color.border.subtle`, `hoverColor` `color.surface.hover`, `focusColor` and the
focus ring of filled, elevated, text and outlined buttons and of focused inputs
`color.focus.ring` (`DsAccent.focusRing`) with `border-width.focus` (focus is always visible,
docs/design/components.md); outlined buttons otherwise `color.border.control` with
`border-width.hairline`. `visualDensity`: `comfortable` gives `VisualDensity.comfortable`,
`compact` `VisualDensity.compact`, `touch` `VisualDensity.standard`. Text theme: display roles
`type.display`; headline and `titleLarge` `type.title`; `titleMedium` `type.heading`;
`titleSmall`, `labelLarge` and `labelMedium` `type.label`; `bodyLarge` and `bodyMedium`
`type.body`; `bodySmall` `type.body-sm`; `labelSmall` `type.caption`; colored with
`color.text.primary`. Under reduced motion every platform uses `DsNoTransitionsBuilder`.

## Fonts

`fonts/` holds the variable TTF files of Atkinson Hyperlegible Next and Mono (upright and italic,
axis `wght`) and the OFL text of each family; `fonts/MANIFEST.json` records source, commit,
SHA-256 and license per file, and `test/fonts_test.dart` checks them (docs/DECISIONS.md
D-M4-04-4, docs/quality/licenses.md "Bundled fonts"). Inside this package the families resolve
under their plain names; an app that depends on the package registers them as
`packages/design_system/<family>`, which `dsTextStyle` puts in front of the fallback list.

To move to a newer upstream state: download the four files from `fonts/variable/` and `OFL.txt`
of both repositories at the new commit, store them under the names of `fonts/`, update
`fonts/MANIFEST.json` (source, commit, `sha256`, `bytes`) and run the tests. Every file stays below
`repo.max_file_kb`.

## Icons

`DsIcon(DsIcons.<name>, label: ...)` draws a semantic icon; the glyph and its weight come from
`design/tokens/icon.tokens.json`, never from a Phosphor name in widget code
(docs/design/icons.md "Rules"). `filled: true` draws the `fill` form (active, reached state);
`size` is `DsIconSizeRole.sm`, `md` (default) or `lg`; the color defaults to
`color.text.primary`. Every icon needs a non-empty `label` (read by the screen reader as an
image) or `decorative: true` (excluded from semantics); otherwise the constructor throws an
`ArgumentError`. `trigger.working`, `run.running` and `state.reconnecting` rotate once per
`motion.duration.pulse` in full motion and stand still in reduced motion.

The assets are generated (docs/design/icons.md "Integration (M4-05)"): `assets/icons/<weight>/<glyph>.vec`
compiled from the vendored Phosphor SVGs by `uv run tools/design-tokens/tokens.py icons` (also
part of `build`), with `assets/icons/LICENSE` (Phosphor, MIT) and `assets/icons/MANIFEST.json`
(source and asset SHA-256, total size). Never edit them; a new icon is a change in
`icon.tokens.json`, then `tokens.py vendor` when its SVG is not vendored yet, then `build`.
`test/icons_complete_test.dart` checks that every semantic and space icon resolves and the
golden `test/goldens/icons_test.dart` shows every semantic icon in its mapped weight and in fill.

## Tokens

`lib/src/generated/design_tokens.g.dart` is generated: never edit it. Change `design/tokens/` and
run `uv run tools/design-tokens/tokens.py build` from the repository root, which writes this copy
and `design/build/dart/design_tokens.g.dart` with the same bytes; `make tokens` fails when they
differ (docs/DECISIONS.md D-M4-04-1).

## Golden tests

`test/flutter_test_config.dart` loads the bundled fonts and configures alchemist: CI goldens
(text as blocks, Ahem) run on every machine; platform goldens with the real fonts run only on
macOS (docs/spec/15-client-architektur.md 20). Golden tests live in `test/goldens/`, their
images in `test/goldens/ci/` and `test/goldens/macos/`. Another member copies the same
`flutter_test_config.dart`, keeps `alchemist` as a dev dependency and builds its matrix with
`dsGoldenMatrix` (docs/DECISIONS.md D-M4-04-5).

Regenerate goldens after an intended visual change with
`flutter test --update-goldens` in the package folder. Platform goldens are updated only on
macOS; on another system the command rewrites only the CI goldens.

## Dependencies

Allowed members (docs/spec/15-client-architektur.md, 3.1; `tools/arch-check/rules.yaml`): `l10n`, `domain` (arrows of the diagram).
Every other member is out of reach; `make arch` fails on a dependency or an import of it.
External: `markdown` (BSD-3-Clause) parses message Markdown into an AST, rendered by the own
widgets of `DsMarkdown` (docs/DECISIONS.md D-M4-07-2); `alchemist` (MIT) as a regular dependency, because `testing.dart` returns its
`GoldenTestGroup` (docs/DECISIONS.md D-M4-04-5); `flutter_test` (SDK) as a regular dependency
for `dsPumpIcons`; `vector_graphics` (BSD-3-Clause) renders the icons; `crypto` as a dev
dependency for the font check; `vector_graphics_compiler` (BSD-3-Clause) as a dev dependency
that `tokens.py icons` runs from the workspace lock.

Further rules checked by `make arch`:

- no native source (rule 4).

## Layout

| Path | Content |
|---|---|
| `lib/design_system.dart` | the public library; every public symbol has a doc comment (`make lint`, `public_member_api_docs`) |
| `lib/testing.dart` | golden helpers for tests of every member |
| `lib/src/generated/` | the generated token copy and icon lookup (no doc lint, no format check, no coverage) |
| `lib/src/icons/` | `DsIcon` |
| `lib/src/components/` | the core components, their view models and the Markdown renderer |
| `assets/icons/` | generated icon assets, Phosphor MIT text, `MANIFEST.json` |
| `lib/src/theme/` | context, resolver, theme builder, scope widget |
| `fonts/` | bundled font files, OFL texts, `MANIFEST.json` |
| `test/` | tests, run by `make test` with coverage (`quality.coverage.dart_min_percent`) |

Run from the repository root: `make lint test coverage arch COMPONENTS=apps`. The API pages
come from `make docs` ([Dart API reference](../../docs/reference/dart-api.md)).
