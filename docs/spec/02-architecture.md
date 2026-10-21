# 02 Architecture

Status: draft M0. Normative for the structure of `services/server/`.
Terms: [00-glossary.md](00-glossary.md). Tables: [03-data-model.md](03-data-model.md).
Events and sync: [04-sync-protocol.md](04-sync-protocol.md). Actions:
[05-actions.md](05-actions.md). Decisions: ADR 0004 to 0009, see the
[ADR index](../adr/README.md).
Diagrams: [../architecture/README.md](../architecture/README.md).

Notation: REST paths are written without a version prefix; [06-api.md](06-api.md) defines the prefix.
Limits, deadlines and targets appear in the text only by name. The default values of this chapter are
in section 22, all others in the chapter that owns the name.

## 1. Guardrails

| Guardrail | Origin | Consequence for the architecture |
|---|---|---|
| An instance first runs on a single host, later in a cloud. | B1, B12 | One binary with roles. PostgreSQL and object storage are the only state. No broker, no cache service, no service mesh. |
| Permissions apply to every request and every delivery. | B7, 05 | A single decision point `authorize` in the module `authz`. Every route proves that it has checked its action (section 9.3). |
| No event is lost, no write request takes effect twice. | B8, 04 | Commit-ordered `event_log`, cursors, idempotency in the same transaction (ADR 0009). |
| Agents, plugins and apps never run in the server process. | B6, B10 | The public API is the only extension point. The server knows agents only as principals, runs only as objects. |
| Infrastructure is replaceable. | B3, B4, B9, B11, B12 | Hexagonal modules: domain logic knows only ports, drivers are adapters (ADR 0008). |
| Operation by one person must be enough. | B1 | Few parts, one configuration path, failure modes with a clear response (section 14). |

## 2. Context (C4 level 1)

Diagram: [c4-1-context.svg](../architecture/c4-1-context.svg)
(source [c4-1-context.mmd](../architecture/c4-1-context.mmd)).

| Party | Kind | Interface to the instance | Trust |
|---|---|---|---|
| Member | person | apps over REST and the gateway WebSocket | only own grants |
| Owner | person | apps and web console, administration with step-up | everything except other people's direct messages (05, R-AZ-02) |
| Instance operator | person | containers, configuration, server commands `bothy ...` | host access; no access through the API without own grants |
| Bot developer | person | public API, SDK, app manifest | none; her apps get scopes and grants |
| Identity provider | external | OIDC (auth code with PKCE), JWKS of the provider | for the identity, after the signature check |
| Reverse proxy | external, run by the instance operator | HTTPS, WebSocket, optional identity headers | headers only when the server listens exclusively on loopback (B9) |
| Push services | external | HTTPS from the worker | not trusted; receive only IDs and placeholders |
| Agents (adapter with runtime) | external to the server | gateway as agent principal, REST, invocation token | not beyond their grants; check the token themselves (I3) |
| AI providers | external | used only by the runtime, never by the server | the server stores no credentials (B10, I5) |
| Integrations | external | REST with app token, incoming webhooks, outgoing webhooks | scopes and grants (B6) |
| Telemetry backends | external | OTLP through a collector | receive no content (B11) |
| Discord | external | read-only during import | source, no write-back |
| Transcription service | external, optional | port in the worker | receives audio only when the switch `voice.enabled` is on |
| Host service `bothy-host` | external, on connected hosts | outbound only: WebSocket control connection and one data connection per session ([24-terminal.md](24-terminal.md)) | authenticated with its own key; the server sees no terminal content (Noise) |

## 3. Containers (C4 level 2)

Diagram: [c4-2-container.svg](../architecture/c4-2-container.svg)
(source [c4-2-container.mmd](../architecture/c4-2-container.mmd)).

### 3.1 Containers

| Container | Technology | Responsibility | State | Scaling |
|---|---|---|---|---|
| `bothy serve --role=api` | Go, one binary, OCI image | REST, gateway WebSocket, log tail and fan-out in the process, agent consumers | only transient in memory (connections, permission cache, rate-limit buckets) | horizontal from level 2 (section 16) |
| `bothy serve --role=worker` | same binary and image | River jobs, event consumers (push, webhooks), file processing, maintenance | none outside the database | horizontal from level 3 |
| PostgreSQL 18 | database | all domain data, `event_log`, audit, River tables in the schema `jobs.schema` | authoritative | vertical; read replica from level 4 |
| Object storage | file system driver (default) or S3-compatible | files, thumbnails, export packages | authoritative for bytes | through the storage service |
| ClamAV `clamd` | separate process, optional | malware scan via `INSTREAM` | signatures | own container |
| OTel Collector | separate process | accepts OTLP, exports per backend | buffer | own container |
| Apps | Flutter (iOS, Android, macOS, Windows) | chat, outbox, local database, Notification Service Extension | local cache, never authoritative | per device |
| Web console | Flutter Web | administration, statistics and terminal (B5, [ADR 0070](../adr/0070-terminal-in-the-web-console.md)); no chat | none | static files in the server image, served by the api process under `/console/` from `console.assets_dir` ([16-operations.md](16-operations.md) 14; M6-02) |
| Agent adapter | outside the server (reference: Hermes agent plugin, Claude Code runner) | consumer with a durable cursor, checks the invocation token, reports runs | own | run by whoever runs the runtime |
| Host service `bothy-host` | separate Go binary (`services/host`), on every connected host | PTY per terminal session, Noise peer, profiles, self-check ([24-terminal.md](24-terminal.md)) | keys and scrollback buffer on the host | per host |

The secret source (file, environment, later Key Vault or Secrets Manager) is not a separate container
in the diagram. api and worker read it at start and on rotation (section 13).

### 3.2 Roles of a process

| Part | `api` | `worker` |
|---|---|---|
| REST endpoints, gateway WebSocket | yes | no |
| Log tail, hub, permission cache for connections | yes | no |
| Agent consumers (gateway connections of adapters) | yes | no |
| Event consumers `push` and `webhooks` | no | yes |
| River: insert jobs | yes | yes |
| River: run jobs, periodic jobs, leader election | no | yes |
| internal listener `server.internal_listen_addr` (health, incoming webhooks from the stack) | yes | yes |

Role `all` combines both in one process and is the default (`server.role`). Reason: an
installation on one host needs exactly one server container. The split costs only one
line of configuration, because both roles use the same binary and the same database.

### 3.3 Commands of the binary

One binary `bothy` with subcommands (ruling L1). This table is the register; other chapters
name only commands from here.

| Command | Purpose | Chapter |
|---|---|---|
| `bothy serve --role=api\|worker\|all` | start the server | this chapter |
| `bothy migrate up [--bootstrap]\|status\|validate` | migrations of the domain schema (goose) and of the River tables (rivermigrate); on the first run creates the row `instance` (through the port of the module `instance`), sets `instance.schema_version` and `instance.min_server_version`; creates the first partitions of `event_log` (03). `up --bootstrap` first creates or updates the five database roles and their grants as `migrate.bootstrap_user` (16 "Roles"); `status` only reads, `validate` needs no database | ADR 0007, [16-operations.md](16-operations.md) |
| `bothy setup bootstrap-owner\|issue-enrollment-code` | setup: first owner by server command (system action `instance.bootstrap`), enrollment code for recovery | [08-identity.md](08-identity.md) |
| `bothy config check\|print` | check the configuration, print it redacted; `check --secrets` also resolves every reference `*_ref` through the secret provider and names per key only `ok` or the error code | section 13 |
| `bothy import login\|logout`, `bothy import discord ...` | login of the import tool as a `cli` device through device pairing; import and read mirror through the import API (`import.read`, `import.apply`) | [22-import-migration.md](22-import-migration.md) |
| `bothy restore finalize [--relocation <manifest>]` | after a restore: new sync epoch, cursors to `head_seq`, stored database identity (R-DM-50), audit, in one transaction; as the schema owner and only while no `bothy_app` session is connected (exit `1` otherwise). `--relocation` takes the manifest of the final backup of a relocation without data loss: when `event_log.head_seq`, the audit head and `sync_epoch` equal it, only the identity is stored and epoch and cursors stay; a difference is exit `1` naming the field | [16-operations.md](16-operations.md) 8.4, [03-data-model.md](03-data-model.md) 6.7 |
| `bothy secrets init\|export` | create and back up secrets: `init --target DIR [--source DIR] [--owner SERVICE=UID:GID ...] [--adopt-source]` reconciles the store `_store/` with the source and copies per service; `export --from DIR --age-recipient KEY [--age-recipient ...] [--signing-key-dir DIR] [--output FILE\|-] [--armor]` writes the age-encrypted bundle (with the key store under `signing-keys/`); both read only their flags, without a configuration file and database | [16-operations.md](16-operations.md) 5.3, 5.4 |
| `bothy healthcheck --url URL [--timeout DURATION]` | health check against the internal listener for containers without a shell. `--url` takes only `http://` with a loopback host (`127.0.0.0/8`, `::1`, `localhost`), anything else is exit `2`; `--timeout` bounds the whole request (default `5s`, the timeout of the Compose healthcheck); no configuration, secrets or database; exit `0` for `2xx`, else `1` with one line naming the status code or the error class | [16-operations.md](16-operations.md) |
| `bothy files rescan` | maintenance: have files checked again | [12-files.md](12-files.md) |
| `bothy telemetry pseudonym <principal_id> [--at TIME] [--config FILE]` | pseudonym of a principal for the owner's troubleshooting: prints `bothy.principal.pseudo_id` for the period that contains `--at` ([RFC 3339](https://www.rfc-editor.org/rfc/rfc3339), retrieved 2026-10-12, default now) and the start of that period in UTC; reads `telemetry.pseudonym_key_ref` through the secret provider, no database | [../observability/privacy.md](../observability/privacy.md) |
| `bothy dev seed` | test data, only in `instance.environment = dev` | [../operations/environments-and-flags.md](../operations/environments-and-flags.md) |

Exit codes of all commands: `0` success or orderly end; `1` error (start failed, fatal
runtime error, shutdown beyond `shutdown.total_timeout`, negative check result); `2` invocation or
configuration error. From `3` on, codes are per command and are listed in its help: `serve` `3` for a
forced end by a second signal (section 15.3); `setup bootstrap-owner` `3` when the
setup is already complete; `setup issue-enrollment-code` `4` when it is not yet complete
([08-identity.md](08-identity.md)). Reason: orchestrators and scripts tell "called
incorrectly" apart from "failed at runtime" and recognize expected states without comparing text.
Output: what a script is meant to read (for example the enrollment code from `setup`) stands alone on
standard output; logs go to standard error.

Migrations never run implicitly at the start of `serve` (R-AR-16). Reason: with several nodes they would
start at the same time, and a failed step should not keep operation in a
restart loop.

## 4. Modules of the monolith

Diagram: [c4-3-server-components.svg](../architecture/c4-3-server-components.svg)
(source [c4-3-server-components.mmd](../architecture/c4-3-server-components.mmd)).

### 4.1 Rules

1. A module is the package tree `services/server/internal/<module>/`. It owns tables. Only
   this module writes and reads them via SQL (03, principle 12). Other modules go through its
   port.
2. The port is the root package `internal/<module>` (section 5). Other modules import only
   this package, never `domain`, `app` or `adapters` of another module.
3. Dependencies between modules form a directed graph without cycles (table 4.4).
4. If a lower module needs data or reactions of a higher one, there are exactly two ways:
    - **Source port:** The lower module declares an interface in its port; the higher
      module implements it in an adapter. Example: `authz` declares `StructureSource`,
      `chat` implements it.
    - **In-tx event:** The lower module publishes a domain event in the running
      transaction; the higher module subscribes to it (section 8.3). Example: `messaging`
      publishes `MessagePosted`, and `runs` then decides on a run.
5. Every module may use the cross-cutting ports (`internal/kernel`). They are small, stable and each
   implemented by exactly one module (table 4.5).
6. Every table from 03 belongs to exactly one module. Every action from 05 is executed by exactly one
   module as a use case. The permission logic of every action still lies only in `authz` (B7).
7. Two modules own no tables: `gateway`, `search` (03, section 2; ruling L3). They
   are separate modules because they have their own ports, their own dependencies and their own
   performance targets. `stats` was also planned without tables and, since the addendum of 2026-10-10,
   owns the minute aggregates of the own measurement (`stats_minutes`, ruling T7,
   [25-visualization.md](25-visualization.md)).

### 4.2 Module list

| Module | Responsibility | Owns tables | Chapter |
|---|---|---|---|
| `instance` | instance identity, sync epoch, minimum server version, settings and branding, feature flags, server signing keys with JWKS and rotation, sequence counters | `instance`, `instance_settings`, `server_signing_keys`, `feature_flags`, `sequence_counters` | 06, 16, 19 |
| `audit` | audit log with hash chain, signed checkpoints, keys of the IP pseudonyms, query, verification, truncation with anchor | `audit_events`, `audit_checkpoints`, `audit_ip_keys` | 19, 20 |
| `authz` | `authorize`, permission cache, visible targets, groups, grants, permission version, I1 check, "Who has access", "View as person" | `groups`, `group_memberships`, `grants` | 07 |
| `billing` | price table, cost postings, totals per run, thread, space, agent, reconciliation | `price_table`, `cost_entries` | 10 |
| `identity` | principals, people, owners, login drivers, identities, local credentials, enrollment codes, login flows, device pairing, passkeys, devices and device keys, sessions, refresh families, API tokens, step-up, access tokens | `principals`, `users`, `owners`, `identity_providers`, `user_identities`, `local_credentials`, `enrollment_codes`, `auth_login_flows`, `auth_challenges`, `device_pairings`, `passkey_credentials`, `devices`, `device_keys`, `auth_sessions`, `refresh_token_families`, `refresh_tokens`, `api_tokens` | 08 |
| `chat` | spaces, sidebar state, channels with behavior, skills, project binding, DM participants, forum tags, threads with thread sequence | `spaces`, `user_space_states`, `channels`, `channel_skill_bindings`, `project_bindings`, `dm_participants`, `forum_tags`, `threads`, `thread_tags` | 01, 07, 09 |
| `agents` | agents, backend policy and tool class (configured), reported state, consumer cursors with fencing, command register, agent sessions, dev environments (milestone) | `agents`, `agent_consumer_cursors`, `commands`, `agent_sessions`, `dev_environments` | 09, 10, 21 |
| `files` | upload sessions in parts, object storage drivers, scan, metadata removal, thumbnails, attachments, transcripts, file text extracts | `files`, `file_upload_sessions`, `file_upload_parts`, `file_thumbnails`, `message_attachments`, `file_text_extracts` | 12, 14 |
| `messaging` | messages, revisions, mentions, reactions, pins, templates, channel export | `messages`, `message_revisions`, `mentions`, `reactions`, `pins`, `message_templates` | 06, 14 |
| `runs` | trigger decision, runs and run document, steps, follow-ups, components, interactions, command execution, invocation tokens | `runs`, `run_steps`, `run_followups`, `message_components`, `interactions` | 09, 10 |
| `sync` | event log (append, tail, catch-up, redaction, partitions), snapshot, idempotency keys, read state, drafts, personal settings, cursors of the worker consumers | `event_log`, `event_consumer_cursors`, `idempotency_keys`, `read_states`, `drafts`, `user_settings` | 04, 15 |
| `notify` | notification levels, quiet hours, push registrations, push consumer and dispatch | `notification_preferences`, `quiet_hours`, `push_registrations` | 13 |
| `gateway` | WebSocket endpoint, gateway sessions, hub, focus, typing, forwarding of ephemeral events, agent connections, shared presence state of several nodes | `gateway_presence` | 04, 06 |
| `search` | search query over messages, thread titles and files with a permission filter, ranking | none | 14 |
| `stats` | statistics for console and apps: usage per space, runs, tool calls, costs, error rates; own measurement (counters in memory, flush, retention), SLO traffic light, live frame `stats.live`, timeline | `stats_minutes` | 01, 17, 25 |
| `apps` | apps and manifests, installations, scopes, outgoing webhooks with consumer and delivery, incoming webhooks | `apps`, `app_installations`, `webhook_subscriptions`, `webhook_deliveries`, `incoming_webhooks` | 11 |
| `importer` | import sources, batches with origin key (`import.apply`), ownership list and read-only state of mirrored channels; the dry run lives in the tool | `import_sources`, `import_items` | 22 |
| `privacy` | privacy requests, export and deletion through the participants of all modules, deletion journal | `gdpr_requests`, `deletion_journal_state` | 20 |
| `status` | status page, components, incidents, automatic checks, backup runs | `status_components`, `status_incidents`, `status_incident_updates`, `status_incident_components`, `backup_runs` | 16, 17 |
| `terminal` | hosts and pairing, declaration of the LLM login (I6), session users per person (T12), terminal sessions, terminal session tokens, control connections of the host services, relay of the streams | `hosts`, `host_session_users`, `terminal_sessions` | 24 |

Technical tables outside the modules ([03-data-model.md](03-data-model.md), section
4.13): River creates its own tables with the prefix `river_` (among others `river_migration`, `river_job`,
`river_leader`, `river_queue`) [Q6, verified]. They live in the schema `jobs.schema` and belong to
`internal/platform/jobs`. The version table of goose lives in the same schema as the
domain tables.

### 4.3 Ports, events and sources per module

"Port" names the inbound interfaces in the root package (excerpt; the names are proposals for the
build). "Emits" are in-tx events (Go types, never serialized). "Sources" are interfaces
of lower modules that the module implements.

| Module | Port (excerpt) | Emits (in-tx) | Subscribes (in-tx) | Implements sources |
|---|---|---|---|---|
| `instance` | `Settings`, `Flags`, `Keys` (signer per purpose, `Signers` and `PublicKeys` of a purpose for the outgoing webhooks, JWKS, rotation), `Counters` (only for the phase hooks of `authz`, `sync`, `audit`), `Versions` (read counter values without a lock, for example `permission_version` for the permission cache, `event_seq` for catch-up); declares `LoginOptionsSource` (login drivers and password path for the discovery), `FlagUserSource` (people of a rollout), `FlagGroupSource` (valid groups of a person for the evaluation) | | | |
| `audit` | `Log` (query, verification), `RowsVerifier` (verification of a supplied chain, M7-03) | | | kernel `auditing.Recorder` |
| `authz` | `Admin` (groups, grants, reconciliation of the claim groups, explanation, preview, view as person, revocation of all grants of a subject in the caller's permission change), `AppScopes` (scope table for the event subscriptions of an app), `I1Resolver` (guided flow `resolve_i1`, M2-21), `Recipients` (`Readers`: the candidate readers of a channel for hints, a superset that `authorize` decides exactly, never authorized itself, M3-19); declares `PrincipalSource`, `CredentialSource`, `StructureSource`, `AgentPolicySource`, `AgentPolicySwitch` (M2-21), `HostSource` | kernel `authorize.AccessChanged` (with the triggering action; `runs` cancels with reason `user_disabled` on `user.disable`, otherwise with `access_revoked`) | | kernel `authorize.Authorizer`, `instance.FlagGroupSource` |
| `billing` | `Costs` (posting, correction, totals), `Prices` | | | |
| `identity` | `Auth` (login, refresh, logout, child sessions, step-up), `Directory`, `Devices` (with `ActiveForUsers`, the active devices of people for the consumer `push`, M3-19; every insert of a device row writes the durable event `device.pending` in its transaction when the device is `pending`, R-AZ-31), `Tokens`, `Owners`, `PrincipalWrites` (principals of agents and apps); declares `AvatarFiles`, `AppInstallations` (active installation of an app) | `DeviceRevoked`, `SessionEnded` | | kernel `actor.Authenticator`, `authz.PrincipalSource`, `authz.CredentialSource` (session, device, token for the check `check_active`, 07 section 4.2), `instance.LoginOptionsSource`, `instance.FlagUserSource`, kernel `personaldata.Participant` |
| `chat` | `Structure` (spaces, channels, read-only), `Threads` (create for `messaging`, `AppendToThread`, message count on deletion, close, open, move, info, `UnreadCandidates`: the threads of channels that can be unread, for the badge of 13 8, never authorized, M3-19), `ThreadsByParent` (the thread that started from a message, for the replay of an incoming webhook post in `apps`), `AgentConfig` | `ThreadMoved`, `ChannelArchived`, `ChannelReadOnlySet`, `SpaceArchived` | | `authz.StructureSource`, kernel `personaldata.Participant` |
| `agents` | `Agents` (administration, policy, tool class), `Presence`, `Consumers` (cursor, ack, takeover with fencing), `Commands` (register), `Sessions`, `DevEnvironments` (milestone); declares `ConsumerFeed` (the stream of the REST consumer operations, M2-07) | `AgentDisabled` | | `authz.AgentPolicySource` (policy, tool class, readiness of the dev environment), `authz.AgentPolicySwitch` (the switch to `organization` of the guided flow, M2-21), kernel `personaldata.Participant` |
| `files` | `Uploads` (sessions, parts), `Files` (read, open, preview, file tickets, delete), `Attachments`, `Transcripts`, `TextExtracts`; declares `AttachmentLocator` | `FileFinalized` (scan and, if applicable, transcript final) | `chat.ThreadMoved` (copy `message_attachments.channel_id`) | `identity.AvatarFiles`, kernel `personaldata.Participant` |
| `messaging` | `Messages` (create, create a thread with its first message, edit, delete, history, `SetTriggerState`, anchor; the reads of the consumer `push` `LastAnchorBefore`, `AnchorOf`, `LastAgentMessageBefore` and `Newest`, never authorized, M3-19), `Reactions`, `Pins`, `Templates`, `Mentions` (list, unread per thread), `Export`; declares `ComponentReader` | `MessagePosted`, `MessageDeleted` | `chat.ThreadMoved` | `files.AttachmentLocator`, kernel `personaldata.Participant` |
| `runs` | `Runs` (read, context, start, resume, patch, end, cancel, overview), `Delivery` (delivery preparation with I2, guard rules and token; called by the agent consumer engine of `gateway` before every invocation-bearing event, on the WebSocket and in the REST poll `pollAgentEvents`, which the same engine serves, M2-08), `Interactions`, `CommandExec` (including autocompletion, M2-17); declares `AutocompleteRelay` (M2-17) | | `messaging.MessagePosted`, `messaging.MessageDeleted`, `files.FileFinalized` (write the held-back invocation), `agents.AgentDisabled`, `chat.ChannelReadOnlySet`, kernel `authorize.AccessChanged` | `messaging.ComponentReader`, kernel `personaldata.Participant` |
| `sync` | `Tail`, `CatchUp`, `Snapshot`, `Consumers` (worker cursors, with `ConsumerLossHandler` for a retention loss), `LogReader` (one event by `seq` and the smallest retained `seq` for delivery jobs), `History` (`ResourceEvents` and `PersonalEvents`: retained events of one resource or one audience in commit order for worker consumers, M3-19), `ReadStates`, `ReadPositions` (`PositionsFor` and `PositionsOfUser`: stored read positions of people the caller selected, M3-19), `Drafts`, `Settings` | | `messaging.MessagePosted` (clear the draft), `chat.ThreadMoved` (copies of `channel_id`) | kernel `eventlog.Appender`, kernel `eventlog.Redactor`, kernel `idempotency.Store`, kernel `personaldata.Participant` |
| `notify` | `Preferences`, `QuietHours`, `PushRegistrations` (`Active`: the active registrations of people without token, M3-19); owns the worker consumer `push` (a `sync.ConsumerHandler`, 13 12, M3-19) | | `identity.DeviceRevoked`, `identity.SessionEnded` (deactivate the registrations of the session), `chat.ChannelArchived`, `chat.SpaceArchived`, `messaging.MessagePosted` (automatic following of 13 3.2, after the subscribers of `runs` and `sync`, M3-19) | kernel `personaldata.Participant` |
| `gateway` | none for other modules | | | kernel `eventlog.EphemeralPublisher`; implements sources: `agents.ConsumerFeed` (M2-07), `runs.AutocompleteRelay` (M2-17) |
| `search` | `Search` | | | |
| `stats` | `Stats` (queries, timeline, traffic light, backend view), `Live` (base frame per interval, filter per receiver) | | | kernel `measure.Recorder` |
| `apps` | `Apps`, `Webhooks` (`apps.Webhooks`: subscriptions, deliveries, redelivery, `ResumeInstallation`), `IncomingWebhooks` | | `chat.ChannelArchived` | `identity.AppInstallations` |
| `importer` | `Imports` (sources, batches through `import.apply`) | | | |
| `privacy` | `Requests` | | | collects all kernel `personaldata.Participant` |
| `status` | `Status`, `BackupRuns` (for `bothy-backupd`) | | `agents.AgentDisabled` | |
| `terminal` | `Hosts` (administration, pairing, declaration), `Sessions` (open, attach, close, list), `Relay` (stream and host connections, only in the adapter `wsrelay`) | | kernel `authorize.AccessChanged`, `identity.DeviceRevoked`, `identity.SessionEnded` (end sessions; M2-46: `app.Subscribers`, bound in the terminal step of `internal/wire`, D-M2-46-16) | `authz.HostSource` (the host's state, assignee, LLM declaration, server host) |

### 4.4 Allowed dependencies

"May use" means: code of this module may import the port of the named module. Everything
else is forbidden and fails in the gate (section 7).

| Module | May use | Reason per edge |
|---|---|---|
| `instance` | nothing | foundation |
| `billing` | nothing | cost rows carry copies of thread, channel, space and agent (03); no lookup needed |
| `audit` | `instance` | counter `audit_seq` |
| `authz` | `instance` | counter `permission_version`; all other data through source ports |
| `identity` | `instance`, `authz` | signing keys for access tokens; implements `PrincipalSource` and `CredentialSource` |
| `chat` | `identity`, `authz` | the operator is an active agent principal, DM participants are people; implements `StructureSource` |
| `agents` | `identity`, `authz`, `chat` | create the agent principal, the credential holder is a person; implements `AgentPolicySource`; thread for sessions, where commands apply |
| `files` | `identity`, `chat` | implements `AvatarFiles`; check the upload channel; subscription `chat.ThreadMoved` for the copy `message_attachments.channel_id` |
| `messaging` | `identity`, `chat`, `files` | resolve mentions; thread sequence and reopening; bind and read attachments |
| `status` | `agents`, `audit` | agent presence as a status component; the restore test verifies the audit chain of the restored copy with the verification of `audit.verify` (`audit.RowsVerifier`, M7-03) |
| `runs` | `instance`, `chat`, `agents`, `messaging`, `billing` | sign the token; operator, mention rule, project binding; policy and session; anchor and `trigger_state`; post costs |
| `search` | `chat`, `messaging`, `files` | thread titles, messages, file names and transcripts |
| `importer` | `identity`, `chat`, `agents`, `messaging`, `files` | creates people, structure, messages, earlier session references and files; sets the read-only state of the mirrored channels |
| `privacy` | `identity`, `files` | data subject; store the export package |
| `sync` | `instance`, `identity`, `chat`, `agents`, `messaging`, `runs` | counter `event_seq`; principal of the snapshot (handle, display name); snapshot from structure, presence, active runs; subscription to messages |
| `stats` | `chat`, `agents`, `messaging`, `runs`, `billing` | reads metrics through ports |
| `notify` | `identity`, `chat`, `messaging`, `sync`, `authz` | devices and time zone; target path for settings; mentions; read state; candidates with read permission for notifications |
| `apps` | `instance`, `identity`, `authz`, `chat`, `agents`, `messaging`, `sync` | webhook signing key; app principal and tokens; revoke the grants of the app at the uninstall; scope table for event subscriptions; target of the incoming webhooks; commands of apps; messages from webhooks; consumer for outgoing webhooks |
| `gateway` | `chat`, `agents`, `runs`, `sync`, `stats` | thread to channel for focus and typing; consumer cursor and presence; run patches and delivery preparation; tail, catch-up, snapshot; live frame for `stats.subscribe` |
| `terminal` | `instance`, `identity`, `authz`, `agents` | signer for terminal session tokens (purpose `terminal`) and `instance_id` of the `pairHost` response; people, devices, owners, consumption of the step-up, pairing codes (format and HMAC through `identity`, M2-41); grants on hosts and I6; readiness of the dev environment (milestone) |

Tiers (from the bottom): `instance`, `billing` · `audit`, `authz` · `identity` · `chat` · `agents`,
`files` · `messaging`, `status`, `terminal` · `runs`, `search`, `importer`, `privacy` · `sync`,
`stats` · `notify`, `apps`, `gateway`. Every edge points downward.

In addition, for all modules: `kernel` yes, `platform` only in `adapters`, `api` (generated code)
only in `adapters`, River only in `platform/jobs`.

Until a later ticket builds the implementation of a source port, `internal/wire` wires that port
with an empty source that knows nothing; `authorize` then decides fail closed (no
structure, no active device). The ticket of the implementation replaces the empty source (PLAN,
tickets of the modules `identity`, `chat`, `agents`).

### 4.5 Cross-cutting ports (`internal/kernel`)

| Package | Purpose | Implemented by | Used by |
|---|---|---|---|
| `kernel/ids` | generate and check UUIDv7 (03, R-DM-01) | kernel itself | all |
| `kernel/clock` | time source, replaceable in tests | kernel itself | all |
| `kernel/errs` | error kinds with a stable code (section 14.1) | kernel itself | all |
| `kernel/textcheck` | check of display texts per SEC-IN-4 (invalid UTF-8, control characters, bidi control characters); no length check | kernel itself | `domain` packages of all modules with display texts |
| `kernel/reqmeta` | `correlation_id` (trace ID of the request, written as a UUID, not a UUIDv7) and client IP of the request in the context; standard library only | `platform/httpserver` sets the values | `audit`, `sync`, `platform/*` |
| `kernel/jose` | read, build and verify compact JWS (`ES256`, `EdDSA`, `RS256`, `PS256`), read JWK and compute its fingerprint per RFC 7638; standard library only ([ADR 0067](../adr/0067-jose-profile-with-the-standard-library.md)) | kernel itself | `identity` (DPoP, access tokens, step-up, ID tokens, client assertion), `instance` (JWKS, verification of checkpoints), `audit`, later `runs` |
| `kernel/designkeys` | allowed keys of the design system as data: category colors, accents, space icons; generated from `design/build/tokens.resolved.json` (PLAN M1-46) | generator | `instance` (branding), `chat` (spaces, tags) |
| `kernel/searchlang` | Validated language configuration of full-text search: `search.languages` and `search.locale_map`, the configuration of a locale's first language subtag, the query languages and every configuration the server uses ([14-search.md](14-search.md) 3.1, M3-13); standard library only | kernel itself | `identity` (language of new messages and threads), `wire` (checks, start check), later `search` (M3-14) and `files` (M3-15) |
| `kernel/actor` | principal of a request (ID, kind, device, session, step-up time, scopes); `Authenticator` with `Revalidate` (check an already verified principal again without new proof: session, device, token, status; closes gateway connections after the revocation of tokens and agents, which have no `session.revoked`) | `identity` | `platform/httpserver`, `gateway` |
| `kernel/authorize` | actions (generated from `docs/spec/actions.yaml`, [ADR 0049](../adr/0049-machine-readable-action-catalogue-as-the-single-source.md)), resource descriptor (with an optional second target, 07 section 4.2), `Decision`, `Authorizer` (`Authorize`, `Scope` for lists, `Observe` for events of the log), filter mode `Filter(ctx)` for elements of lists, search and snapshot (no denial in the audit, 07 section 4.2), delivery mode `Delivery(ctx)` of the gateway (filter mode plus permission snapshot from `Observe` instead of one read query per event), in-tx event `AccessChanged`. `Registry` collects the event types, WebSocket ops and terminal channel ops a binary emits and accepts; modules fill it through `Register` in their port, and the start checks it against the closed catalogue of `authz` (M1-25) | `authz` | all |
| `kernel/txn` | Unit of Work, phases before the commit, in-tx event bus (section 8) | `platform/pg`; phase hooks of `authz`, `sync`, `audit` | all `app` packages |
| `kernel/eventlog` | `Appender` for durable events, `Redactor` for redacting the payloads of a deleted message (03, section 6.5), `EphemeralPublisher` for ephemeral ones | `sync` or `gateway` respectively | all; `Redactor` only `messaging` (and `files` with M3) |
| `kernel/auditing` | `Recorder`: in the transaction or separately (for denials after a rollback) | `audit` | all |
| `kernel/idempotency` | `Store` for `Idempotency-Key` in the Unit of Work | `sync` | `platform/httpserver` |
| `kernel/personaldata` | `Participant`: export and deletion per module for a person | modules with tables of personal data | `privacy` |
| `kernel/measure` | `Recorder.Record(metric, key, value)` of the own measurement; only counters in memory, never blocking ([25-visualization.md](25-visualization.md), section 3) | `stats` | `messaging`, `runs`, `billing`, `agents`, `notify`, `gateway`, `platform/httpserver` |
| `kernel/presence` | `View`: the live gateway connections of people per device with focus and idle (13 12 point 4), and `None`, which reports no connection; a kernel port because `notify` and `gateway` are on the same top layer (M3-19) | `gateway` (M3-23; until then the push path of `notify` falls back to `None`) | `notify` |

`kernel/ids` checks only IDs per R-DM-01. `correlation_id` is not a UUIDv7 and is never checked with
`kernel/ids`; it comes only from `kernel/reqmeta` or the span context.

`kernel` depends on nothing in the project. Reasons: every module needs these ports; as module edges
they would create cycles (every module calls `authorize`, `authz` needs events and audit).

### 4.6 Actions per module

Every action from [05-actions.md](05-actions.md) belongs to exactly one module. A test compares this
table with the catalogue (section 7.3).

| Module | Actions |
|---|---|
| `instance` | `instance.discovery.read`, `console.csp_report`, `instance.read`, `instance.settings.read`, `instance.settings.update`, `instance.branding.update`, `feature_flag.read`, `feature_flag.update`, `signing_key.rotate`, `signing_key.transition` |
| `audit` | `audit.read`, `audit.verify` |
| `authz` | `access.explain`, `access.hints.read`, `access.preview`, `access.view_as`, `group.read`, `group.create`, `group.update`, `group.delete`, `group.member.add`, `group.member.remove`, `group.member.expire`, `group.member.sync`, `grant.read`, `grant.create`, `grant.revoke`, `grant.expire` |
| `identity` | `instance.bootstrap`, `auth.login.begin`, `auth.login.complete`, `auth.token.refresh`, `auth.logout`, `auth.step_up`, `identity_provider.read`, `identity_provider.manage`, `owner.add`, `owner.remove`, `owner.recover`, `self.read`, `user.read`, `user.create`, `user.update`, `user.disable`, `user.enable`, `self.profile.update`, `identity.read`, `identity.link`, `identity.unlink`, `local_credential.set`, `passkey.register`, `passkey.revoke`, `device.list`, `device.approve`, `device.revoke`, `device.expire`, `session.list`, `session.revoke`, `session.extension.create`, `token.list`, `token.create`, `token.revoke`, `access.view_as.end` |
| `chat` | `space.read`, `space.create`, `space.update`, `space.reorder`, `space.archive`, `space.operator.set`, `sidebar.update`, `channel.read`, `channel.create`, `channel.update`, `channel.reorder`, `channel.behavior.read`, `channel.behavior.update`, `channel.operator.set`, `channel.project.bind`, `channel.read_only.set`, `channel.archive`, `forum_tag.manage`, `dm.create`, `dm.leave`, `thread.read`, `thread.title.update`, `thread.close`, `thread.reopen`, `thread.move`, `thread.tags.set`, `agent.config.read` |
| `agents` | `agent.read`, `agent.create`, `agent.update`, `agent.disable`, `agent.enable`, `agent.billing_mode.set`, `agent.billing_mode.set_all`, `agent.credential_holder.set`, `agent.tool_class.set`, `agent.heartbeat`, `agent.events.consume`, `agent.triggerers.read`, `agent_session.read`, `agent_session.reset`, `agent_session.bind`, `agent_session.report`, `command.read`, `command.register`, `command.remove`, `dev_environment.read`, `dev_environment.manage`, `dev_environment.report` |
| `files` | `file.upload`, `file.read`, `file.delete`, `file.transcribe`, `file.transcript.submit` |
| `messaging` | `thread.create`, `message.read`, `message.create`, `message.update`, `message.delete`, `message.revisions.read`, `reaction.add`, `reaction.remove`, `pin.create`, `pin.delete`, `mention.list`, `template.read`, `template.manage`, `export.channel` |
| `runs` | `agent.invoke`, `run.read`, `run.overview.read`, `run.context.read`, `run.start`, `run.token.renew`, `run.update`, `run.finish`, `run.cancel`, `run.followup.create`, `component.update`, `interaction.create`, `interaction.respond`, `command.autocomplete`, `command.autocomplete.respond`, `command.invoke` |
| `billing` | `cost.read.thread`, `cost.read.aggregate`, `price.read`, `price.manage`, `cost.reconcile`, `cost.reconcile.read`, `cost.reconcile.submit` |
| `sync` | `read_state.update`, `read_state.mark_unread`, `draft.read`, `draft.write`, `settings.read`, `settings.update`, `sync.snapshot.read`, `events.catch_up` |
| `notify` | `notification_pref.update`, `quiet_hours.update`, `push.register`, `push.unregister`, `push.deliver` |
| `gateway` | `gateway.connect`, `gateway.focus`, `typing.start` |
| `search` | `search.query` |
| `stats` | `stats.read`, `client_metrics.submit` |
| `apps` | `app.read`, `app.install`, `app.scopes.update`, `app.suspend`, `app.resume`, `app.uninstall`, `webhook_out.read`, `webhook_out.manage`, `webhook_out.redeliver`, `webhook_out.deliver`, `webhook_out.transition`, `webhook_in.read`, `webhook_in.create`, `webhook_in.revoke`, `webhook_in.post` |
| `importer` | `import.read`, `import.manage`, `import.apply` |
| `privacy` | `gdpr.request.create`, `gdpr.request.read`, `gdpr.request.decide`, `gdpr.export.download` |
| `status` | `status.public.read`, `status.read`, `status.manage` |
| `terminal` | `host.read`, `host.manage`, `host.pair`, `host.connect`, `terminal.open`, `terminal.attach`, `terminal.stream`, `terminal.read_own`, `terminal.close`, `terminal.terminate` |

Rationale for borderline cases:

- `agent.invoke` belongs to `runs`, because the result is always a run, a follow-up or a
  command invocation. `authz` still evaluates the rules from 05 section 5.
- `agent.config.read` belongs to `chat`, because the prompt, skills and project binding live there.
- `thread.create` belongs to `messaging`: a new thread (forum post or thread from a
  message) comes into being with its first message, and `chat` must not use `messaging`.
  `chat` creates the row through the port `Threads` and does not check `thread.create` again;
  only tags with `restricted` are checked by `chat` with `thread.tags.set`, because only `chat`
  knows the tags (05, row `thread.create`). `chat` itself creates the main thread of a new channel as
  part of `channel.create` (R-DM-11).
- `instance.bootstrap` and `owner.recover` belong to `identity`, because they write people, owners,
  identities and enrollment codes; `instance` only sets `setup_completed_at` through
  its port.
- `command.invoke` and `command.autocomplete` belong to `runs` (delivery to the agent with
  `agent.invoke`), the register (`command.register`, `command.remove`, `command.read`) to `agents`.
- `export.channel` belongs to `messaging`, and the export streams NDJSON without storing a
  package; export packages in `files` belong only to the privacy export of `privacy` (M7-06)
  (ruling D-M3-11-8 in [docs/decisions.d/M3-11.md](../decisions.d/M3-11.md)).
- `client_metrics.submit` belongs to `stats`: the endpoint accepts metrics without content and passes
  them on to telemetry ([ADR 0057](../adr/0057-client-telemetry-through-a-server-endpoint.md)); it writes no table.
- The dev environment actions belong to `agents`, because the environment is bound to a runner agent
  (milestone).

### 4.7 Event types and WebSocket ops per module

An event type has exactly one producing module. The receive action is in 05 section 6.

| Module | Durable events | Ephemeral events | WebSocket ops (client to server) |
|---|---|---|---|
| `instance` | `instance.updated` | | |
| `audit` | | | |
| `authz` | `grant.changed`, `group.changed`, `access.changed`, `access.expiring` | | |
| `identity` | `user.updated`, `session.revoked`, `device.pending` | | |
| `chat` | `thread.created`, `thread.updated`, `thread.closed`, `thread.reopened`, `thread.moved`, `thread.moved_out`, `space.created`, `space.updated`, `space.archived`, `channel.created`, `channel.updated`, `channel.settings_changed`, `channel.archived`, `sidebar.updated`, `agent.config_changed` | | |
| `agents` | `agent_session.changed`, `command.changed`, `agent.updated` | `agent.presence_changed` | `ack` |
| `files` | `file.updated` | | |
| `messaging` | `message.created`, `message.updated`, `message.deleted`, `reaction.added`, `reaction.removed`, `pin.added`, `pin.removed` | | |
| `runs` | `run.state_changed`, `agent.invocation`, `agent.followup`, `run.cancel_requested`, `interaction.created`, `command.invoked` | `run.updated`, `command.autocomplete_requested` | `run.update`, `command.autocomplete_result` |
| `billing` | | | |
| `sync` | `read_state.updated`, `draft.updated`, `draft.deleted`, `settings.updated` | | |
| `notify` | `notification_pref.updated`, `quiet_hours.updated` | `notification.created` | |
| `apps` | | | |
| `status` | `status.updated` | | |
| `privacy` | `gdpr.request_updated`, `gdpr.request_due` | | |
| `importer` | `import.progress` | | |
| `gateway` | | `typing.started` | `identify`, `resume`, `heartbeat`, `reauth`, `focus`, `typing.start`, `typing.stop` |
| `search` | | | |
| `stats` | | `stats.live` | `stats.subscribe`, `stats.unsubscribe` (through the shared WebSocket hub) |
| `terminal` | `host.updated`, `terminal_session.changed` | `host.presence_changed` | none on the shared WebSocket; separate connections with their own ops (05, section 7) |

`messaging` produces `message.updated` for `trigger_state`, even when `runs` triggers the change
(through `Messages.SetTriggerState`). Reason: one event type, one producer, one payload definition.

## 5. Layers within a module

| Layer | Package | Contains | May import | Must not |
|---|---|---|---|---|
| Port | `internal/<module>` | Inbound interfaces, command and result types (DTOs), in-tx events, declared source ports | `kernel` | `domain`, `app`, `adapters`, other modules, third-party libraries |
| Domain | `internal/<module>/domain` | Entities, value objects, state machines (for example run transitions), invariants, pure functions | Packages from `kernel-core` (`kernel/ids`, `kernel/clock`, `kernel/errs`, `kernel/textcheck`, `kernel/reqmeta`, `kernel/jose`, `kernel/designkeys`, `kernel/searchlang`) | Input and output, SQL, HTTP, context with a transaction, third-party libraries |
| Application | `internal/<module>/app` | Use cases: `authorize`, Unit of Work, call the domain, append events; outbound ports (repository, clock, external service) as interfaces | own port, own domain, `kernel`, ports of allowed modules | `adapters`, `platform`, `api`, third-party libraries |
| Adapter | `internal/<module>/adapters/<kind>` | `postgres` (sqlc code and repository), `httpapi` (part of the generated server interface), others such as `objectstore`, `clamav`, `apns` | everything of the own module, `kernel`, `platform`, `api`, ports of allowed modules, approved third-party libraries | `app` or `domain` of other modules |

Rules with reasons:

1. The port contains its own DTOs instead of domain types. Reason: the domain stays changeable
   without breaking other modules, and a module can later move behind a process boundary
   (section 16).
2. The application layer knows no third-party library. Reason: use cases are testable with fakes
   without a database; a driver change only affects adapters.
3. HTTP handlers are adapters. Every module implements the part of the generated
   `StrictServerInterface` that belongs to its operations. `internal/wire` embeds all
   module handlers in one struct; the compiler checks that the interface is complete and that
   no operation is implemented twice (duplicate methods are ambiguous under embedding and do not
   satisfy the interface).
4. Third-party libraries are approved per adapter kind (list in `modules.yaml`, section 7.1).
   Example: only `gateway/adapters` may import `github.com/coder/websocket`.
5. Metrics: a use case records metrics through an outbound port
   (`internal/<module>/app/metrics.go`, with a no-op implementation for unit tests); the
   OpenTelemetry implementation lives in `internal/<module>/adapters/otelmetrics`. Platform packages
   record directly through `platform/otel`. Reason: rule 2; names and attributes come only from
   the signal catalogue ([signals.md](../observability/signals.md)).

## 6. Code layout

```text
services/server/
  go.mod                      module path github.com/OWNER/bothy/services/server until V1 (PLAN P-05)
  cmd/bothy/                  main: subcommands serve, migrate, setup, config
  api/
    rest/                     generated from docs/api/openapi.yaml (oapi-codegen: std-http-server, strict-server)
    ws/                       frame and payload types from docs/api/asyncapi.yaml (tool: 06-api.md)
    routes/                   generated: operationId -> action, rate-limit class; op -> action
  architecture/
    modules.yaml              module register: tables, actions, dependencies, third-party libraries
  migrations/                 goose SQL, embedded in the binary (ADR 0007)
  internal/
    kernel/                   cross-cutting ports (section 4.5), without dependencies within the project
    platform/                 technology without domain logic
      config/ secrets/ pg/ httpserver/ ratelimit/ jobs/ otel/ health/ lifecycle/ egress/ buildinfo/
      jsoncanon/
    wire/                     wiring per role, registration of the in-tx subscribers
    <module>/                 port (root package)
      domain/
      app/
      adapters/
        postgres/             queries/*.sql, sqlcgen/ (generated), repository
        httpapi/              handlers for the operations of the module
        ...                   other adapters of the module
      README.md               purpose, ports, dependencies, tables, chapters
  test/                       test support across module boundaries: harness/ (real server, M1-71),
                              integration/, leak/, contract/, canary/, smoke/
  perf/                       load tests (build tag perf) and internal/report (thresholds from section 22)
  tools/archgen/              generator: architecture/modules.yaml -> .go-arch-lint.yml
  tools/flagcheck/            gate: registration of the feature flags against VERSION (ADR 0064)
  sqlc.yaml                   one entry per module, schema from migrations/
  .go-arch-lint.yml           generated from architecture/modules.yaml
services/host/                host service bothy-host (own Go module, own version; 24-terminal.md)
  cmd/bothy-host/             main: subcommands install, pair, doctor, run, restart, session-user,
                              pty-helper (24 section 2.2, 16 section 4.4; M2-42), confirm-agent (M2-45)
  internal/                   noise, frame, fingerprint (M2-38); buildinfo, clock (+ clocktest), config, state,
                              hostos (+ hostostest: the fake operating system), doctor (HD1 to HD11), localchan
                              (local channel, admin endpoint), helper (pty-helper), install (plans, units/),
                              startup (run, the Control port), pairing (M2-42); control (+ controltest, M2-43);
                              session (+ sessiontest: fake server and Go test device), pty, profiles, confirm
                              (M2-45)
  test/platform/              platform suite (build tag platform), only on disposable machines (M2-42)
  testdata/fakeclaude/        stand-in for claude auth status in the tests of HD6 (M2-42)
  testdata/noise/             test vectors, Go and Dart recordings, fingerprint vectors (24, 5.2, 14)
  .go-arch-lint.yml           maintained by hand, one component per package (no module register)
```

Test helpers that tests of several packages use live in the tree of the component whose types they
build, so that the architecture gate assigns them to a component: `internal/platform/<p>/<p>test`
(for example `pgtest`, `httpservertest`) and `internal/<module>/adapters/<module>test` (for example
`identitytest` with device keys, DPoP proofs and a real login). Only `_test.go` files import them; a
test (`go list -deps ./cmd/bothy`) keeps them out of the binary. `test/**` and `perf/**` form the
component `testsupport` (7.1): it may use everything, no production component may use it,
and the coverage per Go module (`quality.coverage.go_min_percent`, 23) does not count it, because it
is test code itself; nor do generated files (`*.gen.go`). Functional subareas of a module
(OIDC, flags, console) are files or subpackages of `domain`, `app` and `adapters`, never
separate packages next to them; a package without a component fails in the gate (7.1).

Places that carry the product name (Baseline section 1, ADR 0044; so that a change stays cheap),
by kind:

- Server: Go module path in `go.mod`; binary `bothy` (`cmd/bothy`); prefix of the environment
  variables `BOTHY_`; names of the images (`bothy-server`, `bothy-postgres`, `bothy-backupd`) and
  paths in the container (`/run/bothy`, `/var/lib/bothy`); roles of the database (`bothy_*`).
- Host service ([24-terminal.md](24-terminal.md)): Go module path under `services/host`; binary,
  service and user name `bothy-host`; state folder (`/var/lib/bothy-host`,
  `%ProgramData%\bothy-host`); protocol identifiers `bothy-terminal/1` (Noise prologue) and
  `bothy-host-fp/1` (fingerprint); release component `host`. Since M2-42: program folders
  `/opt/bothy-host/bin/`, `/Library/Application Support/bothy-host/bin/` and
  `%ProgramFiles%\bothy-host\`; configuration folders `/etc/bothy-host/`,
  `/Library/Application Support/bothy-host/` and `%ProgramFiles%\bothy-host\`; systemd units
  `bothy-host.service`, `bothy-host-helper-<name>.socket` and `.service`; launchd labels
  `bothy-host` and `bothy-host.helper.<name>`; Windows services `bothy-host` and
  `bothy-host-helper-<name>` and the virtual account `NT SERVICE\bothy-host`; channel endpoints
  `/run/bothy-host/`, `/Library/Application Support/bothy-host/run/` and the pipes
  `bothy-host-admin` and `bothy-host-helper-<name>`. Since M2-45: the confirmation endpoints
  `/run/bothy-host-confirm/confirm.sock`,
  `/Library/Application Support/bothy-host/confirm/confirm.sock` and the pipe
  `bothy-host-confirm`; the folder `.config/bothy-host` in the home of a session user (`claude`
  with the configuration of the coding agent at `organization`, on macOS `run` with the run file
  of the live sessions).
- Packages of the apps: Dart packages with the prefix `bothy_` (`bothy_viz`, `bothy_noise`).
- SDKs: Python distribution `bothy-sdk`, import package `bothy_sdk`, npm package `bothy-sdk` (not
  published until OQ-17 is decided).
- Grafana plugin ([25-visualization.md](25-visualization.md), section 8): folder and
  release component `grafana-bothy-live`, plugin ID `bothy-live-panel`, dashboard `bothy-live`.
- Cross-cutting: prefix of the telemetry, alert and SLO names (`bothy.`, `bothy_`, `Bothy`); prefix
  of the API tokens and pairing codes `bothy_` with the gitleaks rule
  ([ADR 0061](../adr/0061-token-format-with-a-fixed-prefix-and-checksum.md)); default value of the
  setting `branding.product_name`.
- Tools and operations: identifiers with the prefix `bothy-` (Semgrep rules, workflows,
  dashboards, SLOs, Compose services, temporary names); the name alone as a value of the default
  configuration (database and service name, components); display names in the tools (NOTICE,
  documentation site, project and quality gate in SonarQube, title of gitleaks, dashboards and
  folder in Grafana, design preview). Fixed and never renamed: the advisory lock keys `bothyjob`
  and `bothyboo` (a new value would let two server versions migrate at the same time).

This list is the only one; a test (PLAN M1-46, `tools/ci/tests/test_product_name_places.py`)
allows the name in code and configuration (without `docs/`, Markdown and code generated from the
documentation) only in these forms: module path, whole path segment, prefix (`bothy-`, `bothy_`,
`bothy.`, `BOTHY_`), alert name `Bothy<Name>` (only the names of the alert rules and the examples
of the alert tooling), binary before a subcommand (in `cmd/bothy` also as the command word and in
identifiers), whole value outside the client code (`apps/`, `packages/`) and display name in the
tool files named above. Prose in comments and texts for people do not name it. A change of the
name does not silently change the protocol identifiers: they are part of the terminal protocol
and change only with a new protocol version.

## 7. Automatic check of the rules

### 7.1 Module register as the source

`services/server/architecture/modules.yaml` is the only source for module boundaries ([ADR 0050](../adr/0050-module-register-as-the-source-for-architecture-gate-and-table-ownership.md)).
From it come the go-arch-lint configuration, the checks from 7.3 and the alignment with
tables 4.2, 4.4, 4.6 and 4.7 of this chapter. Format (normative; later tickets add
entries, never fields):

```yaml
# SPDX-License-Identifier: Apache-2.0
version: 1
exclude: [tools]                  # directories that go-arch-lint does not check (tools)
kernel:
  core: [ids, clock, errs, textcheck, reqmeta, jose, designkeys, searchlang]   # usable by every package
  ports: [actor, authorize, txn, eventlog, auditing, idempotency, personaldata, measure]
platform:                         # one component per platform package
  jobs: {depends_on: [pg], vendors: [river]}
  pg: {vendors: [pgx]}
wire: {vendors: []}
cmd: {vendors: []}
api: {vendors: []}               # with the first generated code (M1-17)
testsupport: {paths: [test, perf]}   # test code across module boundaries; may use everything, nothing may use it
modules:
  chat:
    tables: [spaces, user_space_states, channels, channel_skill_bindings, project_bindings,
             dm_participants, forum_tags, threads, thread_tags]
    actions: [space.read, space.create, thread.move, agent.config.read]   # complete in the register
    events: [thread.created, channel.created]                              # complete in the register
    ephemeral_events: []
    ws_ops: []
    depends_on: [identity, authz]
    adapter_vendors: [pgx, otel]
  # identity, authz and the other modules: entries like chat, omitted here
vendors:                          # <group>: import paths
  otel: [go.opentelemetry.io/otel, go.opentelemetry.io/otel/**]
  pgx: [github.com/jackc/pgx/v5, github.com/jackc/pgx/v5/**]
  river: [github.com/riverqueue/river, github.com/riverqueue/river/**]
```

Rules: modules appear in the order of the levels from 4.4; `tables`, `actions`, `events`,
`ephemeral_events`, `ws_ops` and `depends_on` follow tables 4.2, 4.6, 4.7 and 4.4. The
generator rejects unknown fields, duplicate entries, unknown modules or vendor groups, and
cycles. It also rejects keys and values that are not plain strings (tagged, for example with
`!!binary`, or written without quotes so that a YAML 1.1 reader such as PyYAML does not read them
as a string, for example `no`, `on` or `1:20`, and also `y`, `n`, `yes`, `no`, `on` and `off` in
any case), a `version` that is not written in decimal, entries in `testsupport.paths` under
`internal`, `cmd` or `api`, entries in `exclude` that match `internal`, `cmd`, `api` or another
directory of the module as a string prefix (go-arch-lint skips every path that starts with the
entry, for `in` that is all of `internal/`; both would take product code out of the gate), and
vendor patterns that are not a Go import path: the first element is a domain with a dot, the
others contain only letters, digits, `-`, `.`, `_` and `~`, and `**` stands only at the end after
a domain and one more fixed element. Reason: go-arch-lint turns every pattern into a regular
expression and escapes only `.` and `/`, and `github.com/**` would
allow almost every library (go-arch-lint v1.19.0 source:
[`exclude` as a string prefix](https://github.com/fe3dback/go-arch-lint/blob/v1.19.0/internal/services/project/scanner/scanner.go#L121),
[pattern to regular expression](https://github.com/fe3dback/go-arch-lint/blob/v1.19.0/internal/models/utils_path.go#L32-L38),
retrieved 2026-10-12; `TestRunRejectsExcludePrefixOfOtherDirectories` and
`TestArchLintVendorPatternIsLiteral` hold both). A ticket that introduces a platform package or a third-party library adds its
entry and its vendor group in the same change; a package without an entry fails in
`make arch` ("not attached to any component").
The optional top-level section `migrations: {used_by: [<platform package>, ...]}` (M1-12) makes
the package `migrations`, the embedded SQL migrations of ADR 0007, a component of its own that
uses nothing; `wire`, `cmd` and the platform packages listed in `used_by` may use it (the server
register lists `pg`, which runs them and builds the test databases from them).

### 7.2 go-arch-lint

Tool: go-arch-lint (MIT), currently v1.19.0 from 2026-09-07 [Q8, verified]. The generator
`tools/archgen` creates from `modules.yaml` four components per module (`<m>`, `<m>-domain`,
`<m>-app`, `<m>-adapters`), one component `platform-<p>` per platform package, plus `wire`, `cmd`,
`api` and `testsupport` (all project and third-party dependencies allowed, in no `mayDependOn`
of another component), and the rules from section 5. Reason for one component per platform package:
only this way can a third-party library be limited to exactly one package (River only in
`platform/jobs`, section 12.1). Generated excerpt for `chat`:

```yaml
version: 3
workdir: .
allow:
  depOnAnyVendor: false
  ignoreNotFoundComponents: true
exclude: [tools]
excludeFiles: ["^.*_test\\.go$"]
vendors:
  v-otel:  { in: [go.opentelemetry.io/otel, go.opentelemetry.io/otel/**] }
  v-pgx:   { in: [github.com/jackc/pgx/v5, github.com/jackc/pgx/v5/**] }
  v-river: { in: [github.com/riverqueue/river, github.com/riverqueue/river/**] }
components:
  kernel-core:   { in: [internal/kernel/ids, internal/kernel/clock, internal/kernel/errs,
                        internal/kernel/textcheck, internal/kernel/reqmeta, internal/kernel/jose,
                        internal/kernel/designkeys, internal/kernel/searchlang] }
  kernel-ports:  { in: [internal/kernel/actor, internal/kernel/authorize, internal/kernel/txn,
                        internal/kernel/eventlog, internal/kernel/auditing,
                        internal/kernel/idempotency, internal/kernel/personaldata,
                        internal/kernel/measure] }
  platform-jobs: { in: [internal/platform/jobs, internal/platform/jobs/**] }
  platform-pg:   { in: [internal/platform/pg, internal/platform/pg/**] }
  wire:          { in: [internal/wire, internal/wire/**] }
  cmd:           { in: cmd/** }
  api:           { in: api/** }
  chat:          { in: internal/chat }
  chat-domain:   { in: [internal/chat/domain, internal/chat/domain/**] }
  chat-app:      { in: [internal/chat/app, internal/chat/app/**] }
  chat-adapters: { in: internal/chat/adapters/** }
commonComponents: [kernel-core]
deps:
  kernel-core:   { mayDependOn: [kernel-core] }
  platform-jobs: { mayDependOn: [platform-jobs, kernel-ports, platform-pg], canUse: [v-river] }
  platform-pg:   { mayDependOn: [platform-pg, kernel-ports], canUse: [v-pgx] }
  chat:          { mayDependOn: [chat, kernel-ports] }
  chat-domain:   { mayDependOn: [chat-domain] }
  chat-app:      { mayDependOn: [chat-app, chat, chat-domain, kernel-ports, identity, authz] }
  chat-adapters: { mayDependOn: [chat-adapters, chat, chat-app, chat-domain, kernel-ports,
                                 platform-jobs, platform-pg, api, identity, authz],
                   canUse: [v-pgx, v-otel] }
```

The excerpt is a part of what `tools/archgen` generates from the example in 7.1; the test
`TestChapter02ExamplesMatchGenerator` checks example and excerpt against the generator.

Every component names itself first in its `mayDependOn`: go-arch-lint allows a component to
import its own packages only then or when it is listed under `commonComponents`, and the layout
from section 6 needs this (subpackages of `domain`, `app` and `adapters`,
`adapters/postgres/sqlcgen`, the test helpers `platform/<p>/<p>test` and `adapters/<module>test`,
`api/routes` calls `api/rest`). `kernel-core` is listed under `commonComponents` and so would not
need the entry; it keeps it for the uniform rule. In addition, `wire` may use all module,
platform and kernel components and, when the register has an `api` entry, `api`; `cmd` only
`wire`, platform and `kernel-ports`. `<m>-domain` may use only itself and `kernel-core`.
`kernel-ports` has no entry under `deps`, because go-arch-lint rejects an empty `mayDependOn`,
and so may use only `kernel-core`: the kernel ports do not use each other. Section 4.5 says
only that `kernel` depends on nothing in the project; the narrower reading applies until a
ticket justifies an edge between two ports and adjusts this rule in the same change.
`ignoreNotFoundComponents: true` admits components whose patterns do not yet match a directory
(`<m>-domain`, `<m>-app` and `<m>-adapters` before the first use case, `testsupport` before
`test/` and `perf/`); without the setting, go-arch-lint v1.19.0 aborts for every such pattern
([syntax](https://github.com/fe3dback/go-arch-lint/blob/v1.19.0/docs/syntax/README.md),
"ignore not found components (default `false`)", retrieved 2026-10-12). A package without a
component and an unknown component name in `mayDependOn` still fail (checked on 2026-10-11 in
M1-05, ruling M1-05/3; `TestArchLintPackageWithoutComponent` holds the first).

`make arch` rejects every symbolic link below the module before go-arch-lint runs: go-arch-lint
does not follow a linked directory and assigns it by the path of the link, but the Go compiler
builds the code behind it (reproduced on 2026-10-11 in M1-05 with go-arch-lint v1.19.0, ruling
M1-05/24; `tools/ci/tests/test_make_server_gates.py` holds the rejection). Only editor lock
files are exempt: links named `.#*` whose target does not exist; nothing can be built behind
them. Because go-arch-lint v1.19.0 aborts at every dangling link below a component with `**`
(reproduced on 2026-10-11 in M1-05), `make arch` then checks a copy of the module without these
names and names the files of the module in its messages; the Go compiler skips files whose name
starts with a dot anyway ("Directory and file names that begin with "." or "_" are ignored by
the go tool", [go command, Package lists and
patterns](https://pkg.go.dev/cmd/go#hdr-Package_lists_and_patterns), retrieved 2026-10-12;
ruling M1-05/27).

Checked on 2026-10-10 on a skeleton with all modules from section 4.2 (Go 1.27.2, go-arch-lint
v1.19.0): a clean skeleton yields "No warnings found", exit code 0. Four inserted violations were
all reported, exit code 1: `chat/domain` imports pgx; `chat/app` imports the port of
`messaging`; `messaging/app` imports `runs/app`; a new package `chat/util` without a component
("not attached to any component"). The last point is intended: new packages must go into the
register. Since M1-05, the tests in `tools/archgen` confirm the form with platform components on a
skeleton with a platform package and a driver stub: `TestArchLintSkeletonClean` (exit code 0)
and the four violations `TestArchLintDomainImportsDriver`, `TestArchLintAppImportsForeignPort`,
`TestArchLintAppImportsForeignApp` (there `messaging/app` imports the app of the allowed module
`chat`) and `TestArchLintPackageWithoutComponent` (exit code 1 with the package path), plus
`TestArchLintTestSupport` for the component `testsupport`, `TestArchLintChapter02Layout` (the
layout from section 6 with several packages per component, exit code 0) and
`TestArchLintLayersStayClosed` (domain uses its own adapters or app, port uses its own domain,
app uses a subpackage of a foreign app, a kernel port uses another one: exit code 1)
([ADR 0050](../adr/0050-module-register-as-the-source-for-architecture-gate-and-table-ownership.md), "Confirmation").

### 7.3 Other checks in the gate

| Check | What it holds | Implementation (ticket) |
|---|---|---|
| Table ownership | SQL of a module names only its own tables (principle 12 of 03) | Pass over `adapters/postgres/queries/*.sql`, table names against `modules.yaml` |
| Completeness | every table from 03 and every action from 05 exactly once in the register | `make register-check` (M1-06): alignment of the register with 03 and `actions.yaml`, also event types and ops; the alignment with the migrations comes with the first schema |
| Routes and ops | every operation has a known action; its handler lives in the module that owns the action | Startup check of the server and a test over the generated route table |
| In-tx subscriptions | every subscription is an allowed direction (a higher module subscribes to a lower one) | Golden file of the registrations in `internal/wire`; the registration derives publisher and subscriber from the Go packages, not from declarations |
| Counter access | only the phase hooks of `authz`, `sync`, `audit` use `instance.Counters` | Phase guard in `Counters` (runtime, 8.2) and a static test: only `instance`, `wire` and the packages of the registered hooks reference `Counters` |
| Docs alignment | tables 4.2, 4.4, 4.6, 4.7 of this chapter match the register | `make register-check` (M1-06): compares the machine columns, not the prose; also holds 4.5, the component diagram and the module READMEs |
| Generated files current | `.go-arch-lint.yml`, sqlc code, API code match their source | Regenerate and `git diff --exit-code` |

## 8. Transactions and Unit of Work

### 8.1 Flow

Every write use case runs in exactly one database transaction, `READ COMMITTED` (03,
principle 11). `kernel/txn` puts it into the context. If a use case calls a port of another
module, that module writes in the same transaction (ambient transaction). Only `platform/pg`
opens and ends transactions.

Phases (diagram [flow-event-path.svg](../architecture/flow-event-path.svg)):

1. **Begin.** For permission changes (option `RightsChange`), `platform/pg` first calls the
   phase hook of `authz`. It locks `sequence_counters('permission_version')` through
   `instance.Counters`, increments it (03, 6.3) and records the visible targets of the affected
   parties before the change. `RightsChange` names the affected parties directly (principals, for
   example the subject of a grant) or a target (space or channel, for example when archiving); for
   a target, `authz` determines the affected parties itself: subjects of valid grants on the target
   and its ancestors (groups resolved), active owners and the agents that serve as operator of the
   target under the operator rule. If the use case learns of further affected parties during the
   work (for example the new operator), it reports them before writing (04, section 8.2).
2. **Work.** Write domain rows, publish in-tx events, append durable events
   (buffered in the context, still without a number).
3. **Before the commit, fixed order:**
    1. `authz` (phase hook): only for a permission change. Check I1 (03, 6.4), compute the
       difference of the visible targets before and after, append `access.changed`, publish the
       in-tx event `AccessChanged`. Subscribers (for example `runs` for cancellations) still run
       here inside the transaction.
    2. `sync` (phase hook): lock `event_seq` and increase it by the number of events,
       write `event_log`, read `permission_version` (after the lock, so that the value matches the
       commit order), `pg_notify('event_log', <highest seq>)`.
    3. `audit` (phase hook): lock `audit_seq`, extend the hash chain.
4. **Commit.** Then the response to the caller.

`platform/pg` knows no domain modules. `kernel/txn` defines the phases and the registry
`txn.Registry` (`SetHook`, `Subscribe`, `Seal`); `internal/wire` registers the hooks of `authz`
(phase 1 and 3.1), `sync` (3.2) and `audit` (3.3) and all subscribers in exactly one place and
seals the registry before the first Unit of Work; `platform/pg` calls the hooks in this
order. Hooks and subscribers are functions or method values of the owning module, never
closures in `internal/wire`; the registry derives their module from the Go package and rejects them
otherwise. Reason: the golden file should show the real owner. `RightsChange` applies only to the
outermost Unit of Work; in-tx events and durable events arise only in the phases "Work" and
3.1.

A veto in phase 2 or 3.1 (domain error, I1 violation) rolls everything back. Rejections that must
go into the audit are written afterwards by `auditing.Recorder` in a separate transaction (03, 6.4).

### 8.2 Lock order

The order from 03 section 6.3 is enforced by the Unit of Work, not by the individual
use case: only these phase hooks lock counters through `instance.Counters`; no other module calls
`Counters`. In addition, `Counters` checks the phase at runtime (`permission_version` only in
phase 1, `event_seq` only in 3.2, `audit_seq` only in 3.3; otherwise error kind `internal`), and a
static test limits the packages that name `Counters` (7.3). A module locks several rows of the same
table in ascending `id`. If a transaction fails with a deadlock (`40P01`) or a
lock timeout (`55P03`), `platform/pg` retries the whole Unit of Work at most
`database.tx_retry_max` times. This is safe because the Unit of Work has been fully rolled back and
contains no external calls. Exception: the store reports waiting on the idempotency key (8.4)
as `request_in_progress`, not as a retryable `55P03`; otherwise the retry would multiply
the wait time.

### 8.3 In-Tx events

- Types live in the port of the publishing module, cross-cutting events in `kernel`
  (`authorize.AccessChanged`).
- Delivery is synchronous, in the order of registration, in the same transaction. A subscriber
  may write, append durable events and prevent the commit with an error.
- Registration only in `internal/wire`, as one list. A golden file records it; every change
  is visible in the diff. Every subscriber gets its own OTel span `txn.subscribe <Subscriber>`
  with `bothy.outcome` (`ok` or `veto`) ([signals.md](../observability/signals.md), section 5.2).
- Reason for synchronous instead of asynchronous: the consequences (create a run, clear the draft,
  cancel a run on revocation) must be atomic with the cause. What does not need to be atomic goes
  through the `event_log` (section 11).

### 8.4 Idempotency in the Unit of Work

- Messages and interactions: the pair of author and `client_msg_id` or `client_interaction_id`
  according to 04 section 5.1.
- All other write operations with `Idempotency-Key`: a stage of the HTTP chain
  (`platform/httpserver`, store `kernel/idempotency.Store` from `sync`) opens the Unit of Work,
  creates the row in `idempotency_keys` with `in_progress`, calls the use case (which joins the
  transaction), on `2xx` stores status and response body, and commits everything together.
  Final error responses (06, section 7, item 2) have no effect: the Unit of Work rolls
  back, and the response is stored afterwards in a separate short transaction. A
  concurrent retry waits on the primary key until the first transaction ends, and then reads
  the stored response. If the wait ends with `database.lock_timeout`, the server answers
  `409 request_in_progress` (04, 5.2) and stores nothing.
- Exception (M2-41): a final error response that the handler marks with
  `idempotency.KeepEffectOnFinal` keeps its effect; the stage stores it with `Store.Complete` in
  the same transaction and commits both together. Only `409 fingerprint_mismatch` of
  `confirmHost` is marked, because every failed attempt must count even with a key (TM-T19); a
  marked `401` or `5xx` still rolls back (docs/DECISIONS.md D-M2-41-7).
- Rights changes behind the stage (M1-31): the stage opens its unit with
  `txn.AllowLateRightsChange()`. The use case declares its rights change in its nested unit as
  everywhere else; that nested unit then runs the rights begin hook (phase 1), its work and the
  rights check (phase 3.1) inside the stage's transaction, and sync and audit run once at the end.
  The stage has written only its key row by then, so `permission_version` is still the first
  domain lock (section "Lock order"); a second rights change in the same unit is refused.
- Reason: key and effect arise in one transaction. A crash in between leaves no key without an
  effect and no effect without a key.

### 8.5 What never happens in a transaction

No outbound network calls (object storage, push services, webhooks, identity providers,
clamd, transcription) inside a database transaction. Outbound consequences arise after the
commit through an event consumer or a River job that was inserted with `InsertTx` in the same
transaction [Q5, verified]. Reason: long transactions hold locks, lengthen the
serialized commit phase at the `event_seq` counter and make retries unsafe.

## 9. Request path

Diagram: [flow-request-path.svg](../architecture/flow-request-path.svg).

### 9.1 Chain in the api node

Outer part, for every request on the public listener before routing:

| Step | Place | Task |
|---|---|---|
| 1 | `platform/httpserver` | Catch panics (`500 internal_error`, stack only in the log) |
| 2 | `platform/otel` | Server span through otelhttp (inbound `traceparent` according to 17, section 3), `correlation_id` from the trace ID, access log without contents, query and tokens |
| 3 | `platform/httpserver` | Base headers (SEC-HDR-1, `Correlation-Id`), client IP from trusted proxies in `kernel/reqmeta` |
| 4 | `net/http` router | Find the operation; no pattern: `404 not_found`, wrong method: `405 method_not_allowed` with `Allow` |

Per route, wrapped around the handler of every operation (pattern and operation are known):

| Step | Place | Task |
|---|---|---|
| 5 | `platform/httpserver` | Operation with action and rate-limit class from `api/routes` into the context; limits per route class: `http.max_json_body_bytes` or `file.max_upload_bytes`, deadline `http.request_timeout` or `http.transfer_timeout`; response writer with the proof check (9.3). The two operations with action `webhook_in.post` have the body limit `webhook_in.max_body_bytes` (`Operation.BodyLimit`); a declared `Content-Length` above it is refused at step 11 instead of here, so the token check of step 8a answers first (11 section 8.4: token before size) |
| 6 | `platform/httpserver` | CORS (SEC-CORS-1, SEC-CORS-2) |
| 7 | `kernel/actor` (from `identity`) | Check the access token with DPoP or the API token, session, device, principal status; without credentials: principal `anonymous`. Operations with action `webhook_in.post` (`Operation.AnonymousCredentials`) skip the credentials precheck and the authenticator and stay `anonymous`, whatever `Authorization` holds |
| 8 | `platform/ratelimit` | Token bucket of the class, `429` with `Retry-After` |
| 8a | `apps/adapters/httpapi` (`TokenStage` at `httpserver.StageWebhookToken`, installed by `internal/wire`) | Only for action `webhook_in.post`: `apps` checks the token of the incoming webhook (11 section 8.4), from the path value `token` (`postIncomingWebhook`) or from `Authorization: Bearer` (`postIncomingWebhookWithBearer`). A failed check answers `404 not_found` and keeps one token of `rl.public_auth` of the client IP (`ratelimit.Limiter.FailedCredentialGuard`); while that bucket is empty the answer is `429` without a check. Success replaces the anonymous actor with the webhook actor |
| 9 | `platform/httpserver` | Maintenance mode (05, rule 9) |
| 10 | `platform/httpserver` | `Idempotency-Key` (8.4) |
| 11 | `platform/httpserver` | Media type; JSON guard: duplicate keys, invalid UTF-8, nesting beyond `http.max_json_depth` (SEC-IN-2) |
| 12 | `platform/httpserver` | Required `If-Match` (`428 precondition_required`) |
| 13 | `api/rest` (generated) | Validate the request against the schema (SEC-IN-1) |
| 14 | `platform/httpserver` | Install the check log for the proof (9.3) |
| 15 | `<module>/adapters/httpapi`, `<module>/app` | Build the DTO, use case with `authorize`, Unit of Work, domain, repository; map the response |

Every step reports errors as `application/problem+json` (section 14.1). Reason for the
order: body limit and deadline depend on the route class, and that is only known after
routing; throttling runs before the expensive part (decoding, schema, database), but
after authentication, because most classes count per principal (05, section 3).

Against floods of invalid credentials before throttling: if in step 7 the check of presented
credentials fails (`401` except `token_expired` and `use_dpop_nonce`, which are normal flows),
the request consumes a token from `rl.public_auth` of the client IP. If this bucket is empty,
step 7 answers requests with credentials from this client IP with `429`, without checking the
credentials (05, section 3). Reason: every check costs a signature verification, and without
this rule it would be unbounded.

The internal listener (3.2) serves only the two operations with action `webhook_in.post`, through
a chain of its own with the same per-route steps: it has no trusted proxies, so the client IP of
step 3 is the peer address and forwarded headers are never read
([19-security.md](19-security.md), R-SE-09), and it has its own rate limiter, so
`rl.webhook_in` and `rl.public_auth` count per listener.

### 9.2 404 instead of 403

A use case loads the resource before it calls `authorize`, and on `not_found` returns the same
error as for a missing resource. Same response, same body, same code paths up to
the error (05, test 4).

### 9.3 Proof of the check

`kernel/authorize` logs every check in the request context (action, kind, decision); every
implementation of `Authorizer` writes one entry per `Authorize` and `Scope`. The
response writer from step 5 checks on the first `WriteHeader` or `Write`: on success (status
below `400`) at least one check with the action of the route must have taken place; lists
count through `Scope`. If it is missing, the writer discards the headers and body of the handler,
sends `500 internal_error` (cause `authz_not_checked` only in log and metric, 14.1), increments
`bothy.authz.errors` with `bothy.reason = authz_not_checked`
([signals.md](../observability/signals.md)), and tests fail.
Responses from `400` upward pass through unchanged. Reason for the place: the strict middleware sees
a typed response object, not the status. Reason: B7 requires that every route
checks. The lint rule in the spec holds only the declaration; this proof holds the execution.

The gateway upgrade `GET /api/v1/gateway` is the one route marked as authorized in-band
(`AuthorizedInBand`, M1-64): it passes only steps 1 to 4, because `identify` and `resume`
authenticate and authorize on the connection (R-SY-01). The same proof applies per op there: the
gateway dispatcher runs every op with its own check log and, after a successful handler, requires a
check of the op's action (05 "WebSocket ops and action"); a missing check closes the connection with
`4000` and raises `bothy.authz.errors` with `bothy.reason = authz_not_checked`.

## 10. Gateway

The gateway is part of the role `api`. Per connection, one read and one write goroutine run. The
writer reads from a bounded queue (`sync.outbound_queue_max`) and writes with the deadline
`gateway.write_timeout`. If the queue overflows or a write misses the deadline,
the server closes with `4014` (04, 6.4 and 7.2). It drops ephemeral events when the queue is
full (04, 7.2 item 6).

Gateway sessions live only in the memory of the node. A `resume` on another node ends with
`4006`; the client does `identify` with its cursor and receives the same catch-up (R-AR-11,
diagram [seq-3-reconnect.svg](../architecture/seq-3-reconnect.svg)). Reason: a
shared session store would be another state; the difference between `resume` and
`identify` is only the size of the snapshot and the restored focus.

Agent connections are consumers with a durable cursor (`agent_consumer_cursors`, 04 section
15). The gateway hands personal invocation events (`agent.invocation`, `agent.followup`,
`interaction.created`, `command.invoked`) to `runs.Delivery` before sending. There, `agent.invoke`
with current permissions, I2 and the signing of the invocation token take place (04, 15.3). If the
check fails, `runs` sets the run to `failed` in a separate transaction and the gateway sends nothing.
The REST poll goes through the same engine and the same call; a refused event counts as checked and
`next_cursor` covers it, an event whose preparation failed with an error is not sent and comes
again with the next delivery (M2-08).
An agent consumer connection reads the log through its own runner, up to the hub position (the
highest `seq` the log tail of the process has passed), and is woken by the hub after each event
(M2-07).

## 11. Event path

Diagram: [flow-event-path.svg](../architecture/flow-event-path.svg).

### 11.1 Writing

As in section 8.1. `pg_notify` carries only the highest `seq`, never contents. PostgreSQL delivers
the notification only after the commit [Q1, verified].

### 11.2 Log tail per api process

- Exactly one reader per process (`sync.Tail`) with its own, non-pooled connection for `LISTEN
  event_log`. It holds `local_head` and reads `event_log WHERE seq > local_head ORDER BY seq LIMIT
  sync.tail_batch_size`.
- The wake-up signal is `NOTIFY`; in addition it reads every `sync.log_poll_interval`. If the
  `LISTEN` connection is lost, it reconnects with backoff up to `database.listen_reconnect_backoff_cap`
  and reads from `local_head`. Because `seq` is gapless, nothing is lost in the process.
- It passes events in `seq` order to the permission cache and then to the hub, in one
  goroutine. Reason: order is only easy to guarantee when one reader produces it.
- Third consumer (M2-44, D-M2-44-7): after the hub every event goes to the observer of the
  terminal relay (`Relay.ObserveLogEvent`, non-blocking). For a `terminal_session.changed` whose
  payload has `state = closed` it closes the device stream (`StreamCodeForEnd`, the
  `close_reason` as close reason) and the data connection (`4100`), drops the slot without
  writing its counters and sends `session.close` to the host service. Every end of a session, from
  any module and any process (a worker or another node), commits a row of `event_log`, so it
  reaches the connections and the host service without an after-commit hook (`txn` has none).

### 11.3 Permission cache

The cache lives in `authz`, per process, per principal, keyed by `permission_version`. The
log tail reports every `access.changed` through `Authorizer.Observe` before the hub distributes
later events (R-SY-02). Entries carry a validity until the earliest expiry of an involved
grant or membership and are recomputed afterwards. Structural changes
(`channel.created`, `channel.updated` with read-only, `channel.archived`, `space.archived`,
`thread.moved`) are also applied by the cache in log order; archiving additionally brings
a new permission version (04, section 8.1). Catch-up additionally filters by fully visible spaces, so that events
of new channels are not missing (04, section 7.2).

### 11.4 Hub and delivery

- The hub holds indexes: channel to connections, principal to connections, thread to focused
  connections.
- For every event it determines the candidates through the indexes and calls, per candidate,
  `authorize(recipient, receive_action, resource)` from the cache. Only then does it put the frame
  into the queue of the connection. The hub never blocks on a connection.
- Connections in catch-up buffer live events and send them after `catchup.complete` (04,
  7.2).
- Agent consumer connections take no durable dispatch from the hub: each reads the log through its
  own runner up to the hub position and is woken by the hub after each event (M2-07).

### 11.5 Worker consumers (push, webhooks)

Outbound consequences arise in the worker from the log, not in the write path (R-AR-08):

1. Per consumer name (`push`, `webhooks`) there is one cursor row in `event_consumer_cursors`
   (03, [ADR 0051](../adr/0051-worker-consumers-of-the-event-log-with-a-cursor.md)).
2. One round: open a transaction, lock the row with `FOR UPDATE SKIP LOCKED` (if another worker
   is already running, the round ends at once), read up to `consumer.batch_size` events from the
   cursor, determine the recipients per event and insert River jobs with `InsertTx`, advance the
   cursor, commit.
3. The wake-up signal is the same `NOTIFY event_log`; in addition every `consumer.poll_interval`.
4. Jobs check `push.deliver` or `webhook_out.deliver` again before sending (05, R-AZ-11).

Reason against inserting jobs in the write path: the write path would have to know every
consequence; a new consumer would change every producer. The log already is the outbox. A round
creates jobs exactly once, because cursor and jobs commit in one transaction. `webhook_deliveries`
additionally has the unique key `(subscription_id, event_seq)` (03).

Rules of the frame (M2-01, [DECISIONS.md](../DECISIONS.md) D-M2-01-1 to D-M2-01-9):

5. **Start at the head.** In the roles `worker` and `all` the start inserts, before the first
   round, the row of every registered consumer that has none, with `last_seq = max(seq)` of the
   log. A new consumer therefore never replays retained events, like the first connection of an
   agent consumer (04, 15.1 item 2). No migration inserts rows; names that are not registered have
   no row. When two workers start together, one row wins.
6. **Gaps.** When the events from `last_seq + 1` on are missing because retention detached their
   partition (the first number read equals `min(seq)`), the round processes from `min(seq)` and
   writes the error line `event consumer lost events to retention`; the events cannot come back,
   and a webhook subscription detects its own gap (11, section 6). Any other gap is a bug (04, 3.2
   items 3 and 4): the round rolls back, the cursor never passes the gap, the consumer stalls and
   the alarm reports it.
7. **Failures.** A handler error or panic, a database error and a gap roll back the whole round:
   no job, no cursor move, the next attempt at the next wake or poll. No event is ever skipped.
   The first failed round after a success writes `event consumer round failed`, the next success
   `event consumer recovered`. Consumers are independent; a stalled consumer holds back no other.
8. **No durable events in a round.** A handler that appends an event fails the round: the
   consumer would read its own output, and the round would take the lock of the counter
   `event_seq` (11.7).
9. **One `LISTEN` connection per process** for all consumers, apart from the log tail's
   (application name `bothy-consumer-wake`), reconnected with a backoff from
   `consumer.poll_interval` up to `database.listen_reconnect_backoff_cap`. After a full batch the
   next round starts at once. Every role exports `bothy.event.consumer.lag` and
   `bothy.event.consumer.pending_age` per consumer ([signals.md](../observability/signals.md) 4.2);
   the alarm `BothyEventConsumerLag` fires at `consumer.pending_alert_after`, which is strictly
   below `sync.event_retention` (13, item 4).

### 11.6 Ephemeral events

| Event | Created in | One node | Several nodes (`cluster.mode = multi`) |
|---|---|---|---|
| `typing.started` | `gateway` (op `typing.start`) | directly to the hub | additionally `NOTIFY ephemeral` with thread ID and principal ID |
| `run.updated` | `runs` (patch stored, transaction without `event_log`) | hub coalesces per `sync.run_patch_coalesce_interval` | `NOTIFY ephemeral` with run ID and version; the other node loads the run document and sends `run.updated` with `snapshot` |
| `agent.presence_changed` | `agents` (heartbeat) | directly to the hub | `NOTIFY ephemeral` with agent ID and state |
| Replacement of a device connection (`4017`, 04 section 6.1) | `gateway` (`identify`, `resume`) | directly to the older connection | `NOTIFY ephemeral` with device ID and gateway session |

Reason: R-Stack allows `LISTEN/NOTIFY` only as a wake-up signal without content. Run contents
therefore do not go through `NOTIFY`; the receiving node reads them from the database. The channel
`ephemeral` is only active in mode `multi`, because every `NOTIFY` takes an instance-wide lock at
commit [Q2, verified] and a single node does not need it (03, section 6.6). `notification.created`
and `command.autocomplete_requested` take the same path with IDs.

Terminal signals (M2-47, D-M2-47-3, D-M2-47-4; [24-terminal.md](24-terminal.md), section 9). The
module `terminal` sends its own internal signals on the channel `ephemeral`: `terminal.session_open`
(host and session ID), `terminal.host_close` (host ID), `terminal.host_present` and
`terminal.host_gone` (host and connection ID) and `terminal.session_paired` (host and session ID).
They are no event types of 05, so the table above has no rows for them. With one node a use case
hands its signal to the local handler after the commit; with several nodes it stages
`NOTIFY ephemeral` in its transaction (presence and pairing go out in their own statement), and
every node, the sender included, receives it. The end of a terminal session goes through the log
tail (11.2), not through these signals. The module `terminal` holds its own `LISTEN ephemeral`
connection per api process until M7-17 shares one per process; the namespaced kinds (prefix
`terminal.`) allow that.

### 11.7 Cost of serialization

All transactions with durable events serialize at the end on `event_seq` (04, 3.2).
The additional `NOTIFY` lock at commit falls into the same section and lengthens it
[derived from Q2 and 04]. Transactions without an event (run patches, heartbeat storage)
take neither a counter nor `NOTIFY`. The target `perf.event_commit_min_per_s` holds the throughput
(section 17).

## 12. Background jobs (River)

### 12.1 Usage

- River (MPL-2.0) only in `internal/platform/jobs`; modules define job arguments and
  handlers against an interface of its own in `platform/jobs`. Reason: River is 0.x; a
  breaking change then affects one package.
- Tables in their own schema `jobs.schema`; migration through `rivermigrate` in `bothy migrate`
  [Q7, verified].
- Role `api` uses a pure insert client (without `Start`, without queues); it does not take part
  in leader election [Q5, verified]. Role `worker` runs jobs, periodic jobs and maintenance.
- No Pro features (R-Stack). For flows with several steps, a job chains the next one
  via `InsertTx`.
- Job arguments contain only IDs, never contents or secrets. Reason: `river_job` is another
  storage location; deletion and data protection stay with the domain tables. Enforced at
  registration: the argument type is a struct whose exported fields are only `kernel/ids.ID`,
  `[]kernel/ids.ID` or `int64` (sequence numbers and versions), so a string, byte slice, map,
  interface, pointer or nested struct fails the start (`jobs.ValidateArgsType`, M1-14).

### 12.2 Queues

| Queue | Content | Concurrency |
|---|---|---|
| `default` | short domain jobs | `jobs.queue.default.max_workers` |
| `files` | scan, thumbnails, transcription (CPU, memory) | `jobs.queue.files.max_workers` |
| `delivery` | push, webhooks (network) | `jobs.queue.delivery.max_workers` |
| `maintenance` | cleanup, partitions, checks | `jobs.queue.maintenance.max_workers` |
| `privacy` | export and erasure | `jobs.queue.privacy.max_workers` |

Reason for separate queues: many thumbnails or a large GDPR export must not delay push
and webhooks. Imports do not run as a job in the server, but in the tool
`bothy import` through the API ([22-import-migration.md](22-import-migration.md), section 2).

In maintenance mode (`server.maintenance_mode = read_only`,
[ADR 0065](../adr/0065-maintenance-mode-read-only.md)) the `worker` processes only the consumers
`push` and `webhooks` ([ADR 0051](../adr/0051-worker-consumers-of-the-event-log-with-a-cursor.md)),
the queue `delivery` and the job `runs.cancel_timeout`; their transactions are exempt from the
read-only Unit of Work. All other queues and periodic jobs pause and only continue
after the restart without the mode, or, during a move, on the target from the copied state.
Reason: every other job would write state that would be missing on the target.

### 12.3 Job catalogue

| Job | Module | Trigger | Queue | Domain rule in |
|---|---|---|---|---|
| `files.scan` | `files` | `InsertTx` when an upload completes | `files` | 12 |
| `files.thumbnail` | `files` | after `clean` or `skipped`: image originals (12 7.1), thumbnails (12 7.3), dimensions and duration of MP4 and QuickTime | `files` | 12 |
| `files.extract_text` | `files` | after `clean`, supported type | `files` | 14 |
| `files.transcribe` | `files` | `file.transcribe` with server transcription | `files` | 09, 12 |
| `files.cleanup_pending` | `files` | periodic | `maintenance` | 12 |
| `files.purge_objects` | `files` | periodic, deleted files | `maintenance` | 12, 20 |
| `notify.push_send` | `notify` | consumer `push` (one job per active registration of a hint), the flush `notify.coalesce_flush` and the badge sync `notify.badge_sync`; a retry inserts the same args with the next attempt (M3-19) | `delivery` | 13 |
| `notify.coalesce_window` | `notify` | consumer `push` or the flush `notify.coalesce_flush`: a send that opens a bundling window inserts it unique per (recipient, thread, window end), scheduled at the window end; its handler does nothing (13 4.5, M3-19) | `delivery` | 13 |
| `notify.coalesce_flush` | `notify` | consumer `push`: the first `message` hint collected in an open window inserts it unique, scheduled at the window end; it sends the unread count or nothing (13 4.5, M3-19) | `delivery` | 13 |
| `notify.badge_sync` | `notify` | consumer `push`: a `read_state.updated` that lowers the badge number inserts it unique per person and time, now or at the end of a pending `notify.badge_sync_hold` (13 8, M3-19) | `delivery` | 13 |
| `notify.badge_sync_hold` | `notify` | the job `notify.badge_sync`: inserted unique per person and end at now plus `notify.badge_sync_min_interval`; its handler does nothing (13 8, M3-19) | `delivery` | 13 |
| `apps.webhook_deliver` | `apps` | one attempt of a subscription (args: the subscription only, M2-26): a round of the consumer `webhooks`, the chain after a finished delivery, a scheduled retry, the lease wake-up after `webhook.delivery_timeout`, an owner resume or re-enable, `webhook_out.redeliver` and `app.resume`; jobs of one subscription serialize on its row lock | `delivery` | 11 |
| `runs.cancel_timeout` | `runs` | scheduled with `InsertTx` per `run.cancel_requested` that does not end the run at once, at now plus `agent.cancel_grace` (args: the run only): a run still `queued`, `running` or `waiting_input` then ends `cancelled` with `cancel_timeout`; an ended run is left as it is (M2-09, D-M2-09-17) | `maintenance` | 09 |
| `runs.stall_check` | `runs` | periodic, `agent.presence_timeout`: fails runs in `running` or `waiting_input` with `interrupted` or `stale` from the persisted heartbeat (09 section 5.5), never retried; `agent_unavailable` stays with `runs.queued_expiry` (M2-09, D-M2-09-19) | `maintenance` | 09 |
| `runs.component_expiry` | `runs` | periodic, `component.default_ttl` | `maintenance` | 09 |
| `runs.invocation_hold_timeout` | `runs` | scheduled, `file.invocation_hold_max` or `voice.transcript_hold_max` | `default` | 04, 12 |
| `runs.queued_expiry` | `runs` | scheduled with `InsertTx` in the transaction that creates the run, at `queued_at` plus `run.queued_max_age` (args: the run only): a run still `queued` then fails with `agent_unavailable`; a run not yet due (the key was raised) schedules the job again (M2-08) | `default` | 09 |
| `authz.expiry_reminder` | `authz` | periodic, `access.expiring` before expiry | `maintenance` | 07 |
| `authz.expiry_sweep` | `authz` | periodic, `access.changed` after expiry (system actions `grant.expire`, `group.member.expire`) | `maintenance` | 07 |
| `identity.auth_cleanup` | `identity` | periodic every `auth.cleanup_interval`: delete expired rows of `auth_challenges`, `auth_login_flows` and `device_pairings`; revoke pending devices after `auth.device_pending_ttl`, and paired devices that were never picked up and unused devices after `auth.device_inactive_revoke_after` (system action `device.expire`); the decision per request never depends on the job | `maintenance` | 08, 20 |
| `instance.key_rotation` | `instance` | periodic, states `next` to `retired` | `maintenance` | 19 |
| `sync.partition_maintenance` | `sync` | periodic every `sync.partition_maintenance_interval` and at the start of the worker: create `sync.event_log_partitions_ahead` partitions ahead of the head, detach after `sync.event_retention`, never the partition with the head (03 `event_log`); `bothy migrate up` calls the same routine | `maintenance` | 03, 04 |
| `sync.idempotency_cleanup` | `sync` | periodic every `sync.idempotency_cleanup_interval`, deletes expired rows in batches of `sync.idempotency_cleanup_batch_size` | `maintenance` | 04 |
| `audit.chain_verify` | `audit` | periodic every `audit.checkpoint_interval`: recompute the section since the last checkpoint, only then sign the next checkpoint, store it and deposit it at `audit.checkpoint_sink`; a break ends without a checkpoint and without a retry (never sign an unchecked chain) | `maintenance` | 19 |
| `audit.retention_trim` | `audit` | periodic, separate maintenance role of the database | `maintenance` | 20 |
| `privacy.export`, `privacy.erase` | `privacy` | approval of a privacy request | `privacy` | 20 |
| `billing.reconcile` | `billing` | `cost.reconcile` (manual) and `cost.reconcile.submit` (reconciliation app): apply submitted entries, update the totals on runs and sessions; never periodic, because the server fetches no billing data and holds no admin key ([ADR 0055](../adr/0055-cost-reconciliation-as-its-own-app-admin-key-never-in-the-server.md)) | `default` | 10 |
| `status.probe` | `status` | periodic | `maintenance` | 16 |
| `retention.<class>` | owning module | periodic per retention class | `maintenance` | 20 |
| `stats.retention` | `stats` | periodic: create daily partitions of `stats_minutes` in advance, detach partitions older than `retention.stats_minutes`; replaces the general `retention.<class>` for the class `stats`, because the table is partitioned | `maintenance` | 25 |
| `terminal.sweep` | `terminal` | periodic every `terminal.sweep_interval` and once at the start of the worker (roles `worker` and `all`); M2-46 (D-M2-46-8, D-M2-46-9): no actor (system); it reads every session that is not `closed` with the state of its host and ends each in its own unit of work (host row, then session row, the verdict decided again on the locked rows) through the one end function of the module (`terminal.terminate`) with the first match in this order: host not `active` (`host_disabled`, `host_retired`, `host_rekeyed`); login session revoked or past `expires_at` (`session_revoked`); `opening` past the last grant plus `terminal.open_timeout` (`open_failed` with `open_timeout`; the last grant is the UUIDv7 time of `token_jti`), except on a host reporting `require_local_confirm`; `terminal.max_session_duration` after `opened_at` (`max_duration`); `terminal.detach_grace` after `detached_at` (`detach_expired`); the latest of the stored `last_input_at`, `started_at` and `opened_at` plus `terminal.idle_timeout` plus `terminal.idle_grace` (`idle_timeout`). Fallback for the check in the relay, in case a node failed. An expired pairing code is not discarded: 03 `hosts` requires `pairing_code_hash` while `state = 'pairing'`, so the code stays stored until the next code or the retirement and is rejected at redemption (`pairing_expires_at`, 24 3.2) | `maintenance` | 24 |

### 12.4 Rules for jobs

1. Every job is repeatable without a double effect (check of the state before the effect, unique
   keys such as `(subscription_id, event_seq)`).
2. Every job passes its context on and returns `ctx.Err()` on cancellation; a `nil` counts as
   success in River [Q4, verified].
3. External calls have a deadline (`egress.timeout` or a name of its own in the domain
   chapter).
4. River handles retries with backoff; domain chapters with a rule of their own (webhooks:
   `webhook.retry_backoff_base`, `webhook.retry_backoff_cap`) set their own retry rule.
5. Otherwise the defaults of the pinned River version apply for job deadline, fetch interval,
   retry backoff and maximum number of attempts; the README of `platform/jobs` names them together
   with the version. A job with different needs sets a named value of its domain chapter. Reason:
   a name of its own without a need would be a second source for the same value. One River
   default is off: the nightly reindexer, because `bothy_app` owns no index of River and the
   reindex could only fail (16 "Roles"; docs/DECISIONS.md M1-14).
6. SQL in jobs runs in the same Unit of Work as in the request path, that is with
   `database.statement_timeout` and `database.lock_timeout`. A job splits long work into batches
   (rule 1). Migrations set only `migrate.lock_timeout` per step
   ([upgrade.md](../operations/upgrade.md), rule M6).

## 13. Configuration loading

The basis is ADR 0011 (12-Factor, secret provider; operations team). This chapter defines how the
server loads.

1. **Two kinds of values.**
    - *Configuration* (instance operator): addresses, limits, deadlines, drivers, secret
      references. Applies from start; a change needs a restart.
    - *Settings* (owner): `instance_settings` and `feature_flags` in the database. Changed through
      the API with audit, effective immediately, event `instance.updated`. Reason for the
      separation: infrastructure belongs to the instance operator, domain matters to the owner, and
      only settings need a permission check.
2. **Sources, precedence from low to high:** built-in defaults (in the schema,
   `internal/wire/configkeys_*.go`, equal to the chapter table of the key) < file (`--config` or
   `BOTHY_CONFIG_FILE`, YAML, nested like the names) < environment (`BOTHY_` plus the name in upper
   case, `.` becomes `__`, example `sync.event_retention` becomes `BOTHY_SYNC__EVENT_RETENTION`) <
   command line (only `--role` and `--config`). The mapping is reversible (ADR 0011, [16-operations.md](16-operations.md) 5.1).
3. **Secrets** only as a reference in keys ending in `_ref` (for example `database.password_ref`).
   The secret provider resolves them at start. Values never appear in logs, error texts or
   `bothy config print` (there only the reference appears; only `bothy config check --secrets`
   resolves and names per key `ok` or the error code). The provider is created only in
   `internal/wire` and `cmd/bothy`; modules get values through their `Config` or a narrow
   interface and read neither files nor the environment themselves.
4. **Check at start, fail fast:** unknown keys and unknown `BOTHY_` variables
   abort (typos would otherwise be silent; exceptions in [16-operations.md](16-operations.md) 5.1);
   types and ranges; cross conditions. Cross conditions today: `http.max_json_body_bytes` strictly
   greater than `message.max_content_bytes` (the envelope of a message is small compared to the
   gap between the defaults; a value of its own for it would be a guess without a measurement);
   `server.internal_listen_addr` and `server.listen_addr` share no port other than `0` when their
   hosts are equal or one of the hosts is empty, `0.0.0.0` or `::` (also with an IPv6 zone; an
   IPv4-mapped IPv6 address counts as its IPv4 address). Port `0` may occur twice, because tests
   bind port 0. `consumer.pending_alert_after` is strictly below `sync.event_retention`
   (cross-check `consumer_alert_below_retention`, M2-01), so the alarm `BothyEventConsumerLag`
   fires while a stalled worker consumer can still catch up. `webhook.retry_backoff_base` is at
   most `webhook.retry_backoff_cap` (cross-check `webhook_backoff_order`, M2-26), and
   `webhook.retry_max_age` is strictly below `sync.event_retention` (cross-check
   `webhook_retry_within_retention`, M2-26), because a retry needs its event in the log. `push.apns.team_id`, `push.apns.key_id` and `push.apns.topics` are set together or not at all, with `push.apns.private_key_ref` set when they are, and no topic is empty or repeated (cross-check `push_apns_complete`, M3-20), because a partial APNs configuration would start without sending to Apple. With `file.storage.driver = fs`, `file.fs.root` is an absolute path in
   clean form (no `.` or `..` element, no trailing `/`; cross-check `file_fs_root_absolute`,
   M3-01), because a relative root would depend on the working directory of the process; with
   `s3` the root is not read. With `file.storage.driver = s3` the cross-check `file_s3` (M3-02)
   holds `file.s3.endpoint` (set, an absolute `http` or `https` URL with a host and without user
   info, query, fragment or a path other than `/`), `file.s3.bucket` (set, S3 bucket naming
   rules), `file.s3.region` (set), `file.s3.prefix` (empty or the key grammar of
   [12-files.md](12-files.md) 4.1), `file.s3.use_path_style` (true when the endpoint host is
   an IP address), `file.s3.checksum_mode` (`when_supported` only with an `https` endpoint) and
   `egress.allowed_hosts` (an `http` endpoint needs an entry with `allow_private` for the request
   host and port, because the egress client allows `http` only there,
   [19-security.md](19-security.md) SEC-NET-2); with `fs` no `file.s3.*` value is read. The entries of `search.languages` are unique, unqualified lower-case PostgreSQL
   identifiers and never `simple` (check `search_languages`); every key of `search.locale_map` is
   a first language subtag in lower-case ASCII letters and every value is listed in
   `search.languages` or is `simple` (check `search_locale_map`, the start error of
   [14-search.md](14-search.md) 3.1; M3-13). `file.scan.clamd_address` is `unix://` followed by an
   absolute path in every scan mode (cross-check `clamd_address_unix`,
   [19-security.md](19-security.md) SEC-TLS-5, M3-04), and with `file.scan.mode = required`,
   `file.max_upload_bytes` is at most `file.scan.clamd_stream_max_bytes`, the mirror of the clamd
   option `StreamMaxLength` (cross-check `scan_stream_limit`, [12-files.md](12-files.md) 5.4,
   M3-04). In addition the loader checks a rule for a single key that no type expresses:
   `secrets.dir` is an absolute path in clean form (no `.` or `..`, no double or trailing `/`),
   not `/`, without a NUL character and without `<`, as the secret provider requires; a wrong
   value is a problem of `secrets.dir` alone, not of the references that use `<secrets.dir>`.
   Likewise `server.public_url` and every entry of `api.cors_allowed_origins` are an origin
   (scheme `http` or `https`, host, optional port, no path, no query, no fragment; M1-18),
   because the chain builds the DPoP `htu` from this origin and compares `Origin` exactly.
   The rule "identity headers only with a loopback listener" is not a configuration check,
   because the driver is a setting of the owner in the database; it applies at readiness
   ([16-operations.md](16-operations.md) 4.3). Messages name the key and the source, never the
   value.
5. **Typed per module.** Every module and every platform package has its own `Config` struct
   in its own package; `internal/wire` fills it from `platform/config` (binding via struct tags).
   Modules never read environment variables. A key enters the schema with the ticket that builds
   its first user; until then the configuration reference lists it as "planned" with a milestone.
   Reason: testability, one place for checks and no key without an effect.
6. **No reloading during operation** except secret rotation (signing keys, webhook secrets),
   which the secret provider reports. Reason: predictable behavior; half-applied limits are
   harder to diagnose than a restart.
7. `bothy config check` runs steps 2 to 4 without starting and writes, for every key `*_ref`
   whose effective reference uses `env:`, a warning to standard error; with `--secrets` it also
   resolves every reference `*_ref` through the secret provider (line `secret <key>: ok` or
   `secret <key>: <error code>`, exit code 1 when one fails). `bothy config print` shows the
   effective values with the source per key.

## 14. Error and timeout strategy

### 14.1 Error kinds

`kernel/errs` knows these kinds. Every error carries a stable public `code` from the
closed list in [06-api.md](06-api.md), section 5; the kind follows from the code, never from the
caller. 06 defines `type` URIs and bodies according to RFC 9457 (ADR 0015). The gateway uses the
same codes in the op `error`.

| Kind | HTTP | Codes from 06 (examples) |
|---|---|---|
| `bad_request` | `400` | `validation_failed`, `malformed_request`, `cursor_invalid` |
| `unauthenticated` | `401` | `unauthenticated`, `token_expired`, `invalid_dpop_proof` |
| `step_up_required` | `401` | `step_up_required` |
| `forbidden` | `403` | `forbidden`, `scope_missing`, `channel_read_only` |
| `not_found` | `404` | `not_found` (also without read permission) |
| `method_not_allowed` | `405` | `method_not_allowed` |
| `conflict` | `409` | `conflict`, `already_started`, `request_in_progress` |
| `invariant_violation` | `409` | `i1_violation`, `last_owner` |
| `gone` | `410` | `message_deleted`, `resync_required` |
| `precondition_failed` | `412` | `version_conflict` |
| `too_large` | `413` | `payload_too_large` |
| `unsupported_media_type` | `415` | `unsupported_media_type` |
| `range_not_satisfiable` | `416` | `range_not_satisfiable` |
| `unprocessable` | `422` | `idempotency_key_reuse` |
| `precondition_required` | `428` | `precondition_required` |
| `rate_limited` | `429` | `rate_limited` with `Retry-After` |
| `internal` | `500` | `internal_error` |
| `unavailable` | `503` | `unavailable`, `maintenance`, always with `Retry-After` (`http.unavailable_retry_after`) |

Reasons for a denial (`level_insufficient`, `capability_missing` and so on, 07 section 4.1) are
not codes: they appear in the audit and, where 06 provides for it, in the extension `reason`; the
code is `forbidden`. Internal causes are identifiers for log and metric, never part of a response:
`authz_not_checked`, `unknown_action` (both `500 internal_error`), `db_pool_exhausted`,
`shutting_down`, `deadline_exceeded`, `dependency_unavailable` (all `503 unavailable`). Reason:
06 holds the closed set of public codes, and internal states do not belong in responses.

Internal details (SQL errors, stack) appear only in the log with `correlation_id`, never in the
response.

### 14.2 Deadlines

| Place | Deadline | Reason |
|---|---|---|
| reading headers | `http.read_header_timeout` | protection against slow clients |
| normal request (context) | `http.request_timeout` | limits waiting time and resources |
| upload and download (via `ResponseController`) | `http.transfer_timeout` | large files |
| keep-alive idle | `http.idle_timeout` | release connections |
| getting a connection from the pool | `database.acquire_timeout` | overload becomes `503`, not a queue |
| SQL statement | `database.statement_timeout` (per transaction via `SET LOCAL`) | no query outlier blocks |
| waiting for a lock | `database.lock_timeout` | counter locks never unbounded |
| open transaction without work | `database.idle_in_tx_timeout` | protects the serialized commit phase |
| WebSocket write | `gateway.write_timeout` | slow readers do not block |
| outbound calls | `egress.timeout` ([19-security.md](19-security.md)) or a name of the domain chapter | every network call has a deadline |

The context of an aborted client cancels the transaction. If it has already committed, idempotency
covers the client's retry.

### 14.3 Retries

- In the request path: only the whole Unit of Work on `40P01` and `55P03` (section 8.2). After
  `database.tx_retry_max` retries it ends with error kind `unavailable` (`503 unavailable`
  with `Retry-After`); the database error appears only in the log. No retry of external calls in
  the request path (there are none there).
- In jobs: River with backoff; domain chapters can set their own rules.
- Clients retry according to 04 (outbox, full jitter).

### 14.4 Failure of a dependency

| Dependency | Behavior | Kind |
|---|---|---|
| PostgreSQL | api reports not ready, requests `503`; gateway keeps connections, delivers nothing; worker pauses; recovery without intervention | fail closed |
| `LISTEN` connection | reconnect with backoff, polling bridges the gap; no loss (gapless `seq`) | soft |
| `LISTEN` connection of the terminal (M2-47, `cluster.mode = multi`) | reconnect at once, then every `database.listen_reconnect_backoff_cap`; after `LISTEN` succeeded again a reconciliation of the node's connections against the host states; ends of sessions still arrive through the log tail; no readiness check (24, section 9) | soft |
| object storage | upload and download `503`; messages without files continue | partial |
| clamd | scan jobs retry; files stay `pending` and are not served | fail closed |
| push services | jobs retry; message path unaffected | soft |
| webhook targets | retries up to the limit, then subscription `disabled` (03) | soft |
| OTel Collector | discard data and count it; never block (R-Telemetrie) | fail open |
| identity provider | new login impossible; existing sessions continue | partial |
| secret provider | start aborts; rotation fails: the old value stays, alert | fail fast |
| transcription service | job retries, then `transcript_status = failed` | soft |

### 14.5 Panics

Caught per request, per job and per connection: request `500`, job as an error to River,
connection closed with `4000`. A panic in the handler of a worker consumer becomes the error of
its round, which rolls back (11.5 item 7); a panic in the frame of the consumers itself ends the
process. A panic in the log tail or the hub ends the process with an error
code, because a process without fan-out would not be ready but would still hold connections. The
orchestrator restarts, clients resume according to 04.

## 15. Start and graceful shutdown

### 15.1 Start

1. Load and check the configuration (section 13), start telemetry.
2. Resolve secrets. The internal listener opens now, so that the orchestrator sees `/healthz`
   and `/readyz` fails with `startup` while the start is running. Every role also checks the
   key store `secrets.signing_key_dir` (owned by the service user, mode `0700`, writable) and ends
   the start otherwise (ADR 0066, M1-32).
3. Reach the database, for at most `startup.dependency_wait` with backoff; then abort.
4. Check the schema version: the binary knows the smallest required version. If the database is
   older, the start aborts with a pointer to `bothy migrate up` (exit code `1`). Newer versions are
   allowed, because migrations are built by expand and contract (ADR 0007). River's
   migrations in `jobs.schema` must be complete as well; otherwise the start aborts with the same
   hint (checked in step 8 for every role, M1-14).
   Then the text search of the database (M3-13, [14-search.md](14-search.md) 3.1): every
   configuration of `search.languages` and `search.locale_map` (plus `simple`) must exist on the
   search path, and the database must fold the case of non-ASCII letters
   ([16-operations.md](16-operations.md) 7.2); otherwise the start aborts with exit code `1` before any
   public listener, and the message names the key and the entry, never the configured name.
5. Load the instance (if the row is missing: abort with a pointer to `bothy migrate up`). Without
   `setup_completed_at` the server starts completely, but answers only health checks and
   discovery, everything else with `503 setup_required` (08, section 4); it checks the state per
   request until it is set, and needs no restart after setup. `/readyz` does not depend on
   setup, otherwise an orchestrator would never open the way to redeeming the code.
   If
   `instance.min_server_version` requires a newer version than its own, the process keeps running,
   but never becomes ready: `/readyz` fails with `schema_requires_newer_server`. The same holds
   for the start lock of R-DM-50 ([16-operations.md](16-operations.md) 8.4, M7-04): unless
   `database.restore_guard` is `off`, a database identity (`pg_control_system().system_identifier`,
   `pg_control_checkpoint().timeline_id`) that differs from the one stored in `instance` keeps
   every role not ready with `restore_not_finalized` and logs once that `bothy restore finalize`
   must run; the process then skips step 8 (no log tail, presence tracker, terminal reconcile, River, worker consumers
   or public listener), so it takes no writes and delivers nothing. An empty stored identity
   never blocks.
6. Ensure signing keys: every purpose without an `active` key gets one
   ([19-security.md](19-security.md), section 10; [ADR 0066](../adr/0066-signing-keys-in-the-writable-key-store.md)).
   Keys whose private file is missing from the key store are retired first, so the purpose gets
   a fresh active key; when the key store holds the file of none of the unretired keys (a new or
   wrong volume, a restore without the key store), the start ends with an error instead, unless
   `signing_key.accept_lost_keys` is set ([19-security.md](19-security.md), section 16). This
   runs after audit, sync, authz and identity (steps 6a to 6d of `internal/wire`), whose unit of work records `signing_key.transition`.
7. Register routes and check the route metadata: every operation has a known action, otherwise
   abort (05, rule 3). Roles `api` and `all` with one node (M2-44, D-M2-44-9): before the
   public listener opens, the terminal module ends every session in `opening` with `open_failed`
   and `open_timeout` and sets every `active` session `detached`, one unit of work per session in
   ascending `id`, because the relay slots and connections lived only in the memory of the previous
   process. With several nodes (`cluster.mode = multi`, M2-47) this step does nothing.
8. Role `api`: start the log tail, set `local_head` to the current head, open the public listener.
   Role `worker`: start River and the consumers.
9. Report ready.

Reason for aborting on a schema that is too old, but "not ready" on `min_server_version`: a binary
in front of a schema that is too old does not find its tables; continuing to run would be of no
use. A binary that is too old in front of a newer schema (going back across more than one version)
could run, but must not; as a process that is not ready it shows the reason on `/readyz` and does
not keep the orchestrator in a restart loop (R-AR-16).

### 15.2 Health

On the internal listener `server.internal_listen_addr`, not in the public API
(R-AR-13, Ruling L2); [16-operations.md](16-operations.md), section 4.2 defines the paths and error codes:

- `/healthz` (alive): the process answers. If the log tail, the hub (api) or the
  River client (worker) fails unexpectedly, the process exits with exit code `1` (section
  14.5); that is why `/healthz` needs no check of its own for this. The same holds for the runner
  of the worker consumers (worker, M2-01); a failing round is no such end (11.5 item 7).
- `/readyz` (ready): start completed (`startup`), database reachable (`database`), version
  matching the schema (`schema_requires_newer_server`), the database identity as stored or the
  restore finalized (`restore_not_finalized`, 16 8.4), one active signing key per purpose
  (`signing_keys`), no shutdown started (`shutting_down`). The domain chapters bring further
  checks (for example `trusted_header_requires_loopback`, 16 section 4.3). Response format:
  16, section 4.2.

Reason for the separate listener: health needs no login and no action, but must not be
publicly reachable. That is why health has no entry in the action catalogue (05, R-AZ-22).

### 15.3 Shutdown

The signals `SIGTERM` and `SIGINT` start it, a second signal ends immediately (exit code `3`).
Total deadline `shutdown.total_timeout`; each step runs at most with its own budget and
the rest of the total deadline. During wiring, the sum of the step budgets
(`shutdown.drain_delay`, `shutdown.http_grace`, `shutdown.jobs_soft_timeout`,
`shutdown.telemetry_flush_timeout`) must not exceed `shutdown.total_timeout`, otherwise the
start aborts (exit code `2`). The orchestrator must wait longer: Compose sets `stop_grace_period`
greater than `shutdown.total_timeout` ([infra/compose/compose.yaml](../../infra/compose/compose.yaml),
a test holds it).

Role `api`:

1. Set `ready` to false, wait `shutdown.drain_delay` so that the proxy sends no new requests.
2. `http.Server.Shutdown` with `shutdown.http_grace`: close listeners, finish running requests.
   `Shutdown` does not close hijacked connections such as WebSockets [Q9, verified]; therefore:
3. Gateway: store `acked_seq` of all agent consumers immediately; close all connections with `4013
   server_restart` (04, 6.4; clients reconnect with a random delay). Control
   connections of host services (24 3.5, M2-43): the sessions of each host turn `detached`, then
   the connection closes with `4013`; the host service reconnects after a random delay below
   `host_service.reconnect_backoff_base` and keeps its PTYs until `terminal.detach_grace`. The
   relay of the terminal (M2-44) closes every device stream and data connection with `4013`.
   Store the presence state of the agents, without an event. (Stop step `agent_presence` in the
   phase of the connections, after the gateway's step, so no gateway heartbeat arrives later: it
   stops the deadline timers and flushes every unsaved heartbeat within the rest of
   `shutdown.total_timeout`; a restart of the server is not an agent going offline.)
4. Stop the log tail and `LISTEN`.
5. Flush telemetry, at most `shutdown.telemetry_flush_timeout`.
6. Close the pool.

Role `worker`:

1. Set `ready` to false.
2. Consumers finish their round (commit or rollback) and start no new one. The step has no
   own budget, only the rest of `shutdown.total_timeout`; after it the round is cancelled and
   rolls back.
3. River: soft stop with `SoftStopTimeout` equal to `shutdown.jobs_soft_timeout`, after that the
   job contexts are canceled [Q3, Q4, verified]; canceled jobs return an error and are
   retried.
4. Flush telemetry, close the pool.

Role `all`: api steps 1 to 4, then worker steps 2 and 3, then telemetry and pool. Reason:
first accept no new requests and events, then finish running jobs.

## 16. Scaling path

| Level | Setup | Trigger (measurable) | What changes |
|---|---|---|---|
| 0 | one process `--role=all`, PostgreSQL and object storage alongside | default | nothing |
| 1 | api and worker separate, one database | jobs violate `perf.message_create_p99`, or the memory use of the jobs disturbs the api process | only the role per container |
| 2 | several api nodes behind the proxy | a node reaches `perf.ws_connections_per_node` or its CPU limit; rolling updates without downtime | `cluster.mode = multi`: channel `ephemeral`, fencing of the agent consumers, rate limits per node, migrations strictly by expand and contract |
| 3 | several workers | the job backlog grows permanently (metric in 17) | nothing in the code: River distributes, consumers lock with `SKIP LOCKED`, periodic jobs via the River leader |
| 4 | read replica for catch-up, search, statistics | CPU of the database | read paths choose the connection; `head_seq` and pages from the same source; writes and invocation delivery stay on the primary |
| 5 | split off a module | criteria below | new role or separate service |

Multiple nodes in detail (level 2):

- Event log: every node has its log tail; the `NOTIFY` wakes all of them. Nothing else needed.
- Gateway session: `resume` on another node becomes `identify` (section 10).
- Agent consumers: a new `identify` on node B must end the old connection on node A
  (`4015`). When taking over, node B writes its gateway session ID into the cursor row; node A
  detects at the next save of `acked_seq` that the row no longer belongs to it, and
  closes with `4015` (fencing via `agent_consumer_cursors.holder_session_id`, 03). A signal via `NOTIFY ephemeral` speeds
  this up. Until then, the event ID and the run start by compare-and-swap prevent duplicate work
  (04, 15.3).
- Rate limits: buckets in the process. With `n` nodes, up to `n` times the value applies; the
  instance operator divides the values or sets affinity at the proxy. Reason: no shared counter
  service.
- Presence and typing: via `NOTIFY ephemeral` (section 11.6).
- Terminal: [24-terminal.md](24-terminal.md), section 9 (M2-47).

A module is split off only when all three conditions hold:

1. It has a need for resources or failure isolation that a separate role of the same binary does
   not solve.
2. It shares no write transaction with other modules; it talks only via ports and the
   event log.
3. A measurement from section 17 or 17-observability proves the need.

Candidates are the file processing of `files` (first as a separate role) and `importer`. Not
candidates are `chat`, `messaging`, `runs`, `authz`, `sync`, because they share transactions. The
limit of the pattern is the global counter `event_seq`: if `perf.event_commit_min_per_s` is not
enough, a new decision is needed (for example sequences per space) with its own ADR.

## 17. Performance targets

Targets are named values (section 22). A test in `make perf` holds every target; until it exists,
the target is "unchecked" (tickets). [ADR 0063](../adr/0063-performance-gate-for-the-event-log.md) governs when and on which environment measurements take place.

| Name | What is measured | Test |
|---|---|---|
| `perf.reference_profile` | environment in which the targets apply | all load tests run on it |
| `perf.message_create_p99` | server time of `message.create` (without attachment) under `perf.sustained_messages_per_s` | `perf/load_messages` |
| `perf.event_commit_min_per_s` | durable events per second with the counter pattern | `perf/event_commit` |
| `perf.fanout_local_p99` | commit to `dispatch` at a connected client, same node | `perf/gateway_fanout` |
| `perf.fanout_cross_node_p99` | the same across two api nodes | `perf/gateway_fanout_multi` |
| `perf.invocation_delivery_p99` | commit of the triggering message to `agent.invocation` at the fake adapter | `perf/invocation` |
| `perf.run_patch_fanout_p99` | `run.update` of the adapter to `run.updated` at the focused client | `perf/run_patch` |
| `perf.catchup_page_p99` | one catch-up page (`sync.catchup_page_size`) including `authorize` per event | `perf/catchup` |
| `perf.authorize_cached_p99` | `authorize` from the warm cache | benchmark `BenchmarkAuthorizeCached` |
| `perf.ws_connections_per_node` | concurrent connections of one api node with heartbeats | `perf/ws_soak` |
| `perf.ws_idle_conn_mem_max` | memory per idle connection | `perf/ws_soak` |
| `perf.idle_rss_max` | memory of an `all` process without load | `perf/startup` |
| `perf.startup_ready_max` | start to `ready` without migrations | `perf/startup` |
| `perf.push_handoff_p95` | commit to handoff to the push service (fake) | `perf/push_handoff` |

## 18. Modules and chapters 06 to 23

| Chapter | Content | Modules |
|---|---|---|
| [06-api.md](06-api.md) | conventions, endpoints, ops, code generation | all `httpapi` adapters, `gateway`, `api/` |
| [07-permissions.md](07-permissions.md) | grant model, `authorize`, filters, revocation | `authz`, permission cache in `gateway` and `sync` |
| [08-identity.md](08-identity.md) | providers, devices, tokens, step-up, setup | `identity` |
| [09-agents.md](09-agents.md) | adapter contract, runs, components, commands, sessions | `agents`, `runs`, parts of `chat` |
| [10-backend-policy.md](10-backend-policy.md) | I1 to I6, switching, costs | `agents`, `authz` (I1, I6), `runs` (I2, I3), `billing`, `terminal` (I6) |
| [11-extensions.md](11-extensions.md) | apps, manifest, webhooks, SDK | `apps` |
| [12-files.md](12-files.md) | upload, storage, scan, preview | `files` |
| [13-push-notifications.md](13-push-notifications.md) | notification, push | `notify` |
| [14-search.md](14-search.md) | full-text search | `search`, search vectors in `messaging`, `chat`, `files` |
| [15-client-architecture.md](15-client-architecture.md) | apps, local database, sync engine | counterpart of `gateway`, `sync` |
| [16-operations.md](16-operations.md) | installation, configuration, backup | `platform/config`, `platform/lifecycle`, `status` |
| [17-observability.md](17-observability.md) | telemetry, SLOs, dashboards | `platform/otel`, `stats` (delimitation) |
| [18-release.md](18-release.md) | versions, images, signatures | `cmd/bothy`, build |
| [19-security.md](19-security.md) | threat model, hardening | `audit`, `instance` (keys), all adapters |
| [20-privacy.md](20-privacy.md) | GDPR, retention, deletion | `privacy`, `kernel/personaldata`, retention jobs |
| [21-dev-isolation.md](21-dev-isolation.md) | multi-user dev environment | `chat` (project binding), `runs` (check `ready`) |
| [22-import-migration.md](22-import-migration.md) | Discord import | `importer` |
| [23-quality.md](23-quality.md) | gates, tests, tools | section 7, load tests from section 17 |
| [24-terminal.md](24-terminal.md) | hosts, host service, Noise stream, I6, terminal sessions | `terminal`; host service `services/host` outside the server |
| [25-visualization.md](25-visualization.md) | own measurement, live frame, visualizations, Grafana plugin | `stats`, `kernel/measure`; plugin `plugins/grafana-bothy-live` outside the server |

## 19. Decisions of this chapter

| No. | Decision | Reason |
|---|---|---|
| R-AR-01 | One binary `bothy` with subcommands; roles via `--role`. | The baseline names the binary `bothy`; B1 describes the server role. One image for all roles. |
| R-AR-02 | Modules `gateway` and `search` without tables of their own; `stats` with exactly one table (`stats_minutes`). | Their own ports, dependencies and performance targets; otherwise they would bloat `sync`, `messaging` or `runs`. The own measurement needs a store that belongs to no domain module (Ruling T7). |
| R-AR-03 | The port is the root package with its own DTOs. | The domain stays changeable internally; a later process boundary without rebuilding the callers. |
| R-AR-04 | Module dependencies form a graph without cycles; the reverse direction only via source ports or in-tx events. | Separability and comprehensibility; Go forbids import cycles anyway. |
| R-AR-05 | Cross-cutting ports in `internal/kernel`. | Every module needs `authorize`, events, audit, Unit of Work; as module edges they would create cycles. |
| R-AR-06 | Ambient transaction in the context, Unit of Work with fixed phases before the commit. | Enforces the lock order from 03 centrally; modules cannot get it wrong. |
| R-AR-07 | No external calls in transactions. | Short serialized commit phase, safe retry. |
| R-AR-08 | Outbound effects via worker consumers with a cursor, not via job inserts in the write path. | Producers do not know the consumers; the log already is the outbox. |
| R-AR-09 | River only in `platform/jobs`, own schema, no Pro features. | 0.x dependency in one place; the domain schema stays free of foreign tables. |
| R-AR-10 | Ephemeral events across nodes only as an ID signal (`NOTIFY ephemeral`), only in mode `multi`. | R-Stack: NOTIFY without content; every NOTIFY costs an instance-wide lock at commit. |
| R-AR-11 | Gateway sessions only in the process; another node answers `resume` with `4006`. | No shared session store; `identify` with a cursor delivers the same catch-up. |
| R-AR-12 | Rate limits in the process, per node when there are several nodes. | No further service; limits are protection, not billing. |
| R-AR-13 | Health on the internal listener outside the public API; metrics only via OTLP, no metrics endpoint (ADR 0034). | Usable without login for the orchestrator, but not public; no action in the catalogue needed. |
| R-AR-14 | Proof per request that `authorize` ran with the action of the route. | Holds B7 at runtime, not only in the spec. |
| R-AR-15 | Separation of configuration (instance operator, restart) and settings (owner, database, immediate); unknown keys abort the start. | Clear responsibility; typos show up immediately. |
| R-AR-16 | Migrations only via `bothy migrate`, never implicitly at start; the server checks the schema version. | No concurrent migration by several nodes, no restart loop. |
| R-AR-17 | Shutdown closes WebSockets with `4013` and saves the agent cursors before that. | `http.Server.Shutdown` does not cover hijacked connections; clients resume in an orderly way. |
| R-AR-18 | In the request path, only a retry of the whole Unit of Work on deadlock and lock timeout. | Other retries belong in jobs or clients. |
| R-AR-19 | Architecture gate with go-arch-lint, configuration generated from `modules.yaml` ([ADR 0050](../adr/0050-module-register-as-the-source-for-architecture-gate-and-table-ownership.md)). | One source for module boundaries; the tool checks packages and third-party libraries. |
| R-AR-20 | Performance targets as named values, held by load tests in `make perf` ([ADR 0063](../adr/0063-performance-gate-for-the-event-log.md)). | No number without a test. |

## 20. Alignment with the core contract

The items that this chapter had requested as additions while it was drafted are now in the
core contract (integration of 2026-10-10):

| Item | Where |
|---|---|
| table `event_consumer_cursors` for the worker consumers | 03, section 4.10; [ADR 0051](../adr/0051-worker-consumers-of-the-event-log-with-a-cursor.md) |
| River tables as technical tables | 03, section 4.13 |
| fencing of the agent consumers (`holder_session_id`) | 03 `agent_consumer_cursors`; 04, section 15.1 |
| catch-up with fully visible spaces; the cache follows structure events | 04, sections 7.2 and 8.1 |
| expiry produces `access.changed` via `authz.expiry_sweep` | 04, section 8.2; 05 `grant.expire`, `group.member.expire` |
| moving a thread: copies of `channel_id`, files follow the message | 03 `threads`; 05, section 2.2 |
| path prefix `/api/v1` | 04, header and section 6.1 |
| machine-readable action catalogue | [`docs/spec/actions.yaml`](actions.yaml) ([ADR 0049](../adr/0049-machine-readable-action-catalogue-as-the-single-source.md)), checked by `make catalog-check` (05, rule 3) |
| health without an action, on the internal listener | 05, R-AZ-22; section 15.2 |
| modules `gateway`, `search` without tables; `stats` with `stats_minutes` | 03, sections 2 and 4.15 |
| `agent.read` for the agent itself | 05, section 4.10 |
| unique live box anchor | 03 `messages`, partial index on `run_id` |
| channel `ephemeral` in mode `multi` | 03, section 6.6; 04, section 12 |

## 21. Open

- Tool for Go types from AsyncAPI 3.0: decided by [06-api.md](06-api.md); until then
  handwritten types with a contract test against the schemas.
- Whether `PollOnly` is needed for River under load (River's own NOTIFY load) is shown by
  `perf/event_commit`. The option exists [Q3, verified].

## 22. Configuration values of this chapter

Defaults. Performance targets (`perf.*`, place `Load test`) are thresholds for tests, not
runtime configuration: they have no environment name, are not in the configuration schema of the
server, and `make perf` reads them from this table ([ADR 0063](../adr/0063-performance-gate-for-the-event-log.md)).
Listeners, role, public URL, trusted proxies and the connection data of the database
(`server.*`, `database.host` to `database.max_conns`) are listed in [16-operations.md](16-operations.md);
the limits of outbound calls (`egress.*`) are listed in [19-security.md](19-security.md).

| Name | Place | Default | Meaning |
|---|---|---|---|
| `cluster.mode` | Server | `single` | `single` or `multi` (several api nodes) |
| `http.read_header_timeout` | Server | `5s` | deadline for headers |
| `http.request_timeout` | Server | `15s` | context deadline of normal requests |
| `http.transfer_timeout` | Server | `10m` | deadline for upload and download |
| `http.idle_timeout` | Server | `120s` | idle time of keep-alive connections |
| `http.max_header_bytes` | Server | `65536` | largest headers |
| `http.max_json_body_bytes` | Server | `1048576` | largest JSON body; strictly greater than `message.max_content_bytes` (13, item 4) |
| `http.max_json_depth` | Server | `32` | largest nesting depth of a JSON body (SEC-IN-2); a test holds that every request example in `openapi.yaml` stays below it |
| `http.unavailable_retry_after` | Server | `5s` | `Retry-After` of every `503 unavailable` response (in whole seconds) |
| `database.acquire_timeout` | Server | `2s` | waiting for a pool connection |
| `database.statement_timeout` | Server | `5s` | deadline per SQL statement |
| `database.lock_timeout` | Server | `2s` | deadline for locks |
| `database.idle_in_tx_timeout` | Server | `10s` | open transaction without work |
| `database.tx_retry_max` | Server | `2` | retries of the Unit of Work on `40P01` and `55P03` |
| `database.listen_reconnect_backoff_cap` | Server | `10s` | largest interval when re-establishing `LISTEN` |
| `startup.dependency_wait` | Server | `60s` | waiting for the database at start |
| `sync.log_poll_interval` | Server | `1s` | poll of the log tail without `NOTIFY` |
| `sync.tail_batch_size` | Server | `1000` | events per read of the log tail |
| `sync.partition_maintenance_interval` | Server | `10m` | interval of the job `sync.partition_maintenance` |
| `sync.event_log_partitions_ahead` | Server | `3` | partitions of `event_log` that are ready ahead of the head; with `sync.event_log_partition_size` they last longer than two intervals `sync.partition_maintenance_interval` at `perf.event_commit_min_per_s` |
| `gateway.write_timeout` | Server | `10s` | deadline for one WebSocket write |
| `consumer.batch_size` | Server | `500` | events per round of a worker consumer |
| `consumer.poll_interval` | Server | `1s` | poll of a worker consumer without `NOTIFY` |
| `consumer.pending_alert_after` | Server | `15m` | age of the oldest unprocessed event of a worker consumer that raises the alarm `BothyEventConsumerLag`; strictly below `sync.event_retention` (13, item 4) |
| `sync.idempotency_cleanup_interval` | Server | `1h` | interval of the job `sync.idempotency_cleanup` |
| `sync.idempotency_cleanup_batch_size` | Server | `1000` | rows per deletion batch of `sync.idempotency_cleanup` |
| `ratelimit.max_tracked_keys` | Server | `100000` | a process holds at most this many rate-limit buckets; beyond that, full buckets are removed first, then the oldest buckets |
| `ratelimit.sweep_interval` | Server | `1m` | interval at which full buckets are removed (a full bucket decides like a missing one) |
| `jobs.schema` | Server | `jobs` | PostgreSQL schema of the River tables |
| `jobs.queue.default.max_workers` | Server | `8` | parallelism `default` |
| `jobs.queue.files.max_workers` | Server | `2` | parallelism `files` |
| `jobs.queue.delivery.max_workers` | Server | `16` | parallelism `delivery` |
| `jobs.queue.maintenance.max_workers` | Server | `2` | parallelism `maintenance` |
| `jobs.queue.privacy.max_workers` | Server | `1` | parallelism `privacy` |
| `shutdown.drain_delay` | Server | `5s` | waiting after `ready = false` |
| `shutdown.http_grace` | Server | `20s` | deadline for running HTTP requests |
| `shutdown.jobs_soft_timeout` | Server | `20s` | soft stop of River |
| `shutdown.telemetry_flush_timeout` | Server | `5s` | flushing telemetry |
| `shutdown.total_timeout` | Server | `60s` | total deadline until the end; at least the sum of the step budgets (15.3) |
| `perf.reference_profile` | Load test | `4 vCPU, 8 GiB RAM, SSD, PostgreSQL 18 local with fsync and synchronous_commit on, role all` | environment of the load tests; measurement modes in ADR 0063 |
| `perf.sustained_messages_per_s` | Load test | `200` | sustained load for `perf.message_create_p99` |
| `perf.message_create_p99` | Load test | `150ms` | target |
| `perf.event_commit_min_per_s` | Load test | `1000` | target (minimum) |
| `perf.fanout_local_p99` | Load test | `200ms` | target |
| `perf.fanout_cross_node_p99` | Load test | `500ms` | target |
| `perf.invocation_delivery_p99` | Load test | `500ms` | target |
| `perf.run_patch_fanout_p99` | Load test | `500ms` | target |
| `perf.catchup_page_p99` | Load test | `250ms` | target |
| `perf.authorize_cached_p99` | Load test | `100us` | target |
| `perf.ws_connections_per_node` | Load test | `5000` | target (minimum) |
| `perf.ws_idle_conn_mem_max` | Load test | `64KiB` | target (maximum) |
| `perf.idle_rss_max` | Load test | `256MiB` | target (maximum) |
| `perf.startup_ready_max` | Load test | `5s` | target (maximum) |
| `perf.push_handoff_p95` | Load test | `2s` | target |

## Sources

All retrieved on 2026-10-10.

- [Q1] PostgreSQL, NOTIFY: https://www.postgresql.org/docs/current/sql-notify.html. Delivery only
  after the commit, payload limit. [verified]
- [Q2] Recall.ai, "Postgres LISTEN/NOTIFY does not scale":
  https://recall.ai/blog/postgres-listen-notify-does-not-scale. A transaction with NOTIFY takes
  an instance-wide lock at commit. [verified]
- [Q3] River, `client.go` (fields `PollOnly`, `SoftStopTimeout`, `FetchPollInterval`):
  https://github.com/riverqueue/river/blob/master/client.go. [verified]
- [Q4] River, Graceful Shutdown: https://riverqueue.com/docs/graceful-shutdown. Soft and hard
  stop; jobs must return an error on cancellation. [verified]
- [Q5] River, Insert-only clients and `InsertTx`: https://riverqueue.com/docs/insert-only-clients
  and https://riverqueue.com/docs. [verified]
- [Q6] River, migrations of the pgx driver:
  https://github.com/riverqueue/river/tree/master/riverdriver/riverpgxv5/migration/main
  (main branch). [verified]
- [Q7] River, `rivermigrate`: https://pkg.go.dev/github.com/riverqueue/river/rivermigrate (v0.49.0).
  [verified]
- [Q8] go-arch-lint, syntax: https://github.com/fe3dback/go-arch-lint/blob/master/docs/syntax/README.md;
  version v1.19.0 of 2026-09-07 according to https://proxy.golang.org/github.com/fe3dback/go-arch-lint/@latest.
  [verified]
- [Q9] Go, `http.Server.Shutdown`: https://pkg.go.dev/net/http#Server.Shutdown (read in the source
  code of go1.27.2). [verified]
- [Q10] Go, `http.Request.Pattern`: https://pkg.go.dev/net/http#Request (read in the source code of
  go1.27.2). [verified]
- [Q11] oapi-codegen runtime, `StrictHTTPMiddlewareFunc(f, operationID)`:
  https://pkg.go.dev/github.com/oapi-codegen/runtime/strictmiddleware/nethttp (v1.7.0). [verified]
