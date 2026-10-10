# M1-49: Who has access, preview and view as

Status: ready | Depends on: M1-48 | Etappe: M1

## Goal

After this ticket the module `authz` answers the three questions an owner or a person with `manage`
asks around a rights change: who can access a space or channel and why (`access.explain`), what a
grant would change before it exists (`access.preview`, a dry run in the real transaction with a
plain-text explanation from fixed templates in German and English), and what one person sees
(`access.view_as`, read only, time limited, audited). All three are computed by the same snapshot
code that `authorize` uses, so they cannot disagree with a real decision. They feed the console
screens A2 to A4 (M6) and carry hint H1 ("Lesehorizont") wherever the specification asks for it.

## Read first

- `docs/PLAN.md`, section 7 "M1 Fundament": row M1-49, "Voraussetzungen aus dem Kern-Vertrag"
  (row `access.preview` ... K-RE-7 to K-RE-9), "Teststrategie", "Red Team"; section 3
  "Arbeitsweise je Ticket". `docs/briefs/README.md`.
- `docs/spec/07-rechte.md`: "3. Prinzipal-Kontext" (field `view_as`), "4.1 Entscheidungen und
  Gründe", "4.2 Ablauf" (steps 4 and 7), "4.3 Sichtbarkeit", "4.4 Invarianten in der
  Transaktion", "SR2 Werkzeugklasse", "SR6 Direktnachrichten nur für Teilnehmer", "6. Lesehorizont
  von Agenten" (H(A), T(A), H1), "7.2 Rechte-Stand (Snapshot)" (provenance), "9.1 Wer hat
  Zugriff?", "9.2 Als Person ansehen", "9.3 Vorschau in Klartext", "10. Beispiele", "11.1
  Testmatrix" (world, AZ-M21, AZ-M31, AZ-M37, AZ-M38, AZ-M46, AZ-M47), "11.2 Pflicht-Tests" (AZ-P8),
  "12. Entscheidungen" (R-RE-07, R-RE-10), "14. Konfigurationswerte".
- `docs/spec/05-aktionen.md`: "1. Verbindliche Regeln" (rules 1.6, 1.7), "2.3 Wirksame Stufe",
  "2.4" (columns), "4.2 Instanz und Verwaltung" (rows `access.explain`, `access.preview`,
  `access.view_as`), "4.4 Gruppen und Freigaben" (`grant.create`), "8" (R-AZ-05, R-AZ-13).
- `docs/spec/03-datenmodell.md`: tables `grants`, `groups`, `group_memberships`, `owners`,
  `principals`, `users` (`locale`, `time_zone`), `agents` (`billing_mode`, `credential_holder_id`,
  `tool_class`, `reported_tool_class`, `reported_details`); "6.3 Sperrreihenfolge", "6.4
  I1-Prüfung".
- `docs/spec/10-backend-richtlinie.md`: "5.3 Abfrage" (triggerer query), "11. Geführter Ablauf"
  (`i1_conflicts`, `organization_ready`).
- `docs/spec/04-sync-protokoll.md`: "8.2 Was beim Ändern passiert" (diff format).
- `docs/spec/02-architektur.md`: "4.1 Regeln" (source ports), "4.3 Ports, Ereignisse und Quellen je
  Modul", "4.4 Erlaubte Abhängigkeiten", "5. Schichten im Modul", "8.1 Ablauf" (unit of work and
  phase hooks), "9.3 Nachweis der Prüfung".
- `docs/spec/01-produkt.md`: A2, A3, A4 wireframes; "9. Sprachen" (language follows `users.locale`).
- `docs/spec/06-api.md`: "5. Fehler", "7. Idempotenz" (`previewGrant` has no `Idempotency-Key`),
  "16. Endpunkte" group "Access", "13. Versionierung" (additive changes).
- `docs/api/openapi.yaml`: operations `explainSpaceAccess`, `explainChannelAccess`, `previewGrant`,
  `viewAsUser`, `createGrant`; schemas `AccessExplanation`, `AccessEntry`, `AccessSource`,
  `AccessTargetKind`, `GrantCreate`, `GrantPreview`, `AccessEffect`, `InvariantViolation`,
  `ViewAsSnapshot`, `SpaceView`, `ChannelView`, `Space`, `Channel`, `PrincipalRef`, `Level`,
  `LevelOrNone`.
- ADR 0017, ADR 0018; `docs/security/threat-model.md` row T-OW-01.
- `docs/operations/configuration.md`: `authz.view_as_max_duration`, `auth.step_up_max_age`.
- Code already merged: `services/server/internal/authz` (M1-27 to M1-30, M1-48: snapshot,
  `authorize`, cache, `Scope`, phase hook with `VerifyInvariants`, admin port, the fake-source
  harness and the random-world generator of AZ-P8), `internal/kernel/authorize`,
  `internal/kernel/actor`, `internal/kernel/txn`, `internal/platform/pg/pgtest`,
  `services/server/api/operations.yaml`.
- `docs/quality/gates.md`, `CONTRIBUTING.md`, `docs/quality/red-team-gate.md`.

## Fixed requirements that apply

1. No MVP. Built from scratch directly in its final state; etappen are only the build order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined
   interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface has a docstring or doc
   comment (purpose, parameters, return value, errors, example where useful); comments explain why,
   not what. API documentation is generated from code (OpenAPI from the routes, dartdoc, Python API
   docs in the docs site). In this repository `docs/api/openapi.yaml` is normative and the server
   interface is generated from it (ADR 0013); the route test keeps both identical.
4. Documentation in the repo: README per module/service, an ADR for every architecture decision, a
   CHANGELOG entry per release, docs site kept current. A change without matching documentation is
   not done.
5. Quality gates (SonarQube, linters, type checks, security scanners, test coverage) block the merge.
6. Test first: every behaviour gets a test that fails before the code exists (red), then passes
   (green). Keep the red output for the report.
7. No en dash or em dash characters (U+2013, U+2014) in any file or commit message.
8. White label: no company, person, host or domain names except the examples Alex, Sam, Robin,
   the agents from 07 "10. Beispiele" and `example.com`; no paths of a real machine.
9. Limits and durations only through named keys from `docs/operations/configuration.md`; no bare
   numbers in production code or documentation.
10. Specification first: every API change below lands in `docs/api/openapi.yaml` and the chapter in
    the same change, before the code that implements it (PLAN principle 3).
11. Product requirements implemented here: "Wer hat Zugriff?" with provenance per principal
    (07 "9.1"); preview of every rights change as a dry run in the real transaction with rollback,
    I1 and SR8 results and hints in plain text from templates (07 "9.3", R-RE-10); "Als Person
    ansehen" only for owners with step-up, intersection of rights, DM channels hidden, every write
    `view_as_read_only`, ends after `authz.view_as_max_duration`, audited (07 "9.2", SR6, AZ-M46,
    AZ-M47); hint H1 in preview and explanation (07 "6").

## Scope

Specification changes (do these first, in the same change):

- `docs/api/openapi.yaml`, additive only:
  - `GrantPreview`: add required `hints` (array of `PreviewHint`), required `text` (string, lines
    separated by `\n`, no trailing newline), required `text_locale` (enum `de`, `en`), optional
    `denial_reason` (string; a reason code from 07 "4.1").
  - new schema `PreviewHint`: required `code` (enum `read_horizon`, `trigger_blocked_tool_class`,
    `group_reach`, `expiry_missing`); optional `agent_id`, `principal_id`, `group_id`, `space_id`,
    `channel_id` (all `Uuid`), `count` (integer, int64).
  - `InvariantViolation`: add optional `credential_holder_id` (`Uuid`) and `organization_ready`
    (boolean; true when `organization` is in the agent's reported `billing_modes_supported`).
  - `AccessEntry`: `level` becomes `LevelOrNone` (`none` only for entries whose sole sources are
    `via_agent`); add optional `inactive` (boolean, true when the principal status is not `active`;
    such entries show the level their grants would give, as 07 "9.1" shows them struck through).
  - `AccessSource.kind`: add `operator`, `via_agent`, `guard_blocked`; add optional `agent_id`
    (`Uuid`), `rule` (enum `sr2`, `sr3`), `expires_at` (`Timestamp`, of the grant),
    `membership_expires_at` (`Timestamp`), `reason` (string, the grant reason).
  - `ViewAsSnapshot`: add required `expires_at` (`Timestamp`).
  - Descriptions of the four operations state the rules of this brief in one or two sentences.
- `docs/spec/07-rechte.md`: "9.1" names the new source kinds and fields; "9.3" names `hints`,
  `text`, `text_locale`, `denial_reason`, the language rule (below) and the location of the message
  catalog; the example block in "9.3" shows line 4 unwrapped exactly as the golden file below.
  "9.2" states how the API realises view-as in M1 (snapshot operation plus `actor.ViewAs`).
- Run the API gates (`make api-lint`, `make api-checks`) and regenerate the server code with the
  code generation target added by M1-17 so that `make codegen-check` shows no diff.

Code (paths under `services/server/`):

- `internal/kernel/actor`: add `ViewAs *ViewAs` to the principal context with
  `type ViewAs struct { UserID ids.ID; StartedAt time.Time }` (doc comment: set only by `authz`
  for an owner session; never parsed from client input in M1).
- `internal/authz` (port): extend the `Admin` port with
  `ExplainAccess(ctx, target AccessTargetRef) (AccessExplanation, error)`,
  `PreviewGrant(ctx, in GrantCreateInput) (GrantPreview, error)`,
  `ViewAs(ctx, userID ids.ID) (ViewAsSnapshot, error)`; port DTOs mirror the openapi schemas.
  Extend the declared source ports where the base branch lacks a capability (keep names of the
  base; these are the needed capabilities):
  - `PrincipalSource`: describe principals by ID: kind, handle, display name, status, and for
    humans `locale` and `time_zone`.
  - `StructureSource`: names of spaces and channels; channels of a space; effective operator per
    channel and default operator per space; the targets an agent serves (07 "6", 10 "5.2"); the
    data needed to fill the openapi `Space` and `Channel` objects for `ViewAsSnapshot`.
  - `AgentPolicySource`: per agent status, `billing_mode`, `credential_holder_id`, effective tool
    class (higher of `tool_class` and `reported_tool_class`), `billing_modes_supported`.
  Update every fake in the authz test harness and the existing real implementations (identity for
  `PrincipalSource`; chat and agents only if their adapters exist in the base branch) in this
  change. A shared contract test per source port runs against fake and real implementations.
- `internal/authz/domain`: provenance mode of the snapshot builder (`explain.go`), read horizon and
  trigger set (`horizon.go`: `H(A)`, `T(A)` minus principals blocked by SR2, H1 pairs), preview
  diff to `AccessEffect` (`preview.go`), plain-text renderer `internal/authz/domain/previewtext`
  with the catalog files `catalog_de.go` and `catalog_en.go` (stdlib `text/template` or plain
  formatting; no third-party library in domain).
- `internal/authz/app`: use cases `explain.go`, `preview.go`, `viewas.go`; `authorize` honours
  `ViewAs` (07 "4.2" step 4 intersection, step 7 `view_as_read_only`, expiry rule below) if the
  base does not already.
- `internal/authz/adapters/postgres`: queries for explanation candidates (grants by target on the
  path, valid group members, owners) and for the trigger set of any agent (generalise the I1
  triggerer query of 10 "5.3"; no copy).
- `internal/authz/adapters/httpapi`: handlers `explainSpaceAccess`, `explainChannelAccess`,
  `previewGrant`, `viewAsUser`.
- `internal/wire`: wiring; `services/server/api/operations.yaml`: move the four operations from
  `later` to the enabled list.
- Configuration: register `authz.view_as_max_duration` (duration) in the typed `authz` config and
  the config schema (M1-07); the chapter row in 07 "14" exists.
- Docs: `internal/authz/README.md` (ports, new operations, rules), `services/server/CHANGELOG.md`
  under Unreleased, rulings in `docs/DECISIONS.md` (list in Design notes, "Rulings to record").

## Out of scope

- Gruppen and grants API itself (M1-48, merged), expiry jobs (M1-30), `resolve_i1` (not in the
  `GrantCreate` schema; preview of it waits for the ticket that adds it).
- Previews of other actions with "Rechte" = `ja` (07 "9.3": they come with their routes).
- The console screens, the "Rechte-Hinweise" list and console browsing of content as another
  person (M6). The transport of a view-as context on ordinary requests is not specified yet (see
  spec gap); in M1 only `authz` sets `actor.ViewAs`, inside `viewAsUser` and in tests.
- SR3 data (dev environments, V2; project bindings, M2): `rule = sr3` is defined in the schema but
  emitted only once those exist.
- Spaces, channels and agents tables and their APIs (M1-51 to M1-53).

## Boundaries

- Work in a dedicated git worktree on branch `feat/M1-49-authz-explain`.
- Conventional Commits with scope (`feat(authz): ...`, `docs(api): ...`) and DCO sign-off
  (`git commit -s`).
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope above except the CHANGELOG and docs entries named here.
- Follow the host rules supplied by the launcher.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`; remove
  them at the end.
- Never read or print secrets. No network calls except package downloads for pinned tools.

## Design notes

Same code as `authorize`:

- Explanation, preview diff and view-as all use the snapshot builder of M1-28 (07 "7.2"). The
  explanation runs it in provenance mode (provenance is never cached). A differential test proves
  that the level and capabilities in every `AccessEntry` equal what `authorize` derives.

`access.explain` (`explainSpaceAccess`, `explainChannelAccess`):

- Authorization: `authorize(p, access.explain, space|channel)`: no read gives 404 (same body as an
  unknown ID), read without `owner` or `manage` gives 403 `forbidden` (`capability_missing`). No
  step-up, no audit, class `rl.read`.
- Candidates: active and inactive owners, subjects of valid and not valid grants on the target's
  path (instance, space, and for a space target its channels are not included), valid members of
  those groups, the effective operator of a channel target and the default operator of a space
  target. For each candidate compute effective level and capabilities on the target with
  provenance. Entries with level `none` are dropped unless they carry a `via_agent` source.
- Sources: `owner`; `direct` (grant on the target itself, with `grant_id`, `expires_at`,
  `reason`); `group` (with `group_id`, `grant_id`, `membership_expires_at`, `expires_at`);
  `inherited` (with `from_target_kind`, `from_target_id`, `grant_id`; combined with `group` data
  when the inherited grant is a group grant); `operator` (agent is effective operator of the
  channel, or default operator of the space; level `write` per R-AZ-13); `dm_participant` (DM
  channels; owners never see them, SR6); `via_agent` (H1: principal P is in `T(A)` of an agent A
  whose horizon `H(A)` contains the channel, and P cannot read it; for a space target: any channel
  of the space); `guard_blocked` (P has `trigger` but SR2 blocks triggering the channel's operator
  A: `rule = sr2`, `agent_id = A`).
- `inactive = true` for principals with status other than `active` (their grants do not act, SR7;
  the entry shows the level the grants would give).
- Ordering: owners first, then by display name, then by ID. No pagination (small sets, 06 "6" rule
  6).
- `H(A)`: channels A can read (grants of A plus operator rule, including channels with a thread
  whose fixed operator is A). `T(A)`: active humans with effective `trigger` on any target A serves
  (10 "5.2"), minus those SR2 blocks (A's effective tool class is `host` and P is not an owner).

`access.preview` (`previewGrant`):

- Authorization: `authorize(p, access.preview, target of the grant)` with the requirement of
  `grant.create` (owner, or `manage` on the target), never step-up, never audit, class `rl.admin`.
  No read on the target gives 404, read without `manage` gives 403. The limits of 07 "2.6"
  (`delegation_exceeds_own_level`, `delegation_exceeds_own_capabilities`, subject agent or app by a
  non-owner or agent and app with `trigger` or capabilities other than `upload`:
  `subject_kind_not_allowed`, instance target by a non-owner: `owner_only`) do not fail the request:
  the response is `200` with `allowed = false` and `denial_reason`.
- Dry run in the real transaction (R-RE-10): structure the `createGrant` use case of M1-48 into an
  authorization step and an apply step. The preview opens the unit of work with the same
  `RightsChange` option as `createGrant` (phase 1 locks `permission_version` and captures the
  before state), runs the apply step (insert, events and audit records stay buffered), then calls
  the same evaluator the authz pre-commit hook uses (diff of visible targets, I1, SR8) directly,
  stores the result and returns a package-private sentinel error so the unit of work rolls back.
  The sync and audit hooks never run, so no `event_log` row, no `pg_notify`, no audit row and no
  counter change survive. The handler maps the sentinel to the preview result. Do not add a
  dry-run mode to `kernel/txn`.
- `effects`: one `AccessEffect` per affected principal and target from the diff (same diff as
  `access.changed`, 04 "8.2"), `before_level` and `after_level` as `LevelOrNone`,
  `capabilities_added`.
- `invariant_violations`: per violating personal agent `code = i1_violation`, `agent_id`,
  `principal_ids` (the extra triggerers), `credential_holder_id`, `organization_ready`, `hint`
  ("Switch the agent to organization first"). `allowed = false` if any violation or a denial
  reason exists.
- `hints` (07 "9.3"): `trigger_blocked_tool_class` per operator agent of the target with effective
  class `host` when the subject is not an owner (for group subjects always); `read_horizon` per
  (agent A served by the target, channel K in `H(A)`, principal gaining `trigger` who cannot read K
  after the change); `group_reach` for group subjects with `count` = active members with a valid
  membership; `expiry_missing` for subject kind `user` without `expires_at`.

Plain-text rendering (`text`):

- Language: primary subtag of the caller's `users.locale` (01 "9. Sprachen"); `de` or `en`,
  anything else falls back to `de` (product default). `text_locale` names the language used.
- Dates: the instant `expires_at` formatted in the caller's `users.time_zone` (invalid zone: UTC);
  `de` uses `02.01.2006`, `en` uses `2006-01-02` (Go layouts).
- Names: principal display name; group name; `<space>/<channel>` for channels.
- Lines in this order: summary; abilities; capabilities; then one line per hint in the order
  denial, I1 violations (by agent name), tool class (by agent name), read horizon (by agent name,
  space name, channel name), group reach, missing expiry.
- Abilities by level: `read` = ability.read; `write` = ability.write; `trigger` = ability.write
  followed by `, ` plus ability.trigger_agent for each active operator agent of the target (channel:
  its effective operator; space: default operator and operators of its channels; instance: all
  operators), sorted by display name, excluding agents with an I1 violation and agents blocked by
  SR2. Agents and apps as subjects never get the trigger part.
- Capabilities line: always upload and approve lines; ` ` plus caps.export and ` ` plus caps.manage
  appended only when granted.
- Catalog (placeholders in braces; quotes are ASCII):

| Key | de | en |
|---|---|---|
| `level.read` / `.write` / `.trigger` | `Lesen` / `Schreiben` / `Auslösen` | `Read` / `Write` / `Trigger` |
| `subject.group` | `Gruppe {group}` | `Group {group}` |
| `where.instance` | `in der ganzen Instanz` | `in the whole instance` |
| `where.space` | `im Bereich {space}` | `in space {space}` |
| `where.channel` | `im Kanal {space}/{channel}` | `in channel {space}/{channel}` |
| `summary` | `{subject} erhält{until} die Stufe "{level}" {where}.` | `{subject} gets the level "{level}" {where}{until}.` |
| `until` | ` bis {date}` | ` until {date}` |
| `abilities.single` | `{subject} kann damit: {abilities}.` | `{subject} can then: {abilities}.` |
| `abilities.group` | `Mitglieder von {group} können damit: {abilities}.` | `Members of {group} can then: {abilities}.` |
| `ability.read` | `Nachrichten lesen` | `read messages` |
| `ability.write` | `Nachrichten lesen und schreiben, Threads anlegen` | `read and write messages, create threads` |
| `ability.trigger_agent` | `Agent {agent} auslösen` | `trigger agent {agent}` |
| `caps` | `Dateien hochladen: {upload}. Freigabe-Karten bestätigen: {approve}.` | `Upload files: {upload}. Confirm approval cards: {approve}.` |
| `caps.export` / `caps.manage` | `Exportieren: ja.` / `Verwalten: ja.` | `Export: yes.` / `Manage: yes.` |
| `yes` / `no` | `ja` / `nein` | `yes` / `no` |
| `denial.delegation_exceeds_own_level` | `Abgelehnt: Du kannst höchstens deine eigene Stufe weitergeben.` | `Rejected: you can grant at most your own level.` |
| `denial.delegation_exceeds_own_capabilities` | `Abgelehnt: Du kannst nur eigene Zusatzrechte weitergeben.` | `Rejected: you can only grant additional rights you have yourself.` |
| `denial.subject_kind_not_allowed` | `Abgelehnt: Agenten und Apps bekommen Freigaben nur vom Owner, höchstens Schreiben und nur Dateien hochladen.` | `Rejected: agents and apps get grants only from an owner, at most Write and only Upload files.` |
| `denial.owner_only` | `Abgelehnt: Freigaben auf der ganzen Instanz legt nur ein Owner an.` | `Rejected: only an owner grants on the whole instance.` |
| `hint.i1` | `Abgelehnt: {agent} läuft auf dem persönlichen Abo von {holder}. Zuerst auf Organisation umstellen.` | `Rejected: {agent} runs on the personal subscription of {holder}. Switch it to organization first.` |
| `hint.tool_class` | `Hinweis: {agent} hat die Werkzeugklasse Host; {subject} kann {agent} damit nicht auslösen (nur Owner).` | `Note: {agent} has the tool class host; {subject} cannot trigger {agent} (owners only).` |
| `hint.read_horizon` | `Hinweis: {agent} liest auch {space}/{channel}. {subject} darf {space}/{channel} nicht lesen, kann aber über {agent} Inhalte daraus erfahren.` | `Note: {agent} also reads {space}/{channel}. {subject} may not read {space}/{channel} but can learn its contents through {agent}.` |
| `hint.read_horizon.group` (count > 1) | `Hinweis: {agent} liest auch {space}/{channel}. {count} Mitglieder von {group} dürfen {space}/{channel} nicht lesen, können aber über {agent} Inhalte daraus erfahren.` | `Note: {agent} also reads {space}/{channel}. {count} members of {group} may not read {space}/{channel} but can learn its contents through {agent}.` |
| `hint.read_horizon.group_one` | `Hinweis: {agent} liest auch {space}/{channel}. 1 Mitglied von {group} darf {space}/{channel} nicht lesen, kann aber über {agent} Inhalte daraus erfahren.` | `Note: {agent} also reads {space}/{channel}. 1 member of {group} may not read {space}/{channel} but can learn its contents through {agent}.` |
| `hint.group_reach` / `.one` | `Hinweis: Die Gruppe {group} erreicht {count} Menschen.` / `Hinweis: Die Gruppe {group} erreicht einen Menschen.` | `Note: the group {group} reaches {count} people.` / `Note: the group {group} reaches one person.` |
| `hint.expiry_missing` | `Hinweis: Die Freigabe für {subject} hat kein Ablaufdatum.` | `Note: the grant for {subject} has no expiry date.` |

- Golden file `internal/authz/domain/previewtext/testdata/example_07_de.golden` (exactly 4 lines):

```text
Sam erhält bis 31.12.2026 die Stufe "Auslösen" im Kanal Marketing/Kampagnen.
Sam kann damit: Nachrichten lesen und schreiben, Threads anlegen, Agent Texter auslösen.
Dateien hochladen: nein. Freigabe-Karten bestätigen: nein.
Hinweis: Texter liest auch Marketing/Intern. Sam darf Marketing/Intern nicht lesen, kann aber über Texter Inhalte daraus erfahren.
```

- Golden file `example_07_en.golden`:

```text
Sam gets the level "Trigger" in channel Marketing/Kampagnen until 2026-12-31.
Sam can then: read and write messages, create threads, trigger agent Texter.
Upload files: no. Confirm approval cards: no.
Note: Texter also reads Marketing/Intern. Sam may not read Marketing/Intern but can learn its contents through Texter.
```

- World of both golden tests: owner (locale `de` resp. `en`, time zone `UTC`); Sam without grants;
  space Marketing with channels Kampagnen and Intern; agent Texter, `organization`, tool class
  `chat`, default operator of Marketing; grant: Sam, channel Marketing/Kampagnen, `trigger`, no
  capabilities, `expires_at = 2026-12-31T12:00:00Z`, reason `Kampagne`.

`access.view_as` (`viewAsUser`):

- `authorize(p, access.view_as, user)`: owner in a session, step-up, audit of allow and deny, class
  `rl.admin`. Unknown user or a principal that is not a human gives 404.
- The handler sets `actor.ViewAs{UserID, StartedAt: request_time}` and computes the visible spaces
  and channels through `Scope`/`authorize` with that context: effective rights are the
  intersection of owner and person (07 "4.2" step 4); DM channels never appear (SR6); archived
  structure never appears. Each item carries the intersected `level` and `capabilities`. A disabled
  person yields empty lists. `expires_at = request_time + authz.view_as_max_duration`.
- Audit entry: action `access.view_as`, resource `user`, details
  `{"target_user_id": ..., "expires_at": ...}`; no names.
- `authorize` with `ViewAs` set: every action whose catalog right writes ends with `forbidden`,
  reason `view_as_read_only` (07 "4.2" step 7); once
  `request_time >= StartedAt + authz.view_as_max_duration` every decision is `step_up_required`
  (the view ended; a new one needs a fresh step-up). No new reason code.

Other rules:

- No JSON `null` anywhere; absent optional fields are omitted (ADR 0053).
- Every handler proves its `authorize` call (02 "9.3").
- Rulings to record in `docs/DECISIONS.md`: language and date rule of the preview text; denial
  reasons give `allowed = false` instead of 403; preview rollback by sentinel error without a
  `kernel/txn` mode; view-as in M1 is a snapshot with `actor.ViewAs`, expiry as
  `step_up_required`; which capabilities the text always lists.

## Tests (write them first)

Use the fake-source harness of M1-28/M1-29 for domain and app tests and `pgtest` (PostgreSQL 18
throwaway container, `127.0.0.1`, random port) for integration tests. Test names carry the IDs.

Domain and app (fakes, controllable clock):

- `TestPreviewTextExample07German` and `TestPreviewTextExample07English`: render the world above
  and compare byte for byte with the golden files.
- `TestPreviewTextLanguageFallback`: locale `fr-FR` renders German, `en-GB` English.
- `TestPreviewHintToolClassHost`: Runner (`host`) operates Engineering/Werkzeuge; preview of
  Alex `trigger` there has the hint, and Runner is not in the abilities line.
- `TestPreviewHintGroupReach`: group with three valid and one expired membership gives `count` 3.
- `TestPreviewHintExpiryMissing`: user subject without expiry has the hint, group subject not.
- `TestPreviewAZM21_I1ViolationIsReportedNotVetoed`: world of 07 "11.1"; Owner previews Sam
  `trigger` on Research/Notizen while Assistent is `personal`: `allowed = false`, one violation with
  `agent_id` Assistent, `principal_ids` [Sam], `credential_holder_id` Owner, `organization_ready`
  false, text line `hint.i1`.
- `TestPreviewAZM31_AgentTriggerIsDenialNotError`, `TestPreviewAZM37_DelegationLevel`,
  `TestPreviewAZM38_DelegationCapabilities`: `allowed = false` with the reason code.
- `TestExplainSources`: world of 07 "11.1"; explanation of Support/Allgemein lists Owner (`owner`),
  Alex (`inherited` from G-1), Sam (`group` with membership expiry), Lotse (`direct` G-5 and
  `operator`).
- `TestExplainReadHorizonEntry`: Texter reads Marketing/Intern by a grant, Sam has `trigger` on
  Marketing/Kampagnen: explanation of Intern has Sam with level `none` and source `via_agent`
  (Texter).
- `TestExplainGuardBlockedSR2`: Alex `trigger` on Engineering/Werkzeuge: source `guard_blocked`,
  `rule = sr2`, agent Runner.
- `TestExplainInactivePrincipalMarked`: disabled Robin with a grant appears with `inactive = true`.
- `TestExplainMatchesAuthorizeInRandomWorlds` (property test with the AZ-P8 generator): for every
  entry, level and capabilities equal those `authorize` implies; every principal `authorize` lets
  read the target appears.
- `TestAZM46_ViewAsAlexCannotReadMarketing`: Owner with `ViewAs` Alex, `message.read` on
  Marketing/Kampagnen gives `not_found`.
- `TestAZM47_ViewAsIsReadOnly`: Owner with `ViewAs` Alex, `message.create` on Support/Allgemein
  gives `forbidden` `view_as_read_only`.
- `TestViewAsEndsAfterMaxDuration`: one tick before the end the intersection decides; at
  `StartedAt + authz.view_as_max_duration` the decision is `step_up_required`.
- `TestViewAsHidesDMs` (SR6): the DM Alex-Sam is absent from the snapshot of Alex.

Integration (`pgtest`, real authz and identity, fake chat and agent sources where the tables do
not exist yet):

- `TestPreviewRollsBackEverything`: before and after one preview the row counts of `grants`,
  `event_log`, `audit_events` and the values of `event_seq`, `audit_seq`, `permission_version` are
  equal, and a `LISTEN event_log` connection receives no notification.
- `TestPreviewNeedsNoStepUpAndWritesNoAudit`: owner with an expired step-up gets `200`.
- `TestPreviewAuthorization`: no read on the target `404`, read without `manage` `403`, `manage`
  `200`.
- `TestExplainAuthorization`: same three cases for both explain operations.
- `TestViewAsRequiresOwnerAndStepUp`: non-owner `403` (`owner_only`), owner without fresh step-up
  `401` `step_up_required`, owner with step-up `200`; each attempt has an audit entry (allow or
  deny).
- `TestViewAsSnapshotOfAlex`: G-1 world: only Support and its channels, levels and capabilities as
  `authorize` gives them for Alex, `expires_at` as specified.
- `TestAccessOperationsMatchSpec`: every response of the four operations validates against
  openapi.yaml (response validation of the test build) and contains no `null`.

Commands that must pass:

```sh
cd services/server && go test -race ./internal/authz/... ./internal/kernel/actor/... ./internal/identity/...
make codegen-check
make api-lint
make check
```

## Acceptance criteria

- [ ] The openapi and chapter changes above are merged in the same change and `make api-lint`,
      `make api-checks`, `make codegen-check` are green.
- [ ] The example from 07 "9.3" is reproducible byte for byte (German and English golden tests).
- [ ] The preview runs in the real transaction and leaves no row, counter or notification behind.
- [ ] AZ-M46 and AZ-M47 are green; view-as ends after `authz.view_as_max_duration`; every
      `viewAsUser` call has an audit entry.
- [ ] Hint H1 appears in preview (`read_horizon`) and explanation (`via_agent`).
- [ ] The differential test explanation versus `authorize` is green.
- [ ] The four operations are in the enabled list, registered with their actions (AZ-P1 stays
      green), and 404 and 403 follow 05 rule 2.
- [ ] `authz.view_as_max_duration` is in the config schema; `make config-doc-check` is green.
- [ ] `make check` is green.

## Definition of done

Code and tests as above; `internal/authz/README.md` updated (ports, operations, rules); doc comment
on every public symbol; no ADR needed unless you take an architecture decision beyond the rulings
listed (then write one); `services/server/CHANGELOG.md` entry under Unreleased; rulings in
`docs/DECISIONS.md`; `make check` green; an independent review and the red team
(`docs/quality/red-team-gate.md`) over the diff with the verdict recorded in the report; merge
locally into the base branch only if both are green, then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then commits,
files with line counts, test results with counts (including the red runs), what is not proven,
follow-ups that could not be done in scope with reason. Then write `{{DONE_PATH}}` with
`exit=0 sha=<sha>` or `exit=1 <reason>`.
