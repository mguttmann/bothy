# Module `identity`

Status: principal tables and directory built (M1-26); devices, device keys and the DPoP verifier built (M1-33); sessions, logins and tokens follow with the tickets of the milestone.
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
  failure), `CredentialSource` (device state; sessions and tokens not active until M1-34 and
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

## Port (root package `internal/identity`)

- Inbound: `Auth` (login, refresh, logout, child sessions, step-up), `Directory`, `Devices`, `Tokens`, `Owners`; declares `AvatarFiles`.
- Emits in-tx events: `DeviceRevoked`, `SessionEnded`.
- Subscribes to in-tx events: none.
- Implements source ports and cross-cutting ports: kernel `actor.Authenticator`, `authz.PrincipalSource`, `authz.CredentialSource` (session, device, token for the check `check_active`, 07 section 4.2), `instance.LoginOptionsSource`, `instance.FlagUserSource`, kernel `personaldata.Participant`.

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
| `identity.session_cleanup` | periodic | `maintenance` |

## Notes for the build

- Tokens only as HMAC with a server pepper; refresh rotation with a lock on the family; reuse revokes the family and the session (03, 4.2).
- Implements `kernel/actor.Authenticator` for `platform/httpserver` and `gateway`.
- Revoking a device or a session increments the permission version (05, column Permissions) and closes connections with `4012` through the event `session.revoked`.

## Chapters

[02-architecture.md](../../../../docs/spec/02-architecture.md), [08-identity.md](../../../../docs/spec/08-identity.md)
