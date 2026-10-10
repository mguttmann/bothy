# M1-29: Rights changes, access.changed and invariants I1 and SR8

Status: ready | Depends on: M1-16, M1-28 | Etappe: M1

## Goal

After this ticket every rights change in the server runs through one transactional path owned by `authz`: the rights version is locked and raised first, the visible targets of every affected principal are compared before and after the change, every principal with a difference receives the personal event `access.changed`, and invariant I1 and protection rule SR8 are verified in the same transaction before the commit. A violating change leaves nothing behind except one deny entry in the audit log, written in its own transaction. The `authz` Admin port gains the use cases for groups, memberships and grants on top of this path. Every later ticket that changes owners, principal status, operators or the backend policy (M1-31, M1-45, M1-51, M1-54) only declares its change and inherits these guarantees, which is why this path must be complete and exact now.

## Read first

- `docs/PLAN.md`: section 3 (Arbeitsweise je Ticket), section 7 (M1) with Voraussetzungen aus dem Kern-Vertrag and Bekannte Widersprueche, the rows M1-28, M1-29, M1-30, M1-31, M1-48, M1-51, M1-54.
- `docs/spec/07-rechte.md`: 2.1, 2.5 (Gueltigkeit einer Freigabe), 2.6 (Delegation mit manage), 2.7 (Owner), 2.9 (Agenten als Prinzipale), 4.1 (Entscheidungen und Gruende), 4.2 (Ablauf), 4.4 (Invarianten in der Transaktion), 5 (SR1, SR5, SR7, SR8), 7.2, 7.3, 8.2 (Sofortige Wirkung bei Entzug), 11.1 (AZ-M21 to AZ-M26, AZ-M31, AZ-M36 to AZ-M39), 11.2 (AZ-P7, AZ-P8), 12, 13.
- `docs/spec/03-datenmodell.md`: 1 (principles 10 to 13), 4.1 `sequence_counters`, 4.2 `principals` and `owners`, 4.3 `groups`, `group_memberships`, `grants`, 4.4 `agents` (fields `billing_mode`, `credential_holder_id`), 6.3 (Sperrreihenfolge), 6.4 (I1-Pruefung), 8 (R-DM-07, R-DM-08, R-DM-09, R-DM-19, R-DM-20).
- `docs/spec/04-sync-protokoll.md`: 2.2 (Umschlag), 3 (Globale, commit-geordnete Sequenz), 8.1, 8.2 (Was beim Aendern passiert), 16.4.
- `docs/spec/05-aktionen.md`: rules 1.6, 1.7, 1.10; 2.3, 2.4, 2.5; table 4.2 (`owner.add`, `owner.remove`), 4.3 (`user.disable`, `user.enable`), 4.4 (every group and grant action), 4.5 (`space.operator.set`, `channel.operator.set`), 4.10 (`agent.enable`, `agent.billing_mode.set`, `agent.billing_mode.set_all`, `agent.credential_holder.set`); 6 (`access.changed`, `grant.changed`, `group.changed`); 8 (R-AZ-05, R-AZ-13, R-AZ-14); 9 test 5. The machine-readable catalogue is `docs/spec/actions.yaml` (M1-02).
- `docs/spec/10-backend-richtlinie.md`: 3 (Invarianten), 5.1 to 5.4, 15 (BR-01 to BR-03).
- `docs/spec/02-architektur.md`: 4.1, 4.3 (row `authz`), 4.4, 4.5, 5, 8.1 (Ablauf), 8.2, 8.3, 9.2, 11.3, 14.1.
- `docs/spec/06-api.md`: 4 point 5, 5 (Fehler).
- `docs/api/openapi.yaml`: schemas `AccessChangedPayload`, `AccessChangedEvent`, `AccessTarget`, `AccessTargetKind`, `Problem`, `InvariantViolation`, response `Conflict`. `docs/api/asyncapi.yaml`: message `event.access.changed`.
- ADRs: 0009, 0017, 0018, 0021, 0024, 0052, 0053.
- Merged code you build on. Read it before writing anything; where its names differ from this brief only in naming, the merged code wins: `internal/kernel/txn` and the unit of work in `internal/platform/pg` (M1-13), `instance.Counters` (M1-15), `kernel/eventlog.Appender` and its implementation in `internal/sync` (M1-16), `kernel/auditing.Recorder` (M1-24), `authz.PrincipalSource` and its identity adapter (M1-26), the grant store and the pure validity and delegation functions in `internal/authz` (M1-27), snapshot, cache, `authorize`, the fake sources and the source ports (M1-28).
- `docs/quality/gates.md`, `CONTRIBUTING.md`, `docs/quality/red-team-gate.md`.

## Fixed requirements that apply

1. No MVP. Built from scratch directly in its final state; etappen are only the build order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface has a docstring or doc comment (purpose, parameters, return value, errors, example where useful); comments explain why, not what. API documentation is generated from code (OpenAPI from the routes, dartdoc, Python API docs in the docs site).
4. Documentation in the repo: README per module/service, an ADR for every architecture decision, a CHANGELOG entry per release, docs site kept current. A change without matching documentation is not done.
5. Quality gates (SonarQube, linters, type checks, security scanners, test coverage) block the merge.
6. Test first: every behaviour below gets a test that is red before the code exists (PLAN section 1, principle 4); keep the red output for the report.
7. No en dash or em dash in any file you write. White Label: example people are Alex, Sam and Robin, domains `example.com`; nothing instance specific.
8. Configuration values only through named keys listed in `docs/operations/configuration.md`. This ticket adds none.
9. I1 (Baseline section 5; 07 SR1; 03 section 6.4; 10 section 5): for every agent with `billing_mode = 'personal'` the set of active principals with effective `trigger` on any target the agent serves (owners, group members and inheritance included, expired rows excluded) is exactly `{credential_holder_id}`. It is checked in the transaction of every action of 05 rule 1.6. A violation is `409 invariant_violation` with code `i1_violation` and the hint to switch to organization first.
10. SR8 (07; 03 `owners`): no transaction may leave zero active owners; code `last_owner`.
11. Lock order (03 section 6.3): `permission_version` first, then domain rows in ascending `id`, then `event_seq`, then `audit_seq`.
12. Every action with column Rechte = ja raises `permission_version` and writes `access.changed` for the affected principals (05 rule 1.7; 04 section 8.2).
13. A rejected change leaves no partial change; the deny audit entry is written in its own transaction (07 section 4.4 step 5; 02 section 8.1).
14. No read right gives `404`, never `403` (B7, R-AZ-01); the Admin use cases load first and return the same error for missing and invisible resources (02 section 9.2).

## Scope

### 1. Rights-change hook (phases 1 and 3.1 of the unit of work)

Files: `services/server/internal/authz/app/rightschange.go` with tests, registration in `services/server/internal/wire`.

The unit of work from M1-13 accepts a rights-change option (02 section 8.1). Implement the `authz` hooks for it. If the merged kernel type lacks one of these fields, add it in `internal/kernel/txn` (kernel types stay free of module types):

```go
// RightsChange declares a rights change to the unit of work (02 section 8.1).
type RightsChange struct {
    Action     authorize.Action      // catalogue action that causes the change
    Resource   authorize.ResourceRef // kind and id written into the deny audit entry
    Principals []ids.ID              // principals whose visible targets may change
}
```

- `Begin(ctx, rc)`, phase 1, the first statement of the transaction: lock and increment `sequence_counters('permission_version')` through `instance.Counters`, keep the new value in the transaction state, and compute the before snapshot of every principal in `rc.Principals` from the sources inside the ambient transaction. Never use the snapshot cache here; it is keyed by committed versions and would hide uncommitted state.
- Phase 2 belongs to the use case. A use case that learns about further affected principals during phase 2 calls `AddAffected(ctx, principals...)` on the transaction state before it writes rows that concern them; their before snapshot is computed at that moment.
- `BeforeCommit(ctx)`, phase 3.1: call `VerifyInvariants`. On a violation return the error (the unit of work rolls back) and schedule the deny entry (Design notes). Otherwise compute the after snapshot of each affected principal, the difference, append one `access.changed` per principal with a non-empty difference through `kernel/eventlog.Appender`, and publish the in-transaction event `authorize.AccessChanged` once with all differences.

### 2. VerifyInvariants

Method on the `authz` application service: `VerifyInvariants(ctx context.Context, t time.Time) (Violations, error)`. It is not part of the cross-module port; M1-49 (preview) calls it from inside `authz`.

- SR8: the set of active owners (owner row with `revoked_at IS NULL`, principal `status = 'active'`) is not empty, else violation `last_owner`.
- I1: for every active agent with `billing_mode = 'personal'` from `AgentPolicySource`, compute `TriggerSet(A, t)` and compare it with `{credential_holder_id}`; every difference is a conflict with the extra principal IDs (and an empty set or a missing holder also counts as a violation).
- All reads happen inside the ambient transaction; `t` is read once from `kernel/clock` per call.

### 3. Pure domain functions (`internal/authz/domain`)

- `TriggerSet(w World, agent AgentPolicy, t time.Time) []ids.ID`: active owners; plus active users with a valid `trigger` grant on the instance, on a served space (the space of a served channel, or a space whose default operator is the agent, 03 section 6.4 point 3) or on a served channel; plus active members (membership valid at `t`, group not deleted) of groups that hold such a grant. Validity exactly as 07 section 2.5: at `expires_at = t` a row is already invalid. Agents and apps never appear (R-DM-08). The result is sorted and unique.
- `DiffVisible(before, after Snapshot) PrincipalDiff`: spaces compare by visibility only (`added`, `removed`, never `changed`, no `level`); channels compare `(level, capabilities)`: `added` carries `level` and `capabilities`, `removed` carries `kind` and `id` only, `changed` appears when level or capabilities differ and carries `level`, `previous_level` and `capabilities` (the new set). Lists are sorted by `(kind, id)`.
- The served targets are not computed in `authz`; they come from `StructureSource`.

### 4. Source ports (root package `internal/authz`)

Add the methods that the merged ports from M1-28 lack. Every method reads inside the ambient transaction (the unit of work keeps the transaction in the context).

- `PrincipalSource.ActiveOwnerIDs(ctx) ([]ids.ID, error)`. Implement it in the identity adapter from M1-26 with one sqlc query on `owners` joined with `principals`. This adapter method, its query and its test are the only change allowed in `internal/identity`.
- `StructureSource.ServedTargets(ctx, agentID) ([]ChannelRef, error)` with `ChannelRef{ChannelID, SpaceID}`: channels whose effective operator is the agent and that are not archived, plus channels holding at least one thread with `kind = 'thread'` and `operator_agent_id` equal to the agent, open or closed (10 sections 5.2 and 5.3, BR-02). Guest channels never count. Declaration only; `chat` implements it in M1-54.
- `AgentPolicySource.PersonalAgents(ctx) ([]AgentPolicy, error)` with `AgentPolicy{AgentID, CredentialHolderID}` for active agents with `billing_mode = 'personal'`. Declaration only; `agents` implements it in M1-51.
- Production wiring until M1-51 and M1-54 uses the empty source implementations from M1-28. If they do not exist, add `authz.EmptyAgentPolicySource` and `authz.EmptyStructureSource` with a doc comment naming the ticket that replaces them.

### 5. Admin port use cases

Port `Admin` in `internal/authz`, implementation in `internal/authz/app`, persistence in `internal/authz/adapters/postgres` with sqlc. Each method takes the caller as `actor.Actor` (`kernel/actor`), loads the resource, calls `authorize` with the action and a resource descriptor before any write, runs in one unit of work, and writes the allow audit entry through `auditing.Recorder` inside the transaction (column Audit = ja in 05 table 4.4). All of them require step-up through `authorize` (05 table 4.4).

| Method | Action | Rights change, affected | Rules |
|---|---|---|---|
| `CreateGroup(ctx, actor, CreateGroupCmd{Name, Description})` | `group.create` | no | name not empty, no control or bidi characters (SEC-IN-4), unique case-insensitive among non-deleted groups else `409 conflict`; `source = 'local'` |
| `UpdateGroup(ctx, actor, groupID, UpdateGroupCmd{Name, Description})` (both optional) | `group.update` | no | same name rules; a group with `source = 'oidc_claim'` keeps its name in sync with the claim, so renaming it is `409 conflict` |
| `DeleteGroup(ctx, actor, groupID, reason)` | `group.delete` | yes, all members with `removed_at IS NULL` | sets `deleted_at`; revokes every active grant of the group (`revoked_at`, `revoked_by` = actor, `revoke_reason` = reason) in ascending `id` |
| `AddGroupMember(ctx, actor, groupID, AddMemberCmd{UserID, ExpiresAt})` (`ExpiresAt` optional) | `group.member.add` | yes, the user | subject must be a principal of kind `user` that is not `deleted`, else `404`; `expires_at` must lie after now, else `400 validation_failed`; an existing active membership is `409 conflict` |
| `RemoveGroupMember(ctx, actor, groupID, userID)` | `group.member.remove` | yes, the user | never a member is `404`; a membership that existed but is already removed or expired succeeds without change (06 2 "Ohne Wirkung": no rights version, audit with `no_change: true`) |
| `CreateGrant(ctx, actor, CreateGrantCmd{SubjectKind, SubjectID, TargetKind, TargetID, Level, Capabilities, ExpiresAt, Reason})` | `grant.create` | yes, the subject principal, or all members with `removed_at IS NULL` for a group | reason required; `expires_at` after now; target must exist (else `404`) and not be archived (else `409 conflict`); subject kind and delegation limits are evaluated by the guard of `authorize` (07 section 2.6, AZ-M31, AZ-M37, AZ-M38); the row is never updated later (R-DM-09) |
| `RevokeGrant(ctx, actor, grantID, reason)` | `grant.revoke` | yes, as for `CreateGrant` | an already revoked grant succeeds without change (06 2 "Ohne Wirkung": no rights version, no event, audit with `no_change: true`); revoking is limited by 07 section 2.6 point 3 (AZ-M39) |

If the merged `authorize` from M1-28 has no guard for `grant.create` and `grant.revoke` that applies 07 section 2.6 with the pure delegation functions from M1-27, add the guard in `internal/authz` in this ticket.

### 6. Errors and the problem body

- `internal/authz/errors.go`: type `InvariantError` that unwraps to the `kernel/errs` kind `invariant_violation` with code `i1_violation` or `last_owner`, a hint (`Switch the agent to organization first.` resp. `Appoint another owner first.`) and, for I1, `Conflicts []I1Conflict{AgentID, CredentialHolderID, PrincipalIDs}` sorted by `AgentID`, principal IDs sorted. IDs only, never names (10 section 5.4).
- `internal/kernel/errs`: if missing, add `type ProblemExtender interface { ProblemExtensions() map[string]any }`. `InvariantError` implements it and returns the member `conflicts` for I1.
- `internal/platform/httpserver`: the problem writer from M1-18 merges `ProblemExtensions()` into the RFC 9457 body; extension keys never override the standard members, and no value is JSON `null`.
- `docs/api/openapi.yaml` already has `Problem.conflicts` (array of `InvariantViolation`, present with `i1_violation`) and `InvariantViolation.credential_holder_id` (10 section 5.4). Regenerate the server code (M1-17 generator) if it predates them so that `make codegen-check` shows no drift.

### 7. In-transaction event and wiring

- `internal/kernel/authorize`: `AccessChanged{PermissionVersion int64; Action Action; Changes []PrincipalAccessChange}` with `PrincipalAccessChange{PrincipalID ids.ID; Added, Removed, Changed []AccessTarget}`, a Go mirror of `AccessChangedPayload`. If M1-25 or M1-28 created a placeholder, complete it.
- `internal/wire`: register the `authz` hooks for phase 1 and phase 3.1 as method values of the `authz` hook type (never closures defined in `internal/wire`; the registry derives the owner from the package, 02 "Ablauf") and `AccessChanged` as an event published by `authz`. There is no production subscriber in M1 (`runs` subscribes in M2, 02 section 4.3). Update the golden file of registrations from M1-13.

### 8. The `access.changed` event

Envelope per 04 section 2.2: `type = access.changed`, `schema_version = 1`, `audience = self`, `resource = {kind: principal, id: <principal>}`, `scope` an empty object, `actor` from the request (absent for changes without a principal), `payload` exactly `AccessChangedPayload` with `permission_version` equal to the value from phase 1 and the arrays `added`, `removed`, `changed` always present (empty arrays, never null). The envelope field `permission_version` is set by the sync hook of M1-16.

### 9. Documentation

- `services/server/internal/authz/README.md`: status, the Admin port, the source ports with the new methods, the hooks, the events, tables, actions, and a short code example of how another module declares a rights change.
- `docs/DECISIONS.md`: the rulings marked Ruling under Design notes.
- `services/server/CHANGELOG.md` under Unreleased.

## Out of scope

- HTTP routes for groups, members and grants, the events `grant.changed` and `group.changed`, tests with real authentication: M1-48.
- Expiry jobs (`grant.expire`, `group.member.expire` as real jobs) and reminders: M1-30. Here those actions are only exercised through the hook.
- `owner.add`, `owner.remove`, `user.enable`: M1-31. `user.disable`: M1-45. Operators: M1-54. Agent policy actions: M1-51. In this ticket they appear only as declared changes in tests with fake sources.
- The guided flow `resolve_i1`: M2-21.
- Implementations of `StructureSource.ServedTargets` (M1-54) and `AgentPolicySource.PersonalAgents` (M1-51).
- Who has access, preview and view as: M1-49. Cancelling runs on `AccessChanged`: M2.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-29-authz-invariants`, created from the base branch named by the launcher. One writer per worktree.
- Conventional Commits with the module as scope (for example `feat(authz): verify I1 and SR8 in the rights-change hook`) and a DCO sign-off on every commit (`git commit -s`). Squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope of this brief, except `docs/DECISIONS.md`, `services/server/CHANGELOG.md` and the generated files named here.
- Follow the host rules supplied by the launcher. Tests bind only `127.0.0.1` and port 0 (P-10). PostgreSQL runs as a throwaway container on a random port through `internal/platform/pg/pgtest` (P-09); without a Docker-compatible runtime the integration tests fail, they never skip.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets. No network calls except package downloads for pinned tools.
- No gate may be weakened, no threshold lowered, no test deleted or skipped.

## Design notes

- Ruling: `access.changed` is written only for an affected principal whose difference is non-empty. A change without visible effect (for example a backend policy switch) still raises the rights version, and every later event carries it in the envelope; an empty `access.changed` would only make clients reload for nothing.
- Ruling: I1 is checked for all active personal agents on every rights change. 10 section 5.1 names the minimum set per action; the superset cannot miss a case and costs one evaluation per personal agent, which I1 itself keeps rare. 03 section 6.4 already requires the check for shrinking changes.
- Before and after snapshots use the same functions as `authorize` (M1-28), fed from the in-transaction sources and without the cache. The cache stays correct because it is keyed by the committed `permission_version` (07 section 7.3).
- Deny audit entry: action `rc.Action`, `decision = 'deny'`, `deny_reason` = `i1_violation` or `last_owner`, `resource_kind` and `resource_id` from `rc.Resource`, actor, device and session from the request actor, `details` = `{violations: [{code, agent_id, credential_holder_id, principal_ids}]}` for I1 or `{code: last_owner}`, the request `correlation_id`. It is written through the detached path of `auditing.Recorder` after the rollback (M1-24). Aggregation per `audit.deny_aggregate_window` is the recorder's job; do not aggregate yourself.
- Concurrency: the lock on `permission_version` serializes all rights changes. In `READ COMMITTED` the second transaction gets the lock only after the first commit and then reads the committed state, so I1 needs no `SERIALIZABLE` (03 section 6.3). Retries after `40P01` or `55P03` are done by the unit of work (`database.tx_retry_max`); never retry inside the hook.
- The hook makes no network call and does no slow work; it runs inside the serialized commit section (02 section 8.5).
- Problem codes used: `409 i1_violation`, `409 last_owner`, `409 conflict`, `404 not_found`, `400 validation_failed`, `401 step_up_required`, `403 forbidden` with the reasons of 07 section 4.1 (`owner_only`, `subject_kind_not_allowed`, `delegation_exceeds_own_level`, `delegation_exceeds_own_capabilities`, `step_up_not_possible`). `validation_failed` is `400` as in 06 section 5, not `422`.
- Test names carry the test ID, for example `TestAZ_M21_GrantTriggerOnPersonalTargetRejected`; if M1-28 established another form, follow it.

## Tests (write them first)

Use the fake sources from M1-28 (extended by the new methods), a controllable clock (`kernel/clock`), actor values built in the test (owner in a session with fresh step-up, owner with old step-up, human with a personal token, agent), and PostgreSQL 18 from `pgtest` for everything that touches a transaction. The test world is the world of 07 section 11.1.

Unit tests in `internal/authz/domain`:

1. `TestTriggerSet_*` as a table: owners only; user grant on instance, space, channel; group grant with valid, expired exactly at `expires_at`, and removed membership; deleted group; disabled user; revoked grant; channel served only through a closed thread with fixed operator (shape of BR-02); agent and app subjects never included.
2. `TestTriggerSet_MatchesAuthorizeReference`: in seeded random worlds, `TriggerSet` equals the set of principals for which the effective-rights function of M1-28 yields `trigger` on a served channel, its space or the instance (same idea as AZ-P8).
3. `TestDiffVisible_*` as a table: added, removed, changed by level, changed by capabilities only, space visibility only, sorted output, empty difference.

Integration tests in `internal/authz/app` against PostgreSQL:

4. AZ-M21, AZ-M22, AZ-M23, AZ-M24, AZ-M25, AZ-M26 from 07 section 11.1 with a fake `AgentPolicySource` (Assistent personal, credential holder is the owner) and a fake `StructureSource`. M21 and M25 use the real Admin use cases. M22, M23, M24 and M26 use a test use case that changes the fake source inside the transaction and declares the rights change with the action of the row; M26 expects `last_owner`.
5. AZ-P7: one case per action of 05 rule 1.6, seventeen cases. Expanding actions (`grant.create`, `group.member.add`, `owner.add`, `space.operator.set`, `channel.operator.set`, `agent.billing_mode.set`, `agent.billing_mode.set_all`, `agent.credential_holder.set`, `agent.enable`, `user.enable`) get a change that violates I1 and expect `i1_violation`. Shrinking actions (`grant.revoke`, `grant.expire`, `group.member.remove`, `group.member.expire`, `group.delete`, `owner.remove`, `user.disable`) cannot break a valid world; their case starts from a world made invalid by a fixture written directly in SQL, asserts that a change which does not repair it is rejected (proof that the check runs, 03 section 6.4), and that a change which repairs it commits.
6. No partial change: after each rejection the contents of `groups`, `group_memberships`, `grants`, the value of `permission_version`, the rows of `event_log` and the allow audit rows are unchanged, and exactly one deny audit row with the fields from Design notes exists.
7. BR-03: two transactions that only together violate I1 (add Sam to group G; grant G `trigger` on a channel served by the personal agent) run concurrently, synchronized with a barrier: exactly one commits, the other fails with `i1_violation`, in both orders.
8. SR8 race: with two active owners, two concurrent declared owner removals (a test use case that updates `owners` and declares `owner.remove`) end with exactly one success and one `last_owner`.
9. `access.changed` content: revoking G-1 of Alex gives `removed` with space Support and its channels; a level change gives `changed` with `level` and `previous_level`; a capability-only change gives `changed` with `capabilities`; the payload validates against `AccessChangedPayload`, contains no null, and its `permission_version` equals the new counter value.
10. In-transaction delivery: a test subscriber to `AccessChanged` runs inside the transaction and its veto rolls everything back.
11. After the commit of a revocation, a new `Authorizer` instance with its own cache answers `not_found` for the revoked target.
12. Admin use cases: AZ-M31, AZ-M36 (step-up older than `auth.step_up_max_age`), AZ-M37, AZ-M38, AZ-M39 with actor values; `group.delete` revokes the group grants and members lose access; duplicate membership `409 conflict`; archived target `409 conflict`; a group the caller may not see `404`.
13. Audit: one allow entry per Admin use case with action, actor and resource; details contain no names.

HTTP test in `internal/platform/httpserver`:

14. A test route returning `InvariantError` gives `409`, `code = i1_violation`, `hint`, and `conflicts` valid against the updated schema, with no null anywhere.

Commands that must pass:

- `cd services/server && go test -race ./internal/authz/... ./internal/kernel/... ./internal/platform/httpserver/... ./internal/identity/...`
- `make codegen-check`, `make arch`, `make api-lint`, `make docs`
- `make check`

## Acceptance criteria

- [ ] AZ-P7 has seventeen cases, one per action of 05 rule 1.6, all green.
- [ ] AZ-M21 to AZ-M26 green; AZ-M31, AZ-M36 to AZ-M39 green at use-case level.
- [ ] BR-03 and the SR8 race green in both orders under `-race`.
- [ ] A rejected change leaves no row, no event and no counter change, and exactly one deny audit row.
- [ ] `access.changed` payloads validate against `AccessChangedPayload`; lists sorted; no null.
- [ ] The `TriggerSet` reference property test is green.
- [ ] The problem body of `i1_violation` carries `conflicts` valid against `openapi.yaml`; `make codegen-check` and `make api-lint` green.
- [ ] Golden file of in-transaction registrations updated and green.
- [ ] README, DECISIONS and CHANGELOG updated; every exported symbol has a doc comment.
- [ ] `make check` green.

## Definition of done

- Code, tests and documentation as listed in Scope; the module README is current (status, ports, tables, actions, events, hooks).
- A doc comment on every exported symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- An ADR for every architecture decision taken here (none expected; the rulings go to `docs/DECISIONS.md`).
- CHANGELOG entry under Unreleased in `services/server/CHANGELOG.md`.
- `make check` green on the final commit.
- Independent review of the diff (`make review` plus a code review by a fresh reviewer) and the red team of `docs/quality/red-team-gate.md` over the diff, with its verdict recorded in the report; the red team pays special attention to PLAN section 7 Red Team point 3 (no sequence of changes bypasses I1 or SR8, also in parallel). On `NEIN`: fix, add a test per finding, re-check the delta.
- Merge locally into the base branch only when the review is clean, the red team says `JA` and `make check` is green; then `git worktree remove`.

## Report

Write `{{REPORT_PATH}}`:

- first line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`;
- commits (sha and subject), files changed with line counts, test results with counts (zero skipped), the red output of the first run of each new test file, what is not proven, follow-ups that could not be done in scope with the reason.

Then write `{{DONE_PATH}}` with `exit=0 sha=<merge sha>` or `exit=1 <reason>`.
