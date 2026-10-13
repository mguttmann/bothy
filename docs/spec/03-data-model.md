# 03 Data model (PostgreSQL)

Status: draft M0, integrated after the feedback from all chapters. Normative for migrations,
sqlc queries and repository ports.
Terms: see [00-glossary.md](00-glossary.md). Sync and events: see
[04-sync-protocol.md](04-sync-protocol.md). Actions: see [05-actions.md](05-actions.md).
Retention and deletion: see [20-privacy.md](20-privacy.md). Search: see
[14-search.md](14-search.md). Tables of a later milestone carry the note "Milestone"; their
migration comes with that milestone, the contract already applies now.

## 1. Principles

1. **IDs.** Every table with its own identity has `id uuid` as its primary key. The value is
   a UUIDv7 (RFC 9562). The application generates it, not the database. Reason: the same logic
   on every PostgreSQL version, and clients can generate IDs in advance (`client_msg_id`).
   UUIDv7 is sorted by time and keeps B-tree indexes compact.
2. **Time.** All points in time are `timestamptz` in UTC. Durations are `bigint` in milliseconds
   (suffix `_ms`). The server clock is the only time source for orderings; order, however, never
   comes from timestamps, but from sequence numbers.
3. **Enumerations.** As `text` with `CHECK (... IN (...))`, not as a PostgreSQL `ENUM`.
   Reason: new values are an additive migration without a lock on the type.
4. **Amounts of money.** `bigint` in micro-units of the currency (suffix `_micros`) plus
   `currency text` (ISO 4217). Reason: exact, fast to sum, no rounding error.
5. **Sort keys.** Freely sortable lists (spaces, channels, tags) use
   `sort_key text COLLATE "C"` as a fractional index. Moving an item changes exactly one
   row.
6. **Upper and lower case.** Unique names (handles, slugs) are protected by a
   unique index on `lower(...)`. No `citext` extension is needed.
7. **Deletion.** Principals, channels, threads and messages are never hard-deleted. They
   get `deleted_at` or `archived_at`; content is cleared immediately on deletion
   (tombstone). Reason: sequence numbers, the audit chain and authorship stay consistent. Hard
   deletion exists only for dependent detail rows (`ON DELETE CASCADE`) and through the
   retention jobs from [20-privacy.md](20-privacy.md). The row of a deleted file (`files` with
   `deleted_at`) is removed by the job `files.purge_objects` once its objects are gone; its detail rows
   go by `ON DELETE CASCADE` (R-DM-46).
8. **JSON.** `jsonb` only for data with its own versioned schema (manifest, run metrics,
   component definition, event payload). Every such field names its schema in the
   column "Rules". Fields used for filtering or joining are never only in `jsonb`.
9. **Secrets.** No secrets in plain text. Tokens only as a hash
   (HMAC-SHA-256 with a server pepper from the secret provider). Third-party credentials
   (webhook secrets, OIDC client secrets) only as a reference `secret_ref` to the secret provider.
   Claude.ai credentials or subscription tokens do not exist in the model (B10, I5).
10. **Optimistic locking.** Mutable, synchronized objects have `version bigint`. Every
    change increments it by one. Writing requests carry the expected version as the HTTP
    precondition `If-Match`; if it does not match, the server answers `412 version_conflict`
    (RFC 9110, [06-api.md](06-api.md)).
11. **Isolation level.** Writing transactions run in `READ COMMITTED`. The
    counter patterns from section 6 assume this.
12. **Access.** Only the module that owns a table writes it. Other modules read through
    its port. Queries that filter by permissions get the set of visible channels from
    `authz` and never join `grants` themselves (one truth, B7). The module register records
    table ownership, a gate checks it ([ADR 0050](../adr/0050-module-register-as-the-source-for-architecture-gate-and-table-ownership.md)).
13. **No `null` to the outside.** A column with NULL appears in the API and in events as a
    missing field, never as JSON `null` ([04-sync-protocol.md](04-sync-protocol.md),
    section 2.2; [ADR 0053](../adr/0053-no-json-null-in-api-and-events.md)). Reason: generated clients (dart-dio) know no null type.

## 2. Modules

| Module | Responsibility | Tables |
|---|---|---|
| `instance` | Instance, settings, branding, feature flags, signing keys, counters, sync epoch | yes |
| `identity` | Principals, people, identities, passkeys, local credentials, enrollment codes, login flows, challenges (step-up, passkeys), device pairing, devices, sessions, tokens, owners | yes |
| `authz` | Groups, grants, permission version, `authorize` | yes |
| `chat` | Spaces, channels, threads, tags, project binding, sidebar state | yes |
| `messaging` | Messages, revisions, mentions, reactions, pins, templates | yes |
| `files` | Files, upload sessions, thumbnails, attachments, virus scan, transcripts, file text extracts | yes |
| `agents` | Agents, agent sessions, consumer cursors of the agents, slash commands, dev environments | yes |
| `runs` | Runs, steps, follow-ups, components, interactions | yes |
| `apps` | Apps, installations, incoming and outgoing webhooks | yes |
| `billing` | Cost entries, price table | yes |
| `sync` | Event log, cursors of the worker consumers, idempotency keys, read state, drafts, settings | yes |
| `notify` | Notification preferences, quiet hours, push registrations | yes |
| `audit` | Audit log, checkpoints, keys of the IP pseudonyms | yes |
| `privacy` | Privacy requests | yes |
| `importer` | Import sources and imported items | yes |
| `status` | Status page, backup runs | yes |
| `gateway` | WebSocket endpoint, gateway sessions, focus, typing, ephemeral events, shared presence state of several nodes | yes |
| `search` | Search coordinator over the read ports of `messaging`, `chat` and `files` ([14-search.md](14-search.md)) | none |
| `stats` | Statistics of the console from the read ports of other modules, own measurement as minute aggregates ([25-visualization.md](25-visualization.md)) | yes |
| `terminal` | Hosts, pairing, session users per person, terminal sessions, terminal session tokens, relay of the streams ([24-terminal.md](24-terminal.md)) | yes |

The two modules without tables are separate modules because they have their own ports,
dependencies and performance targets ([02-architecture.md](02-architecture.md), section 4).
Technical tables outside the modules are in section 4.13. The assignment of the tables to
modules is recorded in machine-readable form in the module register ([ADR 0050](../adr/0050-module-register-as-the-source-for-architecture-gate-and-table-ownership.md)).

## 3. Retention classes

Every table names exactly one class. [20-privacy.md](20-privacy.md) sets the duration,
deletion path and exceptions (for example legal retention) per class. This chapter only sets
the assignment.

| Class | Content |
|---|---|
| `config` | Configuration of the instance. Lives until it is changed. |
| `account` | Master data of principals and groups. Lives until deletion or pseudonymization. |
| `security` | Sessions, tokens, devices, keys. Deletable shortly after expiry or revocation. |
| `content` | User content: messages, files, threads, runs. Deletion by the user, an admin or a privacy request. |
| `personal_sync` | Personal sync data: read state, drafts, settings. Follows the account. |
| `event_log` | Event log. Window `sync.event_retention`. |
| `transient` | Short-lived technical data: idempotency keys, delivery attempts. |
| `audit` | Audit log. Its own period, separate from telemetry (B11). |
| `cost` | Costs and prices. Period for billing records. |
| `import` | Import state. Deletable after the import is complete. |
| `privacy_job` | Privacy requests and export packages. |
| `ops` | Status page and backup runs. |
| `stats` | Minute aggregates of the own measurement, without reference to people. Period `retention.stats_minutes`. |

## 4. Tables

Notation: "Null" = `yes` means NULL is allowed. An empty "Default" means no default value.
Foreign keys name their `ON DELETE` behavior. `RESTRICT` is the normal case, because
principals and content are not hard-deleted.

### 4.1 Instance (`instance`)

#### `instance`

Purpose: exactly one row with the identity of this installation. Module: `instance`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `smallint` | no | `1` | `CHECK (id = 1)`; enforces one row |
| `instance_id` | `uuid` | no | | UUIDv7, generated on the first `bothy migrate up` (the row is created there, through the port of `instance`); issuer (`iss`) in tokens |
| `setup_completed_at` | `timestamptz` | yes | | NULL until the first owner has been created by server command |
| `schema_version` | `bigint` | no | | last applied migration, for display only |
| `min_server_version` | `text` | yes | | SemVer; lowest server version that may run with this schema; set by `bothy migrate`. A server with a lower version reports not ready ([16-operations.md](16-operations.md), section 10) |
| `sync_epoch` | `uuid` | no | | UUIDv7 of the current timeline of the event log; generated on the first `bothy migrate up`, new after every restore (section 6.7) |
| `pg_system_identifier` | `text` | yes | | `system_identifier` of `pg_control_system()` of the database cluster, as decimal text (the value is an unsigned 64-bit number); set by `bothy migrate up` while empty and by `bothy restore finalize` (section 6.7, R-DM-50) |
| `pg_timeline_id` | `bigint` | yes | | `timeline_id` of `pg_control_checkpoint()`, read together with `pg_system_identifier` (R-DM-50) |
| `created_at` | `timestamptz` | no | `now()` | |

PK: `id`. FK: none. Unique: `instance_id`. Indexes: no others.
Retention: `config`. `bothy migrate up` creates the row if it is missing
(`INSERT ... ON CONFLICT (id) DO NOTHING`), and then sets `schema_version` and
`min_server_version`; it never changes `instance_id`, `sync_epoch` and `setup_completed_at`, and it sets `pg_system_identifier` and `pg_timeline_id` only while they are empty. Reason:
`migrate` is the first step of every installation, and the readiness check needs
`min_server_version` before there is an owner.

#### `instance_settings`

Purpose: typed settings of the instance, including branding, as key-value pairs.
Module: `instance`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `key` | `text` | no | | for example `branding.product_name`, `branding.accent_color`, `branding.logo_file_id`, `locale.default`; keys and schema in the configuration reference ([../operations/configuration.md](../operations/configuration.md), section 6). `branding.accent_color` is a key of the category colors of the design system, not a free color value |
| `value` | `jsonb` | no | | checked against the schema of the key |
| `version` | `bigint` | no | `1` | optimistic locking |
| `updated_by` | `uuid` | yes | | principal; NULL at setup |
| `updated_at` | `timestamptz` | no | `now()` | |

PK: `key`. FK: `updated_by` -> `principals(id)` ON DELETE RESTRICT. Unique: no others.
Indexes: no others (few rows, fully in the cache). Retention: `config`.

#### `server_signing_keys`

Purpose: public parts of the server keys for access tokens, invocation tokens (I3),
outgoing webhooks and audit checkpoints. The private part is only in the secret provider.
Module: `instance`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `kid` | `text` | no | | key ID in the JWS header |
| `purpose` | `text` | no | | `CHECK (purpose IN ('access_token','invocation','terminal','webhook_out','audit_checkpoint'))`; `terminal` signs terminal session tokens ([24-terminal.md](24-terminal.md), section 5.3) |
| `alg` | `text` | no | | `CHECK (alg IN ('EdDSA','ES256'))`; with `purpose IN ('invocation','terminal','webhook_out')` only `EdDSA` (check), because invocation tokens and terminal session tokens allow exactly one algorithm and Standard Webhooks `v1a` only knows Ed25519 |
| `public_jwk` | `jsonb` | no | | JWK per RFC 7517, without the private part |
| `private_key_ref` | `text` | no | | reference to the secret provider |
| `state` | `text` | no | `'active'` | `CHECK (state IN ('next','active','retiring','retired'))` |
| `created_at` | `timestamptz` | no | `now()` | |
| `not_after` | `timestamptz` | yes | | after this point in time nothing more is signed |

PK: `kid`. FK: none. Unique: partial index `(purpose) WHERE state = 'active'` (exactly one active
key per purpose). Indexes: `(purpose, state)` for the JWKS output.
Retention: `security`.

#### `feature_flags`

Purpose: switches for features, global or for groups and people. Module: `instance`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `key` | `text` | no | | for example `dm.enabled`, `voice.enabled` |
| `enabled` | `boolean` | no | `false` | global value |
| `rollout` | `jsonb` | no | `'{}'` | schema `feature_flag_rollout.v1`: lists of `user_ids` and `group_ids` for which the switch applies differently |
| `version` | `bigint` | no | `1` | |
| `updated_by` | `uuid` | yes | | |
| `updated_at` | `timestamptz` | no | `now()` | |

PK: `key`. FK: `updated_by` -> `principals(id)` ON DELETE RESTRICT. Indexes: no others.
Retention: `config`. Kind, description, default, module and `remove_by` are in the
registration in the code, not in the table ([ADR 0064](../adr/0064-feature-flag-registration-with-a-cleanup-gate.md)); a registered key without a row has
its default and, in the API, the version `0`; a key without a registration is off and
orphaned. The same applies to `instance_settings`: a registered key without a row has its
default value and the version `0`.

#### `sequence_counters`

Purpose: counter rows whose row lock enforces the commit order. Module: `instance`
(read and written only through the functions from section 6).

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `name` | `text` | no | | `CHECK (name IN ('event_seq','audit_seq','permission_version'))` |
| `value` | `bigint` | no | `0` | only increasing |

PK: `name`. FK: none. Indexes: no others. Retention: `config`.

The three rows are created by the migration that creates the table
(`00002_instance_counters.sql`). `permission_version` is the
permission version counter (see 4.3 and [04-sync-protocol.md](04-sync-protocol.md)).

### 4.2 Identity (`identity`)

#### `principals`

Purpose: common identity of all acting principals (person, agent, app). Foreign keys
for "who did it" always point here. Module: `identity`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | UUIDv7 |
| `kind` | `text` | no | | `CHECK (kind IN ('user','agent','app'))`; never changeable |
| `status` | `text` | no | `'active'` | `CHECK (status IN ('pending','active','disabled','deleted'))` |
| `handle` | `text` | no | | for mentions, `CHECK (handle ~ '^[a-z0-9][a-z0-9._-]*$')`, length up to `principal.handle_max_chars` |
| `display_name` | `text` | no | | not empty and not only whitespace, at most `principal.display_name_max_chars` characters, without control and bidi characters ([19-security.md](19-security.md), SEC-IN-4); pseudonymized after deletion |
| `avatar_file_id` | `uuid` | yes | | |
| `created_at` | `timestamptz` | no | `now()` | |
| `disabled_at` | `timestamptz` | yes | | |
| `deleted_at` | `timestamptz` | yes | | pseudonymization by a privacy request |

PK: `id`. FK: `avatar_file_id` -> `files(id)` ON DELETE SET NULL. Unique: `lower(handle)`.
Indexes: `(kind, status)` for directory lists. Retention: `account`.

#### `users`

Purpose: additional data for people. Module: `identity`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `principal_id` | `uuid` | no | | `kind = 'user'` checked by trigger |
| `email` | `text` | yes | | for display only; never for matching, permissions or notifications; never set from a provider's data, because the value is unique and a provider could otherwise block logins ([08-identity.md](08-identity.md), section 3.1) |
| `locale` | `text` | no | `'de'` | BCP 47 |
| `time_zone` | `text` | no | `'UTC'` | IANA name, for quiet hours |
| `last_active_at` | `timestamptz` | yes | | coarse, written at most once per `identity.last_active_granularity` |

PK: `principal_id`. FK: `principal_id` -> `principals(id)` ON DELETE RESTRICT.
Unique: `lower(email)` as a partial index `WHERE email IS NOT NULL`. Indexes: no others.
Retention: `account`.

#### `owners`

Purpose: list of owners. At least one. Module: `identity`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | |
| `user_id` | `uuid` | no | | people only |
| `granted_by` | `uuid` | yes | | NULL only for the first owner (server command) |
| `granted_at` | `timestamptz` | no | `now()` | |
| `revoked_at` | `timestamptz` | yes | | |
| `revoked_by` | `uuid` | yes | | |
| `reason` | `text` | no | | required, recorded in the audit |

PK: `id`. FK: `user_id` -> `users(principal_id)` ON DELETE RESTRICT; `granted_by`,
`revoked_by` -> `principals(id)` ON DELETE RESTRICT. Unique: partial index `(user_id) WHERE
revoked_at IS NULL`. Indexes: via the partial index. Retention: `account`.
Invariant (SR8 in [07-permissions.md](07-permissions.md)): a transaction that revokes an owner
or disables or deletes the principal of an owner (`owner.remove`, `user.disable`, GDPR deletion)
checks before the commit that at least one owner with `principals.status = 'active'` remains (code
`last_owner`), and checks I1 (section 6.4).

#### `identity_providers`

Purpose: configured login drivers (B9). Module: `identity`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | |
| `kind` | `text` | no | | `CHECK (kind IN ('oidc','trusted_header','local'))` |
| `name` | `text` | no | | display, for example "Company SSO" |
| `config` | `jsonb` | no | | schema `identity_provider.<kind>.v1` ([08-identity.md](08-identity.md), section 2.6); secrets only as references (`client_secret_ref`, `client_key_ref`) |
| `enabled` | `boolean` | no | `true` | |
| `allow_provisioning` | `boolean` | no | `false` | `false`: unknown subjects get no access (no self-registration) |
| `created_at` | `timestamptz` | no | `now()` | |
| `updated_at` | `timestamptz` | no | `now()` | |

PK: `id`. FK: none. Unique: `lower(name)`; partial index `(kind) WHERE kind IN ('trusted_header',
'local')` (one each). Indexes: no others. Retention: `config`.

#### `user_identities`

Purpose: link of a person to a subject at a provider. Module: `identity`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | |
| `user_id` | `uuid` | no | | |
| `provider_id` | `uuid` | no | | |
| `subject` | `text` | no | | OIDC `sub` or header login or local login name |
| `issuer` | `text` | yes | | OIDC `iss`, otherwise NULL |
| `email_at_link` | `text` | yes | | snapshot, display only |
| `linked_at` | `timestamptz` | no | `now()` | |
| `last_login_at` | `timestamptz` | yes | | |
| `unlinked_at` | `timestamptz` | yes | | |

PK: `id`. FK: `user_id` -> `users(principal_id)` ON DELETE RESTRICT; `provider_id` ->
`identity_providers(id)` ON DELETE RESTRICT. Unique: partial index `(provider_id, subject) WHERE
unlinked_at IS NULL`. Indexes: `(user_id)` for the account view. Retention: `account`.

#### `passkey_credentials`

Purpose: WebAuthn passkeys for the login of local accounts and for step-up. Module: `identity`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | |
| `user_id` | `uuid` | no | | |
| `credential_id` | `bytea` | no | | WebAuthn credential ID |
| `public_key_cose` | `bytea` | no | | public key (COSE) |
| `sign_count` | `bigint` | no | `0` | a regression is logged and reported in the audit |
| `aaguid` | `uuid` | yes | | |
| `transports` | `text[]` | no | `'{}'` | |
| `backup_eligible` | `boolean` | no | `false` | |
| `backup_state` | `boolean` | no | `false` | |
| `name` | `text` | no | | assigned by the person |
| `created_at` | `timestamptz` | no | `now()` | |
| `last_used_at` | `timestamptz` | yes | | |
| `revoked_at` | `timestamptz` | yes | | |

PK: `id`. FK: `user_id` -> `users(principal_id)` ON DELETE RESTRICT. Unique: `credential_id`.
Indexes: `(user_id) WHERE revoked_at IS NULL` for the login challenge. Retention: `security`.

#### `devices`

Purpose: an installed client of a person. New devices wait for confirmation.
Module: `identity`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | generated by the client at registration (UUIDv7) |
| `user_id` | `uuid` | no | | |
| `name` | `text` | no | | for example "Alex's iPhone" |
| `platform` | `text` | no | | `CHECK (platform IN ('ios','android','macos','windows','web','cli'))` |
| `state` | `text` | no | `'pending'` | `CHECK (state IN ('pending','active','revoked'))` |
| `app_version` | `text` | yes | | |
| `approved_by` | `uuid` | yes | | |
| `approved_at` | `timestamptz` | yes | | |
| `revoked_by` | `uuid` | yes | | |
| `revoked_at` | `timestamptz` | yes | | |
| `revoke_reason` | `text` | yes | | `CHECK (revoke_reason IN ('revoked_by_self','revoked_by_owner','user_disabled','inactive','verification_failed','pending_expired','pairing_expired'))`; a code, never free text; the reason of a request is only in the audit |
| `created_at` | `timestamptz` | no | `now()` | |
| `last_seen_at` | `timestamptz` | yes | | |

PK: `id`. FK: `user_id` -> `users(principal_id)` ON DELETE RESTRICT; `approved_by`,
`revoked_by` -> `principals(id)` ON DELETE RESTRICT. Unique: no others.
Indexes: `(user_id, state)` for the device list; `(state) WHERE state = 'pending'` for the
approval queue of the admin console. Retention: `security`. Checks: `(state = 'revoked') =
(revoked_at IS NOT NULL)`; `revoke_reason IS NULL OR state = 'revoked'`.

#### `device_keys`

Purpose: public device keys from the secure storage. A native device has two: the
DPoP key (binds tokens, signs without user confirmation) and the step-up key
(signs only after biometrics or the device PIN) ([08-identity.md](08-identity.md),
section 6.1). Module: `identity`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | |
| `device_id` | `uuid` | no | | |
| `purpose` | `text` | no | | `CHECK (purpose IN ('dpop','step_up'))` |
| `jkt` | `text` | no | | JWK thumbprint (RFC 7638); for the purpose `dpop` it is in the token as `cnf.jkt` |
| `public_jwk` | `jsonb` | no | | |
| `alg` | `text` | no | | `CHECK (alg IN ('ES256','EdDSA'))`; whether Windows Hello delivers `ES256` is open ([08-identity.md](08-identity.md), section 6.1); an extension by `PS256` is an additive migration |
| `attestation` | `jsonb` | yes | | platform attestation, if present |
| `created_at` | `timestamptz` | no | `now()` | |
| `revoked_at` | `timestamptz` | yes | | device revocation; there is no key change on an existing device ([08-identity.md](08-identity.md), section 5.4) |

PK: `id`. FK: `device_id` -> `devices(id)` ON DELETE CASCADE. Unique: `jkt`; partial index
`(device_id, purpose) WHERE revoked_at IS NULL` (one active key per device and purpose).
Indexes: via the unique indexes. Retention: `security`.
The web console and tools without secure storage have no step-up key; their step-up
runs via passkey, a new OIDC login or password with TOTP.

#### `auth_sessions`

Purpose: login session of a person on a device. Every access token carries its ID as
`sid`; revocation therefore takes effect immediately. Module: `identity`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | |
| `user_id` | `uuid` | no | | |
| `device_id` | `uuid` | no | | |
| `client_component` | `text` | no | `'app'` | `CHECK (client_component IN ('app','share_ext','console'))`; the share extension has its own child session with its own refresh family; the Notification Service Extension has no session, it uses the preview token ([13-push-notifications.md](13-push-notifications.md), section 6.4) |
| `parent_session_id` | `uuid` | yes | | set for child sessions of extensions; revocation of the parent session revokes all children with the same reason |
| `identity_id` | `uuid` | yes | | what was used to log in; NULL for a local login without a provider identity |
| `auth_method` | `text` | no | | `CHECK (auth_method IN ('oidc','trusted_header','passkey','password_totp','enrollment_code','device_pairing'))` |
| `cookie_hash` | `bytea` | yes | | console only (`client_component = 'console'`): HMAC-SHA-256 of the session cookie; new value at login and after step-up |
| `created_at` | `timestamptz` | no | `now()` | |
| `expires_at` | `timestamptz` | no | | absolute end, `created_at + auth.session_max_age` (console: `auth.console_session_max_age`) |
| `step_up_at` | `timestamptz` | yes | | last fresh confirmation by the person; valid for `auth.step_up_max_age` |
| `step_up_method` | `text` | yes | | `CHECK (step_up_method IN ('passkey','device_key','password_totp','oidc_reauth'))`; set together with `step_up_at` |
| `step_up_consumed_at` | `timestamptz` | yes | | set when `terminal.open` or `terminal.attach` has consumed the step-up with `step_up_at`; a consumed step-up (`step_up_consumed_at >= step_up_at`) allows no further terminal opening ([08-identity.md](08-identity.md), section 6.3) |
| `view_as_user_id` | `uuid` | yes | | console only: the person whose view an owner looks at with "View as person" ([07-permissions.md](07-permissions.md), section 9.2); set by `access.view_as`, cleared by `access.view_as.end` or by the first request after `authz.view_as_max_duration` (R-DM-49) |
| `view_as_started_at` | `timestamptz` | yes | | start of the stored view; set and cleared together with `view_as_user_id` |
| `last_seen_at` | `timestamptz` | yes | | |
| `last_ip` | `inet` | yes | | only for the security view, class `security` |
| `revoked_at` | `timestamptz` | yes | | |
| `revoke_reason` | `text` | yes | | `CHECK (revoke_reason IN ('logout','admin','device_revoked','refresh_reuse','user_disabled','expired','inactive','step_up_failed','trusted_header_mismatch'))`; `logout` when the person ends their own session, `admin` when someone else does it |

PK: `id`. FK: `user_id` -> `users(principal_id)` ON DELETE RESTRICT; `device_id` ->
`devices(id)` ON DELETE RESTRICT; `identity_id` -> `user_identities(id)` ON DELETE SET NULL;
`parent_session_id` -> `auth_sessions(id)` ON DELETE CASCADE; `view_as_user_id` -> `users(principal_id)` ON DELETE RESTRICT.
Checks: `(client_component = 'console') = (cookie_hash IS NOT NULL)`; `parent_session_id IS NULL OR
client_component = 'share_ext'`; `(step_up_at IS NULL) = (step_up_method IS
NULL)`; `step_up_consumed_at IS NULL OR step_up_at IS NOT NULL`; `(view_as_user_id IS NULL) =
(view_as_started_at IS NULL)`; `view_as_user_id IS NULL OR client_component = 'console'`. Unique: `cookie_hash` as a partial index `WHERE cookie_hash IS NOT NULL`.
Indexes: `(user_id) WHERE revoked_at IS NULL` for "end all sessions"; `(device_id)`;
`(parent_session_id) WHERE parent_session_id IS NOT NULL`; `(expires_at)` for the cleanup job.
Retention: `security`.

#### `refresh_token_families`

Purpose: family of rotating refresh tokens of a session; reuse revokes the family.
Module: `identity`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | |
| `session_id` | `uuid` | no | | |
| `dpop_jkt` | `text` | no | | thumbprint of the DPoP key; every renewal requires a DPoP proof with exactly this key (RFC 9449, section 5) |
| `current_generation` | `integer` | no | `0` | number of the only valid token |
| `rotated_at` | `timestamptz` | yes | | time of the last rotation; basis of the grace period `auth.refresh_reuse_grace` |
| `created_at` | `timestamptz` | no | `now()` | |
| `revoked_at` | `timestamptz` | yes | | |
| `revoke_reason` | `text` | yes | | `CHECK (revoke_reason IN ('reuse_detected','logout','session_revoked','expired'))` |

PK: `id`. FK: `session_id` -> `auth_sessions(id)` ON DELETE CASCADE. Unique: partial index
`(session_id) WHERE revoked_at IS NULL`. Indexes: via the unique index.
Retention: `security`. Console sessions have no family (cookie instead of refresh token).

#### `refresh_tokens`

Purpose: individual refresh tokens, only as a hash. Module: `identity`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | |
| `family_id` | `uuid` | no | | |
| `generation` | `integer` | no | | |
| `token_hash` | `bytea` | no | | HMAC-SHA-256 |
| `issued_at` | `timestamptz` | no | `now()` | |
| `expires_at` | `timestamptz` | no | | `issued_at + auth.refresh_token_ttl` |
| `used_at` | `timestamptz` | yes | | set on rotation |

PK: `id`. FK: `family_id` -> `refresh_token_families(id)` ON DELETE CASCADE.
Unique: `token_hash`; `(family_id, generation)`. Indexes: via the unique indexes.
Retention: `security`.
Rotation: in one transaction `SELECT ... FOR UPDATE` on the family, with a DPoP proof with
`dpop_jkt` as a precondition.

1. If `generation = current_generation` and `used_at IS NULL`: set `used_at`, create a new token
   with `generation + 1`, increment `current_generation`, `rotated_at = now()`.
2. Grace period: if the token is the immediately preceding generation
   (`generation = current_generation - 1`) and `rotated_at` is less than
   `auth.refresh_reuse_grace` in the past, the server rotates once more as in item 1 (the never
   received generation expires). Reason: a crash between the server rotation and local
   storage (app or extension) must not log the person out; the DPoP proof shows that the same
   device key is asking.
3. Otherwise it is reuse: family and session are revoked (`refresh_reuse`), an
   audit entry is created, the request fails.

#### `api_tokens`

Purpose: long-lived tokens for agents, apps and (via feature flag) personal tools.
Only hash and prefix. Format `bothy_<kind>_<random>_<checksum>` with kind `agt`, `app` or
`pat` ([08-identity.md](08-identity.md), section 12; [ADR 0061](../adr/0061-token-format-with-a-fixed-prefix-and-checksum.md); incoming webhooks use the same
format with kind `whi` in `incoming_webhooks.token_hash`); the fixed prefix allows the rule
`bothy-api-token` in the secret scanner (`.gitleaks.toml`). Module: `identity`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | |
| `principal_id` | `uuid` | no | | holder of the token |
| `name` | `text` | no | | |
| `token_prefix` | `text` | no | | `bothy_<kind>_` and the first 6 characters of the random part, for recognition in the console ([ADR 0061](../adr/0061-token-format-with-a-fixed-prefix-and-checksum.md)) |
| `token_hash` | `bytea` | no | | HMAC-SHA-256 |
| `scopes` | `text[]` | no | `'{}'` | subset of the manifest scopes or agent scopes |
| `dpop_jkt` | `text` | yes | | set: requests must be DPoP-bound to this key |
| `expires_at` | `timestamptz` | no | | required, at most `auth.api_token_max_ttl` or `auth.personal_token_max_ttl` after creation ([08-identity.md](08-identity.md), section 12) |
| `created_by` | `uuid` | no | | |
| `created_at` | `timestamptz` | no | `now()` | |
| `last_used_at` | `timestamptz` | yes | | coarse, at most once per `identity.last_active_granularity` |
| `revoked_at` | `timestamptz` | yes | | |
| `revoked_by` | `uuid` | yes | | |

PK: `id`. FK: `principal_id`, `created_by`, `revoked_by` -> `principals(id)` ON DELETE RESTRICT.
Unique: `token_hash`. Indexes: `(principal_id) WHERE revoked_at IS NULL` for the token list.
Retention: `security`.

#### `local_credentials`

Purpose: password with TOTP as a fallback for local accounts that can be switched off (baseline R-Passkeys,
`auth.local_password_enabled`). One row per local identity. Module: `identity`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `identity_id` | `uuid` | no | | identity at the driver `local` |
| `password_hash` | `text` | no | | Argon2id in PHC format; parameters from `auth.argon2id_*` ([08-identity.md](08-identity.md)) |
| `password_changed_at` | `timestamptz` | no | `now()` | |
| `totp_secret_ciphertext` | `bytea` | yes | | TOTP secret, AEAD-encrypted; NULL until TOTP is set up |
| `totp_key_ref` | `text` | yes | | reference to the key in the secret provider; set exactly together with `totp_secret_ciphertext` |
| `totp_last_step` | `bigint` | yes | | last accepted time step; every digit sequence is valid only once (RFC 6238, section 5.2) |
| `totp_enabled_at` | `timestamptz` | yes | | |
| `failed_count` | `integer` | no | `0` | failed attempts in the current window `auth.login_fail_window` |
| `fail_window_started_at` | `timestamptz` | yes | | start of the current window; NULL without a failed attempt. A failed attempt after `fail_window_started_at + auth.login_fail_window` starts a new window |
| `locked_until` | `timestamptz` | yes | | lockout of the password path only |

PK: `identity_id`. FK: `identity_id` -> `user_identities(id)` ON DELETE CASCADE.
Checks: `(totp_secret_ciphertext IS NULL) = (totp_key_ref IS NULL)`; `(failed_count = 0) = (fail_window_started_at IS NULL)`. Indexes: via the PK.
Retention: `security`. The TOTP secret is one of the two exceptions to principle 9 (the
other: `audit_ip_keys`): the server needs it for checking; therefore it is stored encrypted in the
database and the key only in the secret provider.

#### `enrollment_codes`

Purpose: one-time enrollment codes for the first device, the linking of an identity and
recovery ([08-identity.md](08-identity.md), section 3.2). Module: `identity`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | |
| `user_id` | `uuid` | no | | person for whom the code applies |
| `purpose` | `text` | no | | `CHECK (purpose IN ('first_login','identity_link','recovery'))` |
| `code_hash` | `bytea` | no | | HMAC-SHA-256 of the code; the code appears exactly once |
| `expires_at` | `timestamptz` | no | | `created_at + auth.enrollment_code_ttl` |
| `attempts` | `integer` | no | `0` | failed attempts with this code (a rejected redemption whose code matched); invalid from `auth.enrollment_code_max_attempts` on |
| `created_by` | `uuid` | yes | | NULL when issued by server command |
| `created_at` | `timestamptz` | no | `now()` | |
| `used_at` | `timestamptz` | yes | | |
| `used_by_device_id` | `uuid` | yes | | device that redeemed the code; it thereby becomes `active` |

PK: `id`. FK: `user_id` -> `users(principal_id)` ON DELETE CASCADE; `created_by` ->
`principals(id)` ON DELETE RESTRICT; `used_by_device_id` -> `devices(id)` ON DELETE SET NULL.
Unique: `code_hash`. Indexes: `(user_id) WHERE used_at IS NULL`; `(expires_at)` for the
cleanup job. Retention: `security`. At most one code applies per person: issuing a code sets
`expires_at = now()` on every other unused code of the person ([08-identity.md](08-identity.md),
section 3.2).

#### `auth_login_flows`

Purpose: state of an OIDC login or link flow between the start, the redirect back from the
provider and the redemption of the server code by the app ([08-identity.md](08-identity.md),
section 2.2). In the API the ID is called `login_attempt_id`. Module: `identity`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | |
| `provider_id` | `uuid` | no | | |
| `mode` | `text` | no | | `CHECK (mode IN ('login','link'))` |
| `user_id` | `uuid` | yes | | set for `link` (who is linking) |
| `device_id` | `uuid` | yes | | device ID, if the app already names it at the start; the device is created only at redemption |
| `oidc_state_hash` | `bytea` | no | | HMAC of the `state` sent to the provider |
| `oidc_nonce_hash` | `bytea` | no | | HMAC of the `nonce` in the ID token |
| `client_challenge` | `text` | no | | `S256` of the app's `client_verifier`; binds the server code to this app instance |
| `app_redirect_uri` | `text` | no | | from the fixed list (loopback or private scheme) |
| `login_code_hash` | `bytea` | yes | | HMAC of the one-time server code (`login_code`) after the redirect back |
| `login_code_expires_at` | `timestamptz` | yes | | `auth.server_code_ttl` after the redirect back |
| `enrollment_code_id` | `uuid` | yes | | enrollment code presented and checked at the start; it is redeemed only at completion |
| `external_identity` | `jsonb` | yes | | schema `external_identity.v1` ([08-identity.md](08-identity.md), section 2.2): the provider identity checked at the callback; set together with `status = 'code_issued'`, NULL again at completion or failure |
| `status` | `text` | no | `'started'` | `CHECK (status IN ('started','code_issued','completed','failed'))` |
| `error` | `text` | yes | | code without content |
| `created_at` | `timestamptz` | no | `now()` | |
| `expires_at` | `timestamptz` | no | | `created_at + auth.login_flow_ttl` |

PK: `id`. FK: `provider_id` -> `identity_providers(id)` ON DELETE CASCADE; `user_id` ->
`users(principal_id)` ON DELETE CASCADE; `enrollment_code_id` -> `enrollment_codes(id)` ON DELETE
SET NULL. Checks: `(status = 'code_issued') = (external_identity IS NOT NULL)`; `mode = 'link'`
has no `enrollment_code_id`. Unique: `oidc_state_hash`; `login_code_hash` as a partial index
`WHERE login_code_hash IS NOT NULL`. Indexes: `(expires_at)` for the cleanup job.
Retention: `transient`. The server's PKCE verifier towards the provider is not
stored, but derived from `id` and the token pepper (HMAC-SHA-256); this way no
further secret is in the table. The callback changes no account: matching, linking and
provisioning happen only at completion, which only the app with the `client_verifier` can
trigger (R-DM-30).

#### `auth_challenges`

Purpose: one-time challenges for step-up (device key and passkey) and for the
WebAuthn ceremonies login, first setup with an enrollment code and registration
([08-identity.md](08-identity.md), sections 2.5 and 6.2). In the API the ID is called
`challenge_id`. Module: `identity`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | UUIDv7 |
| `purpose` | `text` | no | | `CHECK (purpose IN ('step_up','passkey_login','passkey_enroll','passkey_register'))` |
| `session_id` | `uuid` | yes | | session for which it applies; set exactly for `step_up` and `passkey_register` |
| `enrollment_code_id` | `uuid` | yes | | checked enrollment code; set exactly for `passkey_enroll` |
| `challenge_hash` | `bytea` | no | | HMAC-SHA-256 of the random value with the token pepper; the value appears only in the response |
| `webauthn_session` | `jsonb` | yes | | schema `webauthn_session.v1`: RP ID, required user verification, allowed credential IDs, user handle; without the challenge and without secrets; NULL for step-up with a device key |
| `status` | `text` | no | `'issued'` | `CHECK (status IN ('issued','used','failed'))` |
| `created_at` | `timestamptz` | no | `now()` | |
| `expires_at` | `timestamptz` | no | | `created_at + auth.step_up_challenge_ttl` for `step_up`, otherwise `created_at + auth.login_flow_ttl` |
| `finished_at` | `timestamptz` | yes | | set together with `used` or `failed` |

PK: `id`. FK: `session_id` -> `auth_sessions(id)` ON DELETE CASCADE; `enrollment_code_id` ->
`enrollment_codes(id)` ON DELETE CASCADE. Checks: `(status = 'issued') = (finished_at IS NULL)`;
`(session_id IS NOT NULL) = (purpose IN ('step_up','passkey_register'))`;
`(enrollment_code_id IS NOT NULL) = (purpose = 'passkey_enroll')`. Unique: `challenge_hash`.
Indexes: `(session_id, finished_at) WHERE status = 'failed'` for the failed attempts at step-up;
`(expires_at)` for the cleanup job. Retention: `transient`.
Redemption: in one transaction `SELECT ... FOR UPDATE` on the row; only `issued` and not
expired is valid, afterwards `used` or, on a wrong confirmation, `failed`. Reason for a table instead of
derived values: one-time use and counted failed attempts thus apply on every node without
depending on synchronized clocks (R-DM-31).

#### `device_pairings`

Purpose: device pairing ([08-identity.md](08-identity.md), section 5.3). The new device
registers without a login with its public keys and shows a pairing code;
an active device of the same person confirms with this code and step-up. Module: `identity`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | `pairing_id` in the API |
| `pairing_code_hash` | `bytea` | no | | HMAC-SHA-256 of the pairing code; the code appears only on the new device |
| `device_registration` | `jsonb` | no | | schema `device_registration.v1`: name, platform, app version, public keys (`dpop`, optionally `step_up`); no secrets |
| `status` | `text` | no | `'requested'` | `CHECK (status IN ('requested','approved','completed','rejected','expired'))` |
| `user_id` | `uuid` | yes | | person whose active device confirmed |
| `approved_by_device_id` | `uuid` | yes | | confirming device |
| `device_id` | `uuid` | yes | | device created on completion (`active`) |
| `created_at` | `timestamptz` | no | `now()` | |
| `expires_at` | `timestamptz` | no | | `created_at + auth.pairing_code_ttl` |
| `completed_at` | `timestamptz` | yes | | |

PK: `id`. FK: `user_id` -> `users(principal_id)` ON DELETE CASCADE; `approved_by_device_id`,
`device_id` -> `devices(id)` ON DELETE SET NULL. Checks: `status NOT IN ('approved','completed')
OR user_id IS NOT NULL`; `status <> 'completed' OR device_id IS NOT NULL`. Unique:
`pairing_code_hash` as a partial index `WHERE status = 'requested'`. Indexes: `(expires_at)` for the
cleanup job. Retention: `transient`. Completion requires a DPoP proof with the
registered `dpop` key; so only the new device can fetch the tokens.

### 4.3 Permissions (`authz`)

#### `groups`

Purpose: named set of people as the subject of grants. Module: `authz`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | |
| `name` | `text` | no | | for example "Support" |
| `description` | `text` | no | `''` | |
| `source` | `text` | no | `'local'` | `CHECK (source IN ('local','oidc_claim'))` |
| `provider_id` | `uuid` | yes | | OIDC provider whose group claim determines the members; set exactly for `oidc_claim` |
| `external_ref` | `text` | yes | | claim value for `oidc_claim` |
| `created_by` | `uuid` | no | | |
| `created_at` | `timestamptz` | no | `now()` | |
| `deleted_at` | `timestamptz` | yes | | |

PK: `id`. FK: `created_by` -> `principals(id)` ON DELETE RESTRICT; `provider_id` ->
`identity_providers(id)` ON DELETE RESTRICT. Checks: `(source = 'oidc_claim') = (provider_id IS
NOT NULL)`; `(source = 'oidc_claim') = (external_ref IS NOT NULL)`. Unique: partial index `lower(name)
WHERE deleted_at IS NULL`; partial index `(provider_id, external_ref) WHERE source = 'oidc_claim' AND
deleted_at IS NULL`. Indexes: covered by the unique indexes. Retention: `account`. `name` at most
`group.name_max_chars`, `description` at most `group.description_max_chars` characters, both without
control and bidi characters. Reason for `provider_id`: without the binding, a second provider could
assert claim values that are meant for the first one.

#### `group_memberships`

Purpose: membership of a person in a group, with history. Module: `authz`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | |
| `group_id` | `uuid` | no | | |
| `user_id` | `uuid` | no | | people only (decision R-DM-07) |
| `added_by` | `uuid` | no | | |
| `added_at` | `timestamptz` | no | `now()` | |
| `expires_at` | `timestamptz` | yes | | checked on every `authorize` |
| `expiry_reminded_at` | `timestamptz` | yes | | expiry reminder written (`access.expiring`) |
| `expiry_reported_at` | `timestamptz` | yes | | expiry reported (`access.changed`, system action `group.member.expire`); exactly once per membership |
| `removed_at` | `timestamptz` | yes | | |
| `removed_by` | `uuid` | yes | | |

PK: `id`. FK: `group_id` -> `groups(id)` ON DELETE RESTRICT; `user_id` ->
`users(principal_id)` ON DELETE RESTRICT; `added_by`, `removed_by` -> `principals(id)` ON DELETE
RESTRICT. Unique: partial index `(group_id, user_id) WHERE removed_at IS NULL`.
Indexes: `(user_id) WHERE removed_at IS NULL` for "groups of a person" when building permissions;
`(expires_at) WHERE removed_at IS NULL AND expires_at IS NOT NULL` for the reminder and the
expiry report.
Retention: `account`.
Only the sync at login changes memberships in groups with `source = 'oidc_claim'`
(system action `group.member.sync`, [08-identity.md](08-identity.md), section 3.4); there
`added_by` and `removed_by` are the person whose login carried the claim.

#### `grants`

Purpose: one grant: subject x target x level x capabilities x expiry x reason. Revocation sets
`revoked_at`, the row stays. Module: `authz`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | |
| `subject_kind` | `text` | no | | `CHECK (subject_kind IN ('user','group','agent','app'))` |
| `subject_principal_id` | `uuid` | yes | | set for `user`, `agent`, `app` |
| `subject_group_id` | `uuid` | yes | | set for `group` |
| `target_kind` | `text` | no | | `CHECK (target_kind IN ('instance','space','channel','host'))` |
| `target_space_id` | `uuid` | yes | | set exactly for `space` |
| `target_channel_id` | `uuid` | yes | | set exactly for `channel` |
| `target_host_id` | `uuid` | yes | | set exactly for `host` ([24-terminal.md](24-terminal.md), section 8.1) |
| `level` | `text` | yes | | `CHECK (level IN ('read','write','trigger'))`; NULL exactly when `target_kind = 'host'`, because levels do not apply on hosts |
| `capabilities` | `text[]` | no | `'{}'` | subset of `approve_actions`, `upload`, `export`, `manage`, `terminal`; `terminal` only and exactly on the target `host` |
| `expires_at` | `timestamptz` | yes | | checked on every `authorize`; the job `authz.expiry_sweep` only reports the expiry (section 6.4) |
| `expiry_reminded_at` | `timestamptz` | yes | | expiry reminder written (`access.expiring`) |
| `expiry_reported_at` | `timestamptz` | yes | | expiry reported (`access.changed`, system action `grant.expire`); exactly once per grant |
| `reason` | `text` | no | | required |
| `granted_by` | `uuid` | no | | |
| `granted_at` | `timestamptz` | no | `now()` | |
| `revoked_at` | `timestamptz` | yes | | |
| `revoked_by` | `uuid` | yes | | |
| `revoke_reason` | `text` | yes | | |

PK: `id`.
FK: `subject_principal_id` -> `principals(id)` ON DELETE RESTRICT; `subject_group_id` ->
`groups(id)` ON DELETE RESTRICT; `target_space_id` -> `spaces(id)` ON DELETE RESTRICT;
`target_channel_id` -> `channels(id)` ON DELETE RESTRICT; `target_host_id` -> `hosts(id)` ON DELETE
RESTRICT; `granted_by`, `revoked_by` -> `principals(id)` ON DELETE RESTRICT.
Checks: exactly one of `subject_principal_id` and `subject_group_id` is set, matching
`subject_kind`; target columns matching `target_kind`; `capabilities <@ ARRAY['approve_actions',
'upload','export','manage','terminal']`; for `subject_kind IN ('agent','app')`, `level <> 'trigger' AND
capabilities <@ ARRAY['upload']` holds (R-DM-08, R-DM-20); `(target_kind = 'host') = (level IS NULL)`;
`target_kind <> 'host' OR (capabilities = ARRAY['terminal'] AND subject_kind IN ('user','group'))`;
`target_kind = 'host' OR NOT ('terminal' = ANY(capabilities))` (R-DM-35).
Unique: none (several grants on the same pair are allowed; the highest valid one applies).
Indexes: `(subject_principal_id) WHERE revoked_at IS NULL` and `(subject_group_id) WHERE
revoked_at IS NULL` for building the permissions of a principal; `(target_space_id) WHERE revoked_at
IS NULL` and `(target_channel_id) WHERE revoked_at IS NULL` for "Who has access?" and the
I1 check; `(target_host_id) WHERE revoked_at IS NULL` for the I6 check and SR10; `(expires_at) WHERE revoked_at IS NULL AND expires_at IS NOT NULL` for the reminder and the
expiry report. Retention: `account`.

Grants are never changed, only revoked and created anew (R-DM-09). The only allowed updates are
the revocation columns and the expiry bookkeeping (`expiry_reminded_at`,
`expiry_reported_at`). The job `authz.expiry_sweep` reports every expired row not yet reported
(`expires_at <= now()` and `expiry_reported_at IS NULL`) and sets the column in the same
transaction; so after an outage of the worker, every expiry is reported exactly once. Every transaction
that changes `grants`, `group_memberships`, `owners`, operator assignments, `agents.billing_mode`,
`agents.credential_holder_id`, `agents.tool_class`, the state of a dev environment, a host's
`state`, `assigned_user_id`, `dev_environment_id`, `llm_credentials`, `credential_holder_id` or
`declared_server_host` or `reported_session_users`, the rows in `host_session_users` or the status of a principal increments `sequence_counters.permission_version` (section 6.3). This
also applies to the job `authz.expiry_sweep`, which reports expired grants and memberships.

### 4.4 Agents and apps (`agents`, `apps`)

#### `agents`

Purpose: additional data for agent principals: operations, backend policy and reported state.
Module: `agents`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `principal_id` | `uuid` | no | | `kind = 'agent'` checked by a trigger |
| `adapter_kind` | `text` | no | | free-form identifier of the adapter, for example `hermes`, `code_runner`, `custom` |
| `description` | `text` | no | `''` | |
| `operator_config` | `jsonb` | no | `'{}'` | schema `agent_operator_config.v1` ([09-agents.md](09-agents.md), section 2.2): default model (`default_model`) and allowed adapter features (`supports_followups`, `supports_components`, `supports_voice`, `supports_title`, `supports_files_out`); a missing key allows; no secrets. An adapter feature is effective only when the adapter also reports it ([09-agents.md](09-agents.md), section 2.2) |
| `billing_mode` | `text` | no | `'organization'` | `CHECK (billing_mode IN ('personal','organization'))` |
| `credential_holder_id` | `uuid` | yes | | required for `personal`, otherwise NULL |
| `billing_mode_changed_at` | `timestamptz` | yes | | |
| `billing_mode_changed_by` | `uuid` | yes | | |
| `tool_class` | `text` | no | `'chat'` | `CHECK (tool_class IN ('chat','scoped','host'))`; configured tool class, order `chat` < `scoped` < `host` ([07-permissions.md](07-permissions.md), section 2.11) |
| `reported_tool_class` | `text` | yes | | reported value from the heartbeat, same values; `authorize` uses the higher of configured and reported |
| `reported_runtime_state` | `text` | no | `'unknown'` | `CHECK (reported_runtime_state IN ('unknown','online','degraded','offline'))` |
| `reported_billing_mode` | `text` | yes | | last reported mode from the heartbeat, `personal` or `organization`; NULL without a heartbeat or for adapters that follow the configured mode per run (then `reported_details.billing_modes_supported` names the modes that can be proven; the check per run is binding) |
| `reported_runtime_version` | `text` | yes | | |
| `reported_details` | `jsonb` | no | `'{}'` | schema `agent_heartbeat.v1` ([09-agents.md](09-agents.md), section 4.8): `capabilities`, `billing_modes_supported`, `billing_evidence` (`provider`, `key_type`, `method`, never a value), `active_runs`, `degraded_reasons`, `rate_limit`; without secrets and without content |
| `last_heartbeat_at` | `timestamptz` | yes | | |
| `version` | `bigint` | no | `1` | |

PK: `principal_id`. FK: `principal_id` -> `principals(id)` ON DELETE RESTRICT;
`credential_holder_id` -> `users(principal_id)` ON DELETE RESTRICT; `billing_mode_changed_by` ->
`principals(id)` ON DELETE RESTRICT.
Checks: `(billing_mode = 'personal') = (credential_holder_id IS NOT NULL)`.
Indexes: `(credential_holder_id) WHERE billing_mode = 'personal'` for the I1 check on
changes to a person; `(last_heartbeat_at)` for the presence evaluation.
Retention: `account`.

Heartbeat fields are written at most once per `agent.heartbeat_persist_interval`;
in between, the server keeps the state in memory. Exception: a change of
`reported_billing_mode` or `reported_tool_class` is stored and audited at once
(`agent.heartbeat`, [05-actions.md](05-actions.md)), because the reported mode is the evidence for I3.

#### `agent_consumer_cursors`

Purpose: durable read position of an agent adapter in the event log. Module: `agents`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `agent_id` | `uuid` | no | | |
| `consumer_name` | `text` | no | `'default'` | several consumers per agent possible |
| `acked_seq` | `bigint` | no | | highest acknowledged global sequence |
| `holder_session_id` | `uuid` | yes | | gateway session that currently holds the consumer (fencing across nodes, [04-sync-protocol.md](04-sync-protocol.md), section 15.1) |
| `created_at` | `timestamptz` | no | `now()` | starting point; catch-up never before this time |
| `updated_at` | `timestamptz` | no | `now()` | |

PK: `(agent_id, consumer_name)`. FK: `agent_id` -> `agents(principal_id)` ON DELETE CASCADE.
Indexes: covered by the PK. Retention: `security`.
`acked_seq` is stored as `UPDATE ... WHERE holder_session_id = $own_session`;
if a node gets no row back, another node has taken over the consumer, and it closes
its connection with `4015`.

#### `dev_environments`

Purpose: isolated dev environment of a person for a runner agent
([21-dev-isolation.md](21-dev-isolation.md), section 6). Milestone: multi-user dev environment.
Module: `agents`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | |
| `user_id` | `uuid` | no | | person who owns the environment |
| `runner_agent_id` | `uuid` | no | | runner that operates the environment |
| `isolation_kind` | `text` | no | | `CHECK (isolation_kind IN ('macos_vm','linux_vm','cloud_vm'))` |
| `environment_ref` | `text` | no | | opaque reference that the runner resolves; no path and no hostname of the server |
| `image_ref` | `text` | no | | version of the clean image |
| `billing_mode` | `text` | no | `'organization'` | `CHECK (billing_mode IN ('organization'))`; a personal backend policy per environment needs its own decision on I1 (R-DM-24) |
| `state` | `text` | no | `'provisioning'` | `CHECK (state IN ('provisioning','verifying','ready','suspended','retired'))` |
| `checks` | `jsonb` | no | `'{}'` | schema `dev_environment_checks.v1`: result and time per check |
| `verified_at` | `timestamptz` | yes | | last fully passed check |
| `red_team_passed_at` | `timestamptz` | yes | | red team passed for the kind and the image |
| `red_team_ref` | `text` | yes | | reference to the report |
| `approved_by` | `uuid` | yes | | owner with step-up |
| `approved_at` | `timestamptz` | yes | | |
| `suspended_reason` | `text` | yes | | for example `check_failed`, `user_disabled`, `manual` |
| `created_at` | `timestamptz` | no | `now()` | |
| `retired_at` | `timestamptz` | yes | | |
| `version` | `bigint` | no | `1` | |

PK: `id`. FK: `user_id` -> `users(principal_id)`, `runner_agent_id` -> `agents(principal_id)`,
`approved_by` -> `principals(id)`, all ON DELETE RESTRICT. Unique: partial index `(user_id,
runner_agent_id) WHERE state <> 'retired'`. Indexes: covered by the unique index.
Retention: `security`. When an environment is ready is defined in
[21-dev-isolation.md](21-dev-isolation.md), section 6.2; it is checked on every
trigger (guard rule SR3 in [07-permissions.md](07-permissions.md)).

#### `apps`

Purpose: additional data for app principals (integrations) with a manifest. Module: `apps`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `principal_id` | `uuid` | no | | `kind = 'app'` checked by a trigger |
| `slug` | `text` | no | | |
| `manifest` | `jsonb` | no | | schema `app_manifest.v1`: scopes, event subscriptions, slash commands, components, webhook URLs |
| `manifest_version` | `text` | no | | SemVer of the manifest |
| `manifest_sha256` | `bytea` | no | | |
| `publisher` | `text` | no | `''` | |
| `homepage_url` | `text` | yes | | |
| `created_at` | `timestamptz` | no | `now()` | |
| `updated_at` | `timestamptz` | no | `now()` | |

PK: `principal_id`. FK: `principal_id` -> `principals(id)` ON DELETE RESTRICT.
Unique: `lower(slug)`. Indexes: covered by the unique index. Retention: `account`.

#### `app_installations`

Purpose: installation of an app in this instance with granted scopes. Module: `apps`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | |
| `app_id` | `uuid` | no | | |
| `granted_scopes` | `text[]` | no | `'{}'` | subset of the manifest scopes |
| `event_subscriptions` | `text[]` | no | `'{}'` | event types from [05-actions.md](05-actions.md) |
| `state` | `text` | no | `'active'` | `CHECK (state IN ('active','suspended','uninstalled'))`; `app.suspend` sets `suspended` (emergency stop without losing the granted scopes), back with `app.resume` |
| `installed_by` | `uuid` | no | | |
| `installed_at` | `timestamptz` | no | `now()` | |
| `uninstalled_at` | `timestamptz` | yes | | |
| `uninstalled_by` | `uuid` | yes | | |

PK: `id`. FK: `app_id` -> `apps(principal_id)` ON DELETE RESTRICT; `installed_by`,
`uninstalled_by` -> `principals(id)` ON DELETE RESTRICT. Unique: partial index `(app_id) WHERE
state <> 'uninstalled'`. Indexes: covered by the unique index. Retention: `config`.

#### `commands`

Purpose: slash commands with options, registered by agents or apps. Module: `agents`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | |
| `owner_principal_id` | `uuid` | no | | agent or app |
| `name` | `text` | no | | `CHECK (name ~ '^[a-z0-9][a-z0-9_-]*$')` |
| `description` | `text` | no | | |
| `scope_kind` | `text` | no | `'instance'` | `CHECK (scope_kind IN ('instance','space','channel'))` |
| `scope_space_id` | `uuid` | yes | | |
| `scope_channel_id` | `uuid` | yes | | |
| `options` | `jsonb` | no | `'[]'` | schema `command_options.v1`: name, type (`string`, `integer`, `boolean`, `choice`, `user`, `file`), required, choice values, `autocomplete` |
| `required_level` | `text` | no | `'trigger'` | `CHECK (required_level IN ('write','trigger'))` |
| `control` | `boolean` | no | `false` | `true`: control command (for example `stop`, `status`), never goes to the model as a prompt |
| `version` | `bigint` | no | `1` | |
| `created_at` | `timestamptz` | no | `now()` | |
| `removed_at` | `timestamptz` | yes | | |

PK: `id`. FK: `owner_principal_id` -> `principals(id)` ON DELETE RESTRICT; `scope_space_id` ->
`spaces(id)` ON DELETE CASCADE; `scope_channel_id` -> `channels(id)` ON DELETE CASCADE.
Checks: scope columns matching `scope_kind`.
Unique: partial index `(owner_principal_id, lower(name), scope_kind, coalesce(scope_space_id,
scope_channel_id)) WHERE removed_at IS NULL`.
Indexes: `(owner_principal_id) WHERE removed_at IS NULL` for the command list of an agent;
`lower(name) text_pattern_ops WHERE removed_at IS NULL` for the suggestion list while typing.
Retention: `config`.
The display resolves name conflicts between agents: commands of the channel's operator come
first; equal names of other agents are qualified with the agent handle.

### 4.5 Structure (`chat`)

#### `spaces`

Purpose: space, top-level group of the sidebar. Module: `chat`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | |
| `name` | `text` | no | | for example "Support"; at most `space.name_max_chars` characters, without control and bidi characters |
| `icon` | `text` | yes | | glyph name from the list `space-icons` of the design tokens (`kernel/designkeys`), no emoji |
| `color` | `text` | yes | | key of a category color of the design system, no hex value |
| `sort_key` | `text COLLATE "C"` | no | | global order set by an admin |
| `default_operator_agent_id` | `uuid` | yes | | default operator; NULL = none |
| `created_by` | `uuid` | no | | |
| `created_at` | `timestamptz` | no | `now()` | |
| `archived_at` | `timestamptz` | yes | | |
| `version` | `bigint` | no | `1` | |

PK: `id`. FK: `default_operator_agent_id` -> `agents(principal_id)` ON DELETE RESTRICT;
`created_by` -> `principals(id)` ON DELETE RESTRICT. Unique: partial index `lower(name) WHERE
archived_at IS NULL`. Indexes: `(sort_key) WHERE archived_at IS NULL` for the sidebar;
`(default_operator_agent_id)` for the I1 check. Retention: `content`.

#### `user_space_states`

Purpose: personal sidebar state per space (collapsed, own order),
across devices. Module: `chat`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `user_id` | `uuid` | no | | |
| `space_id` | `uuid` | no | | |
| `collapsed` | `boolean` | no | `false` | |
| `sort_key` | `text COLLATE "C"` | yes | | NULL = global order |
| `version` | `bigint` | no | `1` | for the rule see [04-sync-protocol.md](04-sync-protocol.md) |
| `updated_by_device_id` | `uuid` | yes | | |
| `updated_at` | `timestamptz` | no | `now()` | |

PK: `(user_id, space_id)`. FK: `user_id` -> `users(principal_id)` ON DELETE CASCADE;
`space_id` -> `spaces(id)` ON DELETE CASCADE; `updated_by_device_id` -> `devices(id)` ON DELETE
SET NULL. Indexes: covered by the PK. Retention: `personal_sync`.

#### `channels`

Purpose: channel (text, forum or direct message) with operator and behavior. Module: `chat`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | |
| `space_id` | `uuid` | yes | | NULL exactly when `kind = 'dm'` (R-DM-10) |
| `kind` | `text` | no | | `CHECK (kind IN ('text','forum','dm'))` |
| `name` | `text` | no | | at most `channel.name_max_chars` characters, without control and bidi characters; empty for `dm`, display from the participants |
| `topic` | `text` | no | `''` | at most `channel.topic_max_chars` characters, without control and bidi characters |
| `sort_key` | `text COLLATE "C"` | no | | order within the space |
| `operator_inherit` | `boolean` | no | `true` | `true`: operator = `spaces.default_operator_agent_id` |
| `operator_agent_id` | `uuid` | yes | | only with `operator_inherit = false`; NULL = no operator |
| `require_mention` | `boolean` | no | `true` | main thread: operator only on a mention |
| `thread_require_mention` | `boolean` | no | `false` | in threads: operator only on a mention |
| `free_response` | `boolean` | no | `false` | `true`: the operator answers everywhere without a mention; overrides both fields above |
| `auto_thread` | `boolean` | no | `true` | a trigger in the main thread creates a thread |
| `allow_mentioned_agents` | `boolean` | no | `false` | further agents allowed by mention (only if their grants match) |
| `history_backfill_limit` | `integer` | no | | number of earlier messages as context at thread start; default from `channel.history_backfill_limit_default`; `CHECK (history_backfill_limit BETWEEN 0 AND channel.history_backfill_limit_max)` by the application |
| `prompt` | `text` | no | `''` | channel prompt for the operator; length up to `channel.prompt_max_bytes` |
| `forum_default_sort` | `text` | yes | | forum only: `CHECK (forum_default_sort IN ('last_activity','created'))` |
| `default_notification_level` | `text` | yes | | `CHECK (default_notification_level IN ('all','mentions','mute'))`; channel preset for people without their own setting (report channels); set with `channel.update` |
| `read_only` | `boolean` | no | `false` | read-only for all principals, owners included; only `import.apply` writes ([05-actions.md](05-actions.md), rule 1.8); set with `channel.read_only.set` |
| `created_by` | `uuid` | no | | |
| `created_at` | `timestamptz` | no | `now()` | |
| `archived_at` | `timestamptz` | yes | | |
| `version` | `bigint` | no | `1` | |
| `settings_version` | `bigint` | no | `1` | separate counter for behavior, prompt, skills, project; events name only this number |

PK: `id`. FK: `space_id` -> `spaces(id)` ON DELETE RESTRICT; `operator_agent_id` ->
`agents(principal_id)` ON DELETE RESTRICT; `created_by` -> `principals(id)` ON DELETE RESTRICT.
Checks: `(kind = 'dm') = (space_id IS NULL)`; `NOT operator_inherit OR operator_agent_id IS
NULL`; for `kind = 'dm'`: `operator_inherit = false AND operator_agent_id IS NULL AND NOT
read_only`.
Unique: partial index `(space_id, lower(name)) WHERE archived_at IS NULL AND kind <> 'dm'`.
Indexes: `(space_id, sort_key) WHERE archived_at IS NULL` for the sidebar;
`(operator_agent_id) WHERE operator_agent_id IS NOT NULL` for "which channels does this agent
operate" (I1). Retention: `content`.

Effective operator: `CASE WHEN operator_inherit THEN spaces.default_operator_agent_id ELSE
operator_agent_id END`. Mention rule: the operator is triggered when an operator exists
and one of these holds: `free_response`, or (main thread and not `require_mention`), or (thread and not
`thread_require_mention`), or the operator is mentioned. All further conditions are listed under
`agent.invoke` in [05-actions.md](05-actions.md); the whole decision per message is in
[09-agents.md](09-agents.md), section 3.4.

#### `channel_skill_bindings`

Purpose: ordered list of skills that the operator loads in this channel. Module: `chat`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `channel_id` | `uuid` | no | | |
| `position` | `integer` | no | | 0, 1, 2 ... |
| `skill_ref` | `text` | no | | identifier that the adapter understands; the server does not interpret it |

PK: `(channel_id, position)`. FK: `channel_id` -> `channels(id)` ON DELETE CASCADE.
Unique: `(channel_id, skill_ref)`. Indexes: covered by the PK. Retention: `config`.

#### `project_bindings`

Purpose: project binding of a forum for dev projects (runner agent as operator).
Module: `chat`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `channel_id` | `uuid` | no | | only `kind = 'forum'` |
| `workspace_ref` | `text` | no | | opaque reference that the runner resolves (for example a project name); never an absolute path of the server |
| `repository_url` | `text` | yes | | for display; the server never loads this URL |
| `default_branch` | `text` | yes | | |
| `runner_config` | `jsonb` | no | `'{}'` | schema `project_runner_config.v1`: model, allowed tools of the runtime within `agents.tool_class`; no secrets |
| `ready` | `boolean` | no | `false` | reported by the runner via heartbeat: work environment ready |
| `created_by` | `uuid` | no | | |
| `created_at` | `timestamptz` | no | `now()` | |
| `version` | `bigint` | no | `1` | |

PK: `channel_id`. FK: `channel_id` -> `channels(id)` ON DELETE CASCADE; `created_by` ->
`principals(id)` ON DELETE RESTRICT. Unique: `lower(workspace_ref)`. Indexes: covered by the PK and the unique index.
Retention: `config`.

#### `dm_participants`

Purpose: participants of a direct message channel. Module: `chat`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `channel_id` | `uuid` | no | | only `kind = 'dm'` |
| `user_id` | `uuid` | no | | people only |
| `joined_at` | `timestamptz` | no | `now()` | |
| `left_at` | `timestamptz` | yes | | |

PK: `(channel_id, user_id)`. FK: `channel_id` -> `channels(id)` ON DELETE CASCADE; `user_id`
-> `users(principal_id)` ON DELETE RESTRICT. Indexes: `(user_id) WHERE left_at IS NULL` for the
DM list. Retention: `content`.

#### `forum_tags`

Purpose: tags of a forum. Module: `chat`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | |
| `channel_id` | `uuid` | no | | forum only |
| `name` | `text` | no | | |
| `color` | `text` | yes | | key of a category color of the design system, no hex value |
| `sort_key` | `text COLLATE "C"` | no | | |
| `restricted` | `boolean` | no | `false` | `true`: can be set only with `manage` |
| `deleted_at` | `timestamptz` | yes | | |

PK: `id`. FK: `channel_id` -> `channels(id)` ON DELETE CASCADE. Unique: partial index
`(channel_id, lower(name)) WHERE deleted_at IS NULL`. Indexes: covered by the unique index.
Retention: `content`. `name` at most `forum_tag.name_max_chars` characters. Every change to
tags increments `channels.version` and writes `channel.updated`; clients then reload the tags
(05, section 6).

#### `threads`

Purpose: unit of conversation and of agent session with a fixed ID. Module: `chat`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | fixed, never changes, not even on a move |
| `channel_id` | `uuid` | no | | |
| `kind` | `text` | no | | `CHECK (kind IN ('main','thread'))`; `main` = continuous history of a text or DM channel (R-DM-11) |
| `parent_message_id` | `uuid` | yes | | message in the main thread from which the thread arose; stays on a move and then points into the source channel (retrievable only with read permission there) |
| `title` | `text` | no | `''` | length up to `thread.title_max_chars` |
| `title_source` | `text` | no | `'auto'` | `CHECK (title_source IN ('auto','manual'))`; once `manual`, it stays `manual` |
| `state` | `text` | no | `'open'` | `CHECK (state IN ('open','closed'))` |
| `closed_at` | `timestamptz` | yes | | |
| `closed_by` | `uuid` | yes | | |
| `operator_agent_id` | `uuid` | yes | | only `kind = 'thread'`: operator at the time of creation, fixed afterwards (R-DM-12); on import from the owner's mapping ([22-import-migration.md](22-import-migration.md)). Always NULL for the main thread: there the effective operator of the channel at the time of the trigger applies (R-DM-21) |
| `last_message_seq` | `bigint` | no | `0` | counter of the thread sequence |
| `unread_floor_seq` | `bigint` | no | `0` | messages up to this `thread_seq` count as unread for nobody; the import sets it to `last_message_seq` at the end of each batch |
| `last_activity_at` | `timestamptz` | no | `now()` | sorting of the post list; on import from the source |
| `message_count` | `integer` | no | `0` | excluding deleted ones |
| `created_by` | `uuid` | no | | |
| `created_at` | `timestamptz` | no | `now()` | on import from the source |
| `moved_at` | `timestamptz` | yes | | last move |
| `version` | `bigint` | no | `1` | |
| `search_language` | `regconfig` | no | `'simple'` | text search configuration from the creator's language; for the choice see [14-search.md](14-search.md) |
| `title_search` | `tsvector` | no | generated | see section 5 |

PK: `id`. FK: `channel_id` -> `channels(id)` ON DELETE RESTRICT; `parent_message_id` ->
`messages(id)` ON DELETE RESTRICT (deferred, `DEFERRABLE INITIALLY DEFERRED`);
`operator_agent_id` -> `agents(principal_id)` ON DELETE RESTRICT; `closed_by`, `created_by` ->
`principals(id)` ON DELETE RESTRICT.
Checks: `(state = 'closed') = (closed_at IS NOT NULL)`; `kind = 'main'` has neither
`parent_message_id` nor `operator_agent_id`.
Unique: partial index `(channel_id) WHERE kind = 'main'` (exactly one main thread per text and
DM channel); `(parent_message_id) WHERE parent_message_id IS NOT NULL` (a message starts
at most one thread).
Indexes: `(channel_id, state, last_activity_at DESC)` for the post list with the
Done section; `(operator_agent_id, channel_id) WHERE operator_agent_id IS NOT NULL` for the
I1 targets, without a condition on `state`, because a new message reopens a done thread
(section 6.4); GIN `(title_search)` for search. Retention: `content`.

A move changes `channel_id`. It is allowed only for `kind = 'thread'` (the main thread belongs
permanently to its channel, R-DM-11) and only when the effective operator of the target channel equals
`operator_agent_id` (Baseline section 4). The same transaction changes the copies
`messages.channel_id`, `message_attachments.channel_id`, `read_states.channel_id` and
`drafts.channel_id` of this thread.
`cost_entries` and `event_log` keep the channel at the time of the booking or of the event, respectively.
Attached files follow their message ([05-actions.md](05-actions.md), section 2.2);
`files.scope_channel_id` applies only before attaching and stays unchanged.

#### `thread_tags`

Purpose: assignment of forum tags to threads. Module: `chat`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `thread_id` | `uuid` | no | | |
| `tag_id` | `uuid` | no | | the tag must belong to the channel of the thread (application) |
| `added_by` | `uuid` | no | | |
| `added_at` | `timestamptz` | no | `now()` | |

PK: `(thread_id, tag_id)`. FK: `thread_id` -> `threads(id)` ON DELETE CASCADE; `tag_id` ->
`forum_tags(id)` ON DELETE CASCADE; `added_by` -> `principals(id)` ON DELETE RESTRICT.
Indexes: `(tag_id)` for the tag filter of the post list. Retention: `content`.
At most `thread.max_tags` tags per thread.

When a thread is moved to another channel, its tags are removed, because a tag belongs to the
channel of the thread.

#### `agent_sessions`

Purpose: mapping of a thread to an external session of an agent. Older rows of the same
thread are the earlier session references. Module: `agents`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | |
| `thread_id` | `uuid` | no | | |
| `agent_id` | `uuid` | no | | |
| `external_session_ref` | `text` | no | | for example the session key of an adapter; the server does not interpret it |
| `is_current` | `boolean` | no | `true` | |
| `started` | `boolean` | no | `false` | `true` as soon as the external session exists (reported by the adapter with `agent_session.report`) |
| `model` | `text` | yes | | last reported model |
| `cumulative_cost_micros` | `bigint` | no | `0` | sum of the `cost_entries` of this session |
| `cost_baseline_micros` | `bigint` | no | `0` | cumulative amount of the external session before the binding (import, rebinding); if a runtime reports totals per session, the server books only the part above it |
| `currency` | `text` | no | `'USD'` | currency of both amounts |
| `cost_is_estimate` | `boolean` | no | `true` | until reconciliation |
| `dev_environment_id` | `uuid` | yes | | dev environment in which the first run of the session ran; NULL outside dev isolation (milestone, SR3) |
| `created_at` | `timestamptz` | no | `now()` | |
| `ended_at` | `timestamptz` | yes | | set when the session ends or was never current |
| `end_reason` | `text` | yes | | `CHECK (end_reason IN ('reset','rebind','import','agent_changed'))`, meaning below |
| `version` | `bigint` | no | `1` | compare-and-swap on rebinding and on `agent_session.report` |

PK: `id`. FK: `thread_id` -> `threads(id)` ON DELETE RESTRICT; `agent_id` ->
`agents(principal_id)` ON DELETE RESTRICT; `dev_environment_id` -> `dev_environments(id)` ON
DELETE RESTRICT.
Checks: `is_current = (ended_at IS NULL)`.
Unique: partial index `(thread_id) WHERE is_current` (one current session per thread); partial index
`(agent_id, external_session_ref) WHERE is_current` (an external session belongs to at most
one thread).
Indexes: `(thread_id, created_at DESC)` for the session history. Retention: `content`.

Meaning of `end_reason`:

| Value | Meaning |
|---|---|
| `reset` | new session via `agent_session.reset` (`/new`) |
| `rebind` | replaced by `agent_session.bind`, or because resuming a bound session failed and the adapter reported a new one |
| `import` | earlier session reference from an import; was never current in this product. On the first trigger without a current session, the server binds it exactly once as a new current row with `started = true` ([22-import-migration.md](22-import-migration.md), section 9) |
| `agent_changed` | operator change in the main thread; the session ends at the next run |

### 4.6 Messages (`messaging`)

#### `messages`

Purpose: a message in a thread, with thread sequence and idempotency key.
Module: `messaging`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | created by the server |
| `thread_id` | `uuid` | no | | |
| `channel_id` | `uuid` | no | | copy of `threads.channel_id` for the permission filter; changed along with a move |
| `thread_seq` | `bigint` | no | | from `threads.last_message_seq`, gapless per thread |
| `author_principal_id` | `uuid` | no | | |
| `client_msg_id` | `uuid` | no | | UUIDv7 from the client; for server messages created by the server |
| `client_payload_sha256` | `bytea` | no | | hash of the canonical request; detects reuse with different content |
| `origin` | `text` | no | | `CHECK (origin IN ('human','agent','app','import','system'))`; only `human` triggers agents |
| `kind` | `text` | no | `'text'` | `CHECK (kind IN ('text','voice','run','card','system'))`; `run` = anchor of the live box; `card` = message with components (question, approval) |
| `system_code` | `text` | yes | | `CHECK (system_code IN ('thread.reopened','agent_session.resume_failed'))`; set exactly when `kind = 'system'`; clients build the line in their language from code and author |
| `content_md` | `text` | no | `''` | Markdown; size up to `message.max_content_bytes`; empty after deletion and for system lines |
| `content_format` | `smallint` | no | `1` | version of the Markdown dialect |
| `reply_to_message_id` | `uuid` | yes | | reply with quote; same thread |
| `run_id` | `uuid` | yes | | for `kind = 'run'`: the run whose live box stands here |
| `trigger_state` | `text` | yes | | `CHECK (trigger_state IN ('received','queued','working','done','failed','cancelled','followup_delivered'))`; NULL = no triggering message; transitions in [09-agents.md](09-agents.md), section 6 |
| `trigger_state_reason` | `text` | yes | | reason code for `failed` and `cancelled`, for example `billing_mode_personal_not_holder`, `run_active_other_agent`, `user_cancelled`; also set on a rejection already at acceptance, when no run is created |
| `trigger_state_at` | `timestamptz` | yes | | |
| `silent` | `boolean` | no | `false` | no notification (for example import) |
| `has_attachments` | `boolean` | no | `false` | `true` exactly when the message was created with at least one attachment and is not deleted. Attachments arise only with the message and disappear only with its deletion (editing does not change them, `file.delete` applies only to files that are not attached), so `messaging` sets the value in the transaction of the creation; deletion sets it to `false`. Filter `has_file` of the search ([14-search.md](14-search.md), section 4.4); R-DM-43 |
| `import_item_id` | `uuid` | yes | | origin on import |
| `version` | `bigint` | no | `1` | rises on edit, deletion, state change; reactions and pinning do not raise it ([04-sync-protocol.md](04-sync-protocol.md), section 9) |
| `edited_at` | `timestamptz` | yes | | |
| `deleted_at` | `timestamptz` | yes | | |
| `deleted_by` | `uuid` | yes | | |
| `created_at` | `timestamptz` | no | `now()` | on import: original time |
| `search_language` | `regconfig` | no | `'simple'` | text search configuration; choice see [14-search.md](14-search.md) |
| `search_vector` | `tsvector` | no | generated | see section 5 |

PK: `id`.
FK: `thread_id` -> `threads(id)` ON DELETE RESTRICT; `channel_id` -> `channels(id)` ON DELETE
RESTRICT; `author_principal_id`, `deleted_by` -> `principals(id)` ON DELETE RESTRICT;
`reply_to_message_id` -> `messages(id)` ON DELETE SET NULL; `run_id` -> `runs(id)` ON DELETE
SET NULL (deferred); `import_item_id` -> `import_items(id)` ON DELETE SET NULL.
Checks: `(kind = 'run') = (run_id IS NOT NULL)`; `trigger_state IS NULL OR origin = 'human'`;
`trigger_state_reason IS NULL OR trigger_state IN ('failed','cancelled')`;
`deleted_at IS NULL OR content_md = ''`; `deleted_at IS NULL OR NOT has_attachments`;
`(kind = 'system') = (system_code IS NOT NULL)`;
`kind <> 'system' OR (origin = 'system' AND silent AND content_md = '')`.
Unique: `(thread_id, thread_seq)`; `(author_principal_id, client_msg_id)`; partial index `(run_id)
WHERE run_id IS NOT NULL` (one live box anchor per run).
Indexes:

- `(thread_id, thread_seq DESC)` for loading the history page by page (via the unique index).
- `(channel_id, created_at DESC)` for "newest messages of a channel" and backfill context.
- `(author_principal_id, created_at DESC)` for GDPR export and "own messages".
- `(trigger_state) WHERE trigger_state IN ('received','queued','working')` for resumption
  after a restart and "Running now".
- GIN `(search_vector)` for full-text search.

Retention: `content`.
Deletion empties `content_md`, sets `has_attachments` to `false`, deletes attachments, revisions,
mentions, components, reactions and the pin, and redacts the payload of the related events
(section 6.5). The row and
`thread_seq` remain. The server writes a system line `thread.reopened` when a message
reopens a thread that is done; the author is the person whose message reopened it. If the server
accepts a report `agent_session.report` with `bind_failed` (the session of the run is missing in
the runtime; fail open according to [09-agents.md](09-agents.md), section 9.5 item 4, and
[22-import-migration.md](22-import-migration.md), section 9 step 5), it writes in the same
transaction exactly one system line `agent_session.resume_failed`; the author is the agent of the
run. A system line does not reopen a done thread (R-DM-42).

Content format `content_format = 1`: Markdown according to CommonMark 0.31.2
(https://spec.commonmark.org/0.31.2/, retrieved 2026-10-10) [verified], without raw HTML
([19-security.md](19-security.md), SEC-OUT-1). Extensions such as tables get a new
`content_format`, because they change the rendering of existing texts. Mentions stand as text in
the content:

1. A mention token is `@` followed by a handle (`[a-z0-9][a-z0-9._-]*`, case-insensitive).
   Before the `@` stands the start of the text or a character that is neither a letter
   nor a digit nor one of `_ . - @ /`; `alex@example.com` is therefore not a mention.
   Trailing `.`, `-` and `_` do not belong to the handle (end of sentence).
2. Inline code and code blocks (fenced and indented, according to CommonMark) contain no
   mentions.
3. On send and edit, the server resolves each distinct handle once among active
   principals of the kind `user` or `agent`. Who can be mentioned is governed by
   [07-permissions.md](07-permissions.md), section 4.6; a token that cannot be resolved stays text.
   `@everyone` and `@here` have no special meaning.
4. Each resolved principal creates one row in `mentions`; `Message.mention_ids` lists them
   sorted. If the client sends `mention_ids`, the set must equal the resolved one, otherwise
   `400 validation_failed` with code `mention_mismatch`. The server is the source; clients show
   a token as a mention only when its principal is in `mention_ids`.
5. More than `message.max_mentions` distinct handles result in `400 validation_failed` with code
   `too_many_mentions`. Reason: the resolution is one query per message and stays bounded.

#### `message_revisions`

Purpose: earlier versions of edited messages. Module: `messaging`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | |
| `message_id` | `uuid` | no | | |
| `revision` | `integer` | no | | 1 = original version |
| `content_md` | `text` | no | | version before the change |
| `edited_by` | `uuid` | no | | |
| `created_at` | `timestamptz` | no | `now()` | |

PK: `id`. FK: `message_id` -> `messages(id)` ON DELETE CASCADE; `edited_by` ->
`principals(id)` ON DELETE RESTRICT. Unique: `(message_id, revision)`. Indexes: via the
unique index. Retention: `content`.
Live box anchors (`kind = 'run'`) create no revisions; their history is in `run_steps`.

#### `mentions`

Purpose: resolved mentions of people and agents (no roles, no @everyone).
Module: `messaging`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `message_id` | `uuid` | no | | |
| `principal_id` | `uuid` | no | | person or agent |
| `thread_id` | `uuid` | no | | copy for counters |
| `created_at` | `timestamptz` | no | `now()` | |

PK: `(message_id, principal_id)`. FK: `message_id` -> `messages(id)` ON DELETE CASCADE;
`principal_id` -> `principals(id)` ON DELETE RESTRICT; `thread_id` -> `threads(id)` ON DELETE
CASCADE. Indexes: `(principal_id, created_at DESC)` for "my mentions"; `(principal_id,
thread_id)` for unread mentions per thread. Retention: `content`. Editing replaces the
rows of a message, deletion removes them; a stored counter therefore does not have to follow
(`read_states`, R-DM-32).
A mention of a principal without read permission is stored, but creates no
notification and reveals nothing (P3).

#### `reactions`

Purpose: free Unicode reactions. Status is not a reaction. Module: `messaging`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `message_id` | `uuid` | no | | |
| `principal_id` | `uuid` | no | | |
| `emoji` | `text` | no | | exactly one Unicode grapheme cluster, at most `reaction.emoji_max_bytes` bytes, checked in the application |
| `created_at` | `timestamptz` | no | `now()` | |

PK: `(message_id, principal_id, emoji)`. FK: `message_id` -> `messages(id)` ON DELETE CASCADE;
`principal_id` -> `principals(id)` ON DELETE RESTRICT. Indexes: via the PK (count per
message). Retention: `content`. Per message at most `reaction.max_distinct_per_message`
distinct emoji; another new emoji results in `400 validation_failed` with code
`limit_reached`.

#### `pins`

Purpose: pinned messages per thread. Module: `messaging`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `message_id` | `uuid` | no | | |
| `thread_id` | `uuid` | no | | |
| `pinned_by` | `uuid` | no | | |
| `pinned_at` | `timestamptz` | no | `now()` | |

PK: `message_id`. FK: `message_id` -> `messages(id)` ON DELETE CASCADE; `thread_id` ->
`threads(id)` ON DELETE CASCADE; `pinned_by` -> `principals(id)` ON DELETE RESTRICT.
Indexes: `(thread_id, pinned_at DESC)` for the list of pinned messages.
Retention: `content`. Per thread at most `pin.max_per_thread` pins (otherwise `400
validation_failed`, code `limit_reached`).

#### `message_templates`

Purpose: templates for frequent inputs, personal or per channel. Module: `messaging`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | |
| `owner_user_id` | `uuid` | yes | | personal template |
| `channel_id` | `uuid` | yes | | channel template |
| `name` | `text` | no | | |
| `content_md` | `text` | no | | |
| `created_by` | `uuid` | no | | |
| `created_at` | `timestamptz` | no | `now()` | |
| `deleted_at` | `timestamptz` | yes | | |
| `version` | `bigint` | no | `1` | |

PK: `id`. FK: `owner_user_id` -> `users(principal_id)` ON DELETE CASCADE; `channel_id` ->
`channels(id)` ON DELETE CASCADE; `created_by` -> `principals(id)` ON DELETE RESTRICT.
Checks: exactly one of `owner_user_id` and `channel_id`. Indexes: `(owner_user_id) WHERE
deleted_at IS NULL`; `(channel_id) WHERE deleted_at IS NULL`. Retention: `content`. `name`
has at most `template.name_max_chars` characters without control and bidi characters; per
person or per channel at most `template.max_per_scope` templates (otherwise
`400 validation_failed`, code `limit_reached`).
Permissions: personal templates through the person (resource `user`, `self`), channel
templates through the channel (`manage`), see `template.manage` in [05-actions.md](05-actions.md).

### 4.7 Files (`files`)

#### `files`

Purpose: an uploaded file in object storage with checksum, scan and transcript.
Module: `files`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | |
| `object_key` | `text` | no | | key in object storage (B4); contains no names or IDs of channels |
| `sha256` | `bytea` | no | | over the stored bytes |
| `size_bytes` | `bigint` | no | | up to `file.max_upload_bytes` |
| `mime_type` | `text` | no | | detected by the server (sniffing), not taken from the client |
| `declared_mime_type` | `text` | yes | | the client's statement, for diagnosis only |
| `original_name` | `text` | no | | sanitized |
| `uploaded_by` | `uuid` | no | | |
| `purpose` | `text` | no | `'attachment'` | `CHECK (purpose IN ('attachment','avatar','branding','gdpr_export'))`; determines the read path ([05-actions.md](05-actions.md), section 2.2) |
| `scope_channel_id` | `uuid` | yes | | only `attachment`: channel for which the file was uploaded; governs access before attaching, after that the channel of the message applies |
| `upload_state` | `text` | no | `'pending'` | `CHECK (upload_state IN ('pending','complete','failed'))` |
| `scan_status` | `text` | no | `'pending'` | `CHECK (scan_status IN ('pending','clean','infected','error','skipped'))` |
| `scan_engine` | `text` | yes | | |
| `scanned_at` | `timestamptz` | yes | | |
| `width` | `integer` | yes | | images and videos |
| `height` | `integer` | yes | | |
| `duration_ms` | `bigint` | yes | | audio and video |
| `transcript_status` | `text` | no | `'none'` | `CHECK (transcript_status IN ('none','pending','done','failed'))` |
| `transcript_text` | `text` | yes | | |
| `transcript_language` | `text` | yes | | BCP 47 |
| `transcript_source` | `text` | yes | | `CHECK (transcript_source IN ('agent','service'))`; `files` has no `version` column: the ETag of a transcript (`If-Match` of `file.transcript.submit`) is derived as a strong hash over `transcript_status`, `transcript_source`, `transcript_language` and `transcript_text` (R-DM-48) |
| `created_at` | `timestamptz` | no | `now()` | |
| `deleted_at` | `timestamptz` | yes | | object is removed by the cleanup job |
| `search_vector` | `tsvector` | no | generated | name and transcript, see section 5 |

PK: `id`. FK: `uploaded_by` -> `principals(id)` ON DELETE RESTRICT; `scope_channel_id` ->
`channels(id)` ON DELETE RESTRICT. Checks: `(purpose = 'attachment') = (scope_channel_id IS NOT
NULL)`. Unique: `object_key`. Indexes: `(uploaded_by, created_at DESC)` for export and
quota; `(scan_status) WHERE scan_status = 'pending'` for the scan queue;
`(upload_state, created_at) WHERE upload_state = 'pending'` for cleaning up aborted
uploads; GIN `(search_vector)`. Retention: `content`.
No deduplication across files (R-DM-13). Download only through the server after
`file.read`, also for media players through short-lived file tickets instead of presigned GET
([12-files.md](12-files.md), section 9;
[ADR 0048](../adr/0048-file-download-only-through-the-server-with-short-lived-tickets.md)); served
only when `scan_status IN ('clean','skipped')`. For JPEG and PNG, `sha256`, `size_bytes`,
`width` and `height` describe the stored original after the lossless removal of the metadata
([12-files.md](12-files.md), section 7).

#### `file_thumbnails`

Purpose: thumbnails of a file. Module: `files`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `file_id` | `uuid` | no | | |
| `variant` | `text` | no | | `CHECK (variant IN ('small','medium','large','poster'))` |
| `object_key` | `text` | no | | |
| `mime_type` | `text` | no | | |
| `width` | `integer` | no | | |
| `height` | `integer` | no | | |
| `size_bytes` | `bigint` | no | | |

PK: `(file_id, variant)`. FK: `file_id` -> `files(id)` ON DELETE CASCADE. Unique:
`object_key`. Indexes: via the PK. Retention: `content`.

#### `message_attachments`

Purpose: attachments of a message in a fixed order. Module: `files`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `message_id` | `uuid` | no | | |
| `file_id` | `uuid` | no | | |
| `position` | `smallint` | no | | up to `message.max_attachments` |
| `thread_id` | `uuid` | no | | copy of `messages.thread_id`; fixed, because a message never changes its thread |
| `channel_id` | `uuid` | no | | copy of `messages.channel_id` for permission filters (search); changed along on a move |

PK: `(message_id, position)`. FK: `message_id` -> `messages(id)` ON DELETE CASCADE; `file_id`
-> `files(id)` ON DELETE RESTRICT. Unique: `(file_id)` (a file is attached to at most one
message, R-DM-22); the index also serves "where is this file attached" for `file.read`. Indexes:
`(thread_id)` for following a move. Retention: `content`.
A file may only be attached when `purpose = 'attachment'`, its `scope_channel_id` is the
channel of the message and its upload session is `completed` (application,
[12-files.md](12-files.md), section 10).
`files` writes `thread_id` and `channel_id` on attaching from the message that `messaging` creates
in the same transaction. On moving a thread, `files` changes `channel_id` in the transaction of
the move (see `threads`; in-transaction event `chat.ThreadMoved` as for `messaging` and `sync`,
[02-architecture.md](02-architecture.md), section 4.3). So `files` filters attached files by
channel and thread of their message with `channel_id = ANY($visible)` in its own tables
([07-permissions.md](07-permissions.md), section 7.4; [14-search.md](14-search.md), section 4.3),
without reading `messages` (principle 12); `files.scope_channel_id` does not count for this. The
copies deliberately have no foreign key of their own: the row hangs on its message through
`message_id` and goes with it (ON DELETE CASCADE), and thread and channel of the message hold the
foreign keys of `messages` (ON DELETE RESTRICT); a foreign key to `threads` or `channels` would
only check existence. That the copies equal the values of the message is ensured by attaching and
by the transaction of the move, as for `messages.channel_id` (R-DM-43).

#### `file_upload_sessions`

Purpose: resumable upload of a file in parts, the same for both storage drivers
([12-files.md](12-files.md), section 3). Module: `files`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `file_id` | `uuid` | no | | one session per file |
| `mode` | `text` | no | `'direct'` | `CHECK (mode IN ('direct','presigned'))` |
| `state` | `text` | no | `'open'` | `CHECK (state IN ('open','completed','aborted','expired'))` |
| `declared_size_bytes` | `bigint` | no | | announced by the client |
| `declared_sha256` | `bytea` | no | | announced by the client; is kept after the removal of the metadata |
| `part_size_bytes` | `bigint` | no | | `file.upload_part_bytes` |
| `part_count` | `integer` | no | | `declared_size_bytes / part_size_bytes`, rounded up |
| `storage_upload_id` | `text` | yes | | multipart upload ID with the S3 driver |
| `failure_reason` | `text` | yes | | code without contents |
| `created_at` | `timestamptz` | no | `now()` | |
| `completed_at` | `timestamptz` | yes | | |
| `expires_at` | `timestamptz` | no | | `created_at + file.upload_session_ttl` |

PK: `file_id`. FK: `file_id` -> `files(id)` ON DELETE CASCADE. Indexes: `(expires_at) WHERE state
= 'open'` for cleanup. Retention: `transient`.

#### `file_upload_parts`

Purpose: confirmed parts of an upload session. Module: `files`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `file_id` | `uuid` | no | | |
| `part_number` | `integer` | no | | from 1 |
| `size_bytes` | `bigint` | no | | |
| `sha256` | `bytea` | no | | from `Content-Digest`; the same number with a different digest is a conflict |
| `etag` | `text` | yes | | ETag of the S3 part |
| `received_at` | `timestamptz` | no | `now()` | |

PK: `(file_id, part_number)`. FK: `file_id` -> `file_upload_sessions(file_id)` ON DELETE CASCADE.
Indexes: via the PK. Retention: `transient`.

#### `file_text_extracts`

Purpose: text extracted from a file for search ([14-search.md](14-search.md), section 5).
Module: `files`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `file_id` | `uuid` | no | | |
| `state` | `text` | no | `'pending'` | `CHECK (state IN ('pending','done','failed','unsupported'))` |
| `extractor` | `text` | yes | | name and version of the extractor |
| `text_content` | `text` | yes | | at most `search.extract.max_text_bytes` |
| `truncated` | `boolean` | no | `false` | |
| `search_language` | `regconfig` | no | `'simple'` | language of the instance |
| `extracted_at` | `timestamptz` | yes | | |
| `error` | `text` | yes | | code without contents |
| `search_vector` | `tsvector` | no | generated | see section 5 |

PK: `file_id`. FK: `file_id` -> `files(id)` ON DELETE CASCADE. Indexes: GIN `(search_vector)`;
`(state) WHERE state = 'pending'` for the job. Retention: `content`.

### 4.8 Runs and interactions (`runs`)

#### `runs`

Purpose: one agent turn; at the same time the live box. Module: `runs`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | |
| `thread_id` | `uuid` | no | | |
| `agent_id` | `uuid` | no | | |
| `agent_session_id` | `uuid` | yes | | set at start or at the first `agent_session.report`; NULL for guest runs |
| `trigger_kind` | `text` | no | | `CHECK (trigger_kind IN ('message','command','interaction'))` |
| `triggering_message_id` | `uuid` | yes | | set when `trigger_kind = 'message'`; for a run from deferred follow-ups the first of these messages |
| `triggering_command_id` | `uuid` | yes | | set exactly when `trigger_kind = 'command'` |
| `triggering_interaction_id` | `uuid` | yes | | set exactly when `trigger_kind = 'interaction'` (interaction without a waiting run) |
| `triggered_by` | `uuid` | no | | person who triggered |
| `state` | `text` | no | `'queued'` | `CHECK (state IN ('queued','running','waiting_input','completed','failed','cancelled'))` |
| `state_reason` | `text` | yes | | reason code from [09-agents.md](09-agents.md), section 5.2, for example `billing_mode_mismatch`, `access_revoked`, `stale`, `user_cancelled`, `channel_read_only` |
| `version` | `bigint` | no | `1` | rises with every stored change; basis of the patches |
| `billing_mode_expected` | `text` | no | | configured mode from `agents.billing_mode` at the last delivery; every delivery sets it anew |
| `billing_mode_used` | `text` | yes | | reported mode, as reported by the adapter per run |
| `billing_evidence` | `jsonb` | no | `'{}'` | schema `billing_evidence.v1` from `run.start`: `provider`, `key_type`, `method`; never a value, never a prefix |
| `invocation_jti` | `uuid` | no | | `jti` of the token last issued with a delivery that allows a start (`agent.invocation`; in `waiting_input` also `agent.followup` and `interaction.created`); deliveries during `running` do not change it |
| `started_jti` | `uuid` | yes | | `jti` of the token of the last successful `run.start`; a repetition with the same token returns the result again instead of `409` as long as the run is `running`; in `waiting_input` the token is used up and is rejected |
| `renewed_from_jti` | `uuid` | yes | | `jti` of the token that the last `run.token.renew` replaced; a repetition with it issues again (lost response). Every delivery that replaces `invocation_jti` and every successful `run.start` set `NULL` ([09-agents.md](09-agents.md), section 4.5.1) |
| `dev_environment_id` | `uuid` | yes | | dev environment of the run (milestone, SR3); also stands in the token |
| `model` | `text` | yes | | |
| `status_line` | `text` | yes | | current status line |
| `last_text_excerpt` | `text` | yes | | last text of the agent, shortened to `run.excerpt_max_chars` |
| `plan` | `jsonb` | no | `'{}'` | schema `run_plan.v1`: `items` (`id`, `text`, `status`) for the live box |
| `metrics` | `jsonb` | no | `'{}'` | schema `run_metrics.v1`: `context_pct`, `tokens_in`, `tokens_out`, `tokens_cache_read`, `tokens_cache_write`, `tokens_cache_write_1h`, `subagents`, `background_jobs`, `api_retries`, `last_api_retry` (`attempt`, `max_retries`, `retry_delay_ms`, `error`) |
| `cost_micros` | `bigint` | no | `0` | |
| `currency` | `text` | no | `'USD'` | |
| `cost_is_estimate` | `boolean` | no | `true` | |
| `summary` | `text` | yes | | collapsed line after the end |
| `stall_warned_at` | `timestamptz` | yes | | stall notice after `run.stall_after` |
| `queued_at` | `timestamptz` | no | `now()` | |
| `invocation_held_until` | `timestamptz` | yes | | set exactly when the acceptance holds the event `agent.invocation` of the run because an attachment of the triggering message is not final, or because the server transcript of a voice message (`kind = voice` in transcription mode `service`) is still `pending` (R-DM-47) ([04-sync-protocol.md](04-sync-protocol.md), section 15.3 step 1); the value is the latest time of the release (acceptance plus `file.invocation_hold_max` or `voice.transcript_hold_max`); `ReleaseInvocation` sets NULL in the transaction that writes the event (`version` rises, no `run.state_changed`); a run that ends before keeps the value and is never released (R-DM-44) |
| `started_at` | `timestamptz` | yes | | |
| `finished_at` | `timestamptz` | yes | | |
| `duration_ms` | `bigint` | yes | | |

PK: `id`. FK: `thread_id` -> `threads(id)` ON DELETE RESTRICT; `agent_id` ->
`agents(principal_id)` ON DELETE RESTRICT; `agent_session_id` -> `agent_sessions(id)` ON DELETE
SET NULL; `triggering_message_id` -> `messages(id)` ON DELETE SET NULL;
`triggering_command_id` -> `commands(id)` ON DELETE SET NULL; `triggering_interaction_id` ->
`interactions(id)` ON DELETE SET NULL; `triggered_by` -> `principals(id)` ON DELETE RESTRICT;
`dev_environment_id` -> `dev_environments(id)` ON DELETE RESTRICT.
Checks: `billing_mode_expected IN ('personal','organization')`; final states have
`finished_at`; at most one of the three columns `triggering_*` is set, and only the one matching
`trigger_kind` (it may later become empty through `ON DELETE SET NULL`).
Unique: `invocation_jti`; partial index `(thread_id) WHERE state IN ('queued','running',
'waiting_input')` (at most one active run per thread, R-DM-14).
Indexes: `(thread_id, queued_at DESC)` for the run history; `(state, agent_id) WHERE state
IN ('queued','running','waiting_input')` for "Running now" and resumption;
`(agent_id, finished_at DESC)` for statistics. Retention: `content`.

State transitions (everything else is rejected): `queued -> running | failed | cancelled`;
`running -> waiting_input | completed | failed | cancelled`; `waiting_input -> running |
failed | cancelled`. Every transition is a compare-and-swap on `state` and `version`. Both
transitions to `running` are `run.start` with a valid token
([04-sync-protocol.md](04-sync-protocol.md), section 15.3).

#### `run_steps`

Purpose: ordered steps of a run (tool, status, subagent, workflow,
background job). Module: `runs`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | created by the adapter (UUIDv7), so that repetitions are idempotent |
| `run_id` | `uuid` | no | | |
| `ordinal` | `integer` | no | | order in the run, assigned by the server |
| `parent_step_id` | `uuid` | yes | | nesting (tool of a subagent) |
| `kind` | `text` | no | | `CHECK (kind IN ('tool_call','status','subagent','workflow','background_job'))` |
| `name` | `text` | no | | for example tool name |
| `title` | `text` | no | `''` | short description for the line |
| `status` | `text` | no | `'running'` | `CHECK (status IN ('pending','running','completed','failed','cancelled'))` |
| `detail` | `jsonb` | no | `'{}'` | schema `run_step_detail.v1`; shortened to `run.step_detail_max_bytes` |
| `started_at` | `timestamptz` | no | `now()` | |
| `finished_at` | `timestamptz` | yes | | |

PK: `id`. FK: `run_id` -> `runs(id)` ON DELETE CASCADE; `parent_step_id` -> `run_steps(id)` ON
DELETE CASCADE. Unique: `(run_id, ordinal)`. Indexes: via the unique index (loading in
order); `(run_id) WHERE status = 'running'` for open steps. Retention: `content`.
At most `run.max_steps_persisted` steps per run; after that the server combines older
finished steps into one status step.

#### `run_followups`

Purpose: follow-ups to a running run and their delivery state. Module: `runs`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `run_id` | `uuid` | no | | |
| `message_id` | `uuid` | no | | |
| `state` | `text` | no | `'queued'` | `CHECK (state IN ('queued','delivered','absorbed','deferred','discarded'))`; `deferred` = becomes the next run |
| `created_at` | `timestamptz` | no | `now()` | |
| `updated_at` | `timestamptz` | no | `now()` | |

PK: `(run_id, message_id)`. FK: `run_id` -> `runs(id)` ON DELETE CASCADE; `message_id` ->
`messages(id)` ON DELETE CASCADE. Unique: `(message_id)` (a follow-up belongs to one run).
Indexes: `(run_id, state)` for open follow-ups. Retention: `content`.

#### `message_components`

Purpose: interactive elements on a message with state and single-use rule. Module: `runs`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | |
| `message_id` | `uuid` | no | | |
| `custom_id` | `text` | no | | assigned by the agent, comes back in the interaction |
| `type` | `text` | no | | `CHECK (type IN ('button','select','form'))` |
| `definition` | `jsonb` | no | | schema `component.v1` ([09-agents.md](09-agents.md), section 11.1): label, style, options, form fields. Button style only a role of the design system (`primary`, `secondary`, `success`, `danger`, `ghost`), no free color |
| `position` | `smallint` | no | | order |
| `single_use` | `boolean` | no | `true` | all `single_use` components of a message form a group with exactly one decision |
| `required_capability` | `text` | yes | | for example `approve_actions` |
| `restricted_to` | `text` | no | `'any'` | `CHECK (restricted_to IN ('any','run_starter','owner'))`; `run_starter` = only the person who triggered the run, or the owner; `owner` = only the owner |
| `run_id` | `uuid` | yes | | run to which the question belongs |
| `state` | `text` | no | `'active'` | `CHECK (state IN ('active','used','expired','disabled'))` |
| `expires_at` | `timestamptz` | yes | | default `component.default_ttl` |
| `used_by` | `uuid` | yes | | |
| `used_at` | `timestamptz` | yes | | |
| `version` | `bigint` | no | `1` | |

PK: `id`. FK: `message_id` -> `messages(id)` ON DELETE CASCADE; `run_id` -> `runs(id)` ON
DELETE SET NULL; `used_by` -> `principals(id)` ON DELETE RESTRICT. Unique: `(message_id,
custom_id)`. Indexes: `(expires_at) WHERE state = 'active'` for the expiry job; `(run_id) WHERE
state = 'active'` for disabling at the end of the run. Retention: `content`.
Single-use rule per message: in one transaction `SELECT ... FROM message_components WHERE
message_id = $m AND single_use AND state = 'active' ORDER BY id FOR UPDATE`. If the chosen
component is among them and not expired, it becomes `used` (`used_by`, `used_at`,
`version + 1`), and all other locked rows become `disabled`. Otherwise `409 component_used`.
Only whoever finds the chosen component active has triggered the interaction. Components
without `single_use` stay active.

#### `interactions`

Purpose: one use of a component by a principal. Module: `runs`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | |
| `component_id` | `uuid` | no | | |
| `message_id` | `uuid` | no | | |
| `principal_id` | `uuid` | no | | |
| `client_interaction_id` | `uuid` | no | | UUIDv7 from the client, idempotency |
| `values` | `jsonb` | no | `'{}'` | selection or form values, schema from the component |
| `state` | `text` | no | `'received'` | `CHECK (state IN ('received','dispatched','acknowledged','failed','rejected'))` |
| `response` | `jsonb` | yes | | response of the agent (for example a note text) |
| `created_at` | `timestamptz` | no | `now()` | |
| `updated_at` | `timestamptz` | no | `now()` | |

PK: `id`. FK: `component_id` -> `message_components(id)` ON DELETE CASCADE; `message_id` ->
`messages(id)` ON DELETE CASCADE; `principal_id` -> `principals(id)` ON DELETE RESTRICT.
Unique: `(principal_id, client_interaction_id)`. Indexes: `(component_id)`; `(state) WHERE
state IN ('received','dispatched')` for delivery. Retention: `content`.

### 4.9 Costs (`billing`)

#### `price_table`

Purpose: prices per provider and model with source and retrieval time. Module: `billing`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | |
| `provider` | `text` | no | | |
| `model` | `text` | no | | |
| `unit` | `text` | no | | `CHECK (unit IN ('input_mtok','output_mtok','cache_read_mtok','cache_write_mtok','cache_write_1h_mtok','request'))`; `cache_write_mtok` is a cache write with 5 minutes, `cache_write_1h_mtok` with 1 hour |
| `price_micros` | `bigint` | no | | per unit |
| `currency` | `text` | no | `'USD'` | |
| `valid_from` | `timestamptz` | no | | |
| `valid_to` | `timestamptz` | yes | | |
| `source_url` | `text` | no | | public pricing page |
| `retrieved_at` | `timestamptz` | no | | time of retrieval |
| `created_by` | `uuid` | no | | |
| `created_at` | `timestamptz` | no | `now()` | |

PK: `id`. FK: `created_by` -> `principals(id)` ON DELETE RESTRICT.
Unique: `(provider, model, unit, valid_from)`; in addition the exclusion constraint
`EXCLUDE USING gist (provider WITH =, model WITH =, unit WITH =, tstzrange(valid_from,
valid_to) WITH &&)` (extension `btree_gist`) against overlapping validity.
Indexes: `(provider, model, unit, valid_from DESC)` for "price at time t".
Retention: `cost`.
A row is no longer changed after its first use in a cost entry; a correction
closes it with `valid_to` and creates a new one. This makes every `computed` entry recomputable from
`provider`, `model` and `occurred_at` (R-DM-23).

#### `cost_entries`

Purpose: cost entry per run, as an estimate or reconciled. Module: `billing`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | |
| `run_id` | `uuid` | no | | |
| `agent_id` | `uuid` | no | | copy for analysis |
| `thread_id` | `uuid` | no | | copy |
| `channel_id` | `uuid` | no | | copy |
| `space_id` | `uuid` | yes | | copy; NULL for a DM |
| `agent_session_id` | `uuid` | yes | | |
| `billing_mode` | `text` | no | | `CHECK (billing_mode IN ('personal','organization'))`; reported mode of the run |
| `provider` | `text` | no | | |
| `model` | `text` | no | | |
| `tokens_in` | `bigint` | no | `0` | |
| `tokens_out` | `bigint` | no | `0` | |
| `tokens_cache_read` | `bigint` | no | `0` | |
| `tokens_cache_write` | `bigint` | no | `0` | cache write with 5 minutes (or without a stated duration) |
| `tokens_cache_write_1h` | `bigint` | no | `0` | cache write with 1 hour, counted separately |
| `amount_micros` | `bigint` | no | | `0` for `unpriced` |
| `currency` | `text` | no | `'USD'` | |
| `is_estimate` | `boolean` | no | `true` | |
| `source` | `text` | no | | `CHECK (source IN ('adapter_reported','computed','unpriced','reconciled'))`; `unpriced` = neither a price nor an amount from the adapter, display "Price missing" |
| `occurred_at` | `timestamptz` | no | | for `computed` also the point in time whose price rows apply |
| `reconciled_at` | `timestamptz` | yes | | |
| `supersedes_id` | `uuid` | yes | | a correction entry replaces an earlier one |

PK: `id`. FK: `run_id` -> `runs(id)` ON DELETE RESTRICT; `agent_id` -> `agents(principal_id)`
ON DELETE RESTRICT; `thread_id`, `channel_id`, `space_id`, `agent_session_id` -> the respective
table ON DELETE RESTRICT; `supersedes_id` -> `cost_entries(id)` ON DELETE RESTRICT.
A calculation uses up to five price rows (input, output, cache read, cache write
with 5 minutes and with 1 hour); therefore the entry does not reference a single row, but
the price set for `provider`, `model` and `occurred_at` (see `price_table`).
Unique: partial index `(supersedes_id) WHERE supersedes_id IS NOT NULL`.
Indexes: `(space_id, occurred_at)`, `(agent_id, occurred_at)`, `(thread_id)`, `(run_id)` for
costs per space, agent, thread and run; `(is_estimate, occurred_at) WHERE is_estimate` for
reconciliation. Retention: `cost`.
Valid total = entries without a successor (no other entry points to them via `supersedes_id`).
With `billing_mode = 'personal'` amounts are information only, never an invoice.

### 4.10 Sync and personal data (`sync`, `notify`)

#### `event_log`

Purpose: durable events with a global, commit-ordered sequence (B8). Module: `sync`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `seq` | `bigint` | no | | from `sequence_counters('event_seq')`, gapless |
| `id` | `uuid` | no | | UUIDv7, dedupe key for clients |
| `type` | `text` | no | | event type from [05-actions.md](05-actions.md) |
| `schema_version` | `smallint` | no | `1` | version of the payload of this type |
| `occurred_at` | `timestamptz` | no | `now()` | time of the transaction |
| `actor_kind` | `text` | yes | | `CHECK (actor_kind IN ('user','agent','app','system'))`; envelope `actor.kind`. NULL: no actor (job without a principal) |
| `actor_principal_id` | `uuid` | yes | | set exactly when `actor_kind` is `user`, `agent` or `app`; NULL for `system` (system action without a principal, for example `grant.expire`) and without an actor |
| `space_id` | `uuid` | yes | | scope for the filter |
| `channel_id` | `uuid` | yes | | |
| `thread_id` | `uuid` | yes | | |
| `resource_kind` | `text` | no | | for example `message`, `thread`, `run` |
| `resource_id` | `uuid` | yes | | |
| `resource_version` | `bigint` | yes | | version of the object after the change |
| `audience_principal_id` | `uuid` | yes | | set: personal event, only for this principal |
| `receive_action` | `text` | no | | action that a receiver needs on the resource ([05-actions.md](05-actions.md), section 6). Personal events: `self` when 05 names only "self"; if 05 also names an action, exactly that action stands here (for example `agent.events.consume` for `agent.invocation`, `interaction.created` and `command.invoked`, `agent.config.read` for `agent.config_changed`). If the receiver is an app, delivery checks the scope `events:subscribe` instead of `agent.events.consume` (05, section 6). One value per type; the source is the action catalogue ([ADR 0049](../adr/0049-machine-readable-action-catalogue-as-the-single-source.md)) |
| `payload` | `jsonb` | no | | schema `<type>.v<schema_version>`; size up to `sync.event_payload_max_bytes` |
| `truncated` | `boolean` | no | `false` | envelope `truncated`: the producer built a reduced payload because the object exceeds `sync.event_payload_max_bytes`; the client reloads via REST |
| `redacted_at` | `timestamptz` | yes | | payload redacted (deletion) |
| `correlation_id` | `uuid` | yes | | W3C trace ID of the triggering request (16 bytes, written as a UUID); if a request has no `traceparent`, the server creates one. NULL only for jobs without a request |
| `trace_context` | `text` | yes | | W3C `traceparent` of the triggering request, only when it was recorded; goes to adapters so that their span becomes a child of the request ([17-observability.md](17-observability.md)) |
| `client_msg_id` | `uuid` | yes | | echo for the sender |
| `permission_version` | `bigint` | no | | permission version at the time of the commit |

PK: `seq`. Partitioning: `PARTITION BY RANGE (seq)`, per partition
`sync.event_log_partition_size` numbers (R-DM-15), no default partition. FK: none (deliberately;
the log outlives deleted objects and may detach partitions freely). Unique: `(id, seq)`
(must contain the partition key). Check: `COALESCE(actor_kind IN ('user','agent','app'), false) =
(actor_principal_id IS NOT NULL)` as a table check, so that a principal never stands without a
kind and `system` never stands with a principal (R-DM-29); `receive_action <> 'self' OR
audience_principal_id IS NOT NULL`. The appender rejects an oversized payload; only the producer
can shorten it, because JSON cannot be shortened in general.
Indexes:

- `(channel_id, seq)` for catch-up filtered by channels.
- `(space_id, seq)` for catch-up filtered by fully visible spaces
  ([04-sync-protocol.md](04-sync-protocol.md), section 7.2).
- `(audience_principal_id, seq) WHERE audience_principal_id IS NOT NULL` for personal
  events.
- `(resource_kind, resource_id)` for redaction on deletion.
- BRIN `(occurred_at)` for retention per partition.

Retention: `event_log` (window `sync.event_retention`). A partition is detached and
dropped when its largest `occurred_at` is older than the window, except the partition with the
current head (`max(seq)`): it always stays. Reason: otherwise, on an idle instance,
`max(seq)` would no longer be the counter value, and gap detection and `min_retained_seq`
([04-sync-protocol.md](04-sync-protocol.md), section 7.3) would break. The job
`sync.partition_maintenance` creates partitions in advance
([02-architecture.md](02-architecture.md), section 12.3); `bothy migrate up` calls the same
idempotent routine, so that a fresh database has partitions before the first `serve`. Because
`bothy_app` may only run DML, creating, attaching and detaching run through two
`SECURITY DEFINER` functions of `bothy_owner` ([16-operations.md](16-operations.md), section 7.1).
Ephemeral events (typing, live box patches, presence, notifications, autocomplete
requests) never stand in this table. Delivery fields (invocation token,
`redelivered`) are created per delivery and are never stored.

#### `event_consumer_cursors`

Purpose: durable read position of the consumers in the `worker` that create external effects
from the event log (push, outgoing webhooks, deletion journal; [02-architecture.md](02-architecture.md),
section 11.5). Module: `sync`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `name` | `text` | no | | `CHECK (name IN ('push','webhooks','privacy.journal'))`; new consumers are an additive migration (`privacy.journal`: M7-07, deletion journal of [20-privacy.md](20-privacy.md), section 4.6) |
| `last_seq` | `bigint` | no | `0` | highest processed global sequence |
| `updated_at` | `timestamptz` | no | `now()` | |

PK: `name`. FK: none. Indexes: via the PK. Retention: `config`.
A round locks the row with `FOR UPDATE SKIP LOCKED`, reads from `last_seq`, creates the jobs with
`InsertTx` and advances `last_seq` in the same transaction. This way jobs are created exactly once,
even with more than one `worker`.

#### `idempotency_keys`

Purpose: idempotency for writing requests without their own client key (everything except
messages and interactions). Module: `sync`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `principal_id` | `uuid` | no | | |
| `key` | `uuid` | no | | header `Idempotency-Key`, UUIDv7 |
| `action` | `text` | no | | `x-authz-action` of the route |
| `request_sha256` | `bytea` | no | | hash of method, path and canonical body |
| `state` | `text` | no | `'in_progress'` | `CHECK (state IN ('in_progress','completed'))` |
| `response_status` | `smallint` | yes | | |
| `response_body` | `jsonb` | yes | | |
| `resource_id` | `uuid` | yes | | |
| `created_at` | `timestamptz` | no | `now()` | |
| `expires_at` | `timestamptz` | no | | `created_at + sync.idempotency_key_ttl` |

PK: `(principal_id, key)`. FK: `principal_id` -> `principals(id)` ON DELETE CASCADE.
Indexes: `(expires_at)` for the cleanup job. Retention: `transient`.
The same key with a different `request_sha256` yields `422 idempotency_key_reuse`.
`in_progress` on a retry yields `409 request_in_progress`.

#### `read_states`

Purpose: read state per person and thread. Module: `sync`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `user_id` | `uuid` | no | | |
| `thread_id` | `uuid` | no | | |
| `channel_id` | `uuid` | no | | copy for counters per channel, updated along with a move |
| `last_read_seq` | `bigint` | no | `0` | highest read `thread_seq` |
| `version` | `bigint` | no | `1` | |
| `updated_by_device_id` | `uuid` | yes | | |
| `updated_at` | `timestamptz` | no | `now()` | |

PK: `(user_id, thread_id)`. FK: `user_id` -> `users(principal_id)` ON DELETE CASCADE;
`thread_id` -> `threads(id)` ON DELETE CASCADE; `channel_id` -> `channels(id)` ON DELETE
CASCADE; `updated_by_device_id` -> `devices(id)` ON DELETE SET NULL.
Indexes: `(user_id, channel_id)` for unread counters per channel.
Retention: `personal_sync`.
Unread = `threads.last_message_seq - GREATEST(coalesce(last_read_seq, 0),
threads.unread_floor_seq)`, only for visible threads. Unread mentions = rows in
`mentions` of the person whose message is not deleted and whose `thread_seq` lies above
`GREATEST(last_read_seq, threads.unread_floor_seq)`. The reader computes both counters
(snapshot, `listMyReadStates`, event `read_state.updated`, badge); neither is stored
(R-DM-32). Only the person themselves writes
(`read_state.update`, `read_state.mark_unread`). So that imported content does not
appear as unread, the import sets `threads.unread_floor_seq` instead of creating rows for all
readers; this costs one row per batch instead of one per person.

#### `drafts`

Purpose: a person's draft per thread or per channel (new forum post), versioned.
Module: `sync`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | |
| `user_id` | `uuid` | no | | |
| `thread_id` | `uuid` | yes | | NULL for a new forum post |
| `channel_id` | `uuid` | no | | with `thread_id` a copy of `threads.channel_id`, updated along with a move |
| `content_md` | `text` | no | `''` | |
| `reply_to_message_id` | `uuid` | yes | | |
| `file_ids` | `uuid[]` | no | `'{}'` | uploaded attachments that are not sent yet |
| `version` | `bigint` | no | `1` | |
| `updated_by_device_id` | `uuid` | yes | | |
| `updated_at` | `timestamptz` | no | `now()` | |

PK: `id`. FK: `user_id` -> `users(principal_id)` ON DELETE CASCADE; `thread_id` ->
`threads(id)` ON DELETE CASCADE; `channel_id` -> `channels(id)` ON DELETE CASCADE;
`reply_to_message_id` -> `messages(id)` ON DELETE SET NULL; `updated_by_device_id` ->
`devices(id)` ON DELETE SET NULL. Unique: partial index `(user_id, thread_id) WHERE thread_id IS
NOT NULL`; partial index `(user_id, channel_id) WHERE thread_id IS NULL`. Indexes: covered by the
unique indexes. Retention: `personal_sync`. An empty draft is deleted.

#### `user_settings`

Purpose: synchronized personal settings, versioned per key. Module: `sync`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `user_id` | `uuid` | no | | |
| `key` | `text` | no | | for example `appearance.theme`, `composer.enter_sends`; keys, values and defaults are normative in [01-product.md](01-product.md), section 11, the keys `notify.*` in [13-push-notifications.md](13-push-notifications.md), section 3.3; the server rejects unknown keys |
| `value` | `jsonb` | no | | |
| `version` | `bigint` | no | `1` | |
| `updated_by_device_id` | `uuid` | yes | | |
| `updated_at` | `timestamptz` | no | `now()` | |

PK: `(user_id, key)`. FK: `user_id` -> `users(principal_id)` ON DELETE CASCADE;
`updated_by_device_id` -> `devices(id)` ON DELETE SET NULL. Indexes: covered by the PK.
Retention: `personal_sync`.

#### `notification_preferences`

Purpose: notification level per space, channel or thread. Module: `notify`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `user_id` | `uuid` | no | | |
| `target_kind` | `text` | no | | `CHECK (target_kind IN ('space','channel','thread'))` |
| `target_id` | `uuid` | no | | polymorphic; checked in the application |
| `level` | `text` | no | | `CHECK (level IN ('all','mentions','mute'))` |
| `muted_until` | `timestamptz` | yes | | temporarily muted |
| `version` | `bigint` | no | `1` | |
| `updated_at` | `timestamptz` | no | `now()` | |

PK: `(user_id, target_kind, target_id)`. FK: `user_id` -> `users(principal_id)` ON DELETE
CASCADE; target polymorphic without FK (R-DM-16). Indexes: `(target_kind, target_id)` for
finding the recipients of a push. Retention: `personal_sync`.
Effective level, first existing value: own setting for thread, channel, space; then
`channels.default_notification_level`; for DM channels `all`; then
`user_settings['notify.default_level']`; then `notify.user_default_level`
([13-push-notifications.md](13-push-notifications.md), section 3.1).

#### `quiet_hours`

Purpose: quiet hours of a person. Module: `notify`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | |
| `user_id` | `uuid` | no | | |
| `weekdays` | `smallint` | no | | bit mask, bit 0 = Monday |
| `start_local` | `time` | no | | |
| `end_local` | `time` | no | | may be less than `start_local` (across midnight) |
| `time_zone` | `text` | yes | | NULL = `users.time_zone` |
| `enabled` | `boolean` | no | `true` | |
| `version` | `bigint` | no | `1` | |

PK: `id`. FK: `user_id` -> `users(principal_id)` ON DELETE CASCADE. Checks: `weekdays BETWEEN 1
AND 127`. Indexes: `(user_id) WHERE enabled`. Retention: `personal_sync`.

#### `push_registrations`

Purpose: push target of a device. Push carries no content. Module: `notify`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | |
| `user_id` | `uuid` | no | | |
| `device_id` | `uuid` | no | | |
| `auth_session_id` | `uuid` | no | | session that registered; logging out deactivates the registration |
| `provider` | `text` | no | | `CHECK (provider IN ('apns','fcm','unifiedpush','wns'))` |
| `token` | `text` | no | | push token or endpoint of the provider |
| `environment` | `text` | no | `'production'` | `CHECK (environment IN ('production','sandbox'))` |
| `app_id` | `text` | no | | bundle or package ID |
| `preview_token_hash` | `bytea` | yes | | HMAC of the preview token of the Notification Service Extension; rotated with every new registration ([13-push-notifications.md](13-push-notifications.md), section 6.4) |
| `created_at` | `timestamptz` | no | `now()` | |
| `last_success_at` | `timestamptz` | yes | | |
| `failure_count` | `integer` | no | `0` | deactivated from `push.max_consecutive_failures` on |
| `disabled_at` | `timestamptz` | yes | | |
| `disabled_reason` | `text` | yes | | `CHECK (disabled_reason IN ('token_invalid','device_revoked','logout','topic_mismatch','sender_mismatch','repeated_failures'))` |

PK: `id`. FK: `user_id` -> `users(principal_id)` ON DELETE CASCADE; `device_id` ->
`devices(id)` ON DELETE CASCADE; `auth_session_id` -> `auth_sessions(id)` ON DELETE CASCADE.
Checks: `(disabled_at IS NULL) = (disabled_reason IS NULL)`. Unique: `(provider, token)`;
`preview_token_hash` as a partial index `WHERE preview_token_hash IS NOT NULL`. Indexes: `(user_id)
WHERE disabled_at IS NULL` for sending; `(auth_session_id)`. Retention: `security`.

### 4.11 Integrations (`apps`, `importer`)

#### `webhook_subscriptions`

Purpose: outgoing webhooks for subscribed event types, bound to an app installation or to an
agent (webhook as the third agent transport, [09-agents.md](09-agents.md),
section 4.2). Module: `apps`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | |
| `app_installation_id` | `uuid` | yes | | set for subscriptions of an app |
| `agent_id` | `uuid` | yes | | set for subscriptions of an agent; every delivery carries a fresh invocation token, `2xx` counts as an ack up to this `seq` |
| `url` | `text` | no | | `https` only; the host must be listed in `webhook.allowed_hosts` (R-DM-17) |
| `event_types` | `text[]` | no | | for apps a subset of `app_installations.event_subscriptions`; for agents the personal events from [04-sync-protocol.md](04-sync-protocol.md), section 15.1, and the events of its grants |
| `signing_key_kid` | `text` | no | | server key with purpose `webhook_out` (signature `v1a`, Ed25519) |
| `hmac_enabled` | `boolean` | no | `false` | additional signature `v1` (HMAC); the secret is not stored but derived from a master key, `id` and `hmac_generation` ([../api/webhooks.md](../api/webhooks.md)) |
| `hmac_generation` | `integer` | no | `0` | increases with every rotation of the HMAC secret |
| `hmac_rotated_at` | `timestamptz` | yes | | until `webhook.secret_rotation_overlap` after this time the server signs with the old and the new generation |
| `state` | `text` | no | `'active'` | `CHECK (state IN ('active','paused','disabled'))` |
| `consecutive_failures` | `integer` | no | `0` | `disabled` from `webhook.max_consecutive_failures` on |
| `last_delivered_seq` | `bigint` | no | `0` | highest global sequence accepted with `2xx`; the next event goes out only after it (strictly ordered per subscription, [11-extensions.md](11-extensions.md), section 7); set to `head_seq` after a restore (section 6.7) |
| `created_by` | `uuid` | no | | |
| `created_at` | `timestamptz` | no | `now()` | |
| `disabled_at` | `timestamptz` | yes | | |

PK: `id`. FK: `app_installation_id` -> `app_installations(id)` ON DELETE CASCADE; `agent_id` ->
`agents(principal_id)` ON DELETE CASCADE; `signing_key_kid` -> `server_signing_keys(kid)` ON
DELETE RESTRICT; `created_by` -> `principals(id)` ON DELETE RESTRICT.
Checks: exactly one of `app_installation_id` and `agent_id` is set; `hmac_enabled OR
hmac_generation = 0`. Indexes: `(state)` for the delivery worker; `(agent_id) WHERE agent_id IS NOT
NULL`. Retention: `config`.

#### `webhook_deliveries`

Purpose: delivery attempts per event and subscription. Module: `apps`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | |
| `subscription_id` | `uuid` | no | | |
| `event_seq` | `bigint` | no | | |
| `event_id` | `uuid` | no | | in the header, dedupe at the receiver |
| `attempt` | `integer` | no | `0` | |
| `status` | `text` | no | `'pending'` | `CHECK (status IN ('pending','succeeded','failed','dead'))` |
| `response_code` | `smallint` | yes | | |
| `last_error` | `text` | yes | | without the response body |
| `next_attempt_at` | `timestamptz` | yes | | backoff from `webhook.retry_backoff_base` and `webhook.retry_backoff_cap` |
| `created_at` | `timestamptz` | no | `now()` | |
| `delivered_at` | `timestamptz` | yes | | |

PK: `id`. FK: `subscription_id` -> `webhook_subscriptions(id)` ON DELETE CASCADE.
Unique: `(subscription_id, event_seq)`. Indexes: `(next_attempt_at) WHERE status = 'pending'`
for the worker. Retention: `transient`.

#### `incoming_webhooks`

Purpose: incoming URL through which an external system writes into a channel as an app.
Module: `apps`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | in the URL |
| `app_id` | `uuid` | no | | writing principal; needs `write` on the channel |
| `channel_id` | `uuid` | no | | |
| `thread_id` | `uuid` | yes | | fixed thread, otherwise a new thread or the main thread |
| `name` | `text` | no | | |
| `token_hash` | `bytea` | no | | HMAC-SHA-256 |
| `created_by` | `uuid` | no | | |
| `created_at` | `timestamptz` | no | `now()` | |
| `last_used_at` | `timestamptz` | yes | | |
| `revoked_at` | `timestamptz` | yes | | |

PK: `id`. FK: `app_id` -> `apps(principal_id)` ON DELETE RESTRICT; `channel_id` ->
`channels(id)` ON DELETE RESTRICT; `thread_id` -> `threads(id)` ON DELETE RESTRICT;
`created_by` -> `principals(id)` ON DELETE RESTRICT. Unique: `token_hash`. Indexes:
`(channel_id) WHERE revoked_at IS NULL`. Retention: `config`.
Messages from incoming webhooks have `origin = 'app'` and never trigger agents.

#### `import_sources`

Purpose: an import source (for example a Discord server) with its state. Module: `importer`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | |
| `kind` | `text` | no | | `CHECK (kind IN ('discord','archive'))` |
| `name` | `text` | no | | |
| `config` | `jsonb` | no | | schema `import_source.<kind>.v1`; credentials only as `secret_ref` |
| `state` | `text` | no | `'draft'` | `CHECK (state IN ('draft','running','paused','completed','failed'))`; a dry run writes nothing and does not change the state |
| `cursor` | `jsonb` | no | `'{}'` | progress per source channel |
| `stats` | `jsonb` | no | `'{}'` | counters, without content |
| `created_by` | `uuid` | no | | |
| `created_at` | `timestamptz` | no | `now()` | |
| `started_at` | `timestamptz` | yes | | |
| `finished_at` | `timestamptz` | yes | | |

PK: `id`. FK: `created_by` -> `principals(id)` ON DELETE RESTRICT. Indexes: `(state)`.
Retention: `import`.

#### `import_items`

Purpose: an imported item with a unique origin key. Module: `importer`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | |
| `source_id` | `uuid` | no | | |
| `origin_key` | `text` | no | | for example `message:<source-id>:<part>`; stable across runs |
| `item_kind` | `text` | no | | `CHECK (item_kind IN ('space','channel','forum_tag','thread','message','file','user','reaction','pin'))` |
| `target_kind` | `text` | yes | | |
| `target_id` | `uuid` | yes | | created object |
| `state` | `text` | no | `'pending'` | `CHECK (state IN ('pending','posted','skipped','failed'))` |
| `content_sha256` | `bytea` | yes | | detects changes at the source |
| `error` | `text` | yes | | without content or secrets |
| `created_at` | `timestamptz` | no | `now()` | |
| `updated_at` | `timestamptz` | no | `now()` | |

PK: `id`. FK: `source_id` -> `import_sources(id)` ON DELETE CASCADE.
Unique: `(source_id, origin_key)`. Indexes: `(source_id, state)` for resuming;
`(target_kind, target_id)` for "where does this object come from". Retention: `import`.
`pending` is written before the object is created, `posted` in the same transaction as the target
object. This makes a restart without duplicates possible.

What the import sets differently from normal writes (only through `import.apply`, only in
channels of the running source; [22-import-migration.md](22-import-migration.md)):

| Column | Value on import |
|---|---|
| `messages.origin`, `messages.silent` | `import`, `true`; no `trigger_state`, no mentions |
| `messages.created_at`, `threads.created_at`, `threads.last_activity_at` | times from the source |
| `threads.operator_agent_id` | planned operator from the owner's mapping, not the effective operator of the (read-only) channel |
| `agent_sessions` | earlier references with `is_current = false`, `started = true`, `end_reason = 'import'`, optionally `cost_baseline_micros` |
| `threads.unread_floor_seq` | end of the batch; imported content counts as unread for nobody |

### 4.12 Audit, privacy, status (`audit`, `privacy`, `status`)

#### `audit_events`

Purpose: append-only, hash-chained audit log (B11). Module: `audit`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `seq` | `bigint` | no | | from `sequence_counters('audit_seq')`, gapless |
| `id` | `uuid` | no | | |
| `occurred_at` | `timestamptz` | no | `now()` | |
| `actor_principal_id` | `uuid` | yes | | NULL for the system and for a server command |
| `actor_device_id` | `uuid` | yes | | |
| `actor_session_id` | `uuid` | yes | | |
| `action` | `text` | no | | action ID from [05-actions.md](05-actions.md) |
| `decision` | `text` | no | | `CHECK (decision IN ('allow','deny','error'))` |
| `deny_reason` | `text` | yes | | reason code from the closed list in [07-permissions.md](07-permissions.md), section 4.1, for example `no_grant`, `step_up_required`, `i1_violation`, `billing_mode_mismatch`, `malware_detected` |
| `resource_kind` | `text` | yes | | |
| `resource_id` | `uuid` | yes | | |
| `details` | `jsonb` | no | `'{}'` | schema `audit_details.v1` ([19-security.md](19-security.md), SEC-AUD-5); never content, never secrets; the only free text is the required reason of an administrative action (`reason`); an IP only as `ip_hmac` with `ip_key_id` (the key changes every `privacy.audit_ip_key_period`, [20-privacy.md](20-privacy.md)) |
| `correlation_id` | `uuid` | yes | | trace ID of the triggering request, as in `event_log` |
| `prev_hash` | `bytea` | no | | `hash` of the row `seq - 1`; first row 32 zero bytes |
| `hash` | `bytea` | no | | `SHA-256(prev_hash || canonical_json(row without hash))`; `canonical_json` is JCS per RFC 8785 |

PK: `seq`. FK: none (the audit log outlives everything; IDs remain evidence). Unique: `id`; `hash`.
Indexes: `(actor_principal_id, occurred_at DESC)`, `(resource_kind, resource_id)`, `(action,
occurred_at DESC)` for the filterable log view; BRIN `(occurred_at)` for retention.
Retention: `audit`.
Permissions: The application role has only `INSERT` and `SELECT`; `UPDATE` and `DELETE` are
blocked by `REVOKE` and a trigger. Truncation at the end of the period runs through a separate
maintenance role and writes an anchor entry (last deleted `seq` and `hash`) so that the chain
stays verifiable. `deny_reason` deliberately has no `CHECK`: an audit entry must never fail
because of a new code. A test against [07-permissions.md](07-permissions.md) keeps the list closed.

#### `audit_checkpoints`

Purpose: signed checkpoints of the hash chain that are also stored outside the database
(ADR 0021, [19-security.md](19-security.md)). Module: `audit`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `seq` | `bigint` | no | | last covered `audit_events.seq` |
| `hash` | `bytea` | no | | `audit_events.hash` of this row |
| `previous_checkpoint_hash` | `bytea` | yes | | SHA-256 of the canonical payload of the previous checkpoint; NULL for the first one |
| `signature` | `bytea` | no | | the complete compact JWS (UTF-8) over the payload `{instance_id, seq, hash, occurred_at, previous_checkpoint_hash}` per JCS; `occurred_at` is the time of the covered row `seq`; header and format in [19-security.md](19-security.md), section 12 |
| `kid` | `text` | no | | server key with purpose `audit_checkpoint` |
| `created_at` | `timestamptz` | no | `now()` | |
| `exported_at` | `timestamptz` | yes | | stored at `audit.checkpoint_sink` |

PK: `seq`. FK: `kid` -> `server_signing_keys(kid)` ON DELETE RESTRICT. Indexes: `(exported_at)
WHERE exported_at IS NULL` for the export. Retention: `audit`. Permissions as for `audit_events`;
in addition the application role may set `exported_at` once (column grant `UPDATE (exported_at)`
and trigger, M1-50).

#### `audit_ip_keys`

Purpose: one HMAC key per period `privacy.audit_ip_key_period` for the IP pseudonyms in the
audit log (`audit_events.details.ip_hmac`, [20-privacy.md](20-privacy.md) section 4.5).
Module: `audit`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | UUIDv7; appears as `ip_key_id` in `audit_events.details` |
| `valid_from` | `timestamptz` | no | | start of the period, aligned to `privacy.audit_ip_key_period` |
| `valid_until` | `timestamptz` | no | | `valid_from + privacy.audit_ip_key_period` |
| `key_ciphertext` | `bytea` | yes | | 32 random bytes, AEAD-encrypted with the key from `key_ref`; NULL after destruction |
| `key_ref` | `text` | no | | reference to the encryption key in the secret provider (`privacy.audit_ip_kek_ref` at creation time) |
| `created_at` | `timestamptz` | no | `now()` | |
| `destroyed_at` | `timestamptz` | yes | | set as soon as `valid_until + privacy.audit_ip_key_retention` has passed |

PK: `id`. FK: none. Unique: `valid_from`. Checks: `valid_until > valid_from`;
`(key_ciphertext IS NULL) = (destroyed_at IS NOT NULL)`. Indexes: covered by PK and Unique.
Retention: `audit` (the row stays as evidence, only the key is destroyed).
Permissions: The application role may `SELECT`, `INSERT` and `UPDATE` only on `key_ciphertext` and
`destroyed_at`. The server creates the row of a period when it is first needed
(`ON CONFLICT (valid_from) DO NOTHING`, then read again). Like the TOTP secret, the key is
an exception to principle 9: it must be destructible, but the file driver of the secret provider
cannot create keys per period; therefore it is stored encrypted in the database, and the key for it
only in the secret provider.

#### `gdpr_requests`

Purpose: privacy requests (export, deletion, rectification) as jobs. Module: `privacy`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | |
| `subject_user_id` | `uuid` | no | | data subject |
| `kind` | `text` | no | | `CHECK (kind IN ('export','deletion','rectification'))` |
| `state` | `text` | no | `'requested'` | `CHECK (state IN ('requested','approved','running','completed','failed','rejected','cancelled'))` |
| `requested_by` | `uuid` | no | | |
| `approved_by` | `uuid` | yes | | NULL with `approved` means: approved by the system (export of one's own data with `privacy.export_requires_approval = false`) |
| `reason` | `text` | no | | |
| `due_at` | `timestamptz` | no | | statutory deadline per [20-privacy.md](20-privacy.md) |
| `result_file_id` | `uuid` | yes | | export package |
| `result_expires_at` | `timestamptz` | yes | | |
| `progress` | `jsonb` | no | `'{}'` | steps and counters, without content |
| `error` | `text` | yes | | |
| `created_at` | `timestamptz` | no | `now()` | |
| `completed_at` | `timestamptz` | yes | | |

PK: `id`. FK: `subject_user_id` -> `users(principal_id)` ON DELETE RESTRICT; `requested_by`,
`approved_by` -> `principals(id)` ON DELETE RESTRICT; `result_file_id` -> `files(id)` ON
DELETE SET NULL. Unique: partial index `(subject_user_id, kind) WHERE state IN ('requested',
'approved','running')`. Indexes: `(state, due_at)` for the job worker and deadline warnings.
Retention: `privacy_job`.

#### `deletion_journal_state`

Purpose: the numbering of the deletion journal outside the database ([20-privacy.md](20-privacy.md),
section 4.6). The object storage port has no listing, so journal objects carry dense numbers, and
this single row says which number comes next and which is the lowest one not yet removed. Module:
`privacy`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `smallint` | no | `1` | `CHECK (id = 1)`: exactly one row |
| `next_number` | `bigint` | no | `1` | number of the next journal object; moves in the transaction that moves the cursor `privacy.journal` |
| `low_number` | `bigint` | no | `1` | lowest number whose object the retention has not removed yet; the retention removes an object only when it is older than `privacy.deletion_journal_retention` and its number is below the `deletion_journal.next_number` of every backup that is still kept, so a restore of any kept backup finds its first object |
| `updated_at` | `timestamptz` | no | `now()` | |

PK: `id`. FK: none. Indexes: via the PK. Retention: `config`. After a restore, `bothy restore
finalize` reads the journal from the restored `next_number` up to the first missing number and
stores the number after the last object found (R-DM-52).

#### `status_components`

Purpose: components of the status page. Module: `status`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | |
| `name` | `text` | no | | for example "Messages", "Push", "Agent Support" |
| `description` | `text` | no | `''` | |
| `sort_key` | `text COLLATE "C"` | no | | |
| `state` | `text` | no | `'operational'` | `CHECK (state IN ('operational','degraded','partial_outage','major_outage','maintenance'))` |
| `health_source` | `text` | yes | | key of an automatic check; NULL = manual only |
| `agent_id` | `uuid` | yes | | component for an agent (presence) |
| `updated_at` | `timestamptz` | no | `now()` | |

PK: `id`. FK: `agent_id` -> `agents(principal_id)` ON DELETE SET NULL. Unique: `lower(name)`.
Indexes: `(sort_key)`. Retention: `ops`.

#### `status_incidents`

Purpose: incidents and maintenance windows. Module: `status`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | |
| `title` | `text` | no | | |
| `kind` | `text` | no | `'incident'` | `CHECK (kind IN ('incident','maintenance'))` |
| `impact` | `text` | no | | `CHECK (impact IN ('none','minor','major','critical'))` |
| `state` | `text` | no | `'investigating'` | `CHECK (state IN ('scheduled','investigating','identified','monitoring','resolved'))` |
| `started_at` | `timestamptz` | no | | |
| `resolved_at` | `timestamptz` | yes | | |
| `created_by` | `uuid` | yes | | NULL when created automatically |

PK: `id`. FK: `created_by` -> `principals(id)` ON DELETE RESTRICT. Indexes: `(state,
started_at DESC)`. Retention: `ops`.

#### `status_incident_updates`

Purpose: history of an incident. Module: `status`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | |
| `incident_id` | `uuid` | no | | |
| `state` | `text` | no | | as `status_incidents.state` |
| `body_md` | `text` | no | | |
| `created_by` | `uuid` | yes | | |
| `created_at` | `timestamptz` | no | `now()` | |

PK: `id`. FK: `incident_id` -> `status_incidents(id)` ON DELETE CASCADE; `created_by` ->
`principals(id)` ON DELETE RESTRICT. Indexes: `(incident_id, created_at)`. Retention: `ops`.

#### `status_incident_components`

Purpose: affected components of an incident. Module: `status`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `incident_id` | `uuid` | no | | |
| `component_id` | `uuid` | no | | |

PK: `(incident_id, component_id)`. FK: `incident_id` -> `status_incidents(id)` ON DELETE
CASCADE; `component_id` -> `status_components(id)` ON DELETE CASCADE. Indexes:
`(component_id)`. Retention: `ops`.

#### `backup_runs`

Purpose: runs of the backup and of the automatic restore test, with the result per check
([16-operations.md](16-operations.md), section 8). The admin console, the status page and alerts read
them. Module: `status`. Written by the program `bothy-backupd` through the port of the module
`status` with a separate database role that may write only this table.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | |
| `kind` | `text` | no | | `CHECK (kind IN ('pg_full','pg_diff','logical','objects','restore_test'))` |
| `state` | `text` | no | `'running'` | `CHECK (state IN ('running','succeeded','failed'))` |
| `started_at` | `timestamptz` | no | `now()` | |
| `finished_at` | `timestamptz` | yes | | |
| `size_bytes` | `bigint` | yes | | |
| `manifest` | `jsonb` | no | `'{}'` | schema `backup_manifest.v1`: row count per table, `head_seq`, last audit `seq` and `hash`, schema and server version, `sync_epoch`, count and sum of the file sizes; never content |
| `checks` | `jsonb` | no | `'{}'` | schema `restore_test_checks.v1`: result and duration per check (only for `restore_test`) |
| `error` | `text` | yes | | code without content or secrets |

PK: `id`. FK: none. Checks: `(state = 'running') = (finished_at IS NULL)`. Indexes: `(kind,
started_at DESC)` for "last successful run per kind". Retention: `ops`.

### 4.13 Technical tables

These tables belong to no domain module; the respective tool defines their schema.

| Tables | Owner | Schema | Retention | Reason |
|---|---|---|---|---|
| River (`river_migration`, `river_job`, `river_leader`, `river_queue` and others) | `internal/platform/jobs` | own schema `jobs.schema`, migrated with `rivermigrate` | `transient` | job arguments contain only IDs ([02-architecture.md](02-architecture.md), section 12) |
| Version table of goose | `bothy migrate` | schema of the domain tables | `config` | state of the migrations; `instance.schema_version` shows it |

Domain modules never read or write these tables directly.

### 4.14 Terminal (`terminal`)

Normative together with [24-terminal.md](24-terminal.md). Milestone M2.

#### `hosts`

Purpose: a connected host with the host service, its public keys, its declaration of the
LLM login (I6) and its reported state. Module: `terminal`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | UUIDv7; host ID in the prologue and in `aud` of the terminal session token |
| `name` | `text` | no | | not empty, at most `host.name_max_chars` characters, without control and bidi characters |
| `state` | `text` | no | `'pairing'` | `CHECK (state IN ('pairing','confirming','active','disabled','retired'))` (24, section 3.1) |
| `platform` | `text` | yes | | `CHECK (platform IN ('macos','linux','windows'))`; reported at `host.pair` |
| `assigned_user_id` | `uuid` | yes | | host assignee (SR10); NULL means owner only |
| `dev_environment_id` | `uuid` | yes | | dev environment that this host is (milestone "Multi-user dev environment") |
| `llm_credentials` | `text` | no | `'none'` | `CHECK (llm_credentials IN ('none','personal','organization'))` (24, section 7.2) |
| `credential_holder_id` | `uuid` | yes | | holder of the login for `personal` (I6) |
| `llm_credentials_declared_by` | `uuid` | yes | | owner who made the last declaration |
| `llm_credentials_declared_at` | `timestamptz` | yes | | |
| `declared_server_host` | `boolean` | no | `false` | the owner's declaration: the server runs on this host (24, section 2.3) |
| `pairing_code_hash` | `bytea` | yes | | HMAC-SHA-256 of the pairing code (`bothy_hpc_...`); set exactly when `state = 'pairing'` |
| `pairing_expires_at` | `timestamptz` | yes | | `terminal.pairing_code_ttl` after issuing |
| `noise_public_key` | `bytea` | yes | | X25519, exactly 32 bytes (`CHECK (octet_length(noise_public_key) = 32)`) |
| `auth_public_jwk` | `jsonb` | yes | | Ed25519 as JWK (`kty = OKP`, `crv = Ed25519`), without the private part |
| `auth_jkt` | `text` | yes | | JWK thumbprint (RFC 7638) of the authentication key |
| `key_fingerprint` | `text` | yes | | 24, section 4.3 |
| `key_generation` | `integer` | no | `0` | increases by one with every pairing |
| `fingerprint_failures` | `integer` | no | `0` | failed attempts of the current confirmation |
| `paired_at` | `timestamptz` | yes | | last successful `host.pair` |
| `confirmed_by` | `uuid` | yes | | owner who confirmed the fingerprint |
| `confirmed_at` | `timestamptz` | yes | | |
| `reported_profiles` | `jsonb` | no | `'[]'` | schema `host_profiles.v1`: per profile `name`, `display_name`, `kind` (`shell`, `claude`, `command`); never command lines, paths or environment |
| `reported_session_users` | `jsonb` | no | `'[]'` | schema `host_reported_session_users.v1`: per name from `host_service.session_users`, `os_user` and `workdirs`, the display names of its working folders of the profile `claude` from `host_service.claude_workdirs` after HD7 (a folder shared by two session users is missing from both lists), in their order; the host service resolves `wdi` in exactly this list; reported at `host.pair` and with every `report` (24, section 2.2); never paths |
| `reported_doctor` | `jsonb` | no | `'[]'` | schema `host_doctor.v1`: list with result and code per check HD1 to HD11 (24, section 2.2); per account that the rule for names from 24, section 2.2 names, a separate entry with its name (`session_user`): for HD6, HD10 (wrong peer) and HD11 the affected session user, for HD9 every former session user with a readable home, for other users an entry without a name; check and name together are unique; never content, paths or values |
| `reported_is_server_host` | `boolean` | yes | | report of the host service (24, section 2.3) |
| `reported_require_local_confirm` | `boolean` | yes | | report whether `host_service.require_local_confirm` is on |
| `reported_os_version` | `text` | yes | | |
| `reported_host_version` | `text` | yes | | SemVer of the host service |
| `last_seen_at` | `timestamptz` | yes | | coarse, at most once per `identity.last_active_granularity` |
| `created_by` | `uuid` | no | | |
| `created_at` | `timestamptz` | no | `now()` | |
| `disabled_at` | `timestamptz` | yes | | |
| `disabled_reason` | `text` | yes | | `CHECK (disabled_reason IN ('owner','duplicate_connection'))`; the server sets `duplicate_connection` on a second live control connection (24, section 3.4) |
| `retired_at` | `timestamptz` | yes | | |
| `version` | `bigint` | no | `1` | optimistic lock |

PK: `id`. FK: `assigned_user_id`, `credential_holder_id` -> `users(principal_id)` ON DELETE
RESTRICT; `dev_environment_id` -> `dev_environments(id)` ON DELETE RESTRICT;
`llm_credentials_declared_by`, `confirmed_by`, `created_by` -> `principals(id)` ON DELETE RESTRICT.
Checks: `(llm_credentials = 'personal') = (credential_holder_id IS NOT NULL)`;
`(state = 'pairing') = (pairing_code_hash IS NOT NULL)`; `state NOT IN ('confirming','active',
'disabled') OR (noise_public_key IS NOT NULL AND auth_public_jwk IS NOT NULL AND auth_jkt IS NOT
NULL AND key_fingerprint IS NOT NULL)`; `state NOT IN ('active','disabled') OR confirmed_at IS NOT
NULL`; `(state = 'retired') = (retired_at IS NOT NULL)`; `(state = 'disabled') = (disabled_reason IS
NOT NULL)`.
Unique: `auth_jkt` as a partial index `WHERE auth_jkt IS NOT NULL`; `pairing_code_hash` as a partial
index `WHERE pairing_code_hash IS NOT NULL`; partial index `lower(name) WHERE state <> 'retired'`.
Indexes: `(assigned_user_id) WHERE state = 'active'` for SR10; `(credential_holder_id) WHERE
llm_credentials = 'personal'` for I6; `(pairing_expires_at) WHERE state = 'pairing'` for the
cleanup job `terminal.sweep` ([02-architecture.md](02-architecture.md), section 12.3). Retention: `security`.
The server keeps the connection state (connected, disconnected) only in memory; it appears as the
ephemeral event `host.presence_changed` (05).

#### `host_session_users`

Purpose: assignment of a person to their own session user (operating system user) on
a host, ruling T12 ([24-terminal.md](24-terminal.md), section 2.2). A session user
belongs to at most one person, also after the assignment has ended. Module: `terminal`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | UUIDv7 |
| `host_id` | `uuid` | no | | |
| `user_id` | `uuid` | no | | person who owns the session user |
| `os_user` | `text` | no | | name of the session user as in `host_service.session_users`; not empty, at most `host.os_user_max_chars` characters, only letters, digits, `_`, `.` and `-` (`CHECK (os_user ~ '^[A-Za-z0-9_][A-Za-z0-9_.-]*$')`); never a path |
| `created_by` | `uuid` | no | | owner who made the assignment |
| `created_at` | `timestamptz` | no | `now()` | |
| `revoked_by` | `uuid` | yes | | owner who ended the assignment |
| `revoked_at` | `timestamptz` | yes | | end of the assignment; the row stays, the name stays taken for this host |

PK: `id`. FK: `host_id` -> `hosts(id)` ON DELETE RESTRICT; `user_id` -> `users(principal_id)` ON
DELETE RESTRICT; `created_by`, `revoked_by` -> `principals(id)` ON DELETE RESTRICT.
Checks: `(revoked_at IS NULL) = (revoked_by IS NULL)`. A rule of the application, not of the
database, because the report may change later: on creation, `os_user` must be listed in
`hosts.reported_session_users`; otherwise `host.manage` rejects with `400 validation_failed`
(`errors[].code = session_user_not_on_host`, 24 section 2.2).
Unique: `(id, host_id, user_id)` as the target of the composite foreign key of
`terminal_sessions`; `(host_id, lower(os_user))` across all rows, including ended ones (never
shared, not even over time; operating system names are case-insensitive on macOS and Windows);
`(host_id, user_id) WHERE revoked_at IS NULL` (at most one valid assignment per person and
host). A violation of either of the last two yields `409 conflict` with `reason:
session_user_taken`.
Indexes: `(user_id) WHERE revoked_at IS NULL` for ending assignments when the person is disabled.
Retention: `security` (period `retention.security_after_end` after `retired_at` of the host).
Rows are never deleted as long as the host is not `retired`; otherwise the name would become free again.

#### `terminal_sessions`

Purpose: metadata of a terminal session. Never content, never commands. Module: `terminal`.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `id` | `uuid` | no | | UUIDv7; session ID in the prologue and in `sid` of the token |
| `host_id` | `uuid` | no | | |
| `user_id` | `uuid` | no | | person of the session; only people |
| `host_session_user_id` | `uuid` | no | | session user of the person on this host at opening (SR12); source of `osu` in every token of the session |
| `opened_by_device_id` | `uuid` | no | | device at opening |
| `device_id` | `uuid` | no | | device of the last opening or the last reattach |
| `auth_session_id` | `uuid` | no | | login session of the last opening or the last reattach; its revocation ends the session |
| `profile` | `text` | no | | name of the profile from `hosts.reported_profiles` |
| `state` | `text` | no | `'opening'` | `CHECK (state IN ('opening','active','detached','closed'))` (24, section 6.6) |
| `token_jti` | `uuid` | no | | `jti` of the currently valid terminal session token; older tokens are worthless |
| `key_generation` | `integer` | no | | `hosts.key_generation` at opening |
| `opened_at` | `timestamptz` | no | `now()` | start; basis of `terminal.max_session_duration` |
| `started_at` | `timestamptz` | yes | | PTY started (`session.started` of the host service) |
| `detached_at` | `timestamptz` | yes | | last detach; NULL as long as a device is connected |
| `last_input_at` | `timestamptz` | yes | | coarse, from the relay; basis of the server's idle check |
| `closed_at` | `timestamptz` | yes | | |
| `closed_by` | `uuid` | yes | | person for `client_closed` and `closed_by_owner`; otherwise NULL |
| `close_reason` | `text` | yes | | `CHECK (close_reason IN ('client_closed','closed_by_owner','process_exited','idle_timeout','max_duration','detach_expired','access_revoked','i6_violation','host_disabled','host_retired','host_rekeyed','host_restarted','user_disabled','device_revoked','session_revoked','logout','open_failed','host_policy_violation'))` (24, section 10.3) |
| `close_detail` | `text` | yes | | code from the closed list in 24, section 10.3; never free text |
| `exit_code` | `integer` | yes | | exit code of the process, if reported |
| `bytes_in` | `bigint` | no | `0` | bytes of the transport messages from the device, counted in the relay |
| `bytes_out` | `bigint` | no | `0` | bytes of the transport messages to the device, counted in the relay |
| `version` | `bigint` | no | `1` | |

PK: `id`. FK: `host_id` -> `hosts(id)` ON DELETE RESTRICT; `user_id` -> `users(principal_id)` ON
DELETE RESTRICT; `(host_session_user_id, host_id, user_id)` -> `host_session_users(id, host_id,
user_id)` ON DELETE RESTRICT (this way the database holds that the session user belongs to the host
and the person of the session; a bug in the code cannot write an `osu` of another person into a
token);
`opened_by_device_id`, `device_id` -> `devices(id)` ON DELETE RESTRICT;
`auth_session_id` -> `auth_sessions(id)` ON DELETE RESTRICT; `closed_by` -> `principals(id)` ON
DELETE RESTRICT.
Checks: `(state = 'closed') = (closed_at IS NOT NULL)`; `(state = 'closed') = (close_reason IS NOT
NULL)`; `close_detail IS NULL OR state = 'closed'`; `(state = 'detached') = (detached_at IS NOT
NULL)`.
Unique: `token_jti`. Indexes: `(user_id) WHERE state <> 'closed'` for `terminal.max_sessions_per_person`
and for ending sessions on revocation; `(host_id) WHERE state <> 'closed'`; `(auth_session_id) WHERE state <>
'closed'`; `(device_id) WHERE state <> 'closed'`; BRIN `(opened_at)` for retention.
Retention: `security` (period `retention.security_after_end` after `closed_at`).

### 4.15 Statistics (`stats`)

Normative together with [25-visualization.md](25-visualization.md). Milestone M6.

#### `stats_minutes`

Purpose: minute aggregates of the server's own measurement (usage, runs, costs, SLI counters) for
the live view, the timeline and the SLO traffic light ([25-visualization.md](25-visualization.md),
section 3). No people, no content. Module: `stats`. Every row is an aggregate without an identity
of its own; therefore there is no column `id` and no primary key, but a unique key
over all dimensions.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `bucket_start` | `timestamptz` | no | | start of the minute; `CHECK (bucket_start = date_trunc('minute', bucket_start))`; partition key |
| `metric` | `text` | no | | `CHECK (metric IN ('messages_created','threads_created','runs_started','runs_finished','tool_calls','approval_cards','cost','invocations_denied','billing_mode_switches','catchups','resyncs','slow_consumer_closes','sli.api_availability','sli.delivery_latency','sli.reconnect','sli.push_acceptance','sli.run_start','sli.turn_duration'))` (25, section 3.2) |
| `channel_id` | `uuid` | yes | | channel of the measurement; NULL for values of the whole instance |
| `agent_id` | `uuid` | yes | | agent of the measurement, where the metric has one |
| `dimension` | `text` | no | `''` | code from the closed list per metric (25, section 3.2), schema `stats_dimension.v1`; never free text, never tool names |
| `currency` | `text` | yes | | ISO 4217; set exactly when `metric = 'cost'` |
| `count` | `bigint` | no | `0` | number of events |
| `sum` | `bigint` | no | `0` | sum of the values: milliseconds for durations, micro-units for `cost`, otherwise 0 |
| `good` | `bigint` | no | `0` | good events of an SLI; 0 for other metrics |
| `updated_at` | `timestamptz` | no | `now()` | last flush into this row |

PK: none. FK: none (evidence like `event_log`; an archived channel keeps its numbers).
Checks: `count >= 0 AND sum >= 0 AND good >= 0`; `good <= count`; `(currency IS NOT NULL) = (metric
= 'cost')`; `metric LIKE 'sli.%' OR good = 0`.
Unique: `(bucket_start, metric, channel_id, agent_id, dimension, currency) NULLS NOT DISTINCT`
(target of the additive upsert, 25 section 3.4). Indexes: `(metric, bucket_start)` for the timeline
and the traffic light; `(channel_id, bucket_start) WHERE channel_id IS NOT NULL` for the permission
filter.
Partitioning: by `bucket_start` per day (`PARTITION BY RANGE`); the job `stats.retention` creates
partitions in advance and detaches expired ones.
Retention: `stats` (period `retention.stats_minutes`).

### 4.16 Gateway (`gateway`)

#### `gateway_presence`

Purpose: the shared presence state of the counted live connections of humans, so that a `worker`
and other `api` nodes see who is connected where ([13-push-notifications.md](13-push-notifications.md),
section 12 point 4; [02-architecture.md](02-architecture.md), section 16). Module: `gateway`.
`UNLOGGED`: the contents are lost after a crash of the database, and the nodes write them again.

| Field | Type | Null | Default | Rules / notes |
|---|---|---|---|---|
| `gateway_session_id` | `uuid` | no | | gateway session of the connection |
| `node_id` | `uuid` | no | | process instance of the `api` node that holds the connection; new at every start |
| `principal_id` | `uuid` | no | | the human of the connection |
| `device_id` | `uuid` | no | | the device of the connection |
| `focus_thread_ids` | `uuid[]` | no | `'{}'` | threads in focus after the checks of the gateway (op `focus`, [04-sync-protocol.md](04-sync-protocol.md), section 6.2) |
| `idle` | `boolean` | no | `false` | the client reported `idle` |
| `updated_at` | `timestamptz` | no | `now()` | last write or refresh by the node |

PK: `gateway_session_id`. FK: none (a row lives only as long as its connection; it is not
evidence). Indexes: `(principal_id)`; `(node_id)`. Retention: `transient` (the node deletes the row
when the connection ends and all its rows on an orderly shutdown; readers ignore rows whose
`updated_at` is older than `cluster.presence_ttl`, and the gateway deletes them). Only processes in
role `api`, and in role `all` with `cluster.mode = multi`, write the table; a process in role `all`
with `cluster.mode = single` keeps the state in memory (R-DM-51).

## 5. Search vectors

Full-text search uses PostgreSQL `tsvector` as stored, generated columns. This keeps the
vector always consistent with the content, without a trigger and without a job. Every vector
contains stems of the language and unchanged words (`simple`) with their own weight; one GIN query
is enough ([14-search.md](14-search.md), section 3.2).

| Table | Column | Expression |
|---|---|---|
| `messages` | `search_vector` | `GENERATED ALWAYS AS (setweight(to_tsvector(search_language, content_md), 'B') \|\| setweight(to_tsvector('simple'::regconfig, content_md), 'D')) STORED` |
| `threads` | `title_search` | `GENERATED ALWAYS AS (setweight(to_tsvector(search_language, title), 'A') \|\| setweight(to_tsvector('simple'::regconfig, title), 'A')) STORED` |
| `files` | `search_vector` | `GENERATED ALWAYS AS (setweight(to_tsvector('simple', original_name), 'A') \|\| setweight(to_tsvector('simple', coalesce(transcript_text, '')), 'B')) STORED` |
| `file_text_extracts` | `search_vector` | `GENERATED ALWAYS AS (setweight(to_tsvector(search_language, coalesce(text_content, '')), 'C') \|\| setweight(to_tsvector('simple'::regconfig, coalesce(text_content, '')), 'D')) STORED` |

`to_tsvector(regconfig, text)` is `IMMUTABLE` and may therefore appear in generated columns.
PostgreSQL 18 creates generated columns without `STORED` as virtual columns; every migration
writes `STORED` explicitly, and a migration lint checks this.
Every column has a GIN index. Permission filter: every search query is combined with
`channel_id = ANY($visible)`; `authz` builds the list per request (P2, [07-permissions.md](07-permissions.md) section 7.4). Deleted
messages have empty content and therefore an empty vector. Language choice, ranking,
highlighting and possible further indexes (for example trigrams) are defined in [14-search.md](14-search.md);
a change to an expression is a migration that regenerates the column, and it is carried over into
this table.

## 6. Transaction rules

### 6.1 Global event sequence

The exact SQL pattern and the rationale are in
[04-sync-protocol.md](04-sync-protocol.md), section 3. In short: the row
`sequence_counters('event_seq')` is locked and incremented by `UPDATE ...
RETURNING` as the last step before the commit; the events get these numbers. The same applies to
`audit_seq`.

### 6.2 Thread sequence

`UPDATE threads SET last_message_seq = last_message_seq + 1, last_activity_at = now(),
message_count = message_count + 1, version = version + 1 WHERE id = $1 RETURNING
last_message_seq`. The row lock on the thread orders all messages of a thread
strictly. A rolled-back transaction also gives back the number; no gaps arise.

### 6.3 Lock order

Every write transaction takes its locks in this order; otherwise deadlocks can occur:

1. `sequence_counters('permission_version')`, only for permission changes, and then first.
2. Domain rows; for several rows of the same table in ascending `id`.
3. `sequence_counters('event_seq')`.
4. `sequence_counters('audit_seq')`.

Permission changes are therefore fully serialized among each other. This makes the I1 check
(6.4) correct without `SERIALIZABLE`.

### 6.4 I1 check

In every transaction that increments the permission version, `authz` computes, after the change
and before the commit, for every affected active agent (principal status `active`) with
`billing_mode = 'personal'` the set of
principals with effective level `trigger` on any target that the agent serves (including
owners, group members, inheritance, unexpired grants). The targets of agent A are:

1. channels whose effective operator is A (main thread and new threads),
2. channels with at least one thread with `threads.operator_agent_id = A`, open or done,
   even if the channel has another operator by now; a new message reopens a
   done thread, and there A still triggers (R-DM-12),
3. the spaces of these channels and every space whose default operator is A, even without a channel;
   otherwise `channel.create` (not an action with an I1 check) would later make A the operator of a
   channel on whose space someone else already has `trigger`,
4. the instance.

Guest channels (`allow_mentioned_agents`) do not count, because an agent with `personal` is never a
guest agent ([05-actions.md](05-actions.md), section 5). If the set is not exactly
`{credential_holder_id}`, the transaction is rolled back with `409 i1_violation` (decision
`invariant_violation`) and the hint "switch to organization first", and an
audit entry with `decision = 'deny'` is written in a separate transaction. Affected are: all
agents with `personal` whose served targets lie in the path of the changed grant, and, for
owner and group changes, all agents with `personal`. Expiry and revocation only shrink the
set; the check runs anyway, because it is cheap.

Consequence: an agent with `personal` cannot be the operator of a target on which another
principal has `trigger`. Because every owner has `trigger` everywhere, `personal` works only as long
as the `credential_holder` is the only owner. This is intended (Baseline I1, ruling L22): there is
no owner exception. The guided flow "switch first, then grant" in one transaction
is described in [05-actions.md](05-actions.md), rule 1.6 (`resolve_i1`).

### 6.5 Redaction on deletion

Deleting a message or a file writes in the same transaction: a tombstone, an
event `message.deleted`, and `UPDATE event_log SET payload = <redacted payload>,
redacted_at = now() WHERE resource_kind = 'message' AND resource_id = $1`. The envelope fields
stay. Events are therefore append-only in the envelope and redactable in the payload (R-DM-18).
Deleting a file redacts in the same transaction the events with `resource_kind = 'file'` and
`resource_id` = the file, in the shape of [04-sync-protocol.md](04-sync-protocol.md), section 2.2
rule 5 (R-DM-45).

### 6.6 Notifying the gateway processes

Every transaction with new events calls `pg_notify('event_log', <highest seq>)`.
PostgreSQL delivers the notification only after the commit. Every process with the role `api`
then reads `event_log WHERE seq > <own position> ORDER BY seq`. No further broker.

With several api nodes (`cluster.mode = multi`, [02-architecture.md](02-architecture.md),
section 11.6), a second channel `pg_notify('ephemeral', ...)` distributes ephemeral events
between the nodes. It carries only IDs and versions (thread, principal, run, agent, host,
terminal session), never content; the receiving node reads content from the database. With one node the channel
is off, because every `NOTIFY` takes an instance-wide lock at commit.

### 6.7 Sync epoch and restore

Decision: [ADR 0059](../adr/0059-sync-epoch-after-restore.md).

A restore from a backup resets the global sequence. Without a mitigation, the new timeline
overtakes old cursors after the first writes, and a client would silently skip other events
with the same numbers. Therefore:

1. `instance.sync_epoch` names the timeline. After every restore, `bothy restore finalize` sets
   a new UUIDv7, sets `agent_consumer_cursors.acked_seq`,
   `webhook_subscriptions.last_delivered_seq` and `event_consumer_cursors.last_seq` to the new
   `head_seq` and writes an audit entry, all in one transaction.
2. Clients store the epoch together with the cursor. If it differs, the server requires a resync
   ([04-sync-protocol.md](04-sync-protocol.md), section 7.5).
3. A move without data loss ([16-operations.md](16-operations.md), section 11) keeps the epoch.
4. `bothy restore finalize` also stores the identity of the restored database in `instance.pg_system_identifier` and `instance.pg_timeline_id` in the same transaction. A server whose current database identity differs from the stored one does not become ready until `bothy restore finalize` has run, because a restore without it would mix two timelines; a move without data loss stores the new identity without a new epoch (R-DM-50; the readiness reason and the operator switch are in [16-operations.md](16-operations.md), section 8.4).

### 6.8 I6 check

Applies to `grant.create` with `target_kind = 'host'` and to `host.manage` when it sets `llm_credentials`,
`credential_holder_id` or the state of a host ([24-terminal.md](24-terminal.md),
section 7.3). The transaction first locks `permission_version` (6.3), then the row in `hosts`
(`SELECT ... FOR UPDATE`), writes the change and checks before the commit: with
`llm_credentials = 'personal'`, no valid grant on the host has a subject other than
`subject_kind = 'user'` with `subject_principal_id = credential_holder_id`. Otherwise rollback with
`409 i6_violation` (decision `invariant_violation`) and an audit entry `deny` in a separate
transaction. Owners do not count, because I6 removes their implicit authorization on this host.
Group grants are always rejected, because a group can change without a permission check at the
host. On a switch to `personal` or a change of the credential holder, the same transaction ends all
sessions of other people that are not closed (`close_reason = 'i6_violation'`).

## 7. ERD

The ERDs exist only in [../architecture/](../architecture/README.md) as Mermaid sources with SVG,
one diagram per module. They show every table of sections 4.1 to 4.12 and 4.14 to 4.16 exactly
once with its columns and every foreign key relation except pure log columns. The technical
tables of section 4.13 are left out on purpose. The comparison with this chapter is a gate
(`make diagrams`, `tools/docs/erd_check.py`). This chapter stays normative; on a difference,
section 4 applies, and the gate fails.

| Diagram | Modules |
|---|---|
| [erd-1-instance-identity.mmd](../architecture/erd-1-instance-identity.mmd) | `instance`, `identity` |
| [erd-2-permissions-structure.mmd](../architecture/erd-2-permissions-structure.mmd) | `authz`, `chat` |
| [erd-3-agents-apps.mmd](../architecture/erd-3-agents-apps.mmd) | `agents`, `apps` |
| [erd-4-messages-files.mmd](../architecture/erd-4-messages-files.mmd) | `messaging`, `files` |
| [erd-5-runs-costs.mmd](../architecture/erd-5-runs-costs.mmd) | `runs`, `billing` |
| [erd-6-sync.mmd](../architecture/erd-6-sync.mmd) | `sync` |
| [erd-7-terminal.mmd](../architecture/erd-7-terminal.mmd) | `terminal` |
| [erd-8-audit-privacy.mmd](../architecture/erd-8-audit-privacy.mmd) | `audit`, `privacy` |
| [erd-9-import-status-statistics.mmd](../architecture/erd-9-import-status-statistics.mmd) | `importer`, `status`, `stats` |
| [erd-10-notification.mmd](../architecture/erd-10-notification.mmd) | `notify`, `gateway` |

## 8. Decisions of this chapter

| No. | Decision | Reason |
|---|---|---|
| R-DM-01 | The application generates UUIDv7; no DB defaults for IDs. | Independent of the PostgreSQL version; clients generate IDs offline. |
| R-DM-02 | Enumerations as `text` with `CHECK`. | Additive migrations without a type lock. |
| R-DM-03 | Shared table `principals` for person, agent and app, with subtables. | One foreign key for author, grant subject and audit actor. |
| R-DM-04 | Money as `bigint` micro-units plus currency. | Exact and fast. |
| R-DM-05 | Counter rows in one table `sequence_counters` with fixed names. | One pattern for events, audit and permission version; fixed lock order. |
| R-DM-06 | Third-party credentials only as `secret_ref`, tokens only as HMAC. | B10, I5; a DB dump reveals no usable tokens. |
| R-DM-07 | Groups contain only people. | Agents and apps get grants directly; keeps I1 manageable. |
| R-DM-08 | Agents and apps get at most `write`, never `trigger`. | Bot messages never trigger agents (study B); prevents loops and keeps the trigger set of I1 human. |
| R-DM-09 | Grants are immutable; a change = revocation plus a new grant. | Complete history without a revision table. |
| R-DM-10 | DM channels have no space and no grants; access only through `dm_participants`. | Direct messages belong to no space; grants on spaces must not open DMs. |
| R-DM-11 | Every text and DM channel has exactly one thread `kind = 'main'` as its main thread. | Every message has a thread; one sequence mechanism for everything. |
| R-DM-12 | `threads.operator_agent_id` is fixed when a thread (`kind = 'thread'`) is created. | One agent session per thread; an operator change on the channel does not switch a running session. |
| R-DM-13 | No file deduplication across files. | Deletion is simple; the existence of a hash reveals nothing. |
| R-DM-14 | At most one active run per thread (partial index). | Further messages become follow-ups; no parallel runs on one session. |
| R-DM-15 | `event_log` is partitioned by `seq`. | Retention by detaching whole partitions instead of an expensive `DELETE`. |
| R-DM-16 | `notification_preferences` with a polymorphic target without FK. | Three target types in one table; the application handles the cleanup on archiving. |
| R-DM-17 | Outgoing webhooks only to `https` hosts from `webhook.allowed_hosts`. | The baseline forbids server-side loading of third-party URLs (SSRF); webhooks are allowed only to allowed targets. |
| R-DM-18 | Event payloads are redactable, envelopes are not. | Deletion must remove content; order and catch-up stay intact. |
| R-DM-19 | No materialization of effective permissions in the database. | `authz` holds a cache per principal and permission version; one truth, no second permission logic in SQL. |
| R-DM-20 | Agents and apps get at most `upload` as a capability (check in `grants`). | `approve_actions` would mean self-approval, `manage` granting permissions and reading prompts, `export` mass extraction; a bot needs none of them ([07-permissions.md](07-permissions.md), section 2.1). |
| R-DM-21 | The main thread has no fixed operator; the effective operator of the channel at the time of triggering applies. | A main thread is not a single conversation; a frozen operator would silence a channel after the switch. |
| R-DM-22 | A file is attached to at most one message (unique index). | Deleting a message deletes its files unambiguously, without reference counting; forwarding creates a new file. |
| R-DM-23 | Cost entries refer to the price rate for `provider`, `model` and `occurred_at`, not to a price row; used price rows are immutable. | One calculation uses several units; it stays recomputable only with unchanged rows. |
| R-DM-24 | In this milestone, dev environments run only with `billing_mode = 'organization'`. | A personal backend policy per environment would need I1 per environment instead of per agent; that is a separate decision of the "dev environment" milestone (ruling of the lead). |
| R-DM-25 | Read-only channels (`channels.read_only`) as a column, not as a grant. | Owners have `trigger` everywhere; only a state on the channel can also keep owners from writing, as the read mirror of an import needs. |
| R-DM-26 | Sync epoch in `instance`, new after every restore ([ADR 0059](../adr/0059-sync-epoch-after-restore.md)). | A cursor is valid only within one timeline; without an epoch, the new timeline silently overtakes old cursors. |
| R-DM-27 | Cursors of the worker consumers in a separate table (`event_consumer_cursors`) instead of inserting jobs in the write path. | The write path does not need to know the consequences; a new consumer changes no producer ([ADR 0051](../adr/0051-worker-consumers-of-the-event-log-with-a-cursor.md)). |
| R-DM-28 | `audit_events.deny_reason` without `CHECK`, the list closed by a test. | An audit entry must never fail because of a new reason code. |
| R-DM-29 | `event_log.actor_kind` as a separate column instead of deriving it from `actor_principal_id`. | NULL cannot tell `system` apart from "no actor", and `sync` must not ask `identity` for the kind (02, section 4.4). |
| R-DM-30 | The OIDC callback stores only the verified external identity in the login flow (`auth_login_flows.external_identity`); assignment, linking through an enrollment code and provisioning happen only at completion. | The `authorization_url` is not bound to a browser; otherwise anyone who receives it could link their own provider account with someone else's code (login CSRF). Only the app with the `client_verifier` can complete the flow. |
| R-DM-31 | Challenges for step-up and WebAuthn are kept in `auth_challenges` instead of as derived values without storage. | Single use and counted failed attempts thus apply on every node, without depending on synchronized clocks; a table needs only one cleanup job. |
| R-DM-32 | Unread and mention counters are never stored but computed on read. | A stored counter would have to follow every edit, deletion and move and would otherwise silently go stale; `mentions` and `threads` already carry everything needed. |
| R-DM-33 | Groups from claims are bound to their provider (`groups.provider_id`). | Claim values are unique only within one provider; without the binding, a second provider would determine memberships of the first. |
| R-DM-34 | System rows carry a code (`messages.system_code`) instead of text. | Clients show them in their language; a text in the content would be bound to one language and searchable. |
| R-DM-35 | Hosts are a separate target in `grants` (`target_kind = 'host'`) without a level, with exactly the capability `terminal`, only for people and groups (checks). | One grant table for all permissions keeps `authz` the single truth (B7); levels have no meaning on a host, and agents and apps never open a terminal ([24-terminal.md](24-terminal.md), section 8.1). |
| R-DM-36 | `terminal_sessions` holds only metadata; content does not exist in the server. | End-to-end encryption without storage in the server; evidence through audit and metadata ([24-terminal.md](24-terminal.md), section 10.4). |
| R-DM-37 | Consumption of a step-up for the terminal as the column `auth_sessions.step_up_consumed_at`. | A confirmation applies to exactly one opening; a column on the session is enough, because the confirmation applies per session anyway ([08-identity.md](08-identity.md), section 6.3). |
| R-DM-38 | Own measurement as minute aggregates in `stats_minutes` without a primary key, with a unique key over all dimensions (`NULLS NOT DISTINCT`), additive upsert and daily partitions. | Several processes and nodes add into the same row without a coordinator; retention by detaching whole partitions like R-DM-15; the statistics never read from telemetry (R-OB-08, ruling T7). |
| R-DM-39 | Session users per person in `host_session_users`, the name unique per host across all rows, including ended ones. | Ruling T12: the credentials of a session user belong to exactly one person; a reassigned name would give the next person the logins and long-running processes of the previous one ([24-terminal.md](24-terminal.md), section 2.2). |
| R-DM-40 | `terminal_sessions` refers with `(host_session_user_id, host_id, user_id)` to `host_session_users(id, host_id, user_id)`. | A foreign key only on `id` would allow a session user of another host or of another person; the host service would then bind a new name to the wrong person (D-W-66). |
| R-DM-41 | `hosts.reported_session_users` holds the reported names of the session users with the display names of their working folders. | The server assigns only names that the host knows, and offers each person only their own working folders (rulings T12 and T13; D-W-62, D-W-63). |
| R-DM-42 | If the server accepts a report `agent_session.report` with `bind_failed`, it writes in the same transaction exactly one system line `agent_session.resume_failed` with the agent of the run as author; system lines do not reopen a done thread. | 09, section 9.5, and 22, section 9, require exactly one system message on fail open, according to R-DM-34 with a code instead of text; the session belongs to the agent, and reopening would write a second system line `thread.reopened` for the same notice (D-W-104). |
| R-DM-43 | `message_attachments` carries the copies `thread_id` and `channel_id` of its message (`channel_id` changed along on a move, both without their own foreign key), `messages` the column `has_attachments` (created with at least one attachment and not deleted). | Search filters files by channel and thread of their message and messages by `has_file` in SQL before the candidate limit ([14-search.md](14-search.md), sections 4.3 to 4.5); `files` must not read `messages` and `messaging` must not read `message_attachments` (principle 12). Attachments arise only with the message and disappear only with its deletion, so a flag that `messaging` sets on creation and resets on deletion suffices. A foreign key from the copies to `threads` or `channels` would only check existence, which `message_id` with the foreign keys of `messages` already ensures; that the copies equal the values of the message is ensured by attaching and moving in their transaction (D-W-104). |
| R-DM-44 | A held agent invocation is recorded in `runs.invocation_held_until`. | Without a flag `runs` cannot tell whether the invocation of a waiting run has already been written; the release on `files.FileFinalized` and the job `runs.invocation_hold_timeout` serialize on the run row and write it exactly once ([04-sync-protocol.md](04-sync-protocol.md), section 15.3 step 1, R-SY-20, D-K-G6; D-W-104). |
| R-DM-45 | Deleting a file redacts the stored events of resource kind `file` of that file in the same transaction; `file.updated` keeps only the members that 04 section 2.2 rule 5 names, with empty name, size zero, a neutral type and no thumbnails. | 20 sections 4.2 and 4.3 and DP-T01 require the name and digest of a deleted file to be gone, and every `file.updated` carries them; 6.5 named only the resource kind `message`. The kept members are IDs and states, the redacted payload stays valid against `FileUpdatedEvent` (D-W-108). |
| R-DM-46 | The job `files.purge_objects` removes the row of a deleted file after its objects are gone; the detail rows go by `ON DELETE CASCADE`, `principals.avatar_file_id` by `ON DELETE SET NULL`. | The row keeps `original_name`, `sha256`, `transcript_text` and the search vector, which 20 sections 4.2 and 4.3 require to be gone; nothing references a deleted file, because its attachment row went with the message, so a tombstone would serve nobody and need a second redaction of every content column (D-W-108). |
| R-DM-47 | `runs.invocation_held_until` is also set while the server transcript of a triggering voice message is `pending`; the release follows the same rules with `voice.transcript_hold_max`. | 04 section 15.3 step 1 and R-SY-20 hold `agent.invocation` until a server transcript is final, and the column already names `voice.transcript_hold_max`, but its rule named only attachments (D-W-108). |
| R-DM-48 | The ETag of a transcript is derived from its four transcript columns; `files` gets no `version` column. | 05 section 4.8 requires `If-Match` for `file.transcript.submit`; a derived strong ETag (as for other rows without a version, 06 section 8 point 1) detects a lost update without a new column that every file write would have to raise (D-W-108). |
| R-DM-49 | `auth_sessions` stores "View as person" of a console session in `view_as_user_id` and `view_as_started_at` (console sessions only, both set or both empty); `access.view_as` from a console session sets them in the transaction of its audit entry, `access.view_as.end` or the first request after `authz.view_as_max_duration` clears them with an audit entry of the end. | [07-permissions.md](07-permissions.md) section 9.2 requires state on the console session (target person, start) and operations to start and end it, added to the core contract before PLAN M6-01 builds the console session (D-W-100 extended to M5 and M6). |
| R-DM-50 | `instance` stores the identity of the database cluster (`pg_system_identifier`, `pg_timeline_id`); `bothy migrate up` fills it while empty, `bothy restore finalize` replaces it after a restore or a move, and a differing current identity keeps the server not ready (section 6.7). | [ADR 0059](../adr/0059-sync-epoch-after-restore.md) leaves the start lock open; a point-in-time restore raises the timeline and a logical restore changes the system identifier, so both are detected without a value outside the database. |
| R-DM-51 | `gateway_presence` (module `gateway`, `UNLOGGED`) holds the counted live connections of humans with device, focus and `idle`; processes in role `api` (and in role `all` with `cluster.mode = multi`) write it, the `worker` reads it through the presence port; rows of a node that stopped refreshing are ignored after `cluster.presence_ttl`. | With separate roles or several `api` nodes, the presence state of one process is not visible to the `worker` that decides about hints (13, section 12 point 4; RK-37); a table read in the decision's transaction needs no broker, and `UNLOGGED` keeps the write cost low for data that is worthless after a crash. |
| R-DM-52 | The deletion journal numbers its objects densely in `deletion_journal_state` (module `privacy`): the consumer `privacy.journal` writes object `next_number` and moves the number with its cursor, the retention removes objects from `low_number` upward, and `bothy restore finalize` continues from the restored `next_number` to the first missing number. | The object storage port has no listing and no conditional write (core S3, [ADR 0010](../adr/0010-object-storage-behind-an-s3-interface-local-driver-as-default.md)); dense numbers make the journal readable and deletable without either. |

## 9. Configuration values of this chapter

The values are defaults. Other chapters refer only to the name.

| Name | Default | Meaning |
|---|---|---|
| `principal.handle_max_chars` | `32` | length of a handle |
| `principal.display_name_max_chars` | `64` | length of a display name in Unicode characters |
| `branding.product_name_max_chars` | `64` | length of the product name in the branding (`instance_settings`, `branding.product_name`) in Unicode characters |
| `group.name_max_chars` | `100` | length of a group name in Unicode characters |
| `group.description_max_chars` | `500` | length of a group description in Unicode characters |
| `host.name_max_chars` | `100` | length of a host name in Unicode characters |
| `host.os_user_max_chars` | `64` | length of the name of a session user in `host_session_users.os_user` |
| `space.name_max_chars` | `100` | length of a space name in Unicode characters |
| `channel.name_max_chars` | `100` | length of a channel name in Unicode characters |
| `channel.topic_max_chars` | `1024` | length of a channel topic in Unicode characters |
| `forum_tag.name_max_chars` | `50` | length of a tag name in Unicode characters |
| `identity.last_active_granularity` | `5m` | `last_active_at` or `last_used_at` is written at most this often |
| `auth.session_max_age` | `30d` | absolute end of a login session |
| `auth.refresh_token_ttl` | `14d` | lifetime of a refresh token |
| `auth.step_up_max_age` | `5m` | a step-up confirmation counts as fresh for this long |
| `auth.refresh_reuse_grace` | `30s` | grace period for the immediately preceding refresh generation with the same DPoP key |
| `agent.heartbeat_persist_interval` | `60s` | heartbeat fields are stored at most this often |
| `channel.history_backfill_limit_default` | `20` | default for `history_backfill_limit` |
| `channel.history_backfill_limit_max` | `200` | upper limit for `history_backfill_limit` |
| `channel.prompt_max_bytes` | `32768` | size of a channel prompt |
| `thread.title_max_chars` | `200` | length of a thread title |
| `thread.max_tags` | `5` | forum tags per thread |
| `message.max_content_bytes` | `262144` | size of a message (long answers without splitting) |
| `message.max_attachments` | `10` | attachments per message |
| `message.max_mentions` | `50` | distinct mention handles per message (content format, `messages`) |
| `reaction.emoji_max_bytes` | `64` | largest length of a reaction emoji in bytes (otherwise a grapheme cluster has no limit) |
| `reaction.max_distinct_per_message` | `20` | distinct emoji per message; limits `Message.reactions` and the payload of the events |
| `pin.max_per_thread` | `50` | pins per thread; `listPins` is not paginated |
| `template.name_max_chars` | `100` | length of a template name in Unicode characters |
| `template.max_per_scope` | `100` | templates per person or per channel; `listTemplates` is not paginated |
| `file.max_upload_bytes` | `104857600` | size of a file |
| `run.excerpt_max_chars` | `500` | length of `last_text_excerpt` |
| `run.step_detail_max_bytes` | `16384` | size of `run_steps.detail` |
| `run.max_steps_persisted` | `2000` | stored steps per run |
| `run.stall_after` | `10m` | without progress: stall notice |
| `component.default_ttl` | `24h` | expiry of a component without its own value |
| `push.max_consecutive_failures` | `5` | after that, a push registration is deactivated |
| `webhook.allowed_hosts` | empty | allowed target hosts for outgoing webhooks |
| `webhook.max_consecutive_failures` | `20` | after that, a subscription is deactivated |
| `webhook.retry_backoff_base` | `10s` | base of the backoff |
| `webhook.retry_backoff_cap` | `1h` | largest interval |
| `sync.event_log_partition_size` | `1000000` | sequence numbers per partition |
