# M1-52: Spaces and sidebar

Status: ready | Depends on: M1-46, M1-51 | Etappe: M1

## Goal

After this ticket the module `chat` owns spaces: the top level of the sidebar with a global order
kept as a fractional index (moving changes exactly one row), a colour and an icon from the design
system palette, archiving, and each person's collapsed state and own order, synchronised across
devices through personal events. `chat` becomes the space part of `authz.StructureSource`, so
`authorize` resolves spaces from real data. Channels, threads and operators build on this in
M1-53 to M1-55.

## Read first

- `docs/PLAN.md`, section 7: row M1-52, "Bekannte Widersprüche" (none for this ticket), "Red Team";
  section 3 "Arbeitsweise je Ticket". `docs/briefs/README.md`.
- `docs/spec/03-datenmodell.md`: "1. Grundsätze" (1, 5 sort keys, 6, 7, 10, 12, 13), tables
  `spaces`, `user_space_states`, `grants` (FK `target_space_id`), `agents`; "6.3 Sperrreihenfolge".
- `docs/spec/05-aktionen.md`: "2.2 Ziel einer Ressource", "2.3 Wirksame Stufe", "2.6 Scopes für
  Apps" (`space.read`), "4.5 Bereiche, Kanäle, Seitenleiste" rows `space.read`, `space.create`,
  `space.update`, `space.reorder`, `space.archive`, `sidebar.update`; "6" rows `space.created`,
  `space.updated`, `space.archived`, `sidebar.updated`; R-AZ-06.
- `docs/spec/07-rechte.md`: "4.3 Sichtbarkeit", "7.3 Gültigkeit des Caches", "7.4 Abfrage-Muster",
  "8.2 Sofortige Wirkung bei Entzug".
- `docs/spec/04-sync-protokoll.md`: "2.2 Umschlag", "8.1 Grundregel", "8.2 Was beim Ändern
  passiert", "14. Geräteübergreifende Synchronisation" (row Seitenleiste).
- `docs/spec/02-architektur.md`: "4.3" (module `chat`: port `Structure`, In-Tx event
  `SpaceArchived`, source `authz.StructureSource`), "4.4" (`chat` may use `identity`, `authz`),
  "4.5 Querschnitts-Ports", "5. Schichten im Modul", "7.1 Modulregister als Quelle", "8.1" and
  "8.2", "11.3 Rechte-Cache".
- `docs/spec/01-produkt.md`: "3.1 Struktur und Gespräche" (first row), "9. Sprachen", "14.
  Konfigurationswerte".
- `docs/spec/19-sicherheit.md`: SEC-IN-1, SEC-IN-4, SEC-T07.
- `docs/spec/06-api.md`: "6. Paginierung" (rule 6), "7. Idempotenz", "8. Nebenläufigkeit", "16.
  Endpunkte" groups "Spaces" and "Preferences".
- `docs/api/openapi.yaml`: operations `listSpaces`, `createSpace`, `getSpace`, `updateSpace`,
  `moveSpace`, `archiveSpace`, `putMySidebarSpace`; schemas `Space`, `SpaceCreate`, `SpaceUpdate`,
  `MovePosition`, `SpaceView`, `SpaceViewList`, `SidebarState`, `SidebarStatePut`, `ReasonBody`,
  `SpacePayload`, `SidebarPayload`, `SpaceCreatedEvent`, `SpaceUpdatedEvent`, `SpaceArchivedEvent`,
  `SidebarUpdatedEvent`.
- `design/build/tokens.resolved.json` (category colours `amber`, `gorse`, `granite`, `heather`,
  `loch`, `moss`, `rowan`, `spruce`) and the generated palette and icon key list of M1-46 with its
  gate check.
- Code already merged: `internal/authz` (snapshot, `authorize`, `Scope`, phase hook,
  `StructureSource` declaration and fakes), `internal/identity` (display-text validation of M1-26),
  `internal/agents` (M1-51), `internal/instance` (M1-46), `internal/kernel/*`,
  `internal/platform/pg/pgtest`, `services/server/api/operations.yaml`.
- `docs/quality/gates.md`, `CONTRIBUTING.md`, `docs/quality/red-team-gate.md`.

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

Note on rule 3: In this repository `docs/api/openapi.yaml` is normative and the server interface is
generated from it (ADR 0013); the route test keeps both identical.

6. Test first: every behaviour gets a test that fails before the code exists (red), then passes
   (green). Keep the red output for the report.
7. No en dash or em dash characters (U+2013, U+2014) in any file or commit message.
8. White label: example spaces Support, Marketing, Engineering, Research; people Alex, Sam, Robin;
   no company, host or person names.
9. Limits and durations only through named keys from `docs/operations/configuration.md`; no bare
   numbers in production code or documentation.
10. Specification first: the chapter and API changes below land before the code that uses them.
11. Product requirements implemented here: spaces collapsible and sortable, global order only by
    owners (`space.reorder`), own collapse and order per person, synchronised across devices (01
    "3.1", 04 "14"); colour and icon only as keys of the design system, never hex or emoji (03
    `spaces`); archiving as a rights change (05 `space.archive`, "Rechte" = `ja`); display texts
    without control or bidi characters (SEC-IN-4).

## Scope

The specification is in place (planning, spec first): 03 `spaces` (name limit
`space.name_max_chars` in 03 9, icon from `space-icons`), 07 "4.5" and "7.2"/"7.4" (archive rule;
`Scope` builds lists per request, so new spaces need no new rights version), and in
`docs/api/openapi.yaml` the descriptions of `SpaceView.level`, `SpaceCreate`, `SpaceUpdate` and
`archiveSpace`. Change them only where the implementation proves them wrong; run `make api-lint`,
`make api-checks` and the M1-17 generation target; `make codegen-check` shows no diff.

Code (paths under `services/server/`):

- Migration (next free sequential goose version in `migrations/`): tables `spaces` and
  `user_space_states` exactly as 03 (columns, checks, FKs, unique and partial indexes,
  `COLLATE "C"`). If `grants.target_space_id` has no foreign key yet (M1-27 ran before `spaces`
  existed), add `grants.target_space_id -> spaces(id) ON DELETE RESTRICT` here.
- `internal/chat` (port): `Structure` with the space and sidebar part: `ListSpaces`, `GetSpace`,
  `CreateSpace`, `UpdateSpace`, `MoveSpace`, `ArchiveSpace`, `PutSidebarSpace`,
  `ListSidebarStates(ctx, userID)` (needed by M1-61 `getMySidebar`); DTOs; In-Tx event
  `SpaceArchived{SpaceID ids.ID}`.
- `internal/chat/domain`: space entity and rules; package `internal/chat/domain/fracindex`
  (`KeyBetween(a, b string) (string, error)`, `Valid(key string) bool`).
- `internal/chat/app`: use cases per operation.
- `internal/chat/adapters/postgres` (sqlc queries `spaces.sql`, `sidebar.sql`, repository),
  `internal/chat/adapters/httpapi` (handlers of the seven operations),
  `internal/chat/adapters/authzsource` (the space part of `authz.StructureSource`).
- Shared helpers (kernel-core, 02 "4.5"): the generated palette and `space-icons` list in
  `internal/kernel/designkeys` (M1-46) and the display-text validator `internal/kernel/textcheck`
  (M1-26); use them as they are.
- `internal/authz`: only if the base lacks it (each with a test): in an archived space every
  effective level is capped at `read` (07 "4.5"; M1-53 builds the same cap for channels, share
  one implementation); `Scope` for spaces can express "all spaces" for owners and principals with
  instance-level `read`, so new spaces are listed without a new rights version; an
  effective level and capabilities query for one target (used for `SpaceView`), built on the same
  snapshot as `authorize`.
- `internal/wire`: register handlers and the space part of the structure source; keep the channel
  and thread part of the existing placeholder until M1-53 and M1-55.
- `services/server/api/operations.yaml`: enable the seven operations.
- Configuration: register `space.name_max_chars` (positive integer) in the typed `chat` config and
  the config schema.
- Docs: `internal/chat/README.md`, `services/server/CHANGELOG.md` under Unreleased, rulings in
  `docs/DECISIONS.md`.

## Out of scope

- `setSpaceOperator` and the operator rule (M1-54); channels, forum tags and channel scope (M1-53);
  threads (M1-55); `getMySidebar` and the snapshot (M1-61, M1-63).
- Notification preferences cleanup on archive (`notify` subscribes `SpaceArchived` in M3); personal
  data export and erasure for `user_space_states` (M7-06, P-14).
- An unarchive operation (not in 05).

## Boundaries

- A new migration needs its line in `migrations.sha256`: run `make migrations-lock` after creating
  the file and before `make check`; never edit a merged migration (M1-01, ADR 0007).
- Work in a dedicated git worktree on branch `feat/M1-52-chat-spaces`.
- Conventional Commits with scope (`feat(chat): ...`, `docs(spec): ...`) and DCO sign-off.
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope above except the CHANGELOG and docs entries named here.
- Follow the host rules supplied by the launcher.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`; remove
  them at the end.
- Never read or print secrets. No network calls except package downloads for pinned tools.

## Design notes

Catalog facts (05 "4.5"): `space.read` = `read` on the space or on at least one of its channels
(`rl.read`); `space.create` owner, no step-up, audit (`rl.admin`); `space.update` `manage`, no
step-up, audit (`rl.admin`); `space.reorder` owner, no step-up, audit (`rl.admin`); `space.archive`
owner, step-up, audit, "Rechte" = `ja` (`rl.admin`); `sidebar.update` self and read, no audit
(`rl.write`). Apps need scope `messages:read` for `space.read`.

Fractional index (`fracindex`, compatible with the widely used base-62 scheme with integer part):

- Digits `0-9A-Za-z` in ASCII order, so byte order equals key order (`COLLATE "C"`).
- A key is an integer part plus an optional fraction. The integer part starts with a head letter:
  `a` to `z` mean 1 to 26 following digits, `A` to `Z` mean 26 to 1 following digits (negative
  range). The fraction must not end with `0`. The smallest possible integer (`A` followed by 26
  `0`) is invalid.
- `KeyBetween(a, b)` returns a key strictly between `a` and `b`; empty means open end. Rules:
  both empty gives `a0`; `b` empty increments the integer part of `a` (and falls back to a
  fraction when the integer cannot grow); `a` empty decrements the integer part of `b` likewise;
  otherwise the shared prefix plus the digit midpoint. `a >= b` is an error.
- Normative vectors (tests): `("", "") -> "a0"`, `("a0", "") -> "a1"`, `("", "a0") -> "Zz"`,
  `("a0", "a1") -> "a0V"`. Property tests: result strictly between, valid, and repeated inserts at
  the end keep short keys.
- Order of lists: `sort_key COLLATE "C"`, then `id`.

Global order:

- `CreateSpace` with `after_space_id` inserts after that space, else at the end; `MoveSpace` with
  `MovePosition`: exactly one of `after_id` and `before_id`, none moves to the start, both give
  `400 validation_failed`, the space itself or an unknown or archived ID gives `400
  validation_failed` with the pointer of the field.
- To keep keys distinct under concurrency, create and move lock all non-archived space rows in
  ascending ID (`SELECT id FROM spaces WHERE archived_at IS NULL ORDER BY id FOR UPDATE`), then
  read neighbours and compute the key. Spaces are a small set (06 "6" rule 6), so this is cheap and
  follows the lock order of 03 "6.3". A move updates exactly one row (`sort_key`, `version + 1`).

Validation:

- Name: trimmed, non-empty, at most `space.name_max_chars` characters, no control or bidi
  characters (SEC-IN-4); unique case-insensitive among non-archived spaces, else `409 conflict`.
- `color`: a category colour key from the generated palette list; `icon`: an icon key from the
  generated list. Hex values, emoji and unknown keys give `400 validation_failed` (pointer
  `/color` or `/icon`). `SpaceUpdate` cannot clear `icon` or `color` (no `null`, no clear field;
  its description says so).
- `updateSpace` requires `If-Match` (`428` missing, `412` stale with `current`), bumps `version`.

Visibility and archive:

- `listSpaces`: non-archived spaces where `Scope` allows (owners and instance-level readers: all;
  others: spaces with a space-level grant or a readable channel), predicate in SQL of the form
  `archived_at IS NULL AND ($all OR id = ANY($visible_spaces))`, then `authorize(space.read)` per
  item (07 "7.4"). Items are `SpaceView` with level and capabilities on the space; a space visible
  only through a channel reports `read` and no capabilities. No pagination (06 "6" rule 6).
- `getSpace`: `SpaceView` with `ETag`; not visible or unknown gives the same `404`; an archived
  space stays readable for those who could read it (with `archived_at`).
- Archive rule (07 "4.5 Archivierte Bereiche und Kanäle"): archiving is final; in an archived space
  every principal, owners included, has at most `read`; writing and triggering actions there end
  with `403 channel_read_only` and `reason: channel_archived`; management changes of an archived
  space (`updateSpace`, `moveSpace`, operator) give `409 conflict`; archiving it again returns `200`
  with the current space and writes nothing; `listSpaces` omits archived spaces. The data stays for
  retention and export.
- `archiveSpace` (`ReasonBody`, Idempotency-Key): sets `archived_at`, bumps `version`, runs with a
  target-based `RightsChange` of M1-29 (`Targets`, 02 8.1; phase hook: `permission_version` first, affected principals are all
  whose effective level in the space was above `read`, `access.changed` with a `changed` entry per
  affected target at level `read`), publishes `SpaceArchived`, appends `space.archived`, audit with
  the reason. The gateway cache applies `space.archived` in log order (02 "11.3", 04 "8.1").
- New spaces are visible to owners and instance-level readers without a new rights version: the
  REST path must not depend on a cached list of spaces (see `Scope` above).

Sidebar (`putMySidebarSpace`, PUT, no If-Match, last write wins per space, 04 "14"):

- Only humans; agents and apps get `403` (`subject_kind_not_allowed`). Not visible space gives
  `404`.
- Body: `collapsed` required; `sort_key` optional and must satisfy `fracindex.Valid`, else `400`;
  `use_global_order = true` removes the personal key; `sort_key` together with
  `use_global_order = true` gives `400`; neither keeps the stored key. Its length is bounded by
  `http.max_json_body_bytes`.
- Upsert of `user_space_states` with `version + 1` and `updated_by_device_id` from the caller's
  device (absent for personal tokens). Response `SidebarState`.
- Event `sidebar.updated` with `audience = self` for the caller, resource
  `{kind: space, id: space_id, version: state version}`, payload `SidebarPayload`.

Events (all durable, no `null`, scope `{space_id}`): `space.created` (create), `space.updated`
(update and move), `space.archived` (archive), payload `SpacePayload`, resource version = space
version.

Rulings to record in `docs/DECISIONS.md`: fractional index scheme and lock for create and move
(the other decisions are in the specification).

## Tests (write them first)

Integration tests use `pgtest` (PostgreSQL 18 throwaway container on `127.0.0.1`, random port)
with real authz, identity, agents and chat. Test names carry the IDs where the specification has
them.

- `TestFracindexVectors` and `TestFracindexProperties` (random pairs: strictly between, valid;
  invalid inputs rejected).
- `TestMoveSpaceChangesExactlyOneRow`: compare all rows before and after a move: only the moved row
  differs (`sort_key`, `version`).
- `TestMoveSpacePositions`: `after_id`, `before_id`, empty body to the start, both fields `400`,
  self `400`, unknown `400`; result order as expected.
- `TestConcurrentMovesKeepDistinctKeys`: parallel creates and moves into the same gap end with
  distinct keys and a total order.
- `TestUpdateSpaceIfMatch`: missing `428`, stale `412` with `current`, fresh `200`.
- `TestSpaceColorAndIconArePaletteKeys`: `#3366ff`, an emoji and an unknown key give `400`; `moss`
  and a key from the generated icon list succeed.
- `TestSECT07_SpaceNameRejectsControlAndBidi`: names with U+202E and U+0007 give `400`;
  `TestSpaceNameLimitAndUniqueness`: over `space.name_max_chars` `400`, `support` after `Support`
  `409`, a name reused after archive succeeds.
- `TestSpaceReadWithReadOnOneChannelOnly`: Sam has `read` on one channel of Support and nothing on
  the space: `getSpace` `200` with level `read`, listed in `listSpaces`. The channel comes from a
  test structure source that wraps the real chat source and adds one channel (no `channels` table
  yet); M1-53 repeats the test with real channels.
- `TestSpaceCreatedVisibleWithoutRightsVersion`: Robin has an instance-level `read` grant and a
  cached snapshot; the owner creates a space; Robin's next `listSpaces` and `getSpace` show it while
  `permission_version` is unchanged.
- `TestSpaceArchiveIsRightsChange`: Alex has `trigger` on Support, Sam `read`; archive with
  step-up: `permission_version` + 1, `access.changed` for Alex and the owner with a `changed` entry
  at level `read` (none for Sam, whose level does not change), `space.archived` in `event_log`,
  `SpaceArchived` published, audit with reason; afterwards `getSpace` still answers for all three,
  `listSpaces` omits the space, `updateSpace` gives `409 conflict`; archive without step-up `401`.
- `TestSpaceOperationsRights`: create and move owner only (`403` for a visible non-owner); update
  with `manage` on the space succeeds, `read` only `403`, no read `404`; owner create needs no
  step-up; every allowed mutation has one audit entry.
- `TestSpaceEvents`: `space.created`, `space.updated` (update, move), `space.archived` and
  `sidebar.updated` have the envelope of 04 "2.2", no `null`, correct resource versions;
  `sidebar.updated` carries `audience = self`.
- `TestSidebarPut`: last write wins across two devices with `updated_by_device_id`;
  `use_global_order` removes the key; both fields `400`; invalid key `400`; agent token `403`;
  invisible space `404`; repeating the same PUT is idempotent in effect.
- `TestListSidebarStatesPort`: returns only the caller's rows of visible, non-archived spaces.
- Every response validates against openapi.yaml in the test build.

Commands that must pass:

```sh
cd services/server && go test -race ./internal/chat/... ./internal/authz/... ./internal/kernel/...
make codegen-check
make api-lint
make migrations-lint
make arch
make check
```

## Acceptance criteria

- [ ] Moving a space changes exactly one row; order is stable and total under concurrency.
- [ ] `updateSpace` requires `If-Match`.
- [ ] Hex values and emoji as colour or icon are rejected; palette keys are accepted.
- [ ] `space.read` holds with `read` on only one channel.
- [ ] `space.archive` is a rights change with `access.changed`; archived spaces are read-only for everyone and missing from `listSpaces` (07 "4.5").
- [ ] Events `space.created`, `space.updated`, `space.archived`, `sidebar.updated` are written as
      specified.
- [ ] Names with control or bidi characters are rejected.
- [ ] New spaces are visible to owners and instance-level readers without a new rights version.
- [ ] Spec changes are in the same change; `make api-lint`, `make api-checks`,
      `make codegen-check`, `make migrations-lint`, `make arch`, `make config-doc-check` are green;
      the seven operations are enabled.
- [ ] `make check` is green.

## Definition of done

Code and tests as above; `internal/chat/README.md` created or updated (ports, tables, actions,
events, rules); doc comment on every public symbol; an ADR only for an architecture decision beyond
the rulings listed; `services/server/CHANGELOG.md` entry under Unreleased; rulings in
`docs/DECISIONS.md`; `make check` green; an independent review and the red team
(`docs/quality/red-team-gate.md`) over the diff with the verdict recorded; merge locally into the
base branch only if both are green, then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then commits,
files with line counts, test results with counts (including the red runs), what is not proven,
follow-ups that could not be done in scope with reason. Then write `{{DONE_PATH}}` with
`exit=0 sha=<sha>` or `exit=1 <reason>`.
