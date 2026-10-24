# Module `apps`

Status: M2-24 brings the app manifest, the installation, upgrade, scope change, suspend, resume and uninstall of apps with their nine REST operations (section "Apps (M2-24)" below). M2-26 brings the outgoing webhooks (section "Outgoing webhooks (M2-26)" below). M2-27 brings the incoming webhooks (section "Incoming webhooks (M2-27)" below). Commands and components of apps come with M2-25.
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
- `internal/apps` (`webhooks.go`, M2-26): the port `Webhooks` with `CreateSubscription`, `UpdateSubscription`, `DeleteSubscription`, `ListSubscriptions`, `ListDeliveries`, `Redeliver`, `ResumeInstallation`, its DTOs and the constants of the subscription states, delivery statuses and `last_error` codes.
- `domain/` (M2-26): `subscription.go` (states, `NextSubscriptionState`, `ParseAllowedHosts`, `ValidateTarget`, `AllowsHost`, `ValidateEventTypes`, `AppSubscribable`), `delivery.go` (`Classify`, `Backoff`, `ParseRetryAfter`, `NextAttempt`), `signature.go` (`SignedContent`, `DeriveHMACKey`, `FormatSecret`, `ParseSecret`, `HMACSignature`, `SignatureHeader`, `HMACGenerations`).
- `app/` (M2-26): `WebhookService` (`webhooks.go`, the use cases of the port), `Consumer` (`consumer.go`, the handler of the worker consumer `webhooks` with the loss hook), `Deliver` (`deliver.go`, the attempt of the job) and the outgoing ports in `webhookports.go` (`WebhookStore`, `DeliveryScheduler`, `WebhookSender`, `HMACMasterKey`, `Signers`).
- `adapters/postgres/` (M2-26): `queries/webhooks.sql` and `webhooks.go`, the `WebhookStore` of `Repository`.
- `adapters/jobs/` (M2-26): the job `apps.webhook_deliver` (`WebhookDeliver`, `Register`) and `Scheduler`, the `DeliveryScheduler` over `jobs.Inserter.InsertTx`.
- `adapters/webhookhttp/` (M2-26): `Sender`, the `WebhookSender` over one egress client with the span `webhook.deliver`.
- `adapters/hmackey/` (M2-26): `Master`, the `HMACMasterKey` over `webhook.hmac_master_key_ref`.
- `adapters/httpapi/` (M2-26): `Webhooks`, the strict-server handlers of the six webhook operations.
- `internal/apps` (`incoming.go`, M2-27): the port `IncomingWebhooks` with `ListIncoming`, `CreateIncoming`, `RevokeIncoming`, `AuthenticateIncoming`, `PostIncoming` and the DTOs `IncomingWebhook`, `CreateIncomingCmd`, `IncomingCreated`, `PostIncomingCmd`, `IncomingPosted`.
- `domain/incoming/` (M2-27): the pure rules of 11 section 8: `Classify` and `CheckEnabled` (`format.go`), `CheckNative` (`native.go`), `ParseAlertmanager` and `RenderAlertmanager` (`alertmanager.go`, goldens in `testdata/alertmanager`), `ChooseTarget` (`target.go`), `HashContext` (`hashctx.go`).
- `app/` (M2-27): `IncomingService` (`incoming.go`, built by `NewIncoming` with `IncomingConfig` and `IncomingDeps`; the use cases of the port and `revokeIncomingForApp` for the uninstall), `WebhookActor`, the outgoing port `IncomingStore`, the consumer ports `IncomingTokens`, `IncomingThreads`, `IncomingMessages`, and `ArchiveSubscriber` (`NewArchiveSubscriber`, `OnChannelArchived`).
- `adapters/postgres/` (M2-27): `queries/incoming_webhooks.sql` and `incoming.go`, the `IncomingStore` of `Repository`.
- `adapters/httpapi/` (M2-27): `Incoming` (`incoming.go`), the strict-server handlers of the five incoming webhook operations, and `TokenStage` with `CredentialGuard` and `VerifiedWebhook` (`incoming_stage.go`).
- `adapters/appstest/` (M2-24): `Install` and `Manifest` for the tests of M2-24 to M2-28; only `_test.go` files import it.
- `services/server/test/appsworld/` (M2-24): the integration world (PostgreSQL, the real modules, the full HTTP chain); the integration tests of `app`, `adapters/httpapi` and `adapters/postgres` share it, so each package is measured by its own tests (D-M2-24-23).
- Their components come from `tools/archgen` and `architecture/modules.yaml`; a package outside these layers fails in `make arch`.

- Restore cursors (M7-04): the port `RestoreCursors` (`restore.go`) with `MoveCursorsTo(ctx,
  headSeq)`, implemented by `app.NewRestoreCursors` over the outgoing port `CursorMoveStore`
  (`adapters/postgres`, query `MoveAllSubscriptionCursors`): it sets `webhook_subscriptions.last_delivered_seq` of every row to the
  head of the restored log inside the caller's unit of work and refuses to run outside one.
  `bothy restore finalize` calls it through `internal/wire` (`RestoreCursors`) in the transaction
  that sets the new sync epoch (docs/spec/03-data-model.md 6.7 item 1).

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
| `apps.webhook_deliver` | one attempt of a subscription: a round of the consumer `webhooks`, the chain after a finished delivery, a scheduled retry, the lease wake-up, an owner resume or re-enable, `webhook_out.redeliver`, `app.resume` | `delivery` |

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
components (hide on suspend, remove on uninstall). M2-26 added the deliveries and subscriptions,
M2-27 the incoming webhooks (revoke on uninstall, refuse posts while suspended; both below).

## Outgoing webhooks (M2-26)

Normative: [docs/api/webhooks.md](../../../../docs/api/webhooks.md), [11-extensions.md](../../../../docs/spec/11-extensions.md)
sections 6 and 7, ADR 0016 and 0051. Rulings: `docs/DECISIONS.md` D-M2-26-1 to D-M2-26-20.

- **Subscriptions** (`webhook_subscriptions`, retention class `config`): created, changed, paused,
  resumed, rotated and deleted by an owner with step-up (`webhook_out.manage`, audited with the
  host only); lists and deliveries need `webhook_out.read`; `redeliverWebhook` needs
  `webhook_out.redeliver`. The address is `https`, its host and port an entry `host[:port]` of
  `webhook.allowed_hosts` (`domain.ValidateTarget`, the same rule for manifest addresses and the
  egress policy). `event_types` are subscribable by apps and granted to the installation. A
  manifest with `delivery.mode = webhook` gets its subscription in the install transaction;
  uninstall disables every subscription; suspend keeps them and sends nothing; resume makes the
  heads due.
- **States**: `active`, `paused`, `disabled`. The owner pauses, resumes and re-enables (failures
  reset, the head due now). The system disables after `410` (cause `gone`), after
  `webhook.max_consecutive_failures` counted failures (`max_consecutive_failures`; the head stays
  pending), after `webhook.retry_max_age` (`retry_max_age`) and on a gap (`gap`), and pauses at the
  delivery quota `webhook.max_pending_per_subscription` (`max_pending`); each system change is one
  entry of `webhook_out.transition` without an actor.
- **Consumer** `webhooks`: in the round's transaction one `pending` row per matching `active` or
  `paused` subscription of an `active` or `suspended` installation (a personal event only for its
  audience), the quota on the crossing, one job per active subscription; a range lost to
  retention disables every live subscription (`HandleLost`).
- **Deliveries** (`webhook_deliveries`, retention class `transient`): statuses `pending`,
  `succeeded`, `failed` (after `410`), `dead` (after `webhook.retry_max_age`, `not_authorized`,
  `not_subscribed`, `event_expired`); `last_error` is a code of `apps.LastErrorCodes`, never a
  response body. Retention (M7-05) may delete only `succeeded`, `failed` and `dead` rows older than
  `retention.webhook_deliveries`, never `pending` ones.
- **Attempt** (job `apps.webhook_deliver`): claim the due head under the subscription lock, check
  routing and `webhook_out.deliver` in filter mode with the current grants and scopes (no request
  without access), sign, take a lease of `webhook.delivery_timeout` with a wake-up job, send
  through the egress client outside any transaction, record the outcome and chain the next job.
  Strictly in `seq` order; at least once with the same `webhook-id`.
- **Signatures**: `v1a` with every signer of the purpose `webhook_out` (`instance.Keys.Signers`);
  `v1` per subscription with the key `HMAC-SHA256(master, "webhook-v1:" + id + ":" + generation)`,
  never stored, shown once as `whsec_`; inside `webhook.secret_rotation_overlap` the previous
  generation signs too. The master key is the value of `webhook.hmac_master_key_ref` (secret
  `webhook_hmac_master_key`, generator `key`); changing it changes every derived secret, owners
  then rotate (`rotate_hmac_secret`). Committed test vectors:
  [docs/api/webhook-test-vectors.json](../../../../docs/api/webhook-test-vectors.json).

## Incoming webhooks (M2-27)

Normative: [docs/api/webhooks.md](../../../../docs/api/webhooks.md) section 8,
[11-extensions.md](../../../../docs/spec/11-extensions.md) section 8, 05 section 4.11, 08
section 12. Rulings: `docs/DECISIONS.md` D-M2-27-1 to D-M2-27-23.

- **Webhooks** (`incoming_webhooks`, migration `00055_apps_incoming_webhooks.sql`, retention class
  `config`): `createIncomingWebhook` (`webhook_in.create`, `manage` on the channel, step-up,
  audited with `{webhook_id, app_id, thread_id}` only) for an app with an `active` installation
  that may write in the channel (checked with its webhook actor in filter mode), optionally with a
  fixed thread of the channel; else `400 validation_failed` with `app_not_installed`,
  `app_cannot_write` or `not_in_channel`. The answer carries the absolute URL from
  `server.public_url` with the `bothy_whi_` token once (`idempotency.MarkOneTimeSecret`); the row
  holds only the HMAC (`identity.Tokens.NewIncomingWebhookToken`). `listIncomingWebhooks`
  (`webhook_in.read`, `manage`) lists the active webhooks of the channel, never a token or hash;
  `revokeIncomingWebhook` (`webhook_in.revoke`, step-up, audited) sets `revoked_at`, a second
  revoke is audited with `no_change: true`.
- **Token check** (`TokenStage` at `httpserver.StageWebhookToken`, installed by
  `wire.InstallWebhookToken`): after `StageRateLimit` (`rl.webhook_in` per `webhook_id`) and
  before `StageMaintenance` and the body stages, only for operations with action
  `webhook_in.post`. `postIncomingWebhook` takes the path value `token`,
  `postIncomingWebhookWithBearer` exactly one `Authorization: Bearer <token>`; `StageAuthenticate`
  leaves both anonymous (`httpserver.Operation.AnonymousCredentials`). `AuthenticateIncoming`
  hashes the token (`HashIncomingWebhookToken`, a malformed one without a lookup), reads the
  webhook with its installation that is not uninstalled by the primary key and compares with
  `hmac.Equal`. Unknown id, wrong or malformed token, revoked webhook and an installation that is
  not `active` give the same `404 not_found`; a failure keeps one token of `rl.public_auth` of the
  client IP (`CredentialGuard` over `ratelimit.Limiter.FailedCredentialGuard`), and while that
  bucket is empty the stage answers `429 rate_limited` without checking. Success puts the webhook
  actor into the context and the id for `VerifiedWebhook`; the handler uses the path id only when
  it equals that id. A declared length above `webhook_in.max_body_bytes`
  (`httpserver.Operation.BodyLimit`) is refused at `StageBody`, so the token answers first.
- **Webhook actor** (`WebhookActor`): the app principal with its active installation and the
  granted scopes of the installation intersected with `messages:write`, without token, session or
  device; a stolen URL can only post. `webhook_in.post` requires `All(AppKind,
  AtLeast(LevelWrite))` on the webhook's channel.
- **Post** (`PostIncoming`): `Classify` decides the format per request (`format_mixed`), the
  format must be in `webhook_in.formats_enabled` (`format_disabled`), then `CheckNative` (content
  within `message.max_content_bytes`, `client_msg_id` a UUIDv7 or a new server one) or
  `ParseAlertmanager` and `RenderAlertmanager` (one message per notification, a new server
  `client_msg_id` every time). One unit of work locks the row `FOR SHARE`, `ChooseTarget` picks
  the fixed thread (a fixed thread moved to another channel is `404`), the body's `thread_id` in
  the same channel, a new thread with `title` (forum post, or in a text channel a thread started
  from the message in the main thread, `TriggerChannel.MainThreadID`) or the main thread
  (`required_for_forum_post` in a forum), authorizes `webhook_in.post` and writes through
  `messaging` with `SkipMentions` and `HashContext`. The message has `origin = app` with the app
  as author, resolves no mentions and triggers no agent. A replay answers `200` with
  `Idempotent-Replay: true` and, for a text-channel title, the thread found through
  `chat.ThreadsByParent`. Every refusal of the app's rights and a message that ended in another
  channel become the `404` of an unknown webhook. After the commit `last_used_at` moves at most
  once per `identity.last_active_granularity`; a failure is logged without the token.
- **Lifecycle**: uninstall revokes every active webhook of the app in its unit of work
  (`revokeIncomingForApp`); suspend keeps the rows and posts give `404` until resume;
  `ArchiveSubscriber.OnChannelArchived` (`chat.ChannelArchived`, registered after the runs
  acceptor) revokes the active webhooks of the channel with `ArchivedAt` as `revoked_at` in the
  archive transaction.
- **Internal listener**: both post operations are mounted there in every role through a chain of
  their own with empty trusted proxies (client IP = peer) and its own rate limiter; nothing else of
  the API.
- **Configuration**: `webhook_in.max_body_bytes`, `webhook_in.formats_enabled` (start check
  `webhook_in_formats`), `message.max_content_bytes`, `identity.last_active_granularity`,
  `server.public_url`, `ratelimit.rl.webhook_in.*`, `ratelimit.rl.public_auth.*`.

## Statistics source (M6-05)

`apps.StatsSource` (`stats.go`, implemented by `app.Stats` over
`adapters/postgres/queries/stats.sql`) serves `getErrorStats`: `WebhookStats` counts the finished
deliveries and the failed and dead ones per bucket of `created_at`, as far back as the retention of
`webhook_deliveries` reaches. The deliveries have no channel, so the stats use case calls it only
for an owner on the instance.
