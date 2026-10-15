# Webhooks

Status: draft M0. Normative are the section `webhooks:` in [openapi.yaml](openapi.yaml) (one
entry per event type with a schema) and this document (delivery rules). Decision: ADR 0016.
Conventions of the API: [docs/spec/06-api.md](../spec/06-api.md). Events and receive actions:
[docs/spec/05-actions.md](../spec/05-actions.md), section 6.

There are two directions:

| Direction | Who | Purpose |
|---|---|---|
| outgoing | server to an installed app | events that the app has subscribed to, signed, to an HTTPS address |
| incoming | external system to the server | write a message as an app into a channel, through a secret URL |

Plugins never run in the server process (B6). Webhooks and the gateway are their only ways to
learn about events.

## 1. Subscribe

1. The app is installed, has the scope `events:subscribe` and the desired types in
   `event_subscriptions` (`installApp`, `setAppScopes`).
2. An owner creates the subscription (`createWebhookSubscription`, action `webhook_out.manage`,
   step-up). The address must use `https` and its host must be in `webhook.allowed_hosts`
   (R-DM-17); otherwise `400 webhook_host_not_allowed`.
3. Each subscription covers a subset of the subscribed types (`event_types`). A subscription can
   be paused (`state = paused`) and activated again.
4. Validation (M2-26, [DECISIONS.md](../DECISIONS.md) D-M2-26-8 and D-M2-26-18): entries of
   `webhook.allowed_hosts` have the form `host[:port]` (without a port only `443`); the address
   must parse, use `https`, have no user info and no fragment, and its canonical host (lower case,
   no trailing dot) and port must match an entry, otherwise `400 webhook_host_not_allowed` for a
   host or port outside the list and `400 validation_failed` at `/url` for every other defect.
   `event_types` is not empty, has no duplicate (`invalid`), and each type is subscribable by apps
   (`not_subscribable`, section 7) and in the `event_subscriptions` of the installation
   (`not_granted`), each at `/event_types/<index>`. An app without an `active` or `suspended`
   installation answers `409 conflict` with `reason: app_not_installed`. `rotate_hmac_secret`
   without HMAC is `400 validation_failed` with `hmac_not_enabled`. A response with a `whsec_`
   secret is a one-time secret: a replay with the same `Idempotency-Key` answers `409 conflict`
   with `reason: one_time_secret`.

## 2. Delivery

Every delivery is a `POST` with exactly one event:

```http
POST /chat-events HTTP/1.1
Host: hooks.example.com
Content-Type: application/json
User-Agent: chat-server-webhooks/<server-version>
webhook-id: 0192f4a0-7b1c-7a3e-8c1d-5f0e9a2b4c6d
webhook-timestamp: 1791624600
webhook-signature: v1a,<Ed25519 signature in Base64> v1,<HMAC-SHA256 in Base64>

{"id":"0192f4a0-7b1c-7a3e-8c1d-5f0e9a2b4c6d","seq":5301,"type":"message.created", ...}
```

| Part | Rule |
|---|---|
| Body | The event envelope from openapi.yaml (`MessageCreatedEvent` etc.), the same form as in the gateway and in `GET /events`. Standard Webhooks recommends `type`, `timestamp`, `data`; our envelope has `type` at the top level and `occurred_at` instead of `timestamp`, because one form for all delivery channels matters more than the recommendation [verified, W1] ([ADR 0052](../adr/0052-one-event-envelope-for-gateway-rest-and-webhooks.md)). |
| `webhook-id` | Event ID (`id` in the envelope). The same for every retry and for a redelivery triggered by the owner. |
| `webhook-timestamp` | Unix seconds of this attempt. |
| `webhook-signature` | Space-separated list: always `v1a,<base64>` (Ed25519), additionally `v1,<base64>` (HMAC-SHA256) if the subscription uses HMAC. During a key rotation both keys are in the list. |
| Signed content | `{webhook-id}.{webhook-timestamp}.{body}`, the body byte for byte as sent [verified, W1]. |
| Epoch | Every delivery carries the delivery field `sync_epoch` (04, 2.2 and 7.5; [ADR 0059](../adr/0059-sync-epoch-after-restore.md)). |
| Content | Only fields that the app may see with the receive action (04, 2.2). Never channel prompts, skills or secrets (R-AZ-09). Large payloads are truncated and carry `truncated: true`; the app loads the rest via REST. |

### 2.1 Signing keys

1. `v1a` uses the server key with the purpose `webhook_out` (03, `server_signing_keys`). The
   public keys are available at `GET /api/v1/instance/webhook-keys` in the format `whpk_<base64>`
   [verified, W1]. The subscription names its `signing_key_kid`.
2. Key rotation (`rotateSigningKey`, purpose `webhook_out`): the new key appears in the list with
   `state = next` before it signs. After that, the server signs with both until the
   old one reaches `not_after`.
3. `v1` (HMAC) can be enabled per subscription (`enable_hmac`). The secret is not stored but
   derived: `HMAC-SHA256(master key, "webhook-v1:" || subscription_id || ":" ||
   generation)`, with the master key from the secret provider (B12). The server shows it once as
   `whsec_<base64>`. `rotate_hmac_secret` increases the generation; for
   `webhook.secret_rotation_overlap` the server signs with the old and the new generation
   (03 `webhook_subscriptions.hmac_generation` and `hmac_rotated_at`).
4. Signing set and encodings (M2-26, D-M2-26-10 and D-M2-26-11): `v1a` carries one signature per
   signer of the purpose `webhook_out`: the `active` key first, then every `retiring` key whose
   `not_after` lies ahead. `v1` uses the key of `hmac_generation` and, while `hmac_rotated_at +
   webhook.secret_rotation_overlap` lies ahead, the key of the previous generation; never an older
   one. Entries are separated by one space, all `v1a` entries first in signer order, then the `v1`
   entries newest generation first, each `<scheme>,<standard base64 with padding>`. The HMAC
   derivation takes the subscription ID in canonical lower-case hyphenated form and the generation
   in decimal ASCII; the master key is the hex-decoded value of `webhook.hmac_master_key_ref`; the
   derived 32 bytes are the HMAC key, and `whsec_` plus their standard base64 is the secret shown
   once. A public key is `whpk_` plus the standard base64 of its DER-encoded SubjectPublicKeyInfo
   (RFC 8410).
5. Reason for two methods: Standard Webhooks recommends asymmetric signatures [verified, W1];
   but the reference libraries name HMAC in their descriptions, `v1a` is not documented there
   [assumed, W2]. HMAC keeps the hurdle for receivers low.

### 2.2 Test vectors

[webhook-test-vectors.json](webhook-test-vectors.json) is the contract file for every verifier
(server, SDKs; D-M2-26-12): `{"version": 1, "derivation": [...], "signatures": [...]}`. A
derivation case is `{master_hex, subscription_id, generation, derived_hex}` with `derived_hex` the
lower-case hex of the 32 derived bytes; the secret shown to the owner is `whsec_` plus their
standard base64 and is not stored in the file. A signature case is `{name, webhook_id, timestamp,
body, ed25519_seed_base64, public_key, hmac_secrets, signature_header}`; `ed25519_seed_base64` and
`public_key` are lists in signer order (one entry per `v1a` signer), `hmac_secrets` lists the
`whsec_` secrets newest generation first (each carries the marker `EXAMPLE`), and
`signature_header` is the value of `webhook-signature` byte for byte. The cases cover one key and
one secret, two `v1a` signers, two HMAC generations, a body with non-ASCII characters and escapes,
and a body with `truncated: true`.

## 3. Verification by the receiver

1. Read the raw body; do not parse it as JSON and serialize it again first [verified, W1].
2. Check `webhook-timestamp`: the deviation from the own clock is at most
   `webhook.receiver_tolerance`.
3. Check the signatures: try every signature of the list against the trusted keys; one valid
   signature is enough. Fetch the `v1a` keys once and keep them as a trust list; do not take them
   from the request [verified, W1]. Compare HMAC in constant time.
4. Detect duplicates: remember `webhook-id`, at least for the duration of the retries
   (`webhook.retry_max_age`).
5. Respond quickly with `2xx`, within `webhook.delivery_timeout`
   ([11-extensions.md](../spec/11-extensions.md)); do the work afterwards.
6. Establish the order through `seq` and the object state through `resource.version` (04, 9). If
   `sync_epoch` changes, the instance was restored: discard the state remembered by `seq` and
   continue with the new timeline, otherwise its smaller numbers would count as old.

## 4. Retries and disabling

| Response | Effect |
|---|---|
| `2xx` | Success |
| `410` | The subscription is disabled (`state = disabled`) [verified, W1] |
| `429`, `502`, `503`, `504` | Throttle; `Retry-After` is respected [verified, W1] |
| `3xx` | Failure; redirects are not followed [verified, W1] |
| other status, timeout, dropped connection | Failure |

1. Retry with exponential backoff and full jitter between
   `webhook.retry_backoff_base` and `webhook.retry_backoff_cap` (03), until
   `webhook.retry_max_age` is reached. Then the delivery becomes `dead`.
2. After `webhook.max_consecutive_failures` (03) consecutive failures, the subscription becomes
   `disabled`. This is recorded in the audit log (system action `webhook_out.transition`, 05,
   R-AZ-27) and shown in the admin console. The product sends
   no emails (Baseline section 4).
3. Every attempt first checks `webhook_out.deliver` and the receive action of the event for
   the app (05, R-AZ-11). If the app has lost access in the meantime, the delivery becomes `dead`
   with `last_error = not_authorized`, without a request.
4. An owner can trigger any delivery again (`redeliverWebhook`); it carries the same
   `webhook-id`.
5. Response bodies are not stored (03, `webhook_deliveries.last_error`).
6. Statuses (M2-26, D-M2-26-6): `pending` waits for its first or next attempt (also after a failed
   attempt, with `next_attempt_at`); `succeeded` got a `2xx`; `failed` ended with `410` (terminal,
   the owner may redeliver); `dead` ended after `webhook.retry_max_age`, without access
   (`not_authorized`), because the type left `event_types` or the installation's
   `event_subscriptions` (`not_subscribed`), or because its event left the log
   (`event_expired`). `last_error` is a code of the closed list `timeout`, `connection_failed`,
   `tls_failed`, `address_blocked`, `host_not_allowed`, `http_status`, `throttled`, `gone`,
   `not_authorized`, `not_subscribed`, `event_expired`.
7. Only failures count towards `webhook.max_consecutive_failures`: other statuses, `3xx`,
   timeouts, refused or failed connections. A throttle (`429`, `502`, `503`, `504`) and an ending
   without a request (`not_authorized`, `not_subscribed`) do not count. After the n-th failed or
   throttled attempt the pause is uniform in [`webhook.retry_backoff_base`,
   min(`webhook.retry_backoff_cap`, `webhook.retry_backoff_base` * 2^(n-1))]; for a throttle with a
   valid `Retry-After` (delta-seconds or HTTP-date; a past date counts as zero) the larger of
   backoff and `Retry-After` applies. No retry is scheduled after `created_at +
   webhook.retry_max_age` (a later time is cut to it), and a failure at or after it makes the
   delivery `dead` and the subscription `disabled` (D-M2-26-7).
8. A redelivery (`redeliverWebhook`, D-M2-26-14) makes a `pending` row due at once; a `succeeded`,
   `failed` or `dead` row becomes `pending` again with `last_error` and `response_code` cleared,
   `attempt` kept and `created_at` set to now, so the window of `webhook.retry_max_age` starts
   again. It goes out before the later pending rows of the subscription; a paused or disabled
   subscription keeps it waiting. An event that is no longer in the log answers `409 conflict`
   with `reason: event_expired`.
9. Retention (M7-05) may delete only `succeeded`, `failed` and `dead` rows older than
   `retention.webhook_deliveries`, never `pending` ones.

## 5. Order and duplicates

1. Delivery at least once (at-least-once).
2. Per subscription strictly in `seq` order, as on the gateway: the next event goes out only
   after the previous one has been accepted with `2xx`
   ([11-extensions.md](../spec/11-extensions.md), section 7). Reason: receivers can apply
   events in order as in the gateway, and for subscriptions of agents `2xx` counts as an
   acknowledgement up to this `seq` ([04-sync-protocol.md](../spec/04-sync-protocol.md),
   section 15.6).
3. `webhook_subscriptions.last_delivered_seq` is the highest `seq` accepted with `2xx`.
   If an event still fails when `webhook.retry_max_age` is reached, the delivery becomes `dead`
   and the subscription `disabled`; after the owner turns it on again, delivery continues with the
   next event.
4. While a subscription is `paused`, its deliveries are still created and wait; a resume
   continues with them in `seq` order. While it is `disabled`, no delivery is created: events of
   that time are not delivered (D-M2-26-1); a re-enable continues with the remaining pending rows
   (the head first) and then with events committed after it.
5. Gap (D-M2-26-5): when the event of the head left the log, or when the consumer `webhooks` lost
   a range to retention before reading it, the subscription becomes `disabled` with cause `gap`,
   its expired pending deliveries become `dead` with `event_expired`, and the audit entry names the
   lost range; nothing is skipped silently.

## 6. Delivery security

1. Only `https`, only hosts from `webhook.allowed_hosts`, no redirects.
2. Deliveries go through the egress client of the server
   ([19-security.md](../spec/19-security.md), SEC-NET-1 to SEC-NET-5): it checks the address
   after DNS resolution when it opens the connection and rejects loopback, link-local and private
   addresses, except for entries in `egress.allowed_hosts` with `allow_private`. Reason:
   protection against DNS rebinding and SSRF (Baseline section 4).
3. Time limit per attempt `webhook.delivery_timeout`; Standard Webhooks recommends 15 to 30 seconds
   [verified, W1].

## 7. Event types for apps and agents

Generated from openapi.yaml. Events with the audience `self` go only to the app or the agent that
is itself the receiver (own commands, own components, own permissions). Events marked
"agent subscription only" go only to subscriptions of an agent (webhook as transport,
[04-sync-protocol.md](../spec/04-sync-protocol.md) section 15.6).

<!-- BEGIN GENERATED: webhook_events -->
| Event type | Receive action | Receiver and scope in addition to `events:subscribe` | Schema |
|---|---|---|---|
| `message.created` | `message.read` | `messages:read` | `MessageCreatedEvent` |
| `message.updated` | `message.read` | `messages:read` | `MessageUpdatedEvent` |
| `message.deleted` | `message.read` | `messages:read` | `MessageDeletedEvent` |
| `reaction.added` | `message.read` | `messages:read` | `ReactionAddedEvent` |
| `reaction.removed` | `message.read` | `messages:read` | `ReactionRemovedEvent` |
| `pin.added` | `thread.read` | `messages:read` | `PinAddedEvent` |
| `pin.removed` | `thread.read` | `messages:read` | `PinRemovedEvent` |
| `thread.created` | `thread.read` | `messages:read` | `ThreadCreatedEvent` |
| `thread.updated` | `thread.read` | `messages:read` | `ThreadUpdatedEvent` |
| `thread.closed` | `thread.read` | `messages:read` | `ThreadClosedEvent` |
| `thread.reopened` | `thread.read` | `messages:read` | `ThreadReopenedEvent` |
| `thread.moved` | `thread.read` | `messages:read` | `ThreadMovedEvent` |
| `thread.moved_out` | `channel.read` | `messages:read` | `ThreadMovedOutEvent` |
| `agent_session.changed` | `thread.read` | `messages:read` | `AgentSessionChangedEvent` |
| `run.state_changed` | `run.read` | `messages:read` | `RunStateChangedEvent` |
| `space.created` | `space.read` | `messages:read` | `SpaceCreatedEvent` |
| `space.updated` | `space.read` | `messages:read` | `SpaceUpdatedEvent` |
| `space.archived` | `space.read` | `messages:read` | `SpaceArchivedEvent` |
| `channel.created` | `channel.read` | `messages:read` | `ChannelCreatedEvent` |
| `channel.updated` | `channel.read` | `messages:read` | `ChannelUpdatedEvent` |
| `channel.settings_changed` | `channel.read` | `messages:read` | `ChannelSettingsChangedEvent` |
| `channel.archived` | `channel.read` | `messages:read` | `ChannelArchivedEvent` |
| `command.changed` | `command.read` | none (metadata, 05 section 2.6) | `CommandChangedEvent` |
| `file.updated` | `file.read` | `files:read` | `FileUpdatedEvent` |
| `user.updated` | `user.read` | `users:read` | `UserUpdatedEvent` |
| `agent.updated` | `agent.read` | none (metadata, 05 section 2.6) | `AgentUpdatedEvent` |
| `instance.updated` | `instance.read` | none (metadata, 05 section 2.6) | `InstanceUpdatedEvent` |
| `status.updated` | `status.read` | none (metadata, 05 section 2.6) | `StatusUpdatedEvent` |
| `access.changed` | `self` | own only (app or agent is the receiver) | `AccessChangedEvent` |
| `agent.invocation` | `agent.events.consume` | agent subscription only (agent is the receiver) | `AgentInvocationEvent` |
| `agent.followup` | `agent.events.consume` | agent subscription only (agent is the receiver) | `AgentFollowupEvent` |
| `run.cancel_requested` | `agent.events.consume` | agent subscription only (agent is the receiver) | `RunCancelRequestedEvent` |
| `agent.config_changed` | `agent.config.read` | agent subscription only (agent is the receiver) | `AgentConfigChangedEvent` |
| `interaction.created` | `agent.events.consume` | own only (app or agent is the receiver) | `InteractionCreatedEvent` |
| `command.invoked` | `agent.events.consume` | own only (app or agent is the receiver) | `CommandInvokedEvent` |
<!-- END GENERATED: webhook_events -->

Not subscribable: every event of the catalogue that is not in the table is available only through
the gateway and `GET /api/v1/events`. These are all ephemeral events and the following durable ones:

| Group | Event types | Reason |
|---|---|---|
| personal events of people | `read_state.updated`, `draft.updated`, `draft.deleted`, `settings.updated`, `sidebar.updated`, `notification_pref.updated`, `quiet_hours.updated`, `session.revoked` | concern the devices and the interface of a person; an app or an agent is never the receiver |
| expiry reminder | `access.expiring` | a hint for the interface; an app or an agent learns the effective expiry through `access.changed` |
| administration | `grant.changed`, `group.changed`, `gdpr.request_updated`, `import.progress` | data of the console; apps have no `manage` (11 section 5) and see their own permissions through `access.changed` |
| terminal | `host.updated`, `terminal_session.changed` | terminal actions only for people in a session (R-AZ-23) |

Agent events (invocations, follow-ups, cancellation) exist as webhooks only for subscriptions of an
agent (table above). A new durable event type is either in `webhooks` of openapi.yaml or
in this list; the check `api-catalogue-sync` (M1-03, subcheck `webhook-events`) enforces this.

## 8. Incoming webhooks

1. A person with `manage` on the channel creates the incoming webhook
   (`createIncomingWebhook`, step-up). They choose an installed app as the writing principal
   and optionally a fixed thread. The response contains the URL with the token exactly once.
2. External systems send `POST /api/v1/hooks/{webhook_id}/{token}` with `content_md`, optionally
   `title` (a new thread if there is no fixed thread) and `client_msg_id` (idempotency for
   retries of the sender).
3. The message has `origin = app`, and the author is the app. It never triggers agents (03,
   `incoming_webhooks`). The app needs `write` on the channel.
4. Unknown or revoked webhooks and wrong tokens result in `404`. Rate-limit class
   `rl.webhook_in` per webhook.
5. The token is in the URL because many senders can only configure a URL. Server and
   proxies redact the path after `/hooks/{webhook_id}/` in logs and traces. Rotation: create a new
   webhook, revoke the old one.

## 9. Configuration values

The values of this document are in the table of
[11-extensions.md](../spec/11-extensions.md#16-configuration-values-of-this-chapter), so that
the generator takes them into the configuration reference (ruling L4): `webhook.retry_max_age`,
`webhook.secret_rotation_overlap` and `webhook.receiver_tolerance`. The last one is a recommendation
for receivers, not a value of the server. `webhook.allowed_hosts`, `webhook.max_consecutive_failures`,
`webhook.retry_backoff_base` and `webhook.retry_backoff_cap` are in 03, section 9;
`egress.allowed_hosts` is in [19-security.md](../spec/19-security.md). The default for
`webhook.retry_max_age` is close to the example schedule of Standard Webhooks (about 75 hours)
[derived, W1].

## Sources

Retrieved on 2026-10-10.

| No. | Source | Content | Marker |
|---|---|---|---|
| W1 | https://github.com/standard-webhooks/standard-webhooks/blob/main/spec/standard-webhooks.md | Headers, signed content, `v1`/`v1a`, prefixes `whsec_`/`whpk_`, multiple signatures, status rules, backoff example, time limit 15 to 30 s, recommended body form | [verified] |
| W2 | https://github.com/standard-webhooks/standard-webhooks | Reference libraries per language; support for `v1a` is not described there | [assumed] |
