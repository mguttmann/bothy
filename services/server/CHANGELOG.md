# Changelog

All notable changes to the server component (`services/server`, binary `bothy`) are
documented in this file. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this component adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html). Its version is the
[`VERSION`](VERSION) file next to this one; release tags are `server/v<version>`
(docs/spec/18-release.md).

## [Unreleased]

### Added

- `bothy import login` and `bothy import logout` (M8-10): the import tool logs in as a `cli`
  device through device pairing (Ed25519 DPoP key, no step-up key), prints the pairing code alone
  on stdout and polls `completePairing` every `import.login_poll_interval` (new tool key) until the
  owner approves; the session file (`--session-file`, mode `0600` in `0700`) holds device, key and
  tokens. `cmd/bothy/importtool/bothyapi` is the authenticated client for the later import
  commands: DPoP proofs on every request with one `use_dpop_nonce` retry, refresh under the lock
  of the session file with the rotated token stored before use, `429` within the tool's limits,
  no redirects, all through `platform/egress`, and no secret in output, log or error. The API
  client `api/restclient` is generated from the whole `docs/api/openapi.yaml` and split below
  `repo.max_file_kb`.
- Notification levels and quiet hours (M3-18): the module `notify` owns `notification_preferences`
  and `quiet_hours` and serves `listMyNotificationPreferences`, `putNotificationPreference`,
  `deleteNotificationPreference`, `listMyQuietHours`, `createQuietHoursRule`, `putQuietHoursRule`
  and `deleteQuietHoursRule`. Every change writes one `notification_pref.updated` or
  `quiet_hours.updated` for the person; an archived target refuses a new level with
  `409 conflict` and `reason` `target_archived`; archiving a space or channel removes the levels
  below it in the same transaction. `Facts.LevelFacts` and `Facts.QuietRules` gather the facts of the push path in one
  query each. New configuration key `notify.user_default_level`. Agents and apps get
  `subject_kind_not_allowed` for `notification_pref.update` and `quiet_hours.update`.
- Search vectors (M3-13): migration `00035` adds `messages.search_language`,
  `messages.search_vector`, `threads.search_language` and `threads.title_search` as `STORED`
  generated columns with the expressions of 03 section 5 and the platform function
  `search_tsquery(regconfig[], text)` of 14 4.2 (executable only by `bothy_app`); migration
  `00036` builds their GIN indexes concurrently. Every new message and thread takes the language
  of its author or creator through `search.locale_map` (the instance language for agents, apps
  and system lines), set once at creation and never from a request. New configuration keys
  `search.languages` and `search.locale_map` with the new kind `map(string)` and the checks
  `search_languages` and `search_locale_map`; the start stops when a configured text search
  configuration is missing in the database or the database does not fold the case of non-ASCII
  letters. Adding the generated columns rewrites `messages` and `threads` once (no release
  exists yet).
- Outgoing webhooks (M2-26): migrations `00037_webhook_subscriptions.sql` and
  `00038_webhook_deliveries.sql`; the port `apps.Webhooks` with the operations
  `createWebhookSubscription`, `updateWebhookSubscription`, `deleteWebhookSubscription`,
  `listWebhookSubscriptions`, `listWebhookDeliveries` and `redeliverWebhook`, and `getWebhookKeys`
  (public keys of the purpose `webhook_out` as `whpk_`). The worker consumer `webhooks` creates one
  delivery per matching subscription; the job `apps.webhook_deliver` (queue `delivery`) sends them
  as Standard Webhooks through the egress client, strictly in `seq` order per subscription, signed
  with `v1a` (Ed25519) and, per subscription, `v1` (HMAC with a derived secret shown once), after a
  check of `webhook_out.deliver` with the current grants and scopes; with backoff and full jitter,
  `Retry-After`, `410`, failure limit, retry deadline, gap and delivery quota. Committed test
  vectors `docs/api/webhook-test-vectors.json` (gate G10). `webhook.allowed_hosts` entries take the
  form `host[:port]` (without a port only `443`), also for manifest addresses. `sync` gains
  `ConsumerLossHandler` and `LogReader`, `instance.Keys` gains `Signers` and `PublicKeys`, authz
  decides `webhook_out.deliver`, and the system action `webhook_out.transition` audits automatic
  subscription changes. New keys `webhook.max_consecutive_failures`, `webhook.retry_backoff_base`,
  `webhook.retry_backoff_cap`, `webhook.max_pending_per_subscription`, `webhook.retry_max_age`,
  `webhook.secret_rotation_overlap` and `webhook.hmac_master_key_ref` with the cross-checks
  `webhook_backoff_order` and `webhook_retry_within_retention`; new generated secret
  `webhook_hmac_master_key`; new log attribute `bothy.webhook_subscription.id`.
- Apps (M2-24): the migrations `00030_apps.sql` and `00031_app_installations.sql` (with the trigger
  `apps_principal_kind_check`), the port `apps.Apps` and the operations `listApps`, `installApp`,
  `validateManifest`, `getApp`, `upgradeAppManifest`, `setAppScopes`, `suspendApp`, `resumeApp` and
  `uninstallApp` (owner only; the writing ones with step-up, audit and a new rights version). The
  manifest `app_manifest.v1` follows 11 3.1 in `openapi.yaml` and is checked against the rules of
  11 3.2 (scopes of the catalogue, subscribable events, `https` delivery hosts from
  `webhook.allowed_hosts`, never `trigger` or `manage`, no reserved command names); the server stores
  it with the SHA-256 of the bytes as received. An upgrade never adds a scope, an event or a grant;
  `setAppScopes` is the consent. Suspend keeps grants and tokens, uninstall revokes them and
  disables the principal. `createToken` issues app tokens with a subset of the granted scopes of an
  active installation; authz requires the scope of an action in the token and in the installation,
  and an app without an active installation is unauthenticated (`app_not_installed`) with an empty
  rights snapshot. New ports `identity.AppInstallations`, `identity.PrincipalWrites`,
  `authz.AppScopes` and `authz.Admin.RevokeSubjectGrants`; new configuration keys
  `app.manifest_max_bytes`, `webhook.allowed_hosts`, `command.max_per_principal`,
  `command.description_max_chars` and `audit.reason_max_chars`; `httpserver.RawJSONBody`; a
  `depguard` rule forbids the package `plugin` in the server module (ADR 0022). Manifests with
  the escape of NUL or of an unpaired surrogate are refused, and manifest values that are no audit
  code enter the audit hashed, so no checked manifest makes install or upgrade fail with 500.
- Contract tests G7 and G8 (M1-71): `make contract-test` runs Schemathesis against the server of
  this tree with a throwaway PostgreSQL and the matrix world (every enabled operation as owner and as
  a plain user, the stateful phase, SEC-T06, the project checks problem format, `Correlation-Id`, no
  JSON `null` and 404 equality); `go test -tags contract_validate` validates every response of
  every integration test against `docs/api/openapi.yaml` (`internal/platform/httpserver/contractcheck`,
  exit code 3 on a violation, absent from the production build). The shared test harness
  `test/harness` runs the real server in the test process, and `egress.Deps.DenyAll` with
  `wire.Deps.EgressDenyAll` keeps a test server off the network.
- Moving threads (M1-62): `moveThread` moves a thread into another channel whose effective
  operator equals the thread's fixed operator (otherwise `409 operator_mismatch`); it needs
  `manage` on source and target (an invisible target is the same `404` as an unknown one), is
  audited with from, to and reason, uses `rl.admin` and accepts `Idempotency-Key`. The thread id
  stays; `messages.channel_id`, `read_states.channel_id` and `drafts.channel_id` follow in the same
  transaction through the new in-transaction event `chat.ThreadMoved`, and the tags are removed.
  `thread.moved_out` (source, never naming the target) and `thread.moved` (target) are appended in
  this order. Main threads, the same channel, archived targets and DM channels are refused.
- Core metrics per catalogue (M1-68): one Go declaration per instrument
  (`internal/platform/otel/metrics`) with checks for closed attribute sets, no person or ID
  attribute and bucket bounds, and the SDK view that keeps only catalogue instruments and keys
  with the bounds of signals.md 4.7, library instruments included. New instruments:
  `bothy.authz.decisions`, the gateway set (`connections.active`, `connect.duration`,
  `catchup.duration`, `resync`, `close`, `outbound.dropped`), `bothy.delivery.duration`,
  `bothy.event.fanout.lag`, `bothy.message.created` and `bothy.event.appended` (both after the
  commit), `bothy.db.event_seq.lock_wait`, `bothy.job.duration`, `bothy.job.queue`,
  `db.client.operation.duration` from the pool's own query tracer with closed
  `db.operation.name` and `db.collection.name` (the otelpgx metrics are dropped) and
  `bothy.db.pool.connections`. A value outside a closed set records as `other`.
  `txn.AfterCommit` runs registered functions after the commit of a unit of work. The gate
  `make signal-check` holds dashboards, rules and SLOs to the catalogue.
- Gateway frame conformance, gate G9 (M1-67): the test harness `wstest` validates every frame of
  the gateway integration tests as it arrives against `docs/api/asyncapi.yaml` and the body
  schemas of `openapi.yaml` (new test package `api/specschema`: one JSON Schema 2020-12 bundle,
  strict about undeclared members, no `null`, integers below 2^53, at most
  `sync.ws_max_frame_bytes`, `seq` and `permission_version` exactly on durable events,
  `resource.version` as the catalogue says), each client frame on send, the order per connection
  and every close code against `x-close-codes`; a violation fails the test. The register of ops and
  event types and the enable list `gateway_ops` are compared with actions.yaml and asyncapi.yaml,
  a corpus test observes every M1 event type and server op, and a parser test keeps the harness
  the only WebSocket client of the tests. New direct dependency
  `github.com/santhosh-tekuri/jsonschema/v6` (test code only).

- Gateway focus and typing (M1-66): the ops `focus {thread_ids, idle}` (`gateway.focus` per
  thread, `rl.read` per principal, at most `sync.max_focused_threads` threads, unreadable and
  unknown threads dropped silently), `typing.start` and `typing.stop` (`typing.start`, people
  with `write` only, `rl.typing` per connection) are enabled. `typing.started {thread_id,
  principal_id, expires_at}` reaches the live connections that focus the thread and may read it,
  never the typist's own connections, at most once per `sync.typing_min_interval` per connection
  and thread, with `expires_at = occurred_at + sync.typing_ttl`; `typing.stop` only resets that
  throttle. The hub is the process's `kernel/eventlog.EphemeralPublisher`: ephemeral frames carry
  no `seq` and no `permission_version`, are never sent before `catchup.complete`, are dropped on
  a full queue instead of closing with `4014`, and are purged with the targets `access.changed`
  removes. `access.changed`, `thread.moved`, `channel.archived` and `space.archived` re-check the
  focus, and `resume` restores it. The configuration keys `sync.max_focused_threads`,
  `sync.typing_min_interval` and `sync.typing_ttl` are active.
- Performance gate of the server core (M1-72, ADR 0063): `make perf` measures every M1 target
  (`perf.message_create_p99` under `perf.sustained_messages_per_s`, `perf.fanout_local_p99`,
  `perf.catchup_page_p99` with a fractional and a large visible set, `perf.authorize_cached_p99`,
  `perf.ws_connections_per_node`, `perf.ws_idle_conn_mem_max`, `perf.idle_rss_max`,
  `perf.startup_ready_max`, `perf.event_commit_min_per_s`) with thresholds read from chapter 02,
  writes `.cache/perf/results.json`, prints the table with the profile mode (`match`, `emulated`,
  `below`) and fails on a miss, a run below the profile or a database without `fsync` or
  `synchronous_commit`. Chaos tests of the event path: loss of the `LISTEN` connection, a database
  restart during the tail and `SIGKILL` during commits, each followed by the invariants of the
  event log, threads, idempotency keys, audit chain and receivers. The shared harness gains
  `StartProcess` (the real binary through `secrets init`, `migrate up --bootstrap`, `setup
  bootstrap-owner` and `serve`), `Options.DurableDB`, `CreateUser` and a gateway client; pgtest
  gains `Limits` and `DB.Restart`, which restarts the server of a durable container in place. `make lint`
  vets and lints the `perf` tag.
- Gateway delivery (M1-65): the log tail of every api process feeds `Authorizer.Observe` and then
  the hub, which delivers each durable event to exactly the connections whose principal may
  receive it at delivery time (`sync.Admit` in delivery mode, no denial audit), in `seq` order and
  without blocking on a slow reader (`4014` at `sync.outbound_queue_max` or a missed
  `gateway.write_timeout`); live `cursor` frames every `sync.cursor_advance_every` events; catch-up
  over the WebSocket in pages with `cursor` frames, `catchup.complete {head_seq}` and the buffered
  live events after it; `resync_required` keeps the connection live; `access.changed`
  revalidates, re-indexes and purges removed targets; `session.revoked` and revoked credentials
  close with `4012`; shutdown closes every connection with `4013 server_restart`; the liveness
  check `event_tail` fails once the tail and hub goroutine has ended. `sync.cursor_advance_every`
  is an active key.
- WebSocket gateway connection and session (M1-64): `GET /api/v1/gateway` checks the `Origin`
  (`403 forbidden` for a foreign one, SEC-CORS-3), upgrades with `github.com/coder/websocket`,
  sends `hello` with the current DPoP nonce and authenticates in-band with `identify` (access
  token or API token, DPoP proof bound to the nonce) or `resume`, rate limited by
  `rl.gateway_connect` per device or token and authorized with `gateway.connect`. `ready` carries
  the snapshot of `sync`; `resume` within `sync.resume_window` answers `resumed`; heartbeats,
  `reauth_required` and `reauth` keep the connection; the close codes `4001` to `4009`, `4011`,
  `4012`, `4014` and `4016` to `4018` follow 04 section 6.4. One connection per device (`4017`).
  Sessions live in process memory with a sweeper. The ops of the new section `gateway_ops` of
  `api/operations.yaml` are registered; every other client op is unknown (`4001`). New
  configuration keys `sync.ws_max_frame_bytes`, `sync.identify_timeout`,
  `sync.heartbeat_interval`, `sync.heartbeat_miss_limit`, `sync.resume_window`,
  `sync.reauth_lead_time`, `sync.outbound_queue_max` and `gateway.write_timeout`. The test client
  `internal/gateway/adapters/wstest` serves later tickets. Delivery of events follows with M1-65.
- `httpserver.NewSchemaValidator` checks a JSON value against a named schema of the contract;
  `httpserver.Operation.AuthorizedInBand` marks the one route that authorizes on its connection.
- `actor.Actor.CredentialExpiresAt`: the expiry of the access token or API token of a request.
- Catch-up over REST and the snapshot (M1-63): `listEvents` (`GET /api/v1/events`) delivers every
  durable event after a cursor that the caller may receive at delivery time (personal events only
  to their audience, every other event through `authorize` with its receive action) with a cursor
  that means "last checked", or `410 resync_required` with the reason `epoch_changed`,
  `cursor_ahead`, `retention` or `too_far_behind`; the optional query parameter `epoch` and the
  keys `sync.catchup_page_size` and `sync.catchup_max_events` are read. `getSnapshot`
  (`GET /api/v1/sync/snapshot`) returns the visible spaces and channels with versions, the
  sidebar, unread and mention counters, running runs (none before M2-05), agent presence
  (`unknown` before M2-20), settings, draft versions and the evaluated feature flags, without
  thread contents. Both are the sync ports `CatchUp` and `Snapshot`, which the gateway reuses;
  `authorize.ResourceFromEvent` describes a stored event for its delivery check,
  `instance.Versions.EventSeq` reads the head and `chat.Threads.Counters` the thread counters.
- Release chain (M7-10): `make -C services/server release-build VERSION=<v> OUT=<dir>` builds the
  static binaries `bothy_<v>_linux_amd64` and `bothy_<v>_linux_arm64`. A tag `server/vX.Y.Z`
  publishes `bothy-server:X.Y.Z` and `bothy-postgres:pg18-X.Y.Z` (multi-arch, built without
  emulation) only after the vulnerability scan, ScanCode, SBOMs (SPDX and CycloneDX per image
  platform and binary), signatures of every digest, binary and `SHA256SUMS`, and their
  verification; `make release-trial COMPONENT=server` runs the same steps locally.
  `docs/operations/release.md` section 5 names the verification commands of both phases.
- SLI bounds (M6-11): `internal/stats/domain/slibounds.gen.go`, generated from the Sloth files by
  `tools/observability/sli_bounds.py`, maps each `sli.*` metric to its SLO, objective, threshold
  and period; the Prometheus SLO rules carry the label `bothy_sli_metric`.
- CDN fetch of the import tool (M8-02): `discord.Fetcher` loads one Discord attachment with
  `GET` only when the link is `https` on a host of `import.discord.cdn_hosts` (exact match, port
  `443` unless the entry names one), without token or cookie, without redirects or an
  environment proxy, bounded by `import.attachment_max_bytes` (declared size, `Content-Length`
  and the bytes read) and `import.http_timeout`. On `403` or `404` it reads the message once
  more for a fresh link and tries once more (`ErrCDNGone` otherwise); `429` follows the wait
  limits of the read client. Errors and logs show scheme, host and path only (`SafeURL`); the
  body streams to the caller and never touches the disk. The fake CDN `discordtest.NewCDN`
  serves the tests.
- Image `bothy-postgres` (M7-01): `postgres:18` pinned by digest plus pgBackRest 2.59.3 from the
  PostgreSQL project's Debian repository (key checked by fingerprint, a version below 2.59.3 fails
  the build and the start) and `bothy-backupd`, built for linux/amd64 and linux/arm64 without
  emulation (`make postgres-image`). The service `postgres` archives WAL with `archive-push` over
  pgBackRest TLS to the repository host `backup` and has no egress; `backup` holds the encrypted
  repository. `make postgres-image-test` proves archive-push, a full backup, a point-in-time
  restore and `pgbackrest verify` with two containers; the CI jobs `postgres-image (amd64)` and
  `postgres-image (arm64)` run it on both architectures.
- `bothy-backupd` (M7-01): a second program of the server module that supervises the pgBackRest
  TLS server of the repository host, creates the stanza on its first start, stops on `SIGTERM`,
  and offers `healthcheck` (TLS handshake with the stack's CA) and `version`.
- `bothy secrets init` generates `pgbackrest_tls/` (M7-01): an own CA and ECDSA P-256 certificates
  for `postgres` and `backup`, copied only to those two services; the CA key is not kept.
- Reactions and pins (M1-59): `addReaction` and `removeReaction` add and remove the caller's
  reaction, one normalised Unicode grapheme cluster of at most the new key
  `reaction.emoji_max_bytes`; `listReactions` pages the reactions of a message with the principals
  and an optional `emoji` filter. `pinMessage` and `unpinMessage` need `manage` (also in a
  read-only channel), `listPins` lists the pins of a thread newest first. Migration `00041`
  creates `reactions` and `pins`. All six operations are idempotent; the four writes append
  `reaction.added`, `reaction.removed`, `pin.added` or `pin.removed` only on a change, with exact
  counts under the message row lock; none changes `trigger_state` or the message version. A
  deleted message refuses reactions and pins with `410`, and deleting a message removes both.
  Every `Message` now carries the reaction summary and `pinned`, REST responses also
  `my_reactions`. The new keys `reaction.max_distinct_per_message` and `pin.max_per_thread`
  bound distinct symbols per message and pins per thread (`400 validation_failed` (reason
  `limit_reached`)).

- Read state, drafts and personal settings (M1-61): migration `00042` creates `read_states`,
  `drafts` and `user_settings`. `putReadState` only moves the read position forward,
  `markThreadUnread` moves it back with a required `If-Match` (`428`, `412` with `current`) and
  answers the `ETag`; `listMyReadStates` lists the own rows of a readable channel. Every read
  state carries `mention_count`, computed from the unread mentions when the answer is built and
  never stored. `putThreadDraft`, `putChannelDraft` (forums only), `deleteThreadDraft`,
  `deleteChannelDraft` and `listMyDrafts` keep one draft per thread and per forum with last write
  wins in server order and `overwrote_newer`; an empty draft is deleted, drafts work in read-only
  and archived channels, and `clear_draft` on sending deletes the draft in the transaction of the
  message (subscription `messaging.MessagePosted -> sync`). `listMySettings`, `putMySetting` and
  `deleteMySetting` accept exactly the keys and values of 01 section 11 and 13 section 3.3;
  `getMySidebar` reads the own sidebar rows. Every change appends one personal event
  (`read_state.updated`, `draft.updated`, `draft.deleted`, `settings.updated`, audience the
  person, with `updated_by_device_id`); a no-op and a mention append none. Agents and apps get
  `403 forbidden` (reason `subject_kind_not_allowed`) on all twelve operations from
  `authorize`. The chat port adds `Threads.InfoForShare`, the messaging port `ReplyTargets`.

- Mentions and templates (M1-60): `listMyMentions` lists the caller's mentions in channels it may
  read, newest first, with a cursor bound to the list and the caller; a mention in an unreadable
  channel stays stored but is never listed or counted. The port `messaging.Mentions` adds
  `CountUnread`, the unread mentions per thread above a read sequence in one statement, for the
  counters of sync (M1-61, M1-63). Migration `00034` creates `message_templates`; the operations
  `listMyTemplates`, `listTemplates`, `createPersonalTemplate`, `createChannelTemplate`,
  `updateTemplate` (`If-Match` required) and `deleteTemplate` keep personal templates (resource
  `user`, `self`) and channel templates (`read` to list, `manage` to change), bounded exactly by
  the new key `template.max_per_scope`; names are display texts of at most the new key
  `template.name_max_chars`. Templates append no event.
- Reading history (M1-57): `listThreadMessages` pages a thread by `thread_seq` (the newest page,
  `before_seq`, `after_seq` and `around_seq`, at most one of them) and `getMessage` returns one
  message in full with `ETag: "<version>"`. Pages are keyset pages: items ascending and without a
  gap, tombstones at the positions of deleted messages, `has_more_before` and `has_more_after`
  from the same statement as the items. Both operations read with the visible channels of
  `message.read`, check every listed item with `authorize` in filter mode (a mismatch is dropped,
  logged and counted in `bothy.authz.errors` as `filter_mismatch`) and answer a foreign thread or
  message exactly like an unknown one. Lists cut `content_md` above
  `message.list_content_max_bytes` (now an active key) with `content_truncated`; `limit` defaults
  to `api.page_size_default` and is capped at `api.page_size_max`. The port
  `messaging.Messages` adds `History` and `Get` for later callers.
- Editing, deleting and revisions of messages (M1-58): `updateMessage`, `deleteMessage` and
  `listMessageRevisions`. Only the author edits, with `If-Match` (missing `428`, stale `412
  version_conflict` with the current message, deleted `410 message_deleted`); every edit stores
  the previous content as the next revision in the new table `message_revisions` (migration
  `00040`), resolves the mentions again and appends `message.updated`; the same content and
  mentions answer without a write. An edit never triggers an agent and never changes
  `trigger_state`. Deleting is idempotent (`204`): the author with `write`, anyone else with
  `manage` and an audit entry. The tombstone keeps the row and `thread_seq`; content, revisions and
  mentions go, `threads.message_count` drops by one, the payloads of every stored event of the
  message are redacted with the envelope unchanged, the in-transaction event `MessageDeleted` is
  published and `message.deleted` appended, all in one transaction. New kernel port
  `eventlog.Redactor`, implemented by sync with the column right `UPDATE (payload, redacted_at)` on
  `event_log` (migration `00039`); `chat.Threads` adds `LockForMessageChange` and
  `DecrementMessageCount`.

- Sending messages (M1-56): `createMessage` and `createThread`. Migrations `00032` and `00033`
  create `messages` and `mentions` and give `threads.parent_message_id` its deferred foreign key.
  A message is idempotent by its author and `client_msg_id` (a repetition answers `200` with
  `Idempotent-Replay: true`, another body `422 idempotency_key_reuse`, two concurrent identical
  requests make one message), takes the gapless `thread_seq` under the thread lock, gets its
  `origin` from the principal kind, may quote a message of the same thread and resolves mentions
  by the content format of 03 and the rule of 07 section 4.6 (at most `message.max_mentions`
  distinct handles, no collective mentions). A message into a closed thread reopens it and leaves
  a system line `thread.reopened`. Every new message publishes the in-transaction event
  `MessagePosted` and appends `message.created`, whose content is cut with `truncated` above
  `sync.event_payload_max_bytes`. `createThread` creates a forum post or a thread from a message
  of the main thread, with its first message in the same transaction. Attachments, voice, cards,
  components and `run_id` are refused with `not_enabled` until their tickets. New key
  `message.max_mentions`; `identity.Directory` adds the batched handle lookup `ActiveByHandles`.

- Local accounts with password and TOTP (M1-39): the migration
  `00029_identity_local_credentials.sql` (`local_credentials` with the failure window column), the
  operations `setMyLocalCredential` (set the password, start a TOTP enrollment whose otpauth URI is
  shown once, confirm it with a code; step-up only once the password path is established, decided
  in `authz` through `PrincipalSource.HasEstablishedLocalCredential`) and `loginWithPassword`
  (login name, password and TOTP code with a DPoP login proof; a new device is `pending`;
  `404` while `auth.local_password_enabled` is off). Passwords are Argon2id in PHC format
  (`golang.org/x/crypto/argon2`, now a direct dependency) with `auth.argon2id_*`, whose OWASP floors
  stop the start; at least `auth.password_min_length` code points after NFC and not on the embedded
  hashed list of common passwords (SecLists, MIT). TOTP follows RFC 6238 (30 seconds, one step of
  tolerance, every code once); the secret is stored only AES-256-GCM sealed with the key of
  `auth.totp_key_ref`, which `bothy secrets init` now generates as `totp_key`. Failed attempts lock
  only the password path after `auth.login_fail_limit` within `auth.login_fail_window` for
  `auth.login_lock_duration`; unknown names and wrong passwords give the same `401` after exactly
  one Argon2id computation. The credential overview fills `local_password_set` and
  `totp_enrolled`.

- Operators with invariant I1 (M1-54): `setSpaceOperator` and `setChannelOperator` (owner,
  step-up, If-Match, audited) set the default operator of a space (none or an agent) and the
  operator of a channel (inherit, none or an agent). Every change is a rights change: it raises
  `permission_version`, checks I1 in the same transaction (a violation answers
  `409 i1_violation` with hint and `conflicts` and leaves a deny audit entry) and writes
  `access.changed` for the old and the new operator. A new default bumps the version of every
  inheriting channel with its own `channel.updated`. The chat structure source now answers the
  served targets of I1 (including spaces whose default operator is the agent) and the facts of a
  thread, so the operator rule gives the effective operator `write` without a grant. The same
  assignment answers `200` without a change.
- Threads (M1-55): `listChannelThreads`, `getThread`, `setThreadTitle`, `closeThread`,
  `reopenThread` and `setThreadTags`; `createThread` follows with M1-56. Migration `00027` creates
  `thread_tags`. The port `chat.Threads` adds `Create` (inside the caller's unit of work; the
  operator of the channel is fixed on the thread, R-DM-12), `AppendToThread` (the gapless thread
  sequence under the row lock, with an implicit reopen and `thread.reopened`) and `Info`. Titles
  keep their source: a person sets `manual`, a placeholder title stays `auto`, and agents and apps
  get `409 title_manual` on a manual title. Close, reopen, title and tag changes write
  `thread.closed`, `thread.reopened` and `thread.updated` once each, and nothing for a call without
  effect. Restricted tags need `manage`; deleting a forum tag removes its assignments in the same
  transaction. The post list filters by state and tag, sorts by `last_activity` or `created` and
  pages with a cursor bound to caller, channel, filters and sort, without totals. The chat
  structure source now answers threads; a thread created without an operator has none, also for
  `authorize`, after the channel gets one. New keys `thread.max_tags` and
  `thread.placeholder_titles`.

- Channels and forum tags (M1-53): `listSpaceChannels`, `createChannel`, `getChannel`,
  `updateChannel`, `moveChannel`, `archiveChannel`, `listForumTags`, `createForumTag`,
  `updateForumTag`, `moveForumTag` and `deleteForumTag`. Migration `00025` creates `channels`,
  `channel_skill_bindings`, `forum_tags` and `threads` (without the search columns of M3-13 and the
  foreign key to `messages` of M1-56) and the foreign key `grants.target_channel_id`, which `00026`
  validates. A text channel gets its main thread in the same transaction (`thread.created`); a
  channel created in a visible space is visible and listed without a new permission version.
  `archiveChannel` (step-up) is a rights change with `access.changed`; afterwards the channel is
  read-only for everyone, owners included (`403 channel_read_only` with `reason: channel_archived`),
  and manage changes of it are `409 conflict`. Every tag change bumps the channel version and
  writes `channel.updated`. The chat structure source now answers channels (`Channel`, `Channels`,
  `ChannelsOperatedBy`, `ChannelDetails`) for `authorize`. New keys `channel.name_max_chars`,
  `channel.topic_max_chars`, `forum_tag.name_max_chars`, `channel.history_backfill_limit_default`
  and `channel.history_backfill_limit_max`. The decision of reason `channel_archived` now maps to
  the problem code `channel_read_only` (07 section 4.1), for archived spaces too.

- Groups from OIDC claims (M1-42): for a provider with `groups_claim` the callback stores the
  group claim in the login flow (`groups_state` `present`, `absent` or `overage`; another form
  fails the login), and the completion makes the person's memberships in the provider's
  `oidc_claim` groups follow it as the system action `group.member.sync`: new memberships expire
  after `auth.group_claim_max_age` (new key, default `30d`), present ones are extended, missing
  ones removed, each change a rights change with `access.changed`, `group.changed` and one audit
  entry with group ids only; a login without change writes nothing. A missing claim or the Entra
  overage changes nothing and the server never calls Microsoft Graph. A sync that would violate
  I1 fails the whole login with `409 i1_violation` and a deny entry naming provider, groups and
  agents. New store queries `LockClaimMembershipsOfUser`, `ExtendMembership` and
  `ListClaimGroupsByRefs`; the fake provider gets `SetGroups` and `SetGroupsOverage`.

- Who has access, grant preview and view as (M1-49): `explainSpaceAccess` and
  `explainChannelAccess` (`access.explain`, owner or `manage`) list every principal with its
  effective level, capabilities, `inactive` mark and sources (`owner`, `direct`, `group`,
  `inherited`, `operator`, `dm_participant`, `via_agent` for hint H1, `guard_blocked` for SR2),
  computed with the snapshot code of `authorize`. `previewGrant` (`access.preview`, no step-up, no
  audit) runs the apply step of `createGrant` in the real transaction and rolls it back, reports
  the effects per principal, I1 and SR8 violations with `organization_ready`, the hints
  `read_horizon`, `trigger_blocked_tool_class`, `group_reach` and `expiry_missing`, delegation
  limits as `allowed: false` with `denial_reason`, and a plain text from German and English
  templates in the caller's language and time zone. `viewAsUser` (`access.view_as`, owner,
  step-up, audit) returns the spaces and channels a person sees with the intersected rights,
  without direct messages; `actor.ViewAs` carries the start of the view and every decision is
  `step_up_required` once `authz.view_as_max_duration` (new key, default `15m`) has passed. The
  source ports gain `PrincipalSource.Profiles` and `OwnerIDs`, `StructureSource.SpaceDetails` and
  `ChannelDetails` and `AgentPolicy.BillingModesSupported`.

- Discord read client of the import tool (M8-01): `cmd/bothy/importtool/discord` reads a Discord
  server through the `GET` calls of chapter 22 section 4.1 only (user, guild, channels, active
  and archived threads with the page loop by `archive_timestamp`, one channel, messages after,
  before and around an ID in ascending order). The bot token prints as `[redacted]` everywhere and
  goes only to the host of `import.discord.api_base` over `https`, redirects are never followed,
  and every connection goes through the egress client. The rate-limit guard waits
  `X-RateLimit-Reset-After` and `retry_after` or `Retry-After`, retries `429` at most
  `import.discord.max_429_retries` times, aborts on a wait above `import.discord.max_rate_wait`,
  never retries `401` or `403` and ends a channel after its `403` for the run. The test fake
  `discordtest` (TLS on `127.0.0.1`, fixture guild, fault injection, token echo, request log)
  is for later import tickets and never reaches the binary.

- Spaces and the sidebar (M1-52): the tables `spaces` and `user_space_states` (migrations
  `00023_chat_spaces.sql` and `00024_grants_space_fk_validate.sql`, which also give
  `grants.target_space_id` its foreign key) and the operations `listSpaces`, `createSpace`,
  `getSpace`, `updateSpace` (If-Match), `moveSpace`, `archiveSpace` and `putMySidebarSpace`.
  The global order is a fractional index (`chat/domain/fracindex`), so a move writes one row;
  create and move serialise on an order lock. Colour and icon accept only keys of the design
  system, names are trimmed, limited by the new key `space.name_max_chars`, free of control and
  bidi characters and unique case-insensitively among spaces that are not archived. Archiving is
  final and a rights change: every level in the space is capped at `read` (owners included),
  `access.changed` reports the space at level `read` to everyone above it, `SpaceArchived` is
  published, the space leaves `listSpaces` and refuses management changes with `409`. The
  sidebar row of a person (collapsed, own order) is last write wins with `sidebar.updated` to
  the person. `chat` is now the space part of the structure source of authz, and `SpaceView`
  reports the caller's effective level and capabilities.

- Host pairing (M2-41): `issueHostPairingCode` (re-pairing from `pairing`, `confirming`, `active`
  or `disabled`: drops the stored keys at once, ends the sessions of an active host with
  `host_rekeyed`, clears a disabled reason, `host.updated` with `key_changed`), the public
  `pairHost` (redeems a pairing code of kind `hpc` once and before `terminal.pairing_code_ttl`
  ends; every failing code is `401 unauthenticated` with one body, a wrong form or checksum without
  any database access; stores both public keys, the fingerprint of 24 4.3, the next
  `key_generation` and the report with `reported_session_users`; refuses a login key of another
  host and the dropped keys of the same host) and `confirmHost` (step-up, typed prefix of
  `terminal.fingerprint_confirm_chars` characters, `409 fingerprint_mismatch` with
  `attempts_remaining`, fallback to `pairing` after `terminal.fingerprint_max_attempts` failures,
  then session users only from the reported names, the LLM login with I6 and the server host in
  one transaction). `host.pair` is a `Public` requirement. `identity.Tokens.HashHostPairingCode`,
  `idempotency.KeepEffectOnFinal` (a marked final error commits under an `Idempotency-Key`),
  `errs.FingerprintMismatch` and the new keys `terminal.fingerprint_confirm_chars`,
  `terminal.fingerprint_max_attempts`, `terminal.report_label_max_chars` and
  `terminal.report_max_entries` with the start check `fingerprint_keys`.
- Terminal rights (M2-40): `authorize` decides `terminal.open`, `terminal.attach`,
  `terminal.stream`, `terminal.read_own` and `terminal.close` behind the gate of a person in a
  session (agents, apps and personal tokens get `403 forbidden` with `subject_kind_not_allowed`).
  Fresh step-up needs a passkey or device key confirmation not older than
  `terminal.step_up_max_age` (new key, default `60s`) that no opening consumed, never in the web
  console; `terminal.attach` of a detached session from the same device and login session within
  `terminal.quick_reattach_window` (new key, default `60s`) needs none (`Decision.QuickReattach`).
  The guards SR10 (server host, assignee), SR11 (I6, also against other owners) and SR12 (an own
  session user the host reports, also for owners) map to `403 terminal_denied` with their
  `reason`. `identity.StepUp.ConsumeFresh` consumes the step-up under the lock of the session
  row, so one confirmation opens exactly one session. I6 is verified in every rights change
  (`409 i6_violation` with `hint` and `conflicts` per host, deny entry with `host_id`).
  `setHostLlmCredentials` declares the LLM login of a host (switching to `personal` or changing
  the holder ends the sessions of other people with `i6_violation`); `updateHost` ends the
  sessions of an old assignee who is not an owner; `RecordSessionUserReport` and
  `ApplySessionUserReport` make the reported session users rights state for M2-41 and M2-43. The
  preview of a grant on a host reports host effects, I6 violations and the hint
  `terminal_not_effective` with German and English texts. New reason code
  `terminal_no_session_user`.
- Host records of the terminal module (M2-39): the migration `00028_terminal_hosts.sql` with
  `hosts`, `host_session_users` and `terminal_sessions` (the composite key of a session to the
  session user of its host and person) and the foreign key `grants.target_host_id`; the operations
  `listHosts`, `createHost` (pairing code `bothy_hpc_...` shown once, only its HMAC stored),
  `getHost`, `updateHost` (name, holder, session users after ruling T12) and `setHostState`
  (disable, enable, retire after the life cycle of 24 section 3.1). Every change is a rights change
  with step-up, a mandatory reason, an audit entry and the event `host.updated` without keys;
  leaving `active` and ending an assignment end the sessions in the same transaction, audited as
  `terminal.terminate`. In `authz` the host is a target outside the tree: `HostSource`, the
  snapshot map of host grants, `TerminalOn`, the atoms `HumanSession`, `Assignee` and `Terminal`,
  the gate that refuses agents, apps and personal tokens with `subject_kind_not_allowed`, host
  entries in `access.changed` and `Scope(host.read)`. Identity issues the token kind `hpc`, the
  signing-key purpose `terminal` gets an active key at every start, the audit details know the
  reserved member `os_user`. New keys `host.name_max_chars`, `host.os_user_max_chars`,
  `terminal.pairing_code_ttl`, `terminal.session_token_ttl` and `audit.reason_max_chars`. An owner
  reads host grants with `getGrant` and `listGrants?target_kind=host`; every list cursor whose key
  carries a NUL character answers `400 cursor_invalid` instead of `500`.

- Device approval and pairing (M1-38): a device that logs in without an enrollment code stays
  `pending` with tokens but no access beyond reading itself and shows a verification code
  derived from the token pepper, its id and its DPoP key (never stored). `approveDevice` (owner or
  the person from an active device, step-up) activates it with the typed code; a wrong code
  revokes it with its keys and sessions and answers `200` with `state: revoked`. Pairing:
  `beginPairing` registers the keys of a new device and shows a pairing code of
  `auth.pairing_code_chars` Crockford characters, `approvePairing` creates the device as an
  active device of the caller (an unknown code ends the step-up of the session) and
  `completePairing` hands out the tokens under a proof of the registered key (another key rejects
  the pairing). Pending devices past `auth.device_pending_ttl` count as revoked at once and are
  revoked by `identity.auth_cleanup` (`pending_expired`), like approved pairings never collected
  (`pairing_expired`); both are audited as the system action `device.expire`. New migration
  `00018_identity_device_pairings.sql` and the keys `auth.device_pending_ttl`,
  `auth.device_verification_code_digits`, `auth.pairing_code_ttl` and `auth.pairing_code_chars`.
- Passkeys (M1-40): the migration `00019_identity_passkey_credentials.sql` and the operations
  `beginPasskeyLogin`, `completePasskeyLogin` (login of local accounts, new devices `pending`, a
  decoy credential ID for unknown names), `beginEnrollment`, `completeEnrollment` (the first
  passkey with an enrollment code, device `active`), `beginPasskeyRegistration`,
  `completePasskeyRegistration` (step-up once an active passkey exists, decided in `authz`),
  `revokePasskey` and `completeStepUpWithPasskey`; `createStepUpChallenge` adds `passkey_options`.
  The state of every ceremony is a single-use row of `auth_challenges`, so begin and complete may
  run on different nodes; user verification is required, attestation is not requested, only
  ES256, EdDSA and RS256 are accepted, and a falling signature counter is audited. New keys
  `auth.login_flow_ttl`, `auth.webauthn_rp_id` and `auth.webauthn_origins` with the cross-check
  `webauthn_relying_party`; the credential overview lists the passkeys. Dependency:
  `github.com/go-webauthn/webauthn` `v0.18.2` (BSD-3-Clause), only in `identity/adapters/webauthn`,
  with the software authenticator `virtualauthn` for tests.
- OIDC login with PKCE (M1-41): the server is the confidential relying party of every configured
  OIDC provider and the authorization server of its apps, with PKCE `S256` on both legs. The
  operations `beginOidcLogin`, `handleOidcCallback`, `completeOidcLogin`, `beginIdentityLink` and
  `completeIdentityLink`; the migration `00020_identity_login_flows.sql`; the callback stores only
  the verified identity with a one-time `login_code`, and mapping by `(provider_id, issuer,
  subject)`, linking through an enrollment code and provisioning as `pending` happen at complete,
  which needs the app's `client_verifier`. Discovery, JWKS and token requests run through the
  egress client with a per-provider host policy; client authentication with `client_secret_basic`
  or `private_key_jwt`; ID tokens with `RS256`, `PS256`, `ES256` or `EdDSA` (`kernel/jose` gains
  RSA verification and signing). OIDC sessions need a new provider login after
  `auth.idp_recheck_interval`; the job `identity.auth_cleanup` deletes expired flows. New
  configuration keys `auth.server_code_ttl`, `auth.oidc_jwks_cache_ttl`,
  `auth.oidc_client_assertion_ttl` and `auth.idp_recheck_interval`; `auth.login_flow_ttl` (M1-40)
  also bounds an OIDC login flow; operations guide `docs/operations/oidc-entra.md` with a
  Microsoft Entra ID example.
- Login with the identity header of a trusted reverse proxy (M1-43): the operation
  `loginWithTrustedHeader` (`POST /api/v1/auth/trusted-header/login`) maps the decoded login (RFC
  2047, `utf-8` and `us-ascii` only) of a `trusted_header` provider to a known identity, links it
  through an `enrollment_code` or provisions a pending person, and completes the login with device
  keys and DPoP tokens like every driver. The header counts only from a TCP peer in
  `server.trusted_proxies` and only while the public listener is bound to loopback; otherwise the
  login answers `404`, `/readyz` fails with `trusted_header_requires_loopback`, and enabling the
  provider answers `409 conflict` with that reason (PLAN W4). The new key
  `auth.trusted_header_bind_requests` (default `true`) binds every later request and refresh of
  such a session to the same login; a mismatch revokes the session (`trusted_header_mismatch`).
  `kernel/reqmeta.Transport` carries the TCP peer, the header and the route's action from
  `platform/httpserver`; the audit reason `trusted_header_requires_loopback` joins 07 section 4.1.
- Feature flags (M1-47, ADR 0064): the registry of `dm.enabled`, `tokens.personal_enabled` and
  `voice.enabled` (options, off) in `internal/instance`, the migration `00021_feature_flags.sql`,
  the evaluation per principal with rollout lists of people and groups (valid memberships only;
  unknown and orphaned keys are off; agents and apps get the global value), the operations
  `listFeatureFlags` and `putFeatureFlag` (owner, step-up, reason, audit, optional `If-Match`,
  `instance.updated` with the changed key), the evaluated flags in `getInstance`, and the gate
  `make flag-check` (`tools/flagcheck`), which fails once `VERSION` reaches `remove_by` of a
  registered `release` flag or configuration.md 7 drifts from the registry.

- Instance settings, branding and discovery (M1-46): the migration `00017_instance_settings.sql`,
  the settings registry of configuration.md section 6, the generated design keys
  `kernel/designkeys` (category colours and space icons from the design tokens, the accent list of
  ADR 0044) with the generator `tools/designkeysgen`, and the operations `getDiscovery` (public,
  also before setup), `getInstance`, `listInstanceSettings` (owner), `putInstanceSetting` and
  `updateBranding` (owner with step-up, audit, `If-Match`, `instance.updated` with the changed
  keys) and `getProblemType`. The route generator exposes `SpecVersion`,
  `GatewayProtocolVersions` and the problem catalogue; the configuration gains the client limits
  and the client version keys; `createUser` stores `locale.default` when the request names no
  locale; `tools/ci/tests/test_product_name_places.py` holds the places of the working name.
- Audit checkpoints in object storage (M7-22): `audit.checkpoint_sink` accepts
  `s3://<bucket>[/<prefix>]` next to `file://`. The new sink `internal/audit/adapters/s3sink`
  stores every signed checkpoint as the object `[<prefix>/]<instance_id>/<seq>.jws`, written once
  (`If-None-Match: *` after a read of the key; other bytes under a stored `seq` fail with the
  checkpoint conflict and the stored object stays), lists them in `seq` order from the oldest
  version of every key (each version read once per process, new ones in parallel) and pages
  through `ListObjectVersions`. New keys
  `audit.checkpoint_s3_endpoint`, `audit.checkpoint_s3_region`,
  `audit.checkpoint_s3_use_path_style`, `audit.checkpoint_s3_access_key_ref` and
  `audit.checkpoint_s3_secret_key_ref` (own endpoint and credentials, through the egress client
  with purpose `audit_checkpoint`), and `retention.audit` enters the schema. `bothy config check`
  and the server start read the Object Lock configuration of the bucket and fail unless it has a
  default retention in mode `COMPLIANCE` of at least `retention.audit`. A conformance suite
  (`internal/audit/app/sinktest`) runs against the file sink and the object store sink; the
  secret catalogue gains the supplied files `audit_checkpoint_s3_access_key` and
  `audit_checkpoint_s3_secret_key` (server only).

- First setup, enrollment codes and setup lock (M1-37): until `bothy setup bootstrap-owner` has
  run on the host, the public listener answers every request except the discovery reads with
  `503 setup_required`, before authentication and rate limiting; the bootstrap takes effect
  without a restart. `bootstrap-owner` creates the first owner, the local identity provider, the
  local identity and a one-time enrollment code in one transaction with the audit entry
  `instance.bootstrap`; `bothy setup issue-enrollment-code` issues a recovery code for an active
  owner (`owner.recover`). Codes have 160 random bits, are shown once (`en_` plus eight groups of
  four Crockford Base32 characters), stored only as HMAC with the token pepper, valid
  `auth.enrollment_code_ttl`, single use and invalid after `auth.enrollment_code_max_attempts`
  failed attempts, which survive the rollback. New operations `redeemEnrollmentCode` (device keys
  only, the device becomes active, every rejected code the same `401`) and
  `createEnrollmentCode`; `createUser` with `local_account` creates the local identity and a code;
  responses with a code are never stored for an `Idempotency-Key` replay (`409 conflict` (reason `one_time_secret`)).
  Migration `00016_identity_enrollment_codes.sql`. The shared login completion of every later login
  path creates devices, sessions and tokens in one place.
- Groups and grants over the API (M1-48): the operations `listGroups`, `createGroup`,
  `getGroup`, `updateGroup`, `deleteGroup`, `listGroupMembers`, `addGroupMember`,
  `removeGroupMember`, `listGrants`, `createGrant`, `getGrant` and `revokeGrant` in
  `internal/authz/adapters/httpapi`, the read use cases of the port `authz.Admin`, the list
  queries of `adapters/postgres` and `Authorizer.AdminScope`. Group writes are owner only with
  step-up; `grant.create` and `grant.revoke` need owner or `manage` within the delegation limits
  (agents and apps only by owners, never `trigger` or a capability beyond `upload`), checked
  before step-up. Non-owners read only grants inside their `manage` scope and only groups they are
  a member of; everything else answers like a random id. Every write appends `grant.changed` or
  `group.changed` and an audit entry; rights changes raise `permission_version`, check I1 and SR8
  and write `access.changed` in the same transaction. Claim groups (`source = oidc_claim`) can be
  created with `provider_id` and `external_ref`; their members are not changed by hand (`409
  conflict`, reason `claim_managed_group`). New configuration keys `group.name_max_chars` and
  `group.description_max_chars`; `authz.PrincipalSource.Refs` gives the public views of members.

- Agent records, backend policy and credential holder (M1-51, ADR 0024): the migration
  `00022_agents.sql` (`agents` as chapter 03 states it, with a trigger that requires a principal
  of kind `agent`), the port `agents.Agents` with the in-transaction event `AgentDisabled`, the
  identity port `identity.PrincipalWriter` (create an agent principal, set its status and display
  name), and the operations `listAgents`, `createAgent`, `getAgent`, `updateAgent`,
  `disableAgent`, `enableAgent`, `setAgentBillingMode`, `setAllAgentsBillingMode`,
  `setCredentialHolder` and `setAgentToolClass`. Every mutation is owner only with step-up and
  audit; policy, holder, tool class, creation, disabling and enabling are rights changes, so I1 is
  checked in the same transaction (`409 i1_violation`, all or nothing for the switch of several
  agents). Agents read themselves, readers of a served channel see the agent, `agent.updated`
  carries the badge after every change, disabling revokes every API token of the agent, and
  `operator_config` follows the strict schema `agent_operator_config.v1` (no secrets). The agents
  module is now the `authz.AgentPolicySource` of the Authorizer and the rights hooks; I1 counts
  active personal agents only.
- Identity provider port and provider management (M1-36): the migration
  `00013_identity_providers.sql` (`identity_providers`, `user_identities`, and the deferred foreign
  keys `auth_sessions.identity_id` and `groups.provider_id`), the ports `identity.Providers` and
  `identity.Identities` with `ExternalIdentity`, `Resolution`, `Identity` and
  `CredentialOverview`, and the operations `listIdentityProviders`, `createIdentityProvider`,
  `getIdentityProvider`, `updateIdentityProvider`, `deleteIdentityProvider` (owner only; manage
  with step-up and audit), `getMyCredentials`, `getUserCredentials` (`identity.read`, self or
  owner) and `unlinkIdentity` (`identity.unlink`, self or owner, step-up). Configurations follow
  the strict schemas `identity_provider.oidc.v1`, `identity_provider.trusted_header.v1` and
  `identity_provider.local.v1` (unknown members, duplicate keys and null refused; secrets only as
  `file:` or `env:` references, checked for syntax and never resolved). At most one
  `trusted_header` and one `local` provider, names unique without regard to case. An external
  identity maps to a person only through provider, subject and, for OIDC, issuer, never through
  e-mail (ID-T04). Unlinking refuses to remove the last sign-in method (`409 conflict`
  (reason `no_login_method_left`), also under concurrency); unlinking an identity and disabling a provider
  revoke the sessions started through them. A provider that identities or claim groups reference
  cannot be deleted (`409`, disable it instead).
- API tokens with prefix and checksum (M1-44, ADR 0061): the format
  `bothy_<kind>_<random>_<checksum>` (`domain/apitoken`, kinds `agt`, `app`, `pat`, `whi`, 256
  random bits in Base62, CRC-32 checksum in 6 Base62 digits), the migration
  `00014_identity_api_tokens.sql` (`api_tokens`, only the HMAC and `token_prefix` stored), the
  ports `identity.Tokens` (`Issue`, `List`, `Revoke`, `RevokeAllForPrincipal`) and
  `identity.TokenAdmin`, and the operations `listTokens`, `createToken` and `revokeToken`. The
  secret is shown once (`Cache-Control: no-store`; a replay with the same `Idempotency-Key` is
  `409 conflict one_time_secret`), the expiry is mandatory and bounded, agent tokens are
  DPoP-bound by default and refused as `Bearer`, personal tokens exist only for oneself and only
  with the switch `tokens.personal_enabled` (off until M1-47, `403 feature_disabled`), have no
  owner rights and never provide step-up; app tokens answer `409 conflict app_not_installed`
  until M2-24. The Authenticator takes the API token path for values starting with `bothy_`
  (format and checksum without a query, one lookup by HMAC, then the DPoP binding;
  `AuthenticateAPIToken` for the gateway of M1-64); revocation is a rights change and holds at
  the next request, and the Idempotency-Key stage never replays a stored answer to a revoked or
  expired token (`actor.Actor.CredentialInactive`); such a token counts against the client IP
  in the rate limiter, never against its principal's buckets. The authz guard of `token.create` refuses a personal token for another person
  (`not_self`); `CredentialSource` reports the token facts. New configuration keys
  `auth.api_token_max_ttl`, `auth.personal_token_max_ttl` and `auth.dpop_required_for` with the
  cross-check `dpop_required_for`; the gitleaks test builds a runtime token and recomputes the Go
  checksum vectors.
- Audit verification, checkpoints and queries (M1-50): the migration
  `00015_audit_checkpoints.sql` (`audit_checkpoints`, append-only with a once-only `exported_at`),
  the port `audit.Log` with `List`, `Export` and `Verify`, and the operations `listAuditEvents`,
  `exportAuditEvents` and `verifyAuditChain` (actions `audit.read` and `audit.verify`, owner with
  fresh step-up, every call audited). The periodic job `audit.chain_verify` on the queue
  `maintenance` recomputes the chain since the last checkpoint, signs the next checkpoint (compact
  JWS, purpose `audit_checkpoint`, payload linked to the previous one) only over a verified segment
  and hands it to `audit.checkpoint_sink`; a break counts in `bothy.audit.verify.failures`, is
  audited with decision `error` and is never signed, a success sets
  `bothy.audit.verify.last_success.timestamp`. Verification reads checkpoints only from the sink,
  so a changed row (`hash_mismatch`) and a rewritten chain (`checkpoint_mismatch`) are detected
  (SEC-T16); without a sink every result and every job run carries the warning
  `checkpoint_sink_not_configured`. The export streams NDJSON lines that verify offline with RFC
  8785 and SHA-256, in the route class of downloads (`http.transfer_timeout`). New: the file sink
  `file://<absolute path>`, `instance.Keys.Verify`, the route table field `Download`, and the
  configuration keys `audit.checkpoint_interval` and `audit.checkpoint_sink`, whose value `bothy
  config check` refuses unless it is empty or `file://`.
- Step-up with the device key (M1-35): the migration `00012_identity_step_up.sql`
  (`auth_challenges` with every purpose of 03), the port `identity.StepUp` with
  `CreateChallenge` and `CompleteWithDeviceKey`, and the operations `createStepUpChallenge` and
  `completeStepUpWithDeviceKey` (action `auth.step_up`, no `Idempotency-Key`). A challenge is 256
  random bits stored as HMAC, single use and valid `auth.step_up_challenge_ttl`; the confirmation
  is a compact JWS by the active `step_up` key of the session's device over `{challenge, sid,
  iat}` and sets `step_up_at` and `step_up_method = device_key` of that session only. A wrong
  confirmation is `401 step_up_required` and counts; `auth.step_up_fail_limit` failures within
  `auth.login_fail_window` revoke the session (`step_up_failed`, `session.revoked`, `401
  session_revoked`); a caller without a session, a `cli` device and a device without `step_up`
  key get `403 forbidden` (`step_up_not_possible`). The periodic job `identity.auth_cleanup` on
  the queue `maintenance` deletes expired challenges and keeps failed step-ups while they count.
  New configuration keys `auth.step_up_challenge_ttl`, `auth.step_up_fail_limit`,
  `auth.login_fail_window` and `auth.cleanup_interval`.
- Sessions, access tokens, refresh families and logout (M1-34): the migration
  `00011_identity_sessions.sql` (`auth_sessions`, `refresh_token_families`, `refresh_tokens`), the
  port `identity.Auth` with `StartSession`, `StartChildSession`, `CreateExtensionSession`,
  `Refresh`, `Logout`, `RevokeSession` and `ListSessions`, the in-transaction event
  `SessionEnded` and the durable event `session.revoked`. Access tokens are JWS with header `typ`
  `at+jwt`, signed with the `access_token` keys of instance, bound to the DPoP key (`cnf.jkt`) and
  capped at the session end; refresh tokens are 256 random bits stored as HMAC, rotated per
  family with the grace `auth.refresh_reuse_grace`, and a reuse revokes family, session and child
  sessions. identity's `Authenticator` now authenticates every person on the REST chain (token,
  DPoP proof, then session, device and key in one query); `authorize` decides session and device
  state from what it read (`authorize.CredentialObserver`, a revoked or replaced device key is
  `device_inactive`). The operations `refreshToken`, `logout`, `listMySessions`,
  `listUserSessions` and `createExtensionSession`; the configuration keys `auth.session_max_age`,
  `auth.refresh_token_ttl`, `auth.refresh_reuse_grace` and `identity.last_active_granularity`; the
  start step `sessions` after `signing_keys`. A missing required `DPoP` header is `401
  invalid_dpop_proof` instead of `400`; `kernel/reqmeta` carries the DPoP facts of a request and
  a setter of response headers; `kernel/jose.SignerFunc`; depguard bans `math/rand` in identity.
- `internal/authz` (M1-30): the periodic jobs `authz.expiry_sweep` and `authz.expiry_reminder`
  on the queue `maintenance`. The sweep reports every expired grant and membership exactly once
  in batches of `authz.expiry_sweep_batch_max`: raised rights version, `expiry_reported_at`,
  `grant.changed` (`expired`) or `group.changed` (`member_expired`), the system audit actions
  `grant.expire` and `group.member.expire`, I1 and SR8, and `access.changed` for every principal
  whose visible targets changed; a run without due rows leaves the rights version alone. The
  reminder writes `access.expiring` once per active recipient within `authz.expiry_reminder_lead`
  before an expiry. Every rights change first folds in the due expiries (marker, events, audit), so a loss
  is reported exactly once and no gain is reported that `authorize` denies. New: `ExpiryService`,
  `ExpiryStore`, `RightsHook.Folded`,
  `domain.AssumeValid`, `adapters/jobs`, the configuration keys `authz.expiry_sweep_interval`,
  `authz.expiry_reminder_lead` and `authz.expiry_sweep_batch_max`, and the registration of
  `grant.changed`, `group.changed` and `access.expiring`.
- People, owners and own profile (M1-31): the operations `listUsers`, `createUser`, `getUser`,
  `updateUser`, `enableUser`, `listOwners`, `addOwner`, `revokeOwner`, `getMe` and
  `updateMyProfile` with the ports `identity.People` and `identity.Owners`, the durable event
  `user.updated`, audit entries with IDs and field names only, and the rights changes
  `owner.add`, `owner.remove` and `user.enable` through the authz hook (`409 i1_violation`, `409
  last_owner`). No self registration; `local_account` answers `400` until enrollment codes exist.
  `kernel/txn.AllowLateRightsChange` lets the unit of the Idempotency-Key stage become a rights
  change in the use case's nested unit; `httpservertest` gains `Actors`, `InSession`,
  `WithToken` and the response validator `ResponseValidator` (one per contract and
  operation list, `SharedResponseValidator`). Time zones are checked against the IANA names of the toolchain's
  tzdata (`tools/zonegen`). The device reads `getDevice` and `listUserDevices` authorize before they
  answer an unknown device or person.
- Server signing keys (M1-32): the port `instance.Keys` (`Signer`, `VerificationKey`, `PublicKey`,
  `JWKS`, `Rotate`, `EnsureActive`) over the table `server_signing_keys` (migration `00010`) and
  the key store `secrets.signing_key_dir` (Ed25519, PKCS#8 PEM `<purpose>-<kid>.pem`, `kid` after
  RFC 7638). States `next`, `active`, `retiring` and `retired` with scheduled rotation of
  `invocation` and `access_token`, owner rotation `scheduled` or `emergency`, the job
  `instance.key_rotation` and one audit entry per transition. The operations `getJwks` and
  `rotateSigningKey`, the start step `signing_keys` with the readiness check of the same name,
  the start check of the key store in every role, and the configuration keys
  `secrets.signing_key_dir`, `signing_key.cache_ttl`, `signing_key.rotation_check_interval`,
  `signing_key.access_token_rotation_interval`, `signing_key.invocation_rotation_interval`,
  `agent.jwks_max_age`, `agent.invocation_token_ttl`, `auth.access_token_ttl` and
  `webhook.delivery_timeout` with the cross-check of `agent.jwks_max_age` against both rotation
  intervals.
- Secret provider (M1-32): `Provider.Put` and `Provider.Delete`, writable only directly in
  `secrets.signing_key_dir` (`ErrReadOnly`, `ErrExists`, `ErrWriteFailed`), and
  `CheckSigningKeyDir`. `bothy secrets export` adds the key store under `signing-keys/`
  (`--signing-key-dir`); `bothy secrets init` skips that part of a decrypted bundle.

- Idempotency keys (M1-20): `StageIdempotency` of `platform/httpserver` runs every keyed request
  in one unit of work with the key row of `idempotency_keys` (migration `00007`), stores final
  responses only (`2xx`; `400`, `403`, `404`, `409` except `request_in_progress`, `410`, `412`,
  `413`, `415`, `422`), replays them with `Idempotent-Replay: true` and `ETag` from `version`,
  answers `422 idempotency_key_reuse` for another request under the same key and `409
  request_in_progress` after `database.lock_timeout`, and never stores a one-time secret
  (`idempotency.MarkOneTimeSecret`, replay `409` with `reason: one_time_secret`). The port
  `kernel/idempotency`, the store and the job `sync.idempotency_cleanup` in `internal/sync`,
  `platform/jsoncanon` (RFC 8785 over `github.com/gowebpki/jcs`), generator checks for keyed
  operations in `api/routes/gen`, and the configuration keys `sync.idempotency_key_ttl`,
  `sync.idempotency_cleanup_interval` and `sync.idempotency_cleanup_batch_size`.
- `internal/authz` (M1-29): the one transactional path of every rights change. The rights hooks
  of phase 1 and 3.1 lock and raise `permission_version` first, record the visible targets of
  the affected principals (declared directly or derived from targets), verify SR8 (`409
  last_owner`) and I1 for every active personal agent (`409 i1_violation` with `conflicts`) in
  the same transaction, and write one `access.changed` per principal with a difference and the
  in-transaction event `authorize.AccessChanged`; a violation leaves only a deny audit entry. The
  `Admin` port with group, membership and grant use cases (authorize first, step-up, delegation
  limits, allow audit, calls without effect). New source methods `ServedTargets`,
  `SpaceArchived`, `PersonalAgents`, `ChannelFacts.Archived` with the archived cap at read.
- `internal/kernel/txn`: `DeclareRightsChange` with action, resource and targets,
  `RightsChangeOf`, `InRightsChange`; `internal/kernel/authorize`: `AccessChanged`;
  `internal/kernel/errs`: `ProblemExtender`, merged into the problem body by
  `internal/platform/httpserver`.
- `internal/identity` (M1-33): devices with their two public keys (`dpop`, `step_up`) in the new
  tables `devices` and `device_keys` (migration 00008), the port `Devices` (`Register`, `Get`,
  `ListMine`, `ListForUser`, `ActiveKey`) with `device.list` through `authorize`, and the port
  `DPoP`: the one DPoP verifier of RFC 9449 with stateless server nonces derived from the token
  pepper, the `htu` normalization and a per-process `jti` store with a sweeper. The REST
  operations `listMyDevices`, `listUserDevices` and `getDevice` are enabled. The device part of
  `authz.CredentialSource` replaces the empty credential source; test helpers in
  `adapters/identitytest`.
- `internal/kernel/jose` (M1-33, ADR 0067): compact JWS with `ES256` and `EdDSA`, public JWKs of
  EC P-256 and OKP Ed25519 with RFC 7638 thumbprints, signing through a `Signer`; standard
  library only, with fuzz targets and seed corpus.
- Configuration keys `auth.dpop_nonce_ttl`, `auth.dpop_proof_max_age`, `auth.clock_leeway`,
  `auth.token_pepper_ref`, `api.page_size_default` and `api.page_size_max`; `bothy serve` resolves
  the token pepper at its start (step 6d).

- `internal/sync` (M2-01): the frame of the worker consumers of the event log (ADR 0051). The
  table `event_consumer_cursors` (migration `00009`), the port `Consumers` with `ConsumerName`,
  `ConsumerHandler` and `ConsumerState`, `domain.PlanRound` with `ErrConsumerGap`, and in `app`
  the `ConsumerService` (one round per unit of work: `FOR UPDATE SKIP LOCKED` on the cursor row,
  up to `consumer.batch_size` events, jobs through the handler, cursor and jobs in one commit) and
  the `ConsumerRunner` (one goroutine per consumer, one `LISTEN` connection per process with the
  application name `bothy-consumer-wake`, a poll every `consumer.poll_interval`, drain after a full
  batch). Events lost to retention are skipped and logged, other holes and handler failures roll
  the round back. The roles `worker` and `all` start the runner after River with the stop step
  `event_consumers`; every role exports `bothy.event.consumer.lag` and
  `bothy.event.consumer.pending_age`. No production consumer is registered yet (M2-26, M3-19).
- Configuration keys `consumer.batch_size`, `consumer.poll_interval` and
  `consumer.pending_alert_after`, with the start check `consumer_alert_below_retention`; the
  alarm `BothyEventConsumerLag`, the attribute `bothy.consumer.name` and the panel "Worker
  consumers" of the dashboard `delivery-sync`.
- `internal/authz` (M1-28): the one authorization function. `app.Authorizer` implements
  `kernel/authorize.Authorizer` with `Authorize` (07 section 4.2: liveness, pending, resolution
  through the structure, effective rights with owner, dm, operator, agent and app limits, "view
  as" and acting-for intersections, visibility per resource kind, the requirement table of every
  catalogue action, app scopes, read-only channels, step-up and the guards of `agent.invoke`,
  `grant.create` and `grant.revoke`) and `Scope` (the visible channels and spaces per request).
  Permission snapshots are cached per principal and permission version in an LRU, valid until the
  earliest expiry; denials go to the audit recorder, filter mode writes none. New source ports
  `StructureSource`, `AgentPolicySource` and `CredentialSource`, in-memory fakes in
  `adapters/fake`, and the start check rejects routes, ops and event types that name a deferred
  action.
- `internal/kernel/authorize`: `Decision` with its error mapping, `Resource`, `Principal`,
  `Scope`, `Authorizer`, `WithPermissionVersion`, `WithRequestScope` and `Filter`;
  `kernel/actor.Actor` carries the installation, granted scopes, "view as" and acting for and
  implements `authorize.Principal`.
- `internal/instance`: the read-only port `Versions` with the permission version.
- `internal/platform/httpserver`: one request memo of `kernel/authorize` per request.
- `internal/wire`: the authorizer over the identity principal source, the grant store, the
  permission version and the audit recorder, with empty sources until chat, agents and identity
  provide theirs; configuration keys `authz.snapshot_cache_max_entries` and
  `auth.step_up_max_age`.
- Go module `github.com/OWNER/bothy/services/server` with `VERSION` and the golangci-lint v2
  configuration of the module.
- Binary `bothy` with a command table, usage text and the exit codes 0 (success), 1 (failure) and
  2 (usage error); later tickets register the commands.
- `internal/kernel/ids`: UUIDv7 after [RFC 9562](https://www.rfc-editor.org/rfc/rfc9562.html), monotonic within a process, generated by the
  application; parsing accepts upper case and rejects every other version.
- `internal/kernel/clock`: system clock in UTC and a fake clock for tests.
- `internal/kernel/errs`: error kinds with a fixed HTTP status, the closed set of public problem
  codes of `docs/spec/06-api.md` and internal causes that never reach a response; `PublicCode`
  gives the code a response may carry, and the error text and JSON encoding of an error carry the
  same code.
- Port packages of all modules of `docs/spec/02-architecture.md`, the remaining kernel packages and
  `internal/wire`, each with its package documentation.
- Module register `architecture/modules.yaml` and the generator `tools/archgen`, which writes
  `.go-arch-lint.yml` from it; `make arch` enforces the layers and the allowed module
  dependencies. Every component may use its own packages, vendor patterns are Go import paths,
  `exclude` entries may not hit product code or other directories, the register rejects plain
  words and numbers that YAML 1.1 readers such as PyYAML read differently from yaml.v3, and
  `make arch` rejects symbolic links below the module except editor lock files.
- `internal/platform/config`: configuration schema with kind, default, documentation and owning
  chapter per key; loader with the precedence default < file < environment < `--role`, which
  rejects unknown `BOTHY_` variables and file keys, type, range and enum errors, missing required
  keys and failed cross-checks, and reports every problem with key and source but never a value;
  typed access to the values and `Bind` into the `Config` struct of a consumer.
- Configuration schema of the server (`internal/wire`): `server.role`, `server.listen_addr`,
  `server.internal_listen_addr`, `http.max_json_body_bytes` and `message.max_content_bytes`, with
  the cross-checks `listeners_distinct` and `json_limit_above_message`.
- `bothy config check` and `bothy config print [--redacted]`: verify the configuration without
  starting the server (exit codes 0, 1, 2) and show the effective value and source of every key,
  secret references only as references.
- Lint rule (forbidigo): outside `cmd/bothy` no product package reads the process environment
  (`os.Getenv` and its relatives, `exec.Cmd.Environ`, `http.ProxyFromEnvironment`); tests of
  repository tools under `tools/` are exempt.
- Lint rules (gocognit, goconst): `make lint` fails on a function above the gocognit
  `min-complexity`, as rule S3776 of the quality gate does, and on most string literals repeated
  at least the goconst `min-occurrences` in a package; goconst skips some that rule S1192 counts,
  such as literals in call arguments, so only the analysis finds those. `_test.go` files and the generated code that `sonar.exclusions` leaves
  out are exempt.
- `tools/configschema`: exports the configuration schema to `architecture/config-schema.json`
  (`go generate ./tools/configschema`, held current by `make codegen-check`), the input of the
  configuration reference.
- `internal/platform/secrets`: secret provider for references `file:<path>` and `env:<NAME>`
  (`azkv` and `awssm` parse but are not supported yet). The file driver reads only regular files
  with mode `0400` or `0600` owned by the process user and strips one trailing line end; the env
  driver reads the environment the process started with. Values are cached for
  `secrets.cache_ttl`, re-read after it, and a failed re-read keeps the previous value, logs once
  per TTL and counts `bothy.secrets.refresh.failures`. Secrets live in `secrets.Value`, which
  renders as `[redacted]` in every format, log and JSON output; errors name the reference, never
  the content.
- `internal/platform/secrets/stack` with `bothy secrets init` (creates missing secrets, adopts
  supplied ones, copies them into one directory per Compose service with the owner and mode of
  each service, idempotently, and prints fingerprints only) and `bothy secrets export` (writes
  the secret store as an age-encrypted tar bundle, optionally ASCII armored).
- `bothy config check --secrets`: resolves every `*_ref` key through the provider and prints `ok`
  or the error code per key; `bothy config check` warns on stderr about every `*_ref` key that
  uses an `env:` reference.
- Configuration keys `secrets.dir` (an absolute clean path other than `/`, without `<`) and
  `secrets.cache_ttl` (at least `1s`); the placeholder `<secrets.dir>` is expanded in configured
  `*_ref` values as well, and `file:` references must be clean absolute paths.
- Generated REST layer (`api/`, ADR 0013): `api/rest` holds the oapi-codegen v2.8.0 models of every
  schema of `docs/api/openapi.yaml` (also events and gateway frames) and the std-http and strict
  server of the operations enabled in the enable list `api/operations.yaml`; the list enables none
  yet and names every other operation with its milestone (P-04).
- `api/routes`: route table with authorization action, rate-limit class and request metadata per
  enabled operation, the generated action catalogue, and `Build`, which registers the generated
  server on a recorder and verifies every pattern, action and class before anything is mounted
  (AZ-P1). The generator `api/routes/gen` validates spec, catalogue and enable list and names every
  problem, and derives checks and metadata from the document the oapi-codegen loader reads; a
  fixture with three operations runs through the same pipeline.
- `internal/wire`: `MountRoutes`, start step 7 of the roles `api` and `all`, which builds the
  routes from the composite of module handlers; a failed route check logs `route check failed`,
  mounts nothing and returns the error, on which `bothy serve` (M1-11) is to exit before ready.
- `internal/platform/pg`: the PostgreSQL 18 pool of a process (pgx v5). `Acquire`, `Begin` and
  `Ping` wait at most `database.acquire_timeout` and report an exhausted pool as `503 unavailable`
  with the internal cause `db_pool_exhausted`; connections start with `statement_timeout`,
  `lock_timeout`, `idle_in_transaction_session_timeout`, `application_name`, `READ COMMITTED` and
  UTC, resolve their password through the secret provider per new connection and ignore every
  `PG*` variable and libpq file of the home directory; `SetLocalTimeouts` overrides the deadlines
  per transaction; `WaitReady` waits with backoff up to `startup.dependency_wait`. Spans of
  otelpgx never carry query parameters, and statement errors reach them with the SQLSTATE only.
- Schema check: `RequiredSchemaVersion` and `CheckSchema` refuse a database older than the
  binary with `database schema version <have> is older than required <need>; run "bothy migrate up"`;
  a newer schema passes.
- Database roles `bothy_owner`, `bothy_app`, `bothy_maint` and `bothy_monitor`: `pg.Bootstrap`
  creates or updates them idempotently with fixed attributes, sets their passwords as
  SCRAM-SHA-256 verifiers computed in Go (never in plaintext), grants `pg_monitor`, and gives
  database and schema `public` to `bothy_owner` without default privileges.
- `bothy migrate up [--bootstrap]`, `bothy migrate status` and `bothy migrate validate`: embedded
  goose migrations (package `migrations`) under a session lock with before- and after-steps, run as
  the schema owner with `lock_timeout = migrate.lock_timeout`; a schema newer than the binary is
  reported and left alone. Migration `00001_base_privileges` lets `bothy_app` read the version
  table.
- Configuration keys `database.*` (connection, pool, deadlines, `tx_retry_max`,
  `listen_reconnect_backoff_cap`), `startup.dependency_wait` and `migrate.lock_timeout`,
  `migrate.bootstrap_user` and `migrate.*_password_ref`. `bothy config check --secrets` reports
  the `migrate.*` references as skipped, because only the migrate container has their files.
- Test helper `internal/platform/pg/pgtest`: one disposable PostgreSQL 18 container per test
  binary on `127.0.0.1` with a random port and a fresh database per test cloned from a migrated
  template; without a Docker-compatible runtime the test binary fails instead of skipping. The
  test `TestPrivilegesAfterAllMigrations` holds the grant rule for every migration.
- `internal/platform/otel`: the one telemetry setup of the server. `Setup` installs tracer, meter
  and logger providers that export over OTLP (gRPC or HTTP per `OTEL_EXPORTER_OTLP_*PROTOCOL`)
  asynchronously behind one bounded in-flight budget per signal; a full queue or a failed export
  never delays a request and is counted in `bothy.telemetry.dropped{bothy.signal}`. Logs go
  through a level filter and a catalog allowlist to JSON on stdout and/or OTLP
  (`telemetry.logs.exporter`); with `telemetry.enabled = false` logs stay on stdout and every
  request still gets a trace id. `Middleware` and `RecordRoute` give each request a server span
  named after its route pattern, the `http.server.request.duration` metric with catalog
  attributes and one access log record; an incoming `traceparent` is only a span link unless
  `telemetry.traces.trust_remote_parent`. `CorrelationID`, `NewCommandLogger`, `ServerErrorLog`
  and `Pseudonymizer` (HMAC pseudonyms `bothy.principal.pseudo_id` per
  `privacy.telemetry_pseudonym_rotation` period) complete it.
- `internal/platform/otel/attr` (catalog attribute keys and `Allowed`) and
  `internal/platform/otel/genai` (the `gen_ai.*` keys, the forbidden content keys and the
  convention reference, held by a snapshot); tests keep both equal to the collector allowlist.
- `internal/platform/buildinfo`: `Version()`, a constant held equal to `VERSION`.
- Configuration keys `telemetry.enabled`, `telemetry.traces.sampler_ratio`,
  `telemetry.traces.trust_remote_parent`, `telemetry.logs.exporter`, `telemetry.logs.level`,
  `telemetry.pseudonym_key_ref`, `privacy.telemetry_pseudonym_rotation` and
  `instance.environment`.
- `bothy telemetry pseudonym <principal_id> [--at TIME] [--config FILE]`: prints the pseudonym of
  a principal and the start of its period for the owner, without a database. Commands log JSON
  through the same allowlist on stderr.
- Lint rule (sloglint): log messages are constants, attributes are `slog.Attr` values with
  constant keys, and functions with a context log with it; tests are exempt.
- `bothy serve [--role=api|worker|all] [--config FILE]` (M1-11): starts in the order of
  `docs/spec/02-architecture.md` "Start" (telemetry, secrets, internal listener, database within
  `startup.dependency_wait`, schema check, routes, public listener for `api` and `all`, ready),
  never runs migrations and aborts on an older schema with the hint to run `bothy migrate up`.
  `SIGTERM` or `SIGINT` shut down per role within `shutdown.total_timeout`; a second signal ends
  at once with exit code 3.
- `internal/platform/lifecycle`: runner with components in start order, a shutdown plan in fixed
  phases with per-step budgets bounded by the remaining total, `Fail` for fatal runtime errors
  and the exit codes 0, 1, 2 and 3; a step that waits for its budget (the drain delay) ends
  without a timeout warning.
- `internal/platform/health`: `/healthz` and `/readyz` with the built-in states `startup` and
  `shutting_down`, concurrent checks and the answer format of 16 "Two listeners"; readiness
  check `database`.
- `internal/platform/httpserver` (listener part): `Listen`, `NewServer` with
  `http.read_header_timeout`, `http.idle_timeout`, `http.max_header_bytes` and the server error
  log without client addresses and the internal mux.
- `bothy healthcheck --url URL [--timeout DURATION]`: probe of a loopback health endpoint over the
  loopback client of `platform/egress`, for images without shell; refuses other hosts and
  schemes with exit code 2.
- Configuration keys `http.read_header_timeout`, `http.idle_timeout`, `http.max_header_bytes`,
  `shutdown.drain_delay`, `shutdown.http_grace`, `shutdown.telemetry_flush_timeout` and
  `shutdown.total_timeout`; stop budgets above the total fail the start with exit code 2.
- Unit of work (`internal/kernel/txn`, `internal/platform/pg`): every write use case runs in one
  `READ COMMITTED` transaction with `SET LOCAL` deadlines, carried in the context and joined by
  nested calls; the phases `rights_begin` (only with `RightsChange`), `work`, `rights_check`
  (only with `RightsChange`), `sync` and `audit` run in this order before the commit. A unit is
  retried up to `database.tx_retry_max` times on deadlock (`40P01`) or lock timeout (`55P03`)
  and then ends with `503 unavailable`; any other error, a veto or a panic rolls it back.
- In-transaction events: `txn.Publish` delivers synchronously in registration order with a span
  `txn.subscribe <module>` per subscriber; per-unit state (`txn.Key`, `Load`, `Store`) is fresh
  for every attempt.
- `internal/wire`: the txn registry of all roles in one place (`NewTxnRegistry`), its golden file
  `testdata/txn_registrations.golden` and the check that every subscription points from a higher
  module to a lower one and every hook belongs to the module of its phase; `UnitConfig` binds the
  unit of work configuration.
- `pg.Tx` and `pg.Querier` for repositories, and `Exec`, `Query` and `QueryRow` on the pool with
  the `database.acquire_timeout` wait.
- Semgrep rule `bothy-go-tx-only-in-platform-pg`: transactions are opened only in
  `internal/platform/pg`.
- `internal/platform/jobs`: background jobs on River v0.49.0, the only package that imports
  River (vendor group `river` in the architecture gate). Modules register River-free handlers
  (`Register`, `HandlerOptions`, `AddPeriodic`); argument types may carry only IDs and `int64`,
  kinds are `<module>.<name>` of the owning module, queues are the five of chapter 02. Jobs are
  inserted with `InsertTx` in the ambient transaction of a unit of work. Role `api` builds an
  insert-only client; `worker` and `all` run jobs, periodic jobs and River's maintenance. Every
  attempt runs in the span `job <kind>` with `bothy.outcome`; a panic becomes a job error, a nil
  after cancellation becomes the context error.
- Configuration keys `jobs.schema`, `jobs.queue.<queue>.max_workers` (five queues) and
  `shutdown.jobs_soft_timeout`, with the cross-check `jobs_schema`.
- `bothy migrate up` creates `jobs.schema` with the rights of `bothy_app` and runs River's
  migrations before goose; `status` shows the River version, `validate` checks River's embedded
  migrations. `bothy serve` refuses to start when River's migrations are incomplete, soft-stops
  River at shutdown within `shutdown.jobs_soft_timeout` and ends with exit code 1 when the River
  client stops on its own (`lifecycle.Runner.FailOn`).
- `internal/notify/domain` (M3-17): the decision per recipient of
  `docs/spec/13-push-notifications.md` section 4 as pure functions: `ResolveLevel` (the seven
  steps of 13 3.1 with `muted_until` on the chosen row), `InQuietHours` and `ParseLocalTime` (13
  3.4 on the local wall time of each rule's zone, `start_local = end_local` as 24 hours),
  `Decide` (allowed, recipient, level and kind, activity in the thread, quiet hours, coalescing
  window, one route per device with the desktop delay), `Flush` (the one notification at the end of a
  coalescing window with the unread count) and `ReleaseDelayed` (drops a delayed mobile push that
  was read). No I/O, no `authorize`, no clock, no zone loading, no configuration access; a golden
  table of every combination checked against an oracle, quiet hours in `Europe/Berlin` on both
  changeover days, contract tests against `openapi.yaml`, 13 and 03, a purity test and a fuzz
  test.

- Module `instance` (M1-15): migration `00002_instance_counters.sql` with the tables `instance`
  and `sequence_counters`, the three counter rows and a trigger that keeps counter values from
  decreasing; the ports `Counters` (gapless, commit-ordered numbers with a run-time phase guard)
  and `State` (instance row, setup state); SemVer 2.0.0 comparison; the sqlc repository and
  `services/server/sqlc.yaml`.
- `bothy migrate up` creates the instance row if it is missing and records
  `instance.schema_version` and `instance.min_server_version` (`migrations.MinServerVersion`);
  against a newer schema it changes nothing.
- `bothy serve` loads the instance row at start (missing row: exit code 1 with the hint to run
  `bothy migrate up`); below `instance.min_server_version` it keeps running but `/readyz` fails
  with `schema_requires_newer_server`.
- `TestCountersOnlyUsedByPhaseHooks`: only `instance`, `wire` and registered phase hooks may use
  `instance.Counters`.
- `pg.SchemaVersion`, `pgtest.WithSteps`, and the log attributes `bothy.version.own` and
  `bothy.version.required`.
- HTTP chain of the public listener (`internal/platform/httpserver`, M1-18): recover, server span
  and correlation ID, base headers and client IP, routing with `404` and `405` as problems, then
  per route the stages `route` (operation, body limit, deadline, response writer), `cors`,
  `authenticate`, the slots `rate_limit`, `maintenance` and `idempotency` for later tickets,
  `body` (media type, JSON guard), `preconditions`, `validate` and `proof`.
- Every error leaves as `application/problem+json` (RFC 9457) through `httpserver.WriteProblem`
  with type, title, status, code, `correlation_id`, `instance` (secret path segments as
  `{token}`), and `detail`, `errors[]`, `current`, `reason`, `hint` where set; `503 unavailable`
  always carries `Retry-After`. `kernel/errs` gets titles from the contract, response headers on
  errors (`WithHeader`), `VersionConflict` and `PreconditionRequired`.
- Proof of the authorize check: a success without a check of the route's action becomes
  `500 internal_error` and counts in `bothy.authz.errors` (`authz_not_checked`); `kernel/authorize`
  gets the check log (`WithCheckLog`, `RecordCheck`, `Checks`).
- No JSON `null` and no integer beyond the JSON safe-integer limit leaves the server: the response writer removes
  null members (logged) and replaces null elements and unsafe integers by `500`.
- Request checks: JSON guard (duplicate members, invalid UTF-8, nesting above
  `http.max_json_depth` give `400 malformed_request`), body limits before reading
  (`http.max_json_body_bytes`, `file.max_upload_bytes`, `413 payload_too_large`), media type
  (`415`), required `If-Match` (`428`, also for `*`), and schema validation against the embedded
  contract with libopenapi-validator (`400 validation_failed` with `errors[]`, unknown members
  refused).
- `kernel/actor` (Actor, Credentials, Authenticator, `AnonymousOnly`), `kernel/reqmeta`
  (correlation ID and client IP), client IP from `server.trusted_proxies` (`Forwarded` before
  `X-Forwarded-For`, right to left, fail closed), CORS per SEC-CORS-1 and SEC-CORS-2 with
  preflight routes for `api.cors_allowed_origins`, `FormatETag` and `ParseIfMatch`, and the cursor
  codec `EncodeCursor` and `DecodeCursor` (`400 cursor_invalid`).
- Test harness `internal/platform/httpserver/httpservertest` for the chain, with recorded checks,
  a manual metric reader, a log capture and span recording.
- Configuration keys `http.max_json_depth`, `http.request_timeout`, `http.transfer_timeout`,
  `http.unavailable_retry_after`, `file.max_upload_bytes`, `api.cors_allowed_origins`,
  `server.trusted_proxies` and the required `server.public_url`, with the cross-checks
  `public_url_origin` and `cors_origins`.
- Route generator: `api/rest/openapi.gen.yaml`, a byte-identical copy of the contract that
  `api/rest.Spec` embeds, and the check that every request schema is closed (W7).
- Image `bothy-server` (`Dockerfile`, build context the repository root, ignore file
  `Dockerfile.dockerignore`): static `bothy` cross compiled for `linux/amd64` and `linux/arm64`
  on `gcr.io/distroless/static-debian13:nonroot` (ruling M1-23/1), user `65532:65532`, no shell, health check
  `bothy healthcheck --url http://127.0.0.1:8081/readyz` as in Compose, empty
  `/var/lib/bothy/signing-keys` with owner `65532:65532` and mode `0700`, OCI labels from the build
  arguments `VERSION` (must equal `VERSION`), `REVISION` and `SOURCE_URL`; base images pinned by
  digest. `make image` builds and loads it, `make image-test` runs the structure tests in
  `test/image` (build tag `imagetest`).
- Migration `00003_identity_principals`: the tables `principals`, `users` and `owners` with every
  check, unique index (`lower(handle)`, `lower(email)` where present, one active owners row per
  person), index and foreign key of the data model; triggers refuse a `users` row for a principal
  that is not a person and any change of `principals.kind`. The foreign key of
  `principals.avatar_file_id` follows with the files table.
- `internal/kernel/textcheck`: `DisplayText` rejects invalid UTF-8, control characters and bidi
  control characters in display texts (SEC-IN-4) with an error that names rule and rune offset,
  never the text.
- `internal/kernel/actor`: the principal kind of a request (`user`, `agent`, `app`, `anonymous`,
  `host`).
- `internal/identity`: handles and display names validated at the domain boundary (SEC-T07 for
  people), the read-only port `Directory` for other modules, and `PrincipalSource`, which gives
  authz kind, status and owner role of a principal; the repository maps unique, check, trigger
  and foreign key violations to `conflict` or `validation_failed` with a field and never puts SQL
  or values into an error.
- `internal/authz`: the source port `PrincipalSource` with `PrincipalFacts` and `PrincipalStatus`.
- Configuration keys `principal.handle_max_chars` (default 32) and
  `principal.display_name_max_chars` (default 64), bound by `wire.IdentityConfig`.
- `services/server/sqlc.yaml` with the identity entry; `make codegen-check` regenerates its code.
- Audit log (M1-24, ADR 0021): migration `00004_audit_events.sql` with `audit_events` and
  `audit_ip_keys` as in 03 "4.12"; the kernel port `auditing.Recorder` with `Record` (inside the
  unit of work), `RecordDenial` (aggregated, own transaction after a rollback) and
  `RecordDetached` (own transaction at once), the closed reason list of 07 "4.1" and typed
  attribute values; the audit phase hook (phase 3.3) that numbers rows with `audit_seq` and
  chains them with `SHA-256(prev_hash || canonical_json(row))` after RFC 8785; the details
  schema `audit_details.v1`; the aggregation of identical denials per
  `audit.deny_aggregate_window` with at most `audit.deny_aggregate_max_keys` open keys, flushed
  by the stop step `audit_denials` before the pool closes.
- Configuration keys `audit.deny_aggregate_window` (60s), `audit.deny_aggregate_max_keys`
  (10000), `privacy.audit_ip_key_period` (30d), `privacy.audit_ip_key_retention` (90d, at least
  the period) and `privacy.audit_ip_kek_ref`; `bothy serve` needs the secret `audit_ip_kek`.
- `kernel/txn.Detach`: a context outside the unit of work, so a unit can start its own
  transaction from inside an ambient one.
- Closed action catalogue in code (M1-25): `tools/catalogen` generates the identifier types
  `Action`, `EventType`, `Op`, `ResourceKind` and `ChannelOp` of `internal/kernel/authorize` with
  `Parse*` and `All*`, and the catalogue metadata of `internal/authz/domain`, from
  `docs/spec/actions.yaml`; `make codegen-check` holds both files. `authorize.Registry` collects the
  event types, gateway ops and terminal channel ops of the modules. `authz/app.Catalog` fails closed
  on an unknown action (`500 internal_error`, an ERROR log line, `bothy.authz.errors` with
  `unknown_action`) and checks route bindings and registrations at start; `bothy serve` exits with
  code 1 and one line per violation, and the routes are mounted through the same catalogue. The
  Semgrep rule `bothy-go-catalog-id-conversion` forbids converting strings to these identifiers.
- `authz` grant model storage (M1-27): migration `00005_authz_groups_grants.sql` with the tables
  `groups`, `group_memberships` and `grants` as `docs/spec/03-data-model.md` section 4.3 states
  them, with named checks (bot limits for agents and apps, subject and target shapes, host grants
  with exactly `terminal`) and the trigger `authz_grants_immutable`, which allows only one
  revocation and setting each expiry bookkeeping column once. The foreign keys to
  `identity_providers`, `spaces`, `channels` and `hosts` follow with the tickets that create those
  tables. `internal/authz/domain` holds levels, capabilities, `NewGrant`, `NewGroup`,
  `NewMembership`, validity at a time `t` (`expires_at = t` is already invalid), `NextExpiry` and
  the delegation rules `CheckCreate` and `CheckRevoke` (R-AZ-05); `internal/authz/app` declares the
  store port and `internal/authz/adapters/postgres` implements it with sqlc.
- `internal/platform/ratelimit` (M1-19): token buckets per rate-limit class and key of
  `docs/spec/05-actions.md` in sharded maps with a hard cap (`ratelimit.max_tracked_keys`) and a
  sweeper (`ratelimit.sweep_interval`); keys by client IP (IPv6 by /64), principal, agent, host,
  device or token, connection and incoming webhook, with the client IP for anonymous requests;
  `RateLimit-Policy` and `RateLimit` on every response of a limited operation and
  `429 rate_limited` with `Retry-After` in `StageRateLimit`, before `authorize`; requests with
  credentials reserve a token of `rl.public_auth` of the client IP before they are verified, keep
  it only when the credentials fail with `401` (other than `token_expired` and `use_dpop_nonce`),
  and get `429` while the bucket is empty; a `webhook_id` that is not an ID is `404` before a
  bucket exists; counter `bothy.ratelimit.rejected`.
- Configuration keys `ratelimit.<class>.per_minute` and `ratelimit.<class>.burst` for every class
  of 05, `ratelimit.max_tracked_keys` and `ratelimit.sweep_interval`; `bothy serve` (roles api and
  all) refuses to start when an enabled REST operation carries a gateway class or a class without
  limits.
- Event log (M1-16, B8, ADR 0009): migration `00006_sync_event_log.sql` with `event_log`
  partitioned by range of `seq` (no default partition, no foreign key, the indexes and checks of
  03) and the `SECURITY DEFINER` functions `sync_event_log_ensure_partitions` and
  `sync_event_log_drop_expired` of `bothy_owner`; `bothy_app` gets `SELECT, INSERT` and `EXECUTE`
  only. `kernel/eventlog` declares `Event`, `ActorKind` and `Appender`; `internal/sync` holds the
  port types `Event`, `Envelope`, `DeliveryFields`, `Sink`, `Tail` and `ErrGap`, its `app` the
  appender (validation before buffering), the phase hook of `sync` (counter `event_seq`, then
  `permission_version`, insert and `pg_notify('event_log', <last seq>)` in one statement; nothing
  for a unit without events) and the partition maintenance.
- One envelope for gateway, REST catch-up and webhooks (`Event.Envelope`, ADR 0052) without JSON
  `null` (ADR 0053); delivery members only from `DeliveryFields`; `correlation_id` is the trace ID
  of the request, `trace_context` only for a sampled request.
- Log tail of the roles api and all with its own `LISTEN` connection (`bothy-event-tail`,
  `pg.Pool.DialDedicated`), polling every `sync.log_poll_interval`, reconnect with backoff up to
  `database.listen_reconnect_backoff_cap`, gap check and the stop step `event_tail`; a failing
  tail ends the process with exit code 1.
- Job `sync.partition_maintenance` (queue `maintenance`, every
  `sync.partition_maintenance_interval`, also at the start of a worker); `bothy migrate up` runs it
  once after the instance row, so a fresh database has partitions before `serve`.
- Configuration keys `sync.event_log_partition_size`, `sync.event_retention`,
  `sync.event_payload_max_bytes`, `sync.log_poll_interval`, `sync.tail_batch_size`,
  `sync.partition_maintenance_interval` and `sync.event_log_partitions_ahead`.
- `pgtest.Durable()` for databases with `fsync`, `synchronous_commit` and `full_page_writes` on,
  `jobs.Registry.Periodic` and `jobs.Registry.Queue`, and the load test `perf/event_commit` of
  `perf.event_commit_min_per_s` in `make perf` with the threshold read from 02 by
  `perf/internal/report`.

- `internal/files`: the object storage port `ObjectStore` (`internal/files/app`) with exactly the
  nine operations of chapter 12 "4.1", the key schema and the one key grammar of all drivers
  (`internal/files/domain`), the file system driver `internal/files/adapters/fs` (start check of
  `file.fs.root`, no symbolic link followed at any level, atomic writes through `<root>/.tmp`,
  one byte range per read, multipart with parts at `uploads/<file_id>/<n>` and quoted SHA-256
  ETags) and the conformance suite `filestest.RunObjectStoreConformance` that every driver runs.
- Configuration keys `file.storage.driver` and `file.fs.root` with the cross-check
  `file_fs_root_absolute`, and `wire.FilesObjectStore`, which opens the file system driver and
  refuses the driver `s3` until the S3 driver exists; no role opens the store yet.
- `internal/files/adapters/s3` (M3-02): the S3 driver of `ObjectStore` with the AWS SDK for Go
  v2, core S3 only through an eight-operation interface and a presign interface with only
  `PresignUploadPart`. Access keys come from `file.s3.access_key_ref` and
  `file.s3.secret_key_ref` through the secret provider and follow a rotation without restart;
  every request goes through the egress client (purpose `objectstore`, no redirects, no SDK
  retries, `http.transfer_timeout`, `file.max_upload_bytes`); streamed bodies are signed as
  `UNSIGNED-PAYLOAD`, and a body of another length stores nothing. The conformance suite runs
  against SeaweedFS in a throwaway container from the image the Compose stack pins, and a
  presigned part URL is proven to bind its length there.
- Configuration keys `file.s3.endpoint`, `file.s3.region`, `file.s3.bucket`, `file.s3.prefix`,
  `file.s3.use_path_style`, `file.s3.checksum_mode`, `file.s3.access_key_ref` and
  `file.s3.secret_key_ref` with the cross-check `file_s3`; `wire.FilesObjectStore` takes a
  context and `wire.FilesDeps` and builds the S3 driver for `file.storage.driver = s3`;
  `bothy config check --secrets` resolves the two S3 references only with that driver.
- Lint: depguard allows the AWS SDK only in `internal/files/adapters/s3`, forbidigo forbids
  `DeleteObjects` and `PresignGetObject`; the module register has the vendor group `awssdk`.
- Disable a person, revoke a device, end a session (M1-45): the port `identity.Lifecycle`
  (`DisableUser`, `RevokeDevice`, `RevokeSession`), the in-transaction event
  `identity.DeviceRevoked` and the operations `disableUser` (owner, step-up), `revokeDevice`
  (self or owner, step-up) and `revokeSession` (self or owner). Each runs in one rights change in
  the order of 08 sections 10 and 11: the person `disabled` with all sessions, families, devices,
  keys and personal API tokens revoked (the last active owner gets `409 last_owner` and nothing
  changes); the device with its keys, sessions and families revoked; the session and its child
  sessions ended (`logout` for the own session, `admin` otherwise). Every ended session appends
  one `session.revoked`, `permission_version` rises, and from the commit on every request with a
  token of the person, device or session is `401 session_revoked`, also with an
  `Idempotency-Key` whose answer was stored before (the actor is `CredentialInactive`, so no
  stored answer is replayed to it). Repeating a revocation
  succeeds without effect (audit `no_change`). The new key `auth.device_inactive_revoke_after`
  (`180d`): an active device without contact for that long is refused at once and revoked by
  `identity.auth_cleanup` (`device.expire`, reason `inactive`).

### Changed

- Start and memory of the server process (M1-72): the production request validator no longer
  compiles the response schemas of every operation (only the test build with `contract_validate`
  keeps them for G8); `make perf` had measured `perf.startup_ready_max` and `perf.idle_rss_max`
  above their thresholds.
- Live delivery and catch-up read the thread and channel of an event once per event (hub) and once
  per page (`listEvents`, gateway catch-up) through a structure scope of the Authorizer
  (`authorize.WithStructureScope`) instead of once per connection and event; under the load of
  `make perf` live delivery fell behind by seconds.

- The body schemas of every client frame of the gateway refuse unknown members
  (`additionalProperties: false`, W7, M1-64); the route generator checks them and generates
  `routes.GatewayOps` from the section `gateway_ops` of `api/operations.yaml`.
- `createToken` decides personal tokens by the feature flag `tokens.personal_enabled` evaluated
  for the caller (M1-47); the always-off stand-in of M1-44 is removed.
- authz (M1-48): the delegation limits of `grant.create` and `grant.revoke` are part of the
  requirement step, so they come before `step_up_required`; a grant is visible only to owners
  and to holders of `manage` on its target (before: anyone who reads the target).
  `Admin.DeleteGroup` takes no reason (`group_deleted`), `Admin.RevokeGrant` needs one, an unknown
  subject or member is `400 validation_failed` with `unknown_subject` or `unknown_user` instead of
  `404`, an expiry that is not in the future has the field code `in_past`, and the audit entry of
  `grant.create` names a group subject as `subject_id`.
- `ObjectStore.PresignUploadPart` (M3-02) takes the part size before the validity, so the
  signature can bind the length (12 "3.3"); the file system driver refuses a size below 1 with
  `ErrSizeMismatch` and keeps answering `ErrNotSupported`.
- `httpserver.Operation` carries `RateLimitClass`; `Chain.SetCredentialsPrecheck` runs before the
  `Authenticator` for requests with credentials; `httpservertest.Options.Setup` installs stages
  and hooks before the routes are mounted.
- `wire.MigrationSteps` takes the migrate pool and the migration set, returns an error and gives
  only the after-step `instance`; River's step before goose moved to the new
  `wire.MigrateUpSteps`, which `bothy migrate up` calls.
- `golang.org/x/tools` is a direct dependency at v0.50.0 (tests only: `go/packages` for the
  static check; v0.49.0 cannot read the export data of Go 1.27, ruling M1-15/6).
- The public listener answers unmatched requests with a `404 not_found` problem instead of an
  empty `404`; `httpserver.Public` is replaced by `Chain.Public`.
- `wire.MountRoutes` takes the HTTP chain and mounts every route through it; `bothy serve` builds
  the chain with `actor.AnonymousOnly` until identity is wired, so every credential is refused.
- Every command that loads the server configuration needs `server.public_url`.
- Go coverage of the module leaves generated files (`*.gen.go`) out of the total.
- `wire.MigrateUpSteps` takes `wire.MigrateUpConfig` (jobs schema and the event log values) and
  `wire.NewJobRegistry` takes `wire.JobModules`; the txn registry needs the sync hook as well.
- `identity.SessionEnded` carries `At`, and a revocation that ends sessions on several devices
  publishes one event per device (M1-45). `app.NewAuthCleanup` takes the device sweep
  (`app.DeviceExpirer`), and `AuthCleanup.Run` returns deleted challenges plus revoked devices.

### Fixed

- Signing keys: a start whose key store `secrets.signing_key_dir` holds none of the unretired
  keys (a new or wrong volume) no longer retires every key of the instance; it ends with an error
  and changes no key. The new key `signing_key.accept_lost_keys` (default `false`) allows the
  repair for a restore without the key store.
- `instance` registers the event type `instance.updated` it emits, so the start check and gate G9
  see it (M1-67).

- `pgtest` no longer leaves its PostgreSQL container running when a test binary ends without
  reaching the end of `pgtest.Main` (a panicking test, the `-test.timeout` alarm, a signal,
  `SIGKILL`). The test binary holds a lease connection, and a watchdog in the container stops
  PostgreSQL after `leaseMisses` checks in a row find no lease connection, so Docker removes the
  container with its data volume (`AutoRemove`). Once the lease was taken, the watchdog also
  stops a server that gives no readable answer for `leaseStartGrace`, during a run as well.
  `pgtest.Main` also removes test containers labelled `org.bothy.test` without the label
  `org.bothy.test.watchdog` (older `pgtest` versions, other test helpers without a watchdog)
  once they are older than `legacyReapAge`.
  `TestNoContainerSurvivesTheRun` runs a passing, a failing, a panicking, a timed-out and a
  killed child run and checks that neither container nor volume survives.
  `TestWatchdogScript` checks the watchdog script against stub executables.
  `github.com/moby/moby/client` became a direct dependency, used only by the test helper `pgtest`.

### Security

- Go toolchain 1.27.2 to 1.27.3 (`toolchain` of `go.mod`, the builder image `golang:1.27.3-trixie` of
  the server and of `bothy-postgres`): the standard library advisory GO-2026-6809 in
  `crypto/x509` failed the gate `osv`.
- `github.com/getkin/kin-openapi` 0.144.0 to 0.149.0 (and `santhosh-tekuri/jsonschema/v6` 6.0.2 to
  6.0.3 with it): the advisories GHSA-23vg-4cjm-66q5, GHSA-fh4f-47mf-f2gj, GHSA-g8cf-rcjf-c8cf,
  GHSA-mh7x-f8wq-4jhx, GHSA-rj39-cv6w-8wv7 and GHSA-vrcc-3ch7-hppw failed the gate `osv`; the
  generated API code does not change.
- The application role may only insert and read `audit_events`; `UPDATE`, `DELETE` and
  `TRUNCATE` are revoked and refused by triggers for every role, the table owner included (only a
  `DELETE` by `bothy_maint` passes, for the retention trim of M7).
- Client IP addresses reach the audit log only as `ip_hmac` with `ip_key_id`: HMAC-SHA-256 under
  one key per `privacy.audit_ip_key_period`, stored AES-256-GCM encrypted in `audit_ip_keys`
  under the key behind `privacy.audit_ip_kek_ref`; `details` refuses free text outside `reason`,
  floats and `null`.
- `internal/platform/egress`: the one client for outgoing HTTP connections. It refuses targets
  that neither `egress.allowed_hosts` nor the consumer allows before any DNS lookup, checks the
  resolved addresses at connection time and dials only checked addresses (always blocked:
  unspecified, link-local with the cloud metadata address, multicast, reserved, NAT64, 6to4,
  Teredo, the cloud metadata addresses inside private ranges and the other ranges of
  SEC-NET-3; loopback, RFC 1918, RFC 6598 and unique local
  addresses only with `allow_private`), follows redirects only after the same checks, ignores
  environment proxies, always verifies TLS, bounds time and response size, and logs every refused
  attempt without path or query, counting it when metrics are configured. `NewLoopback` is the client for the health
  probe (loopback and plain http only).
- Configuration keys `egress.allowed_hosts` (entries `host[:port][;allow_private]`, invalid and
  duplicate entries fail at load), `egress.timeout` and `egress.max_response_bytes`.
- Semgrep rules `bothy-go-egress-only` (no other HTTP client, transport, dialer or reverse
  proxy in production code) and `bothy-go-tls-verify` (no `InsecureSkipVerify` other than
  `false`, also in tests).
- Transport protection of the public listener (M1-21, SEC-TLS-1 to SEC-TLS-3): in plaintext mode
  only peers on loopback or in `server.trusted_proxies` are served, and every other connection
  is closed at accept time before a byte is read; the first refusal logs a hint at Warn, later
  ones at Debug, never with the peer address. With `server.tls_cert_ref` and
  `server.tls_key_ref` set the server terminates TLS itself with TLS 1.2 as the floor and the Go
  cipher defaults (sources in the httpserver README); the material is checked at start step 2 (pair, validity, host of
  `server.public_url`) and follows rotation through the secret provider, keeping the previous
  certificate when a rotated one is broken. The internal listener stays plaintext without a peer
  filter.
- Every response of both listeners carries `X-Content-Type-Options`, `Referrer-Policy` and no
  `Server` header, applied once more right before the header block goes out, and
  `Strict-Transport-Security: max-age=<http.hsts_max_age>` while `server.public_url` uses
  `https`. The internal listener answers unknown paths and methods with problem bodies.
- Configuration keys `server.tls_cert_ref`, `server.tls_key_ref` (optional secret references,
  both or neither, TLS needs an `https` public URL) and `http.hsts_max_age` (whole seconds,
  default in chapter 19). A `secret_ref` key with an empty default now accepts the empty text as "no
  reference"; `bothy config check --secrets` prints "not set" for it.
