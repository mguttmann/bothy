// SPDX-License-Identifier: Apache-2.0

// Package secrets is the secret provider of the server (docs/spec/16-operations.md 5.2, ADR 0011):
// every secret the server uses is read from a reference "<scheme>:<path>" through a driver
// (file and env in M1), cached for secrets.cache_ttl and held in a Value that never prints.
//
// References: ParseRef replaces the placeholder <secrets.dir> with the effective secrets.dir and
// accepts "file:<absolute clean path>" and "env:<NAME>"; azkv and awssm are later drivers and
// return ErrUnsupportedScheme. Errors never repeat an invalid reference, because a secret value
// may stand where a reference belongs.
//
// Drivers: NewFileDriver reads regular files with mode 0400 or 0600 owned by the effective uid
// (SEC-SEC-1), follows symlinks, strips one trailing newline and refuses files above 1 MiB.
// NewEnvDriver reads a snapshot of the environment that cmd/bothy passes in, because only
// cmd/bothy reads the process environment. Writer exists for drivers that store secrets (the
// signing keys, M1-32); no driver implements it yet.
//
// Provider caches a value per reference for the cache TTL. Concurrent misses read once; a clock
// that moved backwards expires the entry. When a refresh fails, the previous value stays in use,
// one error line is logged and the counter bothy.secrets.refresh.failures grows by one per TTL
// and reference (alarm BothySecretRefreshFailing); without a previous value the error goes to
// the caller, so a start aborts.
//
// Value renders as "[redacted]" through fmt, slog, encoding/json and encoding.TextMarshaler, and
// keeps its bytes behind a pointer so that reflection never reaches them (SEC-SEC-3, SEC-LOG-1).
// Errors of ParseRef, the file and env drivers and Provider.Get are *RefError values that name a
// valid file or env reference and never the content; Provider.Get passes the error of a
// caller-supplied driver on unchanged. ErrorCode maps any error to the codes invalid_ref,
// unsupported_scheme, not_found, insecure_file and read_failed.
//
// Only internal/wire and cmd/bothy create a Provider (one per process); modules receive values
// through their typed Config or a narrow interface and never read files or the environment
// themselves.
//
// Example:
//
//	p, err := secrets.NewProvider(secrets.Options{Dir: cfg.Dir, CacheTTL: cfg.CacheTTL, Env: env})
//	if err != nil {
//		return err
//	}
//	password, _, err := p.Resolve(ctx, "file:<secrets.dir>/db_app_password")
//	if err != nil {
//		return err // names the reference and a code, never the value
//	}
//	use(password.Bytes())
package secrets
