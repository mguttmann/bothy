# M1-38: Device approval and pairing

Status: ready | Depends on: M1-37 | Etappe: M1

## Goal

After this ticket a new device that logged in without an enrollment code waits in state `pending`:
it has tokens but no access beyond reading itself, and it shows a verification code. The person
from one of their active devices, or an owner, approves it with step-up by typing that code; a
wrong code revokes the waiting device. As a second way, an active device pairs a new device by
entering the pairing code the new device shows. Waiting devices and unfinished pairings expire.
This closes the gap "someone else's device gets approved by a blind tap" (T-ID-05) and gives local
accounts without passkeys a way to add devices.

## Read first

- `docs/spec/08-identitaet.md`: "Lokale Konten", "Registrierung", "Bestätigung",
  "Geräte-Kopplung", "Verwaltung" (section 5), "Zwei Schlüssel je nativem Gerät", "DPoP",
  "Schutz der Anmeldung", "Bestätigung eines neuen Geräts" (flow 14.2), "Tests" (ID-T12, ID-T13),
  "Entscheidungen dieses Kapitels" (R-ID-08), "Konfigurationswerte dieses Kapitels".
- `docs/spec/03-datenmodell.md`: tables `devices`, `device_keys`, `device_pairings`,
  `auth_sessions`, `refresh_token_families`, `audit_events`; Grundsätze 9 and 13.
- `docs/spec/05-aktionen.md`: "Entscheidung" (pending device rule), actions `auth.login.begin`,
  `auth.login.complete`, `device.list`, `device.approve`, `self.read`, `auth.token.refresh`,
  `auth.logout`; R-AZ-16.
- `docs/spec/07-rechte.md`: "Entscheidungen und Gründe" (`device_pending`, `device_inactive`,
  `not_self`), "Ablauf".
- `docs/spec/06-api.md`: "Anmeldeabläufe" point 4, "Fehler", "Idempotenz", "Endpunkte" (Auth,
  Devices), "Weitere Punkte" K13 and K16.
- `docs/spec/02-architektur.md`: "Schichten im Modul", "Transaktionen und Unit of Work",
  "Hintergrundjobs (River)" (job catalogue, rules for jobs).
- `docs/PLAN.md`: section 7 W5 and row M1-38.
- ADR 0019, ADR 0020.
- `docs/api/openapi.yaml`: `approveDevice`, `getDevice`, `listMyDevices`, `listUserDevices`,
  `beginPairing`, `approvePairing`, `completePairing`; schemas `Device`, `DeviceRegistration`,
  `DeviceKeyRegistration`, `PairingBeginRequest`, `PairingStarted`, `PairingApprove`,
  `PairingCompleteRequest`, `PairingStatus`, `TokenResponse`; response `Gone`.
- `docs/security/threat-model.md`: T-ID-03, T-ID-05 and the device row in the STRIDE table.
- Code: `internal/identity` (M1-33 devices and DPoP, M1-34 sessions, M1-35 step-up, M1-37
  `CompleteLogin` and enrollment), `internal/authz` (M1-28 pending-device rule),
  `internal/platform/jobs` (M1-14).

## Fixed requirements that apply

1. No MVP. Built from scratch directly in its final state; etappen are only the build order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined
   interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface has a docstring or
   doc comment (purpose, parameters, return value, errors, example where useful); comments explain
   why, not what. API documentation is generated from code (OpenAPI from the routes, dartdoc,
   Python API docs in the docs site).
4. Documentation in the repo: README per module/service, an ADR for every architecture decision, a
   CHANGELOG entry per release, docs site kept current. A change without matching documentation is
   not done.
5. Quality gates (SonarQube, linters, type checks, security scanners, test coverage) block the
   merge.
6. Test first: every behaviour change starts with a failing test (red), then code (green), then
   cleanup. No en dash or em dash in any text. White label: examples use Alex, Sam, Robin and
   `example.com`. Configuration values only through named keys listed in
   `docs/operations/configuration.md`.
7. A new device is `pending` unless it redeems an enrollment code or is confirmed by pairing. A
   pending device has tokens but every action except `self.read`, `device.list` for itself,
   `auth.token.refresh` and `auth.logout` ends with `403 device_pending` (08 "Bestätigung", 05
   "Entscheidung", R-AZ-16). After `auth.device_pending_ttl` without approval the device is
   revoked.
8. Approval (`device.approve`, step-up) by the person from one of their active devices or by an
   owner, by typing the verification code of `auth.device_verification_code_digits` digits that the
   pending device shows; a wrong code rejects (revokes) the pending device (08 "Bestätigung",
   R-ID-08, ID-T13).
9. Pairing: the new device registers its public keys without login and with a DPoP proof, shows a
   pairing code of `auth.pairing_code_chars` Crockford characters valid `auth.pairing_code_ttl`;
   an active device of the same person enters it with step-up; the new device collects its tokens
   with a DPoP proof of the registered key; one failed attempt invalidates the request; rate limits
   per code and per IP (08 "Geräte-Kopplung", "Schutz der Anmeldung").

## Scope

Specification first, on this branch, before production code:

1. `docs/api/openapi.yaml`:
   - `approveDevice`: required request body `DeviceApprove` with required `verification_code`
     (string, `pattern: ^[0-9]+$`), `additionalProperties: false`. Description: on a wrong code the
     device is revoked and the response is `200` with `state: revoked`.
   - `Device`: add optional `verification_code` (string, digits), description "Present only in
     responses to the pending device itself (`TokenResponse`, `getDevice`, `listMyDevices` for its
     own entry); never shown to other devices or owners."
   - `approvePairing`: description adds "A code that matches no open pairing answers `404` and
     ends the step-up of the calling session; the next attempt needs a new step-up."
   - `completePairing`: description adds "A proof signed by another key rejects the pairing."
2. Regenerate the generated blocks of `docs/spec/06-api.md` with the M1-03 generator.
3. `docs/spec/08-identitaet.md`:
   - "Bestätigung": derivation of the verification code, where it is shown, revoke reasons; test
     ID-T12 reads "wartendes Gerät ruft eine Route auf: Tokens, aber kein Zugriff außer
     Selbstauskunft (`403 device_pending`)".
   - "Geräte-Kopplung": device created at approval, collection window, expiry, wrong code rule.
   - "Schutz der Anmeldung": the table row for pairing and verification codes names the concrete
     rules from Design notes.
   - "Konfigurationswerte dieses Kapitels": add `auth.cleanup_interval` only if case B of the
     cleanup job applies (Design notes).
4. `docs/security/threat-model.md`: the STRIDE row "Wartendes Gerät verschafft sich Zugriff" says
   "Tokens ohne Zugriff bis zur Bestätigung (403 device_pending), eingetippter Bestätigungs-Code".
5. `docs/PLAN.md`: row M1-38 acceptance text for ID-T12 as in point 3.
6. `docs/DECISIONS.md`: rulings listed in Design notes.

Production code under `services/server/`:

- `migrations/<next sequence>_identity_device_pairings.sql`: table `device_pairings` exactly as in
  03 (checks, FKs, partial unique index on `pairing_code_hash WHERE status = 'requested'`, index
  `(expires_at)`).
- `internal/identity/adapters/postgres/queries/device_pairings.sql`, queries for approval and
  expiry; regenerated sqlc code.
- `internal/identity/domain/verification.go`: `DeriveVerificationCode`, `VerificationCodeEqual`.
- `internal/identity/domain/pairing.go`: pairing code generation and normalisation, state machine
  `requested -> approved -> completed`, `requested -> rejected | expired`, `approved -> expired`
  (all other transitions are errors), JSON type `DeviceRegistrationV1` for `device_registration`.
- `internal/identity/app/approval.go`: use case `ApproveDevice`; internal function
  `revokeUnconfirmedDevice(ctx, deviceID, reason)`.
- `internal/identity/app/pairing.go`: use cases `BeginPairing`, `ApprovePairing`,
  `CompletePairing`.
- `internal/identity/app/expiry.go`: `ExpirePendingDevices`, `ExpirePairings` for the cleanup job.
- Authentication: the actor builder from M1-34 treats a pending device older than
  `auth.device_pending_ttl` as revoked (`401 session_revoked`, reason `device_inactive`).
- Response mapping of `Device` and `TokenResponse.device`: add `verification_code` only for the
  pending device itself.
- `internal/identity/adapters/httpapi`: handlers `ApproveDevice`, `BeginPairing`,
  `ApprovePairing`, `CompletePairing`.
- Cleanup job (Design notes) registered through `internal/platform/jobs` in `internal/wire`.
- `services/server/api/operations.yaml`: enable `approveDevice`, `beginPairing`, `approvePairing`,
  `completePairing`; regenerate `api/rest` and `api/routes`.
- `internal/identity/README.md`, `services/server/CHANGELOG.md`.

Public interfaces (Go, names fixed):

```go
// internal/identity/domain
// DeriveVerificationCode returns the decimal code the pending device shows (08 "Bestätigung").
func DeriveVerificationCode(pepper []byte, deviceID ids.ID, dpopJKT string, digits int) string
func VerificationCodeEqual(expected, typed string) bool // constant time, ignores spaces

// internal/identity/app
func (s *Service) ApproveDevice(ctx context.Context, deviceID ids.ID, verificationCode string) (DeviceView, error)
func (s *Service) BeginPairing(ctx context.Context, reg DeviceRegistration, proofJKT string, clientIP netip.Addr) (PairingStarted, error)
func (s *Service) ApprovePairing(ctx context.Context, pairingCode string) (DeviceView, error)
func (s *Service) CompletePairing(ctx context.Context, pairingID ids.ID, proofJKT string, clientIP netip.Addr) (PairingOutcome, error) // tokens, or status requested
```

## Out of scope

- Login flows that create pending devices: password (M1-39), passkey (M1-40), OIDC (M1-41),
  trusted header (M1-43). Tests here create pending devices through `CompleteLogin` with
  `TrustUnconfirmed` (M1-37).
- Revoking active devices with the full ordered sequence and push registrations (`device.revoke`,
  M1-45); the gateway close code `4016` for pending devices (M1-64, M1-65).
- Notifying the person that a device waits (push, M3), key rotation of a device (08 "Verwaltung",
  later), the console approval queue (M6), automatic revocation after
  `auth.device_inactive_revoke_after` (not this ticket).

## Boundaries

- Work in a dedicated git worktree on branch `feat/M1-38-identity-device-approval`.
- Conventional Commits with module scope and DCO sign-off (`git commit -s`).
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside Scope, except `docs/DECISIONS.md`, the `docs/PLAN.md` row M1-38 and
  CHANGELOG entries.
- Follow the host rules supplied by the launcher. Tests bind only `127.0.0.1` and port 0;
  PostgreSQL only through `pgtest`.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at
  the end.
- Never read or print secrets. No network calls except package downloads for pinned tools.
- If an element of the core contract (00, 03, 04, 05) named here is missing or different, stop with
  `Verdict: BLOCKED` (P-11).

## Design notes

Pending devices and the verification code:

- The rule "tokens, but `403 device_pending`" is normative in 05 and 07 and already enforced by
  `authorize` (M1-28). ID-T12 in 08, the threat model and the plan row say "no access token"; the
  core contract wins. Fix those texts (Scope) and test the 05 behaviour.
- The code is derived, not stored, so no column is needed: `HMAC-SHA-256(key = pepper, msg =
  "device-verification/v1" || 0x00 || device_id (16 bytes) || 0x00 || dpop_jkt (ASCII))`; take
  the first 8 bytes as big-endian `uint64`, reduce modulo `10^digits`, left-pad with zeros to
  `auth.device_verification_code_digits`. The pepper comes from `auth.token_pepper_ref`. The code
  depends on the device and its DPoP key, so a second login of an attacker shows a different code.
- Shown only to the pending device itself: `TokenResponse.device.verification_code` after a login
  that leaves the device pending, and `getDevice` or `listMyDevices` when the caller's session
  device is that device and it is still pending. Owners and other devices never see it.
- `ApproveDevice` order: load the device (invisible: `404`), `authorize(actor, device.approve,
  device)` (owner or self; a pending caller gets `403 device_pending`; step-up required), state not
  `pending`: `409 invalid_state_transition`, pending older than `auth.device_pending_ttl`: revoke it
  with reason `pending_expired` and answer `409 invalid_state_transition`.
- Correct code: one unit of work with the rights-change option for the device's person: `state =
  'active'`, `approved_by` = actor, `approved_at`; audit `device.approve` allow with details
  `{device_id, result: "approved"}`; `200` with the device.
- Wrong code: same unit of work shape: `revokeUnconfirmedDevice(deviceID, "verification_failed")`;
  audit `device.approve` allow with details `{device_id, result: "rejected_wrong_code"}`; `200`
  with `state: revoked`. Reason for `200`: the request had a committed effect; an error status
  would claim nothing changed.
- `revokeUnconfirmedDevice`: `devices.state = 'revoked'`, `revoked_at`, `revoked_by` (actor or
  NULL for system), `revoke_reason`; `device_keys.revoked_at` for all keys; every session of the
  device `revoked_at` with `revoke_reason = 'device_revoked'`; their refresh families
  `revoke_reason = 'session_revoked'`; one `session.revoked` event per session; in-tx event
  `DeviceRevoked`. Use the session revocation primitive from M1-34. This subset is complete for
  unconfirmed devices because they never had push registrations or gateway connections; M1-45
  owns revocation of active devices.
- Device revoke reasons introduced here (free text column `devices.revoke_reason`, document them
  in 08): `verification_failed`, `pending_expired`, `pairing_expired`.

Pairing:

- Code: `auth.pairing_code_chars` Crockford Base32 characters from `crypto/rand` (default 8
  characters, 40 bits); display in two groups joined by `-`; normalisation as for enrollment codes
  (M1-37) without prefix. Stored as `HMAC-SHA-256(pepper, "pairing-code/v1" || 0x00 ||
  canonical)`. On a unique-index collision with another open pairing, generate a new code (bounded
  retry, named constant).
- `BeginPairing` (`beginPairing`, action `auth.login.begin`, `rl.public_auth` per IP): DPoP login
  proof without `ath`; proof `jkt` must equal the thumbprint of `device.public_jwk` (else `401
  invalid_dpop_proof`); validate the registration with the M1-33 rules (UUIDv7 `device_id` not in
  use, platform, `alg` in `ES256`/`EdDSA`, public JWK without private members, `step_up_key`
  different from the DPoP key). Insert `device_pairings` with `status = 'requested'`,
  `device_registration` as `DeviceRegistrationV1` (`{"v":1, device_id, name, platform,
  app_version, dpop:{jwk, alg}, step_up?:{jwk, alg}, attestation?}`), `expires_at = now +
  auth.pairing_code_ttl`. `201` with `pairing_id`, `pairing_code`, `expires_at`. The code is
  never logged.
- `ApprovePairing` (`approvePairing`, `/me/devices/pairings`, action `device.approve`, step-up,
  `rl.admin`): `authorize` with a device resource owned by the caller (the device is created for
  the caller). Malformed code: `400 validation_failed`, step-up untouched. Well-formed code that
  matches no pairing with `status = 'requested'` and `expires_at > now`: in the same unit of work
  set the calling session's `step_up_at` and `step_up_method` to NULL, audit `device.approve` deny
  (`deny_reason` `no_grant`, details `{result: "pairing_code_unknown"}`), answer `404 not_found`.
  Reason: 08 asks that one failed attempt invalidates the request; the approving request is
  bound to the step-up, so every guess costs a fresh biometric confirmation, and 40 bits within
  `auth.pairing_code_ttl` cannot be searched that way.
- Match: in one unit of work with the rights-change option for the caller: create the device for
  the caller (`state = 'active'`, `approved_by` = caller, `approved_at`) and its keys from the
  registration; set pairing `status = 'approved'`, `user_id`, `approved_by_device_id` (the
  caller's device), `device_id`, and `expires_at = now + auth.pairing_code_ttl` (a fresh window to
  collect); audit allow with details `{pairing_id, device_id}`; `201` with the device.
- `CompletePairing` (`completePairing`, action `auth.login.complete`, `rl.public_auth` per IP):
  DPoP login proof required. Unknown `pairing_id`: `410 gone`. Proof `jkt` not equal to the
  registered `dpop` key: set `status = 'rejected'` and answer `401 invalid_dpop_proof`. Status
  `requested` and not expired: `202` with `PairingStatus`. Status `approved` and not expired:
  `CompleteLogin` (M1-37) with `TrustConfirmed`, `AuthMethod = "device_pairing"`, the created
  device (known device, same key, stays active), then `status = 'completed'`, `completed_at`;
  `201` with `TokenResponse`. Expired, rejected or completed: `410 gone`.
- Expiry: `requested` past `expires_at` becomes `expired`; `approved` past `expires_at` becomes
  `expired` and its device is revoked with `revokeUnconfirmedDevice(deviceID,
  "pairing_expired")` (it never got a session). Applied lazily on access and by the cleanup job.

Cleanup job: the job catalogue in 02 names `identity.session_cleanup` (periodic, queue
`maintenance`). Check with `grep -rn "identity.session_cleanup" services/server`.

- Case A, the job exists: add `ExpirePendingDevices` and `ExpirePairings` to it and use its
  interval key.
- Case B, it does not exist: create it with exactly that name in `internal/identity` through
  `internal/platform/jobs`, queue `maintenance`, interval from the new key `auth.cleanup_interval`
  (default `1h`, type duration) added to the configuration schema (M1-07) and to 08
  "Konfigurationswerte dieses Kapitels", then regenerate `docs/operations/configuration.md` with
  the M1-08 generator.

In both cases correctness never depends on the job: authentication treats an expired pending
device as revoked, and pairing endpoints check `expires_at` themselves (same principle as AZ-P10).
Job arguments carry no data; each run is idempotent.

Configuration used: `auth.device_pending_ttl`, `auth.device_verification_code_digits`,
`auth.pairing_code_ttl`, `auth.pairing_code_chars`, `auth.token_pepper_ref`,
`auth.step_up_max_age`, `ratelimit.rl.public_auth.*`, `ratelimit.rl.admin.*`; possibly
`auth.cleanup_interval` (case B).

Rulings to record in `docs/DECISIONS.md`: ID-T12 wording follows 05; derived verification code
without storage; `200` with `state: revoked` on a wrong code; device created at pairing approval
with a fresh collection window; wrong pairing code ends the caller's step-up; revoke reasons;
cleanup job case taken.

## Tests (write them first)

Integration tests use `pgtest`, the controllable clock, `CompleteLogin` with `TrustUnconfirmed`
to create pending devices, and the step-up helper from M1-35.

Unit:

- `TestDeriveVerificationCode`: fixed vector (pepper, device ID, jkt) gives a fixed string;
  length equals the digits value; leading zeros kept; a different `jkt` or device ID gives a
  different code for the vector set.
- `TestVerificationCodeEqual`: spaces ignored, different lengths false.
- `TestPairingCodeFormatAndNormalize`.
- `TestPairingStateMachine`: every allowed and every forbidden transition.

Integration:

- `TestIDT12_PendingDeviceHasTokensButNoAccess`: with the pending device's tokens `GET
  /api/v1/me` `200`; `GET /api/v1/devices/{own}` `200` with `verification_code`; `GET
  /api/v1/users` and `GET /api/v1/devices/{other device of the person}` `403 device_pending`;
  refresh `200`; logout `204`.
- `TestVerificationCodeOnlyForPendingDeviceItself`: owner and another active device of the
  person read the device without `verification_code`.
- `TestApproveDeviceBySelfFromActiveDevice` and `TestApproveDeviceByOwner`: correct code,
  device `active`, audit entry, then the former pending device reads `GET /api/v1/users` `200`.
- `TestIDT13_WrongCodeRevokesDevice`: device `revoked` with `verification_failed`, keys revoked,
  sessions revoked with `device_revoked`, families revoked, one `session.revoked` event per
  session, response `200` with `state: revoked`; the pending device's next request `401`.
- `TestApproveDeviceNeedsStepUp` (`401 step_up_required`), `TestApproveDeviceFromPendingDevice`
  (`403 device_pending`), `TestApproveForeignDeviceByNonOwner` (`404`),
  `TestApproveActiveDevice` (`409 invalid_state_transition`).
- `TestPendingDeviceExpires`: clock past `auth.device_pending_ttl`: requests `401`; approval
  `409`; after the cleanup run the row is `revoked` with `pending_expired`.
- `TestPairingHappyPath`: begin `201`, complete `202`, approve `201`, complete `201` with
  `auth_method: device_pairing` and device `active`, pairing `completed`.
- `TestPairingWrongCodeEndsStepUp`: approve with an unknown code `404`; the next approve with
  the right code and no new step-up `401 step_up_required`; after a new step-up `201`.
- `TestPairingCompleteWithOtherKeyRejects`: `401 invalid_dpop_proof`, pairing `rejected`, then
  `410` with the right key.
- `TestPairingExpiry`: requested past TTL `410`; approved but not collected past the window:
  pairing `expired`, device revoked with `pairing_expired`.
- `TestPairingBeginRateLimitedPerIP`: requests above the `rl.public_auth` burst from one IP
  `429`; another IP still `201`.
- `TestPairingCodeNeverLogged`: captured logs contain no pairing code and no verification code.
- The route-count test (AZ-P1) passes with the four operations.

Commands that must pass:

```sh
make api-lint api-checks codegen-check arch migrations-lint config-doc-check docs
cd services/server && go test -race ./internal/identity/... ./internal/authz/...
make check
```

## Acceptance criteria

- [ ] openapi.yaml, 06, 08, threat model and PLAN row changed as in Scope; `make api-lint
      api-checks docs` green.
- [ ] ID-T12 and ID-T13 green and named after their IDs.
- [ ] The verification code is shown only to the pending device and is never stored or logged.
- [ ] A wrong verification code revokes the pending device with all its sessions and families.
- [ ] Pending devices are revoked after `auth.device_pending_ttl`, also with the job switched off
      (enforced at authentication).
- [ ] Pairing works end to end; a wrong pairing code ends the caller's step-up; a proof with
      another key rejects the pairing; expired pairings leave no active device.
- [ ] Rate limits apply per IP on the public pairing endpoints and per principal on approval.
- [ ] `make check` green, coverage at or above `quality.coverage.go_min_percent`.

## Definition of done

- Code and tests as above; doc comments on every exported symbol; comments explain why.
- `internal/identity/README.md` updated (table `device_pairings`, use cases, operations, job,
  revoke reasons); `services/server/CHANGELOG.md` entry under "Unreleased".
- Specification and API changes from Scope in the same branch; generated files regenerated;
  `docs/DECISIONS.md` updated. No ADR unless you take an architecture decision beyond this brief
  (then the next free number).
- `make check` green.
- `make review` and an independent code review over the diff, then the red team per
  `docs/quality/red-team-gate.md` (focus: approving a foreign device, guessing pairing codes,
  access of pending devices); verdicts in the report.
- Merge locally into the base branch only if review is clean, the red team verdict is `JA` and
  `make check` is green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then the
commits, the files with line counts, test results with counts, what is not proven, and follow-ups
that could not be done in scope with the reason (state which cleanup-job case was taken). Then
write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
