# M1-48: Groups and grants over the API

Status: ready | Depends on: M1-31, M1-44 | Etappe: M1

## Goal

After this ticket owners manage groups and memberships, and owners and holders of `manage`
manage grants, through the twelve REST operations of 06 "Groups" and "Access". Every change runs
through `authorize` with step-up, respects the delegation limits of `manage` (R-AZ-05), is a
rights change with I1 and SR8 in the same transaction where 05 says so, writes `grant.changed` or
`group.changed` for administrators, `access.changed` for every affected person and an audit
entry. Reads show nothing a caller may not see: no grant outside the caller's `manage` scope, no
group the caller is not in, unless the caller is an owner.

## Read first

- `docs/PLAN.md`: section 3 "Arbeitsweise je Ticket"; section 7 rows M1-27 to M1-31, M1-42,
  M1-44, M1-48, M1-49, M1-54, M2-21; "Voraussetzungen aus dem Kern-Vertrag" (row with K-RE-6).
- `docs/spec/07-rechte.md`: "Freigabe-Modell" (2.1 to 2.7, especially "Gültigkeit einer
  Freigabe" and "Delegation mit `manage`"), "Prinzipal-Kontext" (3), "Entscheidungen und
  Gründe" (4.1), "Ablauf" (4.2: requirement before step-up), "Sichtbarkeit" (4.3),
  "Invarianten in der Transaktion" (4.4), SR1, SR5, SR8, "Sofortige Wirkung bei Entzug" (8.2),
  "Weitere Pflichten" (9.5: every grant needs a reason), "Beispiele" (10), "Testmatrix" (11.1)
  rows AZ-M21, AZ-M25, AZ-M31, AZ-M35 to AZ-M39, "Pflicht-Tests" (11.2) AZ-P4, AZ-P13.
- `docs/spec/05-aktionen.md`: "Verbindliche Regeln" items 2, 6 and 7; 2.1 to 2.5; 4.4 "Gruppen
  und Freigaben"; 6 rows `grant.changed`, `group.changed`, `access.changed`; 8 R-AZ-05, R-AZ-14.
- `docs/spec/03-datenmodell.md`: 4.3 tables `groups`, `group_memberships`, `grants` (paragraph
  after `grants`); "Sperrreihenfolge" (6.3); "I1-Prüfung" (6.4).
- `docs/spec/04-sync-protokoll.md`: "Umschlag (Envelope)" (2.2), "Was beim Ändern passiert"
  (8.2).
- `docs/spec/06-api.md`: "Grundregeln" (2), "Autorisierung in der API" (4), "Fehler" (5),
  "Paginierung" (6), "Idempotenz" (7), 16 "Endpunkte" groups "Groups" and "Access".
- `docs/spec/08-identitaet.md`: "Gruppen aus Claims" (3.4).
- `docs/spec/19-sicherheit.md`: SEC-IN-4.
- `docs/spec/02-architektur.md`: 4.3 (row `authz`: `Admin`, `PrincipalSource`,
  `StructureSource`, `AgentPolicySource`), 4.4, 5, 8.1 to 8.3, 9.2, 9.3.
- ADRs: `docs/adr/0017-eine-autorisierungsfunktion-404-statt-403-registrierungszwang.md`,
  `docs/adr/0018-freigabe-modell-stufen-zusatzrechte-vererbung-kein-verbot.md`,
  `docs/adr/0021-audit-log-hash-verkettet-und-getrennt-von-telemetrie.md`.
- `docs/api/openapi.yaml`: operations `listGroups`, `createGroup`, `getGroup`, `updateGroup`,
  `deleteGroup`, `listGroupMembers`, `addGroupMember`, `removeGroupMember`, `listGrants`,
  `createGrant`, `getGrant`, `revokeGrant`; schemas `Group`, `GroupCreate`, `GroupUpdate`,
  `GroupMember`, `GroupMemberAdd`, `GroupPage`, `GroupMemberPage`, `Grant`, `GrantCreate`,
  `GrantPage`, `ReasonBody`, `PrincipalRef`, `GrantChangedPayload`, `GroupChangedPayload`.
- Code of earlier tickets: `internal/authz` (M1-27 domain and store, M1-28 `authorize`, snapshot
  and `Scope`, M1-29 `Admin` port, rights-change hook and `VerifyInvariants`, M1-30 expiry),
  `internal/identity` (M1-26 `PrincipalSource` implementation, M1-31 cursor helper and response
  validation, M1-34 and M1-35 authentication and step-up, M1-44 personal tokens),
  `internal/platform/httpserver`, `services/server/api/operations.yaml`.
- Tables: `groups`, `group_memberships`, `grants`. Action ids: `group.read`, `group.create`,
  `group.update`, `group.delete`, `group.member.add`, `group.member.remove`, `grant.read`,
  `grant.create`, `grant.revoke`.

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
6. Test first: every behaviour gets a test that is red before the code exists (plan section 3).
7. No en dash or em dash in any text you write; white label (people Alex, Sam, Robin; spaces
   Support, Marketing, Engineering, Research; agents as in 07 10); configuration only through
   named keys from `docs/operations/configuration.md`.
8. 05 4.4: group writes are owner only with step-up and audit; `grant.read` needs owner or
   `manage`; `grant.create` and `grant.revoke` need owner or `manage`, step-up and audit;
   `group.read` is owner or member ("eigene Gruppen auch ohne Owner").
9. 07 2.6 and R-AZ-05: with `manage`, a new grant is at most the own effective level and
   capabilities on its target, only for people and groups, never on the instance; revocation
   only of grants one could have created. Grants to agents and apps only by an owner; agents and
   apps never `trigger` and only `upload` (AZ-M31).
10. 07 4.2: decision order; a missing right is reported before a missing step-up; a personal
    token can never provide step-up (`step_up_not_possible`, AZ-M35); owner rights only in a
    session (SR5, R-AZ-14).
11. 05 rule 1.6 and 07 4.4: `grant.create`, `grant.revoke`, `group.member.add`,
    `group.member.remove`, `group.delete` check I1 and SR8 in the same transaction; a violation
    rolls back completely and answers `409 i1_violation` with `hint` (AZ-M21, AZ-M25).
12. 05 rule 1.7 and 04 8.2: these five actions raise `permission_version` and write
    `access.changed` for every affected principal.
13. 03 R-DM-09 and 07 9.5: grants are never changed, only revoked; every grant and every
    revocation has a reason.
14. 05 rule 2 and 06 4: no read right gives `404`, the same as a missing resource; lists filter
    in the query and carry no totals.
15. 05 6: `grant.changed` (receive action `grant.read`) and `group.changed` (receive action
    `group.read`) on every write.
16. 19 SEC-IN-4: group names and descriptions with control or bidi control characters are
    rejected.

## Scope

Paths are relative to `services/server/` unless they start with `docs/`.

1. **Specification** (in place from planning): 07 2.6 item 6 (visibility of grants to `manage`
   holders) and the paragraph on `oidc_claim` groups (`409 conflict`, `reason:
   claim_managed_group`); 05 `group.member.add`, `group.member.remove`, `grant.read`; 03 `groups`
   (`provider_id`, checks, unique `(provider_id, external_ref)`, length keys); 06 2 "Ohne Wirkung".
   Change them only where the implementation proves them wrong. `docs/DECISIONS.md`: the rulings
   under Design notes, in the file's existing format.
2. **Application** `internal/authz/app`: use cases `ListGroups`, `GetGroup`, `ListGroupMembers`,
   `CreateGroup`, `UpdateGroup`, `DeleteGroup`, `AddGroupMember`, `RemoveGroupMember`,
   `ListGrants`, `GetGrant`, `CreateGrant`, `RevokeGrant`. Every write goes through the `Admin`
   port of M1-29 (or extends it in this module when a method is missing). Read use cases may use
   the store of M1-27 directly (same module).
3. **Delegation in `authorize`.** For `grant.create` the resource descriptor carries the
   proposed grant shape (subject kind, target, level, capabilities); for `grant.revoke` it
   carries the existing grant's shape. `authz` evaluates `domain.CheckCreate` or
   `domain.CheckRevoke` (M1-27) in the requirement step (07 4.2 step 6), so the reasons
   `subject_kind_not_allowed`, `owner_only`, `delegation_exceeds_own_level` and
   `delegation_exceeds_own_capabilities` come before `step_up_required`. If M1-28 or M1-29
   already do this, reuse it; otherwise add it to `internal/authz` with unit tests.
4. **Principal references.** `GroupMember.user` is a `PrincipalRef`. `authz` gets it through
   its `PrincipalSource`; if that port has no batch method for references, add
   `Refs(ctx context.Context, ids []ids.ID) (map[ids.ID]authz.PrincipalRef, error)` to the port
   and implement it in the identity adapter of M1-26.
5. **Adapter postgres** `internal/authz/adapters/postgres`: read queries for the lists below
   (keyset pagination by `id`), `member_count`, membership history for `removeGroupMember`;
   sqlc regenerated.
6. **Adapter httpapi** `internal/authz/adapters/httpapi`: the twelve strict-server methods.
7. **Freischalt-Liste** `api/operations.yaml`: the twelve operations.
8. **Documentation**: `internal/authz/README.md` (API use cases, visibility rules, events),
   `services/server/CHANGELOG.md` under `Unreleased`.

## Out of scope

- `previewGrant`, `explainSpaceAccess`, `explainChannelAccess`, `viewAsUser`: M1-49.
- `resolve_i1` on `grant.create` and `group.member.add` (05 rule 1.6, guided flow): M2-21. The
  schemas have no such field yet; do not add it.
- Claim group reconciliation at login: M1-42. Expiry jobs and reminders: M1-30.
- Spaces and channels as real data: M1-52, M1-53. Until then the tests use a fake
  `StructureSource`; M1-54 reruns the I1 cases with all real sources.
- Console and app screens: M4-26, M4-27, M6.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-48-authz-api`.
- Conventional Commits with module scope (for example `feat(authz): grants over the API`) and
  DCO sign-off (`git commit -s`).
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope above except `services/server/CHANGELOG.md`,
  `docs/DECISIONS.md`, the sentence in 07 9.5 and the `PrincipalSource` implementation in
  `internal/identity` (Scope item 4).
- Follow the host rules supplied by the launcher.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`; remove
  them by their literal path at the end. PostgreSQL runs only as the throwaway container of
  `pgtest` on a random port bound to `127.0.0.1`.
- Never read or print secrets. No network calls except package downloads for pinned tools.

## Design notes

**Visibility and authorization per operation** (`t` = one clock read per request):

| Operation | Resource for `authorize` | Not visible | Visible, not allowed |
|---|---|---|---|
| `listGroups` | instance (scope) | | owner: all live groups; others: groups with an in-effect membership of the caller; `member_id` filter for non-owners other than self gives an empty page |
| `getGroup`, `listGroupMembers` | group | `404` (deleted, unknown, or caller neither owner nor in-effect member) | |
| `createGroup` | instance | | `403 forbidden` with `reason` `owner_only` |
| `updateGroup`, `deleteGroup`, `addGroupMember`, `removeGroupMember` | group | `404` | `403 forbidden` with `reason` `owner_only` |
| `listGrants` | target from filter, or scope | `404` for a target filter the caller cannot read | `403` for a readable target without `manage` |
| `getGrant`, `revokeGrant` | target of the grant, then the grant | `404` unless owner or `manage` on the grant's target (a channel inherits `manage` from its space); instance grants: owners only | `revokeGrant`: delegation reasons `403` |
| `createGrant` | target of the new grant | `404` if the caller cannot read the target | delegation reasons `403` |

A `404` for an existing but invisible grant or group has the same status, body and headers as
for a random UUID (AZ-P4). Step-up is checked after the requirement (07 4.2); without fresh
step-up: `401 step_up_required`; with a token without session (agent, app, personal token):
`403 forbidden` with `reason` `step_up_not_possible`.

**`manage` scope for grant reads.** Non-owners see grants whose target is a space on which they
have `manage`, or a channel on which they have `manage` directly or through its space. Compute
the set from the caller's rights snapshot (M1-28; owners: everything). Instance grants are owner
only. Reason: 05 lists `grant.read` as "owner oder manage" on `space`, `channel`, `principal`; a
`principal` filter alone gives no target, so for non-owners it narrows within the `manage` scope
instead of widening it.

**`listGrants` filters.** `target_kind` with `target_id` (except `instance`, which has no id),
`subject_kind` with `subject_id`; a half pair is `400 validation_failed` with pointer
`#query/<name>`. Default: grants in effect at `t` (not revoked, not expired); `include_revoked =
true` adds revoked and expired ones. Order by `id`, keyset cursor bound to the filters (`400
cursor_invalid` otherwise), `limit` default `api.page_size_default`, capped at
`api.page_size_max`, no totals.

**`createGrant`** (body `GrantCreate`, `Idempotency-Key`):

1. Shape: `target_id` required unless `target_kind = instance`, forbidden for `instance`;
   `capabilities` unique and known; `expires_at` absent or after `t` (else pointer
   `/expires_at`, code `in_past`); `reason` not blank after trim.
2. `authorize(grant.create, target with proposed shape)`: visibility, owner or `manage`,
   delegation (`domain.CheckCreate`: agent or app with `trigger` or a capability beyond
   `upload` is `subject_kind_not_allowed` even for owners; non-owner to agent or app
   `subject_kind_not_allowed`; non-owner on instance `owner_only`; above own level or
   capabilities `delegation_exceeds_own_level` or `delegation_exceeds_own_capabilities`), step-up.
3. Subject exists: `user`, `agent`, `app` through `PrincipalSource` (kind must match, status not
   `deleted`); `group` through the store (not deleted). Otherwise `400 validation_failed`,
   pointer `/subject_id`, code `unknown_subject`. This comes after `authorize`, so a caller
   without rights learns nothing about principal ids.
4. Unit of work with `RightsChange`; affected principals: the subject principal, or all
   people with an in-effect membership of the subject group at `t`. Insert through `Admin`;
   append `grant.changed {grant, change: created}`; audit allow with `details = {subject_kind,
   subject_id, target_kind, target_id, level, capabilities, expires_at, reason}`; hooks do I1,
   SR8, `access.changed`.
5. `201` with `Grant`.

**`revokeGrant`** (body `ReasonBody`, `Idempotency-Key`): visibility as in the table, then
`authorize(grant.revoke, grant)` with `domain.CheckRevoke`, step-up. Already revoked: `200` with
the stored grant, no write, no event, audit allow with `details.no_change = true` (decide this in
a read before opening the RightsChange unit of work, because phase 1 raises
`permission_version` unconditionally). Otherwise set `revoked_at`, `revoked_by`,
`revoke_reason = reason`; affected principals as for create; `grant.changed {change: revoked}`;
audit. An expired grant may be revoked; it changes no rights but closes the record.

**Groups.**

- `createGroup`: `name` trimmed, not blank, display-text check (SEC-IN-4, the shared check of
  M1-26, `kernel/textcheck`) and at most `group.name_max_chars`; `description` same check, may be
  empty, at most `group.description_max_chars`;
  `source` default `local`; `oidc_claim` requires a non-blank `external_ref` and the
  `provider_id` of an existing OIDC provider (else `400`, pointer `/external_ref` or
  `/provider_id`), `local` forbids both. Name taken by a live group (case-insensitive) or
  `(provider_id, external_ref)` taken by a live group: `409 conflict` with `errors` pointer
  `/name` or `/external_ref`. Not a rights change (05 "Rechte" `nein`).
  `group.changed {change: created}`; audit; `201`.
- `updateGroup` (PATCH): at least one of `name`, `description`; same checks and conflicts;
  `group.changed {change: updated}`; audit; `200`.
- `deleteGroup`: sets `deleted_at`, revokes every live grant of the group (`revoke_reason =
  'group_deleted'`, `revoked_by` = actor) in one RightsChange unit of work whose affected
  principals are the group's in-effect members; memberships stay as history; one
  `group.changed {change: deleted}`, one `grant.changed {change: revoked}` per grant; audit;
  `204`. An already deleted group: `204` without change (06 7, DELETE).
- `addGroupMember` (body `GroupMemberAdd`, `Idempotency-Key`): group live, else `404`; group
  with `source = oidc_claim`: `409 conflict` with `reason: claim_managed_group` (07 2.6); `user_id` must be a principal of kind `user` not `deleted` (else `400`,
  pointer `/user_id`, code `unknown_user`); already an in-effect member: `409 conflict`;
  `expires_at` absent or after `t`. RightsChange with affected `[user]` (I1: AZ-M25);
  `group.changed {change: member_added, user_id}`; audit; `201` with `GroupMember`.
- `removeGroupMember`: live group; the in-effect membership gets `removed_at`, `removed_by`;
  RightsChange `[user]`; `group.changed {change: member_removed, user_id}`; audit; `204`. A
  membership that existed but is already removed or expired: `204` without change; never a
  member: `404`. Removing from an `oidc_claim` group gives `409 conflict` with `reason:
  claim_managed_group` (07 2.6): the next login would re-add it silently; an owner cuts access
  with `user.disable` or by deleting the group.
- `member_count` counts memberships in effect at `t`. `listGroupMembers` lists memberships in
  effect at `t`, ordered by membership `id`, keyset cursor.

**Events.** `grant.changed`: `resource = {kind: grant, id}`, `scope` = the target's `space_id`
and `channel_id` (from `StructureSource`; empty for instance grants), payload
`GrantChangedPayload`. `group.changed`: `resource = {kind: group, id}`, empty `scope`, payload
`GroupChangedPayload`. `access.changed` only through the M1-29 hook. Never `null`.

**Rulings for `docs/DECISIONS.md`:**

1. Non-owners see grants only inside their `manage` scope; filters narrow, never widen it.
2. A grant or group the caller may not read is `404` for every operation on it.
3. Revoking an already revoked grant succeeds without change and is audited with `no_change`
   (06 2 "Ohne Wirkung").
4. Deleting a group revokes its grants with reason `group_deleted` and keeps memberships as
   history.
5. `member_count` and member lists show memberships in effect at request time.

## Tests (write them first)

World of 07 10 and 11.1 with real `identity`, `authz`, `audit`, `sync`, `instance` against
PostgreSQL 18, real access tokens with DPoP and real step-up (M1-34, M1-35), personal tokens
created through the identity test helper (not over HTTP, so the test does not depend on the flag
`tokens.personal_enabled`). Fakes only for `authz.StructureSource` (spaces Support, Marketing,
Research, Engineering with their channels) and `authz.AgentPolicySource` (Lotse, Texter,
Assistent with `billing_mode = personal` serving Research/Notizen, credential holder the owner).
A controllable clock. Test names carry the spec IDs.

1. HTTP integration, matrix rows:
    - `TestAZM31_OwnerGrantsTriggerToAgent`: `403 forbidden` with `reason` `subject_kind_not_allowed`.
    - `TestAZM35_PersonalTokenCannotGrant`: Sam with `trigger` and `manage` on Marketing creates
      a `read` grant for Robin with a personal token: `403 forbidden` with `reason` `step_up_not_possible`.
    - `TestAZM36_StaleStepUp`: owner with step-up older than `auth.step_up_max_age`: `401
      step_up_required`.
    - `TestAZM37_DelegationAboveOwnLevel`, `TestAZM38_DelegationAboveOwnCapabilities`,
      `TestAZM39_RevokeStrongerGrant`.
    - `TestAZM21_GrantBreaksI1` and `TestAZM25_MemberAddBreaksI1`: `409 i1_violation` with
      `hint`; a row snapshot of `grants`, `group_memberships` and `sequence_counters` before and
      after is equal; a deny audit entry exists after the rollback.
    - `TestManageCannotGrantToAgentOrOnInstance`, `TestRequirementBeforeStepUp` (a non-manager
      without step-up gets `403`, not `401`).
2. HTTP integration, behaviour:
    - groups: create, duplicate name `409`, bidi name `400`, `oidc_claim` without
      `external_ref` `400`, update, delete revokes grants and writes `access.changed` for every
      member, add and remove member with events, add to and remove from an `oidc_claim` group
      `409 conflict` with `reason` `claim_managed_group`, remove twice
      `204`, never member `404`, `member_count` excludes an expired membership at the exact
      second.
    - group visibility: owner sees all; Sam sees only groups he is in; Robin gets `404` on a
      group he is not in, identical to a random id (AZ-P4 style comparison of status, body,
      headers).
    - grants: list as owner with each filter; as Alex with `manage` on Support only grants on
      Support and its channels; half filter pair `400`; `include_revoked`; cursor round trip and
      foreign cursor `400 cursor_invalid`; `getGrant` of an instance grant by Alex `404`;
      `revokeGrant` twice gives `200` and one event; unknown subject `400 validation_failed` with `errors[].code` `unknown_subject` only
      after authorization (a non-manager gets `404` or `403` for the same body).
    - every write: one audit allow entry; every denial: a deny entry (AZ-P13 aggregation is
      M1-24's; assert presence).
    - every write emits exactly the events listed in Design notes and one `access.changed` per
      affected person per transaction.
3. Unit tests in `internal/authz` for the delegation-in-requirement wiring if you add it.
4. Every response passes the response validation of the test build.

Commands that must pass:

```sh
cd services/server && go test -race ./internal/authz/... ./internal/identity/...
make codegen-check
make arch
make check
```

## Acceptance criteria

- [ ] AZ-M31, AZ-M35 to AZ-M39 are green with real authentication; AZ-M21 and AZ-M25 are green
      through the API with fake structure and agent sources.
- [ ] Grants to agents and apps are created only by owners and never with `trigger` or a
      capability beyond `upload`.
- [ ] `grant.read` works only for owners and within `manage`; nothing outside is listed or
      readable, and invisible ids answer like random ids.
- [ ] Every change writes `access.changed` for all affected principals, `grant.changed` or
      `group.changed`, and an audit entry; a rejected change leaves no partial state.
- [ ] Group writes are owner only with step-up; member lists and counts use in-effect
      memberships.
- [ ] `make check` is green.

## Definition of done

Code, tests and documentation are complete: module README updated, doc comments on every public
symbol, an ADR if you took an architecture decision beyond this brief, CHANGELOG entry under
`Unreleased` in `services/server/CHANGELOG.md`. `make check` is green. An independent review and
the red team (`docs/quality/red-team-gate.md`) have run over the diff with their verdicts
recorded in the report. Merge locally into the base branch only if both are green; then remove
the worktree with `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then the
commits, the files with line counts, the test results with counts (including the red run of each
new test before the code), what is not proven, and follow-ups that could not be done in scope
with the reason. Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
