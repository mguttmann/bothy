# M1-30: Expiry of grants and memberships

Status: ready | Depends on: M1-29 | Etappe: M1

## Goal

After this ticket two periodic jobs in `authz` make expiry visible: `authz.expiry_sweep` reports every expired grant and group membership exactly once with a raised rights version, `access.changed` for each affected principal, the management events and an audit entry with the system actions `grant.expire` and `group.member.expire`; `authz.expiry_reminder` sends the personal event `access.expiring` exactly once before an expiry. The access decision itself never depends on either job: `authorize` already treats a row as invalid from the second of its expiry (07 section 2.5), and this ticket proves that again with the jobs switched off. Clients need the events to clear their caches and, from M2 on, runs of people who lost `trigger` end through the same in-transaction event.

## Read first

- `docs/PLAN.md`: section 3, section 7 (M1), rows M1-27 to M1-31.
- `docs/spec/07-rechte.md`: 2.5 (Gueltigkeit einer Freigabe), 4.4, 8.2, 8.3 (Ablauf durch Zeit), 9.4 (Erinnerung vor Ablauf), 11.2 (AZ-P10), 12 (R-RE-09), 14 (configuration values).
- `docs/spec/03-datenmodell.md`: 4.3 `groups`, `group_memberships`, `grants` (including `expiry_reminded_at` and the expiry index), 6.3, 6.4.
- `docs/spec/04-sync-protokoll.md`: 2.2, 8.2.
- `docs/spec/05-aktionen.md`: rule 1.10 (system actions), table 4.4 (`grant.expire`, `group.member.expire` and the note below the table), 6 (`access.changed`, `access.expiring`, `grant.changed`, `group.changed`), 8 (R-AZ-15).
- `docs/spec/02-architektur.md`: 4.7, 8.1 to 8.3, 12.1 to 12.4 (jobs, queues, job catalogue, job rules).
- `docs/api/openapi.yaml`: schemas `AccessExpiringPayload`, `AccessTarget`, `AccessTargetKind`, `GrantChangedPayload`, `GroupChangedPayload`, `Grant`, `Group`, `EventEnvelope`.
- ADRs: 0018, 0021, 0009, 0052, 0053.
- `docs/security/threat-model.md`: T-AZ-03, T-AZ-04.
- Merged code: `internal/platform/jobs` (M1-14, periodic jobs and handler interface), the rights-change hook, `VerifyInvariants` and the Admin port (M1-29), the grant store (M1-27), the snapshot builder (M1-28), `auditing.Recorder` (M1-24), `kernel/eventlog.Appender` (M1-16). Their names win where they differ from this brief only in naming.
- `docs/quality/gates.md`, `CONTRIBUTING.md`.

## Fixed requirements that apply

1. No MVP. Built from scratch directly in its final state; etappen are only the build order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface has a docstring or doc comment (purpose, parameters, return value, errors, example where useful); comments explain why, not what. API documentation is generated from code (OpenAPI from the routes, dartdoc, Python API docs in the docs site).
4. Documentation in the repo: README per module/service, an ADR for every architecture decision, a CHANGELOG entry per release, docs site kept current. A change without matching documentation is not done.
5. Quality gates (SonarQube, linters, type checks, security scanners, test coverage) block the merge.
6. Test first: red before green, red output kept for the report.
7. No en dash or em dash in any file; White Label.
8. Configuration values only through named keys listed in `docs/operations/configuration.md`; the one new key of this ticket is added there through its chapter table (Scope 6).
9. Expiry is checked at every `authorize` call; the jobs only report (07 principle 3, section 8.3, R-RE-09). With `authz.expiry_sweep` switched off an expired grant still has no effect (AZ-P10).
10. The sweep writes, in one transaction per batch: raised rights version, `access.changed`, audit entries with the system actions `grant.expire` and `group.member.expire` (07 section 8.3, R-AZ-15); invariants are verified as for every rights change (05 rule 1.6).
11. The reminder runs with the period `authz.expiry_sweep_interval`, finds rows expiring within `authz.expiry_reminder_lead` and not yet reminded, and writes `access.expiring` exactly once (07 section 9.4); no e-mail (Baseline section 4).
12. Job arguments contain only IDs, never content or secrets (02 section 12.1); these jobs need no arguments.
13. Lock order of 03 section 6.3.

## Scope

### 0. Precondition (check first)

`docs/spec/03-datenmodell.md` must define the column `expiry_reported_at timestamptz NULL` on `grants` and on `group_memberships` (set by `authz.expiry_sweep` when it has reported the expiry) and an index on `group_memberships (expires_at) WHERE removed_at IS NULL AND expires_at IS NOT NULL`. These come from the review of this brief: without a durable marker the sweep cannot report every expiry exactly once after the worker was down. If 03 uses other names, use them. If 03 has no such marker, stop with `Verdict: BLOCKED core contract 03 lacks an expiry report marker` (P-11) and build nothing.

### 1. Migration

`services/server/migrations/<next number>_authz_expiry.sql` (goose, next free number, never edit an existing migration): add `expiry_reminded_at` to `grants` and `group_memberships` only if the migration of M1-27 did not create it; add `expiry_reported_at` to both; create the membership index from the precondition and the grant index `(expires_at) WHERE revoked_at IS NULL AND expires_at IS NOT NULL` if M1-27 did not create it. `make migrations-lint` stays green; regenerate sqlc (`make codegen-check` without drift).

### 2. Hook extension

In `internal/authz/app/rightschange.go` (M1-29) add `AddAffectedAssumingValid(ctx, principals []ids.ID, assume AssumeValid)` with `AssumeValid{GrantIDs, MembershipIDs []ids.ID}`: the before snapshot of these principals is computed with the listed rows treated as not expired. Extend the snapshot builder of M1-28 with this option inside `internal/authz` only; it never leaves the module. Reason: at sweep time the expired rows are already invalid for `authorize`, so a plain before snapshot would equal the after snapshot and the difference would be empty.

### 3. Job `authz.expiry_sweep`

Handler in `internal/authz/app/expiry.go`, registered through `internal/platform/jobs` in `internal/wire` for the roles `worker` and `all`, queue `maintenance`, period `authz.expiry_sweep_interval`. Algorithm per run, with `t` read once per batch from `kernel/clock`:

1. Grants. Open a unit of work with `RightsChange{Action: grant.expire}` (phase 1 locks `permission_version`). Select `grants WHERE revoked_at IS NULL AND expires_at IS NOT NULL AND expires_at <= t AND expiry_reported_at IS NULL ORDER BY id LIMIT authz.expiry_sweep_batch_max FOR UPDATE`. If none: return a sentinel error so the transaction rolls back and the rights version does not move; treat the sentinel as success. Otherwise:
   - affected principals: the subject principal for subjects `user`, `agent`, `app`; for a group subject all members with `removed_at IS NULL` of a non-deleted group; register them with `AddAffectedAssumingValid` before any write;
   - set `expiry_reported_at = t` on the selected rows;
   - append `grant.changed` per row (envelope `resource = {kind: grant, id}`, `scope` with `space_id` and, for channel targets, `channel_id` resolved through `StructureSource`; payload `GrantChangedPayload{grant: Grant, change: expired}`);
   - record one audit entry per row: action `grant.expire`, `decision = 'allow'`, no actor (system), `resource_kind = 'grant'`, `resource_id` = grant id, `details = {subject_kind, subject_id, target_kind, target_id (absent for instance), expires_at}`;
   - phase 3.1 (M1-29) verifies I1 and SR8, computes the differences and writes `access.changed`.
   Repeat while a full batch was found.
2. Memberships. Same with `RightsChange{Action: group.member.expire}` and `group_memberships WHERE removed_at IS NULL AND expires_at IS NOT NULL AND expires_at <= t AND expiry_reported_at IS NULL`; affected principal is the member; event `group.changed` (envelope `resource = {kind: group, id}`, empty scope, payload `GroupChangedPayload{group: Group, change: member_expired, user_id}`); audit action `group.member.expire`, `resource_kind = 'group'`, `resource_id` = group id, `details = {membership_id, user_id, expires_at}`. Memberships of deleted groups are marked and audited like others; their difference is empty, so no `access.changed` follows.
3. An invariant violation inside the sweep (only possible in a world that was already invalid) aborts that batch, is logged with the action and the agent IDs, and the handler returns the error so that River retries; nothing is marked.

### 4. Job `authz.expiry_reminder`

Handler in the same file, same roles, queue `maintenance`, period `authz.expiry_sweep_interval` (07 section 9.4). Per batch, in one unit of work without rights change, `t` read once:

1. Select `grants WHERE revoked_at IS NULL AND expires_at > t AND expires_at <= t + authz.expiry_reminder_lead AND expiry_reminded_at IS NULL ORDER BY id LIMIT authz.expiry_sweep_batch_max FOR UPDATE`; skip grants of deleted groups.
2. Recipients per row (Ruling, see Design notes), deduplicated, only active principals: grants to `user`, `agent` or `app`: the subject and `granted_by`; grants to a group: all active owners and `granted_by`; memberships: the member and `added_by`.
3. Append one `access.expiring` per recipient: `audience = self`, `resource = {kind: principal, id: <recipient>}`, empty scope, no actor, payload `AccessExpiringPayload`: `kind` (`grant` or `group_membership`), `grant_id` for grants, `group_id` for group grants and memberships, `subject_id` (subject principal, group id for a group grant, member for a membership), `target` (`kind`, `id`, `level`, `capabilities`) only for space and channel targets, `expires_at`.
4. Set `expiry_reminded_at = t`.
5. Same for `group_memberships WHERE removed_at IS NULL AND expires_at > t AND expires_at <= t + lead AND expiry_reminded_at IS NULL` of non-deleted groups.
6. Repeat while a full batch was found. Reminders write no audit entry: they change no right and the catalogue has no action for them.

### 5. Specification first (first commit)

- `docs/api/openapi.yaml`: add the value `member_expired` to the `change` enum of `GroupChangedPayload` (description: membership ended by expiry, reported by `authz.expiry_sweep`). Additive; clients ignore unknown values (06 section 2). Regenerate (`make codegen-check` without drift), `make api-lint`.
- `docs/spec/07-rechte.md` section 14: add `authz.expiry_sweep_batch_max` with default `500` and the meaning rows per transaction of `authz.expiry_sweep` and `authz.expiry_reminder`; mention it in 8.3. Section 9.4: state the recipient rule of this ticket.
- `docs/spec/04-sync-protokoll.md` is core contract and stays unchanged.

### 6. Configuration

Add `authz.expiry_sweep_batch_max` (integer, at least 1) to the configuration schema of `internal/platform/config` and the typed `authz` configuration in `internal/wire` (M1-07 pattern), and regenerate `docs/operations/configuration.md` with the M1-08 tool so that `make config-doc-check` is green. `authz.expiry_sweep_interval` and `authz.expiry_reminder_lead` already exist; wire them if M1-28 or M1-29 did not.

### 7. Documentation

`services/server/internal/authz/README.md` (jobs table, algorithm, exactly-once argument), `docs/DECISIONS.md` (rulings below), `services/server/CHANGELOG.md` under Unreleased.

## Out of scope

- Cancelling runs of people who lost `trigger` (`run.cancel_requested`): `runs` subscribes to `AccessChanged` in M2. This ticket only guarantees that the in-transaction event is published.
- HTTP routes for grants and groups, the console list Laeuft bald ab: M1-48 and M4.
- Groups from OIDC claims and their expiry at login: M1-42 (its memberships expire through this sweep).
- Any change to how `authorize` treats expiry: done in M1-27 and M1-28.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-30-authz-expiry`, created from the base branch named by the launcher. One writer per worktree.
- Conventional Commits with the module as scope (for example `feat(authz): report expired grants and memberships`) and a DCO sign-off on every commit (`git commit -s`). Squash into meaningful commits before the merge.
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope of this brief, except `docs/DECISIONS.md`, `services/server/CHANGELOG.md` and the generated files named here.
- Follow the host rules supplied by the launcher. Tests bind only `127.0.0.1` and port 0 (P-10); PostgreSQL through `pgtest` (P-09), never skipped.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets. No network calls except package downloads for pinned tools.
- No gate may be weakened, no threshold lowered, no test deleted or skipped.

## Design notes

- Exactly once: marking (`expiry_reported_at`, `expiry_reminded_at`), events and audit commit in one transaction. A crash or error rolls back everything, and the next run selects the same rows again. Rows expired while the worker was down are found on the next run because the selection is not bounded by the last run time.
- No empty rights change: a sweep run that finds nothing rolls back its unit of work, so `permission_version` only moves when something expired. Otherwise every run would invalidate every rights cache once per `authz.expiry_sweep_interval`.
- Lock order: phase 1 locks `permission_version` before the sweep selects rows `FOR UPDATE` in ascending `id` (03 section 6.3). The reminder takes no `permission_version` lock.
- The difference uses `AssumeValid`: before = snapshot at `t` with the batch rows treated as valid, after = snapshot at `t`. This isolates the effect of exactly these expiries against the current state of all other rows.
- A grant revoked or a membership removed before the sweep is not reported as expired; the revocation already produced its own `access.changed`.
- Ruling: recipients of `access.expiring` for a grant to a group are all active owners and `granted_by`; 07 section 9.4 says an die betroffene Person bzw. an den Owner, and a group is no principal that could receive a personal event. For memberships the member and `added_by`.
- Ruling: `GroupChangedPayload.change` gets `member_expired`, because `member_removed` would claim a manual removal.
- `access.expiring` for a grant on the instance omits `target`, because `AccessTargetKind` only knows `space` and `channel`.
- Events of jobs carry no `actor` (04 section 2.2); audit entries have `actor_principal_id` NULL (03 `audit_events`).
- Personal data: no names or e-mail addresses in events beyond the schemas, in audit details or in logs (SEC-LOG-4).

## Tests (write them first)

Controllable clock, PostgreSQL 18 through `pgtest`, the world of 07 section 11.1 with real grants and memberships, the fake `StructureSource` and `AgentPolicySource` from M1-28 and M1-29. Call the handlers directly; River is not needed except for the registration test.

1. `TestAZ_P10_ExpiryWithoutJob`: no job registered or run; `authorize` at exactly `expires_at` of G-3 (Robin) and of the Leser membership (Sam) answers `not_found`, one microsecond earlier `allow`; the cached snapshot honours `valid_until`.
2. `TestExpirySweep_Grant`: G-3 expires at T; the sweep at T plus `authz.expiry_sweep_interval` writes exactly one `access.changed` for Robin with `removed` containing the channel, one `grant.changed` with `change = expired`, one audit row `grant.expire` without actor, sets `expiry_reported_at`, and raises `permission_version` by one.
3. `TestExpirySweep_GroupGrant`: a group grant expires; every member whose visible targets change gets `access.changed`; members with an independent grant on the same target get none.
4. `TestExpirySweep_Membership`: the Leser membership expires; Sam gets `access.changed`; `group.changed` with `member_expired`; audit `group.member.expire`.
5. `TestExpirySweep_ExactlyOnce`: two consecutive runs produce one set of events and audit rows; a run whose appender is replaced by a failing fake rolls back completely and the next run reports exactly once.
6. `TestExpirySweep_NoDueRowsKeepsVersion`: nothing due, `permission_version` unchanged, no events.
7. `TestExpirySweep_RevokedBeforeSweep`: a revoked grant past its expiry is not reported.
8. `TestExpirySweep_Batches`: with `authz.expiry_sweep_batch_max` set to two in the test config and five due rows, all are reported, in three transactions, and `permission_version` rises by three.
9. `TestExpirySweep_WorkerWasDown`: rows that expired long before the run are reported on the next run.
10. `TestExpirySweep_InvariantStillVerified`: with a world made invalid by a SQL fixture, the sweep batch is rejected, nothing is marked, and the handler returns an error.
11. `TestExpiryJobs_Registration`: both jobs are registered on queue `maintenance` with the period from `authz.expiry_sweep_interval`, only for the roles `worker` and `all`, without arguments.
12. `TestExpiryReminder_*`: a grant expiring at T gets nothing at `T - lead - 1s` and exactly one `access.expiring` each for subject and granter at `T - lead + 1s`; a second run adds nothing; a grant created with less than the lead left is reminded on the next run; a group grant reaches all active owners and the granter; a membership reaches member and `added_by`; recipients are deduplicated; inactive recipients are skipped; instance grants omit `target`; every payload validates against `AccessExpiringPayload` and contains no null.
13. Configuration: the schema knows `authz.expiry_sweep_batch_max` with its default; `make config-doc-check` is green.

Commands that must pass:

- `cd services/server && go test -race ./internal/authz/... ./internal/platform/config/...`
- `make migrations-lint`, `make codegen-check`, `make api-lint`, `make config-doc-check`, `make docs`
- `make check`

## Acceptance criteria

- [ ] Precondition checked; the migration adds only what is missing.
- [ ] AZ-P10 green with no job running.
- [ ] `access.changed` after an expiry is written by the first sweep run after the expiry, that is within `authz.expiry_sweep_interval`.
- [ ] Every expiry is reported exactly once (tests 5 and 9), with audit `grant.expire` or `group.member.expire`.
- [ ] Every reminder is sent exactly once, `authz.expiry_reminder_lead` before the expiry.
- [ ] A run without due rows leaves `permission_version` unchanged.
- [ ] `GroupChangedPayload` knows `member_expired`; generated code without drift.
- [ ] `authz.expiry_sweep_batch_max` in 07 section 14, in the schema and in the generated reference.
- [ ] README, DECISIONS and CHANGELOG updated; doc comments on every exported symbol.
- [ ] `make check` green.

## Definition of done

- Code, tests and documentation as listed in Scope; the module README is current (jobs, events, tables).
- A doc comment on every exported symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- An ADR for every architecture decision taken here (none expected; rulings go to `docs/DECISIONS.md`).
- CHANGELOG entry under Unreleased in `services/server/CHANGELOG.md`.
- `make check` green on the final commit.
- Independent review of the diff (`make review` plus a code review by a fresh reviewer) and the red team of `docs/quality/red-team-gate.md` over the diff, verdict recorded in the report. On `NEIN`: fix, add a test per finding, re-check the delta.
- Merge locally into the base branch only when the review is clean, the red team says `JA` and `make check` is green; then `git worktree remove`.

## Report

Write `{{REPORT_PATH}}`:

- first line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`;
- commits (sha and subject), files changed with line counts, test results with counts (zero skipped), the red output of the first run of each new test file, what is not proven, follow-ups that could not be done in scope with the reason.

Then write `{{DONE_PATH}}` with `exit=0 sha=<merge sha>` or `exit=1 <reason>`.
