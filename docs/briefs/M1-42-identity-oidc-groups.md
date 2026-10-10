# M1-42: Groups from OIDC claims

Status: ready | Depends on: M1-41 | Etappe: M1

## Goal

After this ticket, groups with `source = 'oidc_claim'` follow the group claim of the identity
provider at every OIDC login: missing memberships are removed, new ones are added with an expiry,
and present ones are extended. Every change is a rights change with `access.changed`, checked
against invariant I1 in the same transaction; if the sync would violate I1, the login fails and
nothing changes. Owners can thus manage access in the identity provider without the provider ever
gaining rights the owner did not map to a group.

## Read first

- `docs/spec/08-identitaet.md`: 2.1 "Port" (`ExternalIdentity.Groups`), 2.2, 2.3 "Beispiel
  Microsoft Entra ID" (groups claim and overage), 3.4 "Gruppen aus Claims", 18
  (`auth.group_claim_max_age`), and the parts of 2.2 rewritten by M1-41 (callback stores the
  external identity, complete maps).
- `docs/spec/03-datenmodell.md`: 4.3 `groups`, `group_memberships`, the paragraph after `grants`
  (every change of `group_memberships` raises `permission_version`); 6.3 "Sperrreihenfolge";
  6.4 "I1-Prüfung"; 4.2 `auth_login_flows` and `identity_providers`; 4.12 `audit_events`.
- `docs/spec/05-aktionen.md` and `docs/spec/actions.yaml`: rule 1.6, 1.7, 1.10; 4.4 "Gruppen und
  Freigaben"; the system action for the sync (preconditions).
- `docs/spec/07-rechte.md`: 2.1 "Subjekte", 2.5 "Gültigkeit einer Freigabe", 4.4 "Invarianten in
  der Transaktion", SR1, 8.3 "Ablauf durch Zeit", 11.1 AZ-M07, AZ-M09, AZ-M25.
- `docs/spec/04-sync-protokoll.md`: 8.2 "Was beim Ändern passiert" (`access.changed`).
- `docs/spec/06-api.md`: 4 point 5 (`409 i1_violation` with `hint`), 5 "Fehler".
- `docs/spec/02-architektur.md`: 4.3, 4.4 (`identity` may use `authz`), 4.6, 5.
- ADRs 0017, 0018, 0019, 0024.
- `docs/api/openapi.yaml`: schemas `Group`, `GroupSource`, `GroupChangedPayload`,
  `AccessChangedPayload`.
- Merged code and READMEs: `internal/authz` (Admin port, rights-change phase hook,
  `VerifyInvariants`, from M1-29), `internal/identity` (OIDC flow of M1-41, provider registry of
  M1-36).
- `docs/PLAN.md` section 3 and section 7 rows M1-29, M1-30, M1-41, M1-42, M1-48;
  `docs/quality/gates.md`; `CONTRIBUTING.md`.

## Fixed requirements that apply

1. No MVP. Built from scratch directly in its final state; etappen are only the build order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined
   interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface has a docstring or
   doc comment (purpose, parameters, return value, errors, example where useful); comments explain
   why, not what. API documentation is generated from code (OpenAPI from the routes, dartdoc,
   Python API docs in the docs site).
4. Documentation in the repo: README per module/service, an ADR for every architecture decision,
   a CHANGELOG entry per release, docs site kept current. A change without matching documentation
   is not done.
5. Quality gates (SonarQube, linters, type checks, security scanners, test coverage) block the
   merge.
6. Test first: every behaviour change has a test that was red before the change (the red run is
   quoted in the report), then green, then refactor.
7. No en dash or em dash characters in any file you write; use a hyphen or rephrase.
8. White label: groups in tests are named Support, Marketing, Engineering, Research; people Alex,
   Sam, Robin; domains `example.com`.
9. Limits and time windows only through named configuration keys (`auth.group_claim_max_age`).
10. Groups with `source = 'oidc_claim'` are reconciled at every OIDC login; missing memberships are
    removed, new ones get `expires_at = login time + auth.group_claim_max_age` (08 3.4).
11. Every change is a rights change with I1 check in the same transaction; an I1 violation fails
    the login and changes nothing (08 3.4, 07 4.4, 05 rule 1.6).
12. Group overage or a missing claim counts as "no group information": no change, memberships run
    out after `auth.group_claim_max_age`; the server never calls Microsoft Graph (08 2.3).
13. Groups contain only people (R-DM-07); rights logic stays in `authz` (B7, 02 4.1 rule 6).

## Scope

- `services/server/internal/authz/domain/`: pure function `ReconcileClaimMemberships`
  (signature in Design notes) with table tests.
- `services/server/internal/authz/` port: new method on the `Admin` port
  `SyncClaimGroups(ctx context.Context, cmd SyncClaimGroupsCmd) (SyncClaimGroupsResult, error)`;
  DTOs `SyncClaimGroupsCmd{UserID uuid.UUID; ProviderID uuid.UUID; ClaimValues []string; Now time.Time; MaxAge time.Duration}`
  and `SyncClaimGroupsResult{Added, Removed, Extended []uuid.UUID}` (group IDs).
- `services/server/internal/authz/app/` and `adapters/postgres/`: the use case on the ambient Unit
  of Work, sqlc queries (read oidc_claim groups by `external_ref`, active memberships of the person
  in oidc_claim groups, insert, extend, remove).
- `services/server/internal/identity/app/` (OIDC use cases of M1-41) and
  `adapters/oidc/`: read the group claim at the callback, store it in the external identity, call
  `authz.Admin.SyncClaimGroups` inside the Unit of Work of `completeOidcLogin`.
- `identity_provider.oidc.v1` field `groups_claim` (08 2.6; M1-41 already accepts it): read it
  here; fake IdP of M1-41 gets knobs for a groups claim and an Entra overage claim.
- The specification is in place (planning, spec first): 08 2.2 step 6 (`external_identity.v1`
  fields `groups_state` and `groups`), 2.6, 3.4; 03 `groups.provider_id` with the unique index
  `(provider_id, external_ref)` (R-DM-33) and the note at `group_memberships`; 05
  `group.member.sync` and rule 1.6; 07 2.6 (claim groups are owned by the claim); 02 4.3 and 4.6.
  Change them only where the implementation proves them wrong.
- `services/server/internal/authz/README.md`, `services/server/internal/identity/README.md`,
  CHANGELOG entry under "Unreleased" of `services/server/CHANGELOG.md`, `docs/DECISIONS.md`
  entries for the rulings below, `docs/operations/oidc-entra.md` section on groups (claim
  configuration in Entra, overage behaviour, mapping by creating groups with `source = oidc_claim`
  and `external_ref`).

## Out of scope

- Creating, renaming and deleting groups and manual membership changes over the API: M1-48 (it
  rejects manual changes on claim groups with `409`, `reason: claim_managed_group`, 07 2.6).
- Expiry sweep and reminders of memberships: M1-30 (the expiry itself is evaluated by `authorize`
  at every call).
- Gateway delivery of `access.changed`: M1-65.
- Automatic creation of groups from unknown claim values: never (the owner maps groups).
- Group claims of the trusted header driver: none exist.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-42-identity-oidc-groups`, created from
  the base branch the launcher names.
- Conventional Commits with the module as scope (`feat(authz): ...`, `feat(identity): ...`) and a
  DCO sign-off on every commit (`git commit -s`).
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the Scope list, except the docs and changelog entries named in this
  brief.
- Follow the host rules supplied by the launcher.
- Tests bind only `127.0.0.1` and port 0 (P-10). PostgreSQL only through `internal/platform/pg/pgtest`;
  without a Docker-compatible runtime the test fails, it never skips (P-09).
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at
  the end.
- Never read or print secrets.
- No network calls except package downloads for pinned tools; never contact a real identity
  provider.
- A gap in the core contract (00, 03, 04, 05, `docs/spec/actions.yaml`) ends the ticket with
  `Verdict: BLOCKED` (P-11); never build a deviation.

## Design notes

**Preconditions (check before writing code; any failure: `Verdict: BLOCKED <which>`).**

1. The action catalogue (`docs/spec/actions.yaml`, 05 4.4) has the system action
   `group.member.sync` (resource `user`, right `system`, audit yes, rights yes, no rate-limit
   class), 05 rule 1.6 lists it among the actions with I1 check, and 02 4.6 assigns it to `authz`.
   Reason: a claim-driven change is neither an owner's `group.member.add` nor the expiry job, and
   every audit entry must name a catalogued action (R-AZ-15).
2. M1-41 is merged with the external identity carried in `auth_login_flows` and the mapping in
   `completeOidcLogin`.
3. M1-29 provides the rights-change path (`permission_version` locked first, diff of visible
   targets, `access.changed`, `VerifyInvariants`, deny audit in its own transaction).

**Provider field `groups_claim`** (`identity_provider.oidc.v1`, optional string, default absent =
no sync for this provider). Claim groups are bound to their provider (`groups.provider_id`, unique
`(provider_id, external_ref)`, R-DM-33): the sync of a login matches and changes only groups of
that login's provider, so several providers with `groups_claim` are allowed and never touch each
other's groups.

**Reading the claim at the callback** (only when `groups_claim` is set), into
`external_identity.v1`:

- Claim present as a JSON array of strings: `groups_state = "present"`, `groups` = the values,
  deduplicated, order irrelevant. Empty array is "present" with no values (removes all claim
  memberships).
- Claim absent: `groups_state = "absent"`.
- Entra overage (the claim name appears in `_claim_names`, 08 2.3): `groups_state = "overage"`.
- Claim present with another JSON type or a non-string element: the callback fails with
  `error=login_failed`.
- Values are never logged and never written to audit details (they can be names).

**Sync at complete.** Inside the Unit of Work of `completeOidcLogin`, after the person is
determined (also for a newly provisioned `pending` person) and before session creation, only for
`mode = 'login'` and `groups_state = "present"`: call `authz.Admin.SyncClaimGroups` with
`Now` = the request time of the clock and `MaxAge = auth.group_claim_max_age`. Any other state
changes nothing.

**`ReconcileClaimMemberships`** (pure, in `authz/domain`):

```go
// Inputs: the person's memberships with removed_at IS NULL in groups with source 'oidc_claim',
// provider_id = the login's provider and deleted_at IS NULL (expired ones included), the
// oidc_claim groups of that provider whose external_ref is one of the claim values, the claim
// values, now and maxAge.
func ReconcileClaimMemberships(current []ClaimMembership, matched []ClaimGroup,
    claim []string, now time.Time, maxAge time.Duration) ClaimDiff
// ClaimDiff{Add []uuid.UUID; Extend []uuid.UUID; Remove []uuid.UUID} (group IDs, sorted by ID)
```

- Add: matched group without a current membership: new row, `expires_at = now + maxAge`,
  `added_by` = the person.
- Extend: matched group with a current membership (expired or not): `expires_at = now + maxAge`,
  `expiry_reminded_at = NULL`. An expired row is extended, never duplicated (the partial unique
  index `(group_id, user_id) WHERE removed_at IS NULL` forbids a second row).
- Remove: current membership whose group is not matched: `removed_at = now`, `removed_by` = the
  person.
- Claim values without a matching group are ignored. Groups with `source = 'local'` and claim
  groups of other providers are never touched.
- Ruling: `added_by` and `removed_by` are the person whose provider asserted the claim, because
  03 requires a principal and the change is caused by that login; the audit entry names the system
  action, so it cannot be mistaken for a manual change.

**Transaction** (in `authz/app`, following 03 6.3 and 07 4.4): lock `permission_version` first,
lock the affected membership rows in ascending `id`, apply the diff, run `VerifyInvariants` (I1
for all agents with `personal`, 03 6.4), then the phase hook writes `access.changed` for the person
(diff of visible targets; follow the M1-29 API, do not compute the diff in `identity`) and one
`group.changed` per added or removed membership (`change` `member_added` or `member_removed`,
`user_id`), and one audit entry: action from precondition 1, actor = the person,
`decision = 'allow'`, `resource_kind = 'user'`, `resource_id` = person, details
`{provider_id, added, removed, extended}` with group IDs only. A diff with nothing to add, extend
or remove writes nothing and does not raise `permission_version`.

**I1 violation.** `VerifyInvariants` fails: the whole Unit of Work of `completeOidcLogin` rolls
back (no memberships, no identity link, no device, no session, the enrollment code stays
unused). In a separate transaction: flow `status = 'failed'`, audit entry with the sync action,
`decision = 'deny'`, `deny_reason = 'i1_violation'`, details `{provider_id, group_ids, agent_ids}`
(this is the owner's notice in the audit log; the console shows it from M6). Response `409
i1_violation` with `hint` "Sign-in blocked by a group mapping. An owner must switch the affected
agent to organization first."

**Concurrency.** Two parallel logins of the same person serialise on `permission_version`; the
second sees the first's rows. Test it.

## Tests (write them first)

Unit tests:

- `TestReconcileClaimMemberships` (table): add, extend of a live and of an expired membership,
  remove, unknown values ignored, local groups untouched, empty claim removes all claim
  memberships, duplicate claim values, deleted groups ignored, output sorted.
- `TestGroupsClaimParsing`: array of strings, empty array, absent, Entra overage via
  `_claim_names`, wrong type, non-string element.
- `TestGroupClaimSync_OtherProviderUntouched`: two providers with `groups_claim` and groups with
  the same `external_ref`; a login through provider A changes only A's groups.

Integration tests (PostgreSQL 18, fake IdP of M1-41, fake `AgentPolicySource` and
`StructureSource` for I1 as in AZ-P7, steerable clock):

- `TestGroupClaimSync_AddsWithExpiry`: Support group mapped; first login adds Sam with
  `expires_at = login + auth.group_claim_max_age`; `access.changed` for Sam; `group.changed`
  `member_added`; `permission_version` raised; audit entry with the sync action.
- `TestGroupClaimSync_ExtendsOnLogin`: second login later moves `expires_at` and clears
  `expiry_reminded_at`.
- `TestGroupClaimSync_RemovesMissing`: value gone from the claim removes the membership with
  `removed_by` = Sam and `member_removed`.
- `TestGroupClaimSync_AbsentClaimNoChange` and `TestGroupClaimSync_EntraOverageNoChange`: no row,
  no event, no version change.
- `TestGroupClaimSync_ExpiryWithoutJob`: with the clock at `expires_at` and no job running, Sam
  loses read access through the group (AZ-M09 behaviour with a claim group).
- `TestGroupClaimSync_I1ViolationFailsLoginNothingChanges`: a fake personal agent operating a
  channel on which the Engineering group has `trigger`; Robin's claim adds Engineering; complete
  gives `409 i1_violation`; no membership, no session, no device, no identity link, enrollment code
  unused; deny audit entry present.
- `TestGroupClaimSync_ProviderWithoutGroupsClaimNoChange`.
- `TestGroupClaimSync_ConcurrentLogins`: two parallel completes for the same person end with one
  consistent membership set.
- `TestGroupClaimSync_ValuesNeverLogged`: log and audit capture contain no claim value.

Commands that must pass:

```sh
cd services/server && go test -race ./internal/authz/... ./internal/identity/...
make codegen-check
make arch
make docs
make check
```

## Acceptance criteria

- [ ] Preconditions verified and named in the report.
- [ ] Missing memberships removed, new ones added with expiry, present ones extended, only in
      `oidc_claim` groups.
- [ ] Every change raises `permission_version`, writes `access.changed` and one audit entry with
      the catalogued sync action; a no-op login writes nothing.
- [ ] An I1 violation fails the login with `409 i1_violation` and leaves no change; the deny audit
      entry exists.
- [ ] Absent claim and overage change nothing; the server makes no call besides the provider
      endpoints of M1-41.
- [ ] Claim groups match only for their own provider.
- [ ] The implementation matches 03, 05, 07 and 08 (changed only where proven wrong); READMEs,
      DECISIONS and `oidc-entra.md` updated; `make check` green.

## Definition of done

- Code and tests as above; every public symbol has a doc comment (purpose, parameters, return
  value, errors, example where useful); comments explain why.
- READMEs of `authz` and `identity` current; CHANGELOG entry under "Unreleased";
  `docs/DECISIONS.md` entries; an ADR only if a decision beyond 08 3.4 and ADR 0018 was taken
  (MADR 4, `make adr-index`).
- `make check` green.
- Independent review (`make review` and a code review over the diff) and the red team gate over
  the diff, verdict recorded in the report; red team item 3 of the M1 section (no sequence of
  changes bypasses I1, also in parallel) applies directly.
- Merge locally into the base branch only if review and red team are green and `make check` is
  green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then the
commits, the files with line counts, the test results with counts (including the red runs), what
is not proven, and follow-ups that could not be done in scope with the reason (at least the note
for M1-48 about manual changes on claim groups). Then write `{{DONE_PATH}}` with
`exit=0 sha=<sha>` or `exit=1 <reason>`.
