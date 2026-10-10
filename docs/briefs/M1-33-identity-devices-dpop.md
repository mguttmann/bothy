# M1-33: identity: devices, device keys and DPoP verification (ADR 0020)

Status: ready | Depends on: M1-28 | Etappe: M1

## Goal

After this ticket the `identity` module stores devices with their two public device keys (`dpop`
and `step_up`), validates and fingerprints those keys (RFC 7638) and verifies DPoP proofs (RFC 9449)
with server nonces and a per-process `jti` replay store. People and owners can list and read
devices over the API. Every later authentication path (sessions M1-34, step-up M1-35, logins M1-37
to M1-43, API tokens M1-44, gateway M1-64) uses this one verifier, so a stolen token without the
non-exportable device key is worthless (08 "Ziele" item 3, threat T-TK-01).

## Read first

- `docs/spec/08-identitaet.md`: "5.1 Registrierung", "5.2 Bestätigung", "5.4 Verwaltung",
  "6.1 Zwei Schlüssel je nativem Gerät", "7.1 Übersicht", "7.3 DPoP", "13. Schutz der Anmeldung",
  "15. Tests" (ID-T14, ID-T27), "16. Entscheidungen dieses Kapitels" (R-ID-04),
  "18. Konfigurationswerte dieses Kapitels".
- `docs/spec/03-datenmodell.md`: "1. Grundsätze" (items 1, 2, 3, 9, 12, 13), "4.2 Identität"
  tables `devices` and `device_keys`, "3. Aufbewahrungsklassen".
- `docs/spec/05-aktionen.md`: "2.1 Spalte Recht", "2.5 Entscheidung" (pending devices),
  "4.3 Menschen, Identität, Geräte, Tokens" (`device.list`).
- `docs/spec/07-rechte.md`: "3. Prinzipal-Kontext", "4.1 Entscheidungen und Gründe",
  "4.2 Ablauf", "4.3 Sichtbarkeit".
- `docs/spec/06-api.md`: "3.1 DPoP", "5. Fehler" (codes `invalid_dpop_proof`, `use_dpop_nonce`,
  `device_pending`, `not_found`, `cursor_invalid`), "6. Paginierung".
- `docs/spec/02-architektur.md`: "4.1 Regeln", "4.5 Querschnitts-Ports", "5. Schichten im Modul",
  "6. Code-Layout", "9. Request-Pfad", "14.1 Fehlerarten".
- `docs/spec/19-sicherheit.md`: "6.1 Eingabevalidierung im Server" (SEC-IN-1, SEC-IN-2, SEC-IN-4),
  "9. Logs ohne Geheimnisse und ohne Inhalte", "10. Geheimnisse".
- `docs/spec/15-client-architektur.md`: "9. Sichere Speicherung und Geräteschlüssel" (clients use
  ES256; EdDSA stays allowed by the data model).
- `docs/spec/04-sync-protokoll.md`: "6.3 Lebenszyklus" (the gateway reuses this verifier with
  `htm = GET` and the nonce from `hello`; not built here).
- ADR 0020 (`docs/adr/0020-geraetebindung-dpop-step-up-fuer-admin-aktionen.md`), ADR 0008, ADR 0017.
- `docs/api/openapi.yaml`: operations `listMyDevices`, `listUserDevices`, `getDevice`; schemas
  `Device`, `DevicePage`, `DevicePlatform`, `DeviceRegistration`, `DeviceKeyRegistration`,
  `Problem`; responses `Unauthorized`, `Forbidden`, `NotFound`; headers `WWW-Authenticate`,
  `DPoP-Nonce`; security scheme `dpopAuth`; parameters `Cursor`, `Limit`, `UserIdPath`,
  `DeviceIdPath`.
- `docs/security/threat-model.md`: T-TK-01, T-ID-05.
- `services/server/internal/identity/README.md`, `services/server/README.md`,
  `services/server/architecture/modules.yaml`, `docs/quality/gates.md`, `CONTRIBUTING.md`,
  `docs/PLAN.md` section "3. Arbeitsweise je Ticket" and the M1 row of this ticket.
- The merged code of M1-07 (`internal/platform/config`), M1-09 (`internal/platform/secrets`),
  M1-12 (`internal/platform/pg`, `pgtest`), M1-13 (`internal/kernel/txn`), M1-17
  (`services/server/api/operations.yaml`, generator), M1-18 (`internal/platform/httpserver`,
  `internal/kernel/actor`, `internal/kernel/errs`, the fake authenticator of its tests), M1-26
  (`internal/identity` principals and users, display-text validation) and M1-28
  (`internal/authz`, `kernel/authorize`, the pending-device rule).

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
6. Test first: every behaviour starts with a failing test (red output goes into the report), then
   code until green, then clean up.
7. No en dash and no em dash in any text you write (code comments, docs, commit messages).
8. White label: no company, person, host or domain names except example.com, example.org and the
   neutral names Alex, Sam, Robin.
9. Configuration values only through named keys from `docs/operations/configuration.md`; no
   literal limits or durations in code paths that the spec names.
10. Two keys per native device, `dpop` without and `step_up` with user verification per use
    (08 "6.1"; R-ID-04; Ruling L10). At most one active key per device and purpose; `jkt` is
    unique across all devices (03 `device_keys`).
11. DPoP after RFC 9449 exactly as 08 "7.3": `typ = dpop+jwt`, `alg` only `ES256` or `EdDSA`,
    `jwk` without private members, claims `jti`, `htm`, `htu` (without query and fragment), `iat`,
    `ath` when a token is presented, `nonce`; `iat` at most `auth.dpop_proof_max_age` old and at
    most `auth.clock_leeway` in the future; `jti` once per `htu` within that window, stored as a
    hash, per process; nonces from the `DPoP-Nonce` header, renewed after `auth.dpop_nonce_ttl`,
    current and previous accepted, proofs without `nonce` rejected.
12. Errors: `401` with `WWW-Authenticate: DPoP error="invalid_dpop_proof"` or
    `error="use_dpop_nonce"` and a fresh `DPoP-Nonce` (08 "7.3", 06 "3.1").
13. A pending device has tokens but every action except `self.read`, `device.list` for its own
    device, `auth.token.refresh` and `auth.logout` ends with `403 device_pending` (05 "2.5").
14. No view right gives `404`, never `403` (05 rule 2, ADR 0017); `device.list` is `self` or
    `owner` (05 "4.3").
15. Display texts (device names) with control or bidi control characters are rejected (SEC-IN-4).

## Scope

Work only in `services/server` and the docs named here.

1. Migration `services/server/migrations/<next free number>_identity_devices.sql` (goose, Up and
   Down): tables `devices` and `device_keys` exactly as 03 "4.2" (all columns, defaults, CHECKs,
   FKs with their `ON DELETE`, the unique index on `jkt`, the partial unique index
   `(device_id, purpose) WHERE revoked_at IS NULL`, the indexes `(user_id, state)` and
   `(state) WHERE state = 'pending'`). A comment names the retention class `security`.
2. sqlc queries `internal/identity/adapters/postgres/queries/devices.sql` and the repository in
   `internal/identity/adapters/postgres/devices.go`: insert device, insert key, get device by id,
   list devices of a user (keyset on `id DESC`), get active key by device and purpose, get key by
   `jkt`. No query on tables of other modules.
3. Port (root package `internal/identity`, files `devices.go` and `dpop.go`), Go names fixed:
   - Types `KeyPurpose` (`dpop`, `step_up`), `DeviceState` (`pending`, `active`, `revoked`),
     `Platform` (values of 03 `devices.platform`), `PublicKey{JWK json.RawMessage, Alg string,
     Attestation json.RawMessage}`, `RegisterDevice{DeviceID, UserID, Name, Platform, AppVersion,
     DPoPKey PublicKey, StepUpKey *PublicKey, State DeviceState, ApprovedBy *ID}`, `Device` (all
     fields of the API schema `Device` except `push_registrations`), `DeviceKey{ID, DeviceID,
     Purpose, JKT, Alg, PublicJWK, RevokedAt}`, `DevicePage{Items []Device, NextCursor string}`.
     IDs use the UUID type of `internal/kernel/ids`.
   - `type Devices interface`:
     `Register(ctx, RegisterDevice) (Device, error)` joins the ambient unit of work and is called
     by the login tickets (M1-37 to M1-43) with `State` `pending` or `active`;
     `Get(ctx, actor.Actor, deviceID) (Device, error)`;
     `ListMine(ctx, actor.Actor, Page) (DevicePage, error)`;
     `ListForUser(ctx, actor.Actor, userID, Page) (DevicePage, error)`;
     `ActiveKey(ctx, deviceID, KeyPurpose) (DeviceKey, error)` (error kind `not_found` when none).
   - `type ProofRequest struct{ Proofs []string; Method string; URL string; AccessToken string;
     ExpectedJKT string }` where `Proofs` holds every value of the `DPoP` header, `URL` is the
     absolute expected `htu`, `AccessToken` is empty when no token is presented and `ExpectedJKT`
     is the token's `cnf.jkt` or empty.
   - `type VerifiedProof struct{ JKT string; Alg string; PublicJWK json.RawMessage; IssuedAt
     time.Time; NonceStale bool }` (`NonceStale` is true when the proof used the previous nonce, so
     the caller attaches the current one to its response).
   - `type DPoP interface{ CurrentNonce() string; Verify(ctx, ProofRequest) (VerifiedProof, error) }`.
   - `type DPoPError struct{ Code string; Nonce string }` with `Code` exactly `invalid_dpop_proof`
     or `use_dpop_nonce`; it maps to error kind `unauthenticated` of `kernel/errs`.
4. Domain, standard library only:
   - `internal/identity/domain/jose`: `ParseJWK(raw) (JWK, error)`, `JWK.Thumbprint() string`,
     `ParseCompactJWS(s) (JWS, error)`, `JWS.Verify(JWK) error` for `ES256` and `EdDSA`.
   - `internal/identity/domain/dpop`: `NormalizeHTU(string) (string, error)`, the claim checks
     of "Design notes", the nonce encode and check functions (pure, key and time passed in).
   - `internal/identity/domain/device`: registration validation and the state machine
     `pending -> active`, `pending -> revoked`, `active -> revoked` (other transitions give
     `409 invalid_state_transition`). Approval and revocation use cases come in M1-37, M1-38 and
     M1-45; they use these transitions.
5. App: `internal/identity/app/devices.go` (Register, Get, ListMine, ListForUser, ActiveKey with
   `authorize` and the unit of work) and `internal/identity/app/dpop.go` (verifier composing
   domain checks, the nonce key and the outgoing port `ReplayStore{ CheckAndRemember(key [32]byte,
   until time.Time) bool }`).
6. Adapters: `internal/identity/adapters/replay` (in-memory `ReplayStore`, concurrency safe,
   entries expire); `internal/identity/adapters/httpapi/devices.go` (handlers of the three
   operations) and `internal/identity/adapters/httpapi/dpop.go` with
   `ExpectedHTU(publicURL *url.URL, r *http.Request) string` (origin of `server.public_url` plus
   the escaped request path) for M1-34 and later login handlers.
7. Test helpers `internal/identity/identitytest`: `NewDeviceKey(alg) (signer, PublicKey)`,
   `SignDPoPProof(signer, ProofClaims) string`, `SignJWS(signer, header, payload) string`. Later
   tickets (M1-34, M1-35, M1-64, M1-73) reuse them. Only `_test.go` files import this package; a
   test runs `go list -deps ./cmd/bothy` and fails if it appears.
8. Wiring in `internal/wire` (verifier, replay store, repository, handlers); the identity
   `Config` struct gets `DPoPNonceTTL`, `DPoPProofMaxAge`, `ClockLeeway`, `TokenPepperRef`,
   `PublicURL`, `PageSizeDefault`, `PageSizeMax` from the keys `auth.dpop_nonce_ttl`,
   `auth.dpop_proof_max_age`, `auth.clock_leeway`, `auth.token_pepper_ref`, `server.public_url`,
   `api.page_size_default`, `api.page_size_max`. Register any of these keys that the schema of
   M1-07 lacks, with the standard values of the chapter tables, and regenerate the reference with
   `make config-reference`. If `bothy secrets init` (M1-09) does not yet create the token pepper
   that `auth.token_pepper_ref` points to, add it there (random, 32 bytes, idempotent, mode 0600)
   with a test.
9. Enable `listMyDevices`, `listUserDevices`, `getDevice` in `services/server/api/operations.yaml`
   and regenerate with the generator command documented by M1-17 in `services/server/README.md`.
10. If `kernel/errs` cannot carry response headers on an error, add an optional header list to
    its error type and render it in `platform/httpserver` (the only change outside `identity`
    besides wiring, config schema, secrets init and pagination below), with a unit test.
11. If no keyset cursor helper exists in `internal/platform/httpserver` yet, add a generic one
    there (opaque base64url, bound to a filter fingerprint, `400 cursor_invalid` on mismatch) with
    unit tests; later list tickets reuse it.
12. Docs: `docs/spec/08-identitaet.md` "7.3 DPoP" gets the nonce construction, the replay store
    rule and the proof profile of "Design notes" (spec first, in the same change);
    `docs/DECISIONS.md` gets the rulings listed in "Design notes"; module README and
    `services/server/CHANGELOG.md` (Unreleased).

## Out of scope

- Sessions, access tokens, the `Authenticator`, refresh and logout (M1-34). This ticket does not
  authenticate requests; HTTP tests use the fake authenticator of M1-18.
- Step-up challenges and the use of the `step_up` key (M1-35).
- Creating devices from a login flow, enrollment codes, approval with verification code, pairing,
  expiry of pending devices (M1-37, M1-38).
- Device revocation, `DeviceRevoked`, closing connections with `4012` (M1-45, M1-65).
- Push registrations in `Device.push_registrations` (M3); omit the member.
- Gateway use of the verifier (M1-64), API tokens with `dpop_jkt` (M1-44).
- Automatic revocation after `auth.device_inactive_revoke_after` and device key rotation
  (08 "5.4"): no M1 ticket owns them yet; do not build them here.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-33-identity-devices-dpop`.
- Conventional Commits with scope `identity` (or `server`, `docs` where fitting), each signed off
  (`git commit -s`, DCO).
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope above except `services/server/CHANGELOG.md`,
  `docs/DECISIONS.md` and the generated references named here.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL runs as a throwaway container on a random
  port through the M1-12 test helper; without a Docker-compatible runtime the test fails, it never
  skips.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at
  the end.
- Never read or print secrets; tests generate their own keys and pepper.
- No network calls except package downloads for pinned tools. The RFCs are not fetched; this brief
  carries what you need.
- If the core contract (00, 03, 04, 05) lacks something this ticket needs, stop with
  `Verdict: BLOCKED` instead of building a deviation (P-11).

## Design notes

Device registration (`Register`):

1. `DeviceID` must be a UUIDv7 (`kernel/ids`); an existing id gives `409 conflict`.
2. `UserID` must be a user principal (`principals.kind = 'user'`); otherwise `not_found`.
3. `Name` is trimmed, must not be empty and must pass the display-text check of M1-26 (control and
   bidi characters rejected, SEC-IN-4); failures are `400 validation_failed` with a field pointer.
4. `Platform` one of `ios`, `android`, `macos`, `windows`, `web`, `cli`.
5. `DPoPKey` required, `StepUpKey` optional for every platform (08 "5.1": only native apps have
   one; `DeviceRegistration.step_up_key` is optional).
6. Each key: `Alg` is `ES256` or `EdDSA`. JWK rules: for `ES256` `kty = "EC"`, `crv = "P-256"`,
   `x` and `y` base64url without padding, each exactly 32 bytes, and the point lies on the curve
   (validate through `crypto/ecdh` `P256().NewPublicKey` with the uncompressed form); for `EdDSA`
   `kty = "OKP"`, `crv = "Ed25519"`, `x` exactly 32 bytes. Members `d`, `p`, `q`, `dp`, `dq`, `qi`,
   `k`, `oth` are rejected (private or symmetric material). A JWK member `alg`, if present, must
   equal `Alg`. Unknown members are ignored for validation and excluded from the thumbprint.
   Duplicate JSON keys are rejected (SEC-IN-2).
7. `jkt` is the RFC 7638 thumbprint: SHA-256 over the UTF-8 bytes of the JSON object with only the
   required members in lexicographic order and no whitespace, base64url without padding
   (`{"crv":"P-256","kty":"EC","x":"...","y":"..."}`, `{"crv":"Ed25519","kty":"OKP","x":"..."}`).
8. The `dpop` and `step_up` keys of one device must differ (different `jkt`), otherwise the
   step-up key would be usable without user verification: `400 validation_failed`. A `jkt` that
   already exists on any device gives `409 conflict`.
9. `State` is `pending` or `active`; `active` requires `ApprovedBy` (a principal id) and sets
   `approved_by` and `approved_at`; `revoked` is never a registration state.
10. Attestation JSON is stored unchanged in `device_keys.attestation`; it is never required in M1
    (`auth.require_key_attestation` stays unused here, 08 "6.1").

JWS profile (`domain/jose`, used by DPoP, access tokens in M1-34 and step-up in M1-35):

1. Compact serialization only: exactly three segments, base64url without padding; any `=`, `+`,
   `/` or whitespace is an error.
2. Header and payload are JSON objects; duplicate keys, invalid UTF-8 and nesting beyond the
   decoder limit of M1-18 are rejected.
3. `alg` must be in the caller's allowlist; `none` and every `HS*` value are always rejected.
   A `crit` member is rejected. `jku`, `x5u` and `x5c` are rejected (keys come only from `jwk` or
   from the server).
4. `ES256` signatures are exactly 64 bytes `R || S` (RFC 7518 section 3.4), verified with
   `crypto/ecdsa` over SHA-256 of the signing input; DER or other lengths are rejected. `EdDSA`
   uses `crypto/ed25519` with a 64-byte signature.
5. No JOSE library: the profile is two algorithms and one serialization, and the domain layer may
   not import third-party code (02 "5. Schichten im Modul"). Record this as a ruling in
   `docs/DECISIONS.md`.

DPoP proof checks (`Verify`), in this order; a failure stops and nothing is remembered:

1. Exactly one `DPoP` header value; otherwise `invalid_dpop_proof`.
2. Parse as JWS with allowlist `ES256`, `EdDSA`; header `typ` equals `dpop+jwt` (ASCII
   case-insensitive); header `jwk` passes the JWK rules above and its `alg` matches; the
   signature verifies with that `jwk`.
3. Payload: `jti` non-empty string, `htm` string, `htu` string, `iat` integer NumericDate;
   `nonce` string; `ath` string exactly when `AccessToken` is not empty, absent otherwise
   (06 "3.1" item 3). Missing or mistyped members: `invalid_dpop_proof`, except a missing
   `nonce`, which gives `use_dpop_nonce`.
4. `htm` equals `Method` exactly.
5. `NormalizeHTU(htu)` equals `NormalizeHTU(URL)`: scheme and host lower case, default port
   removed (`443` for `https`, `80` for `http`), query and fragment dropped, empty path becomes
   `/`, percent-encodings normalized (upper-case hex, unreserved characters decoded). Path
   comparison is otherwise exact.
6. `iat` not older than `auth.dpop_proof_max_age` and not later than now plus
   `auth.clock_leeway` (`kernel/clock`).
7. Nonce: decode and check as below; unknown, malformed or too old gives `use_dpop_nonce`.
8. With `AccessToken`: `ath` equals base64url(SHA-256(ASCII bytes of the token)); with
   `ExpectedJKT`: the proof key's thumbprint equals it. Mismatch: `invalid_dpop_proof`.
9. Replay: key `SHA-256(normalized htu || 0x00 || jti)`; `CheckAndRemember` with expiry
   now plus `auth.dpop_proof_max_age` plus `auth.clock_leeway`. A second use inside that window
   gives `invalid_dpop_proof`. Check and insert are one atomic step.

Nonce construction (stateless, so every process and every restart agrees; 08 says "über Prozesse
hinweg schützt der Nonce"):

- Key: HKDF-SHA256 (`crypto/hkdf`) over the token pepper loaded from `auth.token_pepper_ref`, empty
  salt, info `dpop-nonce/v1`, 32 bytes. Do not put the product name into the info string (02
  "6. Code-Layout" lists the only places with the product name).
- Window `w = floor(unix seconds / seconds of auth.dpop_nonce_ttl)`.
- Nonce = base64url without padding of `uint64 big-endian(w) || first 16 bytes of
  HMAC-SHA256(key, uint64 big-endian(w))`.
- Accepted: decoded length 24, MAC equal (`crypto/hmac.Equal`), `w` equals the current or the
  previous window. `NonceStale` is true for the previous window.
- `CurrentNonce()` returns the nonce of the current window. Record the construction as a ruling in
  `docs/DECISIONS.md` and in 08 "7.3".

Replay store: in-memory, sharded map plus expiry; no named size cap exists in the spec, so none
is invented; memory is bounded by the time window and the rate limits. Expired entries are
removed on insert and by a ticker owned by the lifecycle of `internal/platform/lifecycle`.

Error rendering: `DPoPError` maps to `401` with problem code `invalid_dpop_proof` or
`use_dpop_nonce`, header `WWW-Authenticate: DPoP error="<code>", algs="ES256 EdDSA"` and header
`DPoP-Nonce: <CurrentNonce()>` in both cases. The internal reason (for example `htu_mismatch`) goes
only to the debug log with the correlation id, never into the problem body (no oracle).

Authorization of the three operations (handlers call the app; the app calls `authorize`):

| Operation | Action | Resource passed to `authorize` | Outcome |
|---|---|---|---|
| `listMyDevices` | `device.list` | `user` = caller | own devices; a pending device gets `403 device_pending` |
| `listUserDevices` | `device.list` | `user` = path `user_id` | self or owner; other people and unknown ids `404 not_found` |
| `getDevice` | `device.list` | `device` with owner `user_id` | self or owner; a pending device may read only its own device id; other ids `404`; another own device from a pending device `403 device_pending` |

Load the device before `authorize` and return the same `404` body for "missing" and "not
visible" (02 "9.2"). If the pending rule of M1-28 cannot distinguish "own device" from "another
own device", change that one rule in `internal/authz` with its test; that change is in scope.

Lists: order by `id DESC` (UUIDv7, newest first), all states including `revoked`, page size
`api.page_size_default`, larger `limit` capped at `api.page_size_max`, no totals. Response
members follow `Device`: `key_thumbprint` is the `jkt` of the active `dpop` key,
`step_up_key_thumbprint` of the active `step_up` key; absent when there is none; null columns are
absent members, never JSON `null`.

## Tests (write them first)

Unit tests (`go test`):

- `TestJWKThumbprint_RFC9449Example`: JWK `{"kty":"EC","crv":"P-256","x":"l8tFrhx-34tV3hRICRDY9zCkDlpBhF42UQUfWVAWBFs","y":"9VE4jf_Ok_o64zbTTlcuNJajHmt6v9TDVrU0CdvGRDA"}`
  gives `0ZcOCORZNYy-DWpqq30jZyJGHTN0d2HglBV3uiguA4I` (RFC 9449 sections 4.1 and 6.1).
- `TestJWKThumbprint_RFC8037Example`: JWK `{"kty":"OKP","crv":"Ed25519","x":"11qYAYKxCrfVS_7TyWQHOg7hcvPapiMlrwIaaPcHURo"}`
  gives `kPrK_qmxVWaYVA9wwBF6Iuo3vVzz7TxHCTwXBygrS4k` (RFC 8037 appendix A.3). Both vectors are
  quoted from the RFCs; if one fails, compare the canonical JSON byte by byte before changing the
  algorithm, and report it if the vector itself looks wrong.
- `TestJWK_Rejects`: private member `d`, `kty = RSA`, `kty = oct`, wrong curve, `x` of 31 or 33
  bytes, point not on the curve, padded base64, duplicate keys, `alg` member mismatch.
- `TestJWS_Rejects`: two or four segments, padding, `alg` `none`, `HS256`, `RS256`, `crit`,
  `jku`, `x5u`, `x5c`, DER-encoded ES256 signature, flipped signature bit.
- `TestDPoP_IDT14_*` (table test, one case each, expected code in brackets): wrong `htm`
  [`invalid_dpop_proof`], wrong `htu` host, path and scheme [`invalid_dpop_proof`], `htu` with
  query or fragment equal after normalization [accepted], wrong `ath` [`invalid_dpop_proof`],
  `ath` present without token [`invalid_dpop_proof`], `iat` older than `auth.dpop_proof_max_age`
  and newer than leeway [`invalid_dpop_proof`], repeated `jti` [`invalid_dpop_proof`], same `jti`
  with a different `htu` [accepted], missing nonce [`use_dpop_nonce`], nonce of two windows ago
  [`use_dpop_nonce`], forged nonce MAC [`use_dpop_nonce`], previous window [accepted with
  `NonceStale`], proof key differs from `ExpectedJKT` [`invalid_dpop_proof`], two `DPoP` headers
  [`invalid_dpop_proof`], `typ` `JWT` [`invalid_dpop_proof`]. The Bearer case of ID-T14 belongs to
  M1-34.
- `TestDPoP_NonceAcrossVerifiers`: two verifier instances with the same pepper accept each other's
  nonces; a different pepper rejects them.
- `TestDPoP_ReplayConcurrent`: the same valid proof sent from many goroutines passes exactly once
  (`-race`).
- `TestReplayStore_Expiry`: an entry is forgotten after its expiry (steerable clock).
- `TestDPoPError_Rendering`: `401`, problem code, `WWW-Authenticate` and `DPoP-Nonce` headers
  present; no internal reason in the body.
- `TestDevice_StateMachine`: allowed and forbidden transitions.
- `TestRegister_Validation`: every rule of "Device registration", including equal `dpop` and
  `step_up` keys and control or bidi characters in the name (SEC-T07, part devices).
- Fuzz targets with seed corpus in `testdata/fuzz/<Name>/` (seeds run in every `go test`):
  `FuzzParseJWK`, `FuzzParseCompactJWS`, `FuzzVerifyDPoPProof`. Property: no panic; an accepted
  input verifies again with the same result; seeds include valid ES256 and EdDSA proofs from
  `identitytest` and the malformed cases above.

Integration tests against PostgreSQL 18 (M1-12 helper):

- `TestDevicesMigration_Constraints`: CHECKs on `platform`, `state`, `purpose`, `alg`; unique
  `jkt`; second active key of the same purpose fails, a revoked one plus a new one succeeds;
  `ON DELETE CASCADE` from `devices` to `device_keys`.
- `TestRegister_Persists`: both keys stored with `jkt`, `alg`, attestation; `active` sets
  `approved_by` and `approved_at`; duplicate device id and duplicate `jkt` give `409 conflict`.
- HTTP tests with the fake authenticator of M1-18 and real `authz`: `TestListMyDevices`,
  `TestListUserDevices_SelfOwnerOther` (other person `404`, owner `200`, unknown user `404`),
  `TestGetDevice_PendingOwnOnly`, `TestListMyDevices_PendingForbidden`, `TestDevices_Pagination`
  (stable against new rows, foreign cursor `400 cursor_invalid`, `limit` above
  `api.page_size_max` capped), `TestDevice_NoNullMembers` (response validation of the test build
  of M1-18).
- `TestIdentitytest_NotInBinary`.

Commands that must pass:

```sh
cd services/server && go test -race ./internal/identity/... ./internal/platform/httpserver/... ./internal/authz/...
make arch
make codegen-check
make migrations-lint
make config-reference && git diff --exit-code docs/operations/configuration.md
make check
```

Run each fuzz target once locally with `go test -run=^$ -fuzz=<Name> -fuzztime=<duration of your
choice>` and report duration and result; commit any crashing input as a seed after the fix.

## Acceptance criteria

- [ ] Migration creates `devices` and `device_keys` exactly as 03 "4.2"; `make migrations-lint`
      green.
- [ ] `Register` enforces every rule of "Device registration"; one active key per purpose; `jkt`
      unique; `dpop` and `step_up` keys differ.
- [ ] Thumbprints match the RFC 9449 and RFC 8037 vectors.
- [ ] `Verify` implements the checks and order of "DPoP proof checks"; ID-T14 cases without the
      Bearer case are green with the expected codes.
- [ ] Nonces verify across verifier instances with the same pepper; current and previous window
      accepted; proofs without nonce rejected with `use_dpop_nonce`.
- [ ] A proof is accepted at most once per `htu` within the window, also under concurrency.
- [ ] Fuzz targets with seed corpus run in every `go test`.
- [ ] `listMyDevices`, `listUserDevices`, `getDevice` are enabled, authorize with `device.list`,
      give `404` for invisible devices and `403 device_pending` as specified.
- [ ] 08 "7.3" documents nonce construction, replay store and proof profile; DECISIONS.md has the
      rulings (stdlib JOSE profile, nonce construction).
- [ ] `make check` green.

## Definition of done

Code, tests and docs are complete: the identity README (status, ports `Devices` and `DPoP`, tables
`devices` and `device_keys`, actions, config keys, test ids), a doc comment on every public symbol
(purpose, parameters, return value, errors, example where useful), comments explain why, an ADR
only if you take an architecture decision beyond ADR 0020, the CHANGELOG entry under Unreleased of
`services/server/CHANGELOG.md`, regenerated references. `make check` is green. Run an independent
review (`make review` and a code review over the diff) and the red team gate
(`docs/quality/red-team-gate.md`) over the diff; record both verdicts in the report. Merge locally
into the base branch only if the review is clean, the red team verdict is `JA` and `make check` is
green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then the
commits, the files with line counts, the test results with counts (including the red output of
the first failing tests), the fuzzing runs, what is not proven, and follow-ups that could not be
done in scope, each with its reason. Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or
`exit=1 <reason>`.
