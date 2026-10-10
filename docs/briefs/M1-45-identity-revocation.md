# M1-45: Disable a person, revoke a device, end a session

Status: ready | Depends on: M1-29, M1-35, M1-44 | Etappe: M1

## Goal

After this ticket an owner can disable a person, a person or an owner can revoke a device, and a
person or an owner can end a session. Each runs in one rights-change transaction in the fixed order
of 08 "Person sperren" and "Verlorenes Gerät": every step is visible in the database, the rights
version rises, `access.changed` and one `session.revoked` per ended session land in the event log
(M1-65 closes gateway connections with `4012` from them), and an audit entry is written. From the
commit on, every request that carries a token of the disabled person, the revoked device or the
ended session fails with `401`, without waiting for the token to expire (07 SR7, AZ-M40, AZ-M41).

## Read first

- `docs/PLAN.md`: section 3 "Arbeitsweise je Ticket"; section 7 rows M1-29, M1-34, M1-35, M1-44,
  M1-45, M1-65 and M3-22; "Voraussetzungen aus dem Kern-Vertrag" (row with K-RE-5).
- `docs/spec/08-identitaet.md`: "Verwaltung" (5.4), "Refresh-Tokens: Rotation, Familien,
  Wiederverwendung" (7.4, paragraph "Erweiterungs-Sitzungen"), "Verlorenes Gerät" (10), "Person
  sperren" (11), "API-Tokens für Agenten, Apps und Werkzeuge" (12, item "Grenzen"), "Tests" (15)
  rows ID-T22 and ID-T26.
- `docs/spec/07-rechte.md`: "Prinzipal-Kontext" (3), "Entscheidungen und Gründe" (4.1), "Ablauf"
  (4.2), "Invarianten in der Transaktion" (4.4), SR5, SR7, SR8, "Sofortige Wirkung bei Entzug"
  (8.2), "Testmatrix" (11.1) rows AZ-M26, AZ-M40, AZ-M41.
- `docs/spec/05-aktionen.md`: "Verbindliche Regeln" items 6 and 7; "Spalten Owner, Step-up,
  Audit, Rechte" (2.4); "Entscheidung" (2.5); 4.3 rows `user.disable`, `device.revoke`,
  `session.revoke`, `device.list`; 6 rows `session.revoked`, `user.updated`, `access.changed`.
- `docs/spec/03-datenmodell.md`: 4.2 tables `principals`, `users`, `owners` (SR8 invariant
  paragraph), `devices`, `device_keys`, `auth_sessions`, `refresh_token_families`,
  `refresh_tokens`, `api_tokens`; "Sperrreihenfolge" (6.3); "I1-Prüfung" (6.4).
- `docs/spec/04-sync-protokoll.md`: "Umschlag (Envelope)" (2.2), "Schließ-Codes" (6.4, `4012`),
  "Was beim Ändern passiert" (8.2), "Widerruf von Sitzung, Gerät oder Prinzipal" (8.5).
- `docs/spec/02-architektur.md`: "Ports, Ereignisse und Quellen je Modul" (4.3, row `identity`:
  `DeviceRevoked`, `SessionEnded`), "Schichten im Modul" (5), "Transaktionen und Unit of Work"
  (8.1 to 8.3), "404 statt 403" (9.2), "Nachweis der Prüfung" (9.3).
- `docs/spec/06-api.md`: "Step-up" (3.3), "Autorisierung in der API" (4), "Fehler" (5: codes
  `session_revoked`, `last_owner`, `step_up_required`, `invalid_state_transition`), "Idempotenz"
  (7).
- ADRs: `docs/adr/0017-eine-autorisierungsfunktion-404-statt-403-registrierungszwang.md`,
  `docs/adr/0020-geraetebindung-dpop-step-up-fuer-admin-aktionen.md`,
  `docs/adr/0021-audit-log-hash-verkettet-und-getrennt-von-telemetrie.md`.
- `docs/api/openapi.yaml`: operations `disableUser` (`POST /users/{user_id}/disable`),
  `revokeDevice` (`POST /devices/{device_id}/revoke`), `revokeSession`
  (`POST /sessions/{session_id}/revoke`); schemas `ReasonBody`, `User`, `Device`,
  `SessionRevokedPayload`, `SessionRevokedEvent`, `UserPayload`, `UserUpdatedEvent`.
- Code of earlier tickets (read before writing anything): `internal/identity` (M1-26 principals
  and owners, M1-31 people and owners, M1-33 devices, M1-34 sessions and refresh families,
  M1-35 step-up, M1-44 API tokens), `internal/authz` (M1-28 `authorize`, M1-29 rights-change
  hook and `VerifyInvariants`), `internal/kernel/txn`, `internal/kernel/eventlog`,
  `internal/kernel/auditing`, `internal/platform/httpserver`, `services/server/api/operations.yaml`.
- Tables: `principals`, `users`, `owners`, `devices`, `device_keys`, `auth_sessions`,
  `refresh_token_families`, `api_tokens`. Action ids: `user.disable`, `device.revoke`,
  `session.revoke`.

## Fixed requirements that apply

1. No MVP. Built from scratch directly in its final state; etappen are only the build order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined
   interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface has a docstring or
   doc comment (purpose, parameters, return value, errors, example where useful); comments
   explain why, not what. API documentation is generated from code (OpenAPI from the routes,
   dartdoc, Python API docs in the docs site).
4. Documentation in the repo: README per module/service, an ADR for every architecture decision,
   a CHANGELOG entry per release, docs site kept current. A change without matching
   documentation is not done.
5. Quality gates (SonarQube, linters, type checks, security scanners, test coverage) block the
   merge.
6. Test first: every behaviour gets a test that is red before the code exists (plan section 3).
7. No en dash or em dash in any text you write; white label (people Alex, Sam, Robin, domains
   under example.com); configuration only through named keys from
   `docs/operations/configuration.md`.
8. 08 "Person sperren": `user.disable` (owner, step-up) does in one transaction, in this order:
   last-owner check (SR8); `principals.status = 'disabled'` and `disabled_at`; all sessions
   revoked with reason `user_disabled` and all their refresh families revoked; all devices
   revoked; personal API tokens revoked; rights version raised, I1 checked, `access.changed`
   and `session.revoked` written; audit entry. Grants and group memberships stay untouched.
9. 08 "Verlorenes Gerät": `device.revoke` (self or owner, step-up) revokes in one transaction
   the device, its keys, all its sessions with reason `device_revoked`, their families, raises
   the rights version and writes audit.
10. 05 `session.revoke` (self or owner, no step-up): the session and all its child sessions end
    with the same reason (03 `auth_sessions.parent_session_id`, 08 7.4).
11. 07 SR8 and 03 `owners`: no transaction leaves zero active owners; `user.disable` on the last
    active owner fails with `409 last_owner` and changes nothing (AZ-M26).
12. 07 SR7 and 04 8.5: a disabled principal, a revoked device and a revoked session yield
    `unauthenticated` before any grant is read; the HTTP answer is `401` with problem code
    `session_revoked` (06 "Fehler"), the audit `deny_reason` is `principal_inactive`,
    `device_inactive` or `session_revoked` (07 4.1) (AZ-M40, AZ-M41).
13. 05 rule 1.7: all three actions have "Rechte" = `ja`: they raise `permission_version` and
    write `access.changed` for every affected principal through the M1-29 hook.
14. 04 8.5: every ended session produces one durable `session.revoked` event (audience self,
    the session's user), so that the gateway closes its connections with `4012`.
15. 08 12 "Grenzen": a disabled person's personal API tokens stop working at once.
16. 05 2.4: `user.disable` and `device.revoke` need step-up; all three are audited on allow and
    on deny.

## Scope

Paths are relative to `services/server/` unless they start with `docs/`.

1. **Specification first** (German, like the chapter):
   - `docs/spec/08-identitaet.md` "Verlorenes Gerät" and "Person sperren": name the stored reason
     codes (`devices.revoke_reason` = `revoked_by_self`, `revoked_by_owner` or `user_disabled`;
     `auth_sessions.revoke_reason` = `device_revoked`, `user_disabled`, `logout` or `admin`;
     `refresh_token_families.revoke_reason` = `session_revoked`) and the rule for
     `session.revoke` (Design notes). State that steps 5 (push registrations) and 7 (runs, dev
     environments) of "Person sperren" are carried out by subscribers of the in-transaction
     events in later etappen (M3-22, M2, V2).
   - `docs/DECISIONS.md`: the rulings listed under Design notes, in the file's existing format.
2. **Domain** `internal/identity/domain/lifecycle.go` (pure, imports only `kernel/ids`,
   `kernel/clock`, `kernel/errs`):
   ```go
   type SessionRevokeReason string // values of 03 auth_sessions.revoke_reason
   type DeviceRevokeReason string  // "revoked_by_self", "revoked_by_owner", "user_disabled"
   func SessionRevokeReasonFor(actorID, sessionUserID ids.ID) SessionRevokeReason
       // actor == owner of the session: "logout"; otherwise "admin"
   func DeviceRevokeReasonFor(actorID, deviceUserID ids.ID) DeviceRevokeReason
   func DisableTransition(status string) (change bool, err error)
       // "active", "pending": change; "disabled": no change; "deleted": errs conflict with
       // code invalid_state_transition
   func DeviceRevokeTransition(state string) (change bool)
       // "pending", "active": change; "revoked": no change
   ```
   If M1-33 or M1-34 already defined some of these types, extend them instead of duplicating.
3. **In-transaction events** in the port package `internal/identity` (02 4.3): `DeviceRevoked
   {DeviceID, UserID ids.ID; Reason string; At time.Time}` and `SessionEnded {SessionID, UserID,
   DeviceID ids.ID; Reason string; At time.Time}`. Reuse `SessionEnded` if M1-34 (logout) already
   declared it. Publish one `DeviceRevoked` per revoked device and one `SessionEnded` per ended
   session. No subscriber exists in M1; M3-22 (`notify`) subscribes to deactivate push
   registrations. The golden file of subscriptions in `internal/wire` does not change.
4. **Application** `internal/identity/app/lifecycle.go`:
   ```go
   type DisableUserCmd struct { Actor actor.Principal; UserID ids.ID; Reason string }
   type RevokeDeviceCmd struct { Actor actor.Principal; DeviceID ids.ID; Reason string }
   type RevokeSessionCmd struct { Actor actor.Principal; SessionID ids.ID }
   func (s *Service) DisableUser(ctx context.Context, cmd DisableUserCmd) (identity.UserView, error)
   func (s *Service) RevokeDevice(ctx context.Context, cmd RevokeDeviceCmd) (identity.DeviceView, error)
   func (s *Service) RevokeSession(ctx context.Context, cmd RevokeSessionCmd) error
   ```
   `Service`, `actor.Principal`, `UserView` and `DeviceView` stand for the names the earlier
   identity tickets use; follow them. Outgoing repository methods (add to the existing identity
   repository interface in `app`, suggested names): `LockUserForUpdate`, `SetPrincipalDisabled`,
   `ListLiveSessionsOfUser`, `ListLiveSessionsOfDevice`, `ListLiveChildSessions`,
   `RevokeSessions`, `RevokeFamiliesOfSessions`, `ListLiveDevicesOfUser`, `LockDeviceForUpdate`,
   `RevokeDevices`, `RevokeKeysOfDevices`, `RevokeLiveAPITokensOfPrincipal`.
5. **Adapter postgres** `internal/identity/adapters/postgres`: queries in
   `queries/lifecycle.sql` (or the existing query files of these tables), sqlc regenerated.
6. **Adapter httpapi** `internal/identity/adapters/httpapi`: strict-server methods
   `DisableUser`, `RevokeDevice`, `RevokeSession` mapping the generated request objects to the
   commands and the results to `User`, `Device` and `204`.
7. **Freischalt-Liste** `api/operations.yaml` (P-04): `disableUser`, `revokeDevice`,
   `revokeSession` move from `later` (or are added) to the built list; `make codegen-check`.
8. **Documentation**: `internal/identity/README.md` (lifecycle use cases, reason codes, events,
   what later etappen subscribe), `services/server/CHANGELOG.md` under `Unreleased`.

## Out of scope

- Closing gateway connections with `4012`, AZ-P9 across two processes: M1-65.
- Deactivating push registrations (08 11 step 5, 10 step 2) and the content-free `revoked` push:
  M3-22 subscribes to `DeviceRevoked` and `SessionEnded`.
- `run.cancel_requested` for runs of the disabled person (08 11 step 7): M2. Dev environments
  `suspended`: V2.
- `user.enable`, `owner.remove`, people CRUD: M1-31. Logout of the own session (`auth.logout`):
  M1-34. Device approval and pairing: M1-38.
- "Revoke all grants of this person" as a console step: M4 and M6 (it is `grant.revoke` per
  grant, M1-48).
- Automatic revocation of inactive devices (`auth.device_inactive_revoke_after`) and the job
  `identity.session_cleanup`: not in this ticket (reported as a plan gap).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-45-identity-revocation`.
- Conventional Commits with module scope (for example `feat(identity): disable a person in one
  transaction`) and DCO sign-off (`git commit -s`).
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope above except `services/server/CHANGELOG.md`,
  `docs/DECISIONS.md` and the two chapter paragraphs named in Scope item 1.
- Follow the host rules supplied by the launcher.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`; remove
  them by their literal path at the end. PostgreSQL runs only as the throwaway container of
  `internal/platform/pg/pgtest` on a random port bound to `127.0.0.1`.
- Never read or print secrets. No network calls except package downloads for pinned tools.

## Design notes

**Transaction shape (all three use cases).** One unit of work with the option `RightsChange`
(02 8.1) whose affected principals are: the person (`user.disable`), the device's user
(`device.revoke`), the session's user (`session.revoke`). Inside it, in this order:

1. Load and lock the resource row (`principals` row of a `user`, `devices` row, `auth_sessions`
   row) with `SELECT ... FOR UPDATE`. Missing, or a principal whose `kind` is not `user`: the
   same `not_found` as an invisible resource (02 9.2).
2. `Authorize(ctx, actor, action, resource)` (M1-28) inside the unit of work, after phase 1 has
   locked `permission_version`, so that decision and change see the same rights version. A
   denial returns the decision's error; the unit of work rolls back; the deny audit entry is
   written by `auditing.Recorder` in its own transaction (02 8.1, AZ-P13).
3. Domain transition. Already disabled or already revoked is a no-change case. Detect it
   before opening the RightsChange unit of work, in a plain read unit of work that also runs
   `Authorize` and writes the allow audit entry with `details.no_change = true`, then return
   `200` with the current object (`204` for sessions). Reason: phase 1 of a RightsChange unit
   of work raises `permission_version` unconditionally, and a no-op must not invalidate every
   rights cache. Inside the RightsChange unit of work the transition is checked again; if a
   concurrent request got there first, finish as no-change (the one extra version step is
   harmless and rare).
4. Writes in the lock order of 03 6.3: domain rows of one table in ascending `id`.
5. Events appended through `kernel/eventlog.Appender`, in-transaction events published,
   audit allow entry through `auditing.Recorder` in the same transaction.
6. Pre-commit hooks of M1-29 (`VerifyInvariants`: I1 and SR8, `access.changed`), then sync and
   audit hooks.

Do not add a separate last-owner query: SR8 is enforced by `VerifyInvariants` in the same
transaction, after the writes and before the commit (07 4.4); the outcome is identical to
checking first because the whole unit of work rolls back. The test for AZ-M26 proves that no
partial change survives.

**`user.disable`** (resource `user`, owner, step-up):

| Step (08 11) | Write |
|---|---|
| 1 | SR8 through `VerifyInvariants` (code `last_owner`, `409`) |
| 2 | `principals.status = 'disabled'`, `disabled_at = t` |
| 3 | every `auth_sessions` row of the person with `revoked_at IS NULL`: `revoked_at = t`, `revoke_reason = 'user_disabled'`; every live family of these sessions: `revoked_at = t`, `revoke_reason = 'session_revoked'` |
| 4 | every device with `state <> 'revoked'` (including `pending`): `state = 'revoked'`, `revoked_at = t`, `revoked_by = actor`, `revoke_reason = 'user_disabled'`; every key of these devices with `revoked_at IS NULL`: `revoked_at = t` |
| 5 | every `api_tokens` row with `principal_id = person` and `revoked_at IS NULL`: `revoked_at = t`, `revoked_by = actor` |
| 6 | rights version and `access.changed` (hook); one `session.revoked` per session from step 3; one `user.updated` with the new `User` |
| 8 | audit `user.disable`, `decision = allow`, `resource_kind = user`, `details = {reason, sessions_revoked, devices_revoked, tokens_revoked}` (counts, never token values) |

The owner row in `owners` stays (08 11: grants and memberships stay as well). Response `200`
with `User` (`status = disabled`).

**`device.revoke`** (resource `device`, self or owner, step-up): device `state = 'revoked'`,
`revoked_at`, `revoked_by`, `revoke_reason = DeviceRevokeReasonFor(actor, device.user_id)`; all
its live keys `revoked_at`; all its live sessions `revoke_reason = 'device_revoked'`, their live
child sessions with the same reason, their families `session_revoked`; one `session.revoked` per
ended session; audit with `details = {reason, sessions_revoked}`. Response `200` with `Device`.
A person may revoke the device they are using; the response is still sent, later requests fail.
A device of another person is invisible to a non-owner: `404` (07 4.3, "Ressource gehört p
selbst").

**`session.revoke`** (resource `auth_session`, self or owner, no step-up): the session and every
live child session (`parent_session_id = id`) get `revoke_reason = SessionRevokeReasonFor(actor,
session.user_id)`; families revoked; one `session.revoked` per ended session; audit. Response
`204`. Reason for the `logout`/`admin` split: 03 has no separate code for "ended remotely by the
person"; `logout` is what the person did, `admin` is what someone else did, and the audit names
the actor anyway.

**Events.** `session.revoked`: `audience = self` for the session's user, `resource = {kind:
auth_session, id}`, payload `{session_id, reason}` (`SessionRevokedPayload`). `user.updated`:
`resource = {kind: user, id}`, payload `UserPayload`. No `null` anywhere (ADR 0053).

**Authentication after the commit.** Nothing in this ticket caches state beyond the rights
version: `authorize` step 1 (M1-28) reads principal status, session and device state through the
rights-version-bound cache, so the version raised here invalidates it in every process (07 7.3,
04 8.5). If a test shows a revoked session still passing, the defect is in the cache key of
M1-28 or M1-34; fix it there in this ticket with a test, because the requirement belongs to this
ticket's acceptance. The problem code for all three cases is `session_revoked` (06 "Fehler":
"Sitzung, Gerät oder Prinzipal widerrufen"); if the HTTP chain maps `principal_inactive` or
`device_inactive` to another code today, correct the mapping.

**Refresh after revocation.** A refresh with a token of a revoked family fails with `401
session_revoked` and must not be treated as reuse (no second audit entry of kind
`refresh_reuse`), because the family is already revoked.

**Idempotency.** `disableUser` and `revokeDevice` and `revokeSession` take `Idempotency-Key`
(M1-20); a replay returns the stored response with `Idempotent-Replay: true`. A repeat without
the key is the no-change path above.

**Rulings for `docs/DECISIONS.md`:**
1. `devices.revoke_reason` holds a machine code (`revoked_by_self`, `revoked_by_owner`,
   `user_disabled`); the free-text reason of the request goes only into the audit entry.
   Reason: the column is shown in device lists, the text belongs to the audit trail.
2. `session.revoke` stores `logout` when the session's own person ends it and `admin` otherwise.
3. Revoking or disabling something that is already revoked or disabled succeeds without change
   and is audited with `no_change`. Reason: clients retry after their idempotency window; a
   conflict would look like a failure for a state that is already the wanted one.

## Tests (write them first)

Use the real modules `identity`, `authz`, `audit`, `sync`, `instance` against PostgreSQL 18
(`pgtest`), real access tokens with DPoP proofs and real step-up (M1-34, M1-35). Use the
test helpers the identity tickets introduced for creating people, devices, sessions and tokens;
if none exist outside `_test.go` files, add `internal/identity/identitytest` and attach it to the
module register the way M1-05 attaches test-support packages. Fakes only for
`authz.StructureSource` and `authz.AgentPolicySource` (no chat or agents data in this ticket).
Test names carry the spec IDs.

1. Domain, table driven: `TestSessionRevokeReasonFor`, `TestDeviceRevokeReasonFor`,
   `TestDisableTransition` (`active`, `pending`, `disabled`, `deleted`),
   `TestDeviceRevokeTransition`.
2. Integration, `internal/identity/app` and HTTP through the real chain:
   - `TestIDT22_DisableUserAllSteps`: Alex with one active device (app session plus a
     `notification_ext` child session), one pending device, two refresh families, both device
     keys, one personal token (created through the identity helper, so the test does not depend
     on the flag `tokens.personal_enabled`); the owner disables Alex with fresh step-up. Assert
     each row of the table above, one `access.changed` for Alex, one `session.revoked` per
     session, one `user.updated`, one `DeviceRevoked` per device and one `SessionEnded` per
     session (test subscriber registered only in the test), one allow audit entry with the
     counts, `permission_version` raised.
   - `TestAZM26_DisableLastOwnerFails`: the only owner disables themselves: `409 last_owner`;
     a row-by-row snapshot of all touched tables before and after is equal; a deny audit entry
     exists after the rollback.
   - `TestDisableUser_ConcurrentOwners`: the only two owners disable each other in parallel
     goroutines; exactly one succeeds, the other is rejected (`401 session_revoked`, because
     `authorize` inside the serialized unit of work sees its actor disabled), and exactly one
     active owner remains.
   - `TestAZM40_DisabledPrincipalIsUnauthenticated`: after the disable, Alex's unexpired access
     token with a valid DPoP proof gets `401` with code `session_revoked`; audit `deny_reason =
     principal_inactive`. Alex's personal token gets `401` as well.
   - `TestDisableUser_RequiresOwnerAndStepUp`: Sam (not owner) gets `403` with reason
     `owner_only`; the owner with a step-up older than `auth.step_up_max_age` gets `401
     step_up_required`; the owner's personal token gets `403`.
   - `TestDisableUser_AlreadyDisabledNoChange` and `TestDisableUser_DeletedIsInvalidTransition`.
   - `TestIDT26_RevokeDeviceAllSteps`: keys, sessions (with child), families revoked, reason
     codes as specified, `session.revoked` events, `DeviceRevoked`, audit; a refresh with the old
     refresh token gets `401 session_revoked`; the old access token gets `401`.
   - `TestRevokeDevice_OtherPersonsDeviceIs404ForNonOwner` (same status, body and headers as a
     random device id), `TestRevokeDevice_OwnerRevokesForeignDevice`,
     `TestRevokeDevice_StepUpRequired`, `TestRevokeDevice_SelfCurrentDevice`.
   - `TestAZM41_RevokedSessionIsUnauthenticated`: after `revokeSession`, the session's
     unexpired access token gets `401 session_revoked`, `deny_reason = session_revoked`.
   - `TestRevokeSession_ChildSessionsEnd`, `TestRevokeSession_ReasonLogoutForSelfAdminForOwner`,
     `TestRevokeSession_OtherPersonsSessionIs404`, `TestRevokeSession_NoStepUpNeeded`.
   - `TestRevocation_IdempotencyReplay` for all three operations.
3. Contract: every response of the three operations passes the response validation of the test
   build (M1-31 introduced it).

Commands that must pass:

```sh
cd services/server && go test -race ./internal/identity/... ./internal/authz/...
make codegen-check
make migrations-lint
make arch
make check
```

## Acceptance criteria

- [ ] ID-T22 and ID-T26 are green; every step of 08 "Person sperren" that belongs to M1 is
      asserted in the database state, and steps 5 and 7 are documented as later subscribers.
- [ ] AZ-M26, AZ-M40 and AZ-M41 are green with real authentication.
- [ ] A failed disable leaves no partial change and a deny audit entry.
- [ ] Concurrent disables never leave zero active owners.
- [ ] Each ended session has exactly one `session.revoked` event; each affected person exactly
      one `access.changed` per transaction.
- [ ] Personal tokens of a disabled person are revoked.
- [ ] Reason codes and the no-change rule are in 08 and `docs/DECISIONS.md`.
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
