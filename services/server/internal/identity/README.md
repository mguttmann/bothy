# Module `identity`

Status: skeleton (M1-05). The port package exists; interfaces and use cases come with the tickets of the milestone.
The normative source is [docs/spec/02-architecture.md](../../../../docs/spec/02-architecture.md); if this file differs, the chapter applies.

## Purpose

Principals, people, owners, login drivers, identities, local credentials, enrollment codes, login flows, device pairing, passkeys, devices and device keys, sessions, refresh families, API tokens, step-up, access tokens.

## Code

- `internal/identity` (`doc.go`): port as the root package, so far only with a package comment. Interfaces, DTOs and In-Tx events come with the ticket that implements them.
- `domain/`, `app/` and `adapters/<kind>/` are created with the first use case. `tools/archgen` generates their components from `architecture/modules.yaml`; a package outside these layers fails `make arch`.

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

`instance.bootstrap`, `auth.login.begin`, `auth.login.complete`, `auth.token.refresh`, `auth.logout`, `auth.step_up`, `identity_provider.read`, `identity_provider.manage`, `owner.add`, `owner.remove`, `owner.recover`, `self.read`, `user.read`, `user.create`, `user.update`, `user.disable`, `user.enable`, `self.profile.update`, `identity.read`, `identity.link`, `identity.unlink`, `local_credential.set`, `passkey.register`, `passkey.revoke`, `device.list`, `device.approve`, `device.revoke`, `device.expire`, `session.list`, `session.revoke`, `session.extension.create`, `token.list`, `token.create`, `token.revoke`.

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
