# M1-34: identity: sessions, access tokens, refresh families and logout

Status: ready | Depends on: M1-16, M1-24, M1-32, M1-33 | Etappe: M1

## Goal

After this ticket a person's app holds a session with a short-lived access token (JWS with
`cnf.jkt` and `sid`, no rights inside) and a rotating refresh token in a family bound to the
device's DPoP key. Reuse of a rotated refresh token revokes family and session; extensions get
child sessions with their own family; logout ends a session and its children. The `identity`
module implements `kernel/actor.Authenticator`, so every REST request of a person is
authenticated with token, DPoP proof, session and device state before `authorize` runs. This is
the token model of 08 "7. Token-Modell" that all login tickets (M1-37 to M1-43), the gateway
(M1-64) and step-up (M1-35) build on.

## Read first

- `docs/spec/08-identitaet.md`: "1. Ziele", "5.2 Bestätigung", "7.1 Übersicht",
  "7.2 Zugriffstoken", "7.3 DPoP", "7.4 Refresh-Tokens: Rotation, Familien, Wiederverwendung",
  "8. Sitzungsdauer", "13. Schutz der Anmeldung", "15. Tests" (ID-T14 Bearer case, ID-T15,
  ID-T16, ID-T17, ID-T27), "16. Entscheidungen dieses Kapitels" (R-ID-05),
  "18. Konfigurationswerte dieses Kapitels".
- `docs/spec/03-datenmodell.md`: "1. Grundsätze" (items 1, 2, 9, 11, 13), "4.2 Identität" tables
  `auth_sessions`, `refresh_token_families`, `refresh_tokens` (including the rotation rules under
  `refresh_tokens`), `devices`, `device_keys`; "4.1 Instanz" table `server_signing_keys`;
  "4.10" table `event_log`; "4.12" table `audit_events`; "6.3 Sperrreihenfolge";
  "9. Konfigurationswerte dieses Kapitels" (`auth.session_max_age`, `auth.refresh_token_ttl`,
  `auth.refresh_reuse_grace`, `identity.last_active_granularity`).
- `docs/spec/04-sync-protokoll.md`: "2.2 Umschlag (Envelope)", "8.5 Widerruf von Sitzung, Gerät
  oder Prinzipal".
- `docs/spec/05-aktionen.md`: "2.1 Spalte Recht", "2.4 Spalten Owner, Step-up, Audit, Rechte",
  "2.5 Entscheidung", "4.1 Öffentlich und Anmeldung" (`auth.token.refresh`, `auth.logout`),
  "4.3" (`session.list`), "6. Dauerhafte Ereignistypen und Empfangs-Aktion" (`session.revoked`).
- `docs/spec/07-rechte.md`: "3. Prinzipal-Kontext", "4.1 Entscheidungen und Gründe",
  "4.2 Ablauf" (`check_active`), "SR5", "SR7".
- `docs/spec/06-api.md`: "3. Authentifizierung je Prinzipal-Art", "3.1 DPoP", "5. Fehler",
  "6. Paginierung", "7. Idempotenz" item 3 (no `Idempotency-Key` on refresh).
- `docs/spec/02-architektur.md`: "4.3 Ports, Ereignisse und Quellen je Modul" (`identity`:
  `Auth`, `SessionEnded`), "4.4 Erlaubte Abhängigkeiten" (`identity` uses `instance`, `authz`),
  "4.5 Querschnitts-Ports" (`kernel/actor`), "8. Transaktionen und Unit of Work",
  "9. Request-Pfad".
- `docs/spec/19-sicherheit.md`: "9. Logs ohne Geheimnisse und ohne Inhalte", "10. Geheimnisse".
- `docs/spec/15-client-architektur.md`: "8. Netzwerk und Anmeldung" (single-flight refresh).
- ADR 0020, ADR 0021, ADR 0017.
- `docs/api/openapi.yaml`: operations `refreshToken`, `logout`, `listMySessions`,
  `listUserSessions`; schemas `TokenRefreshRequest`, `TokenResponse`, `Session`, `SessionPage`,
  `Device`, `SessionRevokedPayload`, `SessionRevokedEvent`, `EventEnvelope`, `Problem`;
  parameter `DpopProof`; security schemes `dpopAuth`, `bearerAuth`.
- `docs/security/threat-model.md`: T-TK-01, T-DV-01.
- The merged code of M1-16 (`kernel/eventlog.Appender`), M1-24 (`kernel/auditing.Recorder`),
  M1-28 (`internal/authz`, `check_active`), M1-32 (`internal/instance` port `Keys`, purpose
  `access_token`), M1-33 (`identity.Devices`, `identity.DPoP`, `domain/jose`, `identitytest`,
  `ExpectedHTU`), M1-18 (`platform/httpserver`, `kernel/actor`, problem mapping) and M1-15
  (instance record with `instance_id`).
- `services/server/internal/identity/README.md`, `docs/quality/gates.md`, `CONTRIBUTING.md`,
  `docs/PLAN.md` "3. Arbeitsweise je Ticket".

The plan index lists only M1-32 and M1-33 as dependencies; this ticket also writes
`session.revoked` (M1-16) and audit entries (M1-24). At start, check that both are merged; if not,
stop with `Verdict: BLOCKED dependency M1-16/M1-24 not merged`.

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
6. Test first: every behaviour starts with a failing test (red output goes into the report).
7. No en dash and no em dash in any text you write.
8. White label: only example.com, example.org and the names Alex, Sam, Robin in examples.
9. Configuration values only through named keys from `docs/operations/configuration.md`.
10. Access token: JWS signed with a server key of purpose `access_token`; claims `iss`
    (`instance_id`), `aud`, `sub`, `sid`, `did`, `jti`, `iat`, `nbf`, `exp`, `cnf.jkt`, `amr`; no
    rights in the token (08 "7.2"). Verification with `kid` among keys in state `active` or
    `retiring`, `iss`, `aud`, `exp` and `nbf` with `auth.clock_leeway`, then DPoP, then session and
    device (08 "7.2").
11. A DPoP-bound token used as `Bearer` is rejected (08 "7.3", RFC 9449 section 7.2).
12. Refresh token: 256 random bits, stored only as HMAC-SHA-256 with the server pepper; lifetime
    `auth.refresh_token_ttl`, never beyond `auth_sessions.expires_at` (08 "7.1"); one active family
    per session, bound to `dpop_jkt`; every refresh needs a proof with exactly that key (08 "7.4",
    03 `refresh_token_families`).
13. Rotation, grace and reuse exactly as 03 `refresh_tokens` items 1 to 3 with
    `auth.refresh_reuse_grace`; reuse revokes family and session (`refresh_reuse`), writes audit
    and `session.revoked` (08 "7.4").
14. Child sessions for extensions (`notification_ext`, `share_ext`) with their own family on the
    same device; revoking the parent revokes all children with the same reason (08 "7.4",
    R-ID-05, 03 `auth_sessions`).
15. Session absolute end `auth.session_max_age` (03, 08 "8").
16. `auth.logout` ends the own session; `session.list` is self or owner (05 "4.1", "4.3").
17. A pending device may refresh and log out but nothing else except `self.read` and its own
    `device.list` (05 "2.5").
18. Tokens and token hashes never appear in logs, events, audit details, telemetry or errors
    (SEC-LOG-1, SEC-SEC-3).

## Scope

1. Migration `services/server/migrations/<next free number>_identity_sessions.sql`: tables
   `auth_sessions`, `refresh_token_families`, `refresh_tokens` exactly as 03 "4.2" (columns,
   defaults, CHECKs including the three table checks of `auth_sessions`, FKs, unique and partial
   indexes, indexes). Exception: the FK `auth_sessions.identity_id -> user_identities(id)` is not
   created here because `user_identities` arrives with M1-36; the column exists and a comment
   says that M1-36 adds the constraint.
2. sqlc queries `internal/identity/adapters/postgres/queries/sessions.sql` and `refresh.sql`,
   repository `adapters/postgres/sessions.go`; the authentication lookup is one query: session by
   id joined with its device and the device's active `dpop` key.
3. Port in `internal/identity/auth.go` (names fixed):
   - `ClientComponent` (`app`, `notification_ext`, `share_ext`, `console`), `AuthMethod` (the six
     values of 03), `SessionRevokeReason` (the values of 03 `auth_sessions.revoke_reason`).
   - `StartSession{UserID, DeviceID, AuthMethod, IdentityID *ID}` (component `app`; `console`
     comes with M6-01).
   - `TokenSet{AccessToken, RefreshToken secrets-typed values; ExpiresIn int; RefreshExpiresAt
     time.Time; Session Session; Device Device}`; token values use the redacting secret type of
     `internal/platform/secrets` (M1-09) or an identity-local equivalent that prints
     `[redacted]`.
   - `Session` DTO with the members of the API schema `Session` (`current` is set by the handler).
   - `type Auth interface`:
     `StartSession(ctx, StartSession) (TokenSet, error)` for the login tickets, joins the ambient
     unit of work;
     `StartChildSession(ctx, parentSessionID ID, component ClientComponent) (TokenSet, error)`;
     `Refresh(ctx, refreshToken string, proof VerifiedProof) (TokenSet, error)`;
     `Logout(ctx, actor.Actor) error`;
     `RevokeSession(ctx, sessionID ID, reason SessionRevokeReason, by *ID) error` (also used by
     M1-35 and M1-45);
     `ListSessions(ctx, actor.Actor, userID ID, Page) (SessionPage, error)`.
   - In-transaction event `SessionEnded{SessionIDs []ID; UserID ID; DeviceID ID; Reason
     SessionRevokeReason}` published on every revocation (02 "4.3"; `notify` subscribes in M3).
4. Domain `internal/identity/domain/session`: the pure rotation decision
   `Decide(family, token, now, grace) Outcome` with outcomes `rotate`, `grace_rotate`, `reuse`,
   `expired`, `family_revoked`; claim building and validity capping (`exp` and refresh
   `expires_at` never beyond the session end).
5. App `internal/identity/app`: `sessions.go` (start, child, revoke, list), `refresh.go`,
   `logout.go`, `authenticator.go` (implements `kernel/actor.Authenticator`), `accesstoken.go`
   (issue and verify through `instance.Keys`).
6. Adapters `internal/identity/adapters/httpapi`: handlers `refreshToken`, `logout`,
   `listMySessions`, `listUserSessions`; registration of the `Authenticator` in the HTTP chain
   (step 4 of 02 "9.1") through `internal/wire`.
7. `kernel/actor`: if the merged `Actor` lacks any of `SessionID`, `SessionActive`, `DeviceID`,
   `DeviceState`, `DeviceKeyActive`, `StepUpAt`, `StepUpMethod`, `AuthMethod`, `ClientComponent`,
   `KeyThumbprint`, add them with doc comments (the package is implemented by `identity`, 02
   "4.5").
8. `internal/authz` `check_active`: if M1-28 does not yet decide session and device state from
   the actor, add exactly this rule with tests: principal not `active` gives `principal_inactive`;
   `SessionActive` false gives `session_revoked`; device `revoked` or `DeviceKeyActive` false gives
   `device_inactive`; all three are decision `unauthenticated`. Do not duplicate the rule in
   `identity`.
9. Config keys in the identity `Config`: `auth.access_token_ttl`, `auth.refresh_token_ttl`,
   `auth.session_max_age`, `auth.refresh_reuse_grace`, `auth.clock_leeway`,
   `auth.token_pepper_ref`, `identity.last_active_granularity`, `server.public_url`,
   `api.page_size_default`, `api.page_size_max`; register missing keys in the schema and run
   `make config-reference`.
10. Enable `refreshToken`, `logout`, `listMySessions`, `listUserSessions` in
    `services/server/api/operations.yaml` and regenerate (command from `services/server/README.md`,
    M1-17).
11. Lint: a depguard rule in `services/server/.golangci.yml` forbids `math/rand` and
    `math/rand/v2` in `internal/identity/...` (part of ID-T27), unless a repo-wide rule already
    does.
12. Docs: `docs/spec/08-identitaet.md` "7.2" gets `aud`, header `typ`, `amr` form and the cap of
    `exp` at the session end; "7.4" gets the refresh outcome table of "Design notes"; rulings in
    `docs/DECISIONS.md`; identity README; `services/server/CHANGELOG.md`.

## Out of scope

- Login flows that call `StartSession` (M1-37 to M1-43); tests call `StartSession` directly with
  devices registered through `identity.Devices`.
- API tokens (`Bearer bothy_...`) (M1-44): the `Authenticator` has the dispatch branch and answers
  `401 unauthenticated` until M1-44 fills it.
- Step-up challenges and `step_up_at` writes (M1-35); this ticket only reads `step_up_at` into the
  actor.
- `session.revoke` over the API, `device.revoke`, `user.disable`, raising `permission_version`
  for those actions (M1-45).
- Closing gateway connections with `4012` (M1-65) and the gateway variant of authentication
  (M1-64).
- Console cookie sessions (M6-01).
- Re-login after `auth.idp_recheck_interval` for OIDC sessions (M1-41 adds that refresh rule).
- The job `identity.session_cleanup` and retention deletes: no M1 ticket owns them; expiry is
  enforced at request time without a job.
- An API operation to create child sessions: none exists in `openapi.yaml` or 05; build the app
  function and test it, do not invent a route.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-34-identity-sessions`.
- Conventional Commits with scope `identity` (or `authz`, `kernel`, `docs` where fitting), each
  signed off (`git commit -s`).
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope above except `services/server/CHANGELOG.md`,
  `docs/DECISIONS.md` and generated references.
- Follow the host rules supplied by the launcher.
- Tests bind only `127.0.0.1` and port 0; PostgreSQL 18 runs as a throwaway container through the
  M1-12 helper; no Docker runtime means the test fails, never skips.
- Stop only processes you started, by PID; temporary directories only via `mktemp -d`, removed at
  the end.
- Never read or print secrets; tests generate their own pepper and signing keys through the test
  helpers of M1-09 and M1-32.
- No network calls except package downloads for pinned tools.
- A gap in the core contract (00, 03, 04, 05) ends the ticket with `Verdict: BLOCKED` (P-11).

## Design notes

Access token (issue):

- Header `{"alg": <alg of the active access_token key>, "kid": <kid>, "typ": "at+jwt"}`; `typ`
  follows RFC 9068 section 2.1 and keeps access tokens apart from every other JWS. Signing goes
  through the signer of `instance.Keys` for purpose `access_token`.
- Claims: `iss` = `instance_id` as UUID string (M1-15 instance record); `aud` = origin of
  `server.public_url` plus `/api/v1` (for example `https://chat.example.com/api/v1`); `sub` = user
  id; `sid` = session id; `did` = device id; `jti` = new UUIDv7; `iat` = `nbf` = now (seconds);
  `exp` = min(now + `auth.access_token_ttl`, `auth_sessions.expires_at`); `cnf` =
  `{"jkt": <family dpop_jkt>}`; `amr` = `[<auth_sessions.auth_method>]`. No other claims. Record
  `aud` and `typ` as rulings in DECISIONS.md and in 08 "7.2".
- `TokenResponse.expires_in` = seconds from now to `exp`; `token_type` = `DPoP`.

Authenticator (`Authenticate(r) (actor.Actor, error)`), in this order:

1. No `Authorization` header: anonymous actor.
2. More than one `Authorization` header or a scheme other than `DPoP` and `Bearer`:
   `401 unauthenticated`, `WWW-Authenticate: DPoP algs="ES256 EdDSA"`.
3. `Bearer`: a token starting with `bothy_` goes to the API token branch (M1-44; until then
   `401 unauthenticated`). A token that parses as this server's access token (header `typ` =
   `at+jwt` and a known `kid`) gives `401 invalid_dpop_proof` (ID-T14 Bearer case). Anything else
   `401 unauthenticated`.
4. `DPoP`: parse with `domain/jose`, allowlist = the algorithms of the access-token keys; header
   `typ` must be `at+jwt`; `kid` must belong to a key of purpose `access_token` in state `active`
   or `retiring`; signature, `iss`, `aud` must match; `nbf` <= now + leeway; `exp` > now - leeway.
   Malformed, unknown `kid`, bad signature, wrong `iss` or `aud`: `401 unauthenticated`; expired:
   `401 token_expired`; header `WWW-Authenticate: DPoP error="invalid_token", algs="ES256 EdDSA"`.
5. `identity.DPoP.Verify` with `Method` = request method, `URL` = `ExpectedHTU(server.public_url,
   r)`, `AccessToken` = the raw token, `ExpectedJKT` = `cnf.jkt`. Errors as in M1-33.
6. One query by `sid`: session, device, active `dpop` key. Build the actor (kind `user`, id `sub`,
   all fields of Scope item 7). `SessionActive` is true only if the row exists, `revoked_at` is
   null, `expires_at` > now, `user_id` = `sub` and `device_id` = `did`; `DeviceKeyActive` is true
   only if the active `dpop` key's `jkt` equals `cnf.jkt`. The Authenticator does not reject on
   these flags: `authorize` (`check_active`) does, so the denial is audited with its reason
   (07 "4.2", "4.1"). The HTTP chain maps `principal_inactive`, `session_revoked` and
   `device_inactive` to `401` problem code `session_revoked` (06 "5": "Sitzung, Gerät oder
   Prinzipal widerrufen"), `unauthenticated` to `401 unauthenticated`.
7. If the proof used the previous nonce (`NonceStale`), the response carries `DPoP-Nonce` with the
   current nonce (06 "3.1" item 2).
8. Activity: when `auth_sessions.last_seen_at` or `devices.last_seen_at` is older than
   `identity.last_active_granularity` (or null), update both and `auth_sessions.last_ip` (client IP
   from the trusted-proxy logic of M1-18) in one short statement outside the request's unit of
   work; a failure is logged and does not fail the request.

Session state is read per request, not cached. Reason: revocation then works across processes at
once (AZ-P9 for sessions, 04 "8.5") without raising `permission_version` for logout and refresh
reuse; the cost is one indexed query per authenticated request. Record as ruling.

Session start (`StartSession`): the device belongs to the user and is not `revoked` (pending is
allowed, 08 "5.2"); `expires_at` = now + `auth.session_max_age`; family with `dpop_jkt` = `jkt`
of the device's active `dpop` key, `current_generation` 0; refresh token generation 0 with
`expires_at` = min(now + `auth.refresh_token_ttl`, session `expires_at`); access token as above.
No audit here; the calling login ticket audits `auth.login.complete`.

Child session (`StartChildSession`): parent active, `client_component = 'app'`, not itself a
child; component `notification_ext` or `share_ext`; same user, device, `auth_method`,
`identity_id`; `parent_session_id` = parent; `expires_at` = parent `expires_at`; own family with
the parent family's `dpop_jkt` (extensions share the device key through the keychain group, 15
"9"). Any other input: `400 validation_failed` or `not_found`.

Refresh (`refreshToken`, action `auth.token.refresh`, no `Idempotency-Key`), one unit of work:

1. A missing `DPoP` header gives `401 invalid_dpop_proof` (map the generated decoder's missing
   header error for this parameter, do not answer `400`). Verify the proof with `AccessToken`
   empty and `ExpectedJKT` empty; the result's `JKT` is compared in step 4.
2. `token_hash` = HMAC-SHA-256(pepper, token); unknown hash: `401 unauthenticated`.
3. `SELECT ... FOR UPDATE` on the family, then read the token row, the session, the device and the
   principal status. Family revoked, session revoked or past `expires_at`, device `revoked`,
   principal not `active`: `401 session_revoked`, nothing changes.
4. Proof `JKT` differs from `family.dpop_jkt`: `401 invalid_dpop_proof`, nothing changes, audit
   deny (ID-T17). Reason for no revocation: a caller without the device key must not be able to
   end someone's session.
5. Token `expires_at` <= now: `401 unauthenticated`, nothing changes.
6. Decide per 03: `generation = current_generation` and `used_at` null: rotate (set `used_at`,
   insert generation + 1, raise `current_generation`, `rotated_at` = now). `generation =
   current_generation - 1` and now - `rotated_at` < `auth.refresh_reuse_grace`: rotate again from
   the current generation and mark the never received current token `used_at` = now. Otherwise
   reuse.
7. Reuse: family `revoke_reason = 'reuse_detected'`, session `revoke_reason = 'refresh_reuse'`,
   child sessions with the same reason and their families `session_revoked`; one `session.revoked`
   event per revoked session; `SessionEnded`; audit deny. This work commits; then the request
   ends with `401 session_revoked`.
8. Success: `200 TokenResponse` with the new pair, `session` and `device`.

The `authorize` call of the handler uses the anonymous principal, action `auth.token.refresh` and
resource `auth_session` (right `public`), so the proof of check of 02 "9.3" holds. A pending device
refreshes like an active one.

Logout (`logout`, action `auth.logout`, resource `auth_session` = the actor's session, right
`self`): revoke the session (`logout`), its children (`logout`), all their families (`logout`);
one `session.revoked` per session; `SessionEnded`; audit allow; `204`. An actor without a session
gets what `authorize` decides for a missing resource (`404`).

Revocation helper (`RevokeSession`): locks the session rows in ascending `id` (03 "6.3"), sets
`revoked_at` and `revoke_reason` on the session and its children, revokes their active families,
appends the events and publishes `SessionEnded`. Already revoked sessions stay unchanged
(idempotent). The event `session.revoked`: `audience` = the session's user, resource
`{kind: "auth_session", id}`, empty scope, `actor` = acting principal or absent, payload
`{session_id, reason}` with reason in the enum of `SessionRevokedPayload`.

Session lists: `listMySessions` (resource `user` = caller) and `listUserSessions` (resource `user`
= path id; self or owner, others `404`); only sessions with `revoked_at` null and `expires_at` >
now, children included, order `id DESC`, cursor helper from M1-33, `current` true for the
caller's session; absent members instead of `null`.

Audit (via `kernel/auditing.Recorder`, denials in their own transaction after a rollback):
`auth.logout` allow; `auth.token.refresh` denials with `deny_reason` from 07 "4.1" only:
unknown token `unauthenticated`; revoked state and reuse `session_revoked` (details
`{"cause":"refresh_reuse","family_id":...}` for reuse); key mismatch `unauthenticated` (details
`{"cause":"dpop_key_mismatch"}`). Never a token or hash in details.

Randomness: refresh token = 32 bytes from `crypto/rand`, base64url without padding (43 chars).

## Tests (write them first)

Unit tests:

- `TestSessionDecide` table: every outcome of the rotation decision, grace boundary exactly at
  `auth.refresh_reuse_grace`, generation `current - 2` is reuse.
- `TestAccessToken_Claims`: exactly the claim set of "Design notes"; no rights claims; `exp`
  capped at the session end; header `typ` `at+jwt`.
- `TestRefreshToken_IDT27`: 32 bytes, 43 characters, distinct across many draws; plus the depguard
  rule (golangci fixture or `make lint` failing on a probe file in a temporary copy).

Integration tests against PostgreSQL 18 with real `instance.Keys`, `authz`, `sync` appender and
`audit` recorder:

- `TestSessionsMigration_Constraints`: the three table checks of `auth_sessions`, one active
  family per session, unique `token_hash`, unique `(family_id, generation)`, cascades.
- `TestAuthenticate_Valid`: actor fields filled from session, device and key.
- `TestAuthenticate_IDT14_BearerRejected`: access token as `Bearer` gives `401 invalid_dpop_proof`.
- `TestAuthenticate_ProofOtherKey`, `TestAuthenticate_AthMismatch`: `401 invalid_dpop_proof`.
- `TestAuthenticate_KeyStates`: token signed with an `active` and a `retiring` key accepted,
  `retired` or unknown `kid` `401 unauthenticated`.
- `TestAuthenticate_ExpiryLeeway`: `exp` within `auth.clock_leeway` accepted, beyond
  `401 token_expired`; `nbf` in the future beyond leeway rejected.
- `TestAuthenticate_WrongIssOrAud`: `401 unauthenticated`.
- `TestAuthenticate_RevokedSessionBeforeExpiry`: after `RevokeSession` the still valid token gets
  `401 session_revoked` and an audit entry with reason `session_revoked` (AZ-M41 shape).
- `TestAuthenticate_RevocationAcrossInstances`: two app instances on one database; revocation
  through instance A is effective on instance B for the next request.
- `TestAuthenticate_StaleNonceGetsFreshNonce`: `DPoP-Nonce` header on the response.
- `TestAuthenticate_LastSeenGranularity`: at most one write per `identity.last_active_granularity`.
- `TestRefresh_Rotates`: new pair, old token `used_at`, `current_generation` + 1.
- `TestRefresh_IDT15_ReuseRevokes`: family `reuse_detected`, session and children
  `refresh_reuse`, one `session.revoked` per session in `event_log`, audit deny, `401
  session_revoked`, the old access token gets `401 session_revoked`.
- `TestRefresh_GraceSameKey`: previous generation within grace with the family key gives a new
  pair; after grace it is reuse.
- `TestRefresh_ConcurrentSameTokenWithinGrace`: two parallel refreshes with the same token and
  key: no revocation, both succeed, the first returned token is no longer valid.
- `TestRefresh_IDT16_ParentAndChildConcurrent`: app and extension refresh in parallel many times;
  no revocation, separate families.
- `TestRefresh_IDT17_OtherKey`: `401 invalid_dpop_proof`, family unchanged, audit deny.
- `TestRefresh_MissingProof`: `401 invalid_dpop_proof`, not `400`.
- `TestRefresh_Expired` and `TestRefresh_SessionEnd`: refresh `expires_at` and access `exp` never
  after the session end; expired refresh token `401 unauthenticated`.
- `TestRefresh_PendingDevice`: allowed.
- `TestRefresh_TokenNeverLogged`: canary refresh and access tokens absent from captured logs,
  audit details and events.
- `TestChildSession_Rules` and `TestRevokeParent_RevokesChildren`.
- `TestLogout`: session, children, families revoked with `logout`, events, `SessionEnded`
  published, audit allow, `204`; the same token afterwards `401 session_revoked`; pending device may
  log out.
- `TestListSessions`: self, owner, other person `404`, revoked and expired omitted, `current`,
  pagination, pending device `403 device_pending`, no `null` members.

Commands that must pass:

```sh
cd services/server && go test -race ./internal/identity/... ./internal/authz/... ./internal/kernel/...
make arch
make codegen-check
make migrations-lint
make config-reference && git diff --exit-code docs/operations/configuration.md
make check
```

## Acceptance criteria

- [ ] Migration creates the three tables exactly as 03 (FK on `identity_id` deferred to M1-36,
      documented in the migration).
- [ ] Access tokens carry exactly the claims of 08 "7.2" plus header `typ` `at+jwt`, no rights,
      `exp` capped at the session end.
- [ ] Authenticator follows the order of "Design notes"; Bearer use of an access token is
      rejected; revoked sessions are rejected by `authorize` with an audited reason.
- [ ] Refresh implements rotation, grace and reuse of 03 with the codes of "Design notes";
      ID-T15, ID-T16, ID-T17 green; a wrong key never revokes.
- [ ] Logout and `RevokeSession` revoke children and families, append `session.revoked`, publish
      `SessionEnded`.
- [ ] `listMySessions`, `listUserSessions`, `refreshToken`, `logout` enabled and conform to the
      schema; no `null` members.
- [ ] Refresh tokens are 256 random bits from `crypto/rand`; `math/rand` banned in identity.
- [ ] 08 "7.2" and "7.4" updated; rulings in DECISIONS.md.
- [ ] `make check` green.

## Definition of done

Code, tests and docs complete: identity README (status, port `Auth`, Authenticator, tables,
actions, event `session.revoked`, in-transaction event `SessionEnded`, config keys, test ids), a doc
comment on every public symbol, comments explain why, an ADR only for an architecture decision
beyond ADR 0020, the CHANGELOG entry under Unreleased of `services/server/CHANGELOG.md`, generated
references current. `make check` green. Independent review (`make review` plus a code review of the
diff) and red team gate (`docs/quality/red-team-gate.md`) over the diff with both verdicts in the
report; merge locally into the base branch only if both are clean and `make check` is green; then
`git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then the
commits, the files with line counts, the test results with counts (including the red output of
the first failing tests), what is not proven, and follow-ups that could not be done in scope, each
with its reason. Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
