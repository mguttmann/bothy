# M1-37: First setup, enrollment codes and setup lock

Status: ready | Depends on: M1-19, M1-36 | Etappe: M1

## Goal

After this ticket a fresh instance answers every public request except the discovery-class reads
with `503 setup_required` until the operator runs `bothy setup bootstrap-owner` on the host. That
command creates the first owner in one transaction and prints a one-time enrollment code. The owner
redeems the code on a device that registers only its device keys (no passkey needed, contradiction
W5) and gets an active device with a DPoP-bound session. Owners issue enrollment codes for local
accounts over the API, and lost access is recovered on the host with
`bothy setup issue-enrollment-code`. The ticket also builds the shared login-completion use case
that every later login flow (M1-38 to M1-43) reuses, so device, session and token creation exist
exactly once.

## Read first

- `docs/spec/08-identitaet.md`: "Lokale Konten", "Zuordnung einer externen Identität",
  "Einladungs-Code", "Erstinstallation", "Registrierung" and "Bestätigung" (section 5), "DPoP",
  "Schutz der Anmeldung", "Tests" (ID-T06, ID-T07, ID-T11, ID-T27), "Entscheidungen dieses
  Kapitels" (R-ID-07, R-ID-09), "Konfigurationswerte dieses Kapitels".
- `docs/spec/03-datenmodell.md`: "Grundsätze" 1, 9, 12, 13; tables `instance`, `principals`,
  `users`, `owners`, `identity_providers`, `user_identities`, `enrollment_codes`, `devices`,
  `device_keys`, `auth_sessions`, `refresh_token_families`, `idempotency_keys`, `audit_events`.
- `docs/spec/05-aktionen.md`: rules 1.5 and 1.10, "Spalten Owner, Step-up, Audit, Rechte",
  "Entscheidung"; actions `instance.discovery.read`, `instance.bootstrap`, `auth.login.complete`,
  `user.create`, `user.update`.
- `docs/spec/02-architektur.md`: "Befehle des Binarys", "Modulliste", "Erlaubte Abhängigkeiten",
  "Querschnitts-Ports", "Schichten im Modul", "Transaktionen und Unit of Work", "Request-Pfad",
  "Fehlerarten", "Start".
- `docs/spec/06-api.md`: "DPoP", "Anmeldeabläufe", "Fehler", "Idempotenz", "Endpunkte" (Auth,
  Users), "Vertragstests und Gate".
- `docs/spec/07-rechte.md`: "Prinzipal-Kontext", "Ablauf".
- `docs/spec/16-betrieb.md`: "Zwei Listener", "Rollen" (database roles).
- `docs/operations/install-compose.md`: section "Start".
- `docs/PLAN.md`: section 3 "Arbeitsweise je Ticket"; section 7 "Bekannte Widersprüche" W3, W5.
- ADR 0013, ADR 0015, ADR 0019, ADR 0020, ADR 0021.
- `docs/api/openapi.yaml`: operations `createEnrollmentCode`, `createUser`, `beginEnrollment`,
  `completeEnrollment`, `getJwks`, `getDiscovery`; schemas `DeviceRegistration`, `TokenResponse`,
  `EnrollmentCode`, `UserCreate`, `UserCreated`; responses `ServiceUnavailable`, `Unauthorized`.
- `docs/security/threat-model.md`: T-ID-03, T-ID-04.
- Code of the dependencies: `services/server/internal/identity` (M1-26, M1-31, M1-33, M1-34,
  M1-35, M1-36), `internal/instance` (M1-15, M1-32), `internal/platform/httpserver` (M1-18,
  M1-19, M1-20), `api/routes` and `services/server/api/operations.yaml` (M1-17),
  `internal/kernel/auditing` (M1-24), `internal/platform/pg/pgtest` (M1-12).

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
   `example.com`; example secrets contain `EXAMPLE`. Configuration values only through named keys
   listed in `docs/operations/configuration.md`.
7. Until `instance.setup_completed_at` is set, the server answers only health checks and
   `instance.discovery.read`; every other request ends with `503 setup_required`. There is no
   "first visitor becomes owner" (08 "Erstinstallation", R-ID-07, T-ID-04).
8. The first owner is created only by a host command, directly against the database, in one
   transaction with the six steps of 08 "Erstinstallation" (check, principal and owner, signing
   keys, enrollment code, audit with system action `instance.bootstrap` and no actor, mark setup
   complete).
9. Enrollment codes: at least 128 random bits from the cryptographic generator, shown exactly once,
   stored only as HMAC-SHA-256 with the server pepper (`auth.token_pepper_ref`), valid
   `auth.enrollment_code_ttl`, single use, invalid after `auth.enrollment_code_max_attempts` failed
   attempts, never sent by e-mail (08 "Einladungs-Code", 03 Grundsatz 9, ID-T11, ID-T27).
10. Redeeming a code confirms the first device: the device that redeems it becomes `active`
    (08 "Einladungs-Code", "Bestätigung").
11. Recovery on the host only for active owners, with an audit entry (08 "Erstinstallation",
    R-ID-09).
12. No information about accounts or codes: every rejected code yields the same answer
    (08 "Schutz der Anmeldung").
13. Every route goes through `authorize` with its catalogue action; the system action
    `instance.bootstrap` is not reachable over API or gateway (05 rules 1.1 and 1.10).

## Scope

Specification first, in this order and on this branch, before production code (plan principle 3):

1. `docs/api/openapi.yaml` (resolves W5 for redemption without passkey):
   - New operation `POST /auth/enroll/redeem`, `operationId: redeemEnrollmentCode`, tag `Auth`,
     summary "Redeem an enrollment code with device keys and log in", `x-authz-action:
     auth.login.complete`, `x-rate-limit-class: rl.public_auth`, `security: []`, parameter
     `DpopProof`, no `Idempotency-Key` (login flows are protected by single-use codes, 06
     "Idempotenz" point 3). Request schema `EnrollRedeemRequest` with required `enrollment_code`
     (string) and `device` (`DeviceRegistration`), `additionalProperties: false`. Responses `201`
     `TokenResponse` (example with `auth_method: enrollment_code`, `state: active`), `400`, `401`,
     `409`, `429`, `default`.
   - `EnrollmentCode`: add optional `purpose` (enum `first_login`, `recovery`, `identity_link`).
   - `ServiceUnavailable`: add problem code `setup_required` to the description list.
   - `createEnrollmentCode` and `createUser`: description says the code in the response is a
     one-time secret that a replay with the same `Idempotency-Key` does not repeat (`409`).
   - Leave `beginEnrollment` and `completeEnrollment` untouched; M1-40 builds them (enrollment
     with passkey).
2. Regenerate the generated blocks of `docs/spec/06-api.md` (endpoint table "Auth", problem table)
   with the generator from M1-03 (`tools/api-checks`, regeneration command in its README). Add to
   06 "Idempotenz" a row: operations whose success response carries a one-time secret store the
   response without the secret; a replay answers `409 conflict` with `reason: one_time_secret`.
   Add the problem code `setup_required` (503, "Instanz ist noch nicht eingerichtet", client
   reaction "Owner richtet die Instanz am Host ein") to the source of the problem table.
3. `docs/spec/08-identitaet.md`:
   - "Einladungs-Code": format (Design notes), purposes and who may consume them, one valid code
     per person (issuing expires older unused codes), definition of a failed attempt, redemption
     via `POST /api/v1/auth/enroll/redeem`.
   - "Erstinstallation": the command also creates the `local` identity provider if none exists and
     a local identity for the owner; it uses the application database role; exit codes.
   - "Lokale Konten" table, row "Einladungs-Code": redemption with device keys only, passkey
     optional (M1-40).
4. `docs/spec/16-betrieb.md`: in "Rollen" add `bothy setup` to the users of `bothy_app`; in
   "Rand und Identität" add two sentences: host access is the root of trust of the instance; the
   first owner and recovery codes come only from `bothy setup` (08 refers to this).
5. `docs/operations/install-compose.md`, section "Start": replace `bothy owner create --login ...`
   with `docker compose exec api bothy setup bootstrap-owner --handle <handle> --display-name
   "<Name>"` and drop the sentence that calls the name a proposal (W3).
6. `docs/PLAN.md`: row M1-37 names `redeemEnrollmentCode` instead of `beginEnrollment` and
   `completeEnrollment`; row M1-40 "Einladung mit Passkey" names `beginEnrollment` and
   `completeEnrollment`. Mark W3 and W5 (redemption part) as resolved by M1-37.
7. `docs/DECISIONS.md`: one entry per ruling listed under Design notes "Rulings to record".

Production code under `services/server/`:

- `migrations/<next sequence>_identity_enrollment_codes.sql`: table `enrollment_codes` exactly as
  in 03 (columns, checks, FKs, unique index on `code_hash`, partial index `(user_id) WHERE used_at
  IS NULL`, index `(expires_at)`). Use the next free goose sequence number; `make migrations-lint`
  must pass.
- `internal/identity/adapters/postgres/queries/enrollment_codes.sql` plus regenerated sqlc code.
- `internal/identity/domain/enrollment.go`: code generation, normalisation, display format,
  purpose rules, attempt rules (pure functions).
- `internal/identity/app/enrollment.go`: service `EnrollmentCodes` with `Issue`, `Match`,
  `RecordFailedAttempt`, `MarkUsed` (used by M1-40 and M1-41, so keep them exported within the
  module); use case `RedeemEnrollmentCode`; use case `CreateEnrollmentCode` (API).
- `internal/identity/app/login_completion.go`: use case `CompleteLogin` (Design notes).
- `internal/identity/app/bootstrap.go`: use cases `BootstrapOwner` and `IssueRecoveryCode`.
- `internal/identity/adapters/httpapi`: handlers `RedeemEnrollmentCode`, `CreateEnrollmentCode`;
  extend the `CreateUser` handler for `local_account`.
- `internal/platform/httpserver/setupgate.go`: middleware `SetupGate` with the consumer interface
  `SetupState`; `internal/wire` connects it to the setup state of `internal/instance` (M1-15).
- One-time secret support in the idempotency layer (`internal/kernel/idempotency` and the
  middleware from M1-20), unless M1-20 already provides an equivalent; see Design notes.
- `cmd/bothy`: subcommands `setup bootstrap-owner` and `setup issue-enrollment-code`.
- `services/server/api/operations.yaml`: enable `redeemEnrollmentCode` and `createEnrollmentCode`
  (P-04); regenerate `api/rest` and `api/routes` (`make codegen-check` clean).
- `internal/kernel/errs` (or the code registry M1-18 uses): code `setup_required`, kind
  `unavailable`, status 503.
- `internal/identity/README.md` (ports, tables, actions, commands), `services/server/CHANGELOG.md`.

Public interfaces (Go, names fixed; adapt the UUID type to the one `kernel/ids` provides):

```go
// internal/platform/httpserver
type SetupState interface {
    // SetupCompleted reports whether instance.setup_completed_at is set.
    SetupCompleted(ctx context.Context) (bool, error)
}
func SetupGate(state SetupState, routes RouteLookup) func(http.Handler) http.Handler

// internal/identity/app
type DeviceTrust int
const (
    TrustConfirmed   DeviceTrust = iota + 1 // enrollment code, device pairing: new device is active
    TrustUnconfirmed                         // passkey, password, OIDC, trusted header: new device is pending
)
type LoginCompletion struct {
    UserID     ids.ID
    IdentityID *ids.ID           // nil for logins without a provider identity
    AuthMethod string            // one of auth_sessions.auth_method
    Device     DeviceRegistration // validated registration from the request (M1-33 type)
    ProofJKT   string            // RFC 7638 thumbprint of the key that signed the DPoP proof
    Trust      DeviceTrust
    ClientIP   netip.Addr
}
type LoginResult struct { Session SessionView; Device DeviceView; Tokens TokenPair; NewDevice bool }
func (s *Service) CompleteLogin(ctx context.Context, in LoginCompletion) (LoginResult, error)

type EnrollmentPurpose string // "first_login" | "recovery" | "identity_link"
type IssuedEnrollmentCode struct { ID ids.ID; Code string; Purpose EnrollmentPurpose; ExpiresAt time.Time }
type MatchedEnrollmentCode struct { ID, UserID ids.ID; Purpose EnrollmentPurpose }
func (e *EnrollmentCodes) Issue(ctx context.Context, userID ids.ID, purpose EnrollmentPurpose, createdBy *ids.ID) (IssuedEnrollmentCode, error)
func (e *EnrollmentCodes) Match(ctx context.Context, code string, allowed ...EnrollmentPurpose) (MatchedEnrollmentCode, error) // row locked FOR UPDATE in the ambient UoW
func (e *EnrollmentCodes) RecordFailedAttempt(ctx context.Context, codeID ids.ID) error           // own transaction
func (e *EnrollmentCodes) MarkUsed(ctx context.Context, codeID, deviceID ids.ID) error
```

## Out of scope

- Device approval with verification code, pairing, pending-device expiry (M1-38).
- Password with TOTP (M1-39); passkeys and the enrollment with passkey `beginEnrollment` and
  `completeEnrollment` (M1-40); OIDC login and linking with an `identity_link` code (M1-41);
  trusted header login (M1-43).
- `setup_required` and other fields in the discovery document (M1-46); this ticket only lets
  discovery-class operations through the gate.
- Gateway handshake (M1-64); the gate covers the gateway path because it wraps the public listener.
- Cleanup of expired codes (retention jobs, M7).

## Boundaries

- Work in a dedicated git worktree on branch `feat/M1-37-identity-bootstrap`.
- Conventional Commits with module scope (for example `feat(identity): redeem enrollment codes`)
  and DCO sign-off (`git commit -s`).
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside Scope, except `docs/DECISIONS.md`, the `docs/PLAN.md` rows named in
  Scope and CHANGELOG entries.
- Follow the host rules supplied by the launcher. Tests bind only `127.0.0.1` and port 0;
  PostgreSQL runs only as the disposable container of `pgtest`.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at
  the end.
- Never read or print secrets. No network calls except package downloads for pinned tools.
- If an element of the core contract (chapters 00, 03, 04, 05) that this brief names is missing
  or different, stop with `Verdict: BLOCKED` (P-11) instead of building a deviation.

## Design notes

Enrollment code format and storage:

- 20 random bytes from `crypto/rand` (160 bits, above the 128-bit floor) encoded as 32 Crockford
  Base32 characters (alphabet `0123456789ABCDEFGHJKMNPQRSTVWXYZ`). Display form: `en_` plus eight
  groups of four characters joined by `-`, for example `en_EXAM-PLE0-...` in docs only with the word
  `EXAMPLE` in the value.
- Normalisation of input: trim, drop an optional `en_` prefix (any case), remove `-` and spaces,
  uppercase, map `O` to `0` and `I`, `L` to `1`; anything that is not 32 alphabet characters is not
  a code.
- Stored value: `code_hash = HMAC-SHA-256(key = pepper, msg = "enrollment-code/v1" || 0x00 ||
  canonical)`. The pepper comes from the secret provider through `auth.token_pepper_ref`; reuse the
  pepper accessor that M1-34 uses for refresh tokens. The code itself never reaches a log, an
  audit detail, an event or `river_job`; logs carry only the code ID.
- The generator takes an `io.Reader`; the package variable used in production is
  `crypto/rand.Reader`. No package under `internal/identity` imports `math/rand` or
  `math/rand/v2` (test in ID-T27).

Purposes (column `enrollment_codes.purpose`):

| Purpose | Issued by | May be consumed by |
|---|---|---|
| `first_login` | `bootstrap-owner`; `createUser` with `local_account: true`; `createEnrollmentCode` for a person with a local identity and no device row yet | `redeemEnrollmentCode`, `completeEnrollment` (M1-40), OIDC linking (M1-41) |
| `recovery` | `bothy setup issue-enrollment-code`; `createEnrollmentCode` for a person with a local identity and at least one device row | `redeemEnrollmentCode`, `completeEnrollment` (M1-40) |
| `identity_link` | `createEnrollmentCode` for a person without a local identity | OIDC linking only (M1-41) |

Issuing a code sets `expires_at = now` on every other unused, unexpired code of the same person, so
at most one code per person is valid. Reason: a forgotten older code would stay a usable secret.

Failed attempts: a code can only be attributed when it matches by hash. A failed attempt is a
request whose code matched an unused, unexpired code with remaining attempts and that was then
rejected (device registration invalid, DPoP proof key not equal to `device.public_jwk`, unknown
platform or algorithm, device ID already taken; for M1-40 also a failed WebAuthn verification).
`RecordFailedAttempt` increments `attempts` in its own short transaction after the failing unit
of work rolled back, so the count survives the rollback. A code with `attempts >=
auth.enrollment_code_max_attempts` is invalid. Non-matching codes are limited per client IP by
`rl.public_auth` and audited as denials.

Redemption `redeemEnrollmentCode`, in this order:

1. `authorize(anonymous, auth.login.complete, instance)` through the normal chain.
2. Verify the DPoP proof as a login proof without `ath` (06 "DPoP" point 3) with the verifier
   from M1-33, including the nonce rules. Failure: `401 invalid_dpop_proof` or `use_dpop_nonce`;
   the code is not looked up.
3. Normalise the code. Malformed, unknown, used, expired (`now >= expires_at`), exhausted, wrong
   purpose, or the person is not `principals.status = 'active'`: `401 unauthenticated` with the
   same body (only `correlation_id` differs) and an audit denial. Lookup uses `Match`, which locks
   the row with `SELECT ... FOR UPDATE` inside the unit of work.
4. `CompleteLogin` with `TrustConfirmed`, `AuthMethod = "enrollment_code"`, `IdentityID` = the
   person's active local identity if one exists, otherwise nil. Rejections here roll back and call
   `RecordFailedAttempt`.
5. `MarkUsed` sets `used_at` and `used_by_device_id` in the same unit of work. Respond `201` with
   `TokenResponse`.

Two parallel redemptions of the same code: the row lock serialises them; the second sees `used_at`
and gets the generic `401`.

`CompleteLogin` (shared by all login flows):

- `ProofJKT` must equal the RFC 7638 thumbprint of `Device.public_jwk`, else `401
  invalid_dpop_proof`.
- Unknown `device_id` (UUIDv7, checked with `kernel/ids`): create `devices` (state `active` for
  `TrustConfirmed`, `pending` for `TrustUnconfirmed`; `approved_at` set only when active) and
  `device_keys` (`dpop`, and `step_up` when `Device.step_up_key` is present, both with `jkt`).
- Known `device_id` of the same person, not revoked, whose active `dpop` key has `jkt =
  ProofJKT`: reuse it; its state does not change. Any other known `device_id` (other person,
  revoked, other key): `409 conflict`.
- The person must be `active`; otherwise `401 unauthenticated`. M1-41 extends this for
  provisioned `pending` people; do not add that here.
- Create the session, refresh family (`dpop_jkt = ProofJKT`) and token pair with the session
  issuing code from M1-34; `auth_sessions.identity_id = IdentityID`, `auth_method = AuthMethod`,
  `last_ip = ClientIP`. Set `user_identities.last_login_at` when `IdentityID` is set.
- Exactly one audit entry for `auth.login.complete` per completed login, decision `allow`, details
  `{auth_method, device_id, session_id, new_device, device_state}`. If the generic per-action audit
  from M1-24/M1-28 already writes the entry, add the details there; never two entries (test).

Setup gate:

- Wraps the public listener only (`server.listen_addr`); the internal listener with `/healthz`
  and `/readyz` is untouched.
- Allowed before setup: every operation whose action in the generated route table is
  `instance.discovery.read` (today `getDiscovery`, `getJwks`, `getWebhookKeys`,
  `getBrandingLogo`, `getProblemType`). Derive the list from `api/routes`, never hardcode it.
- Everything else, including unknown paths and the gateway path, gets `503` with problem code
  `setup_required` and no `Retry-After` (setup waits for a person, not for time).
- The gate runs before authentication (request path step 4) and before rate limiting, so a
  request carrying any token still gets `503`. Resolve the operation with the same route table
  that `api/routes` uses (for example `(*http.ServeMux).Handler` to get the pattern without
  dispatching).
- `SetupCompleted` is read per request until it returns `true`; then the result is cached for the
  process lifetime (an `atomic.Bool`), because setup never reverts. A database error answers
  `503 unavailable` (fail closed). Bootstrap on the host takes effect without a server restart.

`bothy setup bootstrap-owner`:

- Flags: `--handle` (required), `--display-name` (required), `--locale` (default `de`),
  `--time-zone` (default `UTC`), plus the global `--config`. Handle and display name are validated
  with the rules from M1-26 (handle pattern, `principal.handle_max_chars`, no control or bidi
  characters).
- Uses the normal configuration and the application database role `bothy_app` (same as `serve`):
  the transaction only writes rows, so the schema owner role is not needed (least privilege).
- Before the transaction: ensure the active signing keys through the `instance` keys port from
  M1-32 (idempotent; if that function opens its own transaction, call it before the bootstrap
  transaction).
- One unit of work with the rights-change option for the new principal (`instance.bootstrap` has
  "Rechte" `ja`), system actor:
  1. Lock the instance record (`SELECT ... FOR UPDATE` through the instance port; if M1-15 creates
     the record lazily, call its ensure function first). `setup_completed_at` set: abort.
  2. Ensure a `local` identity provider: reuse the existing one, else create `kind = 'local'`,
     `name = 'Local accounts'`, `enabled = true`, `allow_provisioning = false`, `config = {}`
     (schema `identity_provider.local.v1`) through the provider store from M1-36 without
     `authorize` (system path).
  3. Create principal (`kind = 'user'`, `status = 'active'`), `users`, `owners` (`granted_by =
     NULL`, `reason = 'bootstrap'`), and the local identity (`subject` = handle).
  4. `Issue(first_login, createdBy = nil)`.
  5. Audit: actor NULL, action `instance.bootstrap`, decision `allow`, resource `instance`, details
     `{owner_principal_id, enrollment_code_id, local_provider_id}`.
  6. Set `setup_completed_at` through the instance port.
- After commit print to stdout, once: the handle, the principal ID, the code in display form and
  `expires_at` in RFC 3339 UTC. Nothing else prints the code.
- Exit codes: `0` success; `1` infrastructure error (configuration, secrets, database); `2`
  invalid arguments; `3` setup already completed (nothing changed).

`bothy setup issue-enrollment-code --user <handle>`:

- Setup must be complete (else exit `4`). The person must be an active owner (else exit `2` with
  "not an active owner"). Issues a `recovery` code (`created_by = NULL`), prints it once like
  above.
- Audit: actor NULL, action `user.update`, resource `user`, details `{command: "setup
  issue-enrollment-code", purpose: "recovery", enrollment_code_id}`. Reason: 05 has no system
  action for host-issued recovery codes; 03 allows a NULL actor for host commands.
- Recovery works for owners with or without a local identity; redemption then creates an active
  device without an identity.

`createEnrollmentCode` (`POST /users/{user_id}/enrollment-codes`, action `user.update`, owner
with step-up through the catalogue): target must be a human with `status = 'active'`, else `409
conflict` with `reason: user_not_active`; purpose per the table above; `201` with `code`,
`purpose`, `expires_at`.

`createUser` with `local_account: true` (extends M1-31): requires an enabled `local` provider (else
`409 conflict`, `reason: local_provider_disabled`); creates the local identity (`subject` =
handle) and a `first_login` code in the same unit of work and returns it as `enrollment`.

One-time secrets and idempotency: a response that carries a one-time secret is marked by the
handler (add `idempotency.MarkOneTimeSecret(ctx)` to `internal/kernel/idempotency` if M1-20 has
no equivalent). The idempotency layer then stores status and `{"one_time_secret": true}` as body
instead of the response; a replay with the same key answers `409 conflict` with `reason:
one_time_secret` and `hint` "issue a new code". Reason: `idempotency_keys.response_body` must not
hold a secret in plain text (03 Grundsatz 9).

Configuration used (all existing; add none): `auth.enrollment_code_ttl`,
`auth.enrollment_code_max_attempts`, `auth.token_pepper_ref`, `auth.session_max_age`,
`auth.refresh_token_ttl`, `auth.access_token_ttl`, `ratelimit.rl.public_auth.*`.

Error codes: `setup_required` (503, new), `unauthenticated`, `invalid_dpop_proof`,
`use_dpop_nonce` (401), `validation_failed` (400), `conflict` (409), `rate_limited` (429).

Rulings to record in `docs/DECISIONS.md`: W3 resolution and corrected install guide; W5 for
redemption (new operation `redeemEnrollmentCode`, passkey enrollment stays in M1-40); definition
of a failed attempt; one valid code per person; purposes table; audit of recovery codes as
`user.update` with NULL actor; `bothy_app` role for `bothy setup`; non-replayable one-time
secrets; setup gate placement and caching.

## Tests (write them first)

Go test names carry the spec test ID where one exists. Integration tests use `pgtest`
(PostgreSQL 18, disposable container, random port), the controllable clock from `kernel/clock`,
and the test authenticator and step-up helper from M1-31/M1-35.

Unit (`internal/identity/domain`):

- `TestEnrollmentCodeFormat`: 32 Crockford characters, 160 bits, display form with prefix and
  eight groups.
- `TestNormalizeEnrollmentCode`: table with lowercase, hyphens, spaces, `O`/`I`/`L` mapping,
  missing prefix, wrong length, character `U`, empty input.
- `TestIDT27_EnrollmentCodeEntropyAndSource`: generator reads exactly 20 bytes from the injected
  reader; the production reader is `crypto/rand.Reader`; `go list -deps` of every package under
  `internal/identity` contains neither `math/rand` nor `math/rand/v2`.
- `TestEnrollmentPurposeRules`: issuing purpose and allowed consumers per table.

Unit (`internal/platform/httpserver`):

- `TestSetupGateAllowsOnlyDiscoveryActions` with a fake route table and fake `SetupState`.
- `TestSetupGateCachesCompletion`: after the first `true` the fake is not called again.
- `TestSetupGateFailsClosed`: `SetupState` error gives `503 unavailable`.

Integration:

- `TestIDT06_RequestsBeforeSetupGet503`: `GET /api/v1/me` (with and without a token),
  `POST /api/v1/auth/enroll/redeem`, an unknown path and the gateway path return `503` with
  problem code `setup_required`; `GET /api/v1/instance/jwks` is not `503`; `/readyz` on the
  internal listener is unaffected; after `BootstrapOwner` the same `GET /api/v1/me` without a
  token returns `401` without a server restart.
- `TestIDT07_SecondBootstrapAbortsWithoutChange`: row counts of `principals`, `owners`,
  `user_identities`, `identity_providers`, `enrollment_codes`, `audit_events` and the instance
  record are equal before and after; exit code `3`.
- `TestBootstrapIsAtomic`: a fake audit recorder failing at step 5 leaves no row and
  `setup_completed_at` NULL.
- `TestBootstrapConcurrent`: two parallel runs, exactly one succeeds.
- `TestBootstrapAuditEntry`: one entry `instance.bootstrap`, actor NULL, details without the code.
- `TestBootstrapPrintsCodeOnce`: captured stdout contains the code exactly once; captured logs
  contain it zero times.
- `TestRedeemActivatesFirstDevice`: `201`; device `active`; session `auth_method =
  'enrollment_code'`; refresh family `dpop_jkt` equals the proof thumbprint; `used_at` and
  `used_by_device_id` set; one audit entry `auth.login.complete`.
- `TestIDT11_CodeUsedTwiceRejected`, `TestIDT11_ExpiredCodeRejected` (clock past `expires_at`),
  `TestIDT11_CodeRejectedAfterMaxAttempts` (after `auth.enrollment_code_max_attempts` failed
  attempts a correct request fails): all three bodies are equal except `correlation_id`.
- `TestRedeemFailedAttemptSurvivesRollback`: proof key differs from `device.public_jwk`;
  `attempts` is incremented although the unit of work rolled back.
- `TestRedeemUnknownCodeDoesNotTouchAttempts` and `TestRedeemWithoutDPoPProof` (no lookup,
  attempts unchanged).
- `TestRedeemConcurrentSameCode`: one `201`, one `401`.
- `TestRedeemRejectsDisabledPerson`.
- `TestIssueExpiresOlderCodes`.
- `TestCompleteLoginDeviceRules`: table for new device confirmed and unconfirmed, known device
  with same key (state kept), known device of another person, revoked device, other key, proof key
  mismatch.
- `TestCreateEnrollmentCodeRights`: owner with step-up `201`; owner without fresh step-up `401
  step_up_required`; non-owner human `403`; agent token `403`; disabled target `409`; purpose
  selection for the three cases.
- `TestOneTimeSecretNotStoredForReplay`: `idempotency_keys.response_body` of
  `createEnrollmentCode` and `createUser` contains no code; replay answers `409` with `reason:
  one_time_secret`.
- `TestCreateUserLocalAccountIssuesCode`.
- `TestRecoveryCommand`: active owner gets a `recovery` code and can redeem it; non-owner exit
  `2`; before setup exit `4`; audit entry `user.update` with actor NULL.
- The route-count test from M1-17/M1-25 (AZ-P1) passes with the two new operations.

Commands that must pass:

```sh
make api-lint api-checks codegen-check arch migrations-lint config-doc-check docs
cd services/server && go test -race ./internal/identity/... ./internal/platform/httpserver/... ./cmd/bothy/...
make check
```

## Acceptance criteria

- [ ] openapi.yaml, 06, 08, 16, install-compose.md and PLAN.md are changed as in Scope; `make
      api-lint api-checks docs linkcheck` green.
- [ ] ID-T06, ID-T07, ID-T11 and ID-T27 are green and named after their IDs.
- [ ] Before setup only `instance.discovery.read` operations answer without `503
      setup_required`; bootstrap takes effect without restart.
- [ ] `bothy setup bootstrap-owner` creates owner, local provider, local identity, code, audit
      entry and setup mark in one transaction; second run exits `3` with nothing changed.
- [ ] A redeemed code yields an active device and a DPoP-bound session; every rejected code yields
      the same `401` body.
- [ ] Failed attempts are counted only for matching codes and survive rollback.
- [ ] No code value appears in logs, audit details, events, jobs or `idempotency_keys`.
- [ ] `CompleteLogin` is the only place that creates devices, sessions and tokens for logins.
- [ ] `make check` green, coverage of the changed Go packages at or above
      `quality.coverage.go_min_percent`.

## Definition of done

- Code and tests as above; doc comments on every exported symbol (purpose, parameters, return
  value, errors); comments explain why.
- `internal/identity/README.md` lists the new table, use cases, operations, commands and the
  purposes table; `services/server/CHANGELOG.md` has an entry under "Unreleased".
- Specification and API changes from Scope are in the same branch; generated files are
  regenerated; `docs/DECISIONS.md` has the rulings. No ADR is needed unless you take an
  architecture decision beyond this brief; then write it with the next free number.
- `make check` green.
- `make review` and an independent code review over the diff, then the red team over the diff
  per `docs/quality/red-team-gate.md` (focus: setup takeover, code guessing, secrets at rest);
  verdicts recorded in the report.
- Merge locally into the base branch only if review is clean, the red team verdict is `JA` and
  `make check` is green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then the
commits, the files with line counts, test results with counts (unit, integration), what is not
proven, and follow-ups that could not be done in scope with the reason. Then write
`{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
