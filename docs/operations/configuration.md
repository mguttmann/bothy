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
| `cluster.mode` | `BOTHY_CLUSTER__MODE` | Server |  | `single` | `single` or `multi` (several api nodes) | planned (M2) |
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
| `sync.log_poll_interval` | `BOTHY_SYNC__LOG_POLL_INTERVAL` | Server |  | `1s` | poll of the log tail without `NOTIFY` | planned (M1) |
| `sync.tail_batch_size` | `BOTHY_SYNC__TAIL_BATCH_SIZE` | Server |  | `1000` | events per read of the log tail | planned (M1) |
| `sync.partition_maintenance_interval` | `BOTHY_SYNC__PARTITION_MAINTENANCE_INTERVAL` | Server |  | `10m` | interval of the job `sync.partition_maintenance` | planned (M1) |
| `sync.event_log_partitions_ahead` | `BOTHY_SYNC__EVENT_LOG_PARTITIONS_AHEAD` | Server |  | `3` | partitions of `event_log` that are ready ahead of the head; with `sync.event_log_partition_size` they last longer than two intervals `sync.partition_maintenance_interval` at `perf.event_commit_min_per_s` | planned (M1) |
| `gateway.write_timeout` | `BOTHY_GATEWAY__WRITE_TIMEOUT` | Server |  | `10s` | deadline for one WebSocket write | planned (M1) |
| `consumer.batch_size` | `BOTHY_CONSUMER__BATCH_SIZE` | Server |  | `500` | events per round of a worker consumer | planned (M2) |
| `consumer.poll_interval` | `BOTHY_CONSUMER__POLL_INTERVAL` | Server |  | `1s` | poll of a worker consumer without `NOTIFY` | planned (M2) |
| `sync.idempotency_cleanup_interval` | `BOTHY_SYNC__IDEMPOTENCY_CLEANUP_INTERVAL` | Server |  | `1h` | interval of the job `sync.idempotency_cleanup` | planned (M1) |
| `sync.idempotency_cleanup_batch_size` | `BOTHY_SYNC__IDEMPOTENCY_CLEANUP_BATCH_SIZE` | Server |  | `1000` | rows per deletion batch of `sync.idempotency_cleanup` | planned (M1) |
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
| `branding.product_name_max_chars` | `BOTHY_BRANDING__PRODUCT_NAME_MAX_CHARS` | Server |  | `64` | length of the product name in the branding (`instance_settings`, `branding.product_name`) in Unicode characters | planned (M1) |
| `group.name_max_chars` | `BOTHY_GROUP__NAME_MAX_CHARS` | Server |  | `100` | length of a group name in Unicode characters | planned (M1) |
| `group.description_max_chars` | `BOTHY_GROUP__DESCRIPTION_MAX_CHARS` | Server |  | `500` | length of a group description in Unicode characters | planned (M1) |
| `host.name_max_chars` | `BOTHY_HOST__NAME_MAX_CHARS` | Server |  | `100` | length of a host name in Unicode characters | planned (M2) |
| `host.os_user_max_chars` | `BOTHY_HOST__OS_USER_MAX_CHARS` | Server |  | `64` | length of the name of a session user in `host_session_users.os_user` | planned (M2) |
| `space.name_max_chars` | `BOTHY_SPACE__NAME_MAX_CHARS` | Server |  | `100` | length of a space name in Unicode characters | planned (M1) |
| `channel.name_max_chars` | `BOTHY_CHANNEL__NAME_MAX_CHARS` | Server |  | `100` | length of a channel name in Unicode characters | planned (M1) |
| `channel.topic_max_chars` | `BOTHY_CHANNEL__TOPIC_MAX_CHARS` | Server |  | `1024` | length of a channel topic in Unicode characters | planned (M1) |
| `forum_tag.name_max_chars` | `BOTHY_FORUM_TAG__NAME_MAX_CHARS` | Server |  | `50` | length of a tag name in Unicode characters | planned (M1) |
| `identity.last_active_granularity` | `BOTHY_IDENTITY__LAST_ACTIVE_GRANULARITY` | Server |  | `5m` | `last_active_at` or `last_used_at` is written at most this often | planned (M1) |
| `auth.session_max_age` | `BOTHY_AUTH__SESSION_MAX_AGE` | Server |  | `30d` | absolute end of a login session | planned (M1) |
| `auth.refresh_token_ttl` | `BOTHY_AUTH__REFRESH_TOKEN_TTL` | Server |  | `14d` | lifetime of a refresh token | planned (M1) |
| `auth.step_up_max_age` | `BOTHY_AUTH__STEP_UP_MAX_AGE` | Server | duration | `5m` | a step-up confirmation counts as fresh for this long | active |
| `auth.refresh_reuse_grace` | `BOTHY_AUTH__REFRESH_REUSE_GRACE` | Server |  | `30s` | grace period for the immediately preceding refresh generation with the same DPoP key | planned (M1) |
| `agent.heartbeat_persist_interval` | `BOTHY_AGENT__HEARTBEAT_PERSIST_INTERVAL` | Server |  | `60s` | heartbeat fields are stored at most this often | planned (M2) |
| `channel.history_backfill_limit_default` | `BOTHY_CHANNEL__HISTORY_BACKFILL_LIMIT_DEFAULT` | Server |  | `20` | default for `history_backfill_limit` | planned (M1) |
| `channel.history_backfill_limit_max` | `BOTHY_CHANNEL__HISTORY_BACKFILL_LIMIT_MAX` | Server |  | `200` | upper limit for `history_backfill_limit` | planned (M1) |
| `channel.prompt_max_bytes` | `BOTHY_CHANNEL__PROMPT_MAX_BYTES` | Server |  | `32768` | size of a channel prompt | planned (M2) |
| `thread.title_max_chars` | `BOTHY_THREAD__TITLE_MAX_CHARS` | Server |  | `200` | length of a thread title | planned (M1) |
| `thread.max_tags` | `BOTHY_THREAD__MAX_TAGS` | Server |  | `5` | forum tags per thread | planned (M1) |
| `message.max_content_bytes` | `BOTHY_MESSAGE__MAX_CONTENT_BYTES` | Server | int | `262144` | size of a message (long answers without splitting) | active |
| `message.max_attachments` | `BOTHY_MESSAGE__MAX_ATTACHMENTS` | Server |  | `10` | attachments per message | planned (M1) |
| `message.max_mentions` | `BOTHY_MESSAGE__MAX_MENTIONS` | Server |  | `50` | distinct mention handles per message (content format, `messages`) | planned (M1) |
| `reaction.emoji_max_bytes` | `BOTHY_REACTION__EMOJI_MAX_BYTES` | Server |  | `64` | largest length of a reaction emoji in bytes (otherwise a grapheme cluster has no limit) | planned (M1) |
| `reaction.max_distinct_per_message` | `BOTHY_REACTION__MAX_DISTINCT_PER_MESSAGE` | Server |  | `20` | distinct emoji per message; limits `Message.reactions` and the payload of the events | planned (M1) |
| `pin.max_per_thread` | `BOTHY_PIN__MAX_PER_THREAD` | Server |  | `50` | pins per thread; `listPins` is not paginated | planned (M1) |
| `template.name_max_chars` | `BOTHY_TEMPLATE__NAME_MAX_CHARS` | Server |  | `100` | length of a template name in Unicode characters | planned (M1) |
| `template.max_per_scope` | `BOTHY_TEMPLATE__MAX_PER_SCOPE` | Server |  | `100` | templates per person or per channel; `listTemplates` is not paginated | planned (M1) |
| `file.max_upload_bytes` | `BOTHY_FILE__MAX_UPLOAD_BYTES` | Server | int | `104857600` | size of a file | active |
| `run.excerpt_max_chars` | `BOTHY_RUN__EXCERPT_MAX_CHARS` | Server |  | `500` | length of `last_text_excerpt` | planned (M2) |
| `run.step_detail_max_bytes` | `BOTHY_RUN__STEP_DETAIL_MAX_BYTES` | Server |  | `16384` | size of `run_steps.detail` | planned (M2) |
| `run.max_steps_persisted` | `BOTHY_RUN__MAX_STEPS_PERSISTED` | Server |  | `2000` | stored steps per run | planned (M2) |
| `run.stall_after` | `BOTHY_RUN__STALL_AFTER` | Server |  | `10m` | without progress: stall notice | planned (M2) |
| `component.default_ttl` | `BOTHY_COMPONENT__DEFAULT_TTL` | Server |  | `24h` | expiry of a component without its own value | planned (M2) |
| `push.max_consecutive_failures` | `BOTHY_PUSH__MAX_CONSECUTIVE_FAILURES` | Server |  | `5` | after that, a push registration is deactivated | planned (M3) |
| `webhook.allowed_hosts` | `BOTHY_WEBHOOK__ALLOWED_HOSTS` | Server |  | empty | allowed target hosts for outgoing webhooks | planned (M2) |
| `webhook.max_consecutive_failures` | `BOTHY_WEBHOOK__MAX_CONSECUTIVE_FAILURES` | Server |  | `20` | after that, a subscription is deactivated | planned (M2) |
| `webhook.retry_backoff_base` | `BOTHY_WEBHOOK__RETRY_BACKOFF_BASE` | Server |  | `10s` | base of the backoff | planned (M2) |
| `webhook.retry_backoff_cap` | `BOTHY_WEBHOOK__RETRY_BACKOFF_CAP` | Server |  | `1h` | largest interval | planned (M2) |
| `sync.event_log_partition_size` | `BOTHY_SYNC__EVENT_LOG_PARTITION_SIZE` | Server |  | `1000000` | sequence numbers per partition | planned (M1) |

### 04 Event and sync protocol

Source: [04-sync-protocol.md](../spec/04-sync-protocol.md), section 18.

| Key | Environment | Place | Type | Default | Meaning | Status |
|---|---|---|---|---|---|---|
| `sync.event_retention` | `BOTHY_SYNC__EVENT_RETENTION` | Server |  | `30d` | durable events stay available for catch-up this long | planned (M1) |
| `sync.event_payload_max_bytes` | `BOTHY_SYNC__EVENT_PAYLOAD_MAX_BYTES` | Server |  | `65536` | largest payload of an event | planned (M1) |
| `sync.ws_max_frame_bytes` | `BOTHY_SYNC__WS_MAX_FRAME_BYTES` | Server |  | `1048576` | largest WebSocket frame | planned (M1) |
| `sync.identify_timeout` | `BOTHY_SYNC__IDENTIFY_TIMEOUT` | Server |  | `10s` | time between `hello` and `identify` | planned (M1) |
| `sync.heartbeat_interval` | `BOTHY_SYNC__HEARTBEAT_INTERVAL` | Server |  | `30s` | interval of the heartbeats (reported in `hello`) | planned (M1) |
| `sync.heartbeat_miss_limit` | `BOTHY_SYNC__HEARTBEAT_MISS_LIMIT` | Server |  | `2` | missed heartbeats until the connection is closed | planned (M1) |
| `sync.resume_window` | `BOTHY_SYNC__RESUME_WINDOW` | Server |  | `5m` | `resume` is possible this long after a disconnect | planned (M1) |
| `sync.reauth_lead_time` | `BOTHY_SYNC__REAUTH_LEAD_TIME` | Server |  | `60s` | lead time of `reauth_required` before the token expires | planned (M1) |
| `sync.reconnect_backoff_base` | `BOTHY_SYNC__RECONNECT_BACKOFF_BASE` | Server |  | `1s` | base of the backoff when reconnecting | planned (M2) |
| `sync.reconnect_backoff_cap` | `BOTHY_SYNC__RECONNECT_BACKOFF_CAP` | Server |  | `60s` | largest interval when reconnecting | planned (M2) |
| `sync.catchup_page_size` | `BOTHY_SYNC__CATCHUP_PAGE_SIZE` | Server |  | `500` | events per catch-up page | planned (M1) |
| `sync.catchup_max_events` | `BOTHY_SYNC__CATCHUP_MAX_EVENTS` | Server |  | `50000` | a larger backlog yields `resync_required` | planned (M1) |
| `sync.cursor_advance_every` | `BOTHY_SYNC__CURSOR_ADVANCE_EVERY` | Server |  | `1000` | one `cursor` frame after this many skipped events | planned (M1) |
| `sync.outbound_queue_max` | `BOTHY_SYNC__OUTBOUND_QUEUE_MAX` | Server |  | `10000` | frames in the outbound queue of a connection until `4014` | planned (M1) |
| `sync.idempotency_key_ttl` | `BOTHY_SYNC__IDEMPOTENCY_KEY_TTL` | Server |  | `24h` | retention of `Idempotency-Key` | planned (M1) |
| `sync.max_focused_threads` | `BOTHY_SYNC__MAX_FOCUSED_THREADS` | Server |  | `20` | threads in `focus` per connection | planned (M1) |
| `sync.run_patch_coalesce_interval` | `BOTHY_SYNC__RUN_PATCH_COALESCE_INTERVAL` | Server |  | `250ms` | collection window for `run.updated` | planned (M2) |
| `sync.typing_min_interval` | `BOTHY_SYNC__TYPING_MIN_INTERVAL` | Server |  | `5s` | `typing.start` at most this often | planned (M1) |
| `sync.typing_ttl` | `BOTHY_SYNC__TYPING_TTL` | Server |  | `8s` | typing indicator without renewal | planned (M1) |
| `client.cache_max_age` | `BOTHY_CLIENT__CACHE_MAX_AGE` | Server |  | `30d` | unused cache entries | planned (M3) |
| `client.outbox_retry_base` | `BOTHY_CLIENT__OUTBOX_RETRY_BASE` | Server |  | `2s` | base of the backoff in the outbox | planned (M4) |
| `client.outbox_retry_cap` | `BOTHY_CLIENT__OUTBOX_RETRY_CAP` | Server |  | `5m` | largest interval in the outbox | planned (M4) |
| `agent.invocation_token_ttl` | `BOTHY_AGENT__INVOCATION_TOKEN_TTL` | Server |  | `60s` | validity of an invocation token | planned (M1) |
| `agent.ack_timeout` | `BOTHY_AGENT__ACK_TIMEOUT` | Server |  | `30s` | without `ack`: redelivery | planned (M2) |
| `agent.ack_persist_interval` | `BOTHY_AGENT__ACK_PERSIST_INTERVAL` | Server |  | `1s` | `acked_seq` is stored at least this often | planned (M2) |
| `agent.max_inflight_events` | `BOTHY_AGENT__MAX_INFLIGHT_EVENTS` | Server |  | `100` | unacknowledged events per consumer | planned (M2) |
| `agent.run_update_min_interval` | `BOTHY_AGENT__RUN_UPDATE_MIN_INTERVAL` | Server |  | `500ms` | adapter throttle for `run.update` | planned (M2) |
| `agent.presence_timeout` | `BOTHY_AGENT__PRESENCE_TIMEOUT` | Server |  | `90s` | without a heartbeat, the agent counts as `offline` | planned (M2) |
| `agent.cancel_grace` | `BOTHY_AGENT__CANCEL_GRACE` | Server |  | `30s` | time for the adapter to report a cancellation | planned (M2) |

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
| `api.page_size_default` | `BOTHY_API__PAGE_SIZE_DEFAULT` | Server |  | `50` | page size without `limit` | planned (M1) |
| `api.page_size_max` | `BOTHY_API__PAGE_SIZE_MAX` | Server |  | `200` | larger `limit` values are capped | planned (M1) |
| `api.cors_allowed_origins` | `BOTHY_API__CORS_ALLOWED_ORIGINS` | Server | list(string) | empty | additional origins for CORS, each in the form of `server.public_url` | active |
| `api.deprecation_min_notice` | `BOTHY_API__DEPRECATION_MIN_NOTICE` | Server |  | `180d` | minimum gap from `Deprecation` to `Sunset` | planned (M1) |
| `api.major_version_overlap` | `BOTHY_API__MAJOR_VERSION_OVERLAP` | Server |  | `365d` | how long an old major version stays after the new one appears | planned (M7) |
| `api.stats_max_buckets` | `BOTHY_API__STATS_MAX_BUCKETS` | Server |  | `1000` | time buckets per statistics query; more yields `400` | planned (M6) |
| `message.list_content_max_bytes` | `BOTHY_MESSAGE__LIST_CONTENT_MAX_BYTES` | Server |  | `16384` | largest `content_md` per message in lists; above that truncated with `content_truncated` (section 6) | planned (M1) |
| `status.history_window` | `BOTHY_STATUS__HISTORY_WINDOW` | Server |  | `7d` | how long the status page shows resolved incidents | planned (M7) |

### 07 Permissions and authorization

Source: [07-permissions.md](../spec/07-permissions.md), section 14.

| Key | Environment | Place | Type | Default | Meaning | Status |
|---|---|---|---|---|---|---|
| `authz.expiry_sweep_interval` | `BOTHY_AUTHZ__EXPIRY_SWEEP_INTERVAL` | Server |  | `1m` | interval of the job that reports expiries and writes reminders | planned (M1) |
| `authz.expiry_reminder_lead` | `BOTHY_AUTHZ__EXPIRY_REMINDER_LEAD` | Server |  | `48h` | how long before expiry the reminder comes (Study C: two days) | planned (M1) |
| `authz.expiry_sweep_batch_max` | `BOTHY_AUTHZ__EXPIRY_SWEEP_BATCH_MAX` | Server |  | `500` | rows per transaction of the jobs `authz.expiry_sweep` and `authz.expiry_reminder` | planned (M1) |
| `authz.view_as_max_duration` | `BOTHY_AUTHZ__VIEW_AS_MAX_DURATION` | Server |  | `15m` | maximum duration of "View as person" | planned (M1) |
| `authz.snapshot_cache_max_entries` | `BOTHY_AUTHZ__SNAPSHOT_CACHE_MAX_ENTRIES` | Server | int | `10000` | permission snapshots in memory per process (LRU) | active |
| `authz.not_found_timing_tolerance` | `BOTHY_AUTHZ__NOT_FOUND_TIMING_TOLERANCE` | Server |  | `20ms` | allowed deviation of the median in AZ-P4 | planned (M1) |

### 08 Identity, devices and sessions

Source: [08-identity.md](../spec/08-identity.md), section 18.

| Key | Environment | Place | Type | Default | Meaning | Status |
|---|---|---|---|---|---|---|
| `auth.access_token_ttl` | `BOTHY_AUTH__ACCESS_TOKEN_TTL` | Server |  | `10m` | Lifetime of an access token | planned (M1) |
| `auth.dpop_nonce_ttl` | `BOTHY_AUTH__DPOP_NONCE_TTL` | Server |  | `5m` | Validity of a DPoP nonce (`DPoP-Nonce` header and `hello` of the gateway) | planned (M1) |
| `auth.token_pepper_ref` | `BOTHY_AUTH__TOKEN_PEPPER_REF` | Server |  | `file:<secrets.dir>/token_pepper` | Reference to the server pepper for the HMAC hashes of all tokens and codes (03, principle 9) | planned (M1) |
| `auth.clock_leeway` | `BOTHY_AUTH__CLOCK_LEEWAY` | Server |  | `30s` | allowed clock skew for `exp`, `nbf`, `iat` | planned (M1) |
| `auth.dpop_proof_max_age` | `BOTHY_AUTH__DPOP_PROOF_MAX_AGE` | Server |  | `60s` | Maximum age of a DPoP proof (`iat`) | planned (M1) |
| `auth.login_flow_ttl` | `BOTHY_AUTH__LOGIN_FLOW_TTL` | Server |  | `10m` | Validity of a login flow | planned (M1) |
| `auth.server_code_ttl` | `BOTHY_AUTH__SERVER_CODE_TTL` | Server |  | `60s` | Validity of the one-time server code | planned (M1) |
| `auth.oidc_jwks_cache_ttl` | `BOTHY_AUTH__OIDC_JWKS_CACHE_TTL` | Server |  | `1h` | Cache of the provider metadata (discovery) and provider keys (JWKS) | planned (M1) |
| `auth.oidc_client_assertion_ttl` | `BOTHY_AUTH__OIDC_CLIENT_ASSERTION_TTL` | Server |  | `2m` | Validity of a client assertion with `private_key_jwt` | planned (M1) |
| `auth.group_claim_max_age` | `BOTHY_AUTH__GROUP_CLAIM_MAX_AGE` | Server |  | `30d` | Expiry of memberships from group claims after the last login | planned (M1) |
| `auth.idp_recheck_interval` | `BOTHY_AUTH__IDP_RECHECK_INTERVAL` | Server |  | `30d` | new login at the provider at the latest after this | planned (M1) |
| `auth.trusted_header_bind_requests` | `BOTHY_AUTH__TRUSTED_HEADER_BIND_REQUESTS` | Server |  | `true` | every request must carry the same header login | planned (M1) |
| `auth.enrollment_code_ttl` | `BOTHY_AUTH__ENROLLMENT_CODE_TTL` | Server |  | `72h` | Validity of an enrollment code | planned (M1) |
| `auth.enrollment_code_max_attempts` | `BOTHY_AUTH__ENROLLMENT_CODE_MAX_ATTEMPTS` | Server |  | `5` | Failed attempts per enrollment code | planned (M1) |
| `auth.device_pending_ttl` | `BOTHY_AUTH__DEVICE_PENDING_TTL` | Server |  | `24h` | after this, a pending device is revoked | planned (M1) |
| `auth.device_verification_code_digits` | `BOTHY_AUTH__DEVICE_VERIFICATION_CODE_DIGITS` | Server |  | `6` | Length of the verification code | planned (M1) |
| `auth.pairing_code_ttl` | `BOTHY_AUTH__PAIRING_CODE_TTL` | Server |  | `5m` | Validity of a pairing code | planned (M1) |
| `auth.pairing_code_chars` | `BOTHY_AUTH__PAIRING_CODE_CHARS` | Server |  | `8` | Length of the pairing code (Crockford Base32, 40 bits) | planned (M1) |
| `auth.device_inactive_revoke_after` | `BOTHY_AUTH__DEVICE_INACTIVE_REVOKE_AFTER` | Server |  | `180d` | a device without contact is revoked | planned (M1) |
| `auth.cleanup_interval` | `BOTHY_AUTH__CLEANUP_INTERVAL` | Server |  | `1h` | Interval of the job `identity.auth_cleanup` (delete expired challenges, login flows and pairings, revoke pending and unused devices) | planned (M1) |
| `auth.step_up_challenge_ttl` | `BOTHY_AUTH__STEP_UP_CHALLENGE_TTL` | Server |  | `2m` | Validity of a step-up challenge | planned (M1) |
| `auth.step_up_fail_limit` | `BOTHY_AUTH__STEP_UP_FAIL_LIMIT` | Server |  | `5` | Failed step-up attempts per window | planned (M1) |
| `auth.require_key_attestation` | `BOTHY_AUTH__REQUIRE_KEY_ATTESTATION` | Server |  | `false` | Require platform attestation of the device keys | planned (V3) |
| `auth.console_session_max_age` | `BOTHY_AUTH__CONSOLE_SESSION_MAX_AGE` | Server |  | `12h` | absolute duration of a console session | planned (M6) |
| `auth.console_session_idle_timeout` | `BOTHY_AUTH__CONSOLE_SESSION_IDLE_TIMEOUT` | Server |  | `30m` | idle time until the console session ends | planned (M6) |
| `auth.local_password_enabled` | `BOTHY_AUTH__LOCAL_PASSWORD_ENABLED` | Server |  | `true` | password with TOTP as a fallback for local accounts | planned (M1) |
| `auth.totp_key_ref` | `BOTHY_AUTH__TOTP_KEY_REF` | Server |  | `file:<secrets.dir>/totp_key` | Reference to the key (32 bytes) with which TOTP secrets are encrypted | planned (M1) |
| `auth.webauthn_rp_id` | `BOTHY_AUTH__WEBAUTHN_RP_ID` | Server |  | empty | RP ID of the passkeys; empty means the host of `server.public_url` | planned (M1) |
| `auth.webauthn_origins` | `BOTHY_AUTH__WEBAUTHN_ORIGINS` | Server |  | empty | allowed origins of the passkeys; empty means the origin of `server.public_url`; entries may also be app origins such as `android:apk-key-hash:...` | planned (M1) |
| `auth.password_min_length` | `BOTHY_AUTH__PASSWORD_MIN_LENGTH` | Server |  | `12` | Minimum length of a password | planned (M1) |
| `auth.argon2id_memory_kib` | `BOTHY_AUTH__ARGON2ID_MEMORY_KIB` | Server |  | `65536` | Argon2id memory (lower bound 19456) | planned (M1) |
| `auth.argon2id_iterations` | `BOTHY_AUTH__ARGON2ID_ITERATIONS` | Server |  | `3` | Argon2id iterations (lower bound 2) | planned (M1) |
| `auth.argon2id_parallelism` | `BOTHY_AUTH__ARGON2ID_PARALLELISM` | Server |  | `1` | Argon2id parallelism | planned (M1) |
| `auth.login_fail_limit` | `BOTHY_AUTH__LOGIN_FAIL_LIMIT` | Server |  | `10` | Failed attempts per account and window | planned (M1) |
| `auth.login_fail_window` | `BOTHY_AUTH__LOGIN_FAIL_WINDOW` | Server |  | `15m` | Window for failed attempts | planned (M1) |
| `auth.login_lock_duration` | `BOTHY_AUTH__LOGIN_LOCK_DURATION` | Server |  | `15m` | Lockout of the password path | planned (M1) |
| `auth.dpop_required_for` | `BOTHY_AUTH__DPOP_REQUIRED_FOR` | Server |  | `["agent"]` | Token kinds that must be DPoP-bound | planned (M1) |
| `auth.api_token_max_ttl` | `BOTHY_AUTH__API_TOKEN_MAX_TTL` | Server |  | `365d` | Maximum duration of an agent or app token | planned (M1) |
| `auth.personal_token_max_ttl` | `BOTHY_AUTH__PERSONAL_TOKEN_MAX_TTL` | Server |  | `90d` | Maximum duration of a personal token | planned (M1) |
| `client.app_lock_after` | `BOTHY_CLIENT__APP_LOCK_AFTER` | Server |  | `5m` | time without use until the app lock | planned (M5) |
| `security.app_lock_required` | `BOTHY_SECURITY__APP_LOCK_REQUIRED` | Server |  | `false` | Require the app lock instance-wide | planned (M5) |

### 09 Agents

Source: [09-agents.md](../spec/09-agents.md), section 21.

| Key | Environment | Place | Type | Default | Meaning | Status |
|---|---|---|---|---|---|---|
| `agent.context_max_bytes` | `BOTHY_AGENT__CONTEXT_MAX_BYTES` | Server |  | `262144` | sum of the contents in the context without the triggering message | planned (M2) |
| `agent.context_message_max_bytes` | `BOTHY_AGENT__CONTEXT_MESSAGE_MAX_BYTES` | Server |  | `32768` | content per context message, truncated above that | planned (M2) |
| `agent.jwks_max_age` | `BOTHY_AGENT__JWKS_MAX_AGE` | Server |  | `1h` | cache duration of the JWKS; lead time of a `next` key | planned (M1) |
| `signing_key.invocation_rotation_interval` | `BOTHY_SIGNING_KEY__INVOCATION_ROTATION_INTERVAL` | Server |  | `30d` | scheduled change of the token key | planned (M1) |
| `run.queued_max_age` | `BOTHY_RUN__QUEUED_MAX_AGE` | Server |  | `48h` | the maximum age a run that has not started may reach | planned (M2) |
| `run.orphan_timeout` | `BOTHY_RUN__ORPHAN_TIMEOUT` | Server |  | `15m` | without a heartbeat during an active run: `stale` | planned (M2) |
| `interaction.ack_timeout` | `BOTHY_INTERACTION__ACK_TIMEOUT` | Server |  | `30s` | after that the card shows "not acknowledged" | planned (M2) |
| `command.autocomplete_timeout` | `BOTHY_COMMAND__AUTOCOMPLETE_TIMEOUT` | Server |  | `2500ms` | wait time for suggestions | planned (M2) |
| `command.autocomplete_max_choices` | `BOTHY_COMMAND__AUTOCOMPLETE_MAX_CHOICES` | Server |  | `25` | suggestions per answer | planned (M2) |
| `command.max_per_principal` | `BOTHY_COMMAND__MAX_PER_PRINCIPAL` | Server |  | `100` | commands per agent or app | planned (M2) |
| `command.description_max_chars` | `BOTHY_COMMAND__DESCRIPTION_MAX_CHARS` | Server |  | `100` | length of a description | planned (M2) |
| `component.max_per_message` | `BOTHY_COMPONENT__MAX_PER_MESSAGE` | Server |  | `25` | components per message | planned (M2) |
| `component.custom_id_max_chars` | `BOTHY_COMPONENT__CUSTOM_ID_MAX_CHARS` | Server |  | `100` | length of `custom_id` | planned (M2) |
| `component.form_max_fields` | `BOTHY_COMPONENT__FORM_MAX_FIELDS` | Server |  | `10` | fields per form | planned (M2) |
| `thread.placeholder_titles` | `BOTHY_THREAD__PLACEHOLDER_TITLES` | Server |  | `["", "Neuer Chat", "New chat", "Chat", "Ohne Titel", "Untitled", "Test"]` | placeholders for the title detection | planned (M1) |
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
| `billing.mismatch_alert_after` | `BOTHY_BILLING__MISMATCH_ALERT_AFTER` | Server |  | `5m` | how long the reported mode may deviate from the configured mode before an alert goes out | planned (M2) |
| `reconciler.min_request_interval` |  | Reconciliation app |  | `60s` | Interval between requests to the Cost API |  |
| `reconciler.lookback` |  | Reconciliation app |  | `3d` | this many days back are reconciled |  |
| `reconciler.workspace_map` |  | Reconciliation app |  | empty | Assignment of workspace to agent |  |

### 11 Extensions: apps, webhooks, SDK

Source: [11-extensions.md](../spec/11-extensions.md), section 16.

| Key | Environment | Place | Type | Default | Meaning | Status |
|---|---|---|---|---|---|---|
| `app.manifest_max_bytes` | `BOTHY_APP__MANIFEST_MAX_BYTES` | Server |  | `65536` | size of a manifest | planned (M2) |
| `app.interaction_timeout` | `BOTHY_APP__INTERACTION_TIMEOUT` | Server |  | `3s` | deadline for synchronous responses through `interactions_url` | planned (M2) |
| `webhook.delivery_timeout` | `BOTHY_WEBHOOK__DELIVERY_TIMEOUT` | Server |  | `15s` | timeout of a delivery | planned (M1) |
| `webhook.max_pending_per_subscription` | `BOTHY_WEBHOOK__MAX_PENDING_PER_SUBSCRIPTION` | Server |  | `10000` | open deliveries per subscription, above that `paused` | planned (M2) |
| `webhook.retry_max_age` | `BOTHY_WEBHOOK__RETRY_MAX_AGE` | Server |  | `72h` | after that a delivery becomes `dead` ([webhooks.md](../api/webhooks.md), section 4) | planned (M2) |
| `webhook.secret_rotation_overlap` | `BOTHY_WEBHOOK__SECRET_ROTATION_OVERLAP` | Server |  | `24h` | how long the server signs with the old and the new HMAC generation | planned (M2) |
| `webhook.receiver_tolerance` |  | Receiver |  | `5m` | recommendation, not a server value: tolerance for `webhook-timestamp` at the receiver and default of the check function of the SDKs |  |
| `webhook_in.max_body_bytes` | `BOTHY_WEBHOOK_IN__MAX_BODY_BYTES` | Server |  | `1048576` | size of an incoming body | planned (M2) |
| `webhook_in.formats_enabled` | `BOTHY_WEBHOOK_IN__FORMATS_ENABLED` | Server |  | `native, alertmanager` | enabled incoming formats | planned (M2) |
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
| `file.fs.min_free_bytes` | `BOTHY_FILE__FS__MIN_FREE_BYTES` | Server |  | `2147483648` | free space below which uploads are rejected with `503 unavailable` (`reason` `storage_full`) | planned (M3) |
| `file.s3.endpoint` | `BOTHY_FILE__S3__ENDPOINT` | Server |  | empty | S3 endpoint | planned (M3) |
| `file.s3.region` | `BOTHY_FILE__S3__REGION` | Server |  | `us-east-1` | region for the signature | planned (M3) |
| `file.s3.bucket` | `BOTHY_FILE__S3__BUCKET` | Server |  | empty | bucket | planned (M3) |
| `file.s3.prefix` | `BOTHY_FILE__S3__PREFIX` | Server |  | empty | prefix before all keys | planned (M3) |
| `file.s3.use_path_style` | `BOTHY_FILE__S3__USE_PATH_STYLE` | Server |  | `true` | path style instead of virtual hosts | planned (M3) |
| `file.s3.checksum_mode` | `BOTHY_FILE__S3__CHECKSUM_MODE` | Server |  | `when_required` | SDK checksums for request and response | planned (M3) |
| `file.s3.access_key_ref` | `BOTHY_FILE__S3__ACCESS_KEY_REF` | Server |  | `file:<secrets.dir>/s3_access_key` | references to the secret provider; read only with `file.storage.driver = s3` | planned (M3) |
| `file.s3.secret_key_ref` | `BOTHY_FILE__S3__SECRET_KEY_REF` | Server |  | `file:<secrets.dir>/s3_secret_key` | references to the secret provider; read only with `file.storage.driver = s3` | planned (M3) |
| `file.s3.presigned_upload` | `BOTHY_FILE__S3__PRESIGNED_UPLOAD` | Server |  | `false` | allow mode `presigned` | planned (M3) |
| `file.s3.presign_ttl` | `BOTHY_FILE__S3__PRESIGN_TTL` | Server |  | `15m` | validity of a presigned URL | planned (M3) |
| `file.upload_part_bytes` | `BOTHY_FILE__UPLOAD_PART_BYTES` | Server |  | `8388608` | part size | planned (M3) |
| `file.upload_session_ttl` | `BOTHY_FILE__UPLOAD_SESSION_TTL` | Server |  | `24h` | lifetime of an open session | planned (M3) |
| `file.upload_max_open_sessions` | `BOTHY_FILE__UPLOAD_MAX_OPEN_SESSIONS` | Server |  | `20` | open sessions per principal | planned (M3) |
| `file.quota_bytes_per_principal` | `BOTHY_FILE__QUOTA_BYTES_PER_PRINCIPAL` | Server |  | `0` | quota per principal, 0 = off | planned (M3) |
| `file.poster_max_bytes` | `BOTHY_FILE__POSTER_MAX_BYTES` | Server |  | `524288` | size of the video thumbnail from the client | planned (M3) |
| `file.sniff_bytes` | `BOTHY_FILE__SNIFF_BYTES` | Server |  | `1048576` | bytes for type detection | planned (M3) |
| `file.denied_mime_types` | `BOTHY_FILE__DENIED_MIME_TYPES` | Server |  | empty | blocked types | planned (M3) |
| `file.name_max_bytes` | `BOTHY_FILE__NAME_MAX_BYTES` | Server |  | `255` | length of a file name in UTF-8 bytes | planned (M3) |
| `file.scan.mode` | `BOTHY_FILE__SCAN__MODE` | Server |  | `required` | `required` or `off` | planned (M3) |
| `file.scan.clamd_address` | `BOTHY_FILE__SCAN__CLAMD_ADDRESS` | Server |  | `tcp://clamav:3310` | clamd in the internal network; alternatively `unix:///...` | planned (M3) |
| `file.scan.chunk_bytes` | `BOTHY_FILE__SCAN__CHUNK_BYTES` | Server |  | `65536` | chunk size for `INSTREAM` | planned (M3) |
| `file.scan.timeout` | `BOTHY_FILE__SCAN__TIMEOUT` | Server |  | `120s` | time limit per scan | planned (M3) |
| `file.scan.max_concurrent` | `BOTHY_FILE__SCAN__MAX_CONCURRENT` | Server |  | `4` | scans per worker process | planned (M3) |
| `file.scan.retry_backoff_cap` | `BOTHY_FILE__SCAN__RETRY_BACKOFF_CAP` | Server |  | `5m` | largest interval between scan attempts | planned (M3) |
| `file.scan.health_interval` | `BOTHY_FILE__SCAN__HEALTH_INTERVAL` | Server |  | `30s` | `PING` interval | planned (M3) |
| `file.scan.pending_alert_after` | `BOTHY_FILE__SCAN__PENDING_ALERT_AFTER` | Server |  | `10m` | age of the oldest `pending` file before the alert | planned (M3) |
| `file.scan.clamd_stream_max_bytes` | `BOTHY_FILE__SCAN__CLAMD_STREAM_MAX_BYTES` | Server |  | `104857600` | mirror of clamd `StreamMaxLength` | planned (M3) |
| `file.invocation_hold_max` | `BOTHY_FILE__INVOCATION_HOLD_MAX` | Server |  | `120s` | longest wait of the agent invocation for attachments | planned (M3) |
| `file.image.libvips_enabled` | `BOTHY_FILE__IMAGE__LIBVIPS_ENABLED` | Server |  | `false` | use the libvips build | planned (M3) |
| `file.image.max_pixels` | `BOTHY_FILE__IMAGE__MAX_PIXELS` | Server |  | `50000000` | largest decoded pixel count | planned (M3) |
| `file.image.jpeg_quality` | `BOTHY_FILE__IMAGE__JPEG_QUALITY` | Server |  | `90` | quality when converting still WebP images without alpha to JPEG | planned (M3) |
| `file.image.processing_timeout` | `BOTHY_FILE__IMAGE__PROCESSING_TIMEOUT` | Server |  | `60s` | time limit of an image job | planned (M3) |
| `file.image.max_concurrent` | `BOTHY_FILE__IMAGE__MAX_CONCURRENT` | Server |  | `2` | image jobs per worker process | planned (M3) |
| `file.thumbnail.small_px` | `BOTHY_FILE__THUMBNAIL__SMALL_PX` | Server |  | `320` | longest edge `small` | planned (M3) |
| `file.thumbnail.medium_px` | `BOTHY_FILE__THUMBNAIL__MEDIUM_PX` | Server |  | `1024` | longest edge `medium` | planned (M3) |
| `file.thumbnail.large_px` | `BOTHY_FILE__THUMBNAIL__LARGE_PX` | Server |  | `2048` | longest edge `large` | planned (M3) |
| `file.thumbnail.jpeg_quality` | `BOTHY_FILE__THUMBNAIL__JPEG_QUALITY` | Server |  | `82` | quality of the thumbnails | planned (M3) |
| `file.ticket_ttl` | `BOTHY_FILE__TICKET_TTL` | Server |  | `5m` | validity of a file ticket | planned (M3) |
| `file.ticket_key_ref` | `BOTHY_FILE__TICKET_KEY_REF` | Server |  | `file:<secrets.dir>/file_ticket_key` | reference to the ticket key in the secret provider | planned (M3) |
| `file.unattached_ttl` | `BOTHY_FILE__UNATTACHED_TTL` | Server |  | `24h` | period for never attached files | planned (M3) |
| `file.failed_row_ttl` | `BOTHY_FILE__FAILED_ROW_TTL` | Server |  | `7d` | period for rows of failed or blocked files | planned (M3) |
| `file.gc_interval` | `BOTHY_FILE__GC_INTERVAL` | Server |  | `10m` | interval of the cleanup job | planned (M3) |
| `voice.max_duration` | `BOTHY_VOICE__MAX_DURATION` | Server |  | `15m` | longest voice message | planned (M1) |
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
| `push.max_age` | `BOTHY_PUSH__MAX_AGE` | Server |  | `1h` | a notification not yet sent that is older than this is discarded | planned (M3) |
| `push.retry_base` | `BOTHY_PUSH__RETRY_BASE` | Server |  | `2s` | base of the backoff | planned (M3) |
| `push.retry_cap` | `BOTHY_PUSH__RETRY_CAP` | Server |  | `5m` | largest backoff interval | planned (M3) |
| `push.max_concurrent_requests` | `BOTHY_PUSH__MAX_CONCURRENT_REQUESTS` | Server |  | `32` | concurrent requests to providers per process | planned (M3) |
| `push.apns.team_id` | `BOTHY_PUSH__APNS__TEAM_ID` | Server |  | empty | Apple team ID | planned (M3) |
| `push.apns.key_id` | `BOTHY_PUSH__APNS__KEY_ID` | Server |  | empty | key ID of the `.p8` key | planned (M3) |
| `push.apns.private_key_ref` | `BOTHY_PUSH__APNS__PRIVATE_KEY_REF` | Server |  | empty | reference to the key in the secret provider | planned (M3) |
| `push.apns.topics` | `BOTHY_PUSH__APNS__TOPICS` | Server |  | empty | allowed bundle IDs | planned (M3) |
| `push.apns.jwt_refresh_interval` | `BOTHY_PUSH__APNS__JWT_REFRESH_INTERVAL` | Server |  | `40m` | renewal of the provider JWT, must be between 20 and 60 minutes | planned (M3) |
| `push.apns.retry_5xx_delay` | `BOTHY_PUSH__APNS__RETRY_5XX_DELAY` | Server |  | `15m` | waiting time after `500` or `503` | planned (M3) |
| `push.apns.time_sensitive_enabled` | `BOTHY_PUSH__APNS__TIME_SENSITIVE_ENABLED` | Server |  | `false` | `time-sensitive` for `input_needed` | planned (M3) |
| `push.fcm.project_id` | `BOTHY_PUSH__FCM__PROJECT_ID` | Server |  | empty | Firebase project | planned (M3) |
| `push.fcm.service_account_ref` | `BOTHY_PUSH__FCM__SERVICE_ACCOUNT_REF` | Server |  | empty | reference to the service account key | planned (M3) |
| `notify.user_default_level` | `BOTHY_NOTIFY__USER_DEFAULT_LEVEL` | Server |  | `mentions` | level without any setting | planned (M3) |
| `notify.default_preview_mode` | `BOTHY_NOTIFY__DEFAULT_PREVIEW_MODE` | Server |  | `full` | default for `notify.preview_mode` | planned (M3) |
| `notify.default_input_needed_breaks_quiet_hours` | `BOTHY_NOTIFY__DEFAULT_INPUT_NEEDED_BREAKS_QUIET_HOURS` | Server |  | `false` | default for the switch of the same name | planned (M3) |
| `notify.default_admin_enabled` | `BOTHY_NOTIFY__DEFAULT_ADMIN_ENABLED` | Server |  | `true` | default for `notify.admin_enabled` | planned (M3) |
| `notify.coalesce_window` | `BOTHY_NOTIFY__COALESCE_WINDOW` | Server |  | `60s` | coalescing window per recipient and thread | planned (M3) |
| `notify.mobile_delay_when_desktop_active` | `BOTHY_NOTIFY__MOBILE_DELAY_WHEN_DESKTOP_ACTIVE` | Server |  | `60s` | delay of mobile pushes while the desktop is active | planned (M3) |
| `notify.preview_max_chars` | `BOTHY_NOTIFY__PREVIEW_MAX_CHARS` | Server |  | `240` | length of the excerpt in the preview | planned (M3) |
| `notify.badge_sync_min_interval` | `BOTHY_NOTIFY__BADGE_SYNC_MIN_INTERVAL` | Server |  | `60s` | smallest interval between badge syncs per person | planned (M3) |

### 14 Search

Source: [14-search.md](../spec/14-search.md), section 15.

| Key | Environment | Place | Type | Default | Meaning | Status |
|---|---|---|---|---|---|---|
| `search.languages` | `BOTHY_SEARCH__LANGUAGES` | Server |  | `["german","english"]` | configurations for query construction (plus always `simple`) | planned (M3) |
| `search.locale_map` | `BOTHY_SEARCH__LOCALE_MAP` | Server |  | `{"de":"german","en":"english"}` | language tag to text search configuration | planned (M3) |
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
| `secrets.signing_key_dir` | `BOTHY_SECRETS__SIGNING_KEY_DIR` | Server |  | `/var/lib/bothy/signing-keys` | writable directory of the private signing keys; owned by the service user, mode `0700` (5.2, [ADR 0066](../adr/0066-signing-keys-in-the-writable-key-store.md)) | planned (M1) |
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
| `migrate.require_recent_backup` | `BOTHY_MIGRATE__REQUIRE_RECENT_BACKUP` | Server |  | `true` | `migrate` requires a fresh backup | planned (M7) |
| `migrate.recent_backup_max_age` | `BOTHY_MIGRATE__RECENT_BACKUP_MAX_AGE` | Server |  | `24h` | how old this backup may be | planned (M7) |
| `migrate.lock_timeout` | `BOTHY_MIGRATE__LOCK_TIMEOUT` | Server | duration | `5s` | `lock_timeout` per migration step | active |
| `migrate.bootstrap_user` | `BOTHY_MIGRATE__BOOTSTRAP_USER` | Server | string | `postgres` | superuser login only for `migrate up --bootstrap` | active |
| `migrate.bootstrap_password_ref` | `BOTHY_MIGRATE__BOOTSTRAP_PASSWORD_REF` | Server | secret_ref | `file:<secrets.dir>/db_superuser_password` | its password | active |
| `migrate.owner_password_ref` | `BOTHY_MIGRATE__OWNER_PASSWORD_REF` | Server | secret_ref | `file:<secrets.dir>/db_owner_password` | password that `--bootstrap` sets for `bothy_owner` | active |
| `migrate.app_password_ref` | `BOTHY_MIGRATE__APP_PASSWORD_REF` | Server | secret_ref | `file:<secrets.dir>/db_app_password` | password for `bothy_app` | active |
| `migrate.maint_password_ref` | `BOTHY_MIGRATE__MAINT_PASSWORD_REF` | Server | secret_ref | `file:<secrets.dir>/db_maint_password` | password for `bothy_maint` | active |
| `migrate.monitor_password_ref` | `BOTHY_MIGRATE__MONITOR_PASSWORD_REF` | Server | secret_ref | `file:<secrets.dir>/db_monitor_password` | password for `bothy_monitor` | active |
| `backup.pg.full_schedule` | `BOTHY_BACKUP__PG__FULL_SCHEDULE` | Server |  | `0 2 * * 0` | schedule of the full backup (cron) | planned (M7) |
| `backup.pg.diff_schedule` | `BOTHY_BACKUP__PG__DIFF_SCHEDULE` | Server |  | `0 2 * * 1-6` | schedule of the differential backup (cron) | planned (M7) |
| `backup.pg.archive_timeout` | `BOTHY_BACKUP__PG__ARCHIVE_TIMEOUT` | Server |  | `60s` | a WAL segment is archived at the latest after this time | planned (M7) |
| `backup.pg.retention_full` | `BOTHY_BACKUP__PG__RETENTION_FULL` | Server |  | `4` | full backups kept in repo2 | planned (M7) |
| `backup.pg.local_retention_full` | `BOTHY_BACKUP__PG__LOCAL_RETENTION_FULL` | Server |  | `1` | full backups kept in repo1 | planned (M7) |
| `backup.logical.schedule` | `BOTHY_BACKUP__LOGICAL__SCHEDULE` | Server |  | `0 3 * * *` | schedule of `pg_dump` (cron) | planned (M7) |
| `backup.logical.retention` | `BOTHY_BACKUP__LOGICAL__RETENTION` | Server |  | `30d` | retention of the logical backup | planned (M7) |
| `backup.objects.schedule` | `BOTHY_BACKUP__OBJECTS__SCHEDULE` | Server |  | `30 3 * * *` | schedule of the object backup (cron) | planned (M7) |
| `backup.objects.retention` | `BOTHY_BACKUP__OBJECTS__RETENTION` | Server |  | `90d` | retention of the object snapshots | planned (M7) |
| `backup.offsite.target` | `BOTHY_BACKUP__OFFSITE__TARGET` | Server |  | empty | off-site target; empty means local only (warning in console and status) | planned (M7) |
| `backup.max_age` | `BOTHY_BACKUP__MAX_AGE` | Server |  | `26h` | no successful backup for longer than this: alert | planned (M7) |
| `backup.restore_test.schedule` | `BOTHY_BACKUP__RESTORE_TEST__SCHEDULE` | Server |  | `0 5 * * 6` | schedule of the restore test (cron) | planned (M7) |
| `backup.restore_test.sample_files` | `BOTHY_BACKUP__RESTORE_TEST__SAMPLE_FILES` | Server |  | `200` | files checked per test | planned (M7) |
| `backup.restore_test.sample_threads` | `BOTHY_BACKUP__RESTORE_TEST__SAMPLE_THREADS` | Server |  | `200` | threads checked per test | planned (M7) |
| `backup.restore_test.max_age` | `BOTHY_BACKUP__RESTORE_TEST__MAX_AGE` | Server |  | `8d` | no successful test for longer than this: alert | planned (M7) |
| `dr.rpo_target` | `BOTHY_DR__RPO_TARGET` | Server |  | `5m` | target for the largest data loss | planned (M7) |
| `dr.rto_target` | `BOTHY_DR__RTO_TARGET` | Server |  | `4h` | target for the duration of the restore | planned (M7) |
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
| `client.min_version.ios` | `BOTHY_CLIENT__MIN_VERSION__IOS` | Server |  | empty | smallest allowed app version on iOS; empty means no limit | planned (M1) |
| `client.min_version.android` | `BOTHY_CLIENT__MIN_VERSION__ANDROID` | Server |  | empty | the same for Android | planned (M1) |
| `client.min_version.macos` | `BOTHY_CLIENT__MIN_VERSION__MACOS` | Server |  | empty | the same for macOS | planned (M1) |
| `client.min_version.windows` | `BOTHY_CLIENT__MIN_VERSION__WINDOWS` | Server |  | empty | the same for Windows | planned (M1) |
| `client.recommended_version.ios` | `BOTHY_CLIENT__RECOMMENDED_VERSION__IOS` | Server |  | empty | below it the app shows an update notice | planned (M1) |
| `client.recommended_version.android` | `BOTHY_CLIENT__RECOMMENDED_VERSION__ANDROID` | Server |  | empty | the same for Android | planned (M1) |
| `client.recommended_version.macos` | `BOTHY_CLIENT__RECOMMENDED_VERSION__MACOS` | Server |  | empty | the same for macOS | planned (M1) |
| `client.recommended_version.windows` | `BOTHY_CLIENT__RECOMMENDED_VERSION__WINDOWS` | Server |  | empty | the same for Windows | planned (M1) |

### 19 Security

Source: [19-security.md](../spec/19-security.md), section 16.

| Key | Environment | Place | Type | Default | Meaning | Status |
|---|---|---|---|---|---|---|
| `http.hsts_max_age` | `BOTHY_HTTP__HSTS_MAX_AGE` | Server | duration | `31536000s` | `max-age` for HSTS | active |
| `egress.allowed_hosts` | `BOTHY_EGRESS__ALLOWED_HOSTS` | Server | list(string) | empty | additional allowed targets, form `host[:port][;allow_private]` ([configuration.md](../operations/configuration.md), section 2); duplicate entries are a load error | active |
| `egress.timeout` | `BOTHY_EGRESS__TIMEOUT` | Server | duration | `15s` | time limit of an outgoing request | active |
| `egress.max_response_bytes` | `BOTHY_EGRESS__MAX_RESPONSE_BYTES` | Server | int | `1048576` | largest response of an outgoing request | active |
| `audit.checkpoint_interval` | `BOTHY_AUDIT__CHECKPOINT_INTERVAL` | Server |  | `1h` | interval of the signed checkpoints of the audit log | planned (M1) |
| `audit.checkpoint_sink` | `BOTHY_AUDIT__CHECKPOINT_SINK` | Server |  | empty | storage target of the checkpoints outside the database: empty (warning, SEC-AUD-2) or `file://<absolute path>` on storage outside the database volume; `bothy config check` rejects other schemes. An object store with a retention lock is the recommended target for production and comes with PLAN M7-22 | planned (M1) |
| `audit.reason_max_chars` | `BOTHY_AUDIT__REASON_MAX_CHARS` | Server |  | `500` | maximum length of a mandatory reason of an administrative action in Unicode characters (SEC-AUD-5) | planned (M1) |
| `signing_key.cache_ttl` | `BOTHY_SIGNING_KEY__CACHE_TTL` | Server |  | `30s` | longest time a process still accepts a retired key or does not know a new one | planned (M1) |
| `signing_key.rotation_check_interval` | `BOTHY_SIGNING_KEY__ROTATION_CHECK_INTERVAL` | Server |  | `1m` | interval of the job `instance.key_rotation` | planned (M1) |
| `signing_key.access_token_rotation_interval` | `BOTHY_SIGNING_KEY__ACCESS_TOKEN_ROTATION_INTERVAL` | Server |  | `30d` | scheduled rotation of the key for access tokens | planned (M1) |

### 20 Privacy

Source: [20-privacy.md](../spec/20-privacy.md), section 13.

| Key | Environment | Place | Type | Default | Meaning | Status |
|---|---|---|---|---|---|---|
| `retention.content` | `BOTHY_RETENTION__CONTENT` | Server |  | `none` | period for contents; `none` means no automatic deletion (without `retention.content_none_reason` a warning, 4.1) | planned (M7) |
| `retention.content_none_reason` | `BOTHY_RETENTION__CONTENT_NONE_REASON` | Server |  | empty | reason of the instance operator for `retention.content = none`; empty means a warning in `bothy config check` and in the startup log (4.1) | planned (M7) |
| `retention.security_after_end` | `BOTHY_RETENTION__SECURITY_AFTER_END` | Server |  | `90d` | security data after end, expiry or revocation | planned (M7) |
| `retention.audit` | `BOTHY_RETENTION__AUDIT` | Server |  | `365d` | audit log (Study C suggested twelve months) | planned (M7) |
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
| `import.http_timeout` |  | Tool `bothy import` |  | `60s` | time limit per call to Discord and the CDN |  |
| `import.batch_max_items` |  | Tool `bothy import` |  | `100` | items per batch sent to the server |  |
| `import.reply_quote_max_chars` |  | Tool `bothy import` |  | `100` | length of the quote line for replies across threads |  |
| `import.follow.poll_interval` |  | Tool `bothy import` |  | `30s` | interval between mirror rounds |  |
| `import.follow.settle_after` |  | Tool `bothy import` |  | `60s` | quiet time after the last change |  |
| `import.follow.busy_reactions` |  | Tool `bothy import` |  | `👀` | reactions that mean "agent is still working" |  |
| `import.follow.busy_lookback` |  | Tool `bothy import` |  | `10` | messages before the first new one that are checked for the marker |  |
| `import.follow.max_backoff` |  | Tool `bothy import` |  | `5m` | longest interval after failed rounds |  |

### 24 Terminal

Source: [24-terminal.md](../spec/24-terminal.md), section 17.

| Key | Environment | Place | Type | Default | Meaning | Status |
|---|---|---|---|---|---|---|
| `terminal.pairing_code_ttl` | `BOTHY_TERMINAL__PAIRING_CODE_TTL` | Server |  | `15m` | Validity of a pairing code for a host | planned (M2) |
| `terminal.fingerprint_confirm_chars` | `BOTHY_TERMINAL__FINGERPRINT_CONFIRM_CHARS` | Server |  | `8` | this many characters of the fingerprint the owner types for the confirmation | planned (M2) |
| `terminal.fingerprint_max_attempts` | `BOTHY_TERMINAL__FINGERPRINT_MAX_ATTEMPTS` | Server |  | `3` | failed attempts at the confirmation, then fallback to `pairing` | planned (M2) |
| `terminal.step_up_max_age` | `BOTHY_TERMINAL__STEP_UP_MAX_AGE` | Server |  | `60s` | the step-up for `terminal.open` and `terminal.attach` must be at most this old | planned (M2) |
| `terminal.session_token_ttl` | `BOTHY_TERMINAL__SESSION_TOKEN_TTL` | Server |  | `30s` | Validity of a terminal session token | planned (M2) |
| `terminal.open_timeout` | `BOTHY_TERMINAL__OPEN_TIMEOUT` | Server |  | `20s` | Deadline until both sides are connected in the relay and the handshake is complete | planned (M2) |
| `terminal.detach_grace` | `BOTHY_TERMINAL__DETACH_GRACE` | Server |  | `15m` | this long a detached session stays attachable | planned (M2) |
| `terminal.quick_reattach_window` | `BOTHY_TERMINAL__QUICK_REATTACH_WINDOW` | Server |  | `60s` | Reattach from the same device without a new step-up within this time after the detach | planned (M2) |
| `terminal.idle_timeout` | `BOTHY_TERMINAL__IDLE_TIMEOUT` | Server |  | `30m` | without input from the device for this long, the session ends | planned (M2) |
| `terminal.idle_grace` | `BOTHY_TERMINAL__IDLE_GRACE` | Server |  | `2m` | Extra time after which the server itself ends the session because of idleness | planned (M2) |
| `terminal.max_session_duration` | `BOTHY_TERMINAL__MAX_SESSION_DURATION` | Server |  | `12h` | Maximum duration of a session | planned (M2) |
| `terminal.max_sessions_per_person` | `BOTHY_TERMINAL__MAX_SESSIONS_PER_PERSON` | Server |  | `5` | open sessions per person | planned (M2) |
| `terminal.max_sessions_per_host` | `BOTHY_TERMINAL__MAX_SESSIONS_PER_HOST` | Server |  | `10` | open sessions per host | planned (M2) |
| `terminal.relay_buffer_max_bytes` | `BOTHY_TERMINAL__RELAY_BUFFER_MAX_BYTES` | Server |  | `262144` | Buffer of the relay per direction and session | planned (M2) |
| `terminal.host_heartbeat_interval` | `BOTHY_TERMINAL__HOST_HEARTBEAT_INTERVAL` | Server |  | `20s` | Interval of the heartbeats of the control connection (reported in `hello`) | planned (M2) |
| `terminal.host_heartbeat_miss_limit` | `BOTHY_TERMINAL__HOST_HEARTBEAT_MISS_LIMIT` | Server |  | `3` | missing heartbeats until the control connection is closed | planned (M2) |
| `terminal.host_auth_timeout` | `BOTHY_TERMINAL__HOST_AUTH_TIMEOUT` | Server |  | `10s` | Deadline between `hello` and `identify` of the control and data connection | planned (M2) |
| `terminal.sweep_interval` | `BOTHY_TERMINAL__SWEEP_INTERVAL` | Server |  | `30s` | Interval of the job `terminal.sweep` ([02-architecture.md](../spec/02-architecture.md), section 12.3) | planned (M2) |
| `terminal.host_duplicate_probe_timeout` | `BOTHY_TERMINAL__HOST_DUPLICATE_PROBE_TIMEOUT` | Server |  | `5s` | this long the server waits for the answer of the older control connection (3.4) | planned (M2) |
| `host_service.state_dir` |  | Host service |  | `/var/lib/bothy-host` (Windows: `%ProgramData%\bothy-host`) | State directory with the keys of the host, the bindings of the session users to people and, with `organization`, the API key of the profile `claude`; never a subscription login (7.4) |  |
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
| `token_pepper` | generated | server | `auth.token_pepper_ref` |
| `totp_key` | generated, 32 bytes | server | `auth.totp_key_ref`: AES-256-GCM key of the TOTP secrets in `local_credentials` ([08-identity.md](../spec/08-identity.md) 2.5) |
| `telemetry_pseudonym_key` | generated, at least 32 bytes | server, collector | `telemetry.pseudonym_key_ref`, `hmac_key` of the allowlist |
| `audit_ip_kek` | generated, 32 bytes | server | `privacy.audit_ip_kek_ref`: encrypts the IP keys in `audit_ip_keys` |
| `file_ticket_key` | generated, 32 bytes | server | `file.ticket_key_ref`: HMAC of the file tickets ([12-files.md](../spec/12-files.md) 9.2) |
| `pgbackrest_cipher_pass` | generated | backup | encryption of the pgBackRest repositories |
| `pgbackrest_tls/` | generated (own CA) | postgres, backup | mutual TLS between the database and the repository host |
| `restic_password` | generated | backup | encryption of the restic repository |
| `s3_access_key`, `s3_secret_key` | generated (profile s3) or supplied (external service) | server, seaweedfs, backup | S3 access, `file.s3.access_key_ref` and `file.s3.secret_key_ref` |
| `grafana_admin_password` | generated | grafana | admin account in the local Grafana |
| `apns_key.p8` | supplied | server | push for iOS and macOS, `push.apns.private_key_ref` |
| `fcm_service_account.json` | supplied | server | push for Android |
| `oidc/<provider>_client_secret` | supplied | server | OIDC with `client_secret_basic` (`client_secret_ref` in `identity_providers.config`) |
| `oidc/<provider>_client_key.pem` | supplied | server | OIDC with `private_key_jwt`: private key of the client assertion, PKCS#8 (`client_key_ref`, [08-identity.md](../spec/08-identity.md) 2.6) |
| `offsite/` | supplied | backup | access to the off-site target (S3 keys or SSH key) |
| `alertmanager_bothy_webhook_url` | supplied | alertmanager | incoming webhook of the alert channel |
| `alertmanager_external_webhook_url` | supplied | alertmanager | external on-call channel |
| `alertmanager_heartbeat_url` | supplied | alertmanager | external heartbeat for the watchdog |
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

Generators of later milestones: `totp_key` (M1-39), `pgbackrest_tls/` (M7-01) and the generated
form of `s3_access_key` and `s3_secret_key` (profile s3, M3). Until then, `bothy secrets init`
reports these entries as `pending` as long as neither the store nor the source holds them; meanwhile
`s3_*` count as supplied. Passwords are generated as 32 random bytes in base64url without
padding (43 characters) and keys (`token_pepper`, `telemetry_pseudonym_key`, `audit_ip_kek`,
`file_ticket_key`) as 32 random bytes in lower-case hex (64 characters); users that need the raw
key bytes decode the hex text.

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
| `BOTHY_CLAMD_ADDRESS` | `tcp://clamav:3310` | sets `file.scan.clamd_address` (chapter 12) |
| `BOTHY_PG_SHM_SIZE` | `256m` | shared memory of the database container |
| `BOTHY_PG_ARCHIVE_TIMEOUT` | `60s` | sets `archive_timeout` in PostgreSQL; equal to `backup.pg.archive_timeout` |
| `BOTHY_BACKUP_OFFSITE_TARGET` | empty | sets `backup.offsite.target` |
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
| `BOTHY_CLAMAV_MEMORY_LIMIT` | `4g` | memory limit of the ClamAV container |
| `BOTHY_LOG_MAX_SIZE` | `20m` | size of a container log file until rotation |
| `BOTHY_LOG_MAX_FILE` | `5` | kept container log files per service |

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
