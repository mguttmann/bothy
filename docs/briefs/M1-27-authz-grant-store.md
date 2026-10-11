# M1-27: Grant model storage

Status: ready | Depends on: M1-26 | Etappe: M1

## Goal

After this ticket the tables `groups`, `group_memberships` and `grants` exist with every check
of the core contract, grants are immutable in the database except for revocation and the expiry
bookkeeping (`expiry_reminded_at`, `expiry_reported_at`), and the rules of the grant model are
pure, table-tested domain functions: levels and capabilities, validity at a point in time (07
section 2.5) and delegation with `manage` (R-AZ-05, 07 section 2.6). M1-28 builds the rights
snapshot on these functions and this repository; M1-29 and M1-48 write through them.

## Read first

- `docs/PLAN.md`: section 3 "Arbeitsweise je Ticket"; section 7 rows M1-27 to M1-30, M1-42,
  M1-48, M1-52, M1-53.
- `docs/spec/03-datenmodell.md`: section 1 "Grundsätze" (1, 2, 3, 7, 12), section 4.3 tables
  `groups`, `group_memberships`, `grants` (including the paragraph after the `grants` table),
  section 6.3 "Sperrreihenfolge", section 8 R-DM-07, R-DM-08, R-DM-09, R-DM-19, R-DM-20.
- `docs/spec/07-rechte.md`: section 1 "Grundsätze", section 2.1 to 2.6, section 7.2 (how the
  snapshot uses grants), section 12 R-RE-03 and R-RE-08, test rows AZ-M31, AZ-M37 to AZ-M39.
- `docs/spec/05-aktionen.md`: section 2.3 "Wirksame Stufe", section 4.4 "Gruppen und
  Freigaben", section 8 R-AZ-05.
- `docs/adr/0018-freigabe-modell-stufen-zusatzrechte-vererbung-kein-verbot.md`.
- `docs/spec/02-architektur.md`: sections 4.1, 4.2 (`authz` row), 5, 6, 7.3 (table ownership).
- Code of earlier tickets: `internal/authz` port package and `internal/authz/sources.go`
  (M1-05, M1-26), `internal/identity` (M1-26, for the foreign keys), `internal/platform/pg` and
  `pgtest` (M1-12), unit of work (M1-13), `migrations/` and `sqlc.yaml` conventions.
- API operations: none. Tables: `groups`, `group_memberships`, `grants`. Action ids (rules
  only, no use cases): `grant.create`, `grant.revoke`, `group.member.add`.

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
7. No en dash or em dash in any text you write; white label (examples use Alex, Sam, Robin and
   example.com, groups Support, Marketing, Engineering, Research); configuration only through
   named keys from `docs/operations/configuration.md`.
8. 03 section 4.3: columns, checks, unique indexes, indexes and foreign keys exactly as specified
   (exception: the target foreign keys, see Design notes).
9. R-DM-07: groups contain only people (foreign key to `users`).
10. R-DM-08 and R-DM-20: grants to agents and apps never `trigger`, capabilities at most
    `upload`, held by a database check.
11. R-DM-09: grants are never changed; a change is a revocation plus a new row.
12. 07 section 2.5: validity is evaluated at one time `t`; at `expires_at = t` a grant or
    membership is already invalid.
13. R-AZ-05 and 07 section 2.6: delegation limits for creating and revoking grants.

## Scope

Paths are relative to `services/server/`.

1. **Migration** `migrations/<next>_authz_groups_grants.sql` (next free sequence number in the
   M1-12 format; renumber at merge time if a parallel ticket took it): the three tables, their
   indexes and the trigger `authz_grants_immutable`; Down section per the M1-12 convention.
2. **Domain** `internal/authz/domain`: `levels.go` (`Level`), `caps.go` (`Capability`, `Caps`),
   `grant.go` (`SubjectKind`, `TargetKind`, `Subject`, `Target`, `Grant`, `NewGrant`),
   `group.go` (`Group`, `Membership`, `NewGroup`, `NewMembership`), `validity.go`,
   `delegation.go`, `reasons.go` (only the four reasons this ticket returns).
3. **Outgoing port** `internal/authz/app/store.go`: interface `Store` (signatures below). No use
   case in `app` yet.
4. **Adapter** `internal/authz/adapters/postgres`: `queries/groups.sql`,
   `queries/memberships.sql`, `queries/grants.sql`, sqlc output in `sqlcgen/`, repository
   implementing `app.Store`, error mapping. Add or complete the authz entry in `sqlc.yaml`.
5. **Documentation**: `internal/authz/README.md` (tables, the deferred foreign keys, the
   immutability trigger, the pure functions), `services/server/CHANGELOG.md` under
   `Unreleased`.

## Out of scope

- The rights snapshot, cache and `authorize`: M1-28.
- Rights changes with `permission_version`, `access.changed`, I1, SR8, the admin port: M1-29.
  The repository writes of this ticket raise no version; they are only called by tests here.
- Expiry jobs and reminders (`expiry_reminded_at` and `expiry_reported_at` are only columns here):
  M1-30.
- Group and grant HTTP operations, "who has access" and preview: M1-48, M1-49.
- OIDC claim groups: M1-42.
- The foreign keys from `grants` to `spaces` and `channels`: M1-52 and M1-53 add them in the
  migrations that create those tables.

## Boundaries

- A new migration needs its line in `migrations.sha256`: run `make migrations-lock` after creating
  the file and before `make check`; never edit a merged migration (M1-01, ADR 0007).
- Work in a dedicated git worktree on the branch `feat/M1-27-authz-grant-store`.
- Conventional Commits with scope (`feat(authz): ...`) and DCO sign-off (`git commit -s`).
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope above except `services/server/CHANGELOG.md` and
  `docs/DECISIONS.md` (only if you record a ruling).
- Follow the host rules supplied by the launcher.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`; remove
  them by their literal path at the end. PostgreSQL runs only as the throwaway container of
  `pgtest` on a random port bound to `127.0.0.1`.
- Never read or print secrets. No network calls except package downloads for pinned tools.

## Design notes

**Tables** (03 section 4.3, verbatim):

- `groups`: `source` default `'local'` with `CHECK (source IN ('local','oidc_claim'))`,
  `provider_id uuid NULL` (its FK to `identity_providers` follows with M1-36, which creates that
  table), the checks `(source = 'oidc_claim') = (provider_id IS NOT NULL)` and `(source =
  'oidc_claim') = (external_ref IS NOT NULL)`, `description` default `''`, FK `created_by ->
  principals(id)` ON DELETE RESTRICT, partial unique index `lower(name) WHERE deleted_at IS NULL`,
  partial unique index `(provider_id, external_ref) WHERE source = 'oidc_claim' AND deleted_at IS
  NULL` (R-DM-33).
- `group_memberships`: FKs `group_id -> groups(id)`, `user_id -> users(principal_id)`,
  `added_by`, `removed_by -> principals(id)`, all ON DELETE RESTRICT; `expiry_reminded_at`,
  `expiry_reported_at`; partial unique index `(group_id, user_id) WHERE removed_at IS NULL`; index
  `(user_id) WHERE removed_at IS NULL`; index `(expires_at) WHERE removed_at IS NULL AND
  expires_at IS NOT NULL`.
- `grants`: the checks of 03, written as named constraints:
  `grants_subject_kind_check`, `grants_target_kind_check` (`instance`, `space`, `channel`,
  `host`), `grants_level_check` (`level` is nullable: NULL exactly for `target_kind = 'host'`,
  check `(target_kind = 'host') = (level IS NULL)`),
  `grants_capabilities_check` (`capabilities <@ ARRAY['approve_actions','upload','export',
  'manage','terminal']::text[]`), `grants_subject_shape_check` (exactly one of
  `subject_principal_id`, `subject_group_id`, matching `subject_kind`), `grants_target_shape_check`
  (`target_space_id` set exactly for `space`, `target_channel_id` set exactly for `channel`,
  `target_host_id` set exactly for `host`, all null for `instance`), `grants_bot_limits_check`
  (`subject_kind NOT IN ('agent','app') OR (level <> 'trigger' AND capabilities <@
  ARRAY['upload']::text[])`), `grants_host_shape_check` (`target_kind <> 'host' OR (capabilities =
  ARRAY['terminal'] AND subject_kind IN ('user','group'))`) and `grants_terminal_only_on_host_check`
  (`target_kind = 'host' OR NOT ('terminal' = ANY(capabilities))`) (03 R-DM-35);
  `capabilities text[] NOT NULL DEFAULT '{}'`; `reason text NOT NULL`; FKs
  `subject_principal_id`, `granted_by`, `revoked_by -> principals(id)`, `subject_group_id ->
  groups(id)`, all ON DELETE RESTRICT; the six partial indexes of 03 (`subject_principal_id`,
  `subject_group_id`, `target_space_id`, `target_channel_id`, `target_host_id` each `WHERE
  revoked_at IS NULL`, and `(expires_at) WHERE revoked_at IS NULL AND expires_at IS NOT NULL`). No
  unique constraint (03).

**Deferred target foreign keys.** `spaces` and `channels` do not exist before M1-52 and M1-53,
`hosts` not before M2-39. Create `target_space_id`, `target_channel_id` and `target_host_id` as
plain `uuid` columns. The migration carries a comment that names M1-52 (`grants.target_space_id ->
spaces(id) ON DELETE RESTRICT`), M1-53 (`grants.target_channel_id -> channels(id) ON DELETE
RESTRICT`) and M2-39 (`grants.target_host_id -> hosts(id) ON DELETE RESTRICT`); the README repeats
it. Until then no code path creates space, channel or host grants, because no such target exists.

**Immutability trigger** `authz_grants_immutable` (BEFORE UPDATE ON grants, SQLSTATE `23514`):
allowed are only (a) setting `revoked_at`, `revoked_by` and `revoke_reason` when `OLD.revoked_at
IS NULL`, and (b) setting `expiry_reminded_at` or `expiry_reported_at` when it was null (03
`grants`). Every other change, including a
second revocation and any change of level, capabilities, subject, target, expiry, reason or
grantor, raises. Deletion is not blocked in the database (retention jobs of M7 may need it), but
the repository has no delete query. Reason: R-DM-09 says grants are history; the database is the
last line against a code path that updates in place.

**Subject kind versus principal kind.** The database cannot check that `subject_principal_id`
has the kind named in `subject_kind` without reading an identity table. Do not add such a
trigger. The use case (M1-48) validates it through `PrincipalSource`, and M1-28 caps level and
capabilities by the principal's real kind, so a wrong `subject_kind` can never widen rights.

**Domain types and functions.**

```go
type Level uint8 // LevelNone < LevelRead < LevelWrite < LevelTrigger
func ParseLevel(s string) (Level, error)           // "read", "write", "trigger"
func (l Level) String() string
func MaxLevel(a, b Level) Level
func MinLevel(a, b Level) Level

type Capability uint8 // CapApproveActions, CapUpload, CapExport, CapManage, CapTerminal
type Caps uint8       // bit set
func ParseCaps(names []string) (Caps, error)       // rejects unknown names; duplicates collapse
func (c Caps) Has(x Capability) bool
func (c Caps) Union(o Caps) Caps
func (c Caps) Intersect(o Caps) Caps
func (c Caps) SubsetOf(o Caps) bool
func (c Caps) Names() []string                      // sorted, for the text[] column

type SubjectKind string // "user", "group", "agent", "app"
type TargetKind  string // "instance", "space", "channel", "host"
type Subject struct { Kind SubjectKind; PrincipalID, GroupID ids.ID }
type Target  struct { Kind TargetKind;  SpaceID, ChannelID, HostID ids.ID }

type Grant struct {
    ID ids.ID; Subject Subject; Target Target; Level Level; Caps Caps
    ExpiresAt time.Time        // zero: no expiry
    ExpiryRemindedAt time.Time // zero: not reminded
    Reason string; GrantedBy ids.ID; GrantedAt time.Time
    RevokedAt time.Time        // zero: not revoked
    RevokedBy ids.ID; RevokeReason string
}
func NewGrant(id ids.ID, s Subject, tg Target, l Level, c Caps, expiresAt time.Time,
    reason string, by ids.ID, at time.Time) (Grant, error)
// NewGrant checks shape (subject and target ids match their kinds), level not None except on a
// host target, where level is None and caps are exactly {terminal} for user or group subjects
// (03 R-DM-35), terminal only on a host target, a non-blank reason and expiresAt zero or after at. Errors: kernel/errs bad_request
// validation_failed with a field (/subject, /target, /level, /expires_at, /reason).
// It does not judge the bot limits (agent and app subjects: level at most write, caps
// subset of {upload}): CheckCreate decides them with reason subject_kind_not_allowed, and
// grants_bot_limits_check in the database is the last line.

type Group struct { ID ids.ID; Name, Description string; Source string; ExternalRef string;
    CreatedBy ids.ID; CreatedAt, DeletedAt time.Time }
type Membership struct { ID, GroupID, UserID, AddedBy ids.ID; AddedAt time.Time;
    ExpiresAt, ExpiryRemindedAt, RemovedAt time.Time; RemovedBy ids.ID }

func (g Grant) InEffect(t time.Time) bool
    // RevokedAt zero AND (ExpiresAt zero OR ExpiresAt.After(t))
func (m Membership) InEffect(group Group, t time.Time) bool
    // RemovedAt zero AND group.DeletedAt zero AND (ExpiresAt zero OR ExpiresAt.After(t))
func NextExpiry(t time.Time, expiries ...time.Time) time.Time
    // smallest non-zero value strictly after t; zero when none (07 section 7.2 step 7)
```

Subject activity (07 section 2.5, third bullet) and app installation state (fifth bullet) are
not part of these functions: the principal's own status is checked by `authorize` before any
grant is read (07 SR7), and a grant to another subject never acts for the caller.

**Delegation** (07 section 2.6, R-AZ-05):

```go
type Delegator struct {
    IsOwner bool  // effective owner (active, in a session; decided by M1-28)
    Level   Level // delegator's effective level on the target of the grant
    Caps    Caps  // delegator's effective capabilities there
}
type GrantShape struct { SubjectKind SubjectKind; TargetKind TargetKind; Level Level; Caps Caps }
func CheckCreate(d Delegator, g GrantShape) Reason // "" means allowed
func CheckRevoke(d Delegator, existing GrantShape) Reason
```

Order of checks in `CheckCreate`, first failing rule wins:

1. subject `agent` or `app` with level `trigger` or capabilities beyond `upload`:
   `subject_kind_not_allowed` (applies to owners too; AZ-M31).
2. not owner and subject `agent` or `app`: `subject_kind_not_allowed`.
3. not owner and target `instance` or `host`: `owner_only` (05 `grant.create`: on a host only
   owners).
4. not owner and `g.Level > d.Level`: `delegation_exceeds_own_level`.
5. not owner and `g.Caps` not a subset of `d.Caps`: `delegation_exceeds_own_capabilities`.

`CheckRevoke`: owner always allowed; otherwise rules 2 to 5 applied to the existing grant
("only what one could have created", 07 section 2.6 point 3; AZ-M39).

`Reason` is `type Reason string` in the domain with the constants `ReasonSubjectKindNotAllowed`,
`ReasonOwnerOnly`, `ReasonDelegationExceedsOwnLevel`, `ReasonDelegationExceedsOwnCapabilities`;
values equal the codes of 07 section 4.1. M1-28 adds the remaining reasons.

**Outgoing port** (`internal/authz/app/store.go`); "active" means not revoked or removed; time
filtering stays in the domain so one clock decides:

```go
type Store interface {
    InsertGroup(ctx context.Context, g domain.Group) error
    GetGroup(ctx context.Context, id ids.ID) (domain.Group, bool, error)
    SoftDeleteGroup(ctx context.Context, id ids.ID, at time.Time) (bool, error)
    InsertMembership(ctx context.Context, m domain.Membership) error
    RemoveMembership(ctx context.Context, id, by ids.ID, at time.Time) (bool, error)
    ActiveMembershipsOfUser(ctx context.Context, userID ids.ID) ([]domain.MembershipWithGroup, error)
    InsertGrant(ctx context.Context, g domain.Grant) error
    GetGrant(ctx context.Context, id ids.ID) (domain.Grant, bool, error)
    RevokeGrant(ctx context.Context, id, by ids.ID, reason string, at time.Time) (domain.Grant, bool, error)
    ActiveGrantsOfPrincipal(ctx context.Context, principalID ids.ID) ([]domain.Grant, error)
    ActiveGrantsOfGroups(ctx context.Context, groupIDs []ids.ID) ([]domain.Grant, error)
}
```

`MembershipWithGroup` pairs a membership with its group row (including `DeletedAt`), read in one
query. Results are ordered by `id` so tests are deterministic. Reads join the ambient
transaction when present. Write methods carry doc comments stating that they must run inside a
unit of work marked as a rights change (M1-29 enforces this).

**Error mapping.** Unique violation on the group name index or the active membership index:
kind `conflict`, code `conflict`, field `/name` or `/user_id`. Check, trigger and foreign key
violations: kind `bad_request`, code `validation_failed`, field from the constraint name. Never
put SQL text or values into an error.

**Test data for foreign keys.** The integration tests need rows in `principals` and `users`.
Insert them with plain SQL inside the test files only (test files are excluded from the
architecture lint and from the table-ownership check of `queries/*.sql`); do not import identity
adapters.

## Tests (write them first)

1. `internal/authz/domain` (pure, table driven):
    - `TestLevelOrderAndParse`, `TestCapsSetOperations` (including duplicates collapsing and an
      unknown name failing).
    - `TestGrantInEffect`: revoked; expires one second after `t`; expires exactly at `t`
      (invalid); expired; no expiry.
    - `TestMembershipInEffect`: removed; group deleted; expiry exactly at `t` (invalid); valid.
    - `TestNextExpiry`: none, past only, mixed, equal to `t` ignored.
    - `TestNewGrantShape`: every invalid shape and blank reason gives `validation_failed` with
      the right field.
    - `TestCheckCreate` and `TestCheckRevoke` tables covering each rule and its order, named
      after the 07 rows they mirror: `AZM31_OwnerTriggerToAgent`, `AZM37_WriteManageGivesTrigger`,
      `AZM38_CapabilityNotOwned`, `AZM39_RevokeStrongerGrant`, plus owner on instance target,
      non-owner on instance target, non-owner to app subject, passing manage delegation of
      `manage` itself.
2. `internal/authz/adapters/postgres` (integration, PostgreSQL 18 through `pgtest`):
    - migration applies after the identity migration and a second `up` changes nothing;
    - `TestGrantChecks`: agent with `trigger`, app with `manage`, agent with `upload` (passes),
      unknown capability, user grant with group id set, space target without space id, instance
      target with channel id, host target with a level, host target with `upload`, `terminal` on a
      space, host grant to an agent: each rejected or accepted as 03 says (the host rows use a
      random `target_host_id`, the foreign key comes with M2-39);
    - `TestGrantImmutable`: update of level fails; revocation once passes; second revocation
      fails; reminder once passes, twice fails;
    - `TestGroupMembersArePeople`: membership for an agent principal fails (no `users` row);
    - `TestActiveMembershipUnique` and `TestGroupNameUniqueAmongLive` (a deleted group frees its
      name);
    - `TestStoreRoundTrip`: insert, get, revoke, active lists per principal and per groups,
      memberships with group rows; ordering by id.

Commands that must pass:

```sh
cd services/server && go test -race ./internal/authz/...
make migrations-lint
make codegen-check
make arch
make check
```

## Acceptance criteria

- [ ] The three tables match 03 section 4.3; the only deviations are the deferred target foreign
      keys and the deferred FK `groups.provider_id` (M1-36), documented in the migration and the
      README.
- [ ] The database rejects `trigger` and capabilities beyond `upload` for agent and app
      subjects.
- [ ] Grants cannot be changed in place; revocation and the reminder can be set exactly once.
- [ ] Group memberships accept only people.
- [ ] Validity functions treat `expires_at = t` as invalid; all table tests are green.
- [ ] Delegation functions implement R-AZ-05 with the documented order and reasons.
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
new test before the code), what is not proven, and follow-ups that could not be done in scope with
the reason. Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
