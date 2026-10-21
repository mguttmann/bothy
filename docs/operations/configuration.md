# Configuration reference

Status: draft M0. Collection of all named configuration values with environment name, type,
default and meaning. Rules: [16-operations.md](../spec/16-operations.md) 5 and ADR 0011.

The source is the schema in the code (ruling L4): every key the server reads stands with kind,
default, bounds and chapter in `services/server/internal/wire/configkeys_*.go`, exported to
`services/server/architecture/config-schema.json`. The table "Configuration values of this
chapter" of the named chapter gives the same default and the meaning; values the server does not
read yet and values of other programs stand only there. Section 4 is generated from the schema and
the chapter tables (`tools/configdoc/configdoc.py`), and `make config-doc-check` keeps both equal
(section 10).

## 1. Sources

| Source | Content | Change | Effect |
|---|---|---|---|
| Process configuration | all keys from section 4 with place `Server` and status `active` | environment (`BOTHY_*`), YAML file (`--config` or `BOTHY_CONFIG_FILE`) or, for `server.role`, the flag `--role` | at startup; secrets after `secrets.cache_ttl` |
| Instance settings | section 6, table `instance_settings` | admin console, `instance.settings.update`, `instance.branding.update` (owner, step-up, audit) | immediately, event `instance.updated` |
| Feature flags | section 7, table `feature_flags` | admin console, `feature_flag.update` (owner, step-up, audit) | immediately, event `instance.updated` |
| OpenTelemetry | section 8, variables `OTEL_*` | environment | at startup |
| Compose | section 9, file `infra/compose/.env` | file | on `docker compose up` |

Precedence of the process configuration: command line (only `--role`, `--config`) before
environment before file before default. A key lives in exactly one source. Section 4 also lists
keys whose user is only built in a later milestone, with the status `planned (<milestone>)`: this
server version does not know them and rejects them as unknown (section 3, item 1). A key enters
the schema with the ticket that builds its first user
([02-architecture.md](../spec/02-architecture.md) 13, item 5).

## 2. Naming rule and formats

Every kind has exactly one parser; default value, environment, file and flag go through the same
one. Error messages never name the checked text.

| Topic | Rule | Example |
|---|---|---|
| Key | at least two segments separated by `.`, each segment `[a-z][a-z0-9]*(_[a-z0-9]+)*`; exactly the keys of the kind `secret_ref` end in `_ref` | `sync.event_retention` |
| Environment name | `BOTHY_` plus the key in uppercase, a dot becomes `__` | `sync.event_retention` -> `BOTHY_SYNC__EVENT_RETENTION` |
| File | YAML with exactly one document, keys nested by dots. Scalars count as the written text and go to the parser of the kind, so YAML typing changes no value (`yes` stays `yes`, `1.10` stays `1.10`, `0x10` is no number). Duplicate and dotted keys, anchors, aliases, merge keys (`<<`) and explicit tags (`!!str`) are errors, as are a list or a mapping where a value stands, and a scalar or `null` where a section stands. A key whose text does not have the form of a key appears in the message only with its line, because a typo can make a value part of the key (`{role:value}` without a space after the colon) | `sync: {event_retention: 30d}` |
| `string` | any text, also empty | `bothy` |
| `enum` | exactly one of the values the chapter names; case matters | `worker` |
| `bool` | `true` or `false` | `true` |
| `int` | whole number in base ten in the range of int64, optionally with a leading `-`; no `+`, no separator, no exponent, no hexadecimal number | `1048576` |
| `float` | finite decimal number: optional `-`, digits with an optional fractional part or only a fractional part (`.5`), optionally an exponent `e` or `E` with an optional sign; no leading `+`, no `inf`, no `NaN`, no hexadecimal number | `0.05` |
| Duration (`duration`) | exactly `(0\|[1-9][0-9]*)(ms\|s\|m\|h\|d)`: a number without sign, fraction or leading zero and exactly one unit, `d` = 24 h. No composition such as `1h30m`; a duration above the largest duration of Go (`time.Duration`, int64 nanoseconds, [time.Duration](https://pkg.go.dev/time#Duration), retrieved 2026-10-12) is an error, so the largest whole number depends on the unit (`106751d`, `2562047h`) | `250ms`, `30d` |
| List (`list(string)`) | environment and default value: a JSON array of strings if the text starts with `[`, otherwise separated by commas with the whitespace around each element removed; empty text is the empty list; an empty element is an error. File: YAML list of scalars or one scalar in the form of the environment | `a.example.com, b.example.com`, `["x,y"]` |
| Prefix list (`list(cidr)`) | like `list(string)`; every element is an IPv4 or IPv6 prefix in CIDR form without set host bits (`192.0.2.10/24` is an error) | `BOTHY_SERVER__TRUSTED_PROXIES=192.0.2.10/32,198.51.100.0/24` |
| Map (`map(string)`) | Environment and default: a JSON object whose values are strings; keys are not empty and occur once; an array, a number, `null` or a nested object as a value is an error. File: a YAML mapping of plain scalars below the key, or a scalar in the environment form. The canonical text is the JSON object with sorted keys (M3-13, D-M3-13-4) | `BOTHY_SEARCH__LOCALE_MAP={"de":"german","en":"english"}` |
| `listen address` | `host:port`; host empty (`:port` means all interfaces) or an IP literal, IPv6 in brackets, no DNS name; port in decimal from 0 to 65535 without a leading zero | `127.0.0.1:8080`, `[::1]:8081`, `:8080` |
| `cron` | five fields separated by whitespace, in UTC: minute 0 to 59, hour 0 to 23, day 1 to 31, month 1 to 12, weekday 0 to 6 (0 = Sunday). A field is a comma list of `*`, `*/n`, `a`, `a-b` and `a-b/n` with `a <= b` and a step `n` from 1 to the highest value of the field; numbers without a leading zero, no names, no shortcuts such as `@daily` | `0 2 * * 0`, `*/15 0-6 * * 1-5` |
| `semver` | SemVer 2.0.0 without a leading `v`, with optional pre-release and build parts | `1.4.0`, `1.4.0-rc.1` |
| `secret_ref` | `<scheme>:<reference>` ([16-operations.md](../spec/16-operations.md) 5.2): `file:` with an absolute path in clean form (no `.` or `..`, no double or trailing `/`, no NUL character), `env:` with a variable name from `[A-Z_][A-Z0-9_]*` that is neither `BOTHY_CONFIG_FILE` nor has the form of a key variable (`BOTHY_A__B`); usual is `BOTHY_SECRET_<NAME>`. This server version reports `azkv` and `awssm` as not supported; any other scheme, a missing scheme and empty text are errors. A default value may contain `<key>` of a string key; when loading, the effective value of that key stands there. In set values only `<secrets.dir>` is replaced | `file:/run/bothy/secrets/db_app_password`, `file:<secrets.dir>/db_app_password` |
| `url` | scheme `http` or `https`, host not empty, no user part, no fragment (`#`), port, if given, in decimal from 1 to 65535 without a leading zero | `https://chat.example.com` |
| Egress target (`egress.allowed_hosts`) | list like `list(string)`, each element `host[:port][;allow_private]` (read by `egress.ParseRule`); host as a DNS name of letters, digits, `-` and `_` (uppercase becomes lowercase, no trailing dot, the last label is no number such as `1` or `0x7f`) or an IP literal (IPv6 in brackets when a port follows; no zone; an IPv4-mapped IPv6 address counts as the IPv4 address); no wildcard, no scheme, no path, no user part; port 1 to 65535 without a leading zero, without a port only the default port of the scheme applies; the only switch is `allow_private`; two elements with the same host and port are a load error. If the value in the environment starts with `[` (IPv6 literal with port as the first element), it must be a JSON array | `api.example.com:443`, `[2001:db8::10]:8443;allow_private`, `BOTHY_EGRESS__ALLOWED_HOSTS=["[2001:db8::10]:8443","api.example.com"]` |

Keys never contain two consecutive underscores; this makes the naming rule reversible.

## 3. Check

1. Unknown `BOTHY_*` variables and unknown keys in the file abort the startup.
   Exempt are `BOTHY_CONFIG_FILE` and variables that a valid reference `env:` in the
   effective value of a key `*_ref` names ([16-operations.md](../spec/16-operations.md) 5.1).
   Variables without the prefix `BOTHY_` are ignored. If a variable appears twice in the
   environment of the process, the server sees only the first entry (this is how the Go runtime
   passes the environment, [syscall/env_unix.go](https://github.com/golang/go/blob/836fa745188fddf49070d010161e30f360862e57/src/syscall/env_unix.go#L20-L38),
   retrieved 2026-10-12); it does not check the second. `BOTHY_CONFIG_FILE` applies only when
   `--config` is missing; empty means no file. `--config` with an empty path is a usage error.
2. Type and range errors abort the startup, also for a value that a higher source overrides. A
   required key needs a non-empty value from file, environment or flag. After that, the cross
   checks from [02-architecture.md](../spec/02-architecture.md) 13, item 4, run as soon as their
   keys are loaded without error; a key of a rejected file or under a rejected section of the
   file is not.
3. All problems come together, sorted by key, one per line
   `config: <key>: <message> (source: <source>)`, for unknown names
   `config: unknown variable BOTHY_X (source: environment)` or
   `config: unknown key "a.b" (source: file <path>)`; a file key without the form of a key
   appears only with its line (`config: unknown key at line 3 below "server" (source: file
   <path>)`). No message names a value (a secret can be in the wrong key).
   Paths and variable names with invalid UTF-8, non-printable characters, whitespace at the edge or
   a quotation mark at the start stand in quotation marks with Go escapes, so that every problem
   stays one line.
4. `bothy config check [--secrets] [--config FILE] [--role ROLE]` checks without starting:
   `configuration ok` and exit code 0, otherwise every problem on standard error and exit code 1;
   a usage error gives exit code 2. For every key `*_ref` whose effective reference uses `env:`,
   it writes, with and without `--secrets`, the warning
   `warning: <key> uses an env: reference; prefer file: (16-operations.md 5.2)` to standard error;
   the exit code stays unaffected. With `--secrets` it also resolves every value of a key `*_ref`
   through the secret provider and writes per key `secret <key>: ok` or
   `secret <key>: <error code>` (`invalid_ref`, `unsupported_scheme`, `not_found`,
   `insecure_file`, `read_failed`), never a value; the error message with reference and rule is on
   standard error. References of `migrate.*` keys are the exception: they print
   `secret <key>: skipped (read only by bothy migrate up --bootstrap)` and are not resolved, so a
   successful check does not prove the credentials of `migrate up --bootstrap`.
   `configuration ok` follows only when every other reference resolves.
5. `bothy config print [--redacted] [--config FILE] [--role ROLE]` shows in the first line
   `# configuration file: <path>` or `# configuration file: none`, then per key, sorted by name,
   `<key> = <value>  # <source>` with the source `default`, `file <path>`, `env BOTHY_X` or
   `flag --role`. Secrets appear only as a reference; nothing is resolved. The output is always
   redacted, `--redacted` only states this, `--redacted=false` is a usage error. Values stand in
   canonical form that can be read in again: a duration in the largest whole unit (`120s` as
   `2m`), lists separated by `, ` or as a JSON array if an element has a comma or whitespace at
   the edge or the first one starts with `[`, IP addresses in normal form. Text with invalid UTF-8,
   non-printable characters, whitespace at the edge or a quotation mark at the start stands in
   quotation marks with Go escapes.

## 4. Named values of all chapters

Generated by `tools/configdoc/configdoc.py` from the schema of the server and the tables
"Configuration values of this chapter" (ruling L4); `make config-reference` rewrites the block,
`make config-doc-check` checks it (section 10). Do not edit by hand. The columns:

- Environment: the variable according to the naming rule from section 2, only for place `Server`.
- Place: `Server` is the process configuration from section 1. The other places `App`,
  `Adapter`, `Hermes adapter`, `Runner`, `Host service`, `SDK`, `Reconciliation app`, `Receiver`,
  `Installation script` and Tool `bothy import` are values of these programs; `Load test` are
  thresholds of `make perf`, no runtime configuration
  ([02-architecture.md](../spec/02-architecture.md) 22). Another place in a chapter table is a
  finding (section 10).
- Type: the kind from the schema with the grammar from section 2, for `enum` with the allowed
  values; empty as long as the key is not in the schema.
- Default and Meaning: from the chapter table; for status `active` the default is the one of the
  schema.
- Status: `active` means this server version reads the key. `planned (<milestone>)` means a
  ticket of this milestone ([PLAN](../PLAN.md)) builds its user; until then this server version
  rejects the key as unknown (section 3, item 1). The milestone is information from the plan, not
  a contract. Empty for values of other programs.

<!-- config-reference:start -->
### 01 Product

Source: [01-product.md](../spec/01-product.md), section 14.

| Key | Environment | Place | Type | Default | Meaning | Status |
|---|---|---|---|---|---|---|
| `ui.message_group_window` |  | App |  | `5m` | messages of the same author within this time get no header of their own |  |
| `ui.message_collapse_lines` |  | App |  | `24` | from this many lines on, a message is collapsed |  |
| `ui.livebox_visible_steps` |  | App |  | `6` | visible steps in the running live box |  |
| `ui.composer_max_lines` |  | App |  | `12` | the input grows up to this many lines |  |
| `ui.autocomplete_debounce` |  | App |  | `150ms` | waiting time before `command.autocomplete` |  |
| `ui.quick_switch_max_results` |  | App |  | `20` | hits in the quick switcher |  |
| `ui.desktop_min_width_four_columns` |  | App |  | `1280px` | below this width the session column collapses |  |
| `ui.desktop_min_width_three_columns` |  | App |  | `960px` | below this width the post list becomes a stack |  |
| `a11y.live_announce_min_interval` |  | App |  | `10s` | the live box announces a change at most this often |  |
| `client.share.recent_targets` |  | App |  | `8` | recently used targets in the share sheet |  |

### 02 Architecture

Source: [02-architecture.md](../spec/02-architecture.md), section 22.

| Key | Environment | Place | Type | Default | Meaning | Status |
|---|---|---|---|---|---|---|
| `cluster.mode` | `BOTHY_CLUSTER__MODE` | Server | enum(single, multi) | `single` | `single` or `multi` (several api nodes) | active |
| `http.read_header_timeout` | `BOTHY_HTTP__READ_HEADER_TIMEOUT` | Server | duration | `5s` | deadline for headers | active |
| `http.request_timeout` | `BOTHY_HTTP__REQUEST_TIMEOUT` | Server | duration | `15s` | context deadline of normal requests | active |
| `http.transfer_timeout` | `BOTHY_HTTP__TRANSFER_TIMEOUT` | Server | duration | `10m` | deadline for upload and download | active |
| `http.idle_timeout` | `BOTHY_HTTP__IDLE_TIMEOUT` | Server | duration | `120s` | idle time of keep-alive connections | active |
| `http.max_header_bytes` | `BOTHY_HTTP__MAX_HEADER_BYTES` | Server | int | `65536` | largest headers | active |
| `http.max_json_body_bytes` | `BOTHY_HTTP__MAX_JSON_BODY_BYTES` | Server | int | `1048576` | largest JSON body; strictly greater than `message.max_content_bytes` (13, item 4) | active |
| `http.max_json_depth` | `BOTHY_HTTP__MAX_JSON_DEPTH` | Server | int | `32` | largest nesting depth of a JSON body (SEC-IN-2); a test holds that every request example in `openapi.yaml` stays below it | active |
| `http.unavailable_retry_after` | `BOTHY_HTTP__UNAVAILABLE_RETRY_AFTER` | Server | duration | `5s` | `Retry-After` of every `503 unavailable` response (in whole seconds) | active |
| `database.acquire_timeout` | `BOTHY_DATABASE__ACQUIRE_TIMEOUT` | Server | duration | `2s` | waiting for a pool connection | active |
| `database.statement_timeout` | `BOTHY_DATABASE__STATEMENT_TIMEOUT` | Server | duration | `5s` | deadline per SQL statement | active |
| `database.lock_timeout` | `BOTHY_DATABASE__LOCK_TIMEOUT` | Server | duration | `2s` | deadline for locks | active |
| `database.idle_in_tx_timeout` | `BOTHY_DATABASE__IDLE_IN_TX_TIMEOUT` | Server | duration | `10s` | open transaction without work | active |
| `database.tx_retry_max` | `BOTHY_DATABASE__TX_RETRY_MAX` | Server | int | `2` | retries of the Unit of Work on `40P01` and `55P03` | active |
| `database.listen_reconnect_backoff_cap` | `BOTHY_DATABASE__LISTEN_RECONNECT_BACKOFF_CAP` | Server | duration | `10s` | largest interval when re-establishing `LISTEN` | active |
| `startup.dependency_wait` | `BOTHY_STARTUP__DEPENDENCY_WAIT` | Server | duration | `60s` | waiting for the database at start | active |
| `sync.log_poll_interval` | `BOTHY_SYNC__LOG_POLL_INTERVAL` | Server | duration | `1s` | poll of the log tail without `NOTIFY` | active |
| `sync.tail_batch_size` | `BOTHY_SYNC__TAIL_BATCH_SIZE` | Server | int | `1000` | events per read of the log tail | active |
| `sync.partition_maintenance_interval` | `BOTHY_SYNC__PARTITION_MAINTENANCE_INTERVAL` | Server | duration | `10m` | interval of the job `sync.partition_maintenance` | active |
| `sync.event_log_partitions_ahead` | `BOTHY_SYNC__EVENT_LOG_PARTITIONS_AHEAD` | Server | int | `3` | partitions of `event_log` that are ready ahead of the head; with `sync.event_log_partition_size` they last longer than two intervals `sync.partition_maintenance_interval` at `perf.event_commit_min_per_s` | active |
| `gateway.write_timeout` | `BOTHY_GATEWAY__WRITE_TIMEOUT` | Server | duration | `10s` | deadline for one WebSocket write | active |
| `consumer.batch_size` | `BOTHY_CONSUMER__BATCH_SIZE` | Server | int | `500` | events per round of a worker consumer | active |
| `consumer.poll_interval` | `BOTHY_CONSUMER__POLL_INTERVAL` | Server | duration | `1s` | poll of a worker consumer without `NOTIFY` | active |
| `consumer.pending_alert_after` | `BOTHY_CONSUMER__PENDING_ALERT_AFTER` | Server | duration | `15m` | age of the oldest unprocessed event of a worker consumer that raises the alarm `BothyEventConsumerLag`; strictly below `sync.event_retention` (13, item 4) | active |
| `sync.idempotency_cleanup_interval` | `BOTHY_SYNC__IDEMPOTENCY_CLEANUP_INTERVAL` | Server | duration | `1h` | interval of the job `sync.idempotency_cleanup` | active |
| `sync.idempotency_cleanup_batch_size` | `BOTHY_SYNC__IDEMPOTENCY_CLEANUP_BATCH_SIZE` | Server | int | `1000` | rows per deletion batch of `sync.idempotency_cleanup` | active |
| `ratelimit.max_tracked_keys` | `BOTHY_RATELIMIT__MAX_TRACKED_KEYS` | Server | int | `100000` | a process holds at most this many rate-limit buckets; beyond that, full buckets are removed first, then the oldest buckets | active |
| `ratelimit.sweep_interval` | `BOTHY_RATELIMIT__SWEEP_INTERVAL` | Server | duration | `1m` | interval at which full buckets are removed (a full bucket decides like a missing one) | active |
| `jobs.schema` | `BOTHY_JOBS__SCHEMA` | Server | string | `jobs` | PostgreSQL schema of the River tables | active |
| `jobs.queue.default.max_workers` | `BOTHY_JOBS__QUEUE__DEFAULT__MAX_WORKERS` | Server | int | `8` | parallelism `default` | active |
| `jobs.queue.files.max_workers` | `BOTHY_JOBS__QUEUE__FILES__MAX_WORKERS` | Server | int | `2` | parallelism `files` | active |
| `jobs.queue.delivery.max_workers` | `BOTHY_JOBS__QUEUE__DELIVERY__MAX_WORKERS` | Server | int | `16` | parallelism `delivery` | active |
| `jobs.queue.maintenance.max_workers` | `BOTHY_JOBS__QUEUE__MAINTENANCE__MAX_WORKERS` | Server | int | `2` | parallelism `maintenance` | active |
| `jobs.queue.privacy.max_workers` | `BOTHY_JOBS__QUEUE__PRIVACY__MAX_WORKERS` | Server | int | `1` | parallelism `privacy` | active |
| `shutdown.drain_delay` | `BOTHY_SHUTDOWN__DRAIN_DELAY` | Server | duration | `5s` | waiting after `ready = false` | active |
| `shutdown.http_grace` | `BOTHY_SHUTDOWN__HTTP_GRACE` | Server | duration | `20s` | deadline for running HTTP requests | active |
| `shutdown.jobs_soft_timeout` | `BOTHY_SHUTDOWN__JOBS_SOFT_TIMEOUT` | Server | duration | `20s` | soft stop of River | active |
| `shutdown.telemetry_flush_timeout` | `BOTHY_SHUTDOWN__TELEMETRY_FLUSH_TIMEOUT` | Server | duration | `5s` | flushing telemetry | active |
| `shutdown.total_timeout` | `BOTHY_SHUTDOWN__TOTAL_TIMEOUT` | Server | duration | `60s` | total deadline until the end; at least the sum of the step budgets (15.3) | active |
| `perf.reference_profile` |  | Load test |  | `4 vCPU, 8 GiB RAM, SSD, PostgreSQL 18 local with fsync and synchronous_commit on, role all` | environment of the load tests; measurement modes in ADR 0063 |  |
| `perf.sustained_messages_per_s` |  | Load test |  | `200` | sustained load for `perf.message_create_p99` |  |
| `perf.message_create_p99` |  | Load test |  | `150ms` | target |  |
| `perf.event_commit_min_per_s` |  | Load test |  | `1000` | target (minimum) |  |
| `perf.fanout_local_p99` |  | Load test |  | `200ms` | target |  |
| `perf.fanout_cross_node_p99` |  | Load test |  | `500ms` | target |  |
| `perf.invocation_delivery_p99` |  | Load test |  | `500ms` | target |  |
| `perf.run_patch_fanout_p99` |  | Load test |  | `500ms` | target |  |
| `perf.catchup_page_p99` |  | Load test |  | `250ms` | target |  |
| `perf.authorize_cached_p99` |  | Load test |  | `100us` | target |  |
| `perf.ws_connections_per_node` |  | Load test |  | `5000` | target (minimum) |  |
| `perf.ws_idle_conn_mem_max` |  | Load test |  | `64KiB` | target (maximum) |  |
| `perf.idle_rss_max` |  | Load test |  | `256MiB` | target (maximum) |  |
| `perf.startup_ready_max` |  | Load test |  | `5s` | target (maximum) |  |
| `perf.push_handoff_p95` |  | Load test |  | `2s` | target |  |

### 03 Data model (PostgreSQL)

Source: [03-data-model.md](../spec/03-data-model.md), section 9.

| Key | Environment | Place | Type | Default | Meaning | Status |
|---|---|---|---|---|---|---|
| `principal.handle_max_chars` | `BOTHY_PRINCIPAL__HANDLE_MAX_CHARS` | Server | int | `32` | length of a handle | active |
| `principal.display_name_max_chars` | `BOTHY_PRINCIPAL__DISPLAY_NAME_MAX_CHARS` | Server | int | `64` | length of a display name in Unicode characters | active |
| `branding.product_name_max_chars` | `BOTHY_BRANDING__PRODUCT_NAME_MAX_CHARS` | Server | int | `64` | length of the product name in the branding (`instance_settings`, `branding.product_name`) in Unicode characters | active |
| `group.name_max_chars` | `BOTHY_GROUP__NAME_MAX_CHARS` | Server | int | `100` | length of a group name in Unicode characters | active |
| `group.description_max_chars` | `BOTHY_GROUP__DESCRIPTION_MAX_CHARS` | Server | int | `500` | length of a group description in Unicode characters | active |
| `host.name_max_chars` | `BOTHY_HOST__NAME_MAX_CHARS` | Server | int | `100` | length of a host name in Unicode characters | active |
| `host.os_user_max_chars` | `BOTHY_HOST__OS_USER_MAX_CHARS` | Server | int | `64` | length of the name of a session user in `host_session_users.os_user` | active |
| `space.name_max_chars` | `BOTHY_SPACE__NAME_MAX_CHARS` | Server | int | `100` | length of a space name in Unicode characters | active |
| `channel.name_max_chars` | `BOTHY_CHANNEL__NAME_MAX_CHARS` | Server | int | `100` | length of a channel name in Unicode characters | active |
| `channel.topic_max_chars` | `BOTHY_CHANNEL__TOPIC_MAX_CHARS` | Server | int | `1024` | length of a channel topic in Unicode characters | active |
| `forum_tag.name_max_chars` | `BOTHY_FORUM_TAG__NAME_MAX_CHARS` | Server | int | `50` | length of a tag name in Unicode characters | active |
| `identity.last_active_granularity` | `BOTHY_IDENTITY__LAST_ACTIVE_GRANULARITY` | Server | duration | `5m` | `last_active_at` or `last_used_at` is written at most this often | active |
| `auth.session_max_age` | `BOTHY_AUTH__SESSION_MAX_AGE` | Server | duration | `30d` | absolute end of a login session | active |
| `auth.refresh_token_ttl` | `BOTHY_AUTH__REFRESH_TOKEN_TTL` | Server | duration | `14d` | lifetime of a refresh token | active |
| `auth.step_up_max_age` | `BOTHY_AUTH__STEP_UP_MAX_AGE` | Server | duration | `5m` | a step-up confirmation counts as fresh for this long | active |
| `auth.refresh_reuse_grace` | `BOTHY_AUTH__REFRESH_REUSE_GRACE` | Server | duration | `30s` | grace period for the immediately preceding refresh generation with the same DPoP key | active |
| `agent.heartbeat_persist_interval` | `BOTHY_AGENT__HEARTBEAT_PERSIST_INTERVAL` | Server | duration | `60s` | heartbeat fields are stored at most this often | active |
| `channel.history_backfill_limit_default` | `BOTHY_CHANNEL__HISTORY_BACKFILL_LIMIT_DEFAULT` | Server | int | `20` | default for `history_backfill_limit` | active |
| `channel.history_backfill_limit_max` | `BOTHY_CHANNEL__HISTORY_BACKFILL_LIMIT_MAX` | Server | int | `200` | upper limit for `history_backfill_limit` | active |
| `channel.prompt_max_bytes` | `BOTHY_CHANNEL__PROMPT_MAX_BYTES` | Server | int | `32768` | size of a channel prompt | active |
| `thread.title_max_chars` | `BOTHY_THREAD__TITLE_MAX_CHARS` | Server | int | `200` | length of a thread title | active |
| `thread.max_tags` | `BOTHY_THREAD__MAX_TAGS` | Server | int | `5` | forum tags per thread | active |
| `message.max_content_bytes` | `BOTHY_MESSAGE__MAX_CONTENT_BYTES` | Server | int | `262144` | size of a message (long answers without splitting) | active |
| `message.max_attachments` | `BOTHY_MESSAGE__MAX_ATTACHMENTS` | Server | int | `10` | attachments per message | active |
| `message.max_mentions` | `BOTHY_MESSAGE__MAX_MENTIONS` | Server | int | `50` | distinct mention handles per message (content format, `messages`) | active |
| `reaction.emoji_max_bytes` | `BOTHY_REACTION__EMOJI_MAX_BYTES` | Server | int | `64` | largest length of a reaction emoji in bytes (otherwise a grapheme cluster has no limit) | active |
| `reaction.max_distinct_per_message` | `BOTHY_REACTION__MAX_DISTINCT_PER_MESSAGE` | Server | int | `20` | distinct emoji per message; limits `Message.reactions` and the payload of the events | active |
| `pin.max_per_thread` | `BOTHY_PIN__MAX_PER_THREAD` | Server | int | `50` | pins per thread; `listPins` is not paginated | active |
| `template.name_max_chars` | `BOTHY_TEMPLATE__NAME_MAX_CHARS` | Server | int | `100` | length of a template name in Unicode characters | active |
| `template.max_per_scope` | `BOTHY_TEMPLATE__MAX_PER_SCOPE` | Server | int | `100` | templates per person or per channel; `listTemplates` is not paginated | active |
| `file.max_upload_bytes` | `BOTHY_FILE__MAX_UPLOAD_BYTES` | Server | int | `104857600` | size of a file | active |
| `run.excerpt_max_chars` | `BOTHY_RUN__EXCERPT_MAX_CHARS` | Server | int | `500` | length of `last_text_excerpt` | active |
| `run.step_detail_max_bytes` | `BOTHY_RUN__STEP_DETAIL_MAX_BYTES` | Server | int | `16384` | size of `run_steps.detail` | active |
| `run.max_steps_persisted` | `BOTHY_RUN__MAX_STEPS_PERSISTED` | Server | int | `2000` | stored steps per run | active |
| `run.stall_after` | `BOTHY_RUN__STALL_AFTER` | Server |  | `10m` | without progress: stall notice | planned (M2) |
| `component.default_ttl` | `BOTHY_COMPONENT__DEFAULT_TTL` | Server |  | `24h` | expiry of a component without its own value | planned (M2) |
| `push.max_consecutive_failures` | `BOTHY_PUSH__MAX_CONSECUTIVE_FAILURES` | Server | int | `5` | after that, a push registration is deactivated | active |
| `webhook.allowed_hosts` | `BOTHY_WEBHOOK__ALLOWED_HOSTS` | Server | list(string) | empty | allowed target hosts for outgoing webhooks | active |
| `webhook.max_consecutive_failures` | `BOTHY_WEBHOOK__MAX_CONSECUTIVE_FAILURES` | Server | int | `20` | after that, a subscription is deactivated | active |
| `webhook.retry_backoff_base` | `BOTHY_WEBHOOK__RETRY_BACKOFF_BASE` | Server | duration | `10s` | base of the backoff | active |
| `webhook.retry_backoff_cap` | `BOTHY_WEBHOOK__RETRY_BACKOFF_CAP` | Server | duration | `1h` | largest interval | active |
| `sync.event_log_partition_size` | `BOTHY_SYNC__EVENT_LOG_PARTITION_SIZE` | Server | int | `1000000` | sequence numbers per partition | active |

### 04 Event and sync protocol

Source: [04-sync-protocol.md](../spec/04-sync-protocol.md), section 18.

| Key | Environment | Place | Type | Default | Meaning | Status |
|---|---|---|---|---|---|---|
| `sync.event_retention` | `BOTHY_SYNC__EVENT_RETENTION` | Server | duration | `30d` | durable events stay available for catch-up this long | active |
| `sync.event_payload_max_bytes` | `BOTHY_SYNC__EVENT_PAYLOAD_MAX_BYTES` | Server | int | `65536` | largest payload of an event | active |
| `sync.ws_max_frame_bytes` | `BOTHY_SYNC__WS_MAX_FRAME_BYTES` | Server | int | `1048576` | largest WebSocket frame | active |
| `sync.identify_timeout` | `BOTHY_SYNC__IDENTIFY_TIMEOUT` | Server | duration | `10s` | time between `hello` and `identify` | active |
| `sync.heartbeat_interval` | `BOTHY_SYNC__HEARTBEAT_INTERVAL` | Server | duration | `30s` | interval of the heartbeats (reported in `hello`) | active |
| `sync.heartbeat_miss_limit` | `BOTHY_SYNC__HEARTBEAT_MISS_LIMIT` | Server | int | `2` | missed heartbeats until the connection is closed | active |
| `sync.resume_window` | `BOTHY_SYNC__RESUME_WINDOW` | Server | duration | `5m` | `resume` is possible this long after a disconnect | active |
| `sync.reauth_lead_time` | `BOTHY_SYNC__REAUTH_LEAD_TIME` | Server | duration | `60s` | lead time of `reauth_required` before the token expires | active |
| `sync.reconnect_backoff_base` | `BOTHY_SYNC__RECONNECT_BACKOFF_BASE` | Server |  | `1s` | base of the backoff when reconnecting | planned (M2) |
| `sync.reconnect_backoff_cap` | `BOTHY_SYNC__RECONNECT_BACKOFF_CAP` | Server |  | `60s` | largest interval when reconnecting | planned (M2) |
| `sync.catchup_page_size` | `BOTHY_SYNC__CATCHUP_PAGE_SIZE` | Server | int | `500` | events per catch-up page | active |
| `sync.catchup_max_events` | `BOTHY_SYNC__CATCHUP_MAX_EVENTS` | Server | int | `50000` | a larger backlog yields `resync_required` | active |
| `sync.cursor_advance_every` | `BOTHY_SYNC__CURSOR_ADVANCE_EVERY` | Server | int | `1000` | one `cursor` frame after this many skipped events | active |
| `sync.outbound_queue_max` | `BOTHY_SYNC__OUTBOUND_QUEUE_MAX` | Server | int | `10000` | frames in the outbound queue of a connection until `4014` | active |
| `sync.idempotency_key_ttl` | `BOTHY_SYNC__IDEMPOTENCY_KEY_TTL` | Server | duration | `24h` | retention of `Idempotency-Key` | active |
| `sync.max_focused_threads` | `BOTHY_SYNC__MAX_FOCUSED_THREADS` | Server | int | `20` | threads in `focus` per connection | active |
| `sync.run_patch_coalesce_interval` | `BOTHY_SYNC__RUN_PATCH_COALESCE_INTERVAL` | Server | duration | `250ms` | collection window for `run.updated` | active |
| `sync.typing_min_interval` | `BOTHY_SYNC__TYPING_MIN_INTERVAL` | Server | duration | `5s` | `typing.start` at most this often | active |
| `sync.typing_ttl` | `BOTHY_SYNC__TYPING_TTL` | Server | duration | `8s` | typing indicator without renewal | active |
| `client.cache_max_age` | `BOTHY_CLIENT__CACHE_MAX_AGE` | Server | duration | `30d` | unused cache entries | active |
| `client.outbox_retry_base` | `BOTHY_CLIENT__OUTBOX_RETRY_BASE` | Server |  | `2s` | base of the backoff in the outbox | planned (M4) |
| `client.outbox_retry_cap` | `BOTHY_CLIENT__OUTBOX_RETRY_CAP` | Server |  | `5m` | largest interval in the outbox | planned (M4) |
| `agent.invocation_token_ttl` | `BOTHY_AGENT__INVOCATION_TOKEN_TTL` | Server | duration | `60s` | validity of an invocation token | active |
| `agent.ack_timeout` | `BOTHY_AGENT__ACK_TIMEOUT` | Server | duration | `30s` | without `ack`: redelivery | active |
| `agent.ack_persist_interval` | `BOTHY_AGENT__ACK_PERSIST_INTERVAL` | Server | duration | `1s` | `acked_seq` is stored at least this often | active |
| `agent.max_inflight_events` | `BOTHY_AGENT__MAX_INFLIGHT_EVENTS` | Server | int | `100` | unacknowledged events per consumer | active |
| `agent.run_update_min_interval` | `BOTHY_AGENT__RUN_UPDATE_MIN_INTERVAL` | Server |  | `500ms` | adapter throttle for `run.update` | planned (M2) |
| `agent.presence_timeout` | `BOTHY_AGENT__PRESENCE_TIMEOUT` | Server | duration | `90s` | without a heartbeat, the agent counts as `offline` | active |
| `agent.cancel_grace` | `BOTHY_AGENT__CANCEL_GRACE` | Server | duration | `30s` | time for the adapter to report a cancellation | active |

### 05 Authorization action catalogue

Source: [05-actions.md](../spec/05-actions.md), section 10.

| Key | Environment | Place | Type | Default | Meaning | Status |
|---|---|---|---|---|---|---|
| `audit.deny_aggregate_window` | `BOTHY_AUDIT__DENY_AGGREGATE_WINDOW` | Server | duration | `60s` | aggregation window for identical denials | active |
| `audit.deny_aggregate_max_keys` | `BOTHY_AUDIT__DENY_AGGREGATE_MAX_KEYS` | Server | int | `10000` | at most this many open aggregations per process; limits the memory during floods without login | active |
| `status.public_enabled` | `BOTHY_STATUS__PUBLIC_ENABLED` | Server |  | `false` | status page without login | planned (M7) |
| `ratelimit.rl.public_auth.per_minute` | `BOTHY_RATELIMIT__RL__PUBLIC_AUTH__PER_MINUTE` | Server | int | `30` | per client IP | active |
| `ratelimit.rl.public_auth.burst` | `BOTHY_RATELIMIT__RL__PUBLIC_AUTH__BURST` | Server | int | `10` | per client IP | active |
| `ratelimit.rl.gateway_connect.per_minute` | `BOTHY_RATELIMIT__RL__GATEWAY_CONNECT__PER_MINUTE` | Server | int | `12` | per device or token | active |
| `ratelimit.rl.gateway_connect.burst` | `BOTHY_RATELIMIT__RL__GATEWAY_CONNECT__BURST` | Server | int | `4` | per device or token | active |
| `ratelimit.rl.read.per_minute` | `BOTHY_RATELIMIT__RL__READ__PER_MINUTE` | Server | int | `600` | per principal | active |
| `ratelimit.rl.read.burst` | `BOTHY_RATELIMIT__RL__READ__BURST` | Server | int | `100` | per principal | active |
| `ratelimit.rl.write.per_minute` | `BOTHY_RATELIMIT__RL__WRITE__PER_MINUTE` | Server | int | `120` | per principal | active |
| `ratelimit.rl.write.burst` | `BOTHY_RATELIMIT__RL__WRITE__BURST` | Server | int | `30` | per principal | active |
| `ratelimit.rl.message.per_minute` | `BOTHY_RATELIMIT__RL__MESSAGE__PER_MINUTE` | Server | int | `60` | per principal | active |
| `ratelimit.rl.message.burst` | `BOTHY_RATELIMIT__RL__MESSAGE__BURST` | Server | int | `20` | per principal | active |
| `ratelimit.rl.trigger.per_minute` | `BOTHY_RATELIMIT__RL__TRIGGER__PER_MINUTE` | Server | int | `30` | per principal | active |
| `ratelimit.rl.trigger.burst` | `BOTHY_RATELIMIT__RL__TRIGGER__BURST` | Server | int | `10` | per principal | active |
| `ratelimit.rl.interaction.per_minute` | `BOTHY_RATELIMIT__RL__INTERACTION__PER_MINUTE` | Server | int | `60` | per principal | active |
| `ratelimit.rl.interaction.burst` | `BOTHY_RATELIMIT__RL__INTERACTION__BURST` | Server | int | `20` | per principal | active |
| `ratelimit.rl.upload.per_minute` | `BOTHY_RATELIMIT__RL__UPLOAD__PER_MINUTE` | Server | int | `30` | per principal | active |
| `ratelimit.rl.upload.burst` | `BOTHY_RATELIMIT__RL__UPLOAD__BURST` | Server | int | `10` | per principal | active |
| `ratelimit.rl.search.per_minute` | `BOTHY_RATELIMIT__RL__SEARCH__PER_MINUTE` | Server | int | `60` | per principal | active |
| `ratelimit.rl.search.burst` | `BOTHY_RATELIMIT__RL__SEARCH__BURST` | Server | int | `20` | per principal | active |
| `ratelimit.rl.typing.per_minute` | `BOTHY_RATELIMIT__RL__TYPING__PER_MINUTE` | Server | int | `30` | per connection | active |
| `ratelimit.rl.typing.burst` | `BOTHY_RATELIMIT__RL__TYPING__BURST` | Server | int | `5` | per connection | active |
| `ratelimit.rl.admin.per_minute` | `BOTHY_RATELIMIT__RL__ADMIN__PER_MINUTE` | Server | int | `60` | per principal | active |
| `ratelimit.rl.admin.burst` | `BOTHY_RATELIMIT__RL__ADMIN__BURST` | Server | int | `20` | per principal | active |
| `ratelimit.rl.export.per_minute` | `BOTHY_RATELIMIT__RL__EXPORT__PER_MINUTE` | Server | int | `2` | per principal | active |
| `ratelimit.rl.export.burst` | `BOTHY_RATELIMIT__RL__EXPORT__BURST` | Server | int | `2` | per principal | active |
| `ratelimit.rl.agent_runtime.per_minute` | `BOTHY_RATELIMIT__RL__AGENT_RUNTIME__PER_MINUTE` | Server | int | `1200` | per agent | active |
| `ratelimit.rl.agent_runtime.burst` | `BOTHY_RATELIMIT__RL__AGENT_RUNTIME__BURST` | Server | int | `200` | per agent | active |
| `ratelimit.rl.webhook_in.per_minute` | `BOTHY_RATELIMIT__RL__WEBHOOK_IN__PER_MINUTE` | Server | int | `60` | per incoming webhook | active |
| `ratelimit.rl.webhook_in.burst` | `BOTHY_RATELIMIT__RL__WEBHOOK_IN__BURST` | Server | int | `20` | per incoming webhook | active |
| `ratelimit.rl.import.per_minute` | `BOTHY_RATELIMIT__RL__IMPORT__PER_MINUTE` | Server | int | `120` | per owner for `import.apply` | active |
| `ratelimit.rl.import.burst` | `BOTHY_RATELIMIT__RL__IMPORT__BURST` | Server | int | `30` | per owner for `import.apply` | active |
| `ratelimit.rl.terminal.per_minute` | `BOTHY_RATELIMIT__RL__TERMINAL__PER_MINUTE` | Server | int | `20` | per person | active |
| `ratelimit.rl.terminal.burst` | `BOTHY_RATELIMIT__RL__TERMINAL__BURST` | Server | int | `5` | per person | active |
| `ratelimit.rl.host.per_minute` | `BOTHY_RATELIMIT__RL__HOST__PER_MINUTE` | Server | int | `120` | per host | active |
| `ratelimit.rl.host.burst` | `BOTHY_RATELIMIT__RL__HOST__BURST` | Server | int | `30` | per host | active |

### 06 Public API

Source: [06-api.md](../spec/06-api.md), section 20.

| Key | Environment | Place | Type | Default | Meaning | Status |
|---|---|---|---|---|---|---|
| `api.page_size_default` | `BOTHY_API__PAGE_SIZE_DEFAULT` | Server | int | `50` | page size without `limit` | active |
| `api.page_size_max` | `BOTHY_API__PAGE_SIZE_MAX` | Server | int | `200` | larger `limit` values are capped | active |
| `api.cors_allowed_origins` | `BOTHY_API__CORS_ALLOWED_ORIGINS` | Server | list(string) | empty | additional origins for CORS, each in the form of `server.public_url` | active |
| `api.deprecation_min_notice` | `BOTHY_API__DEPRECATION_MIN_NOTICE` | Server |  | `180d` | minimum gap from `Deprecation` to `Sunset` | planned (M1) |
| `api.major_version_overlap` | `BOTHY_API__MAJOR_VERSION_OVERLAP` | Server |  | `365d` | how long an old major version stays after the new one appears | planned (M7) |
| `api.stats_max_buckets` | `BOTHY_API__STATS_MAX_BUCKETS` | Server |  | `1000` | time buckets per statistics query; more yields `400` | planned (M6) |
| `message.list_content_max_bytes` | `BOTHY_MESSAGE__LIST_CONTENT_MAX_BYTES` | Server | int | `16384` | largest `content_md` per message in lists; above that truncated with `content_truncated` (section 6) | active |
| `status.history_window` | `BOTHY_STATUS__HISTORY_WINDOW` | Server |  | `7d` | how long the status page shows resolved incidents | planned (M7) |

### 07 Permissions and authorization

Source: [07-permissions.md](../spec/07-permissions.md), section 14.

| Key | Environment | Place | Type | Default | Meaning | Status |
|---|---|---|---|---|---|---|
| `authz.expiry_sweep_interval` | `BOTHY_AUTHZ__EXPIRY_SWEEP_INTERVAL` | Server | duration | `1m` | interval of the job that reports expiries and writes reminders | active |
| `authz.expiry_reminder_lead` | `BOTHY_AUTHZ__EXPIRY_REMINDER_LEAD` | Server | duration | `48h` | how long before expiry the reminder comes (Study C: two days) | active |
| `authz.expiry_sweep_batch_max` | `BOTHY_AUTHZ__EXPIRY_SWEEP_BATCH_MAX` | Server | int | `500` | rows per transaction of the jobs `authz.expiry_sweep` and `authz.expiry_reminder` | active |
| `authz.view_as_max_duration` | `BOTHY_AUTHZ__VIEW_AS_MAX_DURATION` | Server | duration | `15m` | maximum duration of "View as person" | active |
| `authz.snapshot_cache_max_entries` | `BOTHY_AUTHZ__SNAPSHOT_CACHE_MAX_ENTRIES` | Server | int | `10000` | permission snapshots in memory per process (LRU) | active |
| `authz.not_found_timing_tolerance` | `BOTHY_AUTHZ__NOT_FOUND_TIMING_TOLERANCE` | Server |  | `20ms` | allowed deviation of the median in AZ-P4 | planned (M1) |

### 08 Identity, devices and sessions

Source: [08-identity.md](../spec/08-identity.md), section 18.

| Key | Environment | Place | Type | Default | Meaning | Status |
|---|---|---|---|---|---|---|
| `auth.access_token_ttl` | `BOTHY_AUTH__ACCESS_TOKEN_TTL` | Server | duration | `10m` | Lifetime of an access token | active |
| `auth.dpop_nonce_ttl` | `BOTHY_AUTH__DPOP_NONCE_TTL` | Server | duration | `5m` | Validity of a DPoP nonce (`DPoP-Nonce` header and `hello` of the gateway) | active |
| `auth.token_pepper_ref` | `BOTHY_AUTH__TOKEN_PEPPER_REF` | Server | secret_ref | `file:<secrets.dir>/token_pepper` | Reference to the server pepper for the HMAC hashes of all tokens and codes (03, principle 9) | active |
| `auth.clock_leeway` | `BOTHY_AUTH__CLOCK_LEEWAY` | Server | duration | `30s` | allowed clock skew for `exp`, `nbf`, `iat` | active |
| `auth.dpop_proof_max_age` | `BOTHY_AUTH__DPOP_PROOF_MAX_AGE` | Server | duration | `60s` | Maximum age of a DPoP proof (`iat`) | active |
| `auth.login_flow_ttl` | `BOTHY_AUTH__LOGIN_FLOW_TTL` | Server | duration | `10m` | Validity of a login flow | active |
| `auth.server_code_ttl` | `BOTHY_AUTH__SERVER_CODE_TTL` | Server | duration | `60s` | Validity of the one-time server code | active |
| `auth.oidc_jwks_cache_ttl` | `BOTHY_AUTH__OIDC_JWKS_CACHE_TTL` | Server | duration | `1h` | Cache of the provider metadata (discovery) and provider keys (JWKS) | active |
| `auth.oidc_client_assertion_ttl` | `BOTHY_AUTH__OIDC_CLIENT_ASSERTION_TTL` | Server | duration | `2m` | Validity of a client assertion with `private_key_jwt` | active |
| `auth.group_claim_max_age` | `BOTHY_AUTH__GROUP_CLAIM_MAX_AGE` | Server | duration | `30d` | Expiry of memberships from group claims after the last login | active |
| `auth.idp_recheck_interval` | `BOTHY_AUTH__IDP_RECHECK_INTERVAL` | Server | duration | `30d` | new login at the provider at the latest after this | active |
| `auth.trusted_header_bind_requests` | `BOTHY_AUTH__TRUSTED_HEADER_BIND_REQUESTS` | Server | bool | `true` | every request must carry the same header login | active |
| `auth.enrollment_code_ttl` | `BOTHY_AUTH__ENROLLMENT_CODE_TTL` | Server | duration | `72h` | Validity of an enrollment code | active |
| `auth.enrollment_code_max_attempts` | `BOTHY_AUTH__ENROLLMENT_CODE_MAX_ATTEMPTS` | Server | int | `5` | Failed attempts per enrollment code | active |
| `auth.device_pending_ttl` | `BOTHY_AUTH__DEVICE_PENDING_TTL` | Server | duration | `24h` | after this, a pending device is revoked | active |
| `auth.device_verification_code_digits` | `BOTHY_AUTH__DEVICE_VERIFICATION_CODE_DIGITS` | Server | int | `6` | Length of the verification code | active |
| `auth.pairing_code_ttl` | `BOTHY_AUTH__PAIRING_CODE_TTL` | Server | duration | `5m` | Validity of a pairing code | active |
| `auth.pairing_code_chars` | `BOTHY_AUTH__PAIRING_CODE_CHARS` | Server | int | `8` | Length of the pairing code (Crockford Base32, 40 bits) | active |
| `auth.device_inactive_revoke_after` | `BOTHY_AUTH__DEVICE_INACTIVE_REVOKE_AFTER` | Server | duration | `180d` | a device without contact is revoked | active |
| `auth.cleanup_interval` | `BOTHY_AUTH__CLEANUP_INTERVAL` | Server | duration | `1h` | Interval of the job `identity.auth_cleanup` (delete expired challenges, login flows and pairings, revoke pending and unused devices) | active |
| `auth.step_up_challenge_ttl` | `BOTHY_AUTH__STEP_UP_CHALLENGE_TTL` | Server | duration | `2m` | Validity of a step-up challenge | active |
| `auth.step_up_fail_limit` | `BOTHY_AUTH__STEP_UP_FAIL_LIMIT` | Server | int | `5` | Failed step-up attempts per window | active |
| `auth.require_key_attestation` | `BOTHY_AUTH__REQUIRE_KEY_ATTESTATION` | Server |  | `false` | Require platform attestation of the device keys | planned (V3) |
| `auth.console_session_max_age` | `BOTHY_AUTH__CONSOLE_SESSION_MAX_AGE` | Server | duration | `12h` | absolute duration of a console session | active |
| `auth.console_session_idle_timeout` | `BOTHY_AUTH__CONSOLE_SESSION_IDLE_TIMEOUT` | Server | duration | `30m` | idle time until the console session ends | active |
| `auth.local_password_enabled` | `BOTHY_AUTH__LOCAL_PASSWORD_ENABLED` | Server | bool | `true` | password with TOTP as a fallback for local accounts | active |
| `auth.totp_key_ref` | `BOTHY_AUTH__TOTP_KEY_REF` | Server | secret_ref | `file:<secrets.dir>/totp_key` | Reference to the key (32 bytes) with which TOTP secrets are encrypted | active |
| `auth.webauthn_rp_id` | `BOTHY_AUTH__WEBAUTHN_RP_ID` | Server | string | empty | RP ID of the passkeys; empty means the host of `server.public_url` | active |
| `auth.webauthn_origins` | `BOTHY_AUTH__WEBAUTHN_ORIGINS` | Server | list(string) | empty | allowed origins of the passkeys; empty means the origin of `server.public_url`; entries may also be app origins such as `android:apk-key-hash:...` | active |
| `auth.password_min_length` | `BOTHY_AUTH__PASSWORD_MIN_LENGTH` | Server | int | `12` | Minimum length of a password | active |
| `auth.argon2id_memory_kib` | `BOTHY_AUTH__ARGON2ID_MEMORY_KIB` | Server | int | `65536` | Argon2id memory (lower bound 19456) | active |
| `auth.argon2id_iterations` | `BOTHY_AUTH__ARGON2ID_ITERATIONS` | Server | int | `3` | Argon2id iterations (lower bound 2) | active |
| `auth.argon2id_parallelism` | `BOTHY_AUTH__ARGON2ID_PARALLELISM` | Server | int | `1` | Argon2id parallelism | active |
| `auth.login_fail_limit` | `BOTHY_AUTH__LOGIN_FAIL_LIMIT` | Server | int | `10` | Failed attempts per account and window | active |
| `auth.login_fail_window` | `BOTHY_AUTH__LOGIN_FAIL_WINDOW` | Server | duration | `15m` | Window for failed attempts | active |
| `auth.login_lock_duration` | `BOTHY_AUTH__LOGIN_LOCK_DURATION` | Server | duration | `15m` | Lockout of the password path | active |
| `auth.dpop_required_for` | `BOTHY_AUTH__DPOP_REQUIRED_FOR` | Server | list(string) | `["agent"]` | Token kinds that must be DPoP-bound | active |
| `auth.api_token_max_ttl` | `BOTHY_AUTH__API_TOKEN_MAX_TTL` | Server | duration | `365d` | Maximum duration of an agent or app token | active |
| `auth.personal_token_max_ttl` | `BOTHY_AUTH__PERSONAL_TOKEN_MAX_TTL` | Server | duration | `90d` | Maximum duration of a personal token | active |
| `client.app_lock_after` | `BOTHY_CLIENT__APP_LOCK_AFTER` | Server |  | `5m` | time without use until the app lock | planned (M5) |
| `security.app_lock_required` | `BOTHY_SECURITY__APP_LOCK_REQUIRED` | Server |  | `false` | Require the app lock instance-wide | planned (M5) |

### 09 Agents

Source: [09-agents.md](../spec/09-agents.md), section 21.

| Key | Environment | Place | Type | Default | Meaning | Status |
|---|---|---|---|---|---|---|
| `agent.context_max_bytes` | `BOTHY_AGENT__CONTEXT_MAX_BYTES` | Server |  | `262144` | sum of the contents in the context without the triggering message | planned (M2) |
| `agent.context_message_max_bytes` | `BOTHY_AGENT__CONTEXT_MESSAGE_MAX_BYTES` | Server |  | `32768` | content per context message, truncated above that | planned (M2) |
| `agent.jwks_max_age` | `BOTHY_AGENT__JWKS_MAX_AGE` | Server | duration | `1h` | cache duration of the JWKS; lead time of a `next` key | active |
| `signing_key.invocation_rotation_interval` | `BOTHY_SIGNING_KEY__INVOCATION_ROTATION_INTERVAL` | Server | duration | `30d` | scheduled change of the token key | active |
| `run.queued_max_age` | `BOTHY_RUN__QUEUED_MAX_AGE` | Server | duration | `48h` | the maximum age a run that has not started may reach | active |
| `run.orphan_timeout` | `BOTHY_RUN__ORPHAN_TIMEOUT` | Server | duration | `15m` | without a heartbeat during an active run: `stale` | active |
| `interaction.ack_timeout` | `BOTHY_INTERACTION__ACK_TIMEOUT` | Server |  | `30s` | after that the card shows "not acknowledged" | planned (M2) |
| `command.autocomplete_timeout` | `BOTHY_COMMAND__AUTOCOMPLETE_TIMEOUT` | Server | duration | `2500ms` | wait time for suggestions | active |
| `command.autocomplete_max_choices` | `BOTHY_COMMAND__AUTOCOMPLETE_MAX_CHOICES` | Server | int | `25` | suggestions per answer | active |
| `command.max_per_principal` | `BOTHY_COMMAND__MAX_PER_PRINCIPAL` | Server | int | `100` | commands per agent or app | active |
| `command.description_max_chars` | `BOTHY_COMMAND__DESCRIPTION_MAX_CHARS` | Server | int | `100` | length of a description | active |
| `component.max_per_message` | `BOTHY_COMPONENT__MAX_PER_MESSAGE` | Server |  | `25` | components per message | planned (M2) |
| `component.custom_id_max_chars` | `BOTHY_COMPONENT__CUSTOM_ID_MAX_CHARS` | Server |  | `100` | length of `custom_id` | planned (M2) |
| `component.form_max_fields` | `BOTHY_COMPONENT__FORM_MAX_FIELDS` | Server |  | `10` | fields per form | planned (M2) |
| `project.binding_text_max_chars` | `BOTHY_PROJECT__BINDING_TEXT_MAX_CHARS` | Server | int | `512` | longest `workspace_ref`, `repository_url`, `default_branch`, `runner_config.model` and tool name of a project binding in Unicode characters; 512 characters of at most 4 bytes in UTF-8, also after `lower()`, stay below the PostgreSQL B-tree entry limit of the unique index on `lower(workspace_ref)`, so an over-long reference is a field error, never an internal error (maximum `600`) | active |
| `thread.placeholder_titles` | `BOTHY_THREAD__PLACEHOLDER_TITLES` | Server | list(string) | `["Neuer Chat", "New chat", "Chat", "Ohne Titel", "Untitled", "Test"]` | placeholders for the title detection; the empty title is always a placeholder (12 point 2), a list entry is never empty | active |
| `adapter.invocation_token_leeway` |  | Adapter |  | `10s` | clock tolerance for `exp` and `iat`; at most the server's `auth.clock_leeway` (4.5) |  |
| `adapter.jwks_refresh_min_interval` |  | Adapter |  | `60s` | reload JWKS at most this often |  |
| `hermes_adapter.platform_key` |  | Hermes adapter |  | `bothy` | platform name in Hermes; never change after commissioning |  |
| `hermes_adapter.consumer` |  | Hermes adapter |  | `default` | consumer name at the gateway |  |
| `runner.claude_bin` |  | Runner |  | `claude` | program, searched in the cleaned `PATH` |  |
| `runner.projects_root` |  | Runner |  | no default, required | root of all project folders |  |
| `runner.state_dir` |  | Runner |  | no default, required | state, recordings, incoming files |  |
| `runner.followup_grace` |  | Runner |  | `30s` | stdin stays open for follow-ups after `result` |  |
| `runner.stop_grace` |  | Runner |  | `10s` | after SIGINT until SIGTERM |  |
| `runner.kill_grace` |  | Runner |  | `5s` | after SIGTERM until SIGKILL |  |
| `runner.session_lock_wait` |  | Runner |  | `10m` | wait time for a busy session |  |
| `runner.max_parallel_runs` |  | Runner |  | `4` | concurrent turns |  |
| `runner.stream_log_retention` |  | Runner |  | `14d` | retention of the recordings |  |
| `runner.attach_allowed_roots` |  | Runner |  | project folder, incoming folder, runner temp | allowed roots for reply attachments |  |
| `runner.attach_denied_patterns` |  | Runner |  | list in 18.10 | blocked patterns for reply attachments |  |

### 10 Backend policy

Source: [10-backend-policy.md](../spec/10-backend-policy.md), section 17.

| Key | Environment | Place | Type | Default | Meaning | Status |
|---|---|---|---|---|---|---|
| `billing.show_personal_estimates` | `BOTHY_BILLING__SHOW_PERSONAL_ESTIMATES` | Server |  | `false` | Show amounts in mode `personal` too, as information | planned (M2) |
| `billing.mismatch_alert_after` | `BOTHY_BILLING__MISMATCH_ALERT_AFTER` | Server | duration | `5m` | how long the reported mode may deviate from the configured mode before an alert goes out | active |
| `reconciler.min_request_interval` |  | Reconciliation app |  | `60s` | Interval between requests to the Cost API |  |
| `reconciler.lookback` |  | Reconciliation app |  | `3d` | this many days back are reconciled |  |
| `reconciler.workspace_map` |  | Reconciliation app |  | empty | Assignment of workspace to agent |  |

### 11 Extensions: apps, webhooks, SDK

Source: [11-extensions.md](../spec/11-extensions.md), section 16.

| Key | Environment | Place | Type | Default | Meaning | Status |
|---|---|---|---|---|---|---|
| `app.manifest_max_bytes` | `BOTHY_APP__MANIFEST_MAX_BYTES` | Server | int | `65536` | size of a manifest | active |
| `app.interaction_timeout` | `BOTHY_APP__INTERACTION_TIMEOUT` | Server |  | `3s` | deadline for synchronous responses through `interactions_url` | planned (M2) |
| `webhook.delivery_timeout` | `BOTHY_WEBHOOK__DELIVERY_TIMEOUT` | Server | duration | `15s` | timeout of a delivery | active |
| `webhook.max_pending_per_subscription` | `BOTHY_WEBHOOK__MAX_PENDING_PER_SUBSCRIPTION` | Server | int | `10000` | open deliveries per subscription, above that `paused` | active |
| `webhook.retry_max_age` | `BOTHY_WEBHOOK__RETRY_MAX_AGE` | Server | duration | `72h` | after that a delivery becomes `dead` ([webhooks.md](../api/webhooks.md), section 4) | active |
| `webhook.secret_rotation_overlap` | `BOTHY_WEBHOOK__SECRET_ROTATION_OVERLAP` | Server | duration | `24h` | how long the server signs with the old and the new HMAC generation | active |
| `webhook.hmac_master_key_ref` | `BOTHY_WEBHOOK__HMAC_MASTER_KEY_REF` | Server | secret_ref | `file:<secrets.dir>/webhook_hmac_master_key` | reference to the master key from which the HMAC secrets of the subscriptions are derived ([webhooks.md](../api/webhooks.md), section 2.1) | active |
| `webhook.receiver_tolerance` |  | Receiver |  | `5m` | recommendation, not a server value: tolerance for `webhook-timestamp` at the receiver and default of the check function of the SDKs |  |
| `webhook_in.max_body_bytes` | `BOTHY_WEBHOOK_IN__MAX_BODY_BYTES` | Server | int | `1048576` | size of an incoming body | active |
| `webhook_in.formats_enabled` | `BOTHY_WEBHOOK_IN__FORMATS_ENABLED` | Server | list(string) | `["native", "alertmanager"]` | enabled incoming formats; the start check `webhook_in_formats` allows only `native` and `alertmanager`, each at most once; an empty list switches every format off | active |
| `sdk.http_timeout` |  | SDK |  | `30s` | timeout of a REST request |  |
| `sdk.retry_max_attempts` |  | SDK |  | `5` | attempts per idempotent operation |  |
| `sdk.retry_backoff_base` |  | SDK |  | `500ms` | base of the backoff |  |
| `sdk.retry_backoff_cap` |  | SDK |  | `30s` | largest interval |  |

### 12 Files

Source: [12-files.md](../spec/12-files.md), section 18.

| Key | Environment | Place | Type | Default | Meaning | Status |
|---|---|---|---|---|---|---|
| `file.storage.driver` | `BOTHY_FILE__STORAGE__DRIVER` | Server | enum(fs, s3) | `fs` | `fs` or `s3` | active |
| `file.fs.root` | `BOTHY_FILE__FS__ROOT` | Server | string | `/var/lib/bothy/files` | root of the file system driver (path in the container) | active |
| `file.fs.min_free_bytes` | `BOTHY_FILE__FS__MIN_FREE_BYTES` | Server | int | `2147483648` | free space below which uploads are rejected with `503 unavailable` (`reason` `storage_full`) | active |
| `file.s3.endpoint` | `BOTHY_FILE__S3__ENDPOINT` | Server | string | empty | S3 endpoint | active |
| `file.s3.region` | `BOTHY_FILE__S3__REGION` | Server | string | `us-east-1` | region for the signature | active |
| `file.s3.bucket` | `BOTHY_FILE__S3__BUCKET` | Server | string | empty | bucket | active |
| `file.s3.prefix` | `BOTHY_FILE__S3__PREFIX` | Server | string | empty | prefix before all keys | active |
| `file.s3.use_path_style` | `BOTHY_FILE__S3__USE_PATH_STYLE` | Server | bool | `true` | path style instead of virtual hosts | active |
| `file.s3.checksum_mode` | `BOTHY_FILE__S3__CHECKSUM_MODE` | Server | enum(when_required, when_supported) | `when_required` | SDK checksums for request and response: `when_required` or `when_supported` (only with an `https` endpoint) | active |
| `file.s3.access_key_ref` | `BOTHY_FILE__S3__ACCESS_KEY_REF` | Server | secret_ref | `file:<secrets.dir>/s3_access_key` | reference to the secret provider; read only with `file.storage.driver = s3` | active |
| `file.s3.secret_key_ref` | `BOTHY_FILE__S3__SECRET_KEY_REF` | Server | secret_ref | `file:<secrets.dir>/s3_secret_key` | reference to the secret provider; read only with `file.storage.driver = s3` | active |
| `file.s3.presigned_upload` | `BOTHY_FILE__S3__PRESIGNED_UPLOAD` | Server | bool | `false` | allow mode `presigned` | active |
| `file.s3.presign_ttl` | `BOTHY_FILE__S3__PRESIGN_TTL` | Server |  | `15m` | validity of a presigned URL | planned (M3) |
| `file.upload_part_bytes` | `BOTHY_FILE__UPLOAD_PART_BYTES` | Server | int | `8388608` | part size | active |
| `file.upload_session_ttl` | `BOTHY_FILE__UPLOAD_SESSION_TTL` | Server | duration | `24h` | lifetime of an open session | active |
| `file.upload_max_open_sessions` | `BOTHY_FILE__UPLOAD_MAX_OPEN_SESSIONS` | Server | int | `20` | open sessions per principal | active |
| `file.quota_bytes_per_principal` | `BOTHY_FILE__QUOTA_BYTES_PER_PRINCIPAL` | Server | int | `0` | quota per principal, 0 = off | active |
| `file.poster_max_bytes` | `BOTHY_FILE__POSTER_MAX_BYTES` | Server |  | `524288` | size of the video thumbnail from the client | planned (M3) |
| `file.sniff_bytes` | `BOTHY_FILE__SNIFF_BYTES` | Server | int | `1048576` | bytes for type detection | active |
| `file.denied_mime_types` | `BOTHY_FILE__DENIED_MIME_TYPES` | Server | list(string) | empty | blocked types | active |
| `file.name_max_bytes` | `BOTHY_FILE__NAME_MAX_BYTES` | Server | int | `255` | length of a file name in UTF-8 bytes | active |
| `file.scan.mode` | `BOTHY_FILE__SCAN__MODE` | Server | enum(required, off) | `required` | `required` or `off` | active |
| `file.scan.clamd_address` | `BOTHY_FILE__SCAN__CLAMD_ADDRESS` | Server | string | `unix:///run/clamav/clamd.sock` | Unix socket of clamd, `unix://` plus an absolute path; no other transport (19 SEC-TLS-5) | active |
| `file.scan.chunk_bytes` | `BOTHY_FILE__SCAN__CHUNK_BYTES` | Server | int | `65536` | chunk size for `INSTREAM` | active |
| `file.scan.timeout` | `BOTHY_FILE__SCAN__TIMEOUT` | Server | duration | `120s` | time limit per scan | active |
| `file.scan.max_concurrent` | `BOTHY_FILE__SCAN__MAX_CONCURRENT` | Server | int | `4` | scans per worker process | active |
| `file.scan.retry_backoff_cap` | `BOTHY_FILE__SCAN__RETRY_BACKOFF_CAP` | Server | duration | `5m` | largest interval between scan attempts | active |
| `file.scan.health_interval` | `BOTHY_FILE__SCAN__HEALTH_INTERVAL` | Server | duration | `30s` | `PING` interval | active |
| `file.scan.pending_alert_after` | `BOTHY_FILE__SCAN__PENDING_ALERT_AFTER` | Server |  | `10m` | age of the oldest `pending` file before the alert | planned (M3) |
| `file.scan.clamd_stream_max_bytes` | `BOTHY_FILE__SCAN__CLAMD_STREAM_MAX_BYTES` | Server | int | `104857600` | mirror of clamd `StreamMaxLength` | active |
| `file.invocation_hold_max` | `BOTHY_FILE__INVOCATION_HOLD_MAX` | Server |  | `120s` | longest wait of the agent invocation for attachments | planned (M3) |
| `file.image.libvips_enabled` | `BOTHY_FILE__IMAGE__LIBVIPS_ENABLED` | Server |  | `false` | use the libvips build | planned (M3) |
| `file.image.max_pixels` | `BOTHY_FILE__IMAGE__MAX_PIXELS` | Server | int | `50000000` | largest decoded pixel count | active |
| `file.image.jpeg_quality` | `BOTHY_FILE__IMAGE__JPEG_QUALITY` | Server | int | `90` | quality when converting still WebP images without alpha to JPEG | active |
| `file.image.processing_timeout` | `BOTHY_FILE__IMAGE__PROCESSING_TIMEOUT` | Server | duration | `60s` | time limit of an image job | active |
| `file.image.max_concurrent` | `BOTHY_FILE__IMAGE__MAX_CONCURRENT` | Server | int | `2` | image jobs per worker process | active |
| `file.thumbnail.small_px` | `BOTHY_FILE__THUMBNAIL__SMALL_PX` | Server | int | `320` | longest edge `small` | active |
| `file.thumbnail.medium_px` | `BOTHY_FILE__THUMBNAIL__MEDIUM_PX` | Server | int | `1024` | longest edge `medium` | active |
| `file.thumbnail.large_px` | `BOTHY_FILE__THUMBNAIL__LARGE_PX` | Server | int | `2048` | longest edge `large` | active |
| `file.thumbnail.jpeg_quality` | `BOTHY_FILE__THUMBNAIL__JPEG_QUALITY` | Server | int | `82` | quality of the thumbnails | active |
| `file.ticket_ttl` | `BOTHY_FILE__TICKET_TTL` | Server | duration | `5m` | validity of a file ticket | active |
| `file.ticket_key_ref` | `BOTHY_FILE__TICKET_KEY_REF` | Server | secret_ref | `file:<secrets.dir>/file_ticket_key` | reference to the ticket key in the secret provider | active |
| `file.not_ready_retry_after` | `BOTHY_FILE__NOT_READY_RETRY_AFTER` | Server | duration | `5s` | `Retry-After` of the `409 file_not_ready` responses (in whole seconds) | active |
| `file.unattached_ttl` | `BOTHY_FILE__UNATTACHED_TTL` | Server |  | `24h` | period for never attached files | planned (M3) |
| `file.failed_row_ttl` | `BOTHY_FILE__FAILED_ROW_TTL` | Server |  | `7d` | period for rows of failed or blocked files | planned (M3) |
| `file.gc_interval` | `BOTHY_FILE__GC_INTERVAL` | Server |  | `10m` | interval of the cleanup job | planned (M3) |
| `voice.max_duration` | `BOTHY_VOICE__MAX_DURATION` | Server | duration | `15m` | longest voice message | active |
| `voice.transcription_mode` | `BOTHY_VOICE__TRANSCRIPTION_MODE` | Server |  | `agent` | `off`, `agent` or `service` | planned (M3) |
| `voice.transcription_url` | `BOTHY_VOICE__TRANSCRIPTION_URL` | Server |  | empty | endpoint of the transcription service | planned (M3) |
| `voice.transcription_api_key_ref` | `BOTHY_VOICE__TRANSCRIPTION_API_KEY_REF` | Server |  | empty | reference to the key | planned (M3) |
| `voice.transcription_timeout` | `BOTHY_VOICE__TRANSCRIPTION_TIMEOUT` | Server |  | `120s` | time limit of a transcription | planned (M3) |
| `voice.transcript_hold_max` | `BOTHY_VOICE__TRANSCRIPT_HOLD_MAX` | Server |  | `60s` | longest wait of the agent invocation for the transcript | planned (M3) |

### 13 Push and notifications

Source: [13-push-notifications.md](../spec/13-push-notifications.md), section 17.

| Key | Environment | Place | Type | Default | Meaning | Status |
|---|---|---|---|---|---|---|
| `push.ttl` | `BOTHY_PUSH__TTL` | Server |  | `24h` | lifetime at the provider (`apns-expiration`, `android.ttl`) | planned (M3) |
| `push.max_age` | `BOTHY_PUSH__MAX_AGE` | Server | duration | `1h` | a notification not yet sent that is older than this is discarded | active |
| `push.retry_base` | `BOTHY_PUSH__RETRY_BASE` | Server | duration | `2s` | base of the backoff | active |
| `push.retry_cap` | `BOTHY_PUSH__RETRY_CAP` | Server | duration | `5m` | largest backoff interval | active |
| `push.max_concurrent_requests` | `BOTHY_PUSH__MAX_CONCURRENT_REQUESTS` | Server | int | `32` | concurrent requests to providers per process | active |
| `push.apns.team_id` | `BOTHY_PUSH__APNS__TEAM_ID` | Server |  | empty | Apple team ID | planned (M3) |
| `push.apns.key_id` | `BOTHY_PUSH__APNS__KEY_ID` | Server |  | empty | key ID of the `.p8` key | planned (M3) |
| `push.apns.private_key_ref` | `BOTHY_PUSH__APNS__PRIVATE_KEY_REF` | Server |  | empty | reference to the key in the secret provider | planned (M3) |
| `push.apns.topics` | `BOTHY_PUSH__APNS__TOPICS` | Server |  | empty | allowed bundle IDs | planned (M3) |
| `push.apns.jwt_refresh_interval` | `BOTHY_PUSH__APNS__JWT_REFRESH_INTERVAL` | Server |  | `40m` | renewal of the provider JWT, must be between 20 and 60 minutes | planned (M3) |
| `push.apns.retry_5xx_delay` | `BOTHY_PUSH__APNS__RETRY_5XX_DELAY` | Server |  | `15m` | waiting time after `500` or `503` | planned (M3) |
| `push.apns.time_sensitive_enabled` | `BOTHY_PUSH__APNS__TIME_SENSITIVE_ENABLED` | Server |  | `false` | `time-sensitive` for `input_needed` | planned (M3) |
| `push.fcm.project_id` | `BOTHY_PUSH__FCM__PROJECT_ID` | Server |  | empty | Firebase project | planned (M3) |
| `push.fcm.service_account_ref` | `BOTHY_PUSH__FCM__SERVICE_ACCOUNT_REF` | Server |  | empty | reference to the service account key | planned (M3) |
| `notify.user_default_level` | `BOTHY_NOTIFY__USER_DEFAULT_LEVEL` | Server | enum(all, mentions, mute) | `mentions` | level without any setting | active |
| `notify.default_preview_mode` | `BOTHY_NOTIFY__DEFAULT_PREVIEW_MODE` | Server |  | `full` | default for `notify.preview_mode` | planned (M3) |
| `notify.default_input_needed_breaks_quiet_hours` | `BOTHY_NOTIFY__DEFAULT_INPUT_NEEDED_BREAKS_QUIET_HOURS` | Server | bool | `false` | default for the switch of the same name | active |
| `notify.default_admin_enabled` | `BOTHY_NOTIFY__DEFAULT_ADMIN_ENABLED` | Server | bool | `true` | default for `notify.admin_enabled` | active |
| `notify.coalesce_window` | `BOTHY_NOTIFY__COALESCE_WINDOW` | Server | duration | `60s` | coalescing window per recipient and thread | active |
| `notify.mobile_delay_when_desktop_active` | `BOTHY_NOTIFY__MOBILE_DELAY_WHEN_DESKTOP_ACTIVE` | Server | duration | `60s` | delay of mobile pushes while the desktop is active | active |
| `notify.preview_max_chars` | `BOTHY_NOTIFY__PREVIEW_MAX_CHARS` | Server |  | `240` | length of the excerpt in the preview | planned (M3) |
| `notify.badge_sync_min_interval` | `BOTHY_NOTIFY__BADGE_SYNC_MIN_INTERVAL` | Server | duration | `60s` | smallest interval between badge syncs per person | active |

### 14 Search

Source: [14-search.md](../spec/14-search.md), section 15.

| Key | Environment | Place | Type | Default | Meaning | Status |
|---|---|---|---|---|---|---|
| `search.languages` | `BOTHY_SEARCH__LANGUAGES` | Server | list(string) | `["german","english"]` | configurations for query construction (plus always `simple`) | active |
| `search.locale_map` | `BOTHY_SEARCH__LOCALE_MAP` | Server | map(string) | `{"de":"german","en":"english"}` | language tag to text search configuration | active |
| `search.query_max_chars` | `BOTHY_SEARCH__QUERY_MAX_CHARS` | Server |  | `256` | length of the input | planned (M3) |
| `search.max_candidates` | `BOTHY_SEARCH__MAX_CANDIDATES` | Server |  | `2000` | candidates before ranking | planned (M3) |
| `search.rank_normalization` | `BOTHY_SEARCH__RANK_NORMALIZATION` | Server |  | `1` | normalization for `ts_rank` | planned (M3) |
| `search.recency_scale` | `BOTHY_SEARCH__RECENCY_SCALE` | Server |  | `30d` | age at which the score falls to half | planned (M3) |
| `search.page_size_default` | `BOTHY_SEARCH__PAGE_SIZE_DEFAULT` | Server |  | `20` | hits per page | planned (M3) |
| `search.page_size_max` | `BOTHY_SEARCH__PAGE_SIZE_MAX` | Server |  | `50` | maximum hits per page | planned (M3) |
| `search.headline_input_max_chars` | `BOTHY_SEARCH__HEADLINE_INPUT_MAX_CHARS` | Server |  | `20000` | length of the text for `ts_headline` | planned (M3) |
| `search.headline_max_fragments` | `BOTHY_SEARCH__HEADLINE_MAX_FRAGMENTS` | Server |  | `2` | snippets per hit | planned (M3) |
| `search.headline_max_words` | `BOTHY_SEARCH__HEADLINE_MAX_WORDS` | Server |  | `30` | words per snippet, at most | planned (M3) |
| `search.headline_min_words` | `BOTHY_SEARCH__HEADLINE_MIN_WORDS` | Server |  | `10` | words per snippet, at least | planned (M3) |
| `search.prefix_min_chars` | `BOTHY_SEARCH__PREFIX_MIN_CHARS` | Server |  | `2` | shortest word length in the prefix search | planned (M3) |
| `search.statement_timeout` | `BOTHY_SEARCH__STATEMENT_TIMEOUT` | Server |  | `3s` | time limit of a search query | planned (M3) |
| `search.p95_target` | `BOTHY_SEARCH__P95_TARGET` | Server |  | `500ms` | p95 target in the load test | planned (M3) |
| `search.prefix_p95_target` | `BOTHY_SEARCH__PREFIX_P95_TARGET` | Server |  | `100ms` | p95 target of the prefix search | planned (M3) |
| `search.benchmark_messages` | `BOTHY_SEARCH__BENCHMARK_MESSAGES` | Server |  | `2000000` | corpus size in the load test | planned (M3) |
| `search.extract.max_input_bytes` | `BOTHY_SEARCH__EXTRACT__MAX_INPUT_BYTES` | Server |  | `52428800` | largest file for the extraction | planned (M3) |
| `search.extract.max_text_bytes` | `BOTHY_SEARCH__EXTRACT__MAX_TEXT_BYTES` | Server |  | `1048576` | largest stored text | planned (M3) |
| `search.extract.max_uncompressed_bytes` | `BOTHY_SEARCH__EXTRACT__MAX_UNCOMPRESSED_BYTES` | Server |  | `209715200` | largest unpacked amount for archives | planned (M3) |
| `search.extract.max_archive_entries` | `BOTHY_SEARCH__EXTRACT__MAX_ARCHIVE_ENTRIES` | Server |  | `10000` | entries per archive | planned (M3) |
| `search.extract.timeout` | `BOTHY_SEARCH__EXTRACT__TIMEOUT` | Server |  | `60s` | time limit per extraction | planned (M3) |
| `search.extract.tika_url` | `BOTHY_SEARCH__EXTRACT__TIKA_URL` | Server |  | empty | Tika server; empty = off | planned (V3) |

### 15 Client architecture

Source: [15-client-architecture.md](../spec/15-client-architecture.md), section 22.

| Key | Environment | Place | Type | Default | Meaning | Status |
|---|---|---|---|---|---|---|
| `client.cache_max_bytes` |  | App |  | `1GiB` | upper limit for the local database plus file cache; above it the oldest entries are removed |  |
| `client.sync.parse_offload_bytes` |  | App |  | `262144` | from this page size on, decoding happens in the background isolate |  |
| `client.sync.connect_timeout` |  | App |  | `30s` | deadline of one connection attempt from its start in `Connecting`, before the socket opens, to `ready` or `resumed`; it also covers a connect that never completes; when it passes, the attempt ends and the backoff applies (7.2) |  |
| `client.outbox_stuck_after` |  | App |  | `2m` | from then on a waiting entry shows "Sending delayed" |  |
| `client.push.fetch_timeout` |  | App |  | `20s` | time that the NSE or the Android service uses to fetch the text |  |
| `client.voice.bitrate` |  | App |  | `64kbps` | bitrate of the AAC recording |  |
| `client.metrics.flush_interval` |  | App |  | `5m` | the client sends metrics at most this often |  |
| `client.perf.cold_start_p95` |  | App |  | `2500ms` | target for cold start |  |
| `client.perf.thread_open_p95` |  | App |  | `300ms` | target for opening a thread from the cache |  |
| `client.perf.send_local_echo_p95` |  | App |  | `50ms` | target for the local display on sending |  |
| `client.perf.patch_to_frame_p95` |  | App |  | `100ms` | target for live box patch to frame |  |
| `client.perf.jank_ratio_max` |  | App |  | `0.01` | highest share of frames over budget |  |
| `client.perf.catchup_events_per_s_min` |  | App |  | `2000` | minimum rate during catch-up |  |
| `client.perf.memory_mobile_max` |  | App |  | `300MiB` | memory on mobile |  |
| `client.perf.memory_desktop_max` |  | App |  | `500MiB` | memory on desktop |  |
| `client.perf.terminal_frame_to_paint_p95` |  | App |  | `16ms` | target for terminal frame until drawn |  |
| `client.perf.terminal_throughput_min` |  | App |  | `1MiB/s` | minimum rate of the terminal view during continuous output |  |
| `client.terminal.scrollback_lines` |  | App |  | `10000` | lines in the scrollback of the renderer in memory; the buffer for reattaching is in the host service (`host_service.scrollback_bytes`) |  |
| `client.terminal.font_size_default` |  | App |  | `14` | font size of the terminal view in logical pixels until the person changes it |  |

### 16 Operations

Source: [16-operations.md](../spec/16-operations.md), section 14.

| Key | Environment | Place | Type | Default | Meaning | Status |
|---|---|---|---|---|---|---|
| `instance.environment` | `BOTHY_INSTANCE__ENVIRONMENT` | Server | enum(dev, staging, prod) | `prod` | `dev`, `staging` or `prod` (6.1) | active |
| `server.role` | `BOTHY_SERVER__ROLE` | Server | enum(api, worker, all) | `all` | `api`, `worker` or `all` | active |
| `server.listen_addr` | `BOTHY_SERVER__LISTEN_ADDR` | Server | listen address | `127.0.0.1:8080` | public listener (REST, gateway) | active |
| `server.internal_listen_addr` | `BOTHY_SERVER__INTERNAL_LISTEN_ADDR` | Server | listen address | `127.0.0.1:8081` | health, readiness, incoming webhooks from the stack | active |
| `server.public_url` | `BOTHY_SERVER__PUBLIC_URL` | Server | url | no default, required | Origin under which apps reach the server (scheme, lower-case host, optional port other than the default of the scheme, as browsers send an origin; without path, query and fragment, otherwise `bothy config check` fails); basis of `aud`, DPoP `htu` and WebAuthn ([08-identity.md](../spec/08-identity.md), sections 7.2 and 7.3) | active |
| `server.trusted_proxies` | `BOTHY_SERVER__TRUSTED_PROXIES` | Server | list(cidr) | `127.0.0.1/32, ::1/128` | peers whose forwarding and identity headers are honored and that may send plain text ([19-security.md](../spec/19-security.md), SEC-TLS-1) | active |
| `server.maintenance_mode` | `BOTHY_SERVER__MAINTENANCE_MODE` | Server |  | `off` | `off` or `read_only` (relocation, upgrade; [ADR 0065](../adr/0065-maintenance-mode-read-only.md)) | planned (M7) |
| `secrets.dir` | `BOTHY_SECRETS__DIR` | Server | string | `/run/bothy/secrets` | base of the default references `file:`; an absolute path in clean form (no `.` or `..`, no double or trailing `/`), not `/`, without a NUL character and without `<`, otherwise `bothy config check` fails | active |
| `secrets.cache_ttl` | `BOTHY_SECRETS__CACHE_TTL` | Server | duration | `5m` | how long a read secret stays valid; at least `1s` | active |
| `secrets.signing_key_dir` | `BOTHY_SECRETS__SIGNING_KEY_DIR` | Server | string | `/var/lib/bothy/signing-keys` | writable directory of the private signing keys; owned by the service user, mode `0700` (5.2, [ADR 0066](../adr/0066-signing-keys-in-the-writable-key-store.md)) | active |
| `console.assets_dir` | `BOTHY_CONSOLE__ASSETS_DIR` | Server | string | empty | directory of the built web console (`flutter build web` of `apps/console`) that the api process serves under `/console/` ([02-architecture.md](../spec/02-architecture.md), 3.1); empty: no console, `404` under `/console/`; the image sets `/usr/share/bothy/console`; a set directory without `index.html` ends the start | active |
| `console.csp_report_max_records` | `BOTHY_CONSOLE__CSP_REPORT_MAX_RECORDS` | Server | int | `20` | most log records one request of `submitCspReport` writes (1 to 1000): identical violations share one record, every violation is counted in `bothy.console.csp_violations`; the rate class `rl.public_auth` bounds the requests ([19-security.md](../spec/19-security.md) 5 rule 6, M6-03) | active |
| `server.tls_cert_ref` | `BOTHY_SERVER__TLS_CERT_REF` | Server | secret_ref | empty | reference to the certificate chain (PEM, leaf first); empty: the server does not terminate TLS (SEC-TLS-2) | active |
| `server.tls_key_ref` | `BOTHY_SERVER__TLS_KEY_REF` | Server | secret_ref | empty | reference to the private key (PEM); only together with `server.tls_cert_ref` | active |
| `database.host` | `BOTHY_DATABASE__HOST` | Server | string | `localhost` | host of the database | active |
| `database.port` | `BOTHY_DATABASE__PORT` | Server | int | `5432` | port of the database in the server's network | active |
| `database.name` | `BOTHY_DATABASE__NAME` | Server | string | `bothy` | database name | active |
| `database.user` | `BOTHY_DATABASE__USER` | Server | string | `bothy_app` | role of the server | active |
| `database.password_ref` | `BOTHY_DATABASE__PASSWORD_REF` | Server | secret_ref | `file:<secrets.dir>/db_app_password` | reference to the password | active |
| `database.sslmode` | `BOTHY_DATABASE__SSLMODE` | Server | enum(disable, verify-full) | `verify-full` | TLS to the database; in the internal Compose network `disable` (SEC-TLS-5) | active |
| `database.sslrootcert` | `BOTHY_DATABASE__SSLROOTCERT` | Server | string | empty | CA bundle for `verify-full`; empty uses the system roots | active |
| `database.max_conns` | `BOTHY_DATABASE__MAX_CONNS` | Server | int | `20` | size of the connection pool per process (start value until the load test) | active |
| `database.restore_guard` | `BOTHY_DATABASE__RESTORE_GUARD` | Server | enum(on, off) | `on` | `on` or `off`: with `on` a server whose database identity differs from the stored one stays not ready (`restore_not_finalized`) until `bothy restore finalize`; `off` only where a failover loses no committed transaction (8.4, R-DM-50) | active |
| `migrate.require_recent_backup` | `BOTHY_MIGRATE__REQUIRE_RECENT_BACKUP` | Server | bool | `true` | `migrate` requires a fresh backup | active |
| `migrate.recent_backup_max_age` | `BOTHY_MIGRATE__RECENT_BACKUP_MAX_AGE` | Server | duration | `24h` | how old this backup may be | active |
| `migrate.lock_timeout` | `BOTHY_MIGRATE__LOCK_TIMEOUT` | Server | duration | `5s` | `lock_timeout` per migration step | active |
| `migrate.bootstrap_user` | `BOTHY_MIGRATE__BOOTSTRAP_USER` | Server | string | `postgres` | superuser login only for `migrate up --bootstrap` | active |
| `migrate.bootstrap_password_ref` | `BOTHY_MIGRATE__BOOTSTRAP_PASSWORD_REF` | Server | secret_ref | `file:<secrets.dir>/db_superuser_password` | its password | active |
| `migrate.owner_password_ref` | `BOTHY_MIGRATE__OWNER_PASSWORD_REF` | Server | secret_ref | `file:<secrets.dir>/db_owner_password` | password that `--bootstrap` sets for `bothy_owner` | active |
| `migrate.app_password_ref` | `BOTHY_MIGRATE__APP_PASSWORD_REF` | Server | secret_ref | `file:<secrets.dir>/db_app_password` | password for `bothy_app` | active |
| `migrate.maint_password_ref` | `BOTHY_MIGRATE__MAINT_PASSWORD_REF` | Server | secret_ref | `file:<secrets.dir>/db_maint_password` | password for `bothy_maint` | active |
| `migrate.monitor_password_ref` | `BOTHY_MIGRATE__MONITOR_PASSWORD_REF` | Server | secret_ref | `file:<secrets.dir>/db_monitor_password` | password for `bothy_monitor` | active |
| `migrate.backup_password_ref` | `BOTHY_MIGRATE__BACKUP_PASSWORD_REF` | Server | secret_ref | `file:<secrets.dir>/db_backup_password` | password for `bothy_backup` (M7-02) | active |
| `backup.pg.full_schedule` | `BOTHY_BACKUP__PG__FULL_SCHEDULE` | Server | string | `0 2 * * 0` | schedule of the full backup (cron) | active |
| `backup.pg.diff_schedule` | `BOTHY_BACKUP__PG__DIFF_SCHEDULE` | Server | string | `0 2 * * 1-6` | schedule of the differential backup (cron) | active |
| `backup.pg.archive_timeout` | `BOTHY_BACKUP__PG__ARCHIVE_TIMEOUT` | Server |  | `60s` | a WAL segment is archived at the latest after this time | planned (M7) |
| `backup.pg.retention_full` | `BOTHY_BACKUP__PG__RETENTION_FULL` | Server | int | `4` | full backups kept in repo2 | active |
| `backup.pg.local_retention_full` | `BOTHY_BACKUP__PG__LOCAL_RETENTION_FULL` | Server | int | `1` | full backups kept in repo1 | active |
| `backup.logical.schedule` | `BOTHY_BACKUP__LOGICAL__SCHEDULE` | Server | string | `0 3 * * *` | schedule of `pg_dump` (cron) | active |
| `backup.logical.retention` | `BOTHY_BACKUP__LOGICAL__RETENTION` | Server | duration | `30d` | retention of the logical backup | active |
| `backup.objects.schedule` | `BOTHY_BACKUP__OBJECTS__SCHEDULE` | Server | string | `30 3 * * *` | schedule of the object backup (cron) | active |
| `backup.objects.retention` | `BOTHY_BACKUP__OBJECTS__RETENTION` | Server | duration | `90d` | retention of the object snapshots | active |
| `backup.offsite.target` | `BOTHY_BACKUP__OFFSITE__TARGET` | Server | string | empty | off-site target; empty means local only (warning in console and status) | active |
| `backup.offsite.region` | `BOTHY_BACKUP__OFFSITE__REGION` | Server | string | `us-east-1` | region of the off-site bucket (signing of S3 requests) | active |
| `backup.offsite.use_path_style` | `BOTHY_BACKUP__OFFSITE__USE_PATH_STYLE` | Server | bool | `true` | path-style addressing of the off-site bucket (`https://<host>/<bucket>`); `false` uses virtual hosts | active |
| `backup.max_age` | `BOTHY_BACKUP__MAX_AGE` | Server |  | `26h` | no successful backup for longer than this: alert | planned (M7) |
| `backup.restore_test.schedule` | `BOTHY_BACKUP__RESTORE_TEST__SCHEDULE` | Server | string | `0 5 * * 6` | schedule of the restore test (cron) | active |
| `backup.restore_test.sample_files` | `BOTHY_BACKUP__RESTORE_TEST__SAMPLE_FILES` | Server | int | `200` | files checked per test | active |
| `backup.restore_test.sample_threads` | `BOTHY_BACKUP__RESTORE_TEST__SAMPLE_THREADS` | Server | int | `200` | threads checked per test | active |
| `backup.restore_test.max_age` | `BOTHY_BACKUP__RESTORE_TEST__MAX_AGE` | Server |  | `8d` | no successful test for longer than this: alert | planned (M7) |
| `dr.rpo_target` | `BOTHY_DR__RPO_TARGET` | Server |  | `5m` | target for the largest data loss | planned (M7) |
| `dr.rto_target` | `BOTHY_DR__RTO_TARGET` | Server | duration | `4h` | target for the duration of the restore | active |
| `status.check_interval` | `BOTHY_STATUS__CHECK_INTERVAL` | Server |  | `30s` | interval of the automatic checks | planned (M7) |
| `status.degrade_after` | `BOTHY_STATUS__DEGRADE_AFTER` | Server |  | `2m` | how long errors last until `degraded` | planned (M7) |
| `status.incident_after` | `BOTHY_STATUS__INCIDENT_AFTER` | Server |  | `5m` | how long `major_outage` lasts until the automatic incident | planned (M7) |
| `status.resolve_after` | `BOTHY_STATUS__RESOLVE_AFTER` | Server |  | `10m` | how long the state stays healthy until the automatic resolution | planned (M7) |
| `status.error_ratio_degraded` | `BOTHY_STATUS__ERROR_RATIO_DEGRADED` | Server |  | `0.05` | error ratio from which a check fails | planned (M7) |
| `status.error_ratio_outage` | `BOTHY_STATUS__ERROR_RATIO_OUTAGE` | Server |  | `0.5` | error ratio from which `major_outage` applies | planned (M7) |
| `status.fanout_lag_degraded` | `BOTHY_STATUS__FANOUT_LAG_DEGRADED` | Server |  | `5s` | fan-out lag from which `sync.fanout` fails | planned (M7) |
| `install.ready_timeout` |  | Installation script |  | `5m` | how long `install.sh` waits for `/readyz` after `docker compose up -d` (option `--ready-timeout`) |  |

### 17 Observability

Source: [17-observability.md](../spec/17-observability.md), section 11.

| Key | Environment | Place | Type | Default | Meaning | Status |
|---|---|---|---|---|---|---|
| `telemetry.enabled` | `BOTHY_TELEMETRY__ENABLED` | Server | bool | `true` | telemetry on or off | active |
| `telemetry.traces.sampler_ratio` | `BOTHY_TELEMETRY__TRACES__SAMPLER_RATIO` | Server | float | `1.0` | share of the traces that the SDK records; the collector then samples again | active |
| `telemetry.traces.trust_remote_parent` | `BOTHY_TELEMETRY__TRACES__TRUST_REMOTE_PARENT` | Server | bool | `false` | `true`: an incoming `traceparent` becomes the parent context (its trace ID becomes `correlation_id`, its sampling flag applies); `false`: span link only, own trace | active |
| `telemetry.logs.exporter` | `BOTHY_TELEMETRY__LOGS__EXPORTER` | Server | enum(otlp, stdout, both) | `both` | `otlp`, `stdout` or `both` | active |
| `telemetry.logs.level` | `BOTHY_TELEMETRY__LOGS__LEVEL` | Server | enum(debug, info, warn, error) | `info` | lowest level that is written | active |
| `telemetry.pseudonym_key_ref` | `BOTHY_TELEMETRY__PSEUDONYM_KEY_REF` | Server | secret_ref | `file:<secrets.dir>/telemetry_pseudonym_key` | key for HMAC pseudonyms; at least `otel.MinKeyBytes` bytes | active |
| `telemetry.dashboard_url` | `BOTHY_TELEMETRY__DASHBOARD_URL` | Server |  | empty | link from the admin console to the telemetry backend | planned (M6) |
| `stats.cache_ttl` | `BOTHY_STATS__CACHE_TTL` | Server |  | `5m` | how long a computed statistics result stays valid | planned (M6) |
| `stats.max_range` | `BOTHY_STATS__MAX_RANGE` | Server |  | `400d` | largest queryable time range | planned (M6) |
| `stats.time_zone` | `BOTHY_STATS__TIME_ZONE` | Server |  | `UTC` | IANA time zone of the day boundaries in statistics, daily aggregates and timeline ([admin-statistics.md](../observability/admin-statistics.md), section 5) | planned (M6) |

### 18 Release

Source: [18-release.md](../spec/18-release.md), section 12.

| Key | Environment | Place | Type | Default | Meaning | Status |
|---|---|---|---|---|---|---|
| `client.min_version.ios` | `BOTHY_CLIENT__MIN_VERSION__IOS` | Server | string | empty | smallest allowed app version on iOS; empty means no limit | active |
| `client.min_version.android` | `BOTHY_CLIENT__MIN_VERSION__ANDROID` | Server | string | empty | the same for Android | active |
| `client.min_version.macos` | `BOTHY_CLIENT__MIN_VERSION__MACOS` | Server | string | empty | the same for macOS | active |
| `client.min_version.windows` | `BOTHY_CLIENT__MIN_VERSION__WINDOWS` | Server | string | empty | the same for Windows | active |
| `client.recommended_version.ios` | `BOTHY_CLIENT__RECOMMENDED_VERSION__IOS` | Server | string | empty | below it the app shows an update notice | active |
| `client.recommended_version.android` | `BOTHY_CLIENT__RECOMMENDED_VERSION__ANDROID` | Server | string | empty | the same for Android | active |
| `client.recommended_version.macos` | `BOTHY_CLIENT__RECOMMENDED_VERSION__MACOS` | Server | string | empty | the same for macOS | active |
| `client.recommended_version.windows` | `BOTHY_CLIENT__RECOMMENDED_VERSION__WINDOWS` | Server | string | empty | the same for Windows | active |

### 19 Security

Source: [19-security.md](../spec/19-security.md), section 16.

| Key | Environment | Place | Type | Default | Meaning | Status |
|---|---|---|---|---|---|---|
| `http.hsts_max_age` | `BOTHY_HTTP__HSTS_MAX_AGE` | Server | duration | `31536000s` | `max-age` for HSTS | active |
| `egress.allowed_hosts` | `BOTHY_EGRESS__ALLOWED_HOSTS` | Server | list(string) | empty | additional allowed targets, form `host[:port][;allow_private]` ([configuration.md](../operations/configuration.md), section 2); duplicate entries are a load error | active |
| `egress.timeout` | `BOTHY_EGRESS__TIMEOUT` | Server | duration | `15s` | time limit of an outgoing request | active |
| `egress.max_response_bytes` | `BOTHY_EGRESS__MAX_RESPONSE_BYTES` | Server | int | `1048576` | largest response of an outgoing request | active |
| `audit.checkpoint_interval` | `BOTHY_AUDIT__CHECKPOINT_INTERVAL` | Server | duration | `1h` | interval of the signed checkpoints of the audit log | active |
| `audit.checkpoint_sink` | `BOTHY_AUDIT__CHECKPOINT_SINK` | Server | string | empty | storage target of the checkpoints outside the database: empty (warning, SEC-AUD-2), `file://<absolute path>` on storage outside the database volume, or `s3://<bucket>[/<prefix>]` (M7-22), an S3-compatible object store with a retention lock, the recommended target for production: a bucket in the S3 bucket naming rules, an optional prefix of segments of `[a-z0-9-]` joined by single `/`. `bothy config check` refuses every other form; for `s3://` it and the server start read the Object Lock configuration of the bucket and fail unless Object Lock is enabled with a default retention in mode `COMPLIANCE` of at least `retention.audit` (`GOVERNANCE` fails) | active |
| `audit.checkpoint_s3_endpoint` | `BOTHY_AUDIT__CHECKPOINT_S3_ENDPOINT` | Server | string | empty | endpoint of the object store of an `s3://` sink, an absolute `http` or `https` URL without path; its own endpoint, separate from `file.s3.endpoint` and the backup; `http` only with an `egress.allowed_hosts` entry with `allow_private` | active |
| `audit.checkpoint_s3_region` | `BOTHY_AUDIT__CHECKPOINT_S3_REGION` | Server | string | `us-east-1` | region of the S3 signature of an `s3://` sink | active |
| `audit.checkpoint_s3_use_path_style` | `BOTHY_AUDIT__CHECKPOINT_S3_USE_PATH_STYLE` | Server | bool | `true` | path-style requests of an `s3://` sink instead of `<bucket>.<endpoint host>` | active |
| `audit.checkpoint_s3_access_key_ref` | `BOTHY_AUDIT__CHECKPOINT_S3_ACCESS_KEY_REF` | Server | secret_ref | `file:<secrets.dir>/audit_checkpoint_s3_access_key` | secret reference of the own access key ID of an `s3://` sink, read only with an `s3://` sink; supplied by the owner from a separate account ([backup-restore.md](../operations/backup-restore.md) 8) | active |
| `audit.checkpoint_s3_secret_key_ref` | `BOTHY_AUDIT__CHECKPOINT_S3_SECRET_KEY_REF` | Server | secret_ref | `file:<secrets.dir>/audit_checkpoint_s3_secret_key` | secret reference of the own secret access key of an `s3://` sink, read only with an `s3://` sink | active |
| `audit.reason_max_chars` | `BOTHY_AUDIT__REASON_MAX_CHARS` | Server | int | `500` | maximum length of a mandatory reason of an administrative action in Unicode characters (SEC-AUD-5) | active |
| `signing_key.cache_ttl` | `BOTHY_SIGNING_KEY__CACHE_TTL` | Server | duration | `30s` | longest time a process still accepts a retired key or does not know a new one | active |
| `signing_key.rotation_check_interval` | `BOTHY_SIGNING_KEY__ROTATION_CHECK_INTERVAL` | Server | duration | `1m` | interval of the job `instance.key_rotation` | active |
| `signing_key.access_token_rotation_interval` | `BOTHY_SIGNING_KEY__ACCESS_TOKEN_ROTATION_INTERVAL` | Server | duration | `30d` | scheduled rotation of the key for access tokens | active |
| `signing_key.accept_lost_keys` | `BOTHY_SIGNING_KEY__ACCEPT_LOST_KEYS` | Server | bool | `false` | consent that a start whose key store `secrets.signing_key_dir` holds the private file of none of the unretired keys retires them and creates fresh ones; without it that start ends with an error and changes no key (restore without the key store, [disaster-recovery.md](../operations/disaster-recovery.md) case B) | active |

### 20 Privacy

Source: [20-privacy.md](../spec/20-privacy.md), section 13.

| Key | Environment | Place | Type | Default | Meaning | Status |
|---|---|---|---|---|---|---|
| `retention.content` | `BOTHY_RETENTION__CONTENT` | Server |  | `none` | period for contents; `none` means no automatic deletion (without `retention.content_none_reason` a warning, 4.1) | planned (M7) |
| `retention.content_none_reason` | `BOTHY_RETENTION__CONTENT_NONE_REASON` | Server |  | empty | reason of the instance operator for `retention.content = none`; empty means a warning in `bothy config check` and in the startup log (4.1) | planned (M7) |
| `retention.security_after_end` | `BOTHY_RETENTION__SECURITY_AFTER_END` | Server |  | `90d` | security data after end, expiry or revocation | planned (M7) |
| `retention.audit` | `BOTHY_RETENTION__AUDIT` | Server | duration | `365d` | audit log (Study C suggested twelve months) | active |
| `retention.cost` | `BOTHY_RETENTION__COST` | Server |  | `395d` | costs and prices | planned (M7) |
| `retention.import_after_completion` | `BOTHY_RETENTION__IMPORT_AFTER_COMPLETION` | Server |  | `90d` | import state after completion | planned (M7) |
| `retention.privacy_job` | `BOTHY_RETENTION__PRIVACY_JOB` | Server |  | `1095d` | privacy requests (without export packages) | planned (M7) |
| `retention.ops` | `BOTHY_RETENTION__OPS` | Server |  | `395d` | status page | planned (M7) |
| `retention.stats_minutes` | `BOTHY_RETENTION__STATS_MINUTES` | Server |  | `90d` | minute aggregates of the own measurement (`stats_minutes`); at least the longest SLO period | planned (M6) |
| `retention.webhook_deliveries` | `BOTHY_RETENTION__WEBHOOK_DELIVERIES` | Server |  | `30d` | delivery attempts of outgoing webhooks | planned (M7) |
| `privacy.export_ttl` | `BOTHY_PRIVACY__EXPORT_TTL` | Server |  | `7d` | availability of an export package | planned (M7) |
| `privacy.export_requires_approval` | `BOTHY_PRIVACY__EXPORT_REQUIRES_APPROVAL` | Server |  | `false` | export of one's own data needs the approval of the owner | planned (M7) |
| `privacy.account_deletion_content` | `BOTHY_PRIVACY__ACCOUNT_DELETION_CONTENT` | Server |  | `keep_pseudonymized` | contents of deleted accounts: `keep_pseudonymized` or `delete` | planned (M7) |
| `privacy.object_delete_delay` | `BOTHY_PRIVACY__OBJECT_DELETE_DELAY` | Server |  | `24h` | maximum time until deleted objects are removed | planned (M3) |
| `privacy.deletion_journal_retention` | `BOTHY_PRIVACY__DELETION_JOURNAL_RETENTION` | Server |  | `400d` | retention of the deletion journal; at least the backup period | planned (M7) |
| `privacy.audit_ip_key_period` | `BOTHY_PRIVACY__AUDIT_IP_KEY_PERIOD` | Server | duration | `30d` | period per key for IP pseudonyms | active |
| `privacy.audit_ip_key_retention` | `BOTHY_PRIVACY__AUDIT_IP_KEY_RETENTION` | Server | duration | `90d` | keys are destroyed after this | active |
| `privacy.audit_ip_kek_ref` | `BOTHY_PRIVACY__AUDIT_IP_KEK_REF` | Server | secret_ref | `file:<secrets.dir>/audit_ip_kek` | reference to the key that encrypts the IP keys in `audit_ip_keys` | active |
| `privacy.telemetry_pseudonym_rotation` | `BOTHY_PRIVACY__TELEMETRY_PSEUDONYM_ROTATION` | Server | duration | `30d` | rotation of the key for pseudonymous IDs in telemetry | active |
| `privacy.per_person_stats_enabled` | `BOTHY_PRIVACY__PER_PERSON_STATS_ENABLED` | Server |  | `false` | statistics per person | planned (M6) |
| `privacy.stats_min_group_size` | `BOTHY_PRIVACY__STATS_MIN_GROUP_SIZE` | Server |  | `5` | smallest group for evaluations by persons | planned (M6) |
| `privacy.ai_notice_enabled` | `BOTHY_PRIVACY__AI_NOTICE_ENABLED` | Server |  | `true` | one-time AI notice per channel with an agent | planned (M7) |
| `privacy.notice_url` | `BOTHY_PRIVACY__NOTICE_URL` | Server |  | empty | link to the privacy information of the instance operator | planned (M7) |

### 21 Multi-user dev environment and isolation

Source: [21-dev-isolation.md](../spec/21-dev-isolation.md), section 12.

| Key | Environment | Place | Type | Default | Meaning | Status |
|---|---|---|---|---|---|---|
| `dev.reverify_interval` | `BOTHY_DEV__REVERIFY_INTERVAL` | Server |  | `24h` | an environment is checked again at the latest after this time | planned (V2) |
| `dev.limits` | `BOTHY_DEV__LIMITS` | Server |  | `{"cpus": 4, "memory_mib": 8192, "disk_gib": 50, "pids": 2048}` | limits per environment | planned (V2) |
| `dev.egress_allowlist` | `BOTHY_DEV__EGRESS_ALLOWLIST` | Server |  | empty | allowed targets (hosts) for environments; without an entry no egress | planned (V2) |

### 22 Import and migration from Discord

Source: [22-import-migration.md](../spec/22-import-migration.md), section 19.

| Key | Environment | Place | Type | Default | Meaning | Status |
|---|---|---|---|---|---|---|
| `import.discord.api_base` |  | Tool `bothy import` |  | `https://discord.com/api/v10` | Discord API base [R6, verified: v10 current] |  |
| `import.discord.cdn_hosts` |  | Tool `bothy import` |  | `cdn.discordapp.com`, `media.discordapp.net` | allowed hosts for attachments |  |
| `import.discord.page_size` |  | Tool `bothy import` |  | `100` | messages per page; Discord allows at most 100 [R4, verified] |  |
| `import.discord.max_429_retries` |  | Tool `bothy import` |  | `5` | retries after `429` per call |  |
| `import.discord.max_rate_wait` |  | Tool `bothy import` |  | `60s` | longest wait before the run aborts |  |
| `import.discord.preflight_sample` |  | Tool `bothy import` |  | `20` | messages for the intent probe |  |
| `import.discord.closed_tags` |  | Tool `bothy import` |  | empty | forum tags that additionally mark a thread as done |  |
| `import.attachment_max_bytes` |  | Tool `bothy import` |  | same as `file.max_upload_bytes` | largest imported attachment |  |
| `import.http_timeout` |  | Tool `bothy import` |  | `60s` | time limit per call to Discord and the CDN; also the time limit of each call of the tool to Bothy |  |
| `import.login_poll_interval` |  | Tool `bothy import` |  | `5s` | interval between polls of `completePairing` during `bothy import login`; the server limits public auth calls by `ratelimit.rl.public_auth.*` |  |
| `import.batch_max_items` |  | Tool `bothy import` |  | `100` | items per batch sent to the server |  |
| `import.reply_quote_max_chars` |  | Tool `bothy import` |  | `100` | length of the quote line for replies across threads |  |
| `import.follow.poll_interval` |  | Tool `bothy import` |  | `30s` | interval between mirror rounds |  |
| `import.follow.settle_after` |  | Tool `bothy import` |  | `60s` | quiet time after the last change |  |
| `import.follow.busy_reactions` |  | Tool `bothy import` |  | `👀` | reactions that mean "agent is still working" |  |
| `import.follow.busy_lookback` |  | Tool `bothy import` |  | `10` | messages before the first new one that are checked for the marker |  |
| `import.follow.max_backoff` |  | Tool `bothy import` |  | `5m` | longest interval after failed rounds |  |
| `import.lease_stale_after` |  | Tool `bothy import` |  | `90s` | a run lease renewed less than this long ago blocks a second writing run (section 7.4); larger than `import.follow.poll_interval` |  |

### 24 Terminal

Source: [24-terminal.md](../spec/24-terminal.md), section 17.

| Key | Environment | Place | Type | Default | Meaning | Status |
|---|---|---|---|---|---|---|
| `terminal.pairing_code_ttl` | `BOTHY_TERMINAL__PAIRING_CODE_TTL` | Server | duration | `15m` | Validity of a pairing code for a host | active |
| `terminal.fingerprint_confirm_chars` | `BOTHY_TERMINAL__FINGERPRINT_CONFIRM_CHARS` | Server | int | `8` | this many characters of the fingerprint the owner types for the confirmation | active |
| `terminal.fingerprint_max_attempts` | `BOTHY_TERMINAL__FINGERPRINT_MAX_ATTEMPTS` | Server | int | `3` | failed attempts at the confirmation, then fallback to `pairing` | active |
| `terminal.report_label_max_chars` | `BOTHY_TERMINAL__REPORT_LABEL_MAX_CHARS` | Server | int | `100` | longest profile name, display name, name of a working directory and operating system version that a host reports (3.2) | active |
| `terminal.report_max_entries` | `BOTHY_TERMINAL__REPORT_MAX_ENTRIES` | Server | int | `64` | at most this many entries per reported list: profiles, session users, working directories of a session user (3.2) | active |
| `terminal.step_up_max_age` | `BOTHY_TERMINAL__STEP_UP_MAX_AGE` | Server | duration | `60s` | the step-up for `terminal.open` and `terminal.attach` must be at most this old | active |
| `terminal.session_token_ttl` | `BOTHY_TERMINAL__SESSION_TOKEN_TTL` | Server | duration | `30s` | Validity of a terminal session token | active |
| `terminal.open_timeout` | `BOTHY_TERMINAL__OPEN_TIMEOUT` | Server | duration | `20s` | Deadline until both sides are connected in the relay and the handshake is complete | active |
| `terminal.detach_grace` | `BOTHY_TERMINAL__DETACH_GRACE` | Server | duration | `15m` | this long a detached session stays attachable | active |
| `terminal.quick_reattach_window` | `BOTHY_TERMINAL__QUICK_REATTACH_WINDOW` | Server | duration | `60s` | Reattach from the same device without a new step-up within this time after the detach | active |
| `terminal.idle_timeout` | `BOTHY_TERMINAL__IDLE_TIMEOUT` | Server | duration | `30m` | without input from the device for this long, the session ends | active |
| `terminal.idle_grace` | `BOTHY_TERMINAL__IDLE_GRACE` | Server | duration | `2m` | Extra time after which the server itself ends the session because of idleness | active |
| `terminal.max_session_duration` | `BOTHY_TERMINAL__MAX_SESSION_DURATION` | Server | duration | `12h` | Maximum duration of a session | active |
| `terminal.max_sessions_per_person` | `BOTHY_TERMINAL__MAX_SESSIONS_PER_PERSON` | Server | int | `5` | open sessions per person | active |
| `terminal.max_sessions_per_host` | `BOTHY_TERMINAL__MAX_SESSIONS_PER_HOST` | Server | int | `10` | open sessions per host | active |
| `terminal.relay_buffer_max_bytes` | `BOTHY_TERMINAL__RELAY_BUFFER_MAX_BYTES` | Server | int | `262144` | Buffer of the relay per direction and session | active |
| `terminal.host_heartbeat_interval` | `BOTHY_TERMINAL__HOST_HEARTBEAT_INTERVAL` | Server | duration | `20s` | Interval of the heartbeats of the control connection (reported in `hello`) | active |
| `terminal.host_heartbeat_miss_limit` | `BOTHY_TERMINAL__HOST_HEARTBEAT_MISS_LIMIT` | Server | int | `3` | missing heartbeats until the control connection is closed | active |
| `terminal.host_auth_timeout` | `BOTHY_TERMINAL__HOST_AUTH_TIMEOUT` | Server | duration | `10s` | Deadline between `hello` and `identify` of the control and data connection | active |
| `terminal.sweep_interval` | `BOTHY_TERMINAL__SWEEP_INTERVAL` | Server | duration | `30s` | Interval of the job `terminal.sweep` ([02-architecture.md](../spec/02-architecture.md), section 12.3) | active |
| `terminal.host_duplicate_probe_timeout` | `BOTHY_TERMINAL__HOST_DUPLICATE_PROBE_TIMEOUT` | Server | duration | `5s` | this long the server waits for the answer of the older control connection (3.4) | active |
| `host_service.state_dir` |  | Host service |  | `/var/lib/bothy-host` (Windows: `%ProgramData%\bothy-host`) | State directory with the keys of the host, the bindings of the session users to people and, with `organization`, the API key of the profile `claude`; also the pairing record and the last declaration from `ready`, both without secrets (D-M2-42-4); never a subscription login (7.4) |  |
| `host_service.auth_status_timeout` |  | Host service |  | `30s` | bound of one call of `claude auth status` and of the login shell probe of HD6; a timeout gives `auth_status_unknown` (7.4, D-M2-42-3) |  |
| `host_service.session_users` |  | Host service |  | empty | Session users of this host, one per person; one PTY helper per entry; never the service user; empty means: HD1 not passed; read only at start, changed with `bothy-host session-user add` and `remove`, which restart the host service for this (2.2) |  |
| `host_service.extra_forbidden_groups` |  | Host service |  | empty | further groups that HD2 blocks in addition to the fixed list |  |
| `host_service.credential_check_interval` |  | Host service |  | `60s` | Interval of the checks HD6 and HD11 during operation (2.2, 7.4) |  |
| `host_service.entry_scan_timeout` |  | Host service |  | `10m` | Maximum duration of the check of the entries of HD11 per session user; after that their PTY helper aborts it, and HD11 does not pass for them (`entry_scan_timeout`, 2.2) |  |
| `host_service.profiles` |  | Host service |  | `[shell]` | offered profiles (11.1) |  |
| `host_service.claude_bin` |  | Host service |  | empty | absolute path of the coding agent that HD6 calls for `claude auth status` and that the profile `claude` starts with `organization`; belongs, with folder chain, interpreter and the folders of the search list, to a trusted owner (2.2 item 6, HD10); under Windows an `.exe`; empty means: HD6 checks only the login locations and warns, with `organization` there is no profile `claude`, enforcement rests on the person's own session user (7.4); for HD6 and the profile the value also counts as empty if the path cannot be resolved or the search list does not find the interpreter of a script |  |
| `host_service.claude_workdirs` |  | Host service |  | empty | Working directories of the profile `claude` per session user (name from `host_service.session_users` -> list of folders with display names; only the display names are reported); each folder belongs to exactly this session user (Ruling T13, HD7, HD11); a folder with two session users drops out of both lists, the list after that is reported and used for `wdi`; read only at start, a change takes effect after `bothy-host restart`; without an entry no profile `claude` for this session user, empty means no profile `claude` |  |
| `host_service.scrollback_bytes` |  | Host service |  | `1048576` | Scrollback buffer per session, only in memory |  |
| `host_service.require_local_confirm` |  | Host service |  | `false` | every PTY only after consent on the host (6.4) |  |
| `host_service.local_confirm_timeout` |  | Host service |  | `60s` | Deadline for the consent on the host |  |
| `host_service.reconnect_backoff_base` |  | Host service |  | `1s` | first delay of the exponential backoff with full jitter before a new control connection (3.5) |  |
| `host_service.reconnect_backoff_cap` |  | Host service |  | `5m` | largest delay of that backoff; also the total time the login of a refused start keeps trying (3.5) |  |

### 25 Visualization

Source: [25-visualization.md](../spec/25-visualization.md), section 15.

| Key | Environment | Place | Type | Default | Meaning | Status |
|---|---|---|---|---|---|---|
| `stats.live_enabled` | `BOTHY_STATS__LIVE_ENABLED` | Server |  | `true` | live stream and `getStatsLive` on; off: `403 feature_disabled` | planned (M6) |
| `stats.live_interval` | `BOTHY_STATS__LIVE_INTERVAL` | Server |  | `1s` | interval between live frames; also the lifetime of the base frame for `getStatsLive` | planned (M6) |
| `stats.live_max_runs` | `BOTHY_STATS__LIVE_MAX_RUNS` | Server |  | `200` | at most this many active runs per frame | planned (M6) |
| `stats.live_max_subscribers` | `BOTHY_STATS__LIVE_MAX_SUBSCRIBERS` | Server |  | `200` | subscriptions to `stats.live` per api node | planned (M6) |
| `stats.live_sli_window` | `BOTHY_STATS__LIVE_SLI_WINDOW` | Server |  | `5m` | period of the SLI values in the live frame | planned (M6) |
| `stats.flush_interval` | `BOTHY_STATS__FLUSH_INTERVAL` | Server |  | `10s` | interval at which a process writes its counters to `stats_minutes` | planned (M6) |
| `stats.flush_queue_max` | `BOTHY_STATS__FLUSH_QUEUE_MAX` | Server |  | `20000` | counter keys in memory per process; above that, the oldest are discarded | planned (M6) |
| `stats.timeline_max_range` | `BOTHY_STATS__TIMELINE_MAX_RANGE` | Server |  | `24h` | longest period of a timeline query | planned (M6) |
| `stats.timeline_max_runs` | `BOTHY_STATS__TIMELINE_MAX_RUNS` | Server |  | `5000` | at most this many runs in a timeline response | planned (M6) |
| `stats.slo_warn_budget_ratio` | `BOTHY_STATS__SLO_WARN_BUDGET_RATIO` | Server |  | `0.25` | below this remaining budget, the traffic light turns yellow | planned (M6) |
| `stats.slo_min_events` | `BOTHY_STATS__SLO_MIN_EVENTS` | Server |  | `100` | below this many events in the SLO period, the traffic light is gray | planned (M6) |
| `console.grafana_embed_url` | `BOTHY_CONSOLE__GRAFANA_EMBED_URL` | Server |  | empty | dashboard URL for embedding in the console; empty means no embedding and no `frame-src` | planned (M6) |
| `viz.live_poll_interval` |  | App |  | `2s` | polling of `getStatsLive` in the web console |  |
| `viz.stale_after` |  | App |  | `5s` | without a new frame, the view shows "stale" |  |
| `viz.timeline_speeds` |  | App |  | `60, 300, 900` | playback speeds of the timeline (seconds of time of day per second) |  |
| `viz.tv_rotation_interval` |  | App |  | `30s` | switching of the views in TV mode |  |
| `viz.tv_show_costs` |  | App |  | `false` | show costs in TV mode |  |
| `viz.perf.frame_build_p95` |  | App |  | `8ms` | target: build of one frame of the live topology with `stats.live_max_runs` runs |  |
| `viz.perf.frame_build_p95_ci` |  | App |  | `50ms` | budget of the same test on a shared hosted CI runner (measured p95 about 23 ms on the macOS runner, about twice that as headroom); the reference target above holds on reference hardware through `make perf` |  |
<!-- config-reference:end -->

## 5. Secrets

Catalogue of the files in the volume `secrets` ([16-operations.md](../spec/16-operations.md) 5.3).
Files marked "generated" are created by `secrets-init`; files marked "supplied" are placed by the
owner in `infra/compose/secrets/`. The column "Services" names the subdirectories (`subpath`)
into which `secrets-init` copies the file.

| File | Origin | Services | Use |
|---|---|---|---|
| `db_superuser_password` | generated | postgres, migrate, backup | superuser `postgres` |
| `db_owner_password` | generated | migrate | role `bothy_owner` |
| `db_app_password` | generated | server, migrate | role `bothy_app`, `database.password_ref` |
| `db_maint_password` | generated | server, migrate | role `bothy_maint` (retention job) |
| `db_monitor_password` | generated | collector, migrate | role `bothy_monitor`, receiver `postgresql` |
| `db_backup_password` | generated | migrate, backup | role `bothy_backup` of `bothy-backupd`, which writes only `backup_runs` (`migrate.backup_password_ref`; `database.password_ref` of `backup`) |
| `token_pepper` | generated | server | `auth.token_pepper_ref` |
| `totp_key` | generated, 32 bytes | server | `auth.totp_key_ref`: AES-256-GCM key of the TOTP secrets in `local_credentials` ([08-identity.md](../spec/08-identity.md) 2.5) |
| `telemetry_pseudonym_key` | generated, at least 32 bytes | server, collector | `telemetry.pseudonym_key_ref`, `hmac_key` of the allowlist |
| `audit_ip_kek` | generated, 32 bytes | server | `privacy.audit_ip_kek_ref`: encrypts the IP keys in `audit_ip_keys` |
| `file_ticket_key` | generated, 32 bytes | server | `file.ticket_key_ref`: HMAC of the file tickets ([12-files.md](../spec/12-files.md) 9.2) |
| `webhook_hmac_master_key` | generated, 32 bytes | server | `webhook.hmac_master_key_ref`: master key of the derived webhook secrets ([webhooks.md](../api/webhooks.md) 2.1) |
| `pgbackrest_cipher_pass` | generated | backup | encryption of the pgBackRest repositories |
| `pgbackrest_tls/` | generated (own CA) | postgres, backup | mutual TLS between the database and the repository host: `ca.crt`, `postgres.crt` and `postgres.key`, `backup.crt` and `backup.key` (ECDSA P-256, valid for `stack.PGBackRestTLSValidity`; the CA key is not kept) |
| `restic_password` | generated | backup | encryption of the restic repository |
| `s3_access_key`, `s3_secret_key` | generated (profile s3) or supplied (external service) | server, seaweedfs, backup | S3 access, `file.s3.access_key_ref` and `file.s3.secret_key_ref` |
| `audit_checkpoint_s3_access_key`, `audit_checkpoint_s3_secret_key` | supplied | server | own access key of the object store of the audit checkpoints, `audit.checkpoint_s3_access_key_ref` and `audit.checkpoint_s3_secret_key_ref`; read only with an `s3://` `audit.checkpoint_sink`, from a separate account ([backup-restore.md](backup-restore.md) 8) |
| `grafana_admin_password` | generated | grafana | admin account in the local Grafana |
| `apns_key.p8` | supplied | server | push for iOS and macOS, `push.apns.private_key_ref` |
| `fcm_service_account.json` | supplied | server | push for Android |
| `oidc/<provider>_client_secret` | supplied | server | OIDC with `client_secret_basic` (`client_secret_ref` in `identity_providers.config`) |
| `oidc/<provider>_client_key.pem` | supplied | server | OIDC with `private_key_jwt`: private key of the client assertion, PKCS#8 (`client_key_ref`, [08-identity.md](../spec/08-identity.md) 2.6) |
| `offsite/` | supplied | backup | key of the off-site S3 bucket (`backup.offsite.target`): `access_key_id`, `secret_access_key`, optional `ca.crt` for a private CA; may write and read, never remove a stored version ([backup-restore.md](backup-restore.md) 2) |
| `alertmanager_bothy_webhook_url` | supplied | alertmanager | incoming webhook of the alert channel "Operations" on the internal listener (`http://api:8081/api/v1/hooks/<id>/<token>`); written by `infra/compose/alert-setup.sh channel` with mode `0600` ([alerting-setup.md](alerting-setup.md)) |
| `alertmanager_external_webhook_url` | supplied | alertmanager | external on-call channel of the operator's choice for `page` alerts; `alert-setup.sh external` |
| `alertmanager_heartbeat_url` | supplied | alertmanager | external heartbeat service of the operator's choice for the Watchdog; `alert-setup.sh heartbeat`. Alertmanager does not start while one of the three is missing or empty (start guard `infra/observability/alertmanager/start.sh`) |
| `newrelic_license_key` | supplied | collector | overlay `newrelic` |
| `grafana_otlp_authorization` | supplied | collector | overlay `grafana` (whole header value `Basic ...`) |
| `datadog_api_key` | supplied | collector | overlay `datadog` |
| `azure_client_secret` | supplied | collector | overlay `azure-monitor` (service principal) |
| `sentry_key` | supplied | collector | overlay `sentry` |

Not in the volume `secrets`: the server itself creates the private signing keys
(`<purpose>-<kid>.pem`, purposes `access_token`, `invocation`, `webhook_out`, `audit_checkpoint`)
in the volume `signing-keys` (`secrets.signing_key_dir`, ADR 0066); `server_signing_keys.private_key_ref`
points there. `bothy secrets export` backs up both places; it takes in the key store with M1-32,
which generates the signing keys.

Generators of later milestones: the generated
form of `s3_access_key` and `s3_secret_key` (profile s3, M3). Until then, `bothy secrets init`
reports these entries as `pending` as long as neither the store nor the source holds them; meanwhile
`s3_*` count as supplied. Passwords are generated as 32 random bytes in base64url without
padding (43 characters) and keys (`token_pepper`, `totp_key`, `telemetry_pseudonym_key`,
`audit_ip_kek`, `file_ticket_key`, `webhook_hmac_master_key`) as 32 random bytes in lower-case hex (64 characters); users
that need the raw key bytes decode the hex text.

`pgbackrest_tls/` (M7-01) is generated as a directory: a self-signed CA that may sign only leaf
certificates, and one certificate per pgBackRest host (`postgres`, `backup`) with the host name as
common name and DNS name and both TLS server and client use. The CA key exists only during the
run, so no service can issue further certificates; a rotation removes the directory from `_store`
and runs `secrets init` again, then restarts `postgres` and `backup`. Compose runs `secrets-init`
with `--owner postgres=999:999 --owner backup=999:999`, because pgBackRest reads keys only with
mode `0600` or stricter.

## 6. Instance settings

Stored in `instance_settings` ([03-data-model.md](../spec/03-data-model.md) 4.1), checked
against the schema of the key.

| Key | Type | Default | Meaning |
|---|---|---|---|
| `branding.product_name` | string | `Bothy` | displayed product name (ADR 0044); at most `branding.product_name_max_chars` characters, without control and bidi characters |
| `branding.accent_color` | key of the category colors | `spruce` | accent color; only keys from the checked palette (ADR 0044), no free color value |
| `branding.logo_file_id` | uuid | empty | logo as a file |
| `locale.default` | BCP 47 | `de` | default language of new people |

[01-product.md](../spec/01-product.md), section 12 describes the branding keys; personal
settings (`user_settings`, for example `notify.*`) are in [13-push-notifications.md](../spec/13-push-notifications.md),
section 3.3.

## 7. Feature flags

Stored in `feature_flags`; rules in [16-operations.md](../spec/16-operations.md) 6.2.

| Key | Kind | Default | Meaning | Source |
|---|---|---|---|---|
| `dm.enabled` | option | off | direct messages between people | [05-actions.md](../spec/05-actions.md) `dm.create` |
| `voice.enabled` | option | off | transcription of voice messages | [05-actions.md](../spec/05-actions.md) `file.transcribe` |
| `tokens.personal_enabled` | option | off | personal API tokens | [05-actions.md](../spec/05-actions.md) `token.create` |

## 8. OpenTelemetry variables

The server and `bothy-backupd` read the standard variables of the OpenTelemetry SDK unchanged.
Compose sets these:

| Variable | Value in Compose | Meaning |
|---|---|---|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `http://otel-collector:4317` | collector |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | `grpc` | protocol |
| `OTEL_METRIC_EXPORT_INTERVAL` | from `BOTHY_OTEL_METRIC_EXPORT_INTERVAL_MS` | export interval of the metrics in ms |
| `OTEL_RESOURCE_ATTRIBUTES` | `deployment.environment.name=<environment>` | resource attributes |

The standard variables `OTEL_BSP_*` (spans) and `OTEL_BLRP_*` (logs) set queues and batch sizes.
The server sets `service.name`, `service.namespace` and `bothy.server.role` itself; a value in
`OTEL_RESOURCE_ATTRIBUTES` does not override them.

## 9. Compose variables

File `infra/compose/.env` based on the template [`.env.example`](../../infra/compose/.env.example).
These variables are not server configuration; Compose derives values of the services from them.

| Variable | Template | Meaning |
|---|---|---|
| `COMPOSE_PROFILES` | `direct,scan` | profiles: exactly one of direct, tailscale; in addition scan, s3, observability |
| `BOTHY_REGISTRY` | `ghcr.io/OWNER` | registry path of the Bothy images in lowercase; in the bundle of a product release `ghcr.io/<owner>` from the owner of the repository ([18-release.md](../spec/18-release.md) 2.1) |
| `BOTHY_VERSION` | pinned server version of the last product release, before the first one `0.0.0-placeholder` | server version (SemVer without v); never latest; written by `make release-prep COMPONENT=bothy SERVER_VERSION=<version>`, a server pre-release only in a product beta (`CHANNEL=beta`, gate `release-pins`), a placeholder aborts `make release-build` ([18-release.md](../spec/18-release.md) 2.1) |
| `BOTHY_ENVIRONMENT` | `prod` | sets `instance.environment` and `deployment.environment.name` |
| `BOTHY_PUBLIC_URL` | `https://chat.example.com` | sets `server.public_url` |
| `BOTHY_HTTP_PUBLISH_PORT` | `8090` | host port on 127.0.0.1 for the API (profile direct) |
| `BOTHY_TRUSTED_PROXIES` | `127.0.0.1/32,::1/128` | sets `server.trusted_proxies` in profile direct; enter the gateway address of the Compose network there ([install-compose.md](install-compose.md) 5.1) |
| `BOTHY_OTLP_PUBLISH_PORT` | `4318` | host port on 127.0.0.1 for OTLP/HTTP from adapters on the same host |
| `BOTHY_PROMETHEUS_PUBLISH_PORT` | `9090` | host port on 127.0.0.1 for Prometheus |
| `BOTHY_ALERTMANAGER_PUBLISH_PORT` | `9093` | host port on 127.0.0.1 for Alertmanager |
| `BOTHY_GRAFANA_PUBLISH_PORT` | `3300` | host port on 127.0.0.1 for Grafana |
| `TS_HOSTNAME` | `bothy` | device name in the tailnet; appears in the public certificate log, so choose a neutral name |
| `TS_AUTHKEY` | empty | one-time key for the first Tailscale login; clear it afterwards |
| `BOTHY_TAILSCALE_VERSION` | `v1.102.5` | image version of the Tailscale sidecar |
| `BOTHY_STORAGE_DRIVER` | `fs` | sets `file.storage.driver`: fs or s3 (chapter 12) |
| `BOTHY_S3_ENDPOINT` | `http://seaweedfs:8333` | sets `file.s3.endpoint` with driver s3 |
| `BOTHY_S3_BUCKET` | `bothy-files` | sets `file.s3.bucket` with driver s3 |
| `BOTHY_CLAMD_ADDRESS` | `unix:///run/clamav/clamd.sock` | sets `file.scan.clamd_address` (chapter 12); the socket lies in the volume `clamav-socket` |
| `BOTHY_PG_SHM_SIZE` | `256m` | shared memory of the database container |
| `BOTHY_PG_ARCHIVE_TIMEOUT` | `60s` | sets `archive_timeout` in PostgreSQL; equal to `backup.pg.archive_timeout` |
| `BOTHY_BACKUP_OFFSITE_TARGET` | empty | sets `backup.offsite.target` (`s3:https://<host>[:<port>]/<bucket>[/<prefix>]`) and `BOTHY_PGBACKREST_REPO2` of `postgres` |
| `BOTHY_OTEL_EXPORTER` | `none` | first collector overlay |
| `BOTHY_OTEL_EXPORTER_2` | `none` | second collector overlay |
| `BOTHY_OTEL_METRIC_EXPORT_INTERVAL_MS` | `15000` | sets `OTEL_METRIC_EXPORT_INTERVAL` of the server processes |
| `BOTHY_TRACE_SAMPLE_PERCENT` | `10` | tail sampling: share of the remaining traces in percent |
| `BOTHY_TRACE_SLOW_THRESHOLD_MS` | `2000` | tail sampling: from this duration on, a trace is always kept |
| `BOTHY_OTEL_MEMORY_LIMIT_PERCENT` | `80` | memory_limiter of the collector |
| `NEW_RELIC_OTLP_ENDPOINT` | `https://otlp.eu01.nr-data.net` | overlay newrelic: endpoint (EU or US) |
| `GRAFANA_OTLP_ENDPOINT` | `https://otlp-gateway-REGION.grafana.net/otlp` | overlay grafana: OTLP gateway |
| `GRAFANA_INSTANCE_ID` | empty | overlay grafana: instance ID (information only; the header value is in secrets) |
| `DD_SITE` | `datadoghq.eu` | overlay datadog: Datadog site |
| `AZURE_TENANT_ID` | empty | overlay azure-monitor: tenant |
| `AZURE_CLIENT_ID` | empty | overlay azure-monitor: client ID of the service principal |
| `AZURE_MONITOR_TRACES_ENDPOINT` | empty | overlay azure-monitor: OTLP endpoint for traces (DCE/DCR) |
| `AZURE_MONITOR_METRICS_ENDPOINT` | empty | overlay azure-monitor: OTLP endpoint for metrics |
| `AZURE_MONITOR_LOGS_ENDPOINT` | empty | overlay azure-monitor: OTLP endpoint for logs |
| `SENTRY_OTLP_TRACES_ENDPOINT` | empty | overlay sentry: OTLP endpoint for traces |
| `SENTRY_OTLP_LOGS_ENDPOINT` | empty | overlay sentry: OTLP endpoint for logs |
| `BOTHY_PROMETHEUS_RETENTION` | `30d` | retention in Prometheus |
| `BOTHY_LOKI_RETENTION` | `14d` | retention in Loki |
| `BOTHY_TEMPO_RETENTION` | `168h` | retention in Tempo |
| `BOTHY_OTELCOL_VERSION` | `0.162.0` | image version of the collector (contrib) |
| `BOTHY_CLAMAV_VERSION` | `1.5.4` | image version of ClamAV (clamav-debian, also arm64) |
| `BOTHY_SEAWEEDFS_VERSION` | `4.48` | image version of SeaweedFS |
| `BOTHY_PROMETHEUS_VERSION` | `v3.15.0` | image version of Prometheus |
| `BOTHY_ALERTMANAGER_VERSION` | `v0.34.1` | image version of Alertmanager |
| `BOTHY_LOKI_VERSION` | `3.7.8` | image version of Loki |
| `BOTHY_TEMPO_VERSION` | `3.1.0` | image version of Tempo |
| `BOTHY_GRAFANA_VERSION` | `13.2.3` | image version of Grafana |
| `BOTHY_NGINX_VERSION` | `1.30.6-alpine` | image version of the example edge (nginx-unprivileged) of the overlay `compose.multi-node.yaml` ([install-compose.md](install-compose.md) 5.3) |
| `BOTHY_CLAMAV_MEMORY_LIMIT` | `4g` | memory limit of the ClamAV container |
| `BOTHY_LOG_MAX_SIZE` | `20m` | size of a container log file until rotation |
| `BOTHY_LOG_MAX_FILE` | `5` | kept container log files per service |

Variables of the image `bothy-postgres` (M7-01) that `infra/compose/compose.yaml` sets itself,
not through `.env`; only the entry point of the image reads them
([image README](../../services/server/images/bothy-postgres/README.md)):

| Variable | Service | Meaning |
|---|---|---|
| `BOTHY_PGBACKREST_REPO_HOST` | postgres | host name of the pgBackRest repository host (`backup`); required |
| `BOTHY_PG_ARCHIVE_TIMEOUT` | postgres | `archive_timeout` of PostgreSQL, from the row above; required |
| `BOTHY_PGBACKREST_PG_HOST` | backup | host name of the database host (`postgres`); required |
| `BOTHY_PGBACKREST_REPO2` | postgres | `on` when `BOTHY_BACKUP_OFFSITE_TARGET` is set, else empty: `archive-push` also writes to repo2 (M7-02) |

## 10. Maintenance

1. A new server key enters the schema (`services/server/internal/wire/configkeys_*.go`, chapter in
   the field `Chapter`) and the table "Configuration values of this chapter" of its chapter in the
   same change, with the same default. Then run `go generate ./tools/configschema` in
   `services/server` and `make config-reference` in the repository. That rewrites section 4 and
   reports what is still missing, for example a rule in `tools/configdoc/planned.yaml` that no
   longer matches a planned key; it is deleted. `make codegen-check` keeps the exported file equal
   to the schema, `make config-doc-check` the rest (table below).
2. A value the server does not read yet stands only in the chapter table. With place `Server` it
   needs a milestone in `tools/configdoc/planned.yaml`: a rule with the exact name or with a prefix
   that ends in `.`; the exact name applies before a prefix, a longer prefix before a shorter one.
3. A name lives in exactly one chapter. A name retired by W1 (`tools/configdoc/retired.yaml`)
   does not come back, neither as a name in code nor as an environment variable.
4. A key is never reinterpreted. Removing it is a major version of the server
   ([18-release.md](../spec/18-release.md) 3); before that, it is listed as deprecated in the
   release notes for one minor version and produces a warning at startup.
5. Prose names only the name, never the number.

Rules of `make config-doc-check` (finding as `config-doc: <path>[:<line>]: <rule>: <text>`, exit
code 1). If an input file cannot be read or is malformed, or if the markers of section 4 are
missing, repeated or in the wrong order, the check ends with exit code 2. `make config-reference`
writes section 4 even when D1 to D9 find something, reports the findings and then ends with exit
code 1. These are the exit codes of `tools/configdoc/configdoc.py`;
`make` itself ends with exit code 2 on every failure of a target.

| Rule | Checks |
|---|---|
| D1 | No name stands in two table rows. |
| D2 | Every key of the schema has a row in a chapter table. |
| D3 | That row stands in the chapter the schema names. |
| D4 | Its place is `Server`. |
| D5 | Its default is the one of the schema: the cell without backticks, `empty` for an empty default, `no default, required` for a required key. |
| D6 | Every planned server key has a milestone in `planned.yaml`, and every rule there gives the milestone of at least one planned key. |
| D7 | No name from `retired.yaml` stands in code (fenced code block or code span after CommonMark with GitHub tables, also across line breaks of a paragraph, list item or quote; in other text files only blank lines and fences end a paragraph) and none of its `BOTHY_` variables anywhere in a tracked text file; exempt are `docs/PLAN.md`, `docs/DECISIONS.md`, the brief of M1-08, `retired.yaml` and the tests of `tools/configdoc`. A longer name that contains a retired one does not count. This file is checked in the form that `make config-reference` writes. A test holds that D7 finds, on all tracked Markdown files, every name in code that a CommonMark parser finds. Finding more is intended: cells beyond the header row, which a parser after GitHub tables drops (for example when a vertical bar in code splits the row; on the docs site it does not split it), and code spans in HTML blocks, HTML comments and indented code blocks. Not covered are names in prose without backticks, in indented code blocks outside backticks and in nested YAML form (`a:` with `b:` below it). |
| D8 | At least one Compose service runs the server (`entrypoint` and `command` start with `serve` or with the program `bothy` followed by `serve`), and every such service has a `stop_grace_period` strictly longer than `shutdown.total_timeout` ([02-architecture.md](../spec/02-architecture.md) 15.3): the value the service sets in `environment` for `BOTHY_SHUTDOWN__TOTAL_TIMEOUT`, else the default. Merge keys (`<<: *api`) apply; the check does not read `env_file` or the configuration file. |
| D9 | No two names have the same environment variable, and the exported schema file names for every key the variable of the naming rule from section 2. |
| D10 | Section 4 is current. |
| `table` | Every chapter has at most one section `## <number>. Configuration values of this chapter`; its table has the header `Name`, `Default`, `Meaning` (with `Place` in second position) and a separator row, every row as many cells as the header (`\|` is a bar inside a cell), valid names, a known place, a non-empty cell "Default" and as many defaults as names. Rows in fenced code blocks do not count. |
