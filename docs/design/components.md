# Components

Status: draft M0. Specification of the core components for `packages/design_system`. Sizes and
colors only as token names; numbers in [tokens.md](tokens.md) and [palette.md](palette.md). Every
text or icon color on a surface is listed as a pairing in `design/design-rules.json` and checked in
[contrast.md](contrast.md) for light and dark. Images:
[Components dark](previews/components-dark.png), [light](previews/components-light.png);
Terminal [dark](previews/terminal-dark.png), [light](previews/terminal-light.png); Statistics
[dark](previews/viz-dark.png), [light](previews/viz-light.png).
Data fields according to [03-data-model.md](../spec/03-data-model.md), events according to
[04-sync-protocol.md](../spec/04-sync-protocol.md).

Common rules for all components:

- **States:** rest, hover (pointer only), pressed, focus (ring `color.focus.ring`,
  `border-width.focus`, offset outside), disabled, loading, error. Focus is always visible: a
  control that draws no ring of its own (Material icon buttons, switches, check boxes, list
  tiles, drop-down and segmented buttons, filled buttons, whose fill has the ring's color) gets
  the app-wide ring of `DsFocusRing`, drawn with a gap of `border-width.focus` around the control
  while the person works with the keyboard (after a key press, until the next pointer press;
  brief M4-32).
- **Menus:** every entry of a menu is a `DsMenuItem`: a button for screen readers, and the chosen
  entry of a choice says "selected"; a check mark alone is not read. A state line inside a menu
  is text in `color.text.secondary`, not a disabled entry (brief M4-32). A text field with a
  list to choose from is a `DsDropdownField`, named by its label; no control sits below a text
  field for screen readers (brief M4-32).
- **Hit area** at least `density.target-min`, even when the visible area is smaller.
- **Semantics:** Every component sets role and label for screen readers (Flutter `Semantics`,
  on the web as ARIA). Labels come from `packages/l10n`, never from the agent's data alone. A
  name is read once: a component that gives its own label excludes the texts it is built from,
  and a control in a settings row is named by the row. The audit `dsSemanticsProblems` of
  `package:design_system/testing.dart` checks these rules on every screen (brief M4-32).
- **Never color alone:** States differ in icon shape and text.
- **Motion** only through `motion.*`; no motion in the `reduced` context.
- **Accent roles from `DsAccent`:** components read `color.accent.*`, `color.text.link`,
  `color.focus.ring` and `color.run.running` only from the theme extension `DsAccent` (Dart) or the
  `--ds-*` variables under `data-accent` (web), never from `DsColors`, so the instance accent
  (`branding.accent_color`) applies everywhere. Every variant is in [contrast.md](contrast.md).

## 1. Message

Purpose: a post in a thread (`messages`), from a person, agent, app, import or system.

Anatomy: avatar (person round, agent rounded square `radius.md`, app with a small badge),
header (name, for agents the badge "AGENT", timestamp, for triggering messages the
trigger state), content (Markdown), attachments, components (for `kind = card`), reactions,
reply reference (quote line above the content), hover bar (Reply, React, Edit,
Pin, More).

| Variant | Data | Rendering |
|---|---|---|
| Text | `kind = text` | Markdown in `type.body` or `density.message-font-size` |
| Continuation message | same author within `ui.message_group_window` | without avatar and header; timestamp on hover |
| Voice message | `kind = voice` | play button, duration, progress; transcript below as soon as `transcript_status = done` |
| Live box anchor | `kind = run` | renders the live box (section 2) at this position |
| Card | `kind = card` | content plus components (section 8) in `color.surface.raised` |
| System | `kind = system` | one line in `type.caption`, `color.text.secondary`, icon on the left |
| Mentions me | me in `mentions` | tinted background `color.attention.subtle`, spine `color.attention.border` with `border-width.spine` |
| sending | entry in the outbox | reduced opacity, line "Sending" with the `trigger.queued` icon |
| failed | outbox, permanent error | line in `color.danger.fg` with the reason, buttons "Resend", "Edit", "Discard" |
| edited | `edited_at` set | suffix "(edited)" in `color.text.muted`; versions via `message.revisions.read` |
| deleted | `deleted_at` set | line "Message deleted" in `color.text.muted`; no hover bar |

Long replies: content over `ui.message_collapse_lines` lines is collapsed; a text button
"Show more" (`color.text.link`) expands it. Code blocks in `type.code` on `color.code.bg` wrap
their long lines by default, an indented line with a hanging indent (01 6.14, M4-49). The text
toggle "Wrap lines" in `type.label` in the block header (left of the copy button on the desktop,
highlighted with `color.surface.selected` and `color.text.link` while on) switches every block of
the device to one line per line with a horizontal scroll and a scrollbar that is always visible on
the desktop. A block higher than `ui.code_block_collapse_lines` lines shows that height with a
fade into `color.code.bg` and a text button "Show all" (`color.text.link`) below it; expanded it
reads "Show less". Every code block and quote block copies on its own (01 6.12, M4-48): on
the desktop a text button "Copy code" or "Copy quote" in `type.label`, `color.text.link`, at the
top right of the block; on the phone (density `touch`) the whole block is the tap target, with a
snack bar "Copied" and a light haptic, and a long press on the message opens the message menu.

Sizes: avatar `size.avatar-md`; gap from avatar to content `space.12`; group gap
`density.message-gap`; row padding `density.message-pad-y`; reading width at most
`layout.message-max-width`.

Accessibility: A message is a semantics node with the label "<Name>, <Time>, <Text>"; the
trigger state is a separate node with a label (section 5). The hover bar is reachable by keyboard
through the message (focus on the message, then Tab). A mention is introduced in the label with
"Mentions you".

## 2. Live box

Purpose: display of a run (`runs`) with state, metrics, status line, steps and
last text. It is pinned at the top of the thread while the run is active, and at the end it
collapses into one line.

Data: run document `run_document.v1` (04, section 11): `state`, `version`, `status_line`,
`last_text_excerpt`, `metrics` (`context_pct`, tokens, subagents, background jobs,
API retries), `cost`, `billing_mode_used`, `steps`, `followups`. State changes arrive as the
durable `run.state_changed`, content as the ephemeral `run.updated` (only with focus).

### 2.1 In progress (`queued`, `running`, `waiting_input`)

Anatomy, from the top:

1. **Header:** state icon and state word (`run.*`), run time, context in percent,
   tokens, cost (all in `type.code-sm`), backend policy badge (section 10),
   expand arrow.
2. **Status line** (section 4).
3. **Steps:** tool chips (section 3) in `ordinal` order, substeps indented
   (`parent_step_id`). The last `ui.livebox_visible_steps` are visible; older ones are behind
   "n earlier steps".
4. **Last text:** `last_text_excerpt` as a quote with a line on the left in `color.border.default`.
5. **Footer:** button "Stop" (variant danger-outline, icon `action.stop`), note about follow-ups.
   For `waiting_input`, the form or the buttons of the question (section 8) instead of the note.
6. **Stall:** After `run.stall_after` without progress, a banner appears in
   `color.warning.*` ("No progress for n minutes").

Surface `color.surface.raised`, border `color.border.default`, radius `radius.md`, padding
`density.livebox-pad`. Spine on the left in `border-width.spine` with the color of the state:

| State | Spine and icon | Icon | Word |
|---|---|---|---|
| `queued` | `color.run.queued` | `run.queued` | "Queued" |
| `running` | `color.run.running` | `run.running` (spins) | "Running" |
| `waiting_input` | `color.run.waiting` | `run.waiting` | "Waiting for you" or "Waiting for Alex" |
| `completed` | `color.run.completed` | `run.completed` | "Done" |
| `failed` | `color.run.failed` | `run.failed` | "Failed" plus reason |
| `cancelled` | `color.run.cancelled` | `run.cancelled` | "Canceled" |

Reasons (`state_reason`) are translated, never shown raw: `billing_mode_mismatch` ->
"Backend does not match", `access_revoked` -> "Access revoked", `stale` -> "Canceled after
stall", `user_cancelled` -> "Canceled by <Name>", `adapter_error` -> "Error in the adapter",
`cancel_timeout` -> "Adapter did not confirm the cancellation", `project_not_ready` -> "Project
not ready". Unknown reasons: "Failed" plus the code in `type.code-sm`.

### 2.2 Collapsed (final states)

One line on `color.surface.sunken`, radius `radius.md`: icon, "Ran <Duration>", number of
tools, subagents, cost with "Estimate" as long as `cost_is_estimate`. The summary
comes from `run.state_changed`; expanding loads the document via REST (`run.read`). Study B
proposed the format "Ran 28 min · 3 agents · USD 1.20"; it is adopted.

### 2.3 Mobile

Compact line below the navigation bar (height `density.control-height` plus padding):
state icon, state and run time, status line truncated to one line, stop button on the right with
hit area `density.target-min`. Tapping opens the full live box as a sheet. As built (M5-02): state
icon, state and run time wrap and the status line sits below them, so text scaling up to 200
percent loses nothing; the stop button stays at the top right of the line.

### 2.4 Accessibility

- The live box is a semantics container with the label "Run of <Agent>: <State>, for <Duration>,
  context <n> percent".
- Live region: only state changes and new questions are announced (polite), at most once
  per `a11y.live_announce_min_interval`. Patches to steps and numbers are never announced; they
  would flood screen readers.
- The stop button has the label "Stop run of <Agent>" and is reachable by a shortcut
  (01-product.md, Keyboard shortcuts).

## 3. Tool chip

Purpose: one step of the live box (`run_steps`).

Anatomy: icon by `kind` (`step.*`), name (`name`, for example the tool name) in `type.code-sm`,
short description (`title`) in `color.text.secondary`, duration or "running n s". Surface
`color.surface.sunken`, radius `radius.sm`, height `density.row-channel` minus padding.

| `status` | Icon color | Suffix |
|---|---|---|
| `pending` | `color.run.queued` | "waiting" |
| `running` | `color.run.running`, icon spins | run time |
| `completed` | `color.run.completed` | duration |
| `failed` | `color.run.failed` | "failed", detail on expand |
| `cancelled` | `color.run.cancelled` | "canceled" |

Click or Enter opens `detail` (`run_step_detail.v1`) in a disclosure; contents are truncated
to `run.step_detail_max_bytes`. Label: "<Kind> <Name>, <Status>, <Duration>".

## 4. Status line

One sentence on what the agent is doing right now (`runs.status_line`), in `type.body-sm`,
`color.text.primary`, one line, truncated at the end with a tooltip. Inline code allowed. Empty:
the line is omitted. Not part of the live region (see 2.4).

## 5. Trigger state

Purpose: state on the triggering message of a person (`messages.trigger_state`); replaces
the status emoji from Discord.

Placement: in the header of the message after the timestamp, size `icon.size.sm`. Tapping or
hovering shows the caption and jumps to the live box.

<!-- language-check: off -->

| `trigger_state` | Icon | Color | Label (DE) | Label (EN) |
|---|---|---|---|---|
| `received` | `trigger.received` | `color.trigger.received` | Empfangen | Received |
| `queued` | `trigger.queued` | `color.trigger.queued` | In Warteschlange | Queued |
| `working` | `trigger.working` (spins) | `color.trigger.working` | Wird bearbeitet | Working |
| `done` | `trigger.done` | `color.trigger.done` | Erledigt | Done |
| `failed` | `trigger.failed` | `color.trigger.failed` | Fehlgeschlagen | Failed |
| `cancelled` | `run.cancelled` | `color.run.cancelled` | Abgebrochen | Canceled |
| `followup_delivered` | `trigger.followup` | `color.trigger.followup` | Nachtrag zugestellt | Follow-up delivered |

<!-- language-check: on -->

For `failed`, the reason is in the tooltip and in the semantics label ("Failed: Project not
ready"). Messages without a trigger (`trigger_state` empty) show nothing. `cancelled` is a deliberate stop
(09 section 6): it uses the neutral run color, never the error style of `failed`, and names the
reason the same way ("Canceled: Canceled by Sam").

## 6. Thread list with status (forum)

Purpose: dense list of the posts of a forum, sorted by last activity
(`threads.last_activity_at`) or by creation (`forum_default_sort`). On desktop it is the forum
overview D15 in the content column, pinnable as its own column (01 R-PR-17); on the phone the page
M2. The open posts also appear as thread rows in the sidebar (section 7).

Row (height `density.row-post`, two lines):

- on the left a state icon: active run (`run.*` from "Running now"), otherwise mention
  (`color.attention.fg`), otherwise nothing;
- title in `type.body-sm`, bold when unread; time on the right in `type.caption`;
- second line: badge "Waiting for you" (`color.attention.bg`), operator agent,
  mention counter, tags (`category.<name>.chip-*`), on error the reason in `color.danger.fg`.

States of the row: rest, hover (`color.surface.hover`), selected (`color.surface.selected`),
focus. Header: forum name with the kind sentence (01 6.8 item 11), button "New chat" in a forum with
a project binding or "New post" otherwise (primary), search in the forum, "Pin" (desktop,
`action.toggle-panel`), filter chips (Open, Waiting for me, Tags, Sort). At the bottom the done area: row "Done (n)", collapsed by default;
when expanded, done posts follow in `color.text.muted`.

Accessibility: list with the role "list", rows with the label "<Title>, <State>, <Time>,
<n> unread". Arrow keys move, Enter opens.

## 7. Space and channel tree

Purpose: sidebar with spaces (`spaces`) and channels (`channels`), collapsible and sortable per
person (`user_space_states`).

Space row: arrow (`action.expand` or `action.collapse`), icon of the space from
`space-icons` in the category color (`category.<name>.on-sidebar`, on a lightly tinted
tile of the same color), name in `type.section`, `color.text.on-sidebar`. Collapsed with unread
items: dot on the right. For an owner on the desktop the row has the action "+" (`action.new`,
label "New project in <space>", 01 6.9 item 1), shown on hover and on focus, always in the context
`touch`, like the "+" of a forum row; the space menu starts with the same entry, followed by "Add forums for
existing folders" (01 6.10, M4-44). The menu of a forum with a project binding has "Rename
project" for a person with `manage` (01 6.11).

Channel row (height `density.row-channel`): icon by kind (`channel.text`, `channel.forum`,
`channel.project` for a forum with a project binding), name in `type.body-sm`. The icon has a
tooltip with the kind sentence (01 6.8 item 11). A forum row has the action "+" (`action.new`,
label "New chat in <forum>" or "New post in <forum>"), shown on hover, on focus and on the selected
forum, always in the context `touch`; a channel with thread rows has its own arrow
(`action.expand`, `action.collapse`).

Thread row (addendum of 2026-10-27, 01 6.8): height `density.row-channel`, indented one step
further than the channel row, a tree line on the left in `color.text.on-sidebar-muted` with
`border-width.hairline`; in front of the title the run state icon (`run.running`, `run.waiting`,
`run.failed`) at `icon.size.sm`, otherwise nothing; title in `type.body-sm`, one line, cut with an
ellipsis, full title in the tooltip; the states of the table below. The draft row "New chat" is in
italics in `color.text.on-sidebar-secondary`. The last row of an expanded forum group is "All
posts" in `color.text.on-sidebar-secondary` with the counters of the posts not shown. A forum with
more open posts than `ui.sidebar_forum_posts_visible` has the row "Show more" ("Show less" while
the group lists them all) above "All posts": same indent and tree line, text in
`color.text.on-sidebar-secondary`, no counters, a button whose label names the posts it shows or
hides (addendum of 2026-10-29, M4-46).

| State | Rendering |
|---|---|
| read | `color.text.on-sidebar-secondary` |
| unread | `color.text.on-sidebar`, bold, bar on the left |
| mentions | counter (`color.attention.bg`, `radius.pill`) |
| active run | run dot `category.spruce.on-sidebar` with pulse `motion.duration.pulse` |
| muted | `color.text.on-sidebar-muted`, icon `action.mute` |
| selected | `color.surface.sidebar-selected` |
| hover | `color.surface.sidebar-hover` |
| focus | ring `color.focus.ring-on-sidebar` |

Top: mark and product name (`branding.product_name`), quick switcher field ("Jump to" with a
shortcut), entry "Running now" with a counter. Bottom: own avatar, settings.

Behavior: collapse by click, Enter or arrow left/right; the collapse state of a space and of a
channel's thread group is kept on the device (01 6.7 R-PR-24, 01 6.8 item 7).
Dragging a space changes the personal order (`user_space_states.sort_key`); the global order is
changed only by `space.reorder` in the administration. Channel rows and thread rows cannot be
dragged (01 6.8 item 10).

Accessibility: tree with the role "tree"; spaces are expandable nodes with the state
"collapsed" or "expanded"; channel label "<Name>, <kind>, <n> unread, <m> mentions, run
active"; a channel with thread rows is an expandable node of level 2, a thread row a node of level
3 with the label "<Title>, <State>, <n> unread, <m> mentions"; the action "+" is a button with its
label. The row of a space carries its move actions ("Move up", "Move down") itself; the group
of a space and its channels, which the list gives the actions to move it, is named "Space <Name>
with its channels" (M5-10).

## 8. Components on messages (buttons, select, form)

Purpose: interactive elements of an agent or an app (`message_components`), for example
command approval ("Allow once", "For this session", "Always", "Deny"), model choice,
questions.

| Type | Rendering |
|---|---|
| `button` | button; style from `definition.style`: primary (`color.accent.bg`), success (`color.success.bg`), danger (`color.danger.bg`), secondary (border `color.border.control`), quiet |
| `select` | select field with a menu (`elevation.menu`), single or multiple selection |
| `form` | fields with the label above the field, submit button; validation against the schema of the component |

States (`message_components.state`):

| State | Rendering |
|---|---|
| `active`, permitted | operable |
| `active`, not permitted | visible, disabled, note with `security.lock`: "Only <Triggering person> or an owner can approve this" or "You do not have the permission 'Approve actions' for this" |
| sent, answer pending | the pressed button shows a loading icon; other buttons locked |
| `used` | all buttons gone; line "<Name> chose '<Choice>', <Time>" with `trigger.done` |
| `expired` | line "Expired" in `color.text.muted` |
| `disabled` | line "No longer available" |
| error | banner in `color.danger.*` with the reason; buttons operable again if the component is still `active` |

Rules: One use is one interaction with a `client_interaction_id` from the outbox; a
double click creates no second one. A destructive choice ("Always") requires no second
confirmation, because it follows the permissions; the card does show, however, what "Always"
means. Buttons in a row, on mobile devices stacked at full width (image "Mobile"). On mobile
devices a form opens as a sheet over the keyboard (the card shows a button "Fill in") and a
select opens the system picker in a sheet (iOS: the wheel picker with "Submit"; Android: a list
whose tap on an option sends it); only a press on a button sends, never a swipe, drag or fling
(M5-02, `DsMessageComponents` with `stacked` and `sheets`).

Accessibility: group with a label from the card title; buttons in reading order; no shortcut
triggers an approval globally (accidental operation); Enter or Space only on the focused
button.

Header and texts (M4-07, SEC-OUT-4): above the agent text a framed header with
`security.step-up`, built only from server fields: "Request from <Agent>", "Run <Run>",
"Requires the permission '<Capability>'" and the approval note ("Only <Name> or an owner can
approve this", "Only an owner can approve this", "Anyone with access to this channel can
answer"). Below it a divider labeled "Text from <Agent>", then the agent text. While an answer is
pending the button reads "Sending your choice" to screen readers. Form fields show "Required",
"At most <n> characters" or "Enter a number"; a select without a choice shows "Choose"; a
component type the app does not know shows "This element needs a newer app".

## 9. Input with slash suggestions

Purpose: compose a message, commands (`commands`) with options and autocompletion,
mentions, attachments, voice recording.

Anatomy: field (surface `color.surface.input`, border `color.border.control`, radius `radius.lg`,
minimum height `density.composer-min-height`, grows up to `ui.composer_max_lines`), attachment on
the left, microphone and send on the right. Above it the "typing" line ("Sam is typing"), during an
active run the placeholder "Follow-up to <Agent>".

Suggestion list (overlay `color.surface.overlay`, `elevation.menu`): opens on "/" at the start of
a line and on "@". Rows (`density.row-suggestion`): command in mono, description,
origin (agent) and the hint "Control command" for `control = true`. Order: commands of the
operator of this channel first; equal names of other agents are qualified with the handle
(03-data-model.md, `commands`). After a command is selected, the input shows the options as
placeholders; options with `autocomplete` fetch suggestions through `command.autocomplete`,
throttled by `ui.autocomplete_debounce`. Mention suggestions show only people and agents that
the person may see.

States: empty, text, command selected, options incomplete (send disabled with a reason),
offline (sending goes to the outbox), no write permission (field replaced by the note "Read
only in this channel").

Long texts (M4-47, 01 6.13): a paste over `ui.composer_paste_file_chars` characters or
`ui.composer_paste_file_lines` lines becomes a file chip (`paste.txt`, or `paste.<extension>`
for one fenced block) above the field with the ghost button "Insert as text"; a text over
`message.max_content_bytes` shows the note "Longer than one message: the rest goes out as
message.txt with this message" with `state.info` above the field.

Keyboard: arrows select, Tab or Enter accepts, Escape closes; Enter sends, Shift+Enter
makes a new line (reversible through `composer.enter_sends`); arrow up in the empty field edits
your own last message.

Accessibility: field with the label "Message to <channel or thread>"; suggestion list as a listbox
with an active entry; the number of suggestions is announced.

Texts (M4-07): two or more people typing read "Several people are typing"; offline shows
"Offline: your message waits in the queue" with `state.offline`; incomplete options show
"Missing option: <Option>" with `state.warning` above the field; the buttons read "Send",
"Attach file" and "Record voice message".

## 10. Backend policy badge

Purpose: shows in the thread header, live box and agent card what the agent runs on
(`agents.billing_mode`, baseline section 5).

| Mode | Icon | Color | Text |
|---|---|---|---|
| `personal` | `billing.personal` | `color.badge.personal-bg`, `-fg` | "Personal" |
| `organization` | `billing.organization` | `color.badge.organization-bg`, `-fg` | "Organization" |

Height `size.badge-height`, radius `radius.pill`, text `type.label`. If the reported mode
(`reported_billing_mode`) differs from the configured mode, the agent card also shows a warning
badge (`color.warning.*`, "Reported: Organization"); runs then fail with `billing_mode_mismatch`.
Costs for `organization` carry the suffix "Estimate" until reconciled; for `personal` the suffix "for
information only". Label: "Backend: Personal, runs on the subscription of <Name>".

## 11. Visualizations (`bothy_viz`)

Purpose: basic forms for the views from [25-visualization.md](../spec/25-visualization.md) in
apps and console; the Grafana panel uses the same roles through the TypeScript output of the tokens.

| Basic form | Appearance | Tokens |
|---|---|---|
| Lane (space) | row with a header on the left (category color and icon of the space), line `color.viz.grid` between lanes | `color.border.*`, `space.*` |
| Run card | like a collapsed live box: spine on the left in the state color of the run, agent, tool chip (section 3), backend policy badge (section 10), cost in mono | roles of the live box, `type.code` |
| Metric tile | large number in mono, label above it, comparison with the previous period as text with an icon (never only color) | `type.display`, `type.label` |
| Area and bar | series in `color.viz.series.1` to `.8` in a fixed order; axes `color.viz.axis`; fill `color.viz.area-fill`; guide lines `color.viz.grid` | `color.viz.*` |
| SLO traffic light | dot with icon and text "on target", "budget tight", "target missed", "not enough data" and the label "own measurement" | `color.success.*`, `color.warning.*`, `color.danger.*`, neutral role for "not enough data" |
| Timeline | playhead as a vertical line, controls with icons from `icon.tokens.json` | `motion.*` |

Rules:

- No gradient, no glow, no 3D look (ban list in [README.md](README.md)).
- Every basic form has semantics and a table view with the same values.
- The roles `color.viz.*` are in the tokens; series and axis are checked against `surface.canvas`,
  `surface.list` and `surface.raised` with contrast `ui` in both themes
  ([contrast.md](contrast.md)). `grid` and `area-fill` carry no information alone and therefore
  have no contrast requirement. The order of the series starts with blue and orange, which stay
  distinguishable with red-green color blindness; a second feature (label or pattern) is
  still required. `make tokens` also checks every adjacent pair of series (1 with 2 up to 7
  with 8, and 8 with 1 because the order wraps when a view needs more than eight groups) under
  simulated protanopia, deuteranopia and tritanopia: the OKLab distance of the simulated colours
  must reach `cvd_min_delta_e_ok` of `design/design-rules.json` (section "Color vision
  deficiency" of [contrast.md](contrast.md)).
- The SLO traffic light uses only `color.success.*`, `color.warning.*` and `color.danger.*`; the
  same check holds `success.fg`, `warning.fg` and `danger.fg` pairwise apart by
  `cvd_min_delta_e_ok`, and every state carries an icon and text as well.
- TV mode: dark theme, density `comfortable`, font sizes one step larger; no motion in the
  `reduced` context.

## 12. Terminal view

Purpose: shows a terminal session on a connected host
([24-terminal.md](../spec/24-terminal.md)); screens D11 (desktop and console) and M8 (phone) in
[01-product.md](../spec/01-product.md). Package `packages/terminal_ui`, renderer `xterm` (PLAN M4-35).

| Part | Appearance | Tokens |
|---|---|---|
| Header | icon `terminal-window`, host name (`type.heading`), connection as a dot with text, LLM login badge (section 10), start of the fingerprint in mono, remaining time as text | `color.text.*`, `color.badge.*`, `font.family.mono` |
| Tabs | one tab per own session on the host; selected like a selected row; on the right "Detach" (secondary) and "End" (danger, outlined) | `color.surface.selected`, buttons |
| Screen | surface `color.terminal.bg`, text `color.terminal.fg` in `font.family.mono` with the size from `type.code`, adjustable separately (01, section 8); cursor as a block `color.terminal.cursor` with the character in `color.terminal.cursor-text`; selection `color.terminal.selection-bg` | `color.terminal.*` |
| ANSI colors | SGR 30 to 37 and 90 to 97 map to `color.terminal.ansi.<name>` and `color.terminal.ansi.bright-<name>`; the renderer shows 256 colors and truecolor unchanged, because the program chooses them | `color.terminal.ansi.*` |
| Footer | "End-to-end encrypted" with a lock icon, profile, "Idle timeout active" | `type.caption`, `color.text.secondary` |
| Extra keys bar (mobile) | keys Esc, Tab, Ctrl, Alt, arrows, vertical bar, `~`, `/`, `-` with height `density.control-height`; a latched modifier is filled in `color.accent.bg` with `color.text.on-accent` | button roles, `density.*` |
| Confirmation on opening | dialog with host, profile and badge; main button "Confirm with passkey" or device key; note "Every opening is logged" | `elevation.dialog`, `color.surface.overlay` |
| Key change warning | dialog with border `color.danger.border`, old and new fingerprint in mono; the danger button "Do not connect" is the main action | `color.danger.*` |
| States | connected, detached (banner `info` with "Reattach"), ended (reason as text, screen stays readable), blocked by revocation (banner `danger`) | banner roles |

Rules:

- The roles `color.terminal.*` apply only in the screen of the terminal. Header, tabs, dialogs and
  bars follow the theme of the app.
- Contrast: default text and the twelve chromatic colors as well as bright black (ANSI 8, often for
  dimmed text) reach `text` against `color.terminal.bg`; the grays at the ends (ANSI 0, 7, 15) and
  the cursor reach `ui`. Reason: programs use these three grays as background or
  border; full text contrast is not possible in both themes for all three at once. All
  pairings are in `design/design-rules.json` and in [contrast.md](contrast.md).
- No glow, no scanline effect, no imitation of old screens.
- Motion: the cursor blinks only in the `full` context of `motion.*`; in the `reduced` context it
  stays still.
- Semantics: the line at the cursor and, on command, the visible screen are readable; running output
  is not read aloud (01, section 8). Extra keys carry labels ("Control, latched").
- Golden tests in light and dark with warning, detached and extra keys bar (M4-35).

## 13. Review list (not automatically checkable)

- No glass, blur or glow effects; no gradients.
- No floating cards with a large shadow; shadows only from `elevation.*`.
- No welcome banner, no terminal look in chat and administration (the terminal view in
  section 12 is a separate component), no decorative illustrations.
- No emoji in UI texts or as an icon; status never as a reaction.
- Every new color is a role with a pairing in `design/design-rules.json`.
- Golden tests in light and dark, in `comfortable` and `touch`, with text scaling 1 and 2
  (15-client-architecture.md, Test strategy).

## 14. Configuration values

The values `ui.*` and `a11y.*` that this page names are listed with their defaults in
[01-product.md](../spec/01-product.md), section "Configuration values of this chapter".
