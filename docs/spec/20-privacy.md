# 20 Privacy

Status: draft M0. Normative for retention, deletion, export and telemetry. Not legal advice:
the instance operator decides legal bases, periods with legal effect and balancing of interests; the
product supplies mechanisms, defaults and templates.
Tables and retention classes: [03-data-model.md](03-data-model.md), sections 3 and 4.
Permissions: [07-permissions.md](07-permissions.md). Security: [19-security.md](19-security.md). Templates:
[../compliance/ropa-template.md](../compliance/ropa-template.md) (record of
processing activities), [../compliance/processors-template.md](../compliance/processors-template.md)
(processors).

Legal source: Regulation (EU) 2016/679 (GDPR), https://eur-lex.europa.eu/eli/reg/2016/679/oj;
article texts read via https://gdpr-info.eu, retrieved 2026-10-10.

## 1. Roles

| Role | Who | Consequence |
|---|---|---|
| Controller | Instance operator (organization of the owner) | decides on purposes, legal bases, periods; keeps the record |
| Processor | Service providers that the instance operator engages: hosting, object storage in the cloud, push services, LLM providers, telemetry backends, transcription service, identity providers | contracts under Art. 28 GDPR; list according to the template |
| Manufacturer of the product | Project of the open source product | processes no data of the instance; not a processor as long as no hosting is offered |
| Data subjects | People of the instance; third parties who appear in contents; persons from imported histories | rights under Art. 12 to 22 GDPR |

## 2. Data categories per table

Legend for personal reference: **direct** (identifies a person), **indirect** (attributable to a
person via a principal ID or a device), **content** (free text or file; can contain any
personal data, also of third parties and also special categories under Art. 9),
**no**. Class: retention class from 03. Deletion path: section 4.

| Table | Personal reference | Data categories | Class | Deletion path |
|---|---|---|---|---|
| `instance` | no | identifier of the installation | `config` | none |
| `instance_settings` | indirect | settings, last changed by | `config` | overwritten on change |
| `server_signing_keys` | no | public keys, references | `security` | after `retired` and `retention.security_after_end` |
| `feature_flags` | indirect | switches, lists of persons and groups | `config` | on change |
| `sequence_counters` | no | counters | `config` | none |
| `principals` | direct | handle, display name, avatar, status, times | `account` | pseudonymization (4.4) |
| `users` | direct | email (display), language, time zone, last activity | `account` | pseudonymization |
| `owners` | indirect | role, reason, times | `account` | stays with a pseudonymous ID |
| `identity_providers` | no | configuration, references to secrets | `config` | on change |
| `user_identities` | direct | subject at the provider, email at linking, last login | `account` | deleted on account deletion |
| `passkey_credentials` | indirect | public key, name, usage | `security` | deleted on revocation plus a period, immediately on account deletion |
| `devices` | direct | device name (often with names of persons), platform, version, last seen | `security` | as above |
| `device_keys` | indirect | public keys, attestation | `security` | as above |
| `auth_sessions` | direct | login method, times, last IP | `security` | after the end plus `retention.security_after_end` |
| `refresh_token_families`, `refresh_tokens` | indirect | hashes, times | `security` | as above |
| `api_tokens` | indirect | name, prefix, hash, usage | `security` | as above |
| `groups` | indirect | group name (can map the organization) | `account` | soft-deleted, then like `account` |
| `group_memberships` | indirect | membership, times | `account` | removed on account deletion, row with a pseudonymous ID |
| `grants` | indirect | who has access to what, reason (free text) | `account` | stays with a pseudonymous ID (evidence) |
| `agents` | indirect | `credential_holder_id`, reported state | `account` | stays |
| `agent_consumer_cursors` | no | read position | `security` | with the agent |
| `apps`, `app_installations` | indirect | manifest, installed by | `account` / `config` | stays |
| `commands`, `channel_skill_bindings`, `project_bindings`, `forum_tags` | no | configuration | `config` / `content` | on change |
| `spaces` | indirect | name, created by | `content` | archiving; contents after `retention.content` |
| `user_space_states` | indirect | personal sidebar | `personal_sync` | with the account |
| `channels` | content | name, topic, channel prompt | `content` | archiving; after `retention.content` |
| `dm_participants` | indirect | who writes with whom | `content` | with the DM channel |
| `threads` | content | title, times, created by | `content` | `retention.content` |
| `thread_tags` | indirect | set by | `content` | with the thread |
| `agent_sessions` | indirect | external session reference, costs | `content` | with the thread |
| `messages` | content | text, author, times, trigger state | `content` | tombstone (4.2), `retention.content` |
| `message_revisions` | content | earlier versions | `content` | with the message |
| `mentions` | indirect | who mentions whom | `content` | with the message |
| `reactions` | indirect | who reacts how | `content` | with the message |
| `pins` | indirect | pinned by | `content` | with the message |
| `message_templates` | content | template text | `content` | personal ones with the account |
| `files`, `file_thumbnails` | content | file, name, transcript, images | `content` | 4.3 |
| `file_upload_sessions`, `file_upload_parts` | indirect | declared size and checksum, parts | `transient` | with completion or expiry of the session |
| `file_text_extracts` | content | text extracted from files | `content` | with the file |
| `message_attachments` | no | assignment | `content` | with the message |
| `runs` | content | trigger, status line, last text excerpt, metrics | `content` | with the thread |
| `run_steps` | content | tool names, details (can contain paths, commands, data) | `content` | with the run |
| `run_followups` | indirect | assignment | `content` | with the run |
| `message_components` | content | definition, used by | `content` | with the message |
| `interactions` | content | form values, principal | `content` | with the message |
| `price_table` | no | prices | `cost` | `retention.cost` |
| `cost_entries` | indirect | costs per run (attributable to a person through the run) | `cost` | `retention.cost` |
| `event_log` | content | copies of changes in payloads | `event_log` | window `sync.event_retention` (04); redaction on deletion |
| `event_consumer_cursors` | no | read position of the worker consumers | `config` | stays |
| `idempotency_keys` | content | stored responses | `transient` | `sync.idempotency_key_ttl` (04) |
| `read_states` | indirect | read state per thread | `personal_sync` | with the account; never visible to others (section 9) |
| `drafts` | content | drafts | `personal_sync` | with the account |
| `user_settings`, `notification_preferences` | indirect | settings | `personal_sync` | with the account |
| `quiet_hours` | indirect | quiet hours (reveal working hours) | `personal_sync` | with the account |
| `push_registrations` | direct | push token of the device | `security` | with the device or the account |
| `webhook_subscriptions`, `incoming_webhooks` | indirect | target, created by | `config` | on revocation |
| `webhook_deliveries` | no | delivery attempts | `transient` | `retention.webhook_deliveries` |
| `import_sources`, `import_items` | indirect | identifiers of the source (also foreign person IDs in the origin key) | `import` | `retention.import_after_completion` |
| `audit_events` | indirect | actor, action, decision, resource, pseudonymized IP | `audit` | `retention.audit`, truncation with an anchor (03) |
| `audit_checkpoints` | no | sequence, hash, signature | `audit` | like `audit_events` |
| `gdpr_requests` | direct | data subject, kind, reason, state | `privacy_job` | `retention.privacy_job` |
| `status_components`, `status_incidents`, `status_incident_updates`, `status_incident_components`, `backup_runs` | no | operational status, backup runs | `ops` | `retention.ops` |
| `local_credentials` (03, 08) | indirect | password hash, encrypted TOTP secret | `security` | immediately on account deletion |
| `enrollment_codes` (03, 08) | indirect | hash, times | `security` | `retention.security_after_end` |
| `auth_login_flows` (03, 08) | direct | login flow; between callback and completion the verified identity from the provider (`external_identity`: subject, display name, email hint), empty afterwards | `transient` | `auth.login_flow_ttl` |
| `device_pairings` (03, 08) | indirect | pairing process with public device keys | `transient` | `auth.pairing_code_ttl` |
| `auth_challenges` (03, 08) | indirect | one-time challenges for step-up and passkeys, counted failed attempts | `transient` | `auth.step_up_challenge_ttl` or `auth.login_flow_ttl`, respectively, deleted by the job `identity.auth_cleanup`; a failed step-up challenge stays until its `finished_at` leaves `auth.login_fail_window` (08 section 13) |
| `dev_environments` (03, 21; milestone) | indirect | environment of a person, check results | `security` | after `retired` plus `retention.security_after_end` |
| `hosts` (03, 24) | indirect | host name (often with names of persons), host assignee, holder of the LLM login, public keys, operating system, reported names of the session users with display names of their working folders, check results (with the name of every session user or former session user that the rule for names from 24, section 2.2 names; never names of other users of the host), last seen | `security` | after `retired` plus `retention.security_after_end` |
| `host_session_users` (03, 24) | indirect | which person has or had which session user on which host, who assigned and ended it | `security` | after `retired` of the host plus `retention.security_after_end`; never deleted before that, so that no name becomes free again (24, section 2.2); on account deletion with a pseudonymous ID |
| `terminal_sessions` (03, 24) | indirect | who worked when from which device on which host with which profile, duration, bytes per direction, reason for the end; never contents | `security` | after `closed_at` plus `retention.security_after_end`; on account deletion with a pseudonymous ID |
| `stats_minutes` (03, 25) | no | minute aggregates per channel, agent and code (count, sum, good events); no column for people, no contents. In channels with a single person indirectly relatable to that person (section 9) | `stats` | detaching the daily partitions after `retention.stats_minutes` |

Data outside the database:

| Place | Content | Rule |
|---|---|---|
| Object storage | files, thumbnails, export packages, deletion journal | Section 4 |
| Logs | IDs, route patterns, errors; no contents (19, section 9) | retention in operations ([16-operations.md](16-operations.md)) |
| Telemetry | metrics and traces without contents, pseudonymous IDs (section 7) | retention in the backend of the instance operator |
| Backups | image of the database and the object storage | Section 4.6 |
| Client cache | copies of contents on devices | 04, section 13.1; deletion on withdrawal of access and on revocation |
| Agent runtimes and LLM providers | contents that an agent receives | Section 6 |
| Connected hosts | terminal contents, scrollback buffer only in the memory of the host service; what the shell writes there | outside the server; responsibility of whoever runs the host ([24-terminal.md](24-terminal.md), section 10.4) |

## 3. Legal bases

The instance operator sets the legal basis per processing activity. Placeholders for the
record (template, section "Activities"):

| Processing activity | Purpose | Legal basis (to be filled in by the instance operator) |
|---|---|---|
| Team chat | internal communication | `<for example Art. 6(1)(b) or (f) GDPR; for employees, also check national employee data protection law>` |
| Agent processing | tasks by AI agents | `<...>` |
| Login, devices, security log | access protection, evidence | `<for example Art. 6(1)(c) or (f) in conjunction with Art. 32>` |
| Push notifications | notification on devices | `<...>` |
| Telemetry and operations | stability, troubleshooting | `<...>` |
| Cost tracking | attribution of agent costs | `<...>` |
| Import from earlier systems | takeover of histories | `<...>` |
| Data subject requests | fulfillment of the rights under Art. 15 to 22 | Art. 6(1)(c) GDPR |

## 4. Retention and deletion

### 4.1 Periods per retention class

| Class | Period (configuration value) | Mechanism |
|---|---|---|
| `config` | until the change | overwrite |
| `account` | until account deletion, then pseudonymization | privacy request (4.4) |
| `security` | `retention.security_after_end` after expiry, revocation or end | job `retention.security` |
| `content` | `retention.content` (default: no automatic deletion) | job; tombstones as in 4.2 |
| `personal_sync` | follows the account | with the account deletion |
| `event_log` | `sync.event_retention` (04) | detaching whole partitions (R-DM-15) |
| `transient` | `sync.idempotency_key_ttl` (04), `retention.webhook_deliveries` | job |
| `audit` | `retention.audit` | truncation through the maintenance role with an anchor entry (03, `audit_events`) |
| `cost` | `retention.cost` | job |
| `import` | `retention.import_after_completion` after completion of the import | job |
| `privacy_job` | `retention.privacy_job`; export packages `privacy.export_ttl` | job `retention.privacy_job` (finished requests, M7-06); packages through `privacy.export_expiry` |
| `ops` | `retention.ops` | job |
| `stats` | `retention.stats_minutes` | job `stats.retention` detaches whole daily partitions; never shorter than the longest SLO period ([25-visualization.md](25-visualization.md), section 3.5) |

The jobs are named `retention.<class>` as in the job catalogue ([02-architecture.md](02-architecture.md));
only the class `stats` has its own job `stats.retention`.

How the jobs `retention.<class>` work (M7-05, ruling D-M7-05-1 to D-M7-05-6):

- Each module that owns tables of a class with a job registers a participant of the kernel port
  `retention` for them and deletes only its own tables. A job runs every `retention.job_interval`
  on the queue `maintenance`, calls the participants of its class in batches of
  `retention.batch_size` rows per table, each batch in a short transaction, until nothing is
  left or `retention.job_interval` has passed, and reports the removed rows per class as the
  metric `bothy.retention.deleted` without IDs. A participant that fails ends only its own part of
  the run; the others of the class still run.
- A row is removed only when its end time plus the period lies before the current time (DP-T05);
  a row without an end time is never removed. The end time per table is the expiry, revocation,
  use, retirement or close named in the README of the owning module. A row whose deletion a rule
  of another row refuses (a foreign key, or a check that an `ON DELETE SET NULL` would break) stays
  until that row is gone.
- `security`: after `retention.security_after_end`; signing keys of the purpose `audit_checkpoint`
  stay, because `audit.verify` checks every checkpoint of the sink with its key.
- `content`: with a period, every message created before it is deleted through the deletion of
  4.2 (tombstone, redaction, attachments through the purge of 4.3), without an actor: no audit
  entry, `message.deleted` without `deleted_by`. Spaces, channels, threads and runs are not
  removed by time (03, principle 7); they go with their parent or by archiving as in section 2.
- `transient`: finished deliveries of outgoing webhooks after `retention.webhook_deliveries`.
- `audit`: the trim of 4.5 and the destruction of the IP keys.
- `cost`: cost entries and closed price rows after `retention.cost`.
- `privacy_job` (M7-06): privacy requests in a final state (`completed`, `failed`, `rejected`,
  `cancelled`) after `retention.privacy_job` from `completed_at`; a completed export stays until
  `privacy.export_expiry` has deleted its package.

`retention.content` deliberately has no default period: only the instance operator knows how long a
team history is needed. The decision is in the configuration, not in the console: if
`retention.content = none` applies and `retention.content_none_reason` is empty,
`bothy config check` reports a warning, and the server writes a warning to the log at startup. A
set reason or a period ends the warning; the server writes the effective value to the log at
startup. Reason: storage limitation (Art. 5(1)(e)
GDPR) must be set by the controller; a silent default would be a hidden decision.
Reason for the configuration instead of a confirmation in the console: the server reads values only at
process start ([16-operations.md](16-operations.md), section 5.1); a confirmation in the console
would have neither a storage place nor an effect. An audit entry in the product is therefore not needed; the instance
operator tracks changes of the configuration in their own version control.

### 4.2 Deleting a message

In one transaction (03, section 6.5): empty `content_md`, `deleted_at`, remove revisions,
mentions, components, reactions, pin and attachments, the search vector becomes empty through the
generated column, redact the payloads of the related events (forms in 04, section
2.2), event `message.deleted`. Afterwards:

- Clients remove the message from the cache and the search index on the event.
- Agent adapters receive `message.deleted` and must remove the message in their runtime where
  possible (contract in [09-agents.md](09-agents.md)). What an LLM provider has already
  processed, the provider deletes according to its periods (section 6).
- Entry in the deletion journal (4.6).

### 4.3 Files

A file is deleted with its message or through `file.delete`: row `deleted_at`; the deleting
transaction also redacts the payloads of the stored `file.updated` events of the file (R-DM-45) and
queues the job `files.purge_objects`, which removes the originals, the thumbnails and the parts and
then the row itself (R-DM-46) within `privacy.object_delete_delay`; the cleanup job
`files.cleanup_pending` catches up a purge that failed ([12-files.md](12-files.md) section 12,
M3-08). No deduplication (R-DM-13): deleting a file never affects the file of another person.

### 4.4 Deleting an account

A privacy request `deletion` (05, `gdpr.request.*`) is decided by the owner and then run as a job,
in this order:

1. Disable the account like `user.disable` (08, section 11), if still active. Last owner: rejected.
2. Delete security data: identities, passkeys, local credentials, devices, keys,
   sessions, tokens, push registrations, enrollment codes.
3. Delete personal data: sidebar, read state, drafts, settings,
   notifications, quiet hours, personal templates.
4. Remove memberships, revoke grants (rows stay with the ID as evidence).
5. Contents according to `privacy.account_deletion_content`:
    - `keep_pseudonymized` (default): messages, files and reactions stay, the author
      appears as "Deleted person".
    - `delete`: all own messages are deleted as in 4.2, own files as in 4.3.
6. Pseudonymize: `display_name` becomes "Deleted person" with a short random identifier, `handle`
   becomes `deleted-<random>`, avatar removed, `users.email = NULL`, time zone and language set to
   the default, `principals.status = 'deleted'`, `deleted_at`.
7. Audit entry; entry in the deletion journal.

How the job `privacy.erase` runs (M7-06, rulings D-M7-06-3 and D-M7-06-5): every module with
tables of section 2 implements the kernel port `personaldata.Participant` for its own tables; the
job calls all participants for step 1, then all for step 2 and so on, each call in its own
transactions, and records the last finished step in `gdpr_requests.progress`, so a crashed job
resumes at the next step. Every step is idempotent. Within a step, notify runs before identity
(push registrations hold the devices) and the content modules before identity. The content mode
is taken from `privacy.account_deletion_content` when the job starts and kept in `progress`. A
request of the last active owner is refused at creation, at the approval and again in step 1
(`409 last_owner`; the job ends the request as `failed` with `last_owner`). Step 7 writes the
audit entry `privacy.erase` (system, resource the person, request ID and content mode); the
journal entry follows with M7-07.

Important for the instance operator: pseudonymized data remains personal data as long as an
attribution is possible, for example through contents (Recital 26 GDPR). The default
`keep_pseudonymized` keeps conversation histories readable for the other participants; whether that
holds in an individual case is decided by the instance operator. The ID stays so that sequences, the
audit chain and replies with a quote stay consistent (03, principle 7).

### 4.5 Audit log and IP addresses

- The audit log contains only IDs, actions, decisions and codes; never contents, names, emails
  (03, `audit_events.details`). With the pseudonymization of the principal, the IDs in the audit log
  become pseudonymous without any row being changed. The hash chain therefore stays valid (ADR 0021).
- IP addresses are in the audit log only as an HMAC with one key per period
  (`privacy.audit_ip_key_period`). The keys are stored encrypted in `audit_ip_keys` (03); the
  key for that comes from the secret provider (`privacy.audit_ip_kek_ref`). Keys are destroyed after
  `privacy.audit_ip_key_retention` (column emptied, row stays as a record). After that
  the values can no longer be traced back to an IP; the chain stays unchanged. Within the period
  the owner can check for an investigation whether a known IP matches. Reason for keys in the
  database: they must be destroyable one by one, and the file driver of the secret provider creates
  no keys per period. Backups contain a destroyed key at most until
  the end of their own period (16, section 8.1).
- `auth_sessions.last_ip` is plaintext for the security view and is removed with the session after
  `retention.security_after_end`.
- At the end of `retention.audit` the log is truncated; an anchor entry keeps it verifiable (03).
  The job `retention.audit` connects as `bothy_maint` only for the trim and, in one
  transaction, writes the anchor entry (action `audit.retention_trim`, details `anchor_seq` and
  `anchor_hash`: the last removed row) and removes the rows up to it and the checkpoints up to it.
  It never removes the newest row nor the row of the newest checkpoint stored at
  `audit.checkpoint_sink` or a later row, so only rows that a checkpoint outside the database
  covers are removed; without a configured sink the log is not trimmed. `audit.verify` and the
  restore test start at the first retained row with the hash of the anchor that names the row
  before it. A checkpoint of the sink below the anchor is not compared, but it must be older than
  `retention.audit`, one at the anchor must equal it, and the newest one must lie after the anchor;
  otherwise the result is `checkpoint_mismatch`, so a forged anchor cannot hide an early deletion
  (M7-05).

### 4.6 Backups and deletion journal

- Deleted data stays in backups until they expire after the backup period ([16-operations.md](16-operations.md)).
  It is not brought back into operation.
- Deletion journal: every deletion (message, file, account) writes an entry `{resource_kind,
  resource_id, deleted_at, reason}` without contents. The journal is written continuously and separately
  from the database storage to the backup target and kept for `privacy.deletion_journal_retention`;
  this value must be at least as long as the backup period (startup check).
- Every restore to an earlier state applies the journal before the instance accepts requests
  again (19, SEC-BAK-3). When a restore with running WAL archiving reaches the latest state,
  the journal is in effect empty.

### 4.7 Data outside the server

| Place | Rule |
|---|---|
| Client caches | deletion on `message.deleted`, `access.changed` with `removed`, revocation (`4012`), after `client.cache_max_age` unused (04) |
| Agent runtimes | receive deletion events; whoever runs the runtime is responsible for storage in the runtime; list them as separate recipients in the record |
| LLM providers | periods of the provider; example Anthropic API: inputs and outputs deleted "within 30 days", for policy violations up to 2 years (https://privacy.claude.com/en/articles/7996866-how-long-do-you-store-my-organization-s-data, retrieved 2026-10-10 [verified]) |
| Push services | no contents (ADR 0031) |
| Telemetry backends | no contents; period in the backend |

## 5. Access, export, rectification

### 5.1 Flow

1. The person files the request themselves (`gdpr.request.create`, step-up) or the owner files it for them.
2. Export of one's own data is approved automatically, unless `privacy.export_requires_approval`
   is set. Deletion always needs the decision of the owner (`gdpr.request.decide`), because
   retention obligations can stand against it.
3. Deadline: `gdpr_requests.due_at` = receipt plus one calendar month. Extension by two further
   months only with a justification that is communicated to the person within the first month
   (Art. 12(3) GDPR, https://gdpr-info.eu/art-12-gdpr/, retrieved 2026-10-10 [verified]). The
   deadline is not a configuration value because it is statutory.
4. The console warns before the deadline expires (index `(state, due_at)` from 03). Owners also get the personal event `gdpr.request_due` once per open request (05, R-AZ-35), which becomes a hint of kind `admin` ([13-push-notifications.md](13-push-notifications.md), section 2). The job `privacy.request_reminder` sends it when `due_at` minus `privacy.request_reminder_lead` has passed, to every active owner, once (recorded in `progress`).
5. The package is available for `privacy.export_ttl`; download with step-up (`gdpr.export.download`), then
   deleted by the job `privacy.export_expiry` (file and object; the download answers `410 gone`).

Calendar month (ruling D-M7-06-1): `due_at` is the receipt in UTC plus one calendar month, the same
day of the month; a day that does not exist in the target month moves to the last day of that month
(31 January plus one month is the last day of February), as Regulation (EEC, Euratom) No 1182/71,
Art. 3(2)(c) computes periods in months. The extension of Art. 12(3) is the optional
`extend_months` (1 or 2) of the decision, once and only before the original `due_at`, with the
`reason` that is communicated to the person; it moves `due_at` and is in the audit entry of the
decision. One request per person and kind may be open; a second one is `409 conflict` naming the
open request. The export of one's own data is approved by the system only when the person files it
themselves; an export the owner files for another person waits for an owner's decision, like a
deletion. A `rectification` is completed by the owner's approval, whose `reason` documents the
correction (5.3).

### 5.2 Export format

A ZIP archive with JSON files (Art. 20(1) GDPR requires a "structured, commonly used and
machine-readable format", https://gdpr-info.eu/art-20-gdpr/, retrieved 2026-10-10 [verified]):

```text
export-<user_id>-<date>.zip
  README.txt                 explanation of the files in the language of the person
  manifest.json              schema version, instance, creation time, SHA-256 of every file
  profile.json               principals, users (own fields)
  identities.json            linked identities (without secrets)
  devices.json               devices, sessions (times, login method, last IP)
  access.json                own grants, groups, owner status, with times and reason
  messages.jsonl             own messages with revisions, thread and channel ID, times
  reactions.jsonl            own reactions
  mentions.jsonl             mentions of the person (only message ID and time)
  files/                     own uploads with files.json (name, type, size, checksum)
  runs.jsonl                 runs triggered by the person (state, times, costs)
  interactions.jsonl         own interactions with values
  personal.json              read state, drafts, settings, notifications, quiet hours, templates
  audit.jsonl                audit entries with the person as actor or resource
```

Rules:

- Only own data and metadata. Messages of other persons are not included (Art. 15(4)
  GDPR: rights of others). Channel and thread names only where the person may still read; otherwise
  only IDs.
- No secrets, no hashes, no keys.
- The format is versioned (`manifest.json`, `schema_version`) and described as a schema in
  [06-api.md](06-api.md) section 10: one JSON Schema per file in
  [`docs/api/gdpr-export/v1/`](../api/gdpr-export/v1/README.md).

How the job `privacy.export` builds it (M7-06, ruling D-M7-06-2): each participant writes the
sections of the JSON files and the lines of the JSON Lines files of its own tables into a package in
a temporary directory; every file of the list above is present (an empty object or an empty file
when there is nothing), the uploads are under `files/<file_id>`, `README.txt` is in German for a
person whose `users.locale` is German and in English otherwise, and `manifest.json` is written last
with the SHA-256 of every other file. The ZIP is stored as a file with `purpose = 'gdpr_export'`
and `result_expires_at = now + privacy.export_ttl`. `access.json` also holds the person's own
privacy requests, agents whose credentials the person holds and app installations the person made;
`devices.json` also holds push targets and terminal hosts and sessions.

### 5.3 Rectification

The person changes profile fields themselves (`self.profile.update`), own messages by editing.
The owner rectifies other data; a `rectification` request documents that.

## 6. Agents, LLM providers and processors

- Every grant to an agent and every operator assignment determines which contents go to the
  runtime and from there to an LLM provider. Therefore only the owner creates grants to agents
  (07, section 2.6), and the preview names the data flow.
- The agent card shows per agent the provider note that the instance operator enters (for example
  "processed at provider X, region Y"). Data residency is a property of the runtime, not
  of the server. Example: for the Anthropic API the documentation names only `global` and
  `us` as `inference_geo` (https://platform.claude.com/docs/en/manage-claude/usage-cost-api, retrieved 2026-10-10
  [verified]); EU processing then needs a different path, which the instance operator chooses.
- Data minimization at invocation: the server supplies the thread, `history_backfill_limit` earlier
  messages (03, `channels`) and authors as handle and kind, no email addresses.
- Personal mode (`billing_mode = personal`): the person uses their own subscription for their own
  purposes; the product stores no credentials (baseline I5).

## 7. Telemetry

| Rule | Implementation |
|---|---|
| No contents | no message texts, file names, prompts, transcripts, search terms, titles (baseline B11; 19, SEC-LOG-6) |
| Pseudonymous IDs | people only as an HMAC with one key per period `privacy.telemetry_pseudonym_rotation`; the period key is derived by HMAC from the single secret `telemetry.pseudonym_key_ref` ([privacy.md](../observability/privacy.md) section 3); without this secret not linkable across periods. Device IDs are not in telemetry at all |
| No network data | no IP addresses, no device names, no push tokens |
| Allow list | only named attributes per signal ([17-observability.md](17-observability.md)) |
| LLM attributes | `gen_ai.*` without prompt and response texts |
| Crash reports of the apps | without contents and names; off by default, the owner turns them on |
| Check | canary test (19, SEC-T10) |

## 8. Notice obligations

- **Information for data subjects** (Art. 13 GDPR): the instance operator provides it. The console has
  a field for a link (`privacy.notice_url`); apps show it at the first login and in
  the settings.
- **AI notice:** agent messages always carry a visible badge (19, SEC-OUT-3); it cannot be
  turned off. At the first contact with an agent in a channel the app shows once:
  "Agent replies can be wrong. Check important information." (`privacy.ai_notice_enabled`,
  text adjustable through branding). Basis: Anthropic obliges customers to tell users
  that outputs are not reliable without review (Commercial Terms, section D.3,
  https://www.anthropic.com/legal/commercial-terms, retrieved 2026-10-10 [verified]).
- **Transparency under the AI Act:** Art. 50 of Regulation (EU) 2024/1689 requires that people
  recognize that they interact with an AI system, unless this is obvious; according to
  an evaluation by a law firm this applies since 2026-08-02
  (https://www.goodwinlaw.com/en/insights/publications/2026/08/alerts-technology-dpc-eu-ai-act-transparency-obligations-now-in-force,
  retrieved 2026-10-10 [verified in the secondary source]; regulation:
  https://eur-lex.europa.eu/eli/reg/2024/1689/oj). The badge fulfills the technical part; whether
  obligations fall on the instance operator or the manufacturer is checked by the instance operator.

## 9. Codetermination (note for instance operators)

A team chat with an audit log, costs per run and statistics can be a technical facility
that is able to monitor the behavior or performance of employees. In Germany the works council then
has a right of codetermination under § 87(1) no. 6 BetrVG
(https://www.gesetze-im-internet.de/betrvg/__87.html, retrieved 2026-10-10 [verified]); other countries
have their own rules on employee representation. The product supports an agreement with these
defaults:

| Function | Default |
|---|---|
| Read state | only for the person themselves; no read receipts for others, not even for the owner |
| Presence | only agents have presence; people do not (04, section 12) |
| Last activity | only for the person and in the security view of the owner |
| Statistics per person | off (`privacy.per_person_stats_enabled`); evaluations by groups of persons only from `privacy.stats_min_group_size` persons |
| Costs per person | owners only; in threads one sees costs per run |
| Audit log | owners only, with step-up; every read access is itself in the audit log (05, `audit.read`) |
| "View as person" | step-up, time limit, audit, DMs excluded (07, 9.2) |
| Terminal sessions | metadata only for the person themselves and owners; no content recording; not in the statistics per person ([24-terminal.md](24-terminal.md), section 10) |
| Live statistics and visualizations | on (`stats.live_enabled`); only for owners and space managers, without values per person, without the trigger of a run, without counters about people; in channels with only one person, runs allow conclusions about that person, therefore it can be turned off ([25-visualization.md](25-visualization.md), section 4) |

Every change of these switches is in the audit log. The instance operator checks a data protection
impact assessment (Art. 35 GDPR); agents with access to employee data can be a reason for one.

## 10. Tests

| ID | Test | Expected |
|---|---|---|
| DP-T01 | Delete a message | content, revisions, attachments, search hits, event payloads empty; journal entry |
| DP-T02 | Delete an account with `keep_pseudonymized` and with `delete` | steps from 4.4 verifiable; no clear data in `principals`, `users`; the audit chain still verifies (`audit.verify`) |
| DP-T03 | Export | package matches the schema; no foreign messages, no secrets, no hashes; checksums match |
| DP-T04 | Export expiry | package deleted after `privacy.export_ttl` |
| DP-T05 | Retention job per class with a controlled clock | only expired data removed |
| DP-T06 | IP pseudonym after key destruction | no attribution possible anymore, chain valid |
| DP-T07 | Restore with the deletion journal | deleted data stays deleted (19, SEC-T15) |
| DP-T08 | `privacy.deletion_journal_retention` shorter than the backup period | startup abort |
| DP-T09 | Telemetry canaries (19, SEC-T10) | no contents, no plain IDs |
| DP-T10 | Fetch the read state of another person through the API or events | not possible (`not_found`) |
| DP-T11 | Statistics per person with the switch off or a group that is too small | no output |
| DP-T12 | Live frame, timeline and `stats_minutes` with `privacy.per_person_stats_enabled = true` | still no principal IDs of people, no trigger, no counters about people (25, VZ-T05, VZ-T07) |
| DP-T13 | `retention.content = none` without and with `retention.content_none_reason`; with a period | warning in `bothy config check` and in the startup log only in the first case |

## 11. Decisions of this chapter

| No. | Decision | Reason |
|---|---|---|
| R-DP-01 | No default period for contents; without a period the configuration requires a reason (`retention.content_none_reason`), otherwise `bothy config check` and the startup log warn. Replaces the earlier confirmation in the console, which had no storage place and no action (finding L-12). | The controller sets periods; a silent default hides the decision. |
| R-DP-02 | Account deletion pseudonymizes and keeps contents by default. | Histories of others stay readable; IDs keep the sequence and the audit consistent. Switchable. |
| R-DP-03 | IP addresses in the audit only as an HMAC with destroyable keys. | Investigability for a period, afterwards unlinkability without breaking the hash chain. |
| R-DP-04 | Deletion journal outside the database. | A restore must not bring back deleted data. |
| R-DP-05 | Statutory deadlines (Art. 12) are not configuration values. | A configurable statutory deadline invites mistakes. |
| R-DP-06 | Codetermination-friendly defaults. | Introduction in a company without rework; monitoring functions are deliberate switches. |
| R-DP-07 | Terminal without content recording; only metadata with class `security`. | A terminal shows any data of the host; a recording would be an extraction without purpose limitation. Whether evidence obligations require it is decided by the owner (open question, default no). |
| R-DP-08 | Minute aggregates of the own measurement without persons in a separate class `stats` with its own period; the live stream can be turned off. | Live views need a fine time resolution but no persons; the period is shorter than that of the domain data, because the aggregates only carry the view and the traffic light. |

## 12. Alignment with the core contract

The additions requested during drafting are decided (integration of 2026-10-10):

| No. | Addition | Decision, location |
|---|---|---|
| K-DP-1 | Automatic approval of the export of one's own data | adopted, 03 `gdpr_requests.approved_by` (NULL = system), 05 `gdpr.request.decide` |
| K-DP-2 | IP in the audit only as `ip_hmac` with `ip_key_id` | adopted, 03 `audit_events.details` |

## 13. Configuration values of this chapter

| Name | Default | Meaning |
|---|---|---|
| `retention.content` | `none` | period for contents, at least one day; `none` means no automatic deletion (without `retention.content_none_reason` a warning, 4.1) |
| `retention.content_none_reason` | empty | reason of the instance operator for `retention.content = none`; empty means a warning in `bothy config check` and in the startup log (4.1) |
| `retention.security_after_end` | `90d` | security data after end, expiry or revocation |
| `retention.audit` | `365d` | audit log (Study C suggested twelve months) |
| `retention.cost` | `395d` | costs and prices |
| `retention.import_after_completion` | `90d` | import state after completion |
| `retention.privacy_job` | `1095d` | privacy requests (without export packages) |
| `retention.ops` | `395d` | status page |
| `retention.stats_minutes` | `90d` | minute aggregates of the own measurement (`stats_minutes`); at least the longest SLO period |
| `retention.webhook_deliveries` | `30d` | delivery attempts of outgoing webhooks |
| `retention.job_interval` | `1h` | interval of the jobs `retention.<class>`; it also bounds the time one run may take |
| `retention.batch_size` | `500` | rows a retention job removes per table and transaction |
| `privacy.export_ttl` | `7d` | availability of an export package |
| `privacy.export_requires_approval` | `false` | export of one's own data needs the approval of the owner |
| `privacy.account_deletion_content` | `keep_pseudonymized` | contents of deleted accounts: `keep_pseudonymized` or `delete` |
| `privacy.request_reminder_lead` | `7d` | how long before `due_at` of an open privacy request the owners get `gdpr.request_due`; less than one month |
| `privacy.object_delete_delay` | `24h` | maximum time until deleted objects are removed |
| `privacy.deletion_journal_retention` | `400d` | retention of the deletion journal; at least the backup period |
| `privacy.audit_ip_key_period` | `30d` | period per key for IP pseudonyms |
| `privacy.audit_ip_key_retention` | `90d` | keys are destroyed after this |
| `privacy.audit_ip_kek_ref` | `file:<secrets.dir>/audit_ip_kek` | reference to the key that encrypts the IP keys in `audit_ip_keys` |
| `privacy.telemetry_pseudonym_rotation` | `30d` | rotation of the key for pseudonymous IDs in telemetry |
| `privacy.per_person_stats_enabled` | `false` | statistics per person |
| `privacy.stats_min_group_size` | `5` | smallest group for evaluations by persons |
| `privacy.ai_notice_enabled` | `true` | one-time AI notice per channel with an agent |
| `privacy.notice_url` | empty | link to the privacy information of the instance operator |
