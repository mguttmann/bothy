# M1-09: Secret provider and `bothy secrets`

Status: ready | Depends on: M1-07 | Milestone: M1

## Goal

After this ticket every secret the server uses is read through `internal/platform/secrets` from a
reference `<scheme>:<path>` (drivers `file` and `env`), cached for `secrets.cache_ttl`, held in a
type that can never be printed, and picked up again after a rotation without a restart.
`bothy secrets init` prepares the per-service secret directories of the Compose stack
idempotently, and `bothy secrets export` writes an age-encrypted escrow bundle. The database
password (M1-12) and the telemetry pseudonym key (M1-10) build on this; SEC-SEC-1 to SEC-SEC-3,
SEC-T11 and SEC-T12 become tests.

## Read first

- `docs/spec/16-operations.md`: "Configuration and secrets" with all four subsections
  ("Configuration (ADR 0011)", "Secret provider (ADR 0011)", "Secrets in the Compose stack",
  "Escrow"), "Roles" under "Database", "Configuration values of this chapter"
  (`secrets.dir`, `secrets.cache_ttl`).
- `docs/spec/19-security.md`: "Logs without secrets and without content" (SEC-LOG-1),
  "Secrets" (SEC-SEC-1 to SEC-SEC-5), "Tests" (SEC-T11, SEC-T12).
- `docs/spec/02-architecture.md`: "Commands of the binary", "Code layout", "Configuration loading",
  "Failure of a dependency" (row secret provider), "Module register as the source", "go-arch-lint".
- `docs/spec/03-data-model.md`: "Principles", item 9 (secrets only as hash or `secret_ref`).
- `docs/operations/configuration.md`: "Naming rule and formats" (`secret_ref`), "Secrets"
  (the catalog table you implement and keep equal).
- `docs/operations/disaster-recovery.md` "Case B: host lost" (bundle files are adopted by
  `secrets-init`), `docs/operations/backup-restore.md` (escrow rows).
- `docs/adr/0011-12-factor-configuration-and-secret-provider.md`.
- `docs/spec/18-release.md` "Server images" (you fix the image user there).
- `docs/observability/signals.md` "Backup, audit, telemetry" (you add one metric row).
- `infra/compose/compose.yaml`: service `secrets-init` and every `subpath` mount.
- `docs/PLAN.md`: "Way of working per ticket", the M1 row M1-09 and the contradiction W3.
- Code you build on: `services/server/internal/platform/config` and `cmd/bothy` (M1-07, read
  their READMEs first), `internal/kernel/clock` and `internal/kernel/errs` (M1-05),
  `services/server/architecture/modules.yaml` and its generator (M1-05).
- No API operation, data model table or action id is touched by this ticket.

## Fixed requirements that apply

1. No MVP. New from scratch and directly in the final state; stages are only the order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined
   interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface with a docstring/doc
   comment (purpose, parameters, return value, errors, example where useful); comments explain the
   why, not the what. API documentation is generated from code (OpenAPI from the routes, dartdoc,
   Python API docs in MkDocs).
4. Documentation in the repo: README per module/service, ADR for every architecture decision,
   CHANGELOG per release, keep the documentation site current. A change without matching
   documentation is not done.
5. Quality gates (SonarQube, linters, type checking, security scanners, test coverage) block the
   merge.
6. Test first: write the failing test, show it red, then make it green, then clean up.
7. No en dash or em dash characters in any file you write (the `text-rules` hook rejects them).
8. White label: no company, person, host or domain names except `example.com`, people Alex, Sam,
   Robin; no paths under a user home directory; example credentials contain `EXAMPLE`.
9. Configuration only through named keys listed in `docs/operations/configuration.md`; prose and
   comments name the key, never its number.
10. SEC-SEC-1: the file driver reads only files with mode `0400` or `0600` owned by the service
    user; anything else aborts the start (test SEC-T11).
11. SEC-SEC-2 and SEC-T12: `config print` and diagnostics never show a secret, not even truncated.
12. SEC-SEC-3 and SEC-LOG-1: secrets never appear in logs, errors, events or telemetry; in code
    they have their own type that always renders as `[redacted]`.
13. 16 "Secret provider": references `<scheme>:<path>`; schemes `file` and `env` in M1; driver
    interface `Get(ref) -> (value, version)`, optional `Put`; values cached at most
    `secrets.cache_ttl`; a missing `*_ref` value means `file:<secrets.dir>/<name>`.
14. 02 "Failure of a dependency": the start aborts when a secret cannot be read; a failed
    rotation keeps the old value and raises an alarm signal.
15. 16 "Secrets in the Compose stack" and "Escrow": `secrets init` creates missing
    secrets, one subdirectory per service, never overwrites an existing generated secret, prints
    fingerprints and never values; `secrets export --age-recipient <public key>` writes an
    age-encrypted bundle.

## Scope

All Go paths are relative to `services/server/`.

### `internal/platform/secrets` (the provider)

Public interface (names fixed; signatures may add an `error` where Go needs one, nothing else):

- `type Ref struct { Scheme string; Path string }` with `func ParseRef(raw, secretsDir string) (Ref, error)`
  and `func (Ref) String() string`. `ParseRef` replaces the literal placeholder `<secrets.dir>`
  with `secretsDir` before parsing. Grammar: `file:<absolute path>` (after expansion the path must
  be absolute and clean, no `..` element); `env:<NAME>` with `NAME` matching `^[A-Z_][A-Z0-9_]*$`.
  The schemes `azkv` and `awssm` parse but yield `ErrUnsupportedScheme` (named in 16 as later
  drivers); any other scheme yields `ErrInvalidRef`.
- `type Value struct{ /* unexported []byte */ }` with `NewValue([]byte) Value`, `Bytes() []byte`
  (returns a copy), `Len() int`, `Equal(Value) bool` (constant time via `crypto/subtle`).
  `String`, `GoString`, `Format` (every verb including `%v %+v %#v %s %q %x %X`), `LogValue`
  (`slog.LogValuer`), `MarshalJSON`, `MarshalText` all produce exactly `[redacted]`.
- `type Version string`: opaque; for `file` and `env` it is the first 16 hex characters of the
  SHA-256 of the value, so equal content gives an equal version.
- `type Driver interface { Scheme() string; Get(ctx context.Context, ref Ref) (Value, Version, error) }`
  and `type Writer interface { Driver; Put(ctx context.Context, ref Ref, v Value) (Version, error) }`.
  No M1 driver implements `Writer`; the interface exists because 16 names `Put` for later drivers.
- `type Provider struct` built by `func NewProvider(opts Options) (*Provider, error)`;
  `Options{ Dir string; CacheTTL time.Duration; Clock clock.Clock; Logger *slog.Logger; Drivers []Driver }`
  (nil `Drivers` means `file` and `env`). Methods: `Get(ctx, Ref) (Value, Version, error)` and
  `Resolve(ctx, raw string) (Value, Version, error)` (parse plus get).
- Errors: `ErrInvalidRef`, `ErrUnsupportedScheme`, `ErrNotFound`, `ErrInsecureFile`; every error
  message names the reference and never contains file content or variable values.

### `internal/platform/secrets/stack` (Compose stack preparation)

- `Catalog`: the Go form of the table in `docs/operations/configuration.md` "Secrets"; one
  entry per row: name or name pattern, origin (`generated`, `supplied`), generator, services.
- `func Init(ctx context.Context, opts InitOptions) (Report, error)` and
  `func Export(ctx context.Context, opts ExportOptions) error`, used by the commands below.

### Commands in `cmd/bothy` (use the command framework already present)

- `bothy secrets init --target <dir> [--source <dir>] [--owner <service>=<uid>:<gid> ...] [--adopt-source]`
- `bothy secrets export --from <dir> --age-recipient <age1...> [--age-recipient ...] [--output <file>|-] [--armor]`
- `secrets init` and `secrets export` work without a configuration file and without a database;
  they read only their flags (the `secrets-init` container has no `BOTHY_` environment).
- `bothy config check --secrets`: additionally resolves every `*_ref` key through the provider and
  prints one line per key (`ok` or the error code), never a value.

### Changes to `internal/platform/config` (M1-07 code)

- Register `secrets.dir` (default `/run/bothy/secrets`) and `secrets.cache_ttl` (default `5m`)
  in the schema if M1-07 did not, typed into the `Config` for this package in `internal/wire`.
- Expand `<secrets.dir>` in the default and configured values of every `*_ref` key.
- Unknown-variable rule: an environment variable named by an `env:` reference of any effective
  `*_ref` key is not an unknown `BOTHY_` variable (see Design notes D2).

### Other files

- `internal/wire`: build one `*secrets.Provider` per process from the typed config.
- `architecture/modules.yaml`: register `filippo.io/age` (and `golang.org/x/sync` if you use
  `singleflight`) as vendors allowed for the platform component; regenerate `.go-arch-lint.yml`
  with the M1-05 generator.
- `infra/compose/compose.yaml` already passes `--owner server=65532:65532 --owner
  migrate=65532:65532` to `secrets-init` (16 "Secrets in the Compose stack" item 4); keep it.
- Docs listed under "Definition of done".

## Out of scope

- The database pool and how it uses `database.password_ref` (M1-12).
- The telemetry pseudonym (M1-10); this ticket only guarantees the generated key size.
- Signing keys: the server creates them in `secrets.signing_key_dir` (ADR 0066, M1-32); they are
  no catalogue entry. Generators for `pgbackrest_tls/` (M7-01) and the generated `s3_*` keys of the
  `s3` profile (M3) stay `pending` in the catalog (D6, configuration.md section 5).
- Drivers `azkv` and `awssm` (later milestone).
- The server image itself (M1-23; uid and gid `65532` are in 18 "Server images").
- A `promtool` test for the alarm `BothySecretRefreshFailing` (it exists in
  `infra/observability/prometheus/rules/bothy-alerts.yaml`; rule tests come with M1-04).

## Boundaries

- Work in a dedicated git worktree on branch `feat/m1-09-secret-provider`.
- Conventional Commits with scope (`feat(secrets): ...`, `docs(spec): ...`) and DCO sign-off
  (`git commit -s`); squash to meaningful commits before the merge.
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope above, except `services/server/CHANGELOG.md`,
  `docs/DECISIONS.md` and the docs named in "Definition of done".
- Follow the host rules supplied by the launcher. Stop only processes you started, by PID.
- Temporary directories only via `mktemp -d` (in Go tests `t.TempDir()`), removed at the end.
- Never read or print real secrets; tests use generated values containing `EXAMPLE` or a random
  canary.
- No network calls except package downloads for pinned modules and tools.
- If a needed piece of the core contract (chapters 00, 03, 04, 05) is missing, stop with
  `Verdict: BLOCKED` instead of building a deviation.

## Design notes

D1. File driver (`file:`).

- `os.Stat` (follows symlinks, so Kubernetes-style secret links work), then require a regular
  file, permission bits exactly `0400` or `0600`, and owner uid equal to `os.Geteuid()`;
  otherwise `ErrInsecureFile` naming the reference and the observed mode, never the content.
  Ownership checks use `syscall.Stat_t`; build the check for unix only and fail closed elsewhere.
- Missing file: `ErrNotFound`.
- Content: read whole file; strip exactly one trailing `\n` (and a `\r` directly before it).
  Reason: `echo value > file` is the usual way to write a secret and a trailing newline is never
  part of a password; generated secrets are ASCII without newline, so nothing else changes.

D2. Env driver (`env:`).

- Reads `os.LookupEnv(NAME)` on each uncached `Get`; unset is `ErrNotFound`; an empty value is
  `ErrNotFound` too.
- 16 shows `env:BOTHY_SECRET_DB_APP_PASSWORD`, but M1-07 aborts on unknown `BOTHY_` variables.
  Rule: after loading file and environment, the loader collects the variable names of all
  effective `env:` references and excludes exactly those from the unknown-variable check. Any
  other unknown `BOTHY_` variable still aborts (16 "Configuration (ADR 0011)" item 3).
- `env:` stays allowed but not recommended (16); `config check` prints a warning line per key
  that uses it.

D3. Cache and rotation.

- Entry per reference: value, version, time read. `Get` returns the cached value while
  `clock.Now() - readAt < secrets.cache_ttl`, else re-reads. Concurrent misses for the same
  reference read once (mutex per entry or `singleflight`).
- Re-read fails and a cached value exists: return the cached value, log at error level with the
  constant message `secret refresh failed, keeping previous value` and attribute `error.type`
  (error code only), increment counter `bothy.secrets.refresh.failures` (unit `{refresh}`, no
  attributes) through the global OpenTelemetry meter (`go.opentelemetry.io/otel` API only; M1-10
  installs the provider later, the global delegate forwards). Add this metric as a row to
  `docs/observability/signals.md` "Backup, audit, telemetry" (Prometheus name
  `bothy_secrets_refresh_failures_total`).
- Re-read fails without cached value: return the error (the start aborts, fail fast).

D4. `secrets init` layout and rules.

- Target layout: `<target>/_store/` holds the one canonical copy of every secret (directory
  `0700`, files `0400`, owned by the running user); `<target>/<service>/` holds the copies a
  service mounts with `subpath`. No service may be named `_store`. Relative names inside both are
  the catalog names (`db_app_password`, `oidc/<provider>_client_secret`, `offsite/...`), which is
  also the layout of `--source` and of the export bundle, so a decrypted bundle placed in the
  source directory is adopted (disaster recovery "Case B").
- Per catalog entry, reconcile `_store` with `--source`:

| store | source | generated entry | supplied entry |
|---|---|---|---|
| absent | absent | generate (`generated`) | report `missing` |
| absent | present | copy (`adopted`) | copy (`adopted`) |
| present | absent | keep (`kept`) | keep (`kept`) |
| present | equal | keep (`kept`) | keep (`kept`) |
| present | different | exit 1 naming the entry, unless `--adopt-source` (then `updated`) | replace (`updated`); the owner's source is the master for supplied secrets |

- A file in `--source` that matches no catalog entry: exit 2 naming it (a typo must not be
  silent, same reason as unknown configuration keys).
- Service directories: for each service, the expected set is every stored entry that lists the
  service. Write a copy only if it is absent or differs in content, mode or owner; remove files in
  the service directory that are not in the expected set. Writes are atomic: temp file in the same
  directory, `fsync`, `chmod`, `chown`, `rename`, `fsync` of the directory.
- A second run with unchanged inputs changes nothing: no write, no rename, no mtime change.
- Output on stdout, one line per entry: `<status> <name> <fingerprint>`; status is one of
  `generated`, `adopted`, `kept`, `updated`, `missing`, `pending`; fingerprint is `sha256:` plus
  the first 12 hex characters of the SHA-256 of the content (for a directory entry: of the sorted
  list of relative path and content hash pairs); `missing` and `pending` have no fingerprint. Last
  line: `back up the secrets: bothy secrets export --age-recipient <age public key>`.
- Exit codes: `0` success, `1` runtime failure or conflict, `2` usage error (unknown flag, unknown
  source file, invalid owner, missing owner for `server` or `migrate`).

D5. Generators (crypto/rand only).

- `token`: 32 random bytes, base64url without padding (43 ASCII characters). Used for
  `db_superuser_password`, `db_owner_password`, `db_app_password`, `db_maint_password`,
  `db_monitor_password`, `pgbackrest_cipher_pass`, `restic_password`, `grafana_admin_password`.
- `key`: 32 random bytes, lowercase hex (64 ASCII characters, so the collector's HMAC key and the
  server's key both have at least 32 bytes). Used for `token_pepper`, `telemetry_pseudonym_key`,
  `audit_ip_kek` and `file_ticket_key`; consumers that need raw key bytes hex-decode the value.
  (`totp_key` follows its own ticket, M1-39.)

D6. Catalog.

- Every row of `docs/operations/configuration.md` "Secrets" is a catalog entry with the same
  name, origin and services. Supplied directory entries: `oidc/` (files matching
  `^[a-z0-9_-]+_client_secret$`) and `offsite/` (any regular files, recursive).
- Entries whose generator belongs to another ticket get generator `pending` with a code comment
  naming the owning ticket: `pgbackrest_tls/` (M7-01) and the generated variant of
  `s3_access_key`, `s3_secret_key` (M3); until then `s3_*` are handled as supplied. `init` prints
  `pending` for them and exits 0 (configuration.md section 5 lists them). Signing keys are no
  catalogue entry (ADR 0066).
- A test parses the Markdown table and fails when name, origin or services differ from the Go
  catalog in either direction.

D7. Owners and modes of service copies.

- `--owner <service>=<uid>:<gid>` (repeatable): that service's directory is `0500` and its files
  `0400`, owned by `uid:gid`. Services without `--owner`: directory `0755`, files `0444`, owned by
  root. Reason: third-party images run as their own users that this repository does not control;
  isolation between services comes from the `subpath` mounts (each container sees only its own
  directory), and the volume root is `0700`.
- `server` and `migrate` must have an `--owner` (exit 2 otherwise), because the file driver
  enforces SEC-SEC-1 for them.
- The `bothy-server` image runs as uid and gid `65532` (18 "Server images" item 3); M1-23 builds
  the image with exactly this user.
- Running `init` as a non-root user is allowed when every `--owner` equals the current uid:gid
  (tests do this); copies for services without `--owner` are then owned by the running user
  instead of root. Any other failed `chown` is exit 1 with the entry name.

D8. `secrets export`.

- `--from <dir>` is required and names a `_store` directory (in Compose:
  `docker compose run --rm -T secrets-init secrets export --from /run/bothy/secrets/_store
  --age-recipient age1... > bundle.age`). It needs no configuration file and no database.
  Writes a POSIX tar stream (relative names, mode bits, no owner names, mtime zero) encrypted with
  `filippo.io/age` to every `--age-recipient` (X25519 `age1...`; at least one required, exit 2
  otherwise; invalid recipient exit 2).
- `--output -` (default) writes to stdout; refuse binary output to a terminal unless `--armor`
  (ASCII armor). `--output <file>` creates the file with mode `0600` and refuses to overwrite.
- Never writes plaintext anywhere; no temp files.

D9. W3 (subcommands). Record the plan recommendation as one entry in `docs/DECISIONS.md` if it is
not there yet: Ruling L1 is extended by `secrets`, `healthcheck`, `restore`; `bothy admin ...` in
08 becomes `bothy setup bootstrap-owner` and `bothy setup issue-enrollment-code`. Do not edit 08
(M1-37 does).

D10. Process wiring. `Provider` instances are created only in `internal/wire` and `cmd/bothy`;
modules receive values through their typed `Config` or a narrow interface, never read files or
environment themselves (02 "Configuration loading" item 5).

## Tests (write them first)

Unit tests in `internal/platform/secrets` (fake clock from `kernel/clock`, files in `t.TempDir()`):

- `TestParseRef`: table with valid `file:`, `env:`, `<secrets.dir>` expansion, relative path,
  `..`, empty path, lowercase env name, `azkv:` (unsupported), `foo:` (invalid).
- `TestValueNeverPrints`: canary inside `Value`; `fmt` with every verb, `slog` JSON and text
  handlers, `json.Marshal`, `errors` wrapping; output never contains the canary and is
  `[redacted]` where a rendering exists.
- `TestFileDriverRejectsInsecureModes` (SEC-T11): modes `0644`, `0640`, `0444`, `0700`; directory;
  file owned by another uid (skip only this subcase when the test runs as non-root and cannot
  create one; say so in the report); each yields `ErrInsecureFile` and the message has no content.
- `TestFileDriverAccepts0400And0600`, `TestFileDriverStripsOneTrailingNewline`,
  `TestFileDriverFollowsSymlink`.
- `TestEnvDriver`: set, unset, empty.
- `TestProviderCacheAndRotation`: value v1; file changed; before `secrets.cache_ttl` still v1;
  after advancing the fake clock v2 with a different `Version`.
- `TestProviderRefreshFailureKeepsOldValue`: file removed after first read; after TTL the old value
  is returned, one error log line with the constant message, counter incremented (manual reader
  from `go.opentelemetry.io/otel/sdk/metric` installed as global meter provider in the test).
- `TestProviderConcurrentMissReadsOnce`.

Unit tests in `internal/platform/secrets/stack`:

- `TestCatalogMatchesConfigurationDoc`: parses `docs/operations/configuration.md` "Secrets".
- `TestInitFreshTarget`: every generated entry present in `_store` and in each listed service
  directory with the mode and owner of D7 (owners = current uid:gid); `token` values decode to 32
  bytes; `TestPseudonymKeyAtLeast32Bytes`.
- `TestInitSecondRunChangesNothing`: snapshot of path, mode, uid, gid, size, mtime and SHA-256
  of every file before and after; identical; output has only `kept`, `missing`, `pending`.
- `TestInitReconcileTable`: one subtest per row of the table in D4, including the conflict exit 1
  and `--adopt-source`.
- `TestInitUnknownSourceFile` (exit 2), `TestInitRequiresOwnerForServerAndMigrate` (exit 2),
  `TestInitRemovesStaleServiceCopies`, `TestInitOutputHasNoValues` (canary in a supplied file).
- `TestExportRoundTrip`: export with a test identity from `age.GenerateX25519Identity()`, decrypt,
  untar; names and contents equal `_store`; `TestExportRefusesWithoutRecipient`,
  `TestExportRefusesTerminalWithoutArmor` (inject the terminal check),
  `TestExportOutputFileMode0600AndNoOverwrite`.

Tests for configuration and commands:

- `TestLoaderAllowsEnvVariablesNamedByRefs`: `BOTHY_DATABASE__PASSWORD_REF=env:BOTHY_SECRET_X` plus
  `BOTHY_SECRET_X=...` loads; an unrelated `BOTHY_SECRET_Y` still aborts and names it.
- `TestConfigPrintNeverShowsSecrets` (SEC-T12): canary in a referenced file and in a referenced
  env variable; run `config print --redacted` and `config check --secrets` through the command
  entry point with captured stdout and stderr; canary absent, references present.
- `TestConfigCheckSecretsReportsInsecureFile`: mode `0644` file gives a non-zero exit and the
  error code, no content.

Commands that must pass:

```sh
cd services/server && go test -race ./internal/platform/secrets/... ./internal/platform/config/... ./cmd/bothy/...
make arch
make check
```

## Acceptance criteria

- [ ] `file` and `env` drivers, `Provider` with cache and rotation exist with the interfaces above.
- [ ] A secret file with mode `0644` aborts with `ErrInsecureFile` (SEC-T11).
- [ ] No canary appears in `config print --redacted`, `config check --secrets`, logs or errors
      (SEC-T12, SEC-LOG-1).
- [ ] A changed secret file takes effect after `secrets.cache_ttl`; a failed refresh keeps the old
      value, logs once and increments `bothy.secrets.refresh.failures`.
- [ ] A second `secrets init` run changes nothing (byte, mode, owner and mtime snapshot).
- [ ] The generated telemetry pseudonym key has at least 32 bytes.
- [ ] The export bundle decrypts with a test identity and equals `_store`.
- [ ] The Go catalog equals the table in `docs/operations/configuration.md` "Secrets".
- [ ] `env:` references do not trip the unknown-variable check; other unknown `BOTHY_` variables do.
- [ ] `make arch` and `make check` are green; Go coverage of the module stays at or above the
      threshold `quality.coverage.go_min_percent`.

## Definition of done

- Code and tests as above; doc comment on every exported symbol.
- `internal/platform/secrets/README.md` and `internal/platform/secrets/stack/README.md`: purpose,
  interfaces, reference grammar, file rules, cache, init table, owners, export, what is pending.
- The spec already states D2 (16 5.1 item 3), D4 and D7 (16 5.3), the uid (18), the metric
  (signals.md) and the alarm. Still to write in the same change: 16 "Secret provider" (D1 newline
  rule, D3 cache), "Escrow" (D8 usage line); `docs/spec/02-architecture.md` "Commands of the
  binary" (flags of `secrets init|export`, `config check --secrets`);
  `docs/operations/disaster-recovery.md` "Case B" step 2 (`--adopt-source` when the volume
  already exists).
- `docs/DECISIONS.md`: one entry each for D1 (newline rule), D8 and D9.
- Configuration reference regenerated (`make config-reference`, or the M1-08 target
  `config-doc-check` if it exists) without diff.
- `services/server/CHANGELOG.md` entry under "Unreleased".
- No ADR needed: ADR 0011 already decides the provider; the rulings above are recorded in
  DECISIONS.md.
- `make check` green; independent review over the diff and red team after
  `docs/quality/red-team-gate.md` with verdict `JA` recorded in the report; merge locally into the
  base branch only if both are green, then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`: first line `Verdict: DONE` or `Verdict: BLOCKED <reason>`; then
commits, files with line counts, test results with counts (including the red run of each new test
before the implementation), what is not proven (for example the foreign-owner subcase when not
run as root), follow-ups that could not be done in scope with the reason. Then write
`{{DONE_PATH}}` with `exit=0 sha=<merge sha>` or `exit=1 <reason>`.
