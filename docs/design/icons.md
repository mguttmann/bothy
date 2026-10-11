# Icons

Source: `design/tokens/icon.tokens.json`. Sizes are tokens (`icon.size.sm`, `.md`, `.lg`); the
mapping of semantic names to glyphs is stored as the extension `ds.icons` on the group node `icon`,
because DTCG 2025.10 knows no icon type and forbids custom types
(https://www.designtokens.org/tr/2025.10/format/, retrieved 2026-10-10 [verified]).

## Choice: Phosphor

| Criterion | Phosphor | Lucide | Material Symbols |
|---|---|---|---|
| License | MIT [verified] | ISC [verified, npm `lucide-static`] | Apache-2.0 [verified, npm `material-symbols`] |
| Stroke weights | thin, light, regular, bold, fill, duotone [verified] | one stroke form, width adjustable | variable axes (fill, weight) |
| Filled variant for "active" | yes (`fill`) | no | yes |
| Effect | friendly, geometric, distinctive | neutral, common in the generic look | instantly recognizable as Google/Android |
| Flutter | package `phosphor_flutter` 2.1.0, last published two years ago [verified] | community packages | package `material_symbols_icons` |

Evidence: https://github.com/phosphor-icons/core (MIT), package `@phosphor-icons/core@2.1.1` (license
MIT, folder `assets/<weight>`), https://pub.dev/packages/phosphor_flutter (version 2.1.0,
publisher phosphoricons.com, styles Thin, Light, Regular, Bold, Fill, Duotone); all retrieved
2026-10-10.

Reasons: The filled variant shows "active" and "state reached" without a second icon set; the
weight `bold` stays readable at small sizes; the shapes match the open shapes of
Atkinson Hyperlegible. Material Symbols would make the app look like Android; Lucide has no
filled variant.

**Integration without an outdated package.** Because `phosphor_flutter` has been dormant for two
years, `packages/design_system` integrates Phosphor itself: the package build takes the SVGs of the
used glyphs from `@phosphor-icons/core` in a pinned version and generates an icon font with only
these glyphs from them (or vector assets with `vector_graphics_compiler`, flutter.dev); the ticket
"Icons in the design system package" makes the choice after a comparison of size and sharpness. The
generated class `DsIcons` provides the semantic names (`DsIcons.triggerWorking`). The preview uses
the same SVGs from `tools/design-tokens/vendor/phosphor/` (with the MIT license text).

## Rules

- **Icons only through semantic names.** A component asks for `trigger.working`, not
  `circle-notch`. Swapping the glyph is a change in one place.
- **No emoji as an icon.** Emoji exist only as free reactions of people. Status is never an
  emoji and never a reaction.
- **Never an icon alone.** Every icon with meaning has a caption or a screen reader label
  (see components.md). Color is never the only feature; states differ in
  shape.
- **Weights:** `regular` as the default, `fill` for active and for reached final states, `bold`
  for small control glyphs (arrows, plus, close).
- **Spinning** only for `trigger.working`, `run.running` and `state.reconnecting`, and only in the
  `full` motion context.

## Mapping (excerpt)

Full list: `design/tokens/icon.tokens.json`, resolved in
`design/build/tokens.resolved.json`.

| Semantic name | Glyph | Weight | Meaning |
|---|---|---|---|
| `trigger.received` | `check` | regular | Server has accepted the triggering message |
| `trigger.queued` | `hourglass-medium` | regular | Run waits in the queue |
| `trigger.working` | `circle-notch` | bold, spins | Agent is working |
| `trigger.done` | `check-circle` | fill | Run finished successfully |
| `trigger.failed` | `x-circle` | fill | Run or trigger failed |
| `trigger.followup` | `arrow-bend-down-right` | regular | Follow-up delivered to the running run |
| `run.waiting` | `bell-ringing` | fill | Run waits for an answer (waiting_input) |
| `run.failed` | `warning-octagon` | fill | Run failed |
| `run.cancelled` | `prohibit` | regular | Run canceled |
| `step.tool_call`, `step.subagent`, `step.workflow`, `step.background_job` | `wrench`, `robot`, `flow-arrow`, `clock-countdown` | regular | Step kinds (`run_steps.kind`) |
| `channel.text`, `channel.forum` | `hash`, `chats-circle` | bold, regular | Channel kinds |
| `billing.personal`, `billing.organization` | `user-circle`, `buildings` | fill | Backend policy badge |
| `security.step-up` | `shield-check` | fill | Fresh confirmation needed, approval cards |
| `action.stop` | `stop-circle` | fill | Stop run |

The trigger states replace the earlier status markers from Discord one to one: received,
queue, in progress, finished, error, follow-up. Their shapes differ so that they stay
distinguishable even without color and at small sizes.

## Space icons

`spaces.icon` stores a glyph name from the list `space-icons` in `icon.tokens.json`
(for example `headset`, `code`, `megaphone`, `flask`). The server checks against the same list
(from `design/build/tokens.resolved.json`, ticket "Allowed color and icon keys in the server");
an emoji or a free name is rejected.
