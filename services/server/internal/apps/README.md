# Module `apps`

Status: M2-24 brings the app manifest, the installation, upgrade, scope change, suspend, resume and uninstall of apps with their nine REST operations (section "Apps (M2-24)" below). Webhooks, incoming webhooks, commands and components of apps come with M2-25 to M2-27.
The normative source is [docs/spec/02-architecture.md](../../../../docs/spec/02-architecture.md); if this file differs, the chapter applies.

## Purpose

Apps and manifests, installations, scopes, outgoing webhooks with consumer and delivery, incoming webhooks.

## Code

- `internal/apps` (`apps.go`, M2-24): the port `Apps` with `List`, `Get`, `Validate`, `Install`, `Upgrade`, `SetScopes`, `Suspend`, `Resume`, `Uninstall` and the DTOs `App`, `Installation`, `InstallCmd`, `UpgradeCmd`, `SetScopesCmd`, `ManifestValidation`, `FieldError`, `Page`. No in-transaction event.
- `domain/` (M2-24): `manifest.go` (the typed `app_manifest.v1`, `ParseManifest`, `Hash`, `SameContent`), `rules.go` (`ValidateManifest` with `Env`, pure; the rules of 11 section 3.2), `semver.go` (SemVer 2.0.0 parse and precedence), `consent.go` (`ConsentInstall`, `ConsentUpgrade`, `ConsentSetScopes`), `diff.go` (`DiffManifests`), `installation.go` (states and `Next`).
- `app/` (M2-24): `Service`, one use case per operation; the outgoing `Repository` port and the consumer ports `Principals` (with `IdentityPrincipals` over `identity.Directory` and `identity.PrincipalWrites`), `Tokens`, `Grants`, `Rights`, `Scopes` and `Cursors`.
- `adapters/postgres/` (M2-24): sqlc queries in `queries/`, generated code in `sqlcgen/`, `Repository`.
- `adapters/httpapi/` (M2-24): the strict-server handlers of the nine operations and `Cursors`.
- `adapters/identitysource/` (M2-24): `Source`, the implementation of `identity.AppInstallations`.
- `adapters/appstest/` (M2-24): `Install` and `Manifest` for the tests of M2-24 to M2-28; only `_test.go` files import it.
- `services/server/test/appsworld/` (M2-24): the integration world (PostgreSQL, the real modules, the full HTTP chain); the integration tests of `app`, `adapters/httpapi` and `adapters/postgres` share it, so each package is measured by its own tests (D-M2-24-23).
- Their components come from `tools/archgen` and `architecture/modules.yaml`; a package outside these layers fails in `make arch`.

## Port (root package `internal/apps`)

- Inbound: `Apps`, `Webhooks`, `IncomingWebhooks`.
- Emits in-tx events: none.
- Subscribes to in-tx events: `chat.ChannelArchived`.
- Implements source ports and cross-cutting ports: `identity.AppInstallations` (`adapters/identitysource`).

## Dependencies

May use the ports of these modules: `instance`, `identity`, `authz`, `chat`, `agents`, `messaging`, `sync`.

Reason: webhook signing key; app principal and tokens; revoke the grants of the app at the uninstall; scope table for event subscriptions; target of the incoming webhooks; commands of apps; messages from webhooks; consumer for outgoing webhooks.

In addition, for all modules: `internal/kernel`; `internal/platform` and `api/` only in `adapters/`.
Checked by go-arch-lint (ADR 0008).

## Tables

`apps`, `app_installations`, `webhook_subscriptions`, `webhook_deliveries`, `incoming_webhooks`.

Only this module writes and reads these tables by SQL (03-data-model.md, principle 12).

## Actions (05-actions.md)

`app.read`, `app.install`, `app.scopes.update`, `app.suspend`, `app.resume`, `app.uninstall`, `webhook_out.read`, `webhook_out.manage`, `webhook_out.redeliver`, `webhook_out.deliver`, `webhook_out.transition`, `webhook_in.read`, `webhook_in.create`, `webhook_in.revoke`, `webhook_in.post`.

## Events and WebSocket ops

- Durable: none.
- Ephemeral: none.
- WebSocket ops (client to server): none.

## Background jobs

| Job | Trigger | Queue |
|---|---|---|
| `apps.webhook_deliver` | consumer `webhooks`, `webhook_out.redeliver` | `delivery` |

## Notes for the build

- Apps never run in the server process; they act only with a token, scopes and grants (B6).
- Outgoing webhooks only to `https` targets from `webhook.allowed_hosts` (R-DM-17); before every delivery, `webhook_out.deliver`.
- Messages from incoming webhooks have `origin = app` and never trigger agents.

## Chapters

[02-architecture.md](../../../../docs/spec/02-architecture.md), [11-extensions.md](../../../../docs/spec/11-extensions.md)

## Apps (M2-24)

Normative: [11-extensions.md](../../../../docs/spec/11-extensions.md) sections 3 to 5, 05 section
4.11, 07 section 2.10, 08 section 12. Rulings: `docs/DECISIONS.md` D-M2-24-1 to D-M2-24-17.

- **Manifest.** Uploaded, never fetched. Request validation checks the schema `AppManifest`
  (unknown members are `unknown_member`), then `domain.ValidateManifest` the rules: size
  (`app.manifest_max_bytes`), slug as handle (`principal.handle_max_chars`), name as display name,
  texts through `kernel/textcheck`, SemVer, `min_api_version` not above `routes.SpecVersion`,
  scopes of the catalogue, only subscribable events with the scope of their receive action and
  `events:subscribe` (`authz.AppScopes.EventScope`), the conditional delivery members, `https`
  addresses on hosts of `webhook.allowed_hosts`, `requested_grants` at most `write` and `upload`
  and never on a host, commands without reserved names, at most `command.max_per_principal`.
  `apps.manifest` keeps the content as jsonb, `apps.manifest_sha256` the SHA-256 of the bytes as
  received (the handler reads them with `httpserver.RawJSONBody`).
- **Install** (`app.install`, owner, step-up): authorize before anything is loaded; one unit of
  work declared as a rights change creates the principal (kind `app`, handle = slug, display
  name = name), `apps` and the `active` installation with the consented lists and writes the
  allow entry. No token, no grant: grants per target with `grant.create`, tokens with
  `createToken`. An app whose installations are all uninstalled is reinstalled with the same
  principal; its old tokens and grants stay revoked.
- **Upgrade** (`app.install`): same slug, new content needs another version; never adds a scope,
  an event or a grant (new ones need `setAppScopes`); what the new manifest drops acts at once;
  new delivery hosts are consented by the upgrade itself and named in the audit entry.
- **Scopes** (`app.scopes.update`): the consent for new scopes and events; a removed scope acts on
  the next request of every token, a newly granted one needs a new token.
- **Suspend, resume, uninstall**: suspend keeps grants and tokens, the tokens stop working
  because the installation is not active (identity reads it on every request) and the empty
  snapshot of the app writes `access.changed`; uninstall revokes every token
  (`identity.Tokens.RevokeAllForPrincipal`) and grant (`authz.Admin.RevokeSubjectGrants`) and
  disables the principal; the messages of the app keep their author.
- **Without effect**: an identical upgrade, unchanged scopes and a transition to the current state
  are decided before the rights change and audited with `no_change: true`; no rights version.
- **Reads**: `listApps` and `validateManifest` check `app.read` on the collection of apps
  (`403 forbidden` with the reason `owner_only` for a non-owner); `getApp` on the app (404 for a
  non-owner, like an unknown id).

Later tickets extend these use cases with their parts of 11 section 4.3: M2-25 commands and
components (hide on suspend, remove on uninstall), M2-26 deliveries and subscriptions (pause on
suspend, disable on uninstall), M2-27 incoming webhooks (revoke on uninstall).
