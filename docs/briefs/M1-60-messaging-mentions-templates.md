# M1-60: Mentions and templates

Status: ready | Depends on: M1-56 | Milestone: M1

## Goal

After this ticket a person sees the list of messages that mention them, newest first and only from
channels they may read, and the server offers a port that counts unread mentions per thread for
the counters of the snapshot and the badge. People keep personal templates for frequent inputs,
and holders of `manage` keep templates per channel. Mentions in channels a person cannot read stay
stored but never show up in a list or a count.

## Read first

- `docs/PLAN.md`: section 3 "Way of working per ticket"; section 7 "M1 Foundation" (rows M1-60, M1-61,
  M1-63; "Prerequisites from the core contract": K15).
- `docs/briefs/README.md`.
- `docs/spec/00-glossary.md`: mention, template, read state.
- `docs/spec/01-product.md`: "Feature list" 3.3 (templates) and 3.5 (mention counter, view
  "Mentions").
- `docs/spec/02-architecture.md`: 4.1, 4.3 rows `messaging` (ports `Mentions`, `Templates`) and
  `sync` (reads unread mentions through `Mentions.CountUnread`; no stored counter, R-DM-32), 4.4, "Layers within a module",
  "Transactions and Unit of Work", "Request path".
- `docs/spec/03-data-model.md`: "Principles" (items 7, 10, 12, 13), tables `mentions`,
  `message_templates`, `messages`, `read_states` (computed counters, R-DM-32), 9
  (`template.name_max_chars`, `template.max_per_scope`).
- `docs/spec/05-actions.md`: rules 1.1, 1.2, 1.8; 'Column "Permission"' (`self`, `manage`);
  "Decision"; table 4.7 (`mention.list`, `template.read`, `template.manage`, `message.read`).
- `docs/spec/06-api.md`: "Basic rules", "Authorization in the API", "Errors", "Pagination",
  "Idempotency", "Concurrency" (item 2: templates need `If-Match`), "Endpoints" > "Me" and
  "Channels", "Other points" K15, "Configuration values of this chapter".
- `docs/spec/07-permissions.md`: 4.1 (`not_self`, `subject_kind_not_allowed`), 4.2, 4.3, 7.4,
  "Required tests" (AZ-P5), "Test matrix" (AZ-M48).
- `docs/spec/13-push-notifications.md`: "Badge and sync" (consumer of mention counts).
- `docs/spec/19-security.md`: SEC-IN-1, SEC-IN-4, SEC-LOG-4.
- `docs/spec/22-import-migration.md`: 6.4 (imports write no mentions).
- `docs/api/openapi.yaml`: operations `listMyMentions`, `listTemplates`, `createPersonalTemplate`,
  `createChannelTemplate`, `updateTemplate`, `deleteTemplate`; schemas `MentionItem`,
  `MentionItemPage`, `Template`, `TemplateCreate`, `TemplateUpdate`, `TemplateList`, `Problem`;
  parameters `Cursor`, `Limit`, `IdempotencyKey`, `IfMatchRequired`, `TemplateIdPath`.
- ADRs: `docs/adr/0015-error-format-rfc-9457-idempotency-pagination.md`,
  `0017-one-authorization-function-404-instead-of-403-mandatory-registration.md`.
- Code of earlier tickets: `internal/messaging` (M1-56: tables `messages` and `mentions`,
  representation function), `kernel/authorize` (`Scope`; M1-28), the cursor codec of earlier list
  operations (search for `next_cursor` in `services/server/internal`), the shared display-text
  validator `kernel/textcheck` for names and titles (M1-26), If-Match and ETag helpers (M1-18), the `Idempotency-Key`
  middleware (M1-20), `pgtest` (M1-12), `api/operations.yaml` (M1-17), the configuration schema
  (M1-07) and `tools/configdoc` (M1-08).
- `docs/quality/gates.md`, `CONTRIBUTING.md`, `docs/quality/red-team-gate.md`,
  `services/server/internal/messaging/README.md`.
- Tables: `mentions`, `messages` (read), `message_templates` (new). Actions: `mention.list`,
  `template.read`, `template.manage`, `message.read`.

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
10. The own mention list is filtered by `message.read`: SQL predicate on visible channels plus
    `authorize` per item (05 `mention.list`; 07 7.4).
11. A mention of a principal without read permission is stored but never listed, counted or
    notified (03 `mentions`; AZ-M48).
12. Mention counters include no channel without read permission (PLAN M1-60; AZ-P5).
13. Personal templates have no channel (`owner_user_id` set, `channel_id` empty) and are checked
    against the resource `user` with `self`; channel templates are checked against the resource
    `channel`, reading with `read`, changing with `manage` (03 `message_templates`; 05
    `template.read`, `template.manage`; 06 K15).
14. Template changes need `If-Match` (06 "Concurrency" item 2); POST and PATCH accept
    `Idempotency-Key` (06 "Idempotency").
15. Template names are display texts: control and Bidi control characters are rejected (SEC-IN-4).
16. Lists without pagination stay small (06 "Pagination" item 6): template counts are bounded by a
    named key (note 9).
17. No content, template names or display names in logs or telemetry (SEC-LOG-4).

## Scope

All paths below `services/server/` unless stated.

1. Migration `migrations/<next number>_message_templates.sql`: table `message_templates` exactly as
   03 (columns, check "exactly one of `owner_user_id` and `channel_id`", FKs with ON DELETE
   CASCADE, partial indexes on `owner_user_id` and `channel_id` `WHERE deleted_at IS NULL`). Use
   the next free goose number at your local merge; renumber before merging if a parallel ticket
   took it.
2. Port `internal/messaging`:
    - `Mentions` with `ListMine(ctx, q MentionListQuery) (MentionPage, error)` and
      `CountUnread(ctx, q UnreadMentionQuery) ([]ThreadMentionCount, error)` where
      `UnreadMentionQuery{PrincipalID ids.ID; VisibleChannelIDs []ids.ID; LastReadSeq
      map[ids.ID]int64}` and `ThreadMentionCount{ThreadID, ChannelID ids.ID; Count int}`.
    - `Templates` with `List(ctx, channelID ids.ID) ([]TemplateView, error)`,
      `CreatePersonal(ctx, c TemplateInput) (TemplateView, error)`,
      `CreateForChannel(ctx, channelID ids.ID, c TemplateInput) (TemplateView, error)`,
      `Update(ctx, c TemplateChange) (TemplateView, error)`, `Delete(ctx, templateID ids.ID) error`,
      `ListMine(ctx) ([]TemplateView, error)`.
    `ids.ID` is the UUID type of `kernel/ids`; the actor comes from the context.
3. `internal/messaging/domain`: template rules (name and content validation, no-op detection,
   scope) and the mention page keyset.
4. `internal/messaging/app`: use cases for the seven operations and `CountUnread`.
5. `internal/messaging/adapters/postgres/queries/mentions.sql`, `templates.sql`, regenerated
   `sqlcgen/`.
6. `internal/messaging/adapters/httpapi`: `ListMyMentions`, `ListMyTemplates`, `ListTemplates`,
   `CreatePersonalTemplate`, `CreateChannelTemplate`, `UpdateTemplate`, `DeleteTemplate`.
7. Configuration: `template.name_max_chars` and `template.max_per_scope` are defined in 03 9; add
   both to the configuration schema (M1-07) and keep `docs/operations/configuration.md` current
   (`make config-doc-check`).
8. `api/operations.yaml`: enable the seven operations; regenerate.
9. Docs: `internal/messaging/README.md` (English, existing structure, including who consumes
   `CountUnread`), `services/server/CHANGELOG.md`, `docs/DECISIONS.md` entries for notes 6, 8, 9
   and 12.

## Out of scope

- Creating and resolving mentions when sending (M1-56) or editing (M1-58).
- The read state and the counters in read-state responses, events and the snapshot (M1-61,
  M1-63). No counter is stored (03 `read_states`, R-DM-32); they compute it with `CountUnread`.
- Push and badge (M3); client views (M4); export of personal templates and their deletion with an
  account (M7, `personaldata.Participant`).

## Boundaries

- A new migration needs its line in `migrations.sha256`: run `make migrations-lock` after creating
  the file and before `make check`; never edit a merged migration (M1-01, ADR 0007).
- Work in a dedicated git worktree on the branch `feat/M1-60-messaging-mentions-templates`, created
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

    | operationId | Route | Action | Resource | Class | Headers |
    |---|---|---|---|---|---|
    | `listMyMentions` | `GET /api/v1/me/mentions` | `mention.list` | user = caller | `rl.read` | |
    | `listTemplates` | `GET /api/v1/channels/{channel_id}/templates` | `template.read` | channel | `rl.read` | |
    | `listMyTemplates` | `GET /api/v1/me/templates` | `template.read` | user = caller | `rl.read` | |
    | `createPersonalTemplate` | `POST /api/v1/me/templates` | `template.manage` | user = caller | `rl.write` | `Idempotency-Key` |
    | `createChannelTemplate` | `POST /api/v1/channels/{channel_id}/templates` | `template.manage` | channel | `rl.write` | `Idempotency-Key` |
    | `updateTemplate` | `PATCH /api/v1/templates/{template_id}` | `template.manage` | user (personal) or channel | `rl.write` | `If-Match` required, `Idempotency-Key` |
    | `deleteTemplate` | `DELETE /api/v1/templates/{template_id}` | `template.manage` | user (personal) or channel | `rl.write` | |

    Agents and apps are not users: `mention.list` and personal templates are decided by
    `authorize` for them; the expected decision is `403 forbidden` with reason
    `subject_kind_not_allowed` (07 4.1). Do not add a permission check outside `authz`.

2. `listMyMentions`: `authorize(mention.list, user = caller)`; visible channel IDs for
   `message.read` from `Authorizer.Scope`; query `mentions mn JOIN messages m ON m.id =
   mn.message_id WHERE mn.principal_id = @caller AND m.channel_id = ANY(@visible) AND m.deleted_at
   IS NULL`, keyset `(mn.created_at DESC, mn.message_id DESC)`, `LIMIT L + 1` with `L` from
   `api.page_size_default` and `api.page_size_max` (index `(principal_id, created_at DESC)`). Each
   item passes `authorize(message.read)` in filter mode `authorize.Filter(ctx)` (M1-28; no denial audit, 07 4.2); a denied item is dropped, logged with only action and
   resource kind and counted in `bothy.authz.errors` with `bothy.reason = filter_mismatch` (07 7.4
   rule 3). Items `MentionItem{message,
   created_at}` with the REST representation of M1-56. `next_cursor` only when more exist.
3. Cursor: the codec of `internal/platform/httpserver` (M1-18, 06 "Pagination"), bound to the
   list name and the caller. A cursor of another list, of another principal, or a tampered one
   gives `400 cursor_invalid`.
4. `CountUnread`: for one principal, the number of mentions per thread with `m.thread_seq >
   LastReadSeq[thread]` (0 when the thread is not in the map), `m.deleted_at IS NULL`,
   `m.channel_id = ANY(VisibleChannelIDs)`; threads with count 0 are omitted. One statement; pass
   the map as two parallel arrays and `unnest` them. An empty `VisibleChannelIDs` returns nothing.
   This port has no HTTP route; `sync` uses it in M1-61 and M1-63 to compute the unread mention
   counters of read states (`ReadState.mention_count` in responses and events) and the snapshot.
5. Counters and lists never reveal invisible channels: the SQL predicate is mandatory and the
   result contains only visible threads (AZ-P5 part mentions).
6. `threads.unread_floor_seq` is not needed for mentions: imports write no mentions (22 6.4), so a
   floor of `last_read_seq` or 0 is exact. Record this in DECISIONS.md.
7. `listTemplates`: `authorize(template.read, channel)` (missing or invisible channel `404`). Items:
   the caller's personal templates (only when the caller is a user) plus the channel's templates,
   both without `deleted_at`, ordered by `lower(name)`, then `id`. One statement with
   `(t.owner_user_id = @caller) OR (t.channel_id = @channel_id AND t.channel_id =
   ANY(@visible))`. Personal items pass `authorize(template.read, user = caller)`, channel items are
   covered by the channel check. `Template{id, scope, channel_id (channel scope only), name,
   content_md, version}`.
   `listMyTemplates`: `authorize(template.read, user = caller)`; the caller's personal templates
   without `deleted_at`, ordered by `lower(name)`, then `id`; unpaginated, bounded by
   `template.max_per_scope` (`TemplateList`). Reason: a person without any readable channel can
   still list their own templates.
8. Create: body `TemplateCreate{name, content_md}`. Validation (`400 validation_failed`, pointer and
   code per field): `name` not empty after trimming (`required`), at most `template.name_max_chars`
   code points (`too_long`), no control or Bidi control characters through the shared display-text
   validator (`invalid_characters`); `content_md` not empty after trimming (`required`) and at most
   `message.max_content_bytes` bytes (`too_long`), because a longer template could never be sent.
   Stored as given (no trimming of the stored value). `created_by` = caller, `version = 1`.
   `createPersonalTemplate` sets `owner_user_id` = caller; `createChannelTemplate` needs `manage`
   on the channel (read without `manage` gives `403`, no read `404`) and sets `channel_id`. `201`
   with the `Template`.
9. Count bound: before inserting, take `pg_advisory_xact_lock` on a key derived from the scope
   (`personal:<user id>` or `channel:<channel id>`), count the scope's templates without
   `deleted_at`, and refuse with `400 validation_failed` (pointer empty, code `limit_reached`,
   detail names `template.max_per_scope`) when the bound is reached. The lock makes the bound exact
   under concurrency. With both bounds, `listTemplates` returns at most two scopes' worth of items.
10. `updateTemplate`, precedence `428` > `404` > `403` > `412` > `400`: missing `If-Match` gives
    `428 precondition_required` before any lookup; load the template with `deleted_at IS NULL` and
    `(owner_user_id = @caller OR channel_id = ANY(@visible))`; a missing, deleted or foreign
    personal template and one in an invisible channel give `404` through one code path;
    `authorize(template.manage, user = owner)` or `authorize(template.manage, channel)`; in the
    unit of work lock the row, compare the version (`412 version_conflict` with `current`),
    validate given fields like note 8; no change (also an empty body) returns `200` with the current
    template and no write; otherwise update and raise `version`. `200` with `ETag: "<version>"`.
11. `deleteTemplate`: load like note 10 but including rows with `deleted_at` set; a template that
    was never visible gives `404`; `authorize(template.manage, ...)`; an already deleted one gives
    `204` without write; otherwise set `deleted_at` and raise `version`. `204`. Owners have no
    permission on other people's personal templates (the action has no `owner` permission).
12. Templates have no event type in 05 section 6: no durable event is appended. Clients reload
    templates by REST. Record this in DECISIONS.md.
13. Read-only channels: `template.read` and `template.manage` with `manage` stay allowed (05 rule
    1.8).
14. Every `SELECT`, `UPDATE` or `DELETE` on `mentions` or `message_templates` carries
    `channel_id = ANY(...)` (for mentions via the join to `messages`); keep
    `TestQueriesCarryVisibleChannelPredicate` (M1-57) green when it is on your base.

## Tests (write them first)

Unit:

- `TestTemplateValidation`: empty and blank name, name at and above `template.name_max_chars`, name
  with U+202E, name with U+0007, empty content, content above `message.max_content_bytes`.
- `TestTemplateNoop`: same values and empty body are no-ops.
- `TestListMyTemplates`: only the caller's personal templates, also without any readable channel;
  an agent gets `403`.

Integration against PostgreSQL 18 (`pgtest`, world of 07 "Examples", HTTP through the real chain):

- `TestListMyMentions_NewestFirstAndPaging`: 7 mentions of Alex, `limit=3`, three pages, newest
  first, no duplicates, last page without `next_cursor`.
- `TestAZM48_MentionInInvisibleChannelStoredNotListed`: a Marketing message mentions Alex; the row
  exists in `mentions`, but neither `listMyMentions` nor `CountUnread` show it.
- `TestListMyMentions_ExcludesTombstones` (tombstone created by a test helper if M1-58 is not on the
  base).
- `TestListMyMentions_CursorInvalid`: a cursor from another list, from another principal and a
  tampered one each give `400 cursor_invalid`.
- `TestListMyMentions_AgentGets403`.
- `TestListMyMentions_DropsItemDeniedByAuthorize`: app-level test with a fake `Authorizer` that
  denies one item; the item is dropped, logged without IDs and counted with `bothy.reason =
  filter_mismatch`.
- `TestCountUnread_FloorsAndDeleted`: counts respect `LastReadSeq`, default floor 0, ignore
  tombstones, omit zero counts.
- `TestAZP5_CountUnreadRandomWorlds`: seeded random worlds (fixed seed list, `math/rand/v2`) with
  grants, mentions and read states; `CountUnread` equals a naive reference that filters by
  `authorize(message.read)` per message, and never returns a thread of an invisible channel.
- `TestCreatePersonalTemplate`: `201`, scope `personal`, no `channel_id`, `version = 1`;
  `Idempotency-Key` replay returns the stored response and creates one row; an agent gets `403`.
- `TestCreateChannelTemplate_Rights`: Owner `201`; a reader without `manage` `403`; Alex on a
  Marketing channel `404` equal to a random UUIDv7; a DM participant `403`.
- `TestTemplateLimitExactUnderConcurrency`: with `template.max_per_scope` set small in the test
  configuration, parallel creates stop exactly at the bound; the next one gets `limit_reached`.
- `TestListTemplates`: own personal plus channel templates; never another user's personal ones;
  deleted excluded; order by `lower(name)`, then `id`; invisible channel `404`.
- `TestUpdateTemplate`: `428` without `If-Match`; `412` with `current`; `200` raises `version`
  and returns the `ETag`; another user's personal template `404` (also for the Owner); a channel
  template without `manage` `403`; a deleted template `404`; no-op keeps `version`.
- `TestDeleteTemplate`: `204`; second call `204`; another user's personal template `404`; a channel
  template without `manage` `403`.
- `TestTemplates_NoEvents`: no `event_log` row is written by any template operation.
- `TestTemplates_ReadOnlyChannelAllowsManage`.

Commands that must pass:

- `cd services/server && go test -race ./internal/messaging/...`
- `make codegen-check`, `make migrations-lint`, `make arch`, `make api-lint`,
  `make config-doc-check`, `make docs`
- `make check` (with the local SonarQube; if unreachable, `Verdict: BLOCKED`, never `SONAR_SKIP`).

## Acceptance criteria

- [ ] `listMyMentions` returns only mentions in readable channels, newest first, with a working and
      bound cursor; invisible mentions stay stored.
- [ ] `CountUnread` counts per thread above the given read sequence, ignores tombstones and never
      includes an invisible channel (randomised test green).
- [ ] Personal templates have no channel and are reachable only by their owner; channel templates
      need `read` to list and `manage` to change.
- [ ] Template changes need `If-Match` (`428`, `412` with `current`); creates honour
      `Idempotency-Key`; deletes are idempotent.
- [ ] Names and contents are validated; counts are bounded exactly by `template.max_per_scope`; the
      new keys are in chapter 06, the schema and the generated reference.
- [ ] No event is written for templates; README, doc comments, CHANGELOG and DECISIONS.md entries
      are present.
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
new test), what is not proven, and follow-ups that could not be done in scope with the reason. Then
write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
