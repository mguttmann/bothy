# Module `identity`

Status: principal tables and directory built (M1-26); devices, device keys and the DPoP verifier built (M1-33); people, owners and the own profile built (M1-31); sessions, access tokens, refresh families, logout and the Authenticator built (M1-34); step-up with the device key and the job `identity.auth_cleanup` built (M1-35); identity providers, the mapping of external identities, the credential overview and unlinking built (M1-36); API tokens with prefix and checksum built (M1-44); first setup, enrollment codes, the shared login completion and the setup lock built (M1-37); device approval with the verification code, device pairing and the expiry of unconfirmed devices built (M1-38); passkey login, enrollment, registration and step-up built (M1-40); the OIDC login with PKCE, identity linking, provisioning and the re-login rule built (M1-41); the login with the trusted proxy header and its per-request binding built (M1-43); the principal writer for agent principals built (M1-51); disabling a person, revoking a device, ending a session and the inactive device sweep built (M1-45); groups from the OIDC group claim built (M1-42); local accounts with password and TOTP built (M1-39).
The normative source is [docs/spec/02-architecture.md](../../../../docs/spec/02-architecture.md); if this file differs, the chapter applies.

## Purpose

Principals, people, owners, login drivers, identities, local credentials, enrollment codes, login flows, device pairing, passkeys, devices and device keys, sessions, refresh families, API tokens, step-up, access tokens.

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
  failure), `CredentialSource` (device state; the session part since M1-34, the token part since
  M1-44), the outgoing ports `DeviceStore`, `Cursors` and `ReplayStore`; `Config` binds
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
  e-mail are `409 conflict`; `local_account: true` also creates the local identity and a
  `first_login` code (M1-37). `enableUser` moves `pending` and `disabled` to `active` (clears `disabled_at`,
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
  several headers or another scheme `401 unauthenticated`; a value starting with `bothy_` takes
  the API token path of M1-44 under either scheme;
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

### Fresh step-up of the terminal (M2-40)

- Port: `StepUp.ConsumeFresh(ctx, a)` consumes the fresh step-up of one terminal opening (08
  section 6.3 point 4). It runs only inside the caller's unit of work (M2-44 calls it in the
  transactions of `openTerminalSession` and `attachTerminalSession`), locks the session row
  (`LockSessionForStepUpConsumption`, the first domain row lock of the action), answers `401
  session_revoked` for a revoked or ended session, checks the freshness again on the locked row
  with the rule of `authorize` (`authz.FreshStepUpCheck`, bound in `internal/wire` with
  `terminal.step_up_max_age`) and sets `step_up_consumed_at` (`ConsumeStepUp`). A stale step-up is
  `401 step_up_required`; the audit reason of the caller is `ReasonStepUpNotFresh`. It writes no
  event and no audit entry.
- The Authenticator reads `step_up_consumed_at` in its one lookup query into
  `actor.Actor.StepUpConsumedAt`, so the next request after an opening carries the consumption;
  other step-up actions never read it.
- Tests: `adapters/httpapi/freshstepup_test.go` over PostgreSQL 18 with the real Authenticator:
  `TestIDT28_PasswordTotpAndOidcStepUp`, `TestIDT28_StepUpTooOld`,
  `TestIDT28_TwoOpeningsNeedTwoConfirmations`, `TestIDT28_ConcurrentOpeningsOneConfirmation` and
  `TestConsumeFresh_Rules` (ID-T28 without the console part, which M6-18 adds).

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
  | `unlinkIdentity` | `identity.unlink` on the person (self or owner, step-up) | allow `{identity_id, provider_id, provider_kind, sessions_revoked}` | `204`, `409 conflict` (reason `no_login_method_left`) |

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

### API tokens (M1-44)

- Format (`domain/apitoken`, ADR 0061): `bothy_<kind>_<random>_<checksum>`, 60 characters. Kinds
  `agt` (agent), `app` (app), `pat` (person), `whi` (incoming webhook, format only until M2-27),
  `hpc` (pairing code of a terminal host, M2-39: issued by `Tokens.NewHostPairingCode` as a
  redacting `Token` with its HMAC under the token pepper, without a database call; only
  `host.pair` of M2-41 redeems it, and the authenticator refuses it before any lookup like `whi`;
  `Tokens.HashHostPairingCode` (M2-41) returns the same HMAC for a well-formed code and
  `ErrTokenMalformed` for any other value before computing one, without a database call).
  The random part is 32 bytes from `crypto/rand` as a big-endian integer in Base62
  (`0-9A-Za-z`), left-padded to 43 digits; the checksum is CRC-32 (IEEE) of those 43 ASCII bytes
  in Base62, left-padded to 6 digits. `Parse` accepts exactly
  `^bothy_(agt|app|pat|whi|hpc)_[0-9A-Za-z]{43}_[0-9A-Za-z]{6}$` with an equal checksum and a random
  value below 2^256. `token_prefix` is `bothy_<kind>_` plus the first 6 random characters,
  `token_hash` is HMAC-SHA-256 with the pepper of `auth.token_pepper_ref` over the whole token.
  The constant `bothy` is written once (`product`). Test vectors:
  `domain/apitoken/testdata/checksum_vectors.json`, recomputed in Python by
  `tools/ci/tests/test_gitleaks_rules.py`.
- Port (`tokens.go`): `Tokens` (`Issue`, `List`, `Revoke`, `RevokeAllForPrincipal`; none
  authorizes, all join the ambient unit of work) and `TokenAdmin` (`CreateToken`, `ListTokens`,
  `RevokeToken`, the use cases of the three operations). `IssuedToken.Secret` is an
  `identity.Token`, which prints `[redacted]` through fmt, slog and JSON; only the `createToken`
  handler calls `Reveal`. `Revoke` outside a unit runs its own rights change; inside one, and
  `RevokeAllForPrincipal` always, need a unit declared as a rights change (else
  `internal_error`), because authz caches token liveness per rights version. M1-45
  (`user.disable`) and M1-51 (`agent.disable`) call `RevokeAllForPrincipal` in their own rights
  change and write their own audit.
- `app`: `Tokens` (implements both ports), the outgoing port `APITokenStore`, the consumer port
  `PersonalTokenSwitch` (`PersonalTokensEnabled(ctx, principalID)`), which `internal/wire` binds
  to `adapters/instanceflags` over `instance.Flags` (the flag `tokens.personal_enabled` evaluated
  for the caller, with its rollout; M1-47).
  `CreateToken`: `authorize(token.create)` on the principal (requirement `owner` first, then
  `self`; step-up; a personal token for another person is `forbidden not_self`, for an owner
  through the authz guard), then `404` for an unknown principal, `409 conflict app_not_installed`
  for an app until M2-24, `403 feature_disabled` for a personal token while the switch is off,
  the body rules (`/name`, `/scopes` empty for `agt` and `pat`, `/expires_at` after now and at
  most `auth.api_token_max_ttl` or `auth.personal_token_max_ttl` ahead, `/dpop_jkt` 43 base64url
  characters and required for the kinds in `auth.dpop_required_for`), the insert and the allow
  entry `{token_id, kind, principal_id, dpop_bound, expires_at}` in one unit. `ListTokens`: the
  unrevoked tokens (expired ones included), newest first, cursor bound to caller and principal;
  a principal the caller may not list is `404`. `RevokeToken`: authorizes in a first unit (a
  refusal never raises the rights version), an already revoked token is `204` without a new
  entry, else a rights change revokes, audits and raises `permission_version`; `access.changed`
  follows only where the rights path computes a difference (a token changes no grant).
- Authentication (`app/apitokenauth.go`, `Authenticator.AuthenticateAPIToken`, also the entry
  of the gateway `identify` of M1-64): `Bearer` or `DPoP` with a value that starts with `bothy_`.
  Format, checksum and kind `whi` fail with `401 unauthenticated` before any query; then one
  query by `token_hash` joined with the principal kind, which must fit the token kind. A revoked
  or expired token yields the actor of its principal with the token id and `CredentialInactive`,
  so `authorize` refuses it in `check_active` with `401 unauthenticated` and the audit reason
  `token_revoked`, and the Idempotency-Key stage never replays a stored answer to it. A person's
  access token whose session is missing, revoked or ended, or whose device or key is revoked or
  inactive, yields `CredentialInactive` the same way (M1-45). A bound token
  needs the scheme `DPoP` and a proof of exactly `dpop_jkt` whose `ath` is the hash of the token
  (`401 invalid_dpop_proof` with `WWW-Authenticate: DPoP`, or `use_dpop_nonce`); `Bearer` with a
  bound token is `invalid_dpop_proof` (RFC 9449 section 7.2), an unbound token under `DPoP` is
  `401 unauthenticated`. The actor has no session and no device, so a personal token never has
  owner rights and never provides step-up (SR5, `step_up_not_possible`). Nothing is cached in
  identity; `last_used_at` is written at most once per `identity.last_active_granularity` on the
  pool, a failure is logged with the token id and never fails the request. `Revalidate` reads the
  token again.
- `CredentialSource` reports `TokenActive` (exists, belongs to the principal, not revoked) and
  `TokenExpiresAt` for authz.
- Operations (`adapters/httpapi/tokens.go`):

  | Operation | Action and resource | Class | Answer |
  |---|---|---|---|
  | `listTokens` | `token.list` on the principal | `rl.read` | `200 ApiTokenPage` |
  | `createToken` | `token.create` on the principal, step-up, `Idempotency-Key` | `rl.admin` | `201 ApiTokenCreated`, `Cache-Control: no-store` |
  | `revokeToken` | `token.revoke` on the token (owner: its principal) | `rl.admin` | `204` |

  `createToken` calls `idempotency.MarkOneTimeSecret`: the stored idempotency row holds no part
  of the answer and a replay answers `409 conflict one_time_secret` with `Idempotent-Replay:
  true` (R-API-26).
- `adapters/postgres`: `queries/apitokens.sql`, `apitokens.go`. Migration
  `migrations/00014_identity_api_tokens.sql`: `api_tokens` as 03 section 4.2 (`expires_at NOT
  NULL`, unique `token_hash`, partial index of the active tokens per principal); `bothy_app` may
  read, insert and update.
- Configuration: `auth.api_token_max_ttl` (`365d`), `auth.personal_token_max_ttl` (`90d`),
  `auth.dpop_required_for` (`["agent"]`, values `agent`, `app`, `personal`, cross-check
  `dpop_required_for`), `identity.last_active_granularity`, `auth.token_pepper_ref`.
- Tests: `domain/apitoken` (`TestFormat_RoundTrip`, `TestFormat_ChecksumVectors`,
  `TestParse_Rejects`, `TestGenerate_ReadsExactly32BytesFromCryptoRand`, `TestTokenPrefix`),
  `domain` (`TestCheckToken*`), `app/apitokens_test.go`
  (`TestIDT23_WrongChecksumRejectedWithoutLookup`, `TestAuthenticateAPIToken_Actor`,
  `TestIssuedToken_Redacted`, `TestCredentialSource_Tokens` on PostgreSQL 18),
  `migrations/identity_api_tokens_test.go`, and `adapters/httpapi/tokens_env_test.go` (subtests named
  after the brief in three groups `TestAPITokens_*`, one world per group) over
  PostgreSQL 18 with the real chain, Authenticator, authz, audit and idempotency stage: ID-T23
  (only the hash stored, shown once, expired, revoked, DPoP binding), the body rules, step-up,
  the switch, `not_self`, AZ-M35, SR5, revocation, `RevokeAllForPrincipal`, `last_used_at` and
  no secret in logs or audit. Rulings: docs/DECISIONS.md M1-44.

### First setup, enrollment codes and login completion (M1-37)

- Port (`enrollment.go`): `Enrollment` (`RedeemEnrollmentCode`, `CreateEnrollmentCode`) with
  `EnrollmentCode` (the code behind the redacting `Token`), `RedeemEnrollment` and `RedeemResult`.
  `People.CreateUser` returns `CreatedUser` with the code of `local_account`. `StartSession`
  takes `ClientIP`, stored as `auth_sessions.last_ip`.
- `domain` (`enrollment.go`): `NewEnrollmentCode` (20 bytes of `crypto/rand`, 32 Crockford Base32
  characters), `DisplayEnrollmentCode` (`en_` plus eight groups of four), `NormalizeEnrollmentCode`
  (case, `en_`, `-` and spaces do not count; `O` is `0`, `I` and `L` are `1`),
  `EnrollmentCodeHash` (HMAC-SHA-256 with the token pepper, context `enrollment-code/v1`), the
  purposes with `IssuePurpose` and `ConsumablePurposes`, `EnrollmentCodeState.Usable`.
- `app`: `EnrollmentCodes` (`Issue` locks the person and ends its other codes, `Match` locks the
  row in the ambient unit, `RecordFailedAttempt` in an own transaction after the rollback,
  `MarkUsed`); `Service` with `CompleteLogin` (the only place that creates devices, sessions and
  tokens for logins: `TrustConfirmed` makes a new device `active`, `TrustUnconfirmed` `pending`; a
  known device of the same person with the same active key is reused unchanged, any other known
  device id is `409`; the proof key must be the registered `dpop` key; one allow entry of
  `auth.login.complete` with the person as actor), `RedeemEnrollmentCode`,
  `CreateEnrollmentCode`, `BootstrapOwner` and `IssueRecoveryCode`; outgoing port
  `EnrollmentStore`.
- Purposes (08 section 3.2):

  | Purpose | Issued by | Redeemed by |
  |---|---|---|
  | `first_login` | `bootstrap-owner`; `createUser` with `local_account`; `createEnrollmentCode` for a person with a local identity and no device | `redeemEnrollmentCode`, `completeEnrollment` (M1-40), OIDC or header linking (M1-41, M1-43) |
  | `recovery` | `bothy setup issue-enrollment-code`; `createEnrollmentCode` for a person with a local identity and a device | as `first_login` |
  | `identity_link` | `createEnrollmentCode` for a person without a local identity | OIDC or header linking only |

- Operations (`adapters/httpapi/enrollment.go`):

  | Operation | Action and resource | Audit | Answer |
  |---|---|---|---|
  | `redeemEnrollmentCode` | `auth.login.complete`, anonymous, instance (`rl.public_auth`, no `Idempotency-Key`) | allow with the person as actor `{auth_method, device_id, session_id, new_device, device_state}`; denials with cause and code id | `201 TokenResponse`; every rejected code the same `401 unauthenticated` |
  | `createEnrollmentCode` | `user.update` on the person (owner with step-up) | allow `{purpose, enrollment_code_id}` | `201 EnrollmentCode` (one-time secret), `409 conflict` (reason `user_not_active`) |

  `createUser` with `local_account` needs an enabled `local` provider (else `409 conflict`
  (reason `local_provider_disabled`)); a disabled `local` provider also rejects the redemption of a person
  with a local identity (the same `401`) and `createEnrollmentCode` for that person (`409`), except
  a recovery code issued on the host, which logs in without the identity (docs/DECISIONS.md
  M1-37/12); both responses with a code are one-time secrets: the
  Idempotency-Key stage stores `{"one_time_secret": true}` and a replay is `409 conflict` (reason `one_time_secret`).
- Commands (`cmd/bothy setup`, assembled by `wire.Setup` as `bothy_app`): `bootstrap-owner --handle
  --display-name [--locale de] [--time-zone UTC]` creates, in one unit declared as the rights change
  `instance.bootstrap`, the local provider if missing, the person, its owner row (`granted_by`
  NULL, reason `bootstrap`), its local identity (subject = handle), a `first_login` code, the audit
  entry without actor and `setup_completed_at`; `issue-enrollment-code --user` issues a `recovery`
  code for an active owner with the audit entry `owner.recover` without actor. Each prints the
  code as the only line on stdout. Exit codes: `0`, `1` infrastructure, `2` arguments or no
  active owner, `3` setup already complete, `4` setup pending.
- `adapters/postgres`: `queries/enrollment_codes.sql`, `enrollment.go`.
- Migration `migrations/00016_identity_enrollment_codes.sql`: `enrollment_codes` as 03 section
  4.2, the deferred foreign key `auth_challenges.enrollment_code_id`, `bothy_app` may read, insert
  and update codes (not delete) and set `instance.setup_completed_at`.
- Configuration: `auth.enrollment_code_ttl`, `auth.enrollment_code_max_attempts`,
  `auth.token_pepper_ref` and the session keys of M1-34.
- Tests: `domain/enrollment_test.go` (format, normalisation, ID-T27, purposes),
  `adapters/httpapi/enrollment_test.go` (ID-T11, redemption, failed attempts, concurrency, device
  rules, rights, one-time secrets, bootstrap audit and atomicity, recovery), `internal/wire`
  (`TestIDT06_RequestsBeforeSetupGet503`), `cmd/bothy/setup_test.go` (ID-T07, output, exit codes,
  concurrency). Rulings: docs/DECISIONS.md M1-37.

### Device approval and pairing (M1-38)

- Port (`approval.go`): `DeviceApproval` (`ApproveDevice`, `BeginPairing`, `ApprovePairing`,
  `CompletePairing`; the caller comes from the context) with `PairingStarted` (the code behind the
  redacting `Token`) and `PairingOutcome`; the in-transaction event `DeviceRevoked` and the
  revoke reasons `verification_failed`, `pending_expired` and `pairing_expired`. `Device` carries
  `VerificationCode`.
- `domain`: `verification.go` (`DeriveVerificationCode`: HMAC-SHA-256 with the token pepper over
  `device-verification/v1`, the device id and the `jkt` of its `dpop` key, the first 8 bytes
  modulo `10^digits`, zero-padded; `VerificationCodeEqual` in constant time, spaces ignored) and
  `pairing.go` (`NewPairingCode` from `crypto/rand`, `DisplayPairingCode` in two groups,
  `NormalizePairingCode` as for enrollment codes without prefix, `PairingCodeHash` with context
  `pairing-code/v1`, the state machine `requested -> approved -> completed`, `requested ->
  rejected | expired`, `approved -> expired`, the JSON type `DeviceRegistrationV1`).
- `app`: `approval.go` (`ApproveDevice`, `revokeUnconfirmedDevice`), `pairing.go`
  (`BeginPairing`, `ApprovePairing`, `CompletePairing`), `expiry.go` (`ExpirePendingDevices`,
  `ExpirePairings`, run by `AuthCleanup`), outgoing port `PairingStore`. The verification code is
  derived, never stored, and set only in answers to the pending device itself: the
  `TokenResponse` of a login that leaves it pending and `getDevice` of its own entry
  (`listMyDevices` of a pending device is `403 device_pending`, docs/DECISIONS.md M1-38/3).
- Pending devices (08 section 5.2): tokens, but every action except `self.read`, `device.list`
  for the own device, `auth.token.refresh` and `auth.logout` is `403 device_pending` (authz,
  M1-28). A pending device older than `auth.device_pending_ttl` counts as revoked in the
  Authenticator (`401 session_revoked`, reason `device_inactive`), in the refresh and in a new
  login, before the job revokes it.
- Revoking an unconfirmed device (`revokeUnconfirmedDevice`): `devices` revoked with reason and
  `revoked_by` (NULL for the system), every key revoked, every open session of the device revoked
  with `device_revoked` through `Auth.RevokeSession` (families `session_revoked`, one
  `session.revoked` each, `SessionEnded`), then `DeviceRevoked`. M1-45 revokes active devices.
- Operations (`adapters/httpapi/approval.go`):

  | Operation | Action and resource | Audit | Answer |
  |---|---|---|---|
  | `approveDevice` | `device.approve` on the device (owner or self, step-up, `rl.admin`), rights change of the device's person | allow `{device_id, result}` with `approved` or `rejected_wrong_code` | `200 Device` (`active`, or `revoked` with `verification_failed` after a wrong code); active device unchanged `200`; revoked or expired pending `409 invalid_state_transition` |
  | `beginPairing` | `auth.login.begin`, anonymous (`rl.public_auth` per IP) | none | `201 PairingStarted`; `401 invalid_dpop_proof` when the proof key is not the registered `dpop` key; `409` for a device id in use |
  | `approvePairing` | `device.approve` on a device of the caller (step-up, `rl.admin`), rights change of the caller | allow `{pairing_id, device_id}`; an unknown code deny `no_grant` `{result: pairing_code_unknown}` | `201 Device` (`active`, the caller's); malformed code `400`; unknown code `404` and the step-up of the session ends |
  | `completePairing` | `auth.login.complete`, anonymous (`rl.public_auth` per IP) | `auth.login.complete` of the login completion | `202 PairingStatus` while requested; `201 TokenResponse` (`device_pairing`) once approved; another key `401 invalid_dpop_proof` (a requested pairing becomes `rejected`); unknown, expired, rejected or completed `410 gone` |

- Expiry: a `requested` pairing past `expires_at` becomes `expired`; an `approved` one too, and its
  never collected device is revoked with `pairing_expired`; pending devices past
  `auth.device_pending_ttl` are revoked with `pending_expired`. Both apply on access and in the
  job `identity.auth_cleanup`, each device in its own unit of work declared as the rights change
  `device.expire` and audited as that system action without actor.
- `adapters/postgres`: `queries/device_pairings.sql`, `pairings.go`; device lock, activation,
  revocation and the expiry list in `devices.sql`; the open sessions of a device and the end of a
  step-up in `sessions.sql`. A device row takes `created_at` from the login (the pending window
  counts from the clock of the request).
- Migration `migrations/00018_identity_device_pairings.sql`: `device_pairings` as 03 section 4.2;
  `bothy_app` may read, insert and update pairings (not delete).
- Configuration: `auth.device_pending_ttl`, `auth.device_verification_code_digits`,
  `auth.pairing_code_ttl`, `auth.pairing_code_chars`, `auth.token_pepper_ref`,
  `auth.step_up_max_age`, `auth.cleanup_interval`.
- Tests: `domain/verification_test.go`, `domain/pairing_test.go`,
  `adapters/httpapi/approval_test.go` (ID-T12, ID-T13, approval rights, expiry with and without
  the job, pairing end to end, wrong code, other key, rate limit per IP, no code in logs),
  `migrations/identity_device_pairings_test.go`. Rulings: docs/DECISIONS.md M1-38.

### Discovery source and the language of new people (M1-46)

- `adapters/loginoptions`: `Source` implements `instance.LoginOptionsSource` over the provider
  store (`Providers(ctx, true)`, ordered by `lower(name)`, `id`) and `auth.local_password_enabled`;
  `internal/wire` builds it for the instance settings. Discovery reports the enabled providers and
  offers the password only with an enabled provider of kind `local` (docs/DECISIONS.md M1-46/6).
- `createUser` stores `locale.default` of the instance settings (port `instance.Settings`,
  `PeopleDeps.Settings`) when the request names no `locale`; an explicit locale wins
  (`TestCreateUser_UsesLocaleDefault`).
- The integration tests of the instance operations run in the session world of this package
  (`adapters/httpapi/instance_settings_test.go`, docs/DECISIONS.md M1-46/11).

### Passkeys (M1-40)

- Port (`passkeys.go`, `stepup.go`): `Passkeys` (`BeginPasskeyLogin`, `CompletePasskeyLogin`,
  `BeginEnrollment`, `CompleteEnrollment`, `BeginPasskeyRegistration`,
  `CompletePasskeyRegistration`, `RevokePasskey`) with `WebAuthnChallenge`, `PasskeyLogin` and
  `PasskeyEnrollment`; `StepUp.CompleteWithPasskey`; `Challenge.PasskeyOptions`;
  `StepUpMethodPasskey`.
- `domain` (`ceremony.go`, `relyingparty.go`): the schema `webauthn_session.v1` (RP ID, required
  user verification, allowed credential IDs, user handle; never the challenge), `CheckCeremony`
  (only `issued` and not expired), `ClientDataChallenge` and `ChallengeMatches` (the challenge of
  `clientDataJSON` against the stored HMAC, `hmac.Equal`), `SignCount` (the counter rule),
  `DecoyCredentialID` (`passkey-decoy/v1`), `UserHandle` (16 bytes of the principal ID),
  `ResolveRelyingParty` (defaults from `server.public_url`, the start validation of
  `auth.webauthn_rp_id` and `auth.webauthn_origins`), `CheckPasskeyName`.
- `app`: `Passkeys` (the use cases), the shared `ceremonies` helper (open, check, assert, record a
  use, burn a failed row), `StepUps.CompleteWithPasskey` and the `passkey_options` of
  `CreateChallenge`, `PasskeySource` (credential overview and remaining sign-in methods),
  `EnrollmentCodes.MatchID`; outgoing ports `PasskeyStore` and `PasskeyVerifier` (JSON in and out,
  error class `ErrPasskeyRejected`).
- `adapters/webauthn`: `Verifier` with go-webauthn (pinned `v0.18.2`), see its README;
  `adapters/webauthn/virtualauthn`: the software authenticator of the tests.
- Ceremonies (row of `auth_challenges`, value 32 bytes of `crypto/rand` stored only as HMAC):

  | Purpose | Bound to | Lifetime | Refusal |
  |---|---|---|---|
  | `passkey_login` | nothing (the credential names the person) | `auth.login_flow_ttl` | `401 unauthenticated`, the same body every time |
  | `passkey_enroll` | `enrollment_code_id` (matched at begin, locked and checked again and redeemed at complete) | `auth.login_flow_ttl` | `401`; a refused response counts a failed attempt of the code |
  | `passkey_register` | `session_id` of the caller | `auth.login_flow_ttl` | `400 validation_failed` (`/credential`), duplicate `409` |
  | `step_up` (M1-35) | `session_id`; `passkey_options` over the same value | `auth.step_up_challenge_ttl` | `401 step_up_required`, counts toward `auth.step_up_fail_limit` |

  Complete locks the row, accepts only `issued` before `expires_at`, checks the HMAC, verifies and
  sets `used`; a completion that failed after the row was opened sets it `failed` after the
  rollback (docs/DECISIONS.md M1-40/6). Assertions: origin in the list, RP ID hash, user present
  and user verified, signature with the stored COSE key, credential not revoked, person `active`;
  backup state updated, sign counter per M1-40/3.
- Operations (`adapters/httpapi/passkeys.go`; `createStepUpChallenge` in `stepup.go`):

  | Operation | Action | Answer |
  |---|---|---|
  | `beginPasskeyLogin` | `auth.login.begin`, anonymous | `200 WebAuthnChallenge`; a name without passkeys gets one decoy ID |
  | `completePasskeyLogin` | `auth.login.complete`, anonymous, DPoP login proof | `201 TokenResponse`, new device `pending`; only with an active identity at the enabled `local` provider |
  | `beginEnrollment` | `auth.login.begin`, anonymous | `200 WebAuthnChallenge`; no attempt counted |
  | `completeEnrollment` | `auth.login.complete`, anonymous, DPoP login proof | `201 TokenResponse`, device `active`, code used |
  | `beginPasskeyRegistration`, `completePasskeyRegistration` | `passkey.register` (self; step-up once an active passkey exists, decided in `authz`) | `200 WebAuthnChallenge`, `201 Passkey` |
  | `revokePasskey` | `passkey.revoke` (self or owner, step-up) | `204`, also for a revoked passkey; `404` for another person's |
  | `completeStepUpWithPasskey` | `auth.step_up` | `200 StepUpResult`, `step_up_method = passkey`; a `cli` session `403` |

  An instance whose `server.public_url` names an IP address (or uses `http` outside `localhost`)
  without configured passkey keys has no passkeys (`403 feature_disabled`, M1-40/5).
- `adapters/postgres`: `queries/passkey_credentials.sql`, `passkeys.go`; `InsertChallenge` writes
  `enrollment_code_id` and `webauthn_session`; `LockEnrollmentCode`.
- Migration `migrations/00019_identity_passkey_credentials.sql`: `passkey_credentials` as 03
  section 4.2; `bothy_app` may read, insert and update (not delete).
- Configuration: `auth.login_flow_ttl`, `auth.webauthn_rp_id`, `auth.webauthn_origins` (cross-check
  `webauthn_relying_party`), `auth.token_pepper_ref`.
- Tests: `domain/ceremony_test.go`, `adapters/webauthn/webauthn_test.go`,
  `adapters/httpapi/passkeys_test.go` (every test of the brief), `internal/authz/app`
  (`TestConditionalStepUp`). Rulings: docs/DECISIONS.md M1-40.

### OIDC login (M1-41)

- Port (`oidc.go`): `OIDCLogin` (`BeginOIDCLogin`, `HandleOIDCCallback`, `CompleteOIDCLogin`,
  `BeginIdentityLink`, `CompleteIdentityLink`) with `OIDCBegin`, `OIDCBeginResult`, `OIDCCallback`
  and `OIDCComplete`. The server is the confidential relying party of every configured OIDC
  provider and the authorization server of its apps, PKCE `S256` on both legs (08 section 2.2,
  R-ID-01); provider tokens never reach a device and are never stored.
- `domain/oidcflow` (pure, no I/O): flow states `started -> code_issued -> completed` and `-> failed`,
  the app redirect rules (exact entry of `app_redirect_uris`, or `http://127.0.0.1:<port>/callback`
  and `http://[::1]:<port>/callback`; never `localhost`), `client_challenge` (43 base64url
  characters) and the RFC 7636 verifier, `S256`, the constant-time verifier check, the server
  verifier `base64url(HMAC-SHA-256(pepper, "oidc-pkce-verifier:" || flow_id))`, the HMAC of state,
  nonce and login_code (contexts `oidc-state/v1`, `oidc-nonce/v1`, `oidc-login-code/v1`), the ID
  token claim checks over parsed claims (`iss` with the `{tenantid}` template and `tid`, `aud`,
  `azp`, `exp` and `iat` with `auth.clock_leeway`, `nonce`, `sub`), the schema
  `external_identity.v1`, the provisioned handle (local part of `preferred_username`, else `name`,
  else `user`; collision suffix `-` plus four of `[a-z0-9]`) and the stripped display name.
- `app` (`oidc.go`, `oidc_callback.go`, `oidc_complete.go`): `OIDC` implements the port over the
  login service of M1-37 (`CompleteLogin`, enrollment codes) with the outgoing ports
  `OIDCProviderClient` (`Metadata`, `ExchangeCode`) and `LoginFlowStore`. The callback only verifies
  and stores the external identity with a new `login_code`; mapping (`(provider_id, issuer,
  subject)`), linking through the enrollment code checked at begin (consumed here, device
  `active`, audit `identity.link`), provisioning (`pending`, no grants, `users.email` NULL, device
  `pending`), device, session (`auth_method = oidc`) and tokens happen at complete, which needs the
  `client_verifier` (R-DM-30, R-ID-13). `LoginCompletion` gains `AllowPending` and `Provisioned`;
  `EnrollmentCodes.MatchID` (M1-40) locks the code again at complete.
- Refresh (`app/refresh.go`): an OIDC session whose provider login (`created_at`, of the parent for
  a child) is `auth.idp_recheck_interval` old is revoked (`expired`, families revoked), the refusal
  commits and answers `401 unauthenticated` with the deny entry `idp_recheck` (08 section 3.5).
- `adapters/oidc`: the provider client over `platform/egress` (one client per provider; the host
  policy allows the issuer host, then the hosts of `token_endpoint` and `jwks_uri` of the checked
  discovery document), discovery and JWKS cached for `auth.oidc_jwks_cache_ttl` with single-flight
  loads and one forced reload for an unknown `kid`, the token request with `client_secret_basic`
  or `private_key_jwt` (`PS256`, `RS256` or `ES256` client assertion with `kid` and `x5t#S256`,
  lifetime `auth.oidc_client_assertion_ttl`) and the JWS check of the ID token through
  `kernel/jose`. `adapters/oidc/oidctest` is the fake identity provider of the tests; the binary
  never links it (`TestIdentitytest_NotInBinary`).
- Operations (`adapters/httpapi/oidc.go`), none with `Idempotency-Key`:

  | Operation | Action and resource | Audit | Answer |
  |---|---|---|---|
  | `beginOidcLogin` | `auth.login.begin`, anonymous, instance (`rl.public_auth`) | denials (rejected enrollment code) | `200 OidcBeginResponse` |
  | `handleOidcCallback` | `auth.login.begin`, anonymous | denials `{provider_id, login_attempt_id, error}` | `302` to the app with `login_code` or `error` (`access_denied`, `login_failed`, `flow_expired`); `400` for an unknown state |
  | `completeOidcLogin` | `auth.login.complete`, anonymous, DPoP proof without `ath` | allow (person as actor) and denials with cause | `201 TokenResponse` |
  | `beginIdentityLink` | `identity.link` on the caller (step-up, `rl.admin`) | allow `{step: begin}` | `200 OidcBeginResponse` |
  | `completeIdentityLink` | `identity.link` on the caller (step-up) | allow `{step: complete}`, denials | `201 Identity`, `404` for another person's code, `409` for a linked subject |

- `adapters/postgres`: `queries/login_flows.sql`, `login_flows.go`; `LockEnrollmentCode` of M1-40. Migration
  `migrations/00020_identity_login_flows.sql`: `auth_login_flows` as 03 section 4.2 (retention
  `transient`); `bothy_app` may read, insert, update and delete.
- Job `identity.auth_cleanup` also deletes the flows past `expires_at`.
- `internal/wire`: `BindOIDC` after `BindSessions`; `ProviderSecrets` resolves provider secret
  references only below `<secrets.dir>/idp/` or as `BOTHY_IDP_*` and never a server `*_ref`
  (docs/DECISIONS.md M1-36/14).
- Configuration: `auth.login_flow_ttl`, `auth.server_code_ttl`, `auth.oidc_jwks_cache_ttl`,
  `auth.oidc_client_assertion_ttl`, `auth.idp_recheck_interval`, `auth.clock_leeway`,
  `auth.token_pepper_ref`, `server.public_url`, `egress.*`. Operations guide:
  [docs/operations/oidc-entra.md](../../../../docs/operations/oidc-entra.md).
- Tests: `domain/oidcflow` (`TestRedirectURIRules`, `TestClientChallengeFormat`,
  `TestPKCES256Vector`, `TestServerVerifierDerivation`, `TestIDTokenClaimValidation`,
  `TestProvisionedHandle`, `TestFlowStateMachine`), `kernel/jose` (`TestJWS_RFC7515ExampleRS256`,
  `TestJWKThumbprint_RFC7638ExampleRSA`), `adapters/oidc` (`TestOIDCClientAuth_PrivateKeyJWT`,
  `TestOIDCClientAuth_SecretBasic`, `TestJWKS_UnknownKidReloadsOnce`, `TestMetadataCacheTTL`),
  `adapters/oidc/oidctest` (`TestFakeProvider`), `app` (`TestOIDCUseCases`, `TestOIDCProvisioning`),
  `adapters/postgres` (`TestLoginFlowStoreRoundTrip`),
  `migrations/identity_login_flows_test.go`, `wire` (`TestProviderSecretsConfinement`) and the
  HTTP tests of `adapters/httpapi/oidc_test.go` over PostgreSQL 18 with the fake provider, as
  subtests named after the brief in the groups `TestOIDC_Login`, `TestOIDC_IssParameter`,
  `TestOIDC_Entra` and `TestIdentityLink` (one world per group) and
  `TestOIDCEgress_PrivateTargetsRejected` (ID-T01 to ID-T05, leaked authorization URL,
  provisioning, link mode, private egress targets, no secrets in logs, re-login, cleanup).
  Rulings: docs/DECISIONS.md M1-41.

### Groups from the OIDC group claim (M1-42)

- Callback (`app/oidc_callback.go`): for a provider with `groups_claim` the callback reads the claim
  with `oidcflow.GroupsFromClaims` into `external_identity.v1` (`groups_state` `present` with the
  sorted values, `absent`, or `overage` when the claim name is a key of `_claim_names`, 08 section
  2.3); another JSON type or a non-string element ends the flow with `error=login_failed` (cause
  `groups_claim` in the log). The values are never logged or audited. `oidcflow.Claims.Members`
  keeps every payload member for such configured claims.
- Complete (`app/oidc_complete.go`, `app/login_completion.go`): only `groups_state = present` sets
  `LoginCompletion.Sync`, which `completeLogin` calls after the person check (also for a person
  provisioned as `pending`) and before device and session. It calls the port `authz.ClaimSync`
  (`OIDCDeps.ClaimGroups`, the authz Admin use cases from `internal/wire`) with the time of the
  request and `auth.group_claim_max_age` (`Config.GroupClaimMaxAge`). The unit of work runs first
  without a rights change; when the sync answers `authz.ErrClaimSyncNeedsRightsChange` it rolls
  back and runs once more declared as a rights change of `group.member.sync`, so
  `permission_version` is locked first (03 section 6.3) and only a change raises it. An
  `authz.InvariantError` (I1) leaves nothing of the login, ends the flow as `failed` in its own
  unit and answers `409 i1_violation` with the hint of `authz.HintI1ClaimSync`; the deny entry is
  written by the rights hook of authz.
- Fake provider: `oidctest.IDP.SetGroups` and `SetGroupsOverage`.
- Tests: `domain/oidcflow` (`TestGroupsClaimParsing`), `adapters/httpapi/oidc_groups_test.go`
  (`TestGroupClaimSync` with the add, extend, remove, absent, overage, wrong type, concurrent and
  logging cases, `TestGroupClaimSync_I1` with the AZ-P7 case through the login,
  `TestGroupClaimSync_NoGroupsClaim`). Rulings: docs/DECISIONS.md M1-42.

### Trusted proxy header login (M1-43)

- `domain/trustedheader`: the pure rules of the driver `trusted_header` (08 section 2.4):
  `IsLoopbackListener` (the bound address, `127.0.0.0/8` and `::1` only), `Evaluate` (the TCP peer
  must lie in `server.trusted_proxies`, exactly one field line), `DecodeValue` (RFC 2047 with
  `utf-8` and `us-ascii` only; no control, format or bidi character, not empty), `ParseConfig`
  (`identity_provider.trusted_header.v1` over `domain/provider`) and `HandleFromLogin` /
  `HandleWithSuffix` for provisioned people (docs/DECISIONS.md M1-43/8).
- `app.TrustedHeaderDriver`: the loopback flag of the bound public listener (set by
  `internal/wire` after the listen, false until then), the readiness check
  `trusted_header_requires_loopback` (`Ready`) and the per-request binding (`Bound`, the
  `SessionBinding` of the Authenticator and of `refreshToken`) with
  `auth.trusted_header_bind_requests`.
- `Service.LoginWithTrustedHeader` (port `TrustedHeaderLogins`, handler
  `adapters/httpapi.TrustedHeader`): `404` while the driver is off, `401` with an audit denial
  without the login, then in one unit of work the known identity, the link through an
  `enrollment_code` (`identity.link`, device `active`) or a provisioned `pending` person, and the
  shared `CompleteLogin` with `auth_method = 'trusted_header'`.
- `Providers` refuses to enable a `trusted_header` provider without a loopback listener (`409
  conflict`, reason `trusted_header_requires_loopback`, audit deny with that reason).
- The handler and the binding read the TCP peer and the header from `kernel/reqmeta.Transport`,
  never a forwarded address; header values never reach logs, audit details or problem bodies
  (`TestTrustedHeaderProvisioning/TestTrustedHeaderNeverLogged`). Tests:
  `adapters/httpapi/trustedheader_test.go`.

### Feature flag sources and the personal token switch (M1-47)

- `adapters/flagsource`: `Source` implements `instance.FlagUserSource` over `Principals`: the IDs
  of a rollout that are principals of kind `user` and not `deleted`; agents and apps are refused
  (`unknown_user`).
- `adapters/instanceflags`: `Switch` implements `app.PersonalTokenSwitch` over `instance.Flags`
  (`tokens.personal_enabled` for the caller); `wire.IdentityWrites.Flags` is required.
- The integration tests of the flags run in the session world (`adapters/httpapi/flags_test.go`,
  docs/DECISIONS.md M1-47).

### Principal writer (M1-51)

- Port (`principals.go`): `PrincipalWriter` with `CreatePrincipal` (`CreatePrincipalCmd`: optional
  id, kind `agent` or `app`, handle and display name with the rules of M1-26, a taken handle is
  `409 conflict` on `/handle` without regard to case, also against people), `SetPrincipalStatus`
  (`active` clears `disabled_at`, `disabled` sets it; `changed` false for the same status;
  `invalid_state_transition` for a deleted principal or another target) and `SetDisplayName`
  (`changed` false for the same name). It authorizes nothing (the calling use case of agents did),
  writes only inside the ambient unit of work and refuses people with kind internal, whose
  lifecycle belongs to `People`.
- `app/principalwriter.go`: `PrincipalWriter` over the outgoing port `PrincipalStore`;
  `adapters/postgres/principalwriter.go` implements it with the queries `LockPrincipal` and
  `SetPrincipalStatus` of `queries/principals.sql`. `wire.NewIdentity` builds it as
  `Identity.Principals`.
- Tests: `adapters/postgres/principalwriter_test.go` (`TestPrincipalWriter_*` on PostgreSQL 18).

### Disable a person, revoke a device, end a session (M1-45)

- Port (`lifecycle.go`): `Lifecycle` with `DisableUser`, `RevokeDevice` and `RevokeSession` and
  their commands (`DisableUserCmd`, `RevokeDeviceCmd`, `RevokeSessionCmd`, each with the caller as
  `Actor`), and the in-transaction event `DeviceRevoked {DeviceID, UserID, Reason, At}`.
  `SessionEnded` (M1-34) gains `At`; every step that ends sessions publishes one `SessionEnded`
  per device, so each ended session is in exactly one event.
- `domain/lifecycle.go` (pure): `SessionRevokeReasonFor` (`logout` for the session's own person,
  `admin` otherwise), `DeviceRevokeReasonFor` (`revoked_by_self` or `revoked_by_owner`),
  `DisableTransition` (`active` and `pending` change, `disabled` no change, `deleted`
  `invalid_state_transition`) and `DeviceRevokeTransition`.
- `app/lifecycle.go`: `Lifecycle` over the outgoing port `LifecycleStore` and the use cases of
  `People`, `Sessions` and `Tokens`. Each operation runs a first unit of work that loads the
  resource, authorizes and ends a call without effect there (`200` with the object, `204` for a
  session, audit `no_change`, no rights version), then a unit declared as the rights change of
  its action for the affected person: phase 1 raises `permission_version`, the resource row is
  locked, `authorize` runs again, the steps follow, and the rights hook verifies I1 and SR8 and
  appends `access.changed` where the person's view changed.

  | Use case | Steps in the rights change | Stored reasons |
  |---|---|---|
  | `user.disable` (owner, step-up) | principal `disabled`; devices then sessions locked; sessions and families, devices (pending included) and keys, personal API tokens revoked; `session.revoked` per session, `user.updated`; audit `{reason, sessions_revoked, devices_revoked, tokens_revoked}`; SR8 by the rights hook (`409 last_owner`) | sessions `user_disabled`, families `session_revoked`, devices `user_disabled` |
  | `device.revoke` (self or owner, step-up) | device locked; its sessions (children included) and families, the device and its keys revoked; audit `{reason, user_id, revoke_reason, sessions_revoked}` | sessions `device_revoked`, devices `revoked_by_self` or `revoked_by_owner` |
  | `session.revoke` (self or owner, no step-up) | session and live children locked and revoked with their families; audit `{user_id, revoke_reason, sessions_revoked}` | `logout` (own) or `admin` |

  The owners row, grants and group memberships stay. Later stages subscribe: M3-22 deactivates
  push registrations on `DeviceRevoked` and `SessionEnded` (08 section 11 step 5, section 10
  step 2); M2 cancels runs and V2 suspends dev environments on `AccessChanged` (step 7); M1-65
  closes gateway connections with `4012` on `session.revoked`.
- Inactive devices (08 section 5.4): an active device whose `last_seen_at` (else `approved_at`)
  is older than `auth.device_inactive_revoke_after` is refused at once (the Authenticator reports
  its key as not active, `401 session_revoked` with the audit reason `device_inactive`, and
  records no activity; a refresh is `401 session_revoked`). `ExpireInactiveDevices`, run by
  `identity.auth_cleanup`, revokes each such device in its own rights change `device.expire`
  (reason `inactive`, no actor, sessions `inactive`).
- `adapters/postgres`: `queries/lifecycle.sql` and `lifecycle.go` implement `LifecycleStore`;
  `GetAuthState` also reads `devices.approved_at`. No migration.
- Operations (`adapters/httpapi/lifecycle.go`):

  | Operation | Action and resource | Class | Answer |
  |---|---|---|---|
  | `disableUser` | `user.disable` on the person, step-up, `Idempotency-Key` | `rl.admin` | `200 User` |
  | `revokeDevice` | `device.revoke` on the device, step-up, `Idempotency-Key` | `rl.admin` | `200 Device` |
  | `revokeSession` | `session.revoke` on the session, `Idempotency-Key` | `rl.admin` | `204` |

  A device or session of another person is `404` for a non-owner, the same as an unknown id.
- Configuration: `auth.device_inactive_revoke_after` (`180d`, minimum `1h`).
- Tests: `domain/lifecycle_test.go`, `adapters/postgres/lifecycle_test.go`
  (`TestLifecycle_StoreRoundTrip`), `app/lifecycle_test.go` and
  `adapters/httpapi/lifecycle_env_test.go` over PostgreSQL 18 with the real chain, step-up,
  authz, audit, sync and idempotency: ID-T22, ID-T26, AZ-M26, AZ-M40, AZ-M41, concurrent
  disables of the only two owners, the no-change and visibility rules, idempotency replays and
  the inactive device sweep. Rulings: docs/DECISIONS.md M1-45.

### Local accounts with password and TOTP (M1-39)

- Port (`local.go`): `LocalCredentials` (`SetLocalCredential`, `LoginWithPassword`) with
  `SetLocalCredential`, `LocalCredentialResult` (the otpauth URI behind the redacting `Token`) and
  `PasswordLogin`; `CredentialOverview` gets `local_password_set` and `totp_enrolled` from the
  overview source `app.NewLocalSource`.
- `domain`: `totp.go` (RFC 6238 with HMAC-SHA-1, six digits, step 30 seconds from `T0 = 0`, one
  step of tolerance, a code accepted only for a step after `totp_last_step`, constant-time
  comparison; 20-byte secrets; the otpauth URI with the host of `server.public_url` as issuer),
  `lockout.go` (the failure window of `local_credentials.fail_window_started_at` and the lock) and
  `password_rules.go` (at least `auth.password_min_length` code points of the NFC form, the list
  hit, no composition rules).
- `app/local.go`: the use cases and the outbound ports `PasswordHasher`, `SecretSealer`,
  `CommonPasswords`, `PasswordNormalizer` and `LocalCredentialStore`. `setMyLocalCredential`
  (`local_credential.set`, self): `403 feature_disabled` while `auth.local_password_enabled` is
  off, `409 conflict` (`no_local_identity`) without an active identity at the enabled `local`
  provider, `400 validation_failed` (`required`, `too_short`, `common_password`, `totp_invalid`);
  `totp_enroll` seals a new secret and makes the enrollment pending, `totp_code` confirms it; step-up
  only once the password path is established (TOTP confirmed), decided in `authz` through
  `PrincipalSource.HasEstablishedLocalCredential`. `loginWithPassword` (`auth.login.complete`,
  `rl.public_auth`): `404` while the switch is off or no `local` provider is enabled, the DPoP
  login proof, then one unit of work with the row locked `FOR UPDATE`, exactly one Argon2id
  computation per request (the dummy hash for an unknown name, also while locked), the same `401`
  for every rejection, the failure counter of the window (committed although the login fails) and
  the lock with the deny entry `{lock: true}`; success consumes the TOTP step, resets the counter,
  rehashes on changed parameters and calls `CompleteLogin` with a pending new device and
  `auth_method = password_totp`.
- `adapters/passwordhash`: Argon2id (`golang.org/x/crypto/argon2`) in PHC format with
  `auth.argon2id_*`, 16-byte salt, 32-byte key, constant-time comparison, the dummy hash computed
  at construction; parameters below the OWASP floor are refused (the configuration minimums stop
  the start first, ID-T24).
- `adapters/secretbox`: AES-256-GCM with the key behind `auth.totp_key_ref` (32 bytes as hex,
  generated by `bothy secrets init` as `totp_key`), additional data
  `"totp/v1" || 0x00 || identity_id`, stored layout `0x01 || nonce || ciphertext and tag`; the row
  keeps the reference that sealed it.
- `adapters/passwordpolicy`: the embedded list `common_passwords.txt` (truncated SHA-256 values,
  source and license in its [README](adapters/passwordpolicy/README.md)) and the NFC normalization
  (`golang.org/x/text/unicode/norm`).
- `adapters/postgres`: `queries/local_credentials.sql`, `local.go`. Migration
  `migrations/00029_identity_local_credentials.sql`: `local_credentials` as 03 section 4.2 with
  both checks; `bothy_app` may read, insert and update, not delete.
- Configuration: `auth.local_password_enabled`, `auth.password_min_length`, `auth.argon2id_*`,
  `auth.login_fail_limit`, `auth.login_fail_window`, `auth.login_lock_duration`,
  `auth.totp_key_ref`, `server.public_url`.
- Tests: the domain (`TestIDT27_TOTPVectorsRFC6238`, `TestIDT20_TOTPCodeOnlyOnce`,
  `TestTOTPWindowEdges`, `TestLockoutWindow`, `TestPasswordRules`), the adapters
  (`TestPHCFormatAndRehash`, `TestSecretBoxBindsIdentity`, `TestCommonPasswordsFile`),
  `internal/wire` (`TestIDT24_Argon2BelowFloorStopsStart`), `internal/authz/app`
  (`TestLocalCredentialStepUpOnlyWhenEstablished`) and `adapters/httpapi/local_test.go` over
  PostgreSQL 18 (ID-T21 timing, lockout, enrollment, step-up, validation, secrets at rest, no
  secrets in logs and audit). Rulings: docs/DECISIONS.md M1-39.

### App tokens and installations (M2-24)

- Ports (`apps.go`): `AppInstallations` (`ActiveInstallation`, `LockActiveInstallation` FOR SHARE in
  the ambient unit of work) with `ActiveAppInstallation`, declared here and implemented by the
  module apps (`apps/adapters/identitysource`), so identity never imports apps;
  `NoAppInstallations` (no app is installed; the default without apps). `PrincipalWrites`
  (`CreatePrincipal` for kinds agent and app, `SetPrincipalStatus` active or disabled with
  `disabled_at`, `UpdateDisplayName`) with `NewPrincipalCmd`; `app/principals.go` implements it
  with the handle and display name rules of M1-26.
- `createToken` for an app: the active installation is locked FOR SHARE inside the unit of work;
  none (also suspended or uninstalled) keeps `409 conflict` `app_not_installed`. The token scopes
  are the requested subset of the granted scopes, or all of them for an absent or empty `scopes`
  (`domain.AppTokenScopes`, `400 validation_failed` at `/scopes/<i>`); the audit entry names the
  stored scopes.
- Authentication of an app token reads the active installation on every request (no cache):
  active fills `InstallationID`, `GrantedScopes` and the token scopes (`actor.Scopes`); otherwise
  the installation id stays zero and authorize answers `app_not_installed`. `Revalidate` fails with
  `app_not_installed` once the installation is no longer active or no longer grants a scope of the
  actor. `PrincipalSource.WithApps` sets `authz.PrincipalFacts.AppInstallationActive`, so an app
  without an active installation has an empty rights snapshot. Rulings: docs/DECISIONS.md
  D-M2-24-10 and D-M2-24-11.
- Tests: `app/apptokens_test.go` (`TestCreateToken_AppScopes`, `TestAuthenticateAPIToken_AppInstallation`,
  `TestPrincipalWrites`); the HTTP cases run in `apps/adapters/httpapi`.

### Search language of new messages and threads (M3-13)

`identity.Directory` has two more reads ([14-search.md](../../../../docs/spec/14-search.md) 3.1,
docs/DECISIONS.md D-M3-13-1): `SearchLanguage(ctx, id)` gives for a person the configuration of
the first language subtag of `users.locale` through `search.locale_map` (one read by primary
key), for an agent or app that of `locale.default`, `simple` without a map entry, and `not_found`
for an unknown id; `InstanceSearchLanguage(ctx)` gives that of `locale.default`, read through
`instance.Settings.LocaleDefault`. Both join the ambient transaction. `app.NewDirectory(store)`
answers them only after `WithSearch(langs, settings)` (`kernel/searchlang.Map`); without it both
fail closed with `internal`, so a writer wired without the configuration never writes `simple`
by accident. `wire.NewIdentity` builds its `Directory` that way from `IdentityWrites.Search` and
`Settings` (`wire.NewSearchDirectory`).

## People by id list (M3-18)

- Port: `Directory.Users(ctx, ids)` returns the people among a list of ids in one read (`ListUsersByIDs`), keyed by id; unknown ids and principals that are not people are absent, like `Principals`. It does not authorize: `notify` loads `users.time_zone` for the quiet hours of people it already selected.

## Port (root package `internal/identity`)

- Inbound: `Auth` (login, refresh, logout, child sessions), `StepUp` (step-up, M1-35), `Providers` and `Identities` (M1-36), `Enrollment` (M1-37), `LocalCredentials` (M1-39), `Passkeys` (M1-40), `OIDCLogin` (M1-41), `TrustedHeaderLogins` (M1-43), `Directory`, `Devices`, `Tokens` and `TokenAdmin` (M1-44), `PrincipalWriter` (M1-51), `Lifecycle` (M1-45), `Owners`, `PrincipalWrites` (M2-24); declares `AvatarFiles`, `AppInstallations` (M2-24).
- Emits in-tx events: `DeviceRevoked`, `SessionEnded`.
- Subscribes to in-tx events: none.
- Implements source ports and cross-cutting ports: kernel `actor.Authenticator`, `authz.PrincipalSource` (with `HasActivePasskey`, M1-40), `authz.CredentialSource` (session, device, token for the check `check_active`, 07 section 4.2), `instance.LoginOptionsSource`, `instance.FlagUserSource`, kernel `personaldata.Participant`.

## Dependencies

May use the ports of these modules: `instance`, `authz`.

Reason: signing keys for access tokens; implements `PrincipalSource` and `CredentialSource`.

In addition, for all modules: `internal/kernel`; `internal/platform` and `api/` only in `adapters/`.
Checked by go-arch-lint (ADR 0008).

## Tables

`principals`, `users`, `owners`, `identity_providers`, `user_identities`, `local_credentials`, `enrollment_codes`, `auth_login_flows`, `auth_challenges`, `device_pairings`, `passkey_credentials`, `devices`, `device_keys`, `auth_sessions`, `refresh_token_families`, `refresh_tokens`, `api_tokens`.

Only this module writes and reads these tables by SQL (03-data-model.md, principle 12).

## Actions (05-actions.md)

`instance.bootstrap`, `auth.login.begin`, `auth.login.complete`, `auth.token.refresh`, `auth.logout`, `auth.step_up`, `identity_provider.read`, `identity_provider.manage`, `owner.add`, `owner.remove`, `owner.recover`, `self.read`, `user.read`, `user.create`, `user.update`, `user.disable`, `user.enable`, `self.profile.update`, `identity.read`, `identity.link`, `identity.unlink`, `local_credential.set`, `passkey.register`, `passkey.revoke`, `device.list`, `device.approve`, `device.revoke`, `device.expire`, `session.list`, `session.revoke`, `session.extension.create`, `token.list`, `token.create`, `token.revoke`, `access.view_as.end`.

## Events and WebSocket ops

- Durable: `user.updated`, `session.revoked`, `device.pending`.
- Ephemeral: none.
- WebSocket ops (client to server): none.

## Background jobs

| Job | Trigger | Queue |
|---|---|---|
| `identity.auth_cleanup` | periodic every `auth.cleanup_interval`: delete expired challenges, revoke inactive devices (M1-35, M1-45) | `maintenance` |

## Notes for the build

- Tokens only as HMAC with a server pepper; refresh rotation with a lock on the family; reuse revokes the family and the session (03, 4.2).
- Implements `kernel/actor.Authenticator` for `platform/httpserver` and `gateway`.
- Revoking a device or a session closes connections with `4012` through the event `session.revoked`. Logout and refresh reuse do not raise the permission version: `authorize` reads the session state the Authenticator read for the request (docs/DECISIONS.md M1-34/2); `device.revoke` and `session.revoke` (M1-45) keep the rights change of 05.

## Chapters

[02-architecture.md](../../../../docs/spec/02-architecture.md), [08-identity.md](../../../../docs/spec/08-identity.md)
