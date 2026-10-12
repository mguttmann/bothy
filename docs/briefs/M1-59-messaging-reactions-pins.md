# M1-59: Reactions and pins

Status: ready | Depends on: M1-58 | Milestone: M1

## Goal

After this ticket people and agents react to messages with any single Unicode symbol, and holders
of `manage` pin messages as moderation. Reactions are free expression and never a status: they do
not touch `trigger_state`. Both are idempotent, produce the durable events `reaction.added`,
`reaction.removed`, `pin.added`, `pin.removed`, and appear in every `Message` representation as a
summary, the caller's own reactions and the pinned flag. Clients render reaction bars, the "who
reacted" list and the pinned list of a thread from this.

## Read first

- `docs/PLAN.md`: section 3 "Way of working per ticket"; section 7 "M1 Foundation" (row M1-59,
  "Red team").
- `docs/briefs/README.md`.
- `docs/spec/00-glossary.md`: reaction, pin, trigger state.
- `docs/spec/01-product.md`: "Feature list" 3.3 Messages (reactions, pins).
- `docs/spec/02-architecture.md`: 4.1, 4.3 row `messaging` (ports `Reactions`, `Pins`), 4.4, 4.7,
  "Layers within a module" (no third-party libraries in `domain` and `app`), "Transactions and
  Unit of Work", "Request path".
- `docs/spec/03-data-model.md`: "Principles" (items 10, 12, 13), tables `messages`, `reactions`,
  `pins`; "Transaction rules" 6.3.
- `docs/spec/04-sync-protocol.md`: "Envelope", "Delivery guarantee" (item 3), "Ordering".
- `docs/spec/05-actions.md`: rules 1.1, 1.2, 1.8; "Target of a resource" (resources `reaction`,
  `pin`); "Decision"; table 4.7 (`reaction.add`, `reaction.remove`, `pin.create`, `pin.delete`,
  `message.read`), table 4.6 (`thread.read`); section 6 rows `reaction.*`, `pin.*`; R-AZ-08.
- `docs/spec/06-api.md`: "Basic rules", "Authorization in the API", "Errors", "Pagination",
  "Idempotency" (PUT and DELETE), "Endpoints" > "Messages", "Configuration values of this chapter".
- `docs/spec/07-permissions.md`: 4.1, 4.2, 4.3, 7.4.
- `docs/spec/15-client-architecture.md`: section 5 (event application by `resource.version`) and
  6.1 (`reactions_summary`).
- `docs/spec/19-security.md`: "Input validation in the server" (SEC-IN-1, SEC-IN-2), SEC-LOG-4.
- `docs/api/openapi.yaml`: operations `addReaction`, `removeReaction`, `listReactions`,
  `pinMessage`, `unpinMessage`, `listPins`; schemas `Message` (`reactions`, `my_reactions`,
  `pinned`), `ReactionSummary`, `Reaction`, `ReactionPage`, `Pin`, `PinList`, `ReactionPayload`,
  `PinPayload`, `ReactionAddedEvent`, `ReactionRemovedEvent`, `PinAddedEvent`, `PinRemovedEvent`;
  parameters `EmojiPath`, `Cursor`, `Limit`; response `Gone`.
- ADRs: `docs/adr/0008-hexagonal-modules-and-checked-dependency-rules.md`,
  `0015-error-format-rfc-9457-idempotency-pagination.md`,
  `0017-one-authorization-function-404-instead-of-403-mandatory-registration.md`.
- Code of earlier tickets: `internal/messaging` (M1-56: tables, representation function, resource
  descriptor), `internal/identity` port `Directory` (PrincipalRef data; M1-26), `kernel/authorize`
  (`Scope`; M1-28), the cursor codec of earlier list operations (search for `next_cursor` in
  `services/server/internal`), `pgtest` (M1-12), `api/operations.yaml` (M1-17),
  `architecture/modules.yaml` (M1-05), the configuration schema (M1-07) and `tools/configdoc`
  (M1-08).
- `docs/quality/gates.md`, `docs/quality/licenses.md`, `CONTRIBUTING.md`,
  `docs/quality/red-team-gate.md`, `services/server/internal/messaging/README.md`.
- Tables: `reactions`, `pins` (new), `messages` (read and row lock). Actions: `reaction.add`,
  `reaction.remove`, `pin.create`, `pin.delete`, `message.read`, `thread.read`.

## Fixed requirements that apply

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
6. Test first: write the failing test, record it red, then make it green.
7. No en dash or em dash characters in anything you write; use a hyphen or rephrase.
8. White label: examples use Alex, Sam, Robin, Support, Marketing, Engineering, Research and
   `example.com` only.
9. Limits only through named configuration keys from `docs/operations/configuration.md`.
10. A reaction is exactly one Unicode grapheme cluster, checked in the application
    (03 `reactions`); a reaction is never a status and never changes `trigger_state`
    (00 "Reaction"; 01 3.3; 03 `reactions`).
11. A repeated reaction of the same principal with the same symbol is idempotent (PK of
    `reactions`; 06 "Idempotency": PUT and DELETE).
12. `reaction.add` needs `write`; `reaction.remove` removes only the caller's own reaction
    (05 table 4.7). Read-only channels refuse both with `403 channel_read_only` (05 rule 1.8).
13. Pinning and unpinning need `manage` (05 `pin.create`, `pin.delete`; R-AZ-08); a principal who
    may read but lacks `manage` gets `403`, one who may not read gets `404` (05 rule 2).
14. Events: `reaction.added`, `reaction.removed` with receive action `message.read`; `pin.added`,
    `pin.removed` with receive action `thread.read`; resource kind `message` (05 section 6).
15. `my_reactions` appears only in REST responses, never in events (openapi `Message`).
16. Lists filter in SQL with the visible channels and check each element with `authorize`
    (07 7.4).
17. Third-party libraries only in adapters (02 "Layers within a module"), pinned and with an allowed
    license (`quality/licenses/allowed.txt`).

## Scope

All paths below `services/server/` unless stated.

1. Migration `migrations/<next number>_reactions_pins.sql`: tables `reactions` and `pins` exactly
   as 03 (columns, primary keys, FKs with ON DELETE behaviour, index `(thread_id, pinned_at DESC)`
   on `pins`). Use the next free goose number at your local merge; renumber before merging if a
   parallel ticket took it.
2. Port `internal/messaging`:
    - `Reactions` with `Add(ctx, messageID ids.ID, emoji string) error`,
      `Remove(ctx, messageID ids.ID, emoji string) error`,
      `List(ctx, q ReactionListQuery) (ReactionPage, error)`.
    - `Pins` with `Pin(ctx, messageID ids.ID) error`, `Unpin(ctx, messageID ids.ID) error`,
      `List(ctx, threadID ids.ID) ([]PinView, error)`.
    `ids.ID` is the UUID type of `kernel/ids`; the actor comes from the context.
3. `internal/messaging/domain`: summary ordering (note 7).
4. `internal/messaging/app`: use cases for the six operations; outbound port
   `SymbolNormalizer{Normalize(raw string, maxBytes int) (string, error)}`; batch loaders that fill
   `reactions`, `my_reactions` (REST only) and `pinned` in the single representation function of
   M1-56 for one message or a page (one query per field set, no query per message).
5. `internal/messaging/adapters/unicode`: the `SymbolNormalizer` implementation (note 3) with
   `github.com/rivo/uniseg` (MIT, UAX #29 grapheme clusters) and `golang.org/x/text/unicode/norm`
   (BSD-3-Clause, NFC), both pinned to a released version in `go.mod`; allow exactly these two
   libraries for this adapter in `architecture/modules.yaml` and regenerate `.go-arch-lint.yml`.
6. `internal/messaging/adapters/postgres/queries/reactions.sql`, `pins.sql`, regenerated
   `sqlcgen/`.
7. `internal/messaging/adapters/httpapi`: `AddReaction`, `RemoveReaction`, `ListReactions`,
   `PinMessage`, `UnpinMessage`, `ListPins`.
8. The specification is in place: `docs/api/openapi.yaml` has `410 Gone` on `addReaction` and
   `pinMessage`; 03 9 defines `reaction.emoji_max_bytes`, `reaction.max_distinct_per_message` and
   `pin.max_per_thread`; 03 `reactions` and `pins` state the limits (`400 validation_failed`, code
   `limit_reached`); 04 9 item 3 and 15 5 state that reaction and pin events apply in `seq` order
   without raising the message version. Change them only where the implementation proves them
   wrong.
9. Configuration: add the three keys to the configuration schema (M1-07) and keep
   `docs/operations/configuration.md` current (`make config-doc-check`).
10. `api/operations.yaml`: enable the six operations; regenerate.
11. Docs: `internal/messaging/README.md` (English, existing structure), `services/server/CHANGELOG.md`,
    `docs/DECISIONS.md` entry for note 3 (notes 6 and 9 are in the specification).

## Out of scope

- Editing, deleting, revisions and redaction (M1-58), except the coordination of note 9.
- History paging (M1-57), mentions and templates (M1-60), push for reactions (none planned).
- Delivery of the events over the gateway (M1-65); client rendering (M4); import of pins (M8).

## Boundaries

- A new migration needs its line in `migrations.sha256`: run `make migrations-lock` after creating
  the file and before `make check`; never edit a merged migration (M1-01, ADR 0007).
- Work in a dedicated git worktree on the branch `feat/M1-59-messaging-reactions-pins`, created
  from the base branch the launcher names, after checking that the tree is clean and M1-56 is
  merged.
- Conventional Commits with scopes `messaging`, `api`, `config`, every commit with DCO sign-off
  (`git commit -s`); squash into meaningful commits before the local merge.
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope list, except the CHANGELOG entry and `docs/DECISIONS.md`
  entries named here.
- Follow the host rules supplied by the launcher.
- Tests bind only `127.0.0.1` and port 0; PostgreSQL 18 through `pgtest` on a random port; without
  a Docker runtime the test fails, it never skips.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`; remove them.
- Never read or print secrets; never open `.env` files.
- No network calls except package downloads for pinned tools and pinned Go modules.
- Do not lower a threshold, disable a gate, skip or delete a test.
- If the core contract (00, 03, 04, 05) lacks something this brief needs, stop with
  `Verdict: BLOCKED <reason>` (P-11).

## Design notes

1. Operations.

    | operationId | Route | Action | Resource | Class |
    |---|---|---|---|---|
    | `addReaction` | `PUT /api/v1/messages/{message_id}/reactions/{emoji}` | `reaction.add` | message | `rl.write` |
    | `removeReaction` | `DELETE /api/v1/messages/{message_id}/reactions/{emoji}` | `reaction.remove` | reaction (message path, owner = caller) | `rl.write` |
    | `listReactions` | `GET /api/v1/messages/{message_id}/reactions` | `message.read` | message | `rl.read` |
    | `pinMessage` | `PUT /api/v1/messages/{message_id}/pin` | `pin.create` | message | `rl.write` |
    | `unpinMessage` | `DELETE /api/v1/messages/{message_id}/pin` | `pin.delete` | message | `rl.write` |
    | `listPins` | `GET /api/v1/threads/{thread_id}/pins` | `thread.read` | thread | `rl.read` |

    All are idempotent by HTTP semantics; none takes `Idempotency-Key`.

2. Order of checks for writes: normalise the symbol first (no lookup, so it cannot leak; `400`);
   load the message with the visible predicate and call `authorize` (missing and invisible share
   one code path, `404`; `403` with the decision's reason, for example `level_insufficient`,
   `capability_missing`, `channel_read_only`); then, inside the unit of work, lock the message row
   (`SELECT ... FOR UPDATE` with the visible predicate) and check `deleted_at`: set gives
   `410 message_deleted` for `addReaction` and `pinMessage`, and `204` without change for
   `removeReaction` and `unpinMessage`. The row lock serialises all reaction and pin changes of one
   message, so the counts in the events are exact.
3. Symbol rule (`SymbolNormalizer.Normalize`), applied to the decoded path value and to the
   `emoji` query filter: valid UTF-8; NFC; exactly one extended grapheme cluster after UAX #29; at
   most `reaction.emoji_max_bytes` bytes; no code point of category Cc, Cs, Zs, Zl or Zp; none of
   the Bidi controls U+061C, U+200E, U+200F, U+202A to U+202E, U+2066 to U+2069; and not made only
   of code points of category Cf or Mn (invisible). Category Cf is otherwise allowed, because emoji
   ZWJ and tag sequences need it. Errors: `400 validation_failed` with pointer `#path/emoji` (or
   `#query/emoji`) and code `invalid_emoji`. Store and return the normalised form. Reason for the
   byte cap: Unicode puts no bound on one grapheme cluster (combining marks stack); the cap keeps a
   reaction a single symbol.
4. `addReaction`: `INSERT ... ON CONFLICT DO NOTHING`. Only when a row was inserted: append
   `reaction.added` with actor = caller, scope = space, channel, thread, resource = `{kind:
   message, id, version: current message version}`, payload `ReactionPayload{message_id,
   thread_id, emoji, principal_id, count}` where `count` is the number of rows for this message and
   symbol after the insert. `204` in both cases. `messages.version`, `trigger_state`,
   `trigger_state_at` and `trigger_state_reason` stay unchanged.
5. `removeReaction`: deletes only the caller's row for this message and symbol. Only when a row was
   deleted: append `reaction.removed` with the count after the delete. `204` in both cases; an
   invisible message gives `404`.
6. Message version (03 `messages.version`, 04 9 item 3, 15 5): reactions and pins do not raise
   `messages.version`, because raising it would make an author's next edit fail with `412` after
   someone reacted. Their events carry the current message version in `resource.version`; the
   payload carries absolute values (`count`, the pin), so applying it twice is harmless; clients
   apply them in `seq` order.
   Limits: adding a new symbol to a message that already has `reaction.max_distinct_per_message`
   distinct symbols, and pinning in a thread with `pin.max_per_thread` pins, give `400
   validation_failed` with code `limit_reached` (pointer `#path/emoji` or empty, respectively);
   checked under the message row lock so the bound is exact.
7. Representation (one function for REST and events): `reactions` is a list of `{emoji, count}`
   for the message, ordered by the earliest `created_at` of each symbol, then by symbol;
   `my_reactions` lists the caller's symbols in the same order and appears only in REST responses
   (always present there, possibly empty); `pinned` is true iff a `pins` row exists. A tombstone
   (`deleted_at` set) has `reactions = []`, `my_reactions = []`, `pinned = false`. Pages load
   summaries, own reactions and pins with one query each for all message IDs of the page.
8. `listReactions`: keyset order `created_at` ascending, then `principal_id`, then `emoji`;
   optional `emoji` filter (validated by note 3); `limit` with `api.page_size_default` and
   `api.page_size_max`. Use the cursor codec of `internal/platform/httpserver` (M1-18), bound to
   the list name, the message and the filter (a cursor of another list or filter gives `400
   cursor_invalid`). Items `Reaction{emoji, principal: PrincipalRef, created_at}` with
   `PrincipalRef` from the `identity` `Directory` port in one batch call. A deleted message returns
   an empty page.
9. Deletion of a message removes its `reactions` and `pins` rows in the same transaction (03
   `messages`, 20 4.2; the event payloads are redacted by M1-58). M1-58 is a dependency of this
   ticket: add `DELETE FROM reactions` and `DELETE FROM pins` for the message at the marked step of
   its delete use case and the test `TestDeleteMessage_RemovesReactionsAndPin`. Note 7 keeps
   tombstones free of reactions and pins.
10. `pinMessage`: `INSERT INTO pins (message_id, thread_id, pinned_by, pinned_at) ... ON CONFLICT
    DO NOTHING`; `thread_id` is the message's thread. Only when inserted: append `pin.added` with
    payload `PinPayload{pin}`. Already pinned: `204` without event. Any message kind may be pinned.
    Read-only channels allow pinning, because `manage` actions stay allowed (05 rule 1.8).
11. `unpinMessage`: delete the row; only when deleted append `pin.removed` with the pin as it was.
    `204` in both cases.
12. `listPins`: `authorize(thread.read, thread)`; `SELECT` joins `messages` with
    `m.channel_id = ANY(@visible_channel_ids)` and `m.deleted_at IS NULL`; order `pinned_at DESC`,
    then `message_id DESC`; items `Pin{message_id, thread_id, pinned_by, pinned_at}`; unpaginated
    (`PinList`, bounded by `pin.max_per_thread`). Each item passes `authorize(message.read)`
    in filter mode `authorize.Filter(ctx)` (M1-28; no denial audit, 07 4.2); a
    denied item is dropped, logged with only action and resource kind and counted in
    `bothy.authz.errors` with `bothy.reason = filter_mismatch` (07 7.4 rule 3).
13. Every `SELECT`, `UPDATE` or `DELETE` on `reactions` or `pins` joins `messages` with
    `channel_id = ANY(...)`; keep `TestQueriesCarryVisibleChannelPredicate` (M1-57) green when it is
    on your base.
14. Logs and telemetry never carry the symbol, content or display names (SEC-LOG-4).

## Tests (write them first)

Unit:

- `TestSymbolNormalizer` (in `adapters/unicode`) accepts: a thumbs-up, a thumbs-up with skin tone
  modifier, the ZWJ family of four, the kiss sequence of two people with two skin tones, the
  England subdivision flag (tag sequence), the keycap one, a single letter, the letter e with acute
  in composed and in decomposed form (both return the composed form). Rejects: empty string, two
  thumbs-up, two letters, a space, U+200F alone, U+202E alone, U+0007, U+200B alone, a lone
  combining mark, a letter followed by more combining marks than `reaction.emoji_max_bytes`
  allows, invalid UTF-8.
- `TestReactionSummaryOrder` (domain).

Integration against PostgreSQL 18 (`pgtest`, world of 07 "Examples", HTTP through the real chain):

- `TestAddReaction_IdempotentSingleEvent`: two identical calls give `204` twice, one row, one
  `reaction.added` with `count = 1`.
- `TestAddReaction_CountsPerSymbol`: Alex and the Owner react with the same symbol, counts 1 and 2;
  a different symbol has its own count.
- `TestAddReaction_ConcurrentCountsExact`: parallel adds of the same symbol by several principals
  produce events whose counts are exactly 1 to n.
- `TestAddReaction_IsNotStatus`: on a message with `trigger_state` set, adding and removing a
  reaction leaves `trigger_state`, `trigger_state_at`, `trigger_state_reason` and `version`
  unchanged; changing `trigger_state` through M1-56's `SetTriggerState` (M1-56 is in this ticket's closure)
  creates no reaction row.
- `TestAddReaction_Rights`: a principal with `read` only gets `403 forbidden`; Alex on a Marketing
  message gets `404`, equal to a random UUIDv7; a read-only channel gives `403 channel_read_only`;
  an operator agent reacts in its channel.
- `TestAddReaction_DeletedMessage410` (tombstone through the delete operation of M1-58).
- `TestReactionAndPinLimits`: with small limits in the test configuration, one more distinct symbol
  and one more pin give `400 validation_failed` with `errors[].code` `limit_reached`; an existing symbol can still be added by another
  person.
- `TestAddReaction_InvalidSymbol400` with pointer `#path/emoji`.
- `TestRemoveReaction_OwnOnlyAndIdempotent`: removing removes only the caller's row; a second call
  `204` without event; the event carries the count after removal.
- `TestListReactions_PagingFilterAndPrincipals`: keyset order, `emoji` filter, `PrincipalRef`
  fields, foreign cursor `400 cursor_invalid`, invalid filter `400`, invisible message `404`.
- `TestMessageRepresentation_ReactionsAndPinned`: `getMessage` and `createMessage` responses show
  the summary, `my_reactions` and `pinned`; the payloads of `message.created` and
  `message.updated` never contain `my_reactions`.
- `TestPinMessage_RequiresManage`: Alex with `write` but without `manage` gets `403`; the Owner
  gets `204`; an agent gets `403`; Alex on a Marketing message gets `404`.
- `TestPinMessage_IdempotentAndEvents`: one `pin.added` for two calls; unpin twice gives one
  `pin.removed` carrying the former pin.
- `TestPinMessage_DeletedMessage410`; `TestPinMessage_ReadOnlyChannelAllowsManage`.
- `TestListPins_OrderAndVisibility`: order of note 12; pins of a tombstone are absent; invisible
  thread `404`.
- `TestReactionPinEvents_Envelope`: `event_log` rows have resource kind `message`, the receive
  actions of 05 section 6, scope with space, channel and thread, and `resource.version` equal to
  the message version.
- `TestDeleteMessage_RemovesReactionsAndPin` (note 9; M1-58 is a dependency of this ticket).

Commands that must pass:

- `cd services/server && go test -race ./internal/messaging/...`
- `make codegen-check`, `make migrations-lint`, `make arch`, `make api-lint`,
  `make config-doc-check`, `make licenses`, `make osv`, `make docs`
- `make check` (with the local SonarQube; if unreachable, `Verdict: BLOCKED`, never `SONAR_SKIP`).

## Acceptance criteria

- [ ] The six operations are registered and run `authorize` with their catalogue actions.
- [ ] A reaction is one normalised grapheme cluster within `reaction.emoji_max_bytes`; the cases of
      `TestSymbolNormalizer` pass.
- [ ] Reactions never change `trigger_state` or `messages.version`.
- [ ] Repeated reactions and pins are idempotent and produce no second event; events carry exact
      counts.
- [ ] Pinning without `manage` gives `403` for readers and `404` for non-readers.
- [ ] Events `reaction.added`, `reaction.removed`, `pin.added`, `pin.removed` are appended with the
      receive actions of 05.
- [ ] Every `Message` representation carries `reactions`, `pinned` and, in REST only,
      `my_reactions`.
- [ ] `410` for `addReaction` and `pinMessage` as in `openapi.yaml`; the three configuration keys
      of 03 9 are in the schema and the generated reference; the limits hold.
- [ ] The two new libraries are pinned, licensed as allowed, and used only in
      `adapters/unicode`; README, doc comments, CHANGELOG and DECISIONS.md entries are present.
- [ ] `make check` is green.

## Definition of done

Code, tests and docs are complete: module README updated, a doc comment on every public symbol,
an ADR if an architecture decision was taken (none expected; rulings go to `docs/DECISIONS.md`), a
CHANGELOG entry under `Unreleased`. `make check` is green. An independent review and the red team
(`docs/quality/red-team-gate.md`) have run over the diff and their verdicts are recorded in the
report. Merge locally into the base branch only if both are green, then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then the
commits, the files with line counts, the test results with counts (including the red run of each
new test), what is not proven, and follow-ups that could not be done in scope with the reason. Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or
`exit=1 <reason>`.
