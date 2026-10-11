# M1-35: identity: step-up with the device key

Status: ready | Depends on: M1-34 | Etappe: M1

## Goal

After this ticket a person confirms administrative actions with the device's `step_up` key: the
app fetches a single-use challenge, the operating system asks for biometrics or the device PIN,
the `step_up` key signs the challenge, and the server sets `auth_sessions.step_up_at`, which
counts as fresh for `auth.step_up_max_age`. Too many failed attempts end the session. Every
action with "Step-up = ja" in 05 (owners, grants, providers, keys, agent policy) depends on this
(08 "Ziele" item 4, ADR 0020, SR5).

## Read first

- `docs/spec/03-datenmodell.md` `auth_challenges` (R-DM-31): single-use challenges for step-up and
  the WebAuthn ceremonies in one table (`purpose` `step_up`, `passkey_login`, `passkey_enroll`,
  `passkey_register`), stored as HMAC, with `status`, `finished_at` and the failure index; and
  `step_up_failed` in `auth_sessions.revoke_reason` and `SessionRevokedPayload.reason`. Reason for a
  table: a single-use challenge and a failure limit need state that every process sees (the code
  stays node-neutral from M1 on, plan section 20). This ticket creates the whole table (all
  purposes in the CHECK) and uses `step_up`; M1-40 uses the passkey purposes.
- `docs/spec/08-identitaet.md`: "6.1 Zwei Schlüssel je nativem Gerät", "6.2 Step-up-Ablauf",
  "7.4" (revocation helpers), "8. Sitzungsdauer", "13. Schutz der Anmeldung" (row Step-up),
  "14.3 Step-up" (sequence diagram), "15. Tests" (ID-T18, ID-T27), "18. Konfigurationswerte".
- `docs/spec/03-datenmodell.md`: "4.2" tables `auth_sessions` (`step_up_at`, `step_up_method`,
  checks), `device_keys`, the challenge table above; "6.3 Sperrreihenfolge";
  "9. Konfigurationswerte" (`auth.step_up_max_age`).
- `docs/spec/05-aktionen.md`: "2.4 Spalten Owner, Step-up, Audit, Rechte", "2.5 Entscheidung",
  "4.1" (`auth.step_up`).
- `docs/spec/07-rechte.md`: "3. Prinzipal-Kontext", "4.1" (`step_up_required`,
  `step_up_not_possible`), "4.2 Ablauf" step 8, "SR5", "11.1 Testmatrix" (AZ-M36).
- `docs/spec/06-api.md`: "3.3 Step-up", "5. Fehler", "7. Idempotenz" item 3.
- `docs/spec/02-architektur.md`: "4.3" (`identity` port `Auth`: step-up), "8. Transaktionen und
  Unit of Work", "9. Request-Pfad".
- ADR 0020, ADR 0017, ADR 0021.
- `docs/api/openapi.yaml`: operations `createStepUpChallenge`, `completeStepUpWithDeviceKey`
  (and `completeStepUpWithPasskey`, which M1-40 builds); schemas `StepUpChallenge`,
  `StepUpDeviceKeyRequest`, `StepUpResult`, `Problem`.
- `docs/security/threat-model.md`: T-TK-01, T-ID-05 (step-up when approving devices).
- Merged code of M1-33 (`kernel/jose`, `identity.Devices.ActiveKey`, `identitytest`), M1-34
  (`identity.Auth.RevokeSession`, Authenticator, actor fields `StepUpAt`, `StepUpMethod`), M1-28
  (`authorize` step 8 and its matrix test world), M1-24 (audit recorder), M1-16 (event appender).
- `services/server/internal/identity/README.md`, `docs/quality/gates.md`, `CONTRIBUTING.md`,
  `docs/PLAN.md` "3. Arbeitsweise je Ticket".

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
6. Test first: every behaviour starts with a failing test (red output goes into the report).
7. No en dash and no em dash in any text you write.
8. White label: only example.com, example.org and the names Alex, Sam, Robin in examples.
9. Configuration values only through named keys from `docs/operations/configuration.md`.
10. Flow of 08 "6.2": challenge with a random value valid `auth.step_up_challenge_ttl`; the
    `step_up` key signs a JWS over `{challenge, sid, iat}`; the server checks the signature against
    the active `step_up` key of the session's device, the challenge (single use) and the time;
    then `auth_sessions.step_up_at = now()`, audit; fresh for `auth.step_up_max_age`.
11. Step-up only for people in a session, valid only for that session (08 "6.2", 07 SR5); tokens
    without a session get `403 forbidden` (06 "3.3", reason `step_up_not_possible`).
12. Missing or too old step-up on a step-up action gives `401 step_up_required` (05 "2.4", 07
    "4.2" step 8).
13. `auth.step_up_fail_limit` failures per `auth.login_fail_window` revoke the session
    (08 "13").
14. `createStepUpChallenge` and `completeStepUpWithDeviceKey` have no `Idempotency-Key`
    (06 "7" item 3); action `auth.step_up`, class `rl.public_auth`, audit `ja` (05 "4.1").

## Scope

1. Migration `services/server/migrations/<next free number>_identity_step_up.sql`: the table
   `auth_challenges` exactly as 03 has it (columns, checks, FKs, unique and indexes).
2. sqlc queries `internal/identity/adapters/postgres/queries/stepup.sql` and repository.
3. Port `internal/identity/stepup.go` (names fixed): `Challenge{ID, Value string, ExpiresAt}`,
   `StepUpResult{StepUpAt, ValidUntil time.Time}`, `type StepUp interface{ CreateChallenge(ctx,
   actor.Actor) (Challenge, error); CompleteWithDeviceKey(ctx, actor.Actor, challengeID ID,
   signature string) (StepUpResult, error) }`. M1-40 adds `CompleteWithPasskey` to the same port.
4. Domain `internal/identity/domain/stepup`: pure check of the signed payload (see "Design notes")
   and the failure-limit decision.
5. App `internal/identity/app/stepup.go`: use cases with `authorize`, unit of work, audit and the
   call to `Auth.RevokeSession` (M1-34).
6. Adapter `internal/identity/adapters/httpapi/stepup.go`: handlers `createStepUpChallenge` and
   `completeStepUpWithDeviceKey`; enable both in `services/server/api/operations.yaml` and
   regenerate (command from `services/server/README.md`).
7. Config keys in the identity `Config`: `auth.step_up_challenge_ttl`, `auth.step_up_max_age`,
   `auth.step_up_fail_limit`, `auth.login_fail_window`, `auth.clock_leeway`,
   `auth.token_pepper_ref`; register missing ones in the schema, `make config-reference`.
8. Job `identity.auth_cleanup` (02 "12.3", queue `maintenance`, every `auth.cleanup_interval`,
   role `worker`): deletes rows of `auth_challenges` past `expires_at`; M1-38 adds pairings and
   device expiry, M1-41 adds `auth_login_flows`. Register `auth.cleanup_interval` in the identity
   `Config`. Request handling never depends on the job (expiry is checked per request).
9. Docs: `docs/spec/08-identitaet.md` "6.2" and "13" already state the JWS profile, the failure
   counting and the answers, and `StepUpDeviceKeyRequest` in `openapi.yaml` describes the signed
   payload; change them only where the implementation proves them wrong (spec first). Rulings in
   `docs/DECISIONS.md`; identity README; `services/server/CHANGELOG.md`.

## Out of scope

- Step-up with a passkey (`completeStepUpWithPasskey`, `passkey_options` in the challenge) (M1-40).
- Step-up with password and TOTP and with OIDC re-login (08 "6.2"): the operations come with the
  console session (M6-01), spec first.
- Any administrative action itself; this ticket proves AZ-M36 through `authorize`.
- Device approval with step-up (M1-38), API tokens (M1-44; AZ-M35 with a real personal token is
  tested there).

## Boundaries

- A new migration needs its line in `migrations.sha256`: run `make migrations-lock` after creating
  the file and before `make check`; never edit a merged migration (M1-01, ADR 0007).
- Work in a dedicated git worktree on the branch `feat/M1-35-identity-step-up`.
- Conventional Commits with scope `identity` (or `api`, `docs` where fitting), each signed off
  (`git commit -s`).
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope above except `services/server/CHANGELOG.md`,
  `docs/DECISIONS.md` and generated references.
- Follow the host rules supplied by the launcher.
- Tests bind only `127.0.0.1` and port 0; PostgreSQL 18 as a throwaway container via the M1-12
  helper; no Docker runtime means failure, never a skip.
- Stop only processes you started, by PID; temporary directories only via `mktemp -d`, removed at
  the end.
- Never read or print secrets; tests generate keys with `identitytest`.
- No network calls except package downloads for pinned tools.
- A gap in the core contract ends the ticket with `Verdict: BLOCKED` (P-11).

## Design notes

Create challenge (`createStepUpChallenge`):

1. The actor must be a person with a session (`kind = user`, `SessionID` set) whose device is not
   `platform = 'cli'` (08 "6.4"); otherwise `403 forbidden`, audit deny with reason
   `step_up_not_possible`.
2. `authorize(actor, auth.step_up, auth_session = actor.SessionID)`: right `self`; a pending
   device gets `403 device_pending`; a revoked session `401 session_revoked`.
3. Value: 32 bytes from `crypto/rand`, base64url without padding. Store `challenge_hash` =
   HMAC-SHA-256(token pepper, value), `status = issued`, `expires_at = now +
   auth.step_up_challenge_ttl`, `id` = new UUIDv7.
4. Response `200 StepUpChallenge` with `challenge_id`, `challenge`, `expires_at`;
   `passkey_options` absent. The display text mentioned in 08 "6.2" is not in the schema; clients
   show their own localized text.

Complete with the device key (`completeStepUpWithDeviceKey`):

1. Actor checks and `authorize` as above.
2. The device of the session needs an active `step_up` key (`Devices.ActiveKey`); otherwise
   `403 forbidden`, audit deny reason `step_up_not_possible` (details `{"cause":"no_step_up_key"}`).
3. Lock order (03 "6.3"): `SELECT ... FOR UPDATE` on the session row, then on the challenge row.
   Concurrent completions of one challenge therefore serialize.
4. Challenge unknown, of another session or of another purpose: `401 step_up_required`, not
   counted (the value cannot be signed without the challenge), audit deny.
5. Challenge `status` not `issued` or `expires_at` <= now: `401 step_up_required`, not counted,
   audit deny.
6. Signature check, counted as a failure when it fails:
    - Parse with `kernel/jose`, allowlist exactly the key's `alg`; reject `crit`, `jwk`, `jku`,
      `x5u`, `x5c` in the header (the key comes only from `device_keys`).
    - Verify with the active `step_up` key; a signature by the `dpop` key or any other key fails
      (ID-T18 "falscher Schlüssel").
    - Payload: `challenge` string whose HMAC equals `challenge_hash` (`hmac.Equal`); `sid` equals
      `actor.SessionID`; `iat` integer NumericDate, not before `created_at - auth.clock_leeway` and
      not after now + `auth.clock_leeway`. Unknown members are ignored.
7. Failure: set the challenge `failed`, `finished_at = now`; count failed challenges of this
   session with `finished_at > now - auth.login_fail_window`; if the count reaches
   `auth.step_up_fail_limit`, call `Auth.RevokeSession(session, step_up_failed, actor)` in the same
   unit of work. Commit, write the audit deny (reason `step_up_required`, details with the cause,
   never the signature), then answer `401 session_revoked` if the session was revoked, else
   `401 step_up_required`.
8. Success: challenge `used`, `finished_at = now`; `auth_sessions.step_up_at = now`,
   `step_up_method = 'device_key'`; audit allow; `200 StepUpResult` with `step_up_at` and
   `valid_until = step_up_at + auth.step_up_max_age`.

The step-up time is read per request into the actor by the Authenticator of M1-34, so the next
request sees it in every process; no cache and no `permission_version` change. A child session
has its own `step_up_at`; a step-up in the parent does not count for a child and the reverse.

Choice of codes (spec silent, record as ruling in DECISIONS.md and 08 "6.2"): a failed
confirmation is `401 step_up_required` because the client's correct reaction is a new step-up
(06 "5" table), not a token refresh or a re-login; `403 forbidden` where step-up is impossible for
this caller or device.

## Tests (write them first)

Unit tests:

- `TestStepUpPayload_Check` table: correct payload; wrong `challenge`; wrong `sid`; `iat` before
  `created_at - leeway`; `iat` after now + leeway; missing members; `crit` and `jwk` headers;
  `alg` differing from the key.
- `TestStepUpFailLimit_Decision`: failures inside and outside `auth.login_fail_window` (steerable
  clock), limit reached exactly at `auth.step_up_fail_limit`.
- `TestChallengeValue_IDT27Like`: 32 bytes from `crypto/rand`, distinct across draws.

Integration tests against PostgreSQL 18 with real Authenticator (M1-34), real `authz`, audit and
event appender:

- `TestStepUp_DeviceKeySuccess`: `step_up_at`, `step_up_method = device_key`, `valid_until`,
  audit allow; the next request's actor carries the new `step_up_at`.
- `TestStepUp_IDT18_Missing`: an owner session without step-up calls `authorize` for
  `identity_provider.manage` (instance, owner, step-up): `401 step_up_required`.
- `TestStepUp_AZM36_TooOld`: owner, step-up older than `auth.step_up_max_age` (steerable clock),
  `authorize(grant.create)` on the Marketing channel of the M1-28 matrix world (structure from its
  fake `StructureSource`, principal, session and step-up real): `step_up_required`.
- `TestStepUp_IDT18_AgentToken`: an agent actor (fake, no session) gets `403 forbidden` with audit
  reason `step_up_not_possible` on both operations.
- `TestStepUp_IDT29_CliDevice`: a session of a `cli` device gets `403 forbidden` with audit
  `step_up_not_possible` for the challenge and the completion; a step-up action from it stays
  `401 step_up_required`.
- `TestStepUp_IDT18_WrongKey`: signature by the `dpop` key: `401 step_up_required`, counted.
- `TestStepUp_ChallengeSingleUse`: second completion with the same challenge `401`.
- `TestStepUp_ChallengeExpired`, `TestStepUp_ForeignChallenge`: `401`, not counted.
- `TestStepUp_ConcurrentCompletion`: two parallel completions of one challenge: exactly one
  success.
- `TestStepUp_FailLimitRevokesSession`: after `auth.step_up_fail_limit` counted failures the
  session is revoked with the reason of 03, `session.revoked` appended, `401 session_revoked`;
  failures older than the window do not count.
- `TestStepUp_BoundToSession`: step-up in one session does not satisfy another session of the
  same person, nor its child session.
- `TestStepUp_PendingDevice`: `403 device_pending`.
- `TestStepUp_NoStepUpKey`: device without `step_up` key: `403 forbidden`.
- `TestStepUp_NoSecretsInLogs`: challenge value and signature absent from logs and audit details.

Commands that must pass:

```sh
cd services/server && go test -race ./internal/identity/... ./internal/authz/...
make arch
make codegen-check
make migrations-lint
make api-lint
make config-reference && git diff --exit-code docs/operations/configuration.md
make check
```

## Acceptance criteria

- [ ] Challenges are single use, valid `auth.step_up_challenge_ttl`, stored only as HMAC.
- [ ] Completion verifies against the active `step_up` key of the session's device only; `sid` and
      `iat` checked; success sets `step_up_at` and `step_up_method`, audit allow.
- [ ] ID-T18 (missing, too old, agent token, wrong key) and AZ-M36 green with the codes of
      "Design notes".
- [ ] `auth.step_up_fail_limit` failures within `auth.login_fail_window` revoke the session with
      `session.revoked`.
- [ ] Concurrent completions of one challenge succeed at most once.
- [ ] The job `identity.auth_cleanup` deletes expired challenges (test with a controlled clock).
- [ ] 08 "6.2" and "13" match the implementation; rulings in DECISIONS.md.
- [ ] `make check` green.

## Definition of done

Code, tests and docs complete: identity README (status, port `StepUp`, challenge table, action
`auth.step_up`, config keys, test ids), a doc comment on every public symbol, comments explain why,
an ADR only for an architecture decision beyond ADR 0020, the CHANGELOG entry under Unreleased of
`services/server/CHANGELOG.md`, generated references current. `make check` green. Independent
review (`make review` plus a code review of the diff) and red team gate
(`docs/quality/red-team-gate.md`) over the diff with both verdicts in the report; merge locally
into the base branch only if both are clean and `make check` is green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then the
commits, the files with line counts, the test results with counts (including the red output of
the first failing tests), what is not proven, and follow-ups that could not be done in scope, each
with its reason. Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
