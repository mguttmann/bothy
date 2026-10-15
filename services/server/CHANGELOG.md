# Changelog

All notable changes to the server component (`services/server`, binary `bothy`) are
documented in this file. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this component adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html). Its version is the
[`VERSION`](VERSION) file next to this one; release tags are `server/v<version>`
(docs/spec/18-release.md).

## [Unreleased]

### Added

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

### Changed

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

### Fixed

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
