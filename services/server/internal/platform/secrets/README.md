# Package `platform/secrets`

Status: provider built (M1-09). The normative sources are
[16-operations.md](../../../../../docs/spec/16-operations.md) "Secret provider (ADR 0011)",
[19-security.md](../../../../../docs/spec/19-security.md) "Secrets" (SEC-SEC-1 to
SEC-SEC-3) and "Logs without secrets and without content" (SEC-LOG-1) as well as
[02-architecture.md](../../../../../docs/spec/02-architecture.md) "Failure of a dependency"; if
they differ, the chapter applies. The preparation of the Compose stack (`bothy secrets
init|export`) lives in the subpackage [`stack`](stack/).

## Purpose

The only way the server reads a secret: from a reference `<scheme>:<path>` through a driver, kept
for at most `secrets.cache_ttl`, in a type that is never printed. A rotation thus takes effect
without a restart. Modules never read files or the environment themselves; they get values
through their `Config` or a narrow interface (02 "Configuration loading" item 5).

## Code

| File | Content |
|---|---|
| `ref.go` | `Ref`, `ParseRef`, grammar of the references, placeholder `<secrets.dir>` |
| `errors.go` | sentinel errors, `RefError`, `ErrorCode` |
| `value.go` | `Value` (always `[redacted]`), `Version` |
| `driver.go` | `Driver`, `Writer`, shared check at the start of `Get` |
| `file.go`, `file_unix.go`, `file_other.go` | file driver, owner check per platform |
| `env.go` | environment driver over a snapshot |
| `provider.go` | `Config`, `Options`, `Provider` with cache, rotation and metric |
| `doc.go` | package description |

## Interface

- `ParseRef(raw, secretsDir)` returns `Ref{Scheme, Path}`; `Ref.String()` returns
  `<scheme>:<path>`.
- `NewProvider(Options{Dir, CacheTTL, Clock, Logger, Drivers, Env})` builds the one provider of
  the process. Without `Drivers` the file driver and the environment driver over `Env` apply,
  without `Clock` the system clock, without `Logger` `slog.Default()`. Errors: `Dir` not
  absolute, not clean or `/`, a negative `CacheTTL`, a driver without a scheme, `nil` or a scheme
  twice.
- `(*Provider).Get(ctx, ref)` and `(*Provider).Resolve(ctx, raw)` (parsing plus `Get`) return
  `(Value, Version, error)`.
- `Driver{Scheme, Get}` and `Writer{Driver, Put}`; `NewFileDriver()`, `NewEnvDriver(environ)`.
- `Value`: `NewValue` copies, `Bytes` returns a copy, `Len`, `Equal` in constant time
  (`crypto/subtle`). `String`, `GoString`, `Format` (every verb, width and flags are ignored),
  `LogValue`, `MarshalJSON` and `MarshalText` give exactly `[redacted]`. The bytes sit behind a
  pointer: if `fmt` prints a `Value` through reflection (unexported struct field, verb `%p`),
  only an address appears. `==` therefore compares identity; for the content use `Equal`.

## Reference grammar

| Form | Rule |
|---|---|
| `file:<path>` | absolute, clean (`path.Clean` leaves it unchanged: no `.` or `..`, no double or trailing slash), without NUL |
| `env:<NAME>` | `NAME` matches `^[A-Z_][A-Z0-9_]*$` |
| `azkv:...`, `awssm:...` | `unsupported_scheme`: driver of a later milestone |
| anything else, no colon, empty scheme | `invalid_ref` |

Before parsing, `ParseRef` replaces every `<secrets.dir>` with the effective value of
`secrets.dir`; if the reference contains the placeholder, that value must itself be absolute and
clean. An invalid reference appears in no message, not even in part: a secret may stand in the
wrong key. The message names only a known scheme and the rule. Valid `file:` and `env:`
references are named in full, with control characters quoted. The messages of `Provider.Get` and
of the drivers do not name a `Ref` of another scheme built in code (`azkv`, `awssm`, a driver of
its own): this package does not check its grammar.

## File rules (`file:`)

1. `os.Stat` follows symlinks, so secrets mounted as links work. The target must be
   a regular file; a directory, device or FIFO is `insecure_file` before anything is opened (a
   FIFO would block the open).
2. Mode exactly `0400` or `0600`, checked through the permissions including setuid, setgid and
   sticky. Otherwise `insecure_file` with the observed mode, for example
   `mode 0644, want 0400 or 0600`.
3. The owner is the effective uid of the process (SEC-SEC-1), otherwise `insecure_file` with
   `owner uid <actual>, want uid <expected>`. Without Unix ownership (other platforms) the driver
   reads nothing.
4. After opening, the opened file is checked again and must be the same (`os.SameFile`); a swap
   between check and read is `read_failed`.
5. At most `maxFileSize` (`file.go`), otherwise `read_failed`.
6. Exactly one trailing `\n` and a `\r` directly before it are removed (`echo value > file`
   appends it, and no password ends with it). Empty after that is `not_found` like a missing
   file: an empty password is never valid.

The environment driver (`env:`) reads the snapshot `Options.Env`, which `cmd/bothy` reads once;
with duplicate names the first entry applies, as with `os.Getenv`. Unset or empty is
`not_found`. `env:` is allowed but not recommended (16 5.2).

## Cache and rotation

- One entry per reference with value, version and read time. `Get` returns the entry as long as
  `0 <= now - read time < secrets.cache_ttl`; after that it reads again. If the read time lies in
  the future (clock set back, for example by NTP), the entry counts as expired. `0` reads on every
  call and then logs every failed read; that is why the key `secrets.cache_ttl` is at least `1s`,
  and `0` exists only in tests.
- Concurrent misses on the same reference read once (a mutex per entry): whoever comes during a
  read waits for it and gets its result; the context is checked at the call. Other references do
  not wait.
- If the first read fails, the error goes to the caller and nothing is stored: the start aborts
  (02 "Failure of a dependency").
- If a later read fails, the old value stays in use. In addition there is a log line at level
  ERROR with the fixed message `secret refresh failed, keeping previous value` and only the
  attribute `error.type` (error code), and one step of the counter
  `bothy.secrets.refresh.failures` (unit `{refresh}`, without attributes, global OpenTelemetry
  meter, Prometheus `bothy_secrets_refresh_failures_total`). The attempt counts as a read, so the
  next attempt and the next line follow only one TTL later: one line per TTL and reference, not
  per call. The alert `BothySecretRefreshFailing` hangs on this counter.
- A cancelled context is no failed rotation: `Get` returns the context error without a log line,
  without the counter and without changing the entry.

## Error codes

`ParseRef`, the drivers `file` and `env` and `Provider.Get` return a
`*RefError{Ref, Err, Detail}` with the message `secrets: <reference>: <error>: <detail>` and never
with content. `Err` is a sentinel of the table or, for a cancelled call, the context error.
`Provider.Get` passes the error of a driver of its own (`Options.Drivers`) on unchanged;
`NewProvider` returns plain configuration errors. `ErrorCode` maps every error to one of the eight
codes, for logs, metrics and `bothy config check --secrets`; the three write codes come only from
`Put` and `Delete` (M1-32):

| Code | Sentinel | Meaning |
|---|---|---|
| `invalid_ref` | `ErrInvalidRef` | grammar violated |
| `unsupported_scheme` | `ErrUnsupportedScheme` | scheme without a driver in this version |
| `not_found` | `ErrNotFound` | file or variable missing or empty |
| `insecure_file` | `ErrInsecureFile` | not a regular file, wrong mode or owner |
| `read_failed` | `ErrReadFailed` | read error, too large, swapped; also every other error |
| `read_only` | `ErrReadOnly` | `Put` or `Delete` outside `secrets.signing_key_dir`, `env:` or a driver without `Writer` |
| `exists` | `ErrExists` | `Put` on an existing name; never overwritten |
| `write_failed` | `ErrWriteFailed` | write or delete error |

## Writing: the key store (M1-32)

The server writes exactly one place: the key store `secrets.signing_key_dir` of the signing keys
([ADR 0066](../../../../../docs/adr/0066-signing-keys-in-the-writable-key-store.md),
16 section 5.2). `Options.SigningKeyDir` names it; empty means no key store.

- `Provider.Put(ctx, ref, value)` and `Provider.Delete(ctx, ref)` accept only a `file:` reference
  whose file lies directly in that directory and whose driver implements `Writer`; every other
  reference, `env:` always, is `ErrReadOnly` before the driver is called. Both drop the cached
  value of the reference.
- The file driver's `Put` creates a temporary file (prefix `TempPrefix`, `.bothy-tmp-`) with mode
  `0600` in the same directory, writes and syncs it, links it to the final name (an existing name
  is `ErrExists`, never overwritten), removes the temporary name and syncs the directory. `Delete`
  removes the file and syncs the directory; a missing file is `ErrNotFound`.
- `CheckSigningKeyDir(dir)` is the start check of every role: an absolute clean path to a
  directory owned by the process user with mode `0700` in which a probe file can be created.
  The error names `secrets.signing_key_dir`, the expected mode and what was observed.

`Writer` is the driver interface for this; `azkv` and `awssm` implement it with V3-12.

## Version and fingerprint

For `file:` and `env:`, `Version` is the start (16 hex characters) of the SHA-256 of the value:
same content, same version. The fingerprint of `bothy secrets init` (subpackage `stack`) is a
shortened SHA-256 as well. Neither is a secret, but neither belongs in logs: for a weak value they
allow guesses to be checked.

## `Config` and binding

`Config{Dir, CacheTTL, SigningKeyDir}` carries the keys `secrets.dir`, `secrets.cache_ttl` and
`secrets.signing_key_dir` (defaults in the
[configuration reference](../../../../../docs/operations/configuration.md)). `internal/wire`
fills it with `config.Bind` and builds from it, with the environment from `cmd/bothy`, the one
provider of the process (D10); only `internal/wire` and `cmd/bothy` create providers.

## What is missing

- A re-read that does not hold up waiting callers (for example `singleflight` with a context):
  the drivers in M1 read local files and the environment; the network drivers bring it.
- The log line of a failed re-read does not name the reference (only `error.type` from the list
  in `docs/observability/signals.md` 5.3); `bothy config check --secrets` shows it.
- A key store shared by several nodes (stage 2 of 02 section 16) needs a shared volume or a
  writable vault driver; M7-17 decides it.
- The drivers `azkv` and `awssm` in a later milestone.

## Dependencies

The standard library, `internal/kernel/clock` and the API of OpenTelemetry
(`go.opentelemetry.io/otel`, `go.opentelemetry.io/otel/metric`; register: `platform.secrets` with
the vendor groups `age` and `otel`). Only the tests use the SDK
(`go.opentelemetry.io/otel/sdk/metric`).

## Chapters

[16-operations.md](../../../../../docs/spec/16-operations.md),
[19-security.md](../../../../../docs/spec/19-security.md),
[02-architecture.md](../../../../../docs/spec/02-architecture.md),
[ADR 0011](../../../../../docs/adr/0011-12-factor-configuration-and-secret-provider.md),
[configuration.md](../../../../../docs/operations/configuration.md),
[signals.md](../../../../../docs/observability/signals.md)
