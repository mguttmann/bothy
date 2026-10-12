# 00 Glossary

Status: draft M0, integrated after the feedback of all chapters. Binding for all
specifications. The terms from the planning basis (section 3) are adopted and extended by all
further terms of chapters 03, 04 and 05. Sorted by term.
Code names are English and apply in API, database and code.

| Term | Code name | Definition |
|---|---|---|
| Access token | `access token` | Short-lived, DPoP-bound token of a person with session ID `sid`. |
| Ack | `ack` | Report of an agent adapter up to which global sequence it has durably processed events. |
| Acting-for binding | `acting_for` | Rule that an agent, within a run, has at most the permissions of the triggering person (SR9). |
| Action | `action` | Named operation in dot notation (for example `message.create`) that `authorize` checks. |
| Action catalogue | `action catalogue` | Closed, normative list of all actions in docs/spec/05-actions.md. |
| Agent | `agent` | Principal for a bot account that an adapter runs outside the server; at most level `write` and at most `upload` as a capability; never triggers agents itself. |
| Agent adapter | `adapter` | Program outside the server that connects an agent to its runtime through the public API (for example the reference adapter for Hermes Agent). |
| Agent runtime | `runtime` | Software that actually executes an agent turn; never runs in the server process (B6, B10). |
| Agent session | `agent_session` | Mapping of a thread to an external session of an agent; older rows of the same thread are the earlier session references. |
| API token | `api_token` | Long-lived access token for agents, apps or personal tools in the format `bothy_<kind>_<random>_<checksum>` ([ADR 0061](../adr/0061-token-format-with-a-fixed-prefix-and-checksum.md)); stored only as a hash. |
| App | `app` | Principal of an installed integration with a manifest; acts only through the public API, at most level `write` and capability `upload`. |
| App installation | `app_installation` | Installation of an app in the instance with granted scopes and event subscriptions. |
| App manifest | `manifest` | Versioned description of an app: scopes, event subscriptions, slash commands, components, webhook targets. |
| Attachment | `message_attachment` | Assignment of a file to a message in a fixed order. |
| Audit checkpoint | `audit_checkpoint` | Signed state of the hash chain of the audit log, additionally stored outside the database. |
| Audit log | `audit_event` | Append-only, hash-chained log of security-relevant decisions, separate from telemetry. |
| authorize | `authorize` | The single authorization function `authorize(principal, action, resource) -> Decision` in the module `authz` (B7). |
| Backend policy | `billing_mode` | Setting per agent of what it runs on: `personal` (subscription of exactly one person) or `organization` (organization API key). |
| Backup run | `backup_run` | Run of a backup or of an automatic restore test, with a result per check. |
| Branding | `branding` | Display name, logo and colors of the instance as a setting (`branding.*`). |
| Capability | `capability` | Permission besides the level: `approve_actions`, `upload`, `export`, `manage`; on hosts only `terminal`. |
| Catch-up | `catch-up` | Fetch of all visible durable events after a cursor, filtered by `authorize`. |
| Channel | `channel` | Place for conversations; kinds `text`, `forum`, `dm`. `text` and `forum` are in exactly one space, `dm` in none (`space_id` NULL, 03); optionally read-only (`read_only`). |
| Channel prompt | `prompt` | Instruction text of the channel for the operator; readable only with `manage`, never in events. |
| Check without a request | `recheck principal` | Evaluation of the permissions of a person that the server performs without a request of this person: `agent.invoke` before every delivery to an agent, at `run.token.renew` and for the run from deferred follow-ups, `command.invoke` and `interaction.create` before handing over to an app, the permissions of the trigger in the acting-for binding; the person counts as a principal without session, device and token, with blocks only from their status and the owner status from the current owner list (05 section 5, R-AZ-29). |
| Client interaction ID | `client_interaction_id` | UUIDv7 of the client per interaction; unique per principal, makes clicks repeatable. |
| Client message ID | `client_msg_id` | UUIDv7 of the client per message; unique per author, a repetition returns the original. |
| Close code | `close code` | Code with which the server ends a WebSocket connection, with a defined client reaction. |
| Commit order | `commit order` | Order in which transactions become visible; the global sequence follows it exactly. |
| Component | `component` | Interactive element on a message: button, select or form. |
| Configuration value | `config key` | Named value for a limit, a window or an interval; its default is in exactly one table. |
| Consumer | `consumer` | Named reader of the event log with a durable cursor: per agent in `agent_consumer_cursors`, in the worker (push, webhooks) in `event_consumer_cursors`. |
| Control command | `control command` | Command such as stop or status that never goes to the model as a prompt and never becomes a follow-up. |
| Cost entry | `cost_entry` | Booking of the costs of a run, as an estimate or reconciled, with the reported mode. |
| Counter row | `sequence_counters` | Table row whose lock in the writing transaction enforces the commit order of the numbers. |
| Credential holder | `credential_holder_id` | Person on whose personal subscription an agent with `billing_mode = personal` runs. |
| Cursor | `cursor` | Highest global sequence that the server has checked for a receiver; starting point for catch-up. |
| Delivery | `delivery` | Handover of an event to a connection, a push service or a webhook; checked by `authorize` every time. |
| Delivery field | `delivery field` | Field in the envelope that is created per delivery and never stands in the log: `invocation_token`, `redelivered`, `sync_epoch` (only in webhook deliveries; 04 section 2.2). |
| Dev environment | `dev_environment` | Isolated work environment of a person for a runner agent (milestone multi-user dev environment). |
| Device | `device` | Installed client of a person; a new device waits (`pending`) for approval and until then may only read itself. |
| Device key | `device_key` | Key pair in the secure storage of a device with purpose `dpop` (binds tokens) or `step_up` (signs only after biometrics or device PIN). |
| Direct message (DM) | `dm` | Channel kind for conversations between people without a space, without an agent and without grants; access only for participants. |
| Done | `closed` | Thread state after completion; a new message reopens the thread, a system line does not ([03-data-model.md](03-data-model.md), `messages`). |
| DPoP | `dpop` | Proof-of-Possession according to RFC 9449: tokens are bound to the device key and worthless without it. |
| Draft | `draft` | Text of a person per thread or channel that has not been sent yet, versioned and across devices. |
| Durable event | `durable event` | Event in the event log with a global sequence; can be caught up until `sync.event_retention`. |
| Effective level | `effective level` | Highest level of all valid grants of a principal and its groups on the path from the target upward. |
| Effective operator | `effective operator` | Operator of a channel after resolving the inheritance from the space. |
| Enrollment code | `enrollment_code` | One-time code of the owner for the first device, for linking an identity or for recovery. |
| Ephemeral event | `ephemeral event` | Event without storage and without a sequence (typing, live box patch, presence, notification, autocomplete request); delivered at most once. |
| Event | `event` | Notice of a change with a uniform envelope; durable or ephemeral. |
| Event envelope | `envelope` | Common fields of every event (`id`, `seq`, `type`, `scope`, `resource`, `payload` and others), the same for gateway, REST catch-up and webhooks; without JSON `null`. |
| Event log | `event_log` | Table of the durable events, ordered by the global, commit-ordered sequence (B8). |
| Expiry reminder | `access.expiring` | Personal event before a grant or membership expires. |
| Feature flag | `feature_flag` | Switch for a feature, global or for people and groups. |
| File | `file` | Uploaded object in the object storage with purpose, checksum, virus scan, thumbnails, optional transcript and file text extract. |
| File purpose | `files.purpose` | Kind of a file: `attachment`, `avatar`, `branding`, `gdpr_export`; determines the read path. |
| File text extract | `file_text_extract` | Text extracted from a file, with its own search vector. |
| File ticket | `file ticket` | Short-lived, signed URL for media players that cannot send a DPoP proof; every fetch checks `file.read` again ([ADR 0048](../adr/0048-file-download-only-through-the-server-with-short-lived-tickets.md)). |
| Focus | `focus` | Set of threads that a client currently displays; only they get live box patches and typing. |
| Follow-up | `followup` | Message in a thread with an active run; delivered to the agent as a follow-up to the run. |
| Forum | `forum` | Channel kind as a list of posts; every post is a thread; with tags and a Done section. |
| Forum tag | `forum_tag` | Tag of a forum for filtering posts. |
| Fresh step-up | `terminal.step_up_max_age` | Step-up that is younger than `terminal.step_up_max_age` and is used up for exactly one terminal opening ([08-identity.md](08-identity.md), section 6.3). |
| Gateway | `gateway` | WebSocket interface of the server at `/api/v1/gateway` for events and ephemeral ops; its own module without tables. |
| Gateway session | `gateway session` | Server-side state of a WebSocket connection; resumable within `sync.resume_window`. |
| Global sequence | `seq` | Gapless, commit-ordered number of every durable event of the instance. |
| Grant | `grant` | Subject x target x level x capabilities x optional expiry date x reason; targets `instance`, `space`, `channel` and `host` (there without a level, only `terminal`); revocation sets `revoked_at`, the row stays. |
| Group | `group` | Named set of people as the subject of grants; not an acting principal. |
| Heartbeat | `heartbeat` | Regular sign of life of a client or agent; for agents with state and reported mode. |
| Host | `host` | Connected computer (macOS, Linux, Windows, also a dev VM) with the host service; target of grants with the capability `terminal`. |
| Host assignee | `hosts.assigned_user_id` | Person to whom a host is assigned; as a non-owner, only they get a terminal there (SR10). |
| Host fingerprint | `key_fingerprint` | Check value over the Noise key and the authentication key of a host; the owner types its beginning during pairing ([24-terminal.md](24-terminal.md), section 4.3). |
| Host pairing code | `host pairing code` | One-time code `bothy_hpc_...` of the owner with which the host service registers its public keys with the server. |
| Host service | `bothy-host` | Separate program on a host that holds the keys of the host as a service user without administrator rights and talks to the server only outbound; for each person a PTY helper starts the PTYs as their session user. |
| Idempotency key | `Idempotency-Key` | UUIDv7 in the header of write requests without their own client ID; a repetition returns the stored response. |
| Identity | `user_identity` | Link of a person to a subject at an identity provider. |
| Identity provider | `identity_provider` | Login driver: OIDC, trusted proxy header or local account (passkey, enrollment code, device pairing, password with TOTP as a fallback) (B9). |
| Import source | `import_source` | Configured source of an import (for example a Discord server) with state and progress. |
| Imported item | `import_item` | One imported object with a unique origin key `origin_key` per source. |
| Incident | `status_incident` | Entry of the status page about an outage or a maintenance. |
| Incoming webhook | `incoming_webhook` | Secret URL through which an external system writes into a channel as an app; never triggers agents. |
| Instance | `instance` | One installation; one organization; one server in the Discord sense (B13). |
| Interaction | `interaction` | Use of a component by a principal; authorized like an agent invocation and effective at most once. |
| Invariant I1 | `I1` | With `billing_mode = personal`, exactly the `credential_holder` has effective `trigger` on the targets of the agent; checked transactionally. |
| Invariant I2 | `I2` | Check immediately before every agent invocation that, with `personal`, the triggering person is the `credential_holder`; fail closed. |
| Invariant I3 | `I3` | Signed invocation token; the adapter starts only if its reported mode matches the configured mode of the token. |
| Invariant I6 | `I6` | With `hosts.llm_credentials = personal`, only the holder of the login opens a terminal on the host, not even another owner; checked transactionally and on every opening. |
| Invocation token | `invocation token` | Short-lived JWS (`typ = agent-invocation+jwt`) that the server signs with EdDSA per delivery, for exactly one agent invocation with trigger, target, level, run, backend policy and tool class (I3). |
| Key pinning | `key pinning` | A device remembers the Noise key of a host at the first opening and warns hard on a change. |
| Level | `level` | Ascending permission level of a grant: `read` < `write` < `trigger`. |
| Level trigger | `trigger` | Highest level: may trigger agents and use components. |
| Live box | `live box` | Live-updated representation of a run in the thread; collapses into one line at the end. |
| Live frame | `stats.live` | Ephemeral gateway event with the current state of the runs, agents and rates, every `stats.live_interval` to connections with the subscription `stats.subscribe`; filtered to the visible channels, without content and without persons. |
| LLM login of the host | `llm_credentials` | Declaration of the owner per host: `none`, `personal` (with holder) or `organization`; basis of I6. |
| Local credentials | `local_credential` | Password and TOTP of a local account as a fallback that can be turned off. |
| Login flow | `auth_login_flow` | State of an OIDC login or linking between start, redirect back and redemption of the server code. |
| Main thread | `main thread` | Thread with `kind = main` that carries the continuous history of a text or DM channel; without a fixed operator. |
| Mention | `mention` | Naming of a person or agent in a message; no roles, no @everyone, no @here. |
| Message | `message` | Entry in a thread with thread sequence, origin, kind and Markdown content. |
| Message revision | `message_revision` | Earlier version of an edited message. |
| Minute aggregate | `stats_minutes` | Row of the own measurement: count, sum and good events of a metric per minute, channel, agent and code; retention class `stats`. |
| Noise stream | `noise stream` | End-to-end encrypted terminal stream according to `Noise_NK_25519_ChaChaPoly_SHA256` between device and host service; the server only forwards it. |
| Notification | `notification` | Push or ephemeral `notification.created` to a person; never carries content in the push. |
| Notification preference | `notification_preference` | Level `all`, `mentions` or `mute` of a person per space, channel or thread; without an own preference, the channel default, the personal default and the instance default apply. |
| Operator | `operator` | Exactly one agent or none per channel (inherited from the space, can be overridden); without an operator no agent answers. A thread keeps its operator from its creation, the main thread uses the effective operator. |
| Operator rule | `operator rule` | The effective operator of a channel and the fixed operator of a thread have at least `write` there, without a grant row. |
| Origin | `origin` | Source of a message: `human`, `agent`, `app`, `import`, `system`; only `human` triggers agents. |
| Outbox | `outbox` | Local queue of a client for write operations, each with its own client ID, strictly in order per thread. |
| Outgoing webhook | `webhook_subscription` | Subscription of an app or an agent to receive events, signed (`v1a`, optionally HMAC `v1`), at an allowed HTTPS address. |
| Own measurement | `own measurement` | Counting by the server itself through `kernel/measure` for live views and the SLO traffic light, as minute aggregates in `stats_minutes`; independent of the telemetry, with SLI thresholds from the Sloth files (docs/spec/25-visualization.md). |
| Owner | `owner` | Instance owner; may do everything except read other people's direct messages; owner permissions only in a session (exception: check without a request), never through a personal API token; at least one. |
| Passkey | `passkey_credential` | WebAuthn credentials of a person for local login and step-up. |
| Permission change preview | `access.preview` | Dry run of a permission action with differences, invariants and hints in plain language; without effect. |
| Permission version | `permission_version` | Counter that increases on every permission change and makes caches and open connections re-evaluate. |
| Person | `user` | Principal of a person. |
| Pin | `pin` | Marking of a message so that it can be found at the top of the thread. |
| Price table | `price_table` | Prices per provider, model and unit with source and fetch time. |
| Principal | `principal` | Who acts: person (`user`), agent (`agent`) or app (`app`); `anonymous` only for public actions; a host (`host`) only with `host.connect` through its authentication key, without a row in `principals`. |
| Privacy request | `gdpr_request` | Job for export, deletion or rectification of the personal data of a person. |
| Project binding | `project_binding` | Binding of a forum to a dev project that a runner agent works on as the operator. |
| PTY helper | `bothy-host pty-helper` | Part of the host service that runs once per session user as this user, starts and ends PTYs and knows no keys; a system service that no session user can change or replace. |
| Push registration | `push_registration` | Push target of a device at a provider; push never carries content. |
| Quiet hours | `quiet_hours` | Time window without push for a person. |
| Rate-limit class | `rate-limit class` | Named group of actions with a shared token bucket (for example `rl.message`). |
| Reaction | `reaction` | Free Unicode character on a message; not a status. |
| Read state | `read_state` | Highest read thread sequence of a person per thread; only increases, except through an explicit reset. |
| Read-only | `read_only` | State of a channel in which no principal, not even an owner, writes; only a running import writes (read mirror). |
| Receive action | `receive_action` | Action that a receiver needs on the resource of an event in order to get it. |
| Redaction | `redaction` | Removal of the payload of an event after deletion; the envelope stays. |
| Refresh token family | `refresh_token_family` | Chain of rotating refresh tokens of a session, bound to the DPoP key; a reuse outside the grace period `auth.refresh_reuse_grace` revokes the whole family. |
| Relay | `relay` | Part of the module `terminal` that connects the client and host data connections of a session in the memory of one node, without reading content. |
| Reply with quote | `reply_to_message_id` | Reference of a message to an earlier message of the same thread. |
| Reported mode | `reported_billing_mode` | Actual backend policy reported by the adapter, from the heartbeat and per run; missing for adapters that follow the configured mode per run (then `billing_modes_supported`). |
| Resync required | `resync_required` | Response of the server when catch-up is not possible; the client loads a snapshot. |
| Retention class | `retention class` | Name of a deletion rule (for example `content`, `security`); duration and method are in docs/spec/20-privacy.md. |
| Run | `run` | One agent turn with state, steps, metrics and costs; at the same time the live box. |
| Run context | `run_context` | Context of a run built by the server, with a trust mark per message; the adapter fetches it with `run.context.read`. |
| Run document | `run_document` | JSON representation of a run to which live box patches are applied. |
| Run step | `run_step` | Ordered part of a run: tool call, status line, subagent, workflow or background job. |
| Running now | `run overview` | Overview of all active runs that a person may see. |
| Scope | `scope` | Access area granted to an app in addition to its grants. |
| Session | `auth_session` | Login session of a person on a device (app, extension as a child session, or console); its revocation takes effect immediately on all tokens. |
| Session user | `host_service.session_users`, `host_session_users` | Operating system user on a host under which the terminal sessions of exactly one person run; never shared, not even after the end of the assignment (ruling T12); no other session user reaches its places (home, work folder, `CLAUDE_CONFIG_DIR`) (ruling T13); without read permission on the keys of the host service. |
| Sidebar state | `user_space_state` | Personally collapsed spaces and own order, across devices. |
| Signing key | `server_signing_key` | Key of the server for access tokens, invocation tokens, terminal session tokens, outgoing webhooks and audit checkpoints; private part only in the secret provider. |
| Skill binding | `channel_skill_binding` | Ordered skill reference that the operator loads in a channel. |
| Slash command | `command` | Command registered by agents or apps, with options and autocomplete. |
| Snapshot | `snapshot` | State that the server delivers on `ready` or on `resync_required`, without thread content. |
| Space | `space` | Top-level group of the sidebar with a global order and an optional default operator. |
| Status page | `status page` | Display of the operational state with components and incidents. |
| Step-up | `step_up` | Fresh confirmation by the person (passkey, step-up device key with biometrics or PIN, password with TOTP, or a new OIDC login) that administrative actions require. |
| Sync epoch | `sync_epoch` | Identifier of the timeline of the event log; new after every restore, clients store it with the cursor. |
| System action | `system action` | Action in the catalogue that only the server itself executes (job, server command), for example `grant.expire`. |
| Template | `message_template` | Stored text for frequent inputs, personal or per channel. |
| Terminal profile | `terminal profile` | Start of a session defined in the configuration of the host service (`shell`, `claude`, fixed commands); the server knows only name and kind. |
| Terminal session | `terminal_session` | One PTY on a host for a person, with state `opening`, `active`, `detached` or `closed`; only the device and the host know the content. |
| Terminal session token | `terminal session token` | Short-lived JWS (`typ = terminal-session+jwt`) signed by the server with EdDSA that allows exactly one terminal session on a host, exactly once. |
| Thread | `thread` | Unit of a conversation and of an agent session with a fixed ID, title, title source and state. |
| Thread sequence | `thread_seq` | Gapless number of every message in its thread; determines the display order. |
| Thumbnail | `file_thumbnail` | Reduced representation of a file in a fixed variant. |
| Title source | `title_source` | `auto` or `manual`; a manually set title is never overwritten automatically. |
| Tombstone | `tombstone` | Kept row of a deleted message without content; keeps the thread sequence gapless. |
| Tool class | `tool_class` | What the runtime of an agent may do: `chat` < `scoped` < `host`; configured on the agent, reported in the heartbeat. |
| Transcript | `transcript` | Text of a voice message, created by the agent or a transcription service. |
| Trigger kind | `trigger_kind` | Origin of a run or a token: `message`, `followup`, `interaction`, `command`, `autocomplete`. |
| Trigger state | `trigger_state` | State on the triggering message: `received`, `queued`, `working`, `done`, `failed`, `cancelled`, `followup_delivered`; replaces status emoji; the reason is in `trigger_state_reason`. |
| TV mode | `tv mode` | Full-screen display of the visualizations without controls, in the session of a person, costs hidden by default. |
| Typing indicator | `typing` | Ephemeral event that a person is writing in the thread. |
| Upload session | `file_upload_session` | Resumable upload of a file in parts, the same for both storage drivers. |
