# Modul `identity`

Status: principal tables and directory built (M1-26); devices, device keys and the DPoP verifier built (M1-33); people, owners and the own profile built (M1-31); sessions, access tokens, refresh families, logout and the Authenticator built (M1-34); step-up with the device key and the job `identity.auth_cleanup` built (M1-35); identity providers, the mapping of external identities, the credential overview and unlinking built (M1-36); logins, step-up with a passkey and API tokens follow with the tickets of the etappe.
Normativ ist [docs/spec/02-architektur.md](../../../../docs/spec/02-architektur.md); bei Abweichung gilt das Kapitel.

## Zweck

Prinzipale, Menschen, Owner, Anmelde-Treiber, Identitäten, lokale Anmeldedaten, Einladungs-Codes, Anmeldevorgänge, Geräte-Kopplung, Passkeys, Geräte und Geräteschlüssel, Sitzungen, Refresh-Familien, API-Tokens, Step-up, Zugriffstokens.

## Code

- `internal/identity` (`doc.go`, `directory.go`): the port. `Directory` (read only, for other
  modules; it does not call `authorize`, callers authorize their own actions) with the DTOs
  `PrincipalView`, `UserView`, `OwnerView` and the type `Status`.
- `domain/`: principal kinds and states, the value objects `Handle` (`^[a-z0-9][a-z0-9._-]*$`,
  1 to `principal.handle_max_chars` characters, upper case rejected) and `DisplayName`
  (`kernel/textcheck`, not blank, at most `principal.display_name_max_chars` runes), the entities
  `Principal`, `User` and `Owner` (non-blank reason without control or bidi characters).
  Validation errors are `validation_failed` with a field error on `/handle`, `/display_name` or
  `/reason`.
- `app/`: the outgoing port `Store`, `Directory` (implements `identity.Directory`),
  `PrincipalSource` (implements `authz.PrincipalSource`) and `Config` with the two limits, bound
  by `wire.IdentityConfig`. Reads join the ambient transaction of `kernel/txn`, otherwise they use
  the pool.
- `adapters/postgres/`: `queries/principals.sql`, `queries/owners.sql`, the sqlc output in
  `sqlcgen/` (entry in `services/server/sqlc.yaml`) and `Repository`, which implements
  `app.Store`. Writes (insert principal, person and owner) run only inside a unit of work;
  granting the owner role is a rights change (`txn.RightsChange`, M1-29). Error mapping: a unique
  violation on the handle, email or active owner index is `conflict` with the field `/handle`,
  `/email` or `/user_id`; check, trigger, foreign key and not null violations are
  `validation_failed` with the column as field; no error carries SQL text, constraint names or
  values.
- Migration `migrations/00003_identity_principals.sql`: the tables `principals`, `users` and
  `owners` as 03 section 4.2 states them, with the triggers `identity_users_kind_check` (a users
  row only for a principal of kind `user`) and `identity_principals_kind_immutable` (the kind
  never changes), both SQLSTATE `23514`. `bothy_app` may read, insert and update the three
  tables, not delete. Deviation from 03: `principals.avatar_file_id` has no foreign key yet;
  the migration of M3-03 that creates `files` adds it with `ON DELETE SET NULL`.

### Devices and DPoP (M1-33)

- Port (`devices.go`, `dpop.go`): `Devices` (`Register` joins the ambient unit of work and is
  called by the login tickets; `Get`, `ListMine`, `ListForUser` authorize `device.list`;
  `ActiveKey` does not authorize) with `KeyPurpose`, `DeviceState`, `Platform`, `PublicKey`,
  `RegisterDevice`, `Device`, `DeviceKey`, `Page`, `DevicePage`; `DPoP` (`CurrentNonce`, `Verify`)
  with `ProofRequest`, `VerifiedProof` and `DPoPError` (unwraps to the `401` problem with
  `WWW-Authenticate` and `DPoP-Nonce`).
- `domain/device`: registration rules (UUIDv7 id, trimmed name without control or bidi
  characters, platform, keys `ES256` or `EdDSA` validated by `kernel/jose`, no step-up key on
  `cli`, different `dpop` and `step_up` keys, `pending` or `active` with approver) and the state
  machine `pending -> active`, `pending -> revoked`, `active -> revoked` (`409
  invalid_state_transition` otherwise).
- `domain/dpop`: `NormalizeHTU`, the claim checks, the nonces (HKDF-SHA256 of the token pepper,
  info `dpop-nonce/v1`, window `unix / auth.dpop_nonce_ttl`, current and previous window) and the
  replay key `SHA-256(htu || 0x00 || jti)`.
- `app`: `Devices`, `DPoPVerifier` (checks in the order of the brief, nothing remembered on a
  failure), `CredentialSource` (device state; the session part follows with M1-34, tokens stay not
  active until M1-44), the outgoing ports `DeviceStore`, `Cursors` and `ReplayStore`; `Config` binds
  `auth.dpop_nonce_ttl`, `auth.dpop_proof_max_age`, `auth.clock_leeway`, `auth.token_pepper_ref`,
  `server.public_url`, `api.page_size_default`, `api.page_size_max`.
- `adapters/postgres`: `queries/devices.sql` and the device methods of `Repository`; a reused
  device id or thumbprint is `409 conflict` on `/device_id` or `/public_jwk`.
- `adapters/replay`: the in-memory `jti` store, sharded, with expiry on insert and a sweeper.
- `adapters/httpapi`: the handlers of `listMyDevices`, `listUserDevices`, `getDevice`,
  `ExpectedHTU` (origin of `server.public_url` plus the escaped path) and the cursor codec adapter.
- `adapters/identitytest`: `NewDeviceKey`, `SignDPoPProof`, `SignJWS`; only tests import it
  (`TestIdentitytest_NotInBinary`).
- Migration `migrations/00008_identity_devices.sql`: `devices` and `device_keys` as 03 section 4.2
  states them (retention class `security`); `bothy_app` may read, insert and update, not delete.
- Tests: ID-T14 without the Bearer case (`TestDPoP_IDT14`), ID-T29 and SEC-T07 for devices
  (`TestRegister_Validation`), `TestDPoP_NonceAcrossVerifiers`, `TestDPoP_ReplayConcurrent`,
  the HTTP tests of the three operations and the fuzz targets `FuzzParseJWK`,
  `FuzzParseCompactJWS`, `FuzzVerifyDPoPProof`. Rulings: docs/DECISIONS.md M1-33.

### People, owners and own profile (M1-31)

- Port (`people.go`): `People` (`ListUsers`, `GetUser`, `CreateUser`, `UpdateUser`, `EnableUser`,
  `GetMe`, `UpdateMyProfile`) and `Owners` (`ListOwners`, `AddOwner`, `RevokeOwner`) with the DTOs
  `ListUsersQuery`, `UserPage`, `CreateUserCmd`, `UpdateUserCmd`, `ProfileCmd`, `Me`; `Register`
  declares the durable event `user.updated`. `Directory` stays the actor-free read port for other
  modules.
- Operations (`adapters/httpapi/people.go`, mapping only):

  | Operation | Action and resource | Rights change | Event | Audit details |
  |---|---|---|---|---|
  | `listUsers` | `user.read`, kind `user` owned by the caller | no | none | none |
  | `createUser` | `user.create` on the instance | no | `user.updated` | `{}` |
  | `getUser` | `user.read` on the person | no | none | none |
  | `updateUser` | `user.update` on the person | no | `user.updated` | `{changed_fields}` |
  | `enableUser` | `user.enable` on the person | yes | `user.updated` | `{from_status, reason}` |
  | `listOwners` | `user.read`, kind `user` owned by the caller | no | none | none |
  | `addOwner` | `owner.add` on the person | yes | `user.updated` | `{reason}` |
  | `revokeOwner` | `owner.remove` on the person | yes | `user.updated` | `{reason}` |
  | `getMe` | `self.read` on the caller's principal | no | none | none |
  | `updateMyProfile` | `self.profile.update` on the caller | no | `user.updated` | none |

- Rules (`app/people.go`, `app/owners.go`, `app/me.go`, `app/usecases.go`): no self
  registration; handles `^[a-z0-9][a-z0-9._-]*$` up to `principal.handle_max_chars`, display
  names without control or bidi characters, e-mail one plain address, locale a BCP 47 tag of at
  most 35 characters stored in canonical form (port `Locales`, adapter `adapters/locale` over
  `golang.org/x/text/language`), time zone one of the IANA names of the toolchain's tzdata
  (`domain/zones.gen.go`, written by `tools/zonegen`; never `Local` or `Factory`, the same on every
  host), loaded with `time/tzdata` embedded; duplicates of handle or
  e-mail are `409 conflict`; `local_account: true` is `400` (`/local_account`, `not_enabled`)
  until M1-37. `enableUser` moves `pending` and `disabled` to `active` (clears `disabled_at`,
  restores no device or session), an active person is a call without effect (`200`, audit
  `no_change`, no event, no rights version), `deleted` is `409 invalid_state_transition`.
  `addOwner` needs an active person that is no owner (`409 conflict`), `revokeOwner` an active
  owners row (`404`). Every use case calls `authorize` before it answers a missing person or owner
  role with `404` (`checkThenFound`); the device reads `getDevice` and `listUserDevices` do the
  same inline. An anonymous caller, a person without owner role and a pending device (for the
  people operations) therefore get the same answer for existing and unknown ids; a pending device
  still gets `403 device_pending` for another device of its own person but `404` for an unknown
  device (M1-28/3). Owner changes and `enableUser` authorize once per request in a first unit
  and write in a second unit declared as the rights change, which locks the person (the owner
  changes also the active owners rows in ascending id); the authz hook then answers `409 i1_violation` (with `conflicts`)
  and `409 last_owner`. `getMe` pins the rights version it reports, reports e-mail, locale and
  time zone for people only and `step_up_valid_until` only while the step-up holds; it works from
  a pending device. `listUsers` pages by `(lower(handle), id)` with a cursor bound to caller, `q`
  and `status`; `q` is a literal case-insensitive prefix of handle or display name, valid UTF-8
  without control or bidi characters (`400`, `#query/q`). The schema
  `User` has no e-mail member, so other people's addresses never leave the server.
- Event `user.updated`: resource `user` without version, empty scope, actor of the request,
  payload `{user: User}` with the state after the change, receive action `user.read`.
- `adapters/postgres`: `queries/people.sql` and the owner queries of `queries/owners.sql`
  (`LockActiveOwners`, `GetActiveOwner`, `RevokeOwner`).
- Tests: the HTTP tests of `adapters/httpapi` (`people_test.go`, `people2_test.go`) run the ten
  operations through the full chain with the Idempotency-Key stage, PostgreSQL 18, the rights
  hooks, sync and audit, the test authenticator `httpservertest.Actors` and the fakes of
  credentials, structure and agent policy; every response is validated against
  `docs/api/openapi.yaml` (`httpservertest.ResponseValidator`). Rulings: docs/DECISIONS.md M1-31.

### Sessions, access tokens and refresh families (M1-34)

- Port (`auth.go`): `Auth` (`StartSession` and `StartChildSession` join the ambient unit of work and
  authorize nothing, the login tickets call them; `CreateExtensionSession`, `Refresh`, `Logout`,
  `ListSessions` authorize; `RevokeSession` is for M1-35 and M1-45) with `ClientComponent`,
  `AuthMethod`, `SessionRevokeReason`, `StartSession`, `Session`, `SessionPage`, `TokenSet` and the
  redacting `Token` (prints `[redacted]` through fmt, slog and JSON; only `Reveal` returns the
  value). In-transaction event `SessionEnded{SessionIDs, UserID, DeviceID, Reason}` on every
  revocation that changed something.
- `domain/session`: the rotation decision `Decide` (`rotate`, `grace_rotate`, `reuse`, `expired`,
  `family_revoked`; the grace holds strictly below `auth.refresh_reuse_grace`), `BuildClaims`
  (exactly `iss`, `aud`, `sub`, `sid`, `did`, `jti`, `iat`, `nbf`, `exp`, `cnf.jkt`, `amr`; `exp`
  never after the session end), `RefreshExpiry`, `Audience`, `NewRefreshToken` (32 bytes from
  `crypto/rand`, 43 characters base64url) and `TokenHash` (HMAC-SHA-256 with the token pepper).
- `app`: `AccessTokens` (issues with header `typ` `at+jwt` and the active key of purpose
  `access_token` of `instance.Keys`; verifies `typ`, `kid` of an `active` or `retiring` key,
  signature, `iss`, `aud`, `nbf` and `exp` with `auth.clock_leeway`), `Sessions` (implements
  `identity.Auth`; refresh in one unit of work with the session rows locked in ascending id, then
  the family), `Authenticator` (implements `kernel/actor.Authenticator`: no header anonymous;
  several headers or another scheme `401 unauthenticated`; `Bearer bothy_...` waits for M1-44;
  an access token as Bearer `401 invalid_dpop_proof`; `DPoP`: token, proof, then one query for
  session, device and active key into the actor; a stale nonce gets `DPoP-Nonce`; activity at most
  once per `identity.last_active_granularity`), `CredentialSource` (sessions part:
  `SessionActive`, `SessionExpiresAt`), the outgoing port `SessionStore`. The Authenticator never
  rejects on session or device state: `authorize` does in `check_active` with the facts the actor
  carries (`authorize.CredentialObserver`), so the denial is audited and a revocation takes effect
  at the next request in every process (docs/DECISIONS.md M1-34/2).
- Operations (`adapters/httpapi/sessions.go`):

  | Operation | Action and resource | Audit | Answer |
  |---|---|---|---|
  | `refreshToken` | `auth.token.refresh`, anonymous, `auth_session` | denials (`unauthenticated`, `session_revoked`) | `200 TokenResponse` |
  | `logout` | `auth.logout` on the caller's session | allow | `204` |
  | `listMySessions` | `session.list` on the caller | none | `200 SessionPage` |
  | `listUserSessions` | `session.list` on the person (self or owner, others `404`) | none | `200 SessionPage` |
  | `createExtensionSession` | `session.extension.create` on the caller's session | allow | `201 TokenResponse` |

  Refresh carries no `Authorization` header (the operation is public), only the proof. A missing
  proof `401 invalid_dpop_proof`; unknown or expired token `401
  unauthenticated`; revoked family, session or device, a family key that is no longer the device's
  active key, disabled person or a session past its end `401 session_revoked`; a proof of
  another key `401 invalid_dpop_proof` and nothing changes;
  rotation and grace as 03 `refresh_tokens`; reuse revokes family (`reuse_detected`), session and
  children (`refresh_reuse`, their families `session_revoked`), commits, then `401
  session_revoked`. Lists hold active sessions only, children included, newest first.
- Event `session.revoked`: resource `auth_session`, audience the person, empty scope, actor the
  acting principal or none, payload `{session_id, reason}`, receive action `self`.
- `adapters/postgres`: `queries/sessions.sql`, `queries/refresh.sql`, `sessions.go`.
- Migration `migrations/00011_identity_sessions.sql`: `auth_sessions`, `refresh_token_families`
  and `refresh_tokens` as 03 section 4.2 (the foreign key of `identity_id` follows with M1-36);
  `bothy_app` may read, insert and update, not delete.
- Configuration: `auth.access_token_ttl`, `auth.refresh_token_ttl`, `auth.session_max_age`,
  `auth.refresh_reuse_grace`, `auth.clock_leeway`, `auth.token_pepper_ref`,
  `identity.last_active_granularity`, `server.public_url`, `api.page_size_default`,
  `api.page_size_max`.
- Lint: depguard `crypto-rand-only-in-identity` bans `math/rand` and `math/rand/v2` here (ID-T27).
- Tests: `domain/session` (`TestSessionDecide`, `TestAccessToken_Claims`,
  `TestNewRefreshToken_IDT27`), `app/accesstoken_test.go` (key states, leeway, iss and aud),
  `TestCredentialSource_Sessions`, `migrations/identity_sessions_test.go`, and the HTTP tests of
  `adapters/httpapi/sessions_test.go` over PostgreSQL 18 with real signing keys, authz, sync and
  audit (ID-T14 Bearer case, ID-T15, ID-T16, ID-T17, revocation across two instances, nonce,
  activity, logout, lists). Rulings: docs/DECISIONS.md M1-34.

### Step-up with the device key (M1-35)

- Port (`stepup.go`): `StepUp` (`CreateChallenge`, `CompleteWithDeviceKey`; M1-40 adds
  `CompleteWithPasskey`) with `Challenge{ID, Value, ExpiresAt}`, `StepUpResult{StepUpAt,
  ValidUntil}` and `StepUpMethodDeviceKey`. A port of its own beside `Auth` (02 section 4.3 lists
  step-up under `Auth`), because the brief fixes the names (docs/DECISIONS.md M1-35).
- `domain/stepup`: `NewChallengeValue` (32 bytes from `crypto/rand`, 43 characters base64url),
  `ChallengeHash` (HMAC-SHA-256 with the token pepper), `Check` (compact JWS by `kernel/jose`, no
  `jwk` header, exactly the `alg` of the key, signature by the active `step_up` key, payload
  `{challenge, sid, iat}` with `hmac.Equal`, the session of the request and `iat` inside
  `[created_at - auth.clock_leeway, now + auth.clock_leeway]` in whole seconds; unknown members
  ignored) with the causes `invalid_jws`, `forbidden_header`, `alg_mismatch`, `bad_signature`,
  `payload_invalid`, `challenge_mismatch`, `session_mismatch`, `iat_out_of_range`, and
  `Failures`/`LimitReached` (failures strictly inside `auth.login_fail_window`).
- `app`: `StepUps` (implements `identity.StepUp`), `AuthCleanup` (the job), the outgoing port
  `ChallengeStore`. Both operations first check the caller (a person with a session on a device
  that is not `cli`, else `403 forbidden` with reason `step_up_not_possible`, audited), then
  `authorize(auth.step_up)` with right `self` on the caller's session (`403 device_pending`,
  `401 session_revoked`). The completion locks the session rows, reads the active `step_up` key
  (none: `403 forbidden`, cause `no_step_up_key`), locks the challenge; unknown, foreign, other
  purpose, finished or expired is `401 step_up_required`, not counted, rolled back and audited. A
  wrong confirmation commits the challenge as `failed`, counts the failed step-up challenges of
  the session inside `auth.login_fail_window` and at `auth.step_up_fail_limit` revokes the session
  (`Auth.RevokeSession`, reason `step_up_failed`, `session.revoked`) in the same unit of work,
  with the deny entry (`step_up_required`, `{cause, challenge_id, session_revoked}`); the answer is
  `401 session_revoked` after a revocation, else `401 step_up_required`. Success marks the
  challenge `used` and sets `step_up_at = now`, `step_up_method = device_key`; the Authenticator
  reads it per request, so the next request in every process sees it. A child session has its own
  step-up.
- Operations (`adapters/httpapi/stepup.go`), both without `Idempotency-Key` (`auth_flow`), class
  `rl.public_auth`:

  | Operation | Action and resource | Audit | Answer |
  |---|---|---|---|
  | `createStepUpChallenge` | `auth.step_up` on the caller's session | allow `{step: challenge, challenge_id}` | `200 StepUpChallenge` |
  | `completeStepUpWithDeviceKey` | `auth.step_up` on the caller's session | allow `{step: confirm, method, challenge_id}`, denials with cause | `200 StepUpResult` |

- Job `identity.auth_cleanup` (`adapters/jobs`, queue `maintenance`, every `auth.cleanup_interval`,
  roles `worker` and `all`): deletes challenges past `expires_at`, except failed step-up
  challenges whose `finished_at` is still inside `auth.login_fail_window` (08 section 13).
- `adapters/postgres`: `queries/stepup.sql`, `stepup.go`.
- Migration `migrations/00012_identity_step_up.sql`: `auth_challenges` as 03 section 4.2 with all
  purposes (the foreign key of `enrollment_code_id` follows with `enrollment_codes`, M1-37);
  `bothy_app` may read, insert, update and delete.
- Configuration: `auth.step_up_challenge_ttl`, `auth.step_up_max_age`, `auth.step_up_fail_limit`,
  `auth.login_fail_window`, `auth.clock_leeway`, `auth.token_pepper_ref`, `auth.cleanup_interval`.
- Tests: `domain/stepup` (`TestStepUpPayload_Check`, `TestStepUpFailLimit_Decision`,
  `TestChallengeValue_IDT27Like`), `adapters/jobs`, `migrations/identity_step_up_test.go`, and
  `adapters/httpapi/stepup_test.go` over PostgreSQL 18 with the real Authenticator, authz, audit
  and event appender: ID-T18 (missing, too old as AZ-M36, agent token, wrong key), ID-T29 for a
  `cli` device, single use, expiry, foreign challenges, concurrent completion, the failure limit,
  the binding to one session, pending device, no `step_up` key, no secrets in logs and audit,
  and the cleanup. Rulings: docs/DECISIONS.md M1-35.

### Identity providers and identities (M1-36)

- Port (`providers.go`): `Providers` (`List`, `Get` authorize `identity_provider.read`; `Create`,
  `Update`, `Delete` authorize `identity_provider.manage`; `Enabled` and `ListForDiscovery`
  authorize nothing, the login drivers and the discovery of M1-46 call them) and `Identities`
  (`Resolve`, `Link`, `MarkLogin` authorize nothing and join the login's unit of work;
  `Credentials` authorizes `identity.read`, `Unlink` `identity.unlink`) with `ProviderKind`,
  `ExternalIdentity` (the fields of 08 section 2.1), `Provider`, `CreateProvider`,
  `UpdateProvider` (pointer fields, PATCH), `ProviderInfo`, `Resolution` (`matched`,
  `unmatched`), `Identity`, `Passkey` and `CredentialOverview`.
- `domain/provider`: the kinds, `NewName` (trimmed, display text, at most
  `principal.display_name_max_chars`) and the strict schemas `identity_provider.oidc.v1`,
  `identity_provider.trusted_header.v1`, `identity_provider.local.v1` (08 section 2.6 with the
  refinements of docs/DECISIONS.md M1-36/5): one JSON object, no duplicate key and no null at any
  depth, no unknown member, field errors at `/config/...`, the normalized configuration as result.
  Secret references are checked by the port `SecretRefs` (implemented by `wire.SecretRefSyntax`
  with `platform/secrets.ParseRef`), never resolved.
- `app`: `Providers` and `Identities` with the extension points `ProviderKindRules` (per kind,
  `DefaultProviderRules`; M1-41 and M1-43 extend their kind) and `LoginMethodSource` (per
  credential kind: the sources of identities and of active devices that are not `cli`; M1-39 and
  M1-40 add theirs), the outgoing ports `ProviderStore` and `IdentityStore`.
  Mapping: enabled provider, active row of `(provider_id, subject)`, for `oidc` the same issuer;
  e-mail and display name are never read (R-ID-02, ID-T04). Unlink locks the person's active
  identities in ascending id, counts the remaining methods of every source (zero: `409 conflict`,
  reason `no_login_method_left`), sets `unlinked_at` and revokes the live sessions of the identity
  through `Auth.RevokeSession` (`logout` by the person, `admin` by an owner). Disabling a provider
  revokes the live sessions of all its identities (`admin`) in the same unit of work; `Enabled`
  and `Resolve` take share locks inside a unit, so a concurrent login cannot slip past
  (docs/DECISIONS.md M1-36/7).
- Operations (`adapters/httpapi/providers.go`, `credentials.go`):

  | Operation | Action and resource | Audit | Answer |
  |---|---|---|---|
  | `listIdentityProviders` | `identity_provider.read` on the instance (owner) | none | `200 IdentityProviderList` |
  | `createIdentityProvider` | `identity_provider.manage` on the instance (owner, step-up, `Idempotency-Key`) | allow `{provider_kind}` | `201 IdentityProvider` |
  | `getIdentityProvider` | `identity_provider.read` on the instance | none | `200 IdentityProvider` |
  | `updateIdentityProvider` | `identity_provider.manage` | allow `{provider_kind, changed_fields, sessions_revoked}` | `200 IdentityProvider` |
  | `deleteIdentityProvider` | `identity_provider.manage` | allow `{provider_kind}` | `204`, `409` while referenced |
  | `getMyCredentials` | `identity.read` on the caller | none | `200 CredentialOverview` |
  | `getUserCredentials` | `identity.read` on the person (self or owner, others `404`) | none | `200 CredentialOverview` |
  | `unlinkIdentity` | `identity.unlink` on the person (self or owner, step-up) | allow `{identity_id, provider_id, provider_kind, sessions_revoked}` | `204`, `409 no_login_method_left` |

- `adapters/postgres`: `queries/providers.sql`, `queries/identities.sql`, `providers.go`; a taken
  name is `409` at `/name`, a second `trusted_header` or `local` provider `409` at `/kind`, an
  active subject twice `409` at `/subject`, a referenced provider `409` with reason
  `provider_in_use`.
- Migration `migrations/00013_identity_providers.sql`: `identity_providers` and `user_identities`
  as 03 section 4.2, plus the foreign keys `auth_sessions.identity_id` (`ON DELETE SET NULL`) and
  `groups.provider_id` (`ON DELETE RESTRICT`); `bothy_app` may delete providers, not identities.
- Tests: `domain/provider` (`TestProviderConfig_OIDC`, `TestProviderConfig_TrustedHeader`,
  `TestProviderConfig_Local`), `app` (`TestResolve_IDT04_EmailNeverMatches`, ID-T04),
  `migrations/identity_providers_test.go` and the HTTP tests of `adapters/httpapi/providers_test.go`
  over PostgreSQL 18 with the real Authenticator, step-up, authz and audit (owner and step-up,
  non-owner, single kinds, kind immutable, config on update, delete, idempotency, audit without
  values, issuer, link conflicts, credentials, last method, concurrent unlink, sessions of unlink
  and disable, discovery). Rulings: docs/DECISIONS.md M1-36.

## Port (Wurzelpaket `internal/identity`)

- Eingehend: `Auth` (Anmeldung, Refresh, Logout, Kind-Sitzungen), `StepUp` (Step-up, M1-35), `Providers` and `Identities` (M1-36), `Directory`, `Devices`, `Tokens`, `Owners`; deklariert `AvatarFiles`.
- Gibt In-Tx-Ereignisse aus: `DeviceRevoked`, `SessionEnded`.
- Abonniert In-Tx-Ereignisse: keine.
- Implementiert Quellen-Ports und Querschnitts-Ports: kernel `actor.Authenticator`, `authz.PrincipalSource`, `authz.CredentialSource` (Sitzung, Gerät, Token für die Prüfung `check_active`, 07 Abschnitt 4.2), `instance.LoginOptionsSource`, `instance.FlagUserSource`, kernel `personaldata.Participant`.

## Abhängigkeiten

Darf die Ports dieser Module nutzen: `instance`, `authz`.

Grund: Signierschlüssel für Zugriffstokens; implementiert `PrincipalSource` und `CredentialSource`.

Zusätzlich für alle Module: `internal/kernel`; `internal/platform` und `api/` nur in `adapters/`.
Geprüft durch go-arch-lint (ADR 0008).

## Tabellen

`principals`, `users`, `owners`, `identity_providers`, `user_identities`, `local_credentials`, `enrollment_codes`, `auth_login_flows`, `auth_challenges`, `device_pairings`, `passkey_credentials`, `devices`, `device_keys`, `auth_sessions`, `refresh_token_families`, `refresh_tokens`, `api_tokens`.

Nur dieses Modul schreibt und liest diese Tabellen per SQL (03-datenmodell.md, Grundsatz 12).

## Aktionen (05-aktionen.md)

`instance.bootstrap`, `auth.login.begin`, `auth.login.complete`, `auth.token.refresh`, `auth.logout`, `auth.step_up`, `identity_provider.read`, `identity_provider.manage`, `owner.add`, `owner.remove`, `owner.recover`, `self.read`, `user.read`, `user.create`, `user.update`, `user.disable`, `user.enable`, `self.profile.update`, `identity.read`, `identity.link`, `identity.unlink`, `local_credential.set`, `passkey.register`, `passkey.revoke`, `device.list`, `device.approve`, `device.revoke`, `device.expire`, `session.list`, `session.revoke`, `session.extension.create`, `token.list`, `token.create`, `token.revoke`.

## Ereignisse und WebSocket-Ops

- Dauerhaft: `user.updated`, `session.revoked`.
- Kurzlebig: keine.
- WebSocket-Ops (Client an Server): keine.

## Hintergrundjobs

| Job | Auslöser | Warteschlange |
|---|---|---|
| `identity.session_cleanup` | periodisch | `maintenance` |

## Hinweise für den Bau

- Tokens nur als HMAC mit Server-Pepper; Refresh-Rotation mit Sperre auf der Familie, Wiederverwendung widerruft Familie und Sitzung (03, 4.2).
- Implementiert `kernel/actor.Authenticator` für `platform/httpserver` und `gateway`.
- Widerruf von Gerät oder Sitzung schließt Verbindungen mit `4012` über das Ereignis `session.revoked`. Logout and refresh reuse do not raise the rights version: `authorize` reads the session state the Authenticator read for the request (docs/DECISIONS.md M1-34/2); `device.revoke` and `session.revoke` (M1-45) keep the rights change of 05.

## Kapitel

[02-architektur.md](../../../../docs/spec/02-architektur.md), [08-identitaet.md](../../../../docs/spec/08-identitaet.md)
