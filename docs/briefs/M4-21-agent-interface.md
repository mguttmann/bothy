# M4-21: Live box, trigger state, follow-ups, components and commands

Status: ready | Depends on: M4-20, M2-31 | Milestone: M4

## Goal

After this ticket the agent interface of 01 "Agents" works on the desktop: a run in the open thread shows the live box of components.md 2 pinned at the top of the history while it is active (state, duration counted locally, context, costs with their suffix, backend badge, current tool, steps with subagents, plan, last text, follow-ups, API retry, stall notice, Stop without confirmation) fed by the run store of M4-16 at most once per frame, and collapses into one line at its anchor when the run ends (built from `run.state_changed`, expandable through `getRun`); the triggering message shows its trigger state as an icon with a label (01 6.3, components.md 5); during an active run the composer is labeled "Follow-up to <agent>" and a sent message becomes a follow-up whose state goes to "Follow-up delivered"; components on messages (approval buttons, select, form; components.md 8) send one interaction through the outbox and show the header built only from server fields, separated from the agent's text (SEC-OUT-4, SEC-T14); the slash autocomplete of 01 D5 suggests the commands of the channel (operator first), fetches option suggestions from the agent throttled by `ui.autocomplete_debounce`, and invokes the command; the agent's typing indicator follows the run state (D-L9); and the session column shows the agent session part (agent card with presence and backend badge, model, session, project, context, thread cost, runs, earlier sessions). Why: the live box, the trigger state and the approval cards are what makes Bothy a command center for agents (R-PR-13); an approval card whose header an agent can fake would let the agent obtain approvals under a false name (T-AI-05).

## Read first

- `docs/PLAN.md`: section 3; section 10 (Red team: T-AI-05, T-AI-06; rows M4-16, M4-20, M4-21, M4-24, M4-32, M4-34).
- `docs/spec/01-product.md`: 3.2 "Agents", 5.1 D1 (session column), D3 (live box in four states), D4 (components), D5 (slash autocomplete), 6.2 "Live box", 6.3 "Trigger state as icons", 6.5 "Components", 6.7 (costs), 7 (Stop with Cmd+. and the live box toggle: bindings in M4-24, actions here), 8 (live content announcements), 13 (R-PR-04, R-PR-13), 14 (`ui.livebox_visible_steps`, `ui.autocomplete_debounce`, `a11y.live_announce_min_interval`).
- `docs/design/components.md`: 2 "Live box" (2.1, 2.2, 2.4), 3 "Tool chip", 4 "Status line", 5 "Trigger state", 8 "Components on messages", 9 (commands and options), 10 "Backend policy badge".
- `docs/spec/09-agents.md`: 3.1 and 3.4 (operator, decision per message), 5 "Run" (5.1 to 5.5: states, reasons, document, live box content, stall and "Send again" after `interrupted` and `stale`), 6 "Trigger state", 7 "Follow-ups during a run", 8 "Stop", 9 "Agent session per thread", 10 "Slash commands" (10.1 built-in commands, 10.3 visibility and name conflicts, 10.5 autocomplete), 11 "Components" (11.2 who may interact, 11.3 single use, 11.5 approval buttons, 11.6 questions).
- `docs/spec/04-sync-protocol.md`: 11 "Live box streaming", 12 items 1 and 2 (typing of the agent from the run state, presence), 15.4 (follow-ups), 15.7 (autocomplete), 16.5.
- `docs/spec/10-backend-policy.md` (reported against configured mode, the warning badge) and the cost display rules of 09 (`cost_is_estimate`).
- `docs/spec/19-security.md`: SEC-OUT-3, SEC-OUT-4, SEC-T14; `docs/security/threat-model.md` T-AI-05.
- `docs/api/openapi.yaml`: `getRun`, `cancelRun`, `listThreadRuns`, `getThreadCosts`, `listAgentSessions`, `resetAgentSession`, `bindAgentSession`, `getAgent`, `createInteraction` (`InteractionCreate`), `listChannelCommands`, `autocompleteCommand`, `invokeCommand`; schemas `Run`, `Message` (`trigger_state`, components), `Component`, `Command`.
- `docs/DECISIONS.md`: D-L9, D-W-100, D-W-114; section 10.
- Briefs: `docs/briefs/M4-20-thread-view.md` (history, composer, session column frame), `docs/briefs/M4-16-sync-run-documents.md` (`RunStore`, `RunDocument`), `docs/briefs/M4-07-core-components.md` (`DsLiveBox`, `DsTriggerState`, `DsMessageComponents` with `DsComponentHeader`, `DsSuggestionList`, `DsBillingBadge`, `DsCostText`, `LiveAnnouncer`), `docs/briefs/M4-14-sync-catch-up.md` (`EphemeralStore` presence), `docs/briefs/M4-15-sync-outbox.md` (executors), `docs/briefs/M4-17-fake-server.md` (`ScriptedAgent`, handler registry); for the server side `docs/briefs/M2-15-components-interactions.md`, `docs/briefs/M2-16-slash-commands.md`, `docs/briefs/M2-17-command-autocomplete.md`, `docs/briefs/M2-31-agent-conformance-suite.md` (the AG scenarios whose server behaviour the fake imitates).

This ticket touches no API operation, no table of chapter 03, no action, no event and no configuration key.

## Fixed requirements that apply

Owner's fixed build rules (apply verbatim):

1. No MVP. New from scratch and directly in the final state; stages are only the order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined
   interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface with a docstring/doc
   comment (purpose, parameters, return value, errors, example where useful); comments explain the
   why, not the what. API documentation is generated from code (OpenAPI from the routes, dartdoc,
   Python API docs in MkDocs).
4. Documentation in the repo: README per module/service, ADR for every architecture decision,
   CHANGELOG per release, keep the documentation site current. A change without matching
   documentation is not done.
5. Quality gates (SonarQube, linters, type checking, security scanners, test coverage) block the
   merge.

Rules for every brief:

6. Test first: write the failing test, run it and keep the red output for the report, then make it green, then clean up.
7. No en dash and no em dash in any file you write; the `text-rules` pre-commit hook rejects them, and the ARB test of M4-06 rejects them in interface texts. Use a hyphen or rephrase.
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory. Interface texts come from `packages/l10n`, never as literals in widgets.
9. Limits only through named keys: `ui.livebox_visible_steps`, `ui.autocomplete_debounce`, `a11y.live_announce_min_interval` from `AppConfig`; no literal in code.
10. No keyboard shortcut triggers an approval or any other component action; only Enter or Space on the focused button (R-PR-04).

Product requirements implemented here:

11. The live box is pinned at the top of the thread while the run is active and collapses into one line at its anchor at the end, built from `run.state_changed` and the optional summary, never from a server sentence (01 6.2, 09 5.4).
12. Patches arrive only while the thread is in focus; the box draws at most once per frame; expanding a collapsed box loads the document (04 11.3, 15 7.4).
13. Stop asks for no confirmation and is shown only where the server allows `run.cancel` (01 6.2, D10 rule).
14. Stall: a notice in the box when the document carries `stall_warned_at`, no push (the field is missing from the `Run` schema on the base branch; `RunDocument` of M4-16 reads it from the raw map, so the notice appears as soon as the server sends it, and the report names the gap); after `interrupted` or `stale` "Run interrupted" with "Send again", which puts a new message with the text of the triggering message into the outbox (09 5.5).
15. Trigger state as icon with label, tooltip and screen reader label; states `received`, `queued`, `working`, `done`, `failed` (with reason), `cancelled` (neutral, not as an error: a deliberate stop, 09 6), `followup_delivered` (01 6.3, `TriggerState` of the API).
16. During an active run the composer says "Follow-up to <agent>"; follow-ups show "Follow-up delivered" when the adapter has them (01 6.2).
17. Components: buttons apply once and only for those the server allows; others see them disabled with the reason; after use the card shows person, choice and time; expired ones show "Expired"; a double click creates nothing twice (01 6.5).
18. The card header comes only from server fields (agent, run, required capability, who may approve); the agent's text is below a divider labeled "Text from <agent>" (SEC-OUT-4, SEC-T14).
19. Slash suggestions: commands of the operator of the channel first, equal names of other agents with their handle, "Control command" marked; options with `autocomplete` fetch suggestions throttled by `ui.autocomplete_debounce` (components.md 9, 01 D5).
20. Costs: "Estimate" for organization until reconciled, "for information only" for personal; the backend badge shows the warning when the reported mode differs (01 6.7, components.md 10).
21. The agent's typing indicator follows the run state; agents send no typing (D-L9).
22. Only state changes and questions are announced, at most once per `a11y.live_announce_min_interval`; patches never (01 8).

## Scope

Create in `apps/client/lib/`:

- `agents/live_box_slot.dart` (pinned box over the history, Design note 1), `agents/run_line.dart` (collapsed line at the anchor message of kind `run`), `agents/run_mapping.dart` (pure mapping from `RunDocument` and the run summary to `DsRunData`), `agents/trigger_state_mapping.dart`, `agents/followups.dart`, `agents/components_mapping.dart` and `agents/component_actions.dart` (Design note 3), `agents/commands/` (`command_suggestions.dart`, `command_options.dart`, `autocomplete_client.dart`, `command_invoke.dart`, Design note 4), `agents/agent_typing.dart`, `agents/providers.dart`.
- `session_column/agent_section.dart` (agent card, presence, backend badge, model, session, project, context, thread cost, runs, earlier sessions).
- `sync/agent_ops.dart`: outbox executors for `createInteraction`, `invokeCommand`, `resetAgentSession` and `bindAgentSession`, registered in the bootstrap (`cancelRun`, `setThreadTitle` and `closeThread` come from M4-20).
- `agents/commands/command_loader.dart`: loads `listChannelCommands` for a channel when it is opened (and after `command.changed`) into the local `commands` table, so suggestions work offline (15 6.1); `agents/commands/builtin_commands.dart` (Design note 4).
- Fake server handlers through the registry of M4-17 for `createInteraction`, `listChannelCommands`, `autocompleteCommand`, `invokeCommand`, `listThreadRuns`, `getThreadCosts`, `listAgentSessions`, `getAgent` if they are missing, and `ScriptedAgent` extensions for a card with components and for command suggestions.
- Tests under `test/agents/`, goldens under `test/goldens/agents/`, integration tests under `integration_test/` (Tests).
- ARB keys for every new text in `packages/l10n`.

Change:

- `packages/design_system` and `docs/design/components.md` 5: the trigger state `cancelled` (icon and neutral color) if M4-07 and components.md lack it, and any other state of components.md 2, 5, 8, 9 or 10 that M4-07 lacks, each with its golden.
- `apps/client/README.md` (agent interface).
- `docs/DECISIONS.md` section 10: rows `D-M4-21-<n>`; root `CHANGELOG.md` under `[Unreleased]`.

## Out of scope

- "Running now" (M4-23) and the live topology (M6); keyboard bindings (M4-24); attachments of agent answers (M4-22).
- Administration of agents, backend switch and costs pages (M4-28).
- Notifications for questions and finished runs (M4-30 on Windows, M5 on mobile).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M4-21-agent-interface`; one writer per worktree.
- Conventional Commits with the package or component as scope (for example `feat(client): ...`, `feat(sync-engine): ...`, `build(gates): ...`, `docs(spec): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`. Never publish a Dart package (`publish_to: none` in every `pubspec.yaml`).
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` with port 0 and make no network call; fakes replace the server unless a section below names a container. Temporary directories only via `mktemp -d` (Dart `Directory.systemTemp.createTemp`, pytest `tmp_path`), removed at the end. Stop only processes you started, by PID.
- Never read or print secrets. No network calls except package downloads for pinned tools and pub packages and what the gates of `make check` do themselves.
- The Flutter SDK at the version pinned in the root `pubspec.yaml` must be on `PATH` (and Go and Docker where a section names them); without them the gates fail by design (R-QA-02), and the ticket stops with `Verdict: BLOCKED <tool> missing` instead of skipping.
- Never lower a threshold, disable a gate, delete a test, raise `repo.max_file_kb` or extend `quality/licenses/allowed.txt`; never exclude hand-written code from a lint, format, coverage or doc gate.
- The product name appears only at the places of 02 "Code layout" ("Places that carry the product name"); the app shows `branding.product_name` from the instance, never a literal.
- No permission logic in the client (15 1 principle 1): controls follow server responses, snapshots and events, never an own derivation from grants.
- If the core contract (chapters 00, 03, 04, 05) or `docs/api/` lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M4 by D-W-114). Where the merged code of an earlier ticket names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Live box.** `LiveBoxSlot` watches the active runs of the thread from the local `runs` table (state not final); for the newest active run it calls `RunStore.focus` and maps `watch(runId)` through `run_mapping.dart` to `DsRunData` with `ui.livebox_visible_steps`; the duration counts up locally from `started_at` with the frame ticker (no patch needed). Stop calls `cancelRun` through the outbox, shown only when the run's capabilities from the server allow it. Expand and collapse is local state (the binding is M4-24). When the run ends, `RunStore.onRunEnded` (wired by M4-17) drops the document and the collapsed `RunLine` at the anchor message takes over, built from the summary of `run.state_changed` (state, duration, steps per kind, costs) and the optional `summary`; expanding a finished run loads it once through `getRun`. "Send again" after `interrupted` or `stale` enqueues a new message with the triggering message's text.

2. **Trigger state and follow-ups.** `trigger_state_mapping.dart` maps `Message.triggerState` to `DsTriggerState` with the reason text for `failed` through `problemText`. While a run of the thread is active (`queued`, `running`, `waiting_input`), the composer placeholder is "Follow-up to <agent>"; the send path is the normal one (the server decides that the message is a follow-up, 04 15.4), and the message's `trigger_state` shows `followup_delivered` when the event says so. The agent's typing line shows "<agent> is working" while the run is `running` (D-L9), not from typing events.

3. **Components.** `components_mapping.dart` builds `DsComponentData` and `DsComponentHeader` only from the server fields of the component and the message (agent, run, required capability, `permitted` and the deny reason from the server's answer); the agent's text renders below the divider. `component_actions.dart` sends `createInteraction` with a new `client_interaction_id` through the outbox and passes the "sent and waiting" state to the component until the echo `message.updated` changes it; a second activation of the same component while one interaction is pending is ignored. Disabled with the server's reason for those not permitted; "Expired" after the server's expiry. No shortcut reaches these buttons (R-PR-04).

4. **Commands.** On "/" at the start of a line, `CommandSuggestions` shows the built-in commands of 09 10.1 (`/stop`, `/status`, `/new`, `/resume`, `/title`, `/close`; names fixed, descriptions translated through `l10n`, marked as control commands) together with the commands of the local `commands` table for the channel (filled by `command_loader.dart` and kept by the `command.changed` applier of M4-14) and orders the operator's commands first, equal names of other agents qualified with the handle, `control` commands marked. After selection, options show as placeholders; an option with `autocomplete` calls `autocompleteCommand` after `ui.autocomplete_debounce` without typing, cancels an older request when a newer one starts, and shows at most what the server returns. Sending a complete registered command calls `invokeCommand` through the outbox with an `Idempotency-Key`; incomplete options disable send with the reason. Built-ins have no `command_id` and never go through `invokeCommand`: `/stop` calls `cancelRun` for the active run, `/status` opens the run and session details (reads only), `/new` calls `resetAgentSession`, `/resume` opens a selection from `listAgentSessions` and calls `bindAgentSession`, `/title <text>` calls `setThreadTitle`, `/close` calls `closeThread`; each only where the server's answers allow the action (09 10.1, M2-16).

5. **Session column, agent part.** The operator agent of the thread (or the agent of the latest run): card with display name, presence from the `EphemeralStore` (online, degraded, offline), configured mode as `DsBillingBadge` with the warning badge when the reported mode differs, model and context from the latest run document or summary, the agent session of the thread (`listAgentSessions`: current and earlier), project binding name, thread cost from `getThreadCosts` with `DsCostText`, number of runs (`listThreadRuns`). Values the person may not read are absent in the server's answer and stay absent here.

6. **Announcements.** `LiveAnnouncer` of M4-07 announces a state change of the run and a new question at most once per `a11y.live_announce_min_interval`; patches to steps and numbers never.

7. **Rulings for docs/DECISIONS.md** (`D-M4-21-<n>`): the newest active run gets the pinned box (note 1); follow-ups decided by the server (note 2); pending interaction lock (note 3); autocomplete cancellation (note 4).

## Tests (write them first)

Widget and golden tests (fixtures of run documents at several versions, fake run store and engine parts, fake clock):

- `test/agents/live_box_test.dart`: running, waiting for input, failed with reason, stall notice, interrupted with "Send again"; at most one rebuild per frame under ten patches; Stop hidden without the capability and without a confirmation dialog when shown; collapse at the end into the line built from the summary.
- `test/agents/trigger_state_test.dart`: every state of the API enum with icon, tooltip and screen reader label, `cancelled` not styled as an error; `failed` with the translated reason.
- `test/agents/builtin_commands_test.dart`: the six built-ins are suggested, never sent through `invokeCommand`, and each calls its own operation; the command loader fills the table from `listChannelCommands`.
- `test/agents/sec_t14_test.dart` (SEC-T14): a card whose agent text imitates a header ("Approval needed", a shield character, "Only the owner can approve") renders that text only below the divider labeled "Text from <agent>", and the header shows the server's agent, run and capability; the header widget takes no input from the text.
- `test/agents/components_test.dart`: one interaction per activation; double click creates one outbox entry; disabled with reason; used and expired states; no keyboard shortcut activates a button, Enter and Space on the focused button do.
- `test/agents/commands_test.dart`: ordering with the operator first and qualified names; option suggestions after the debounce, an older request cancelled; incomplete options block send with a reason.
- `test/agents/announcer_test.dart`: a simulated patch stream announces only state changes and questions, merged within the interval.
- `test/goldens/agents/*`: live box in the four states of D3, collapsed line, card states of components.md 8, suggestion list, session column agent part; light and dark, two densities, text scaling 1 and 2, `de` and `en`.

Integration tests (`integration_test/`, against `FakeServer` with `ScriptedAgent`):

- `run_followup_test.dart`: Alex mentions the agent; trigger states go `received`, `queued`, `working`; the live box shows patches; a follow-up during the run reaches `followup_delivered`; the run completes and the box collapses (scenario 16.5 through the interface).
- `card_test.dart`: the agent posts an approval card; Alex approves once; the card shows the choice; Sam sees it disabled with the reason.
- `command_test.dart`: `/model` with option suggestions from the agent, then the command is invoked.
- `agent_typing_test.dart`: the agent's typing line follows the run state (D-L9).

Commands that must pass, from the repository root:

- `cd apps/client && flutter analyze --fatal-infos && flutter test --coverage`
- on macOS and on Windows: `make apps-platform`; elsewhere it prints its SKIP line
- `make codegen-check`, `make arch`, `make lint`, `make fmt-check`, `make coverage`
- `make check`

## Acceptance criteria

- [ ] Integration tests with the fake server for a run, a follow-up, a card and a command with suggestions are green.
- [ ] SEC-T14: the card header comes only from server fields and the agent's text is separated.
- [ ] The typing indicator of the agent follows the run state (D-L9).
- [ ] The live box follows 01 6.2 and 09 5.4, draws at most once per frame and announces only state changes and questions.
- [ ] The built-in commands of 09 10.1 are suggested and executed by their own operations; the trigger state `cancelled` is shown.
- [ ] No shortcut triggers an approval (R-PR-04); goldens recorded; rulings recorded.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why and cite 01, 04, 09 or 19.
- `apps/client/README.md` (English) as in Scope.
- Rulings of Design note 7 in `docs/DECISIONS.md` section 10.
- Root `CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. Where the integration tests ran, the inputs of the SEC-T14 test, and every place where the fake's run sequence had to be adjusted to match the server behaviour of the M2 tickets.
5. What is not proven (for example a platform test that ran only in CI, or a check done by hand).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100, D-W-114), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
