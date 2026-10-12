// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
)

// meterName is the instrumentation scope of the metrics of this package: its import path, as the
// OpenTelemetry conventions ask.
const meterName = "github.com/OWNER/bothy/services/server/internal/platform/secrets"

// refreshFailedMessage is the constant log message of a failed refresh (D3). It never varies, so
// the line can be searched for and no value can slip into it.
const refreshFailedMessage = "secret refresh failed, keeping previous value"

// attrErrorType is the catalog key error.type (docs/observability/signals.md "Attribute in Spans
// und Logs"). It is a constant of this package and not otel/attr.ErrorType because platform/otel
// depends on this package and platform dependency cycles are not allowed (modules.yaml).
const attrErrorType = "error.type"

// Config holds the configuration keys of the provider (docs/operations/configuration.md).
// internal/wire fills it with config.Bind and passes the fields to Options.
type Config struct {
	// Dir is secrets.dir: the base directory of the default file: references; it replaces the
	// placeholder <secrets.dir>.
	Dir string `config:"secrets.dir"`
	// CacheTTL is secrets.cache_ttl: how long a value that was read stays in use before it is
	// read again.
	CacheTTL time.Duration `config:"secrets.cache_ttl"`
	// SigningKeyDir is secrets.signing_key_dir: the key store of the server signing keys, the
	// only directory Put and Delete write (ADR 0066).
	SigningKeyDir string `config:"secrets.signing_key_dir"`
}

// Options configure NewProvider.
type Options struct {
	// Dir is the effective secrets.dir. It must be absolute, clean and not the root directory.
	Dir string
	// SigningKeyDir is the effective secrets.signing_key_dir, the only directory whose direct
	// entries Put and Delete may write (ADR 0066). Empty means no key store: every Put and Delete
	// is ErrReadOnly. When set it must be absolute, clean and not the root directory.
	SigningKeyDir string
	// CacheTTL is how long a value that was read is returned without reading again. It must not
	// be negative. 0 reads on every Get, so a failing refresh logs and counts on every call; the
	// configuration key secrets.cache_ttl is therefore at least 1s, and 0 is for tests.
	CacheTTL time.Duration
	// Clock is the source of the read times; nil means clock.System().
	Clock clock.Clock
	// Logger receives the error line of a failed refresh; nil means slog.Default().
	Logger *slog.Logger
	// Drivers are the drivers by scheme; nil means the file driver and the env driver over Env.
	Drivers []Driver
	// Env is the environment snapshot ("NAME=value" entries) for the default env driver. Only
	// cmd/bothy reads the process environment and passes it here; it is ignored when Drivers is
	// set.
	Env []string
}

// Provider reads secrets through its drivers and caches each value for the cache TTL, so a
// rotated secret takes effect without a restart (docs/spec/16-betrieb.md 5.2, D3). Exactly one
// Provider exists per process, built by internal/wire or cmd/bothy; modules get values through
// their Config or a narrow interface (D10). A Provider is safe for concurrent use.
type Provider struct {
	dir      string
	keyDir   string
	ttl      time.Duration
	clock    clock.Clock
	logger   *slog.Logger
	drivers  map[string]Driver
	failures metric.Int64Counter

	mu      sync.Mutex
	entries map[Ref]*entry
}

// entry is the cache of one reference. Its own mutex serialises the reads of that reference, so
// concurrent misses read once, while other references stay independent.
type entry struct {
	mu      sync.Mutex
	cached  bool
	value   Value
	version Version
	readAt  time.Time
}

// NewProvider returns a Provider for opts.
//
// It fails when Dir is not an absolute clean path or is the root, CacheTTL is negative, a driver is nil or has
// an empty scheme, or two drivers share a scheme. The counter bothy.secrets.refresh.failures is
// created here from the global OpenTelemetry meter provider; until the telemetry setup installs
// one, the global delegate forwards later.
func NewProvider(opts Options) (*Provider, error) {
	if err := checkOptions(opts); err != nil {
		return nil, err
	}
	drivers := opts.Drivers
	if drivers == nil {
		drivers = []Driver{NewFileDriver(), NewEnvDriver(opts.Env)}
	}
	byScheme := make(map[string]Driver, len(drivers))
	for i, d := range drivers {
		if d == nil {
			return nil, fmt.Errorf("secrets: driver %d is nil", i)
		}
		scheme := d.Scheme()
		if scheme == "" {
			return nil, fmt.Errorf("secrets: driver %d has an empty scheme", i)
		}
		if _, dup := byScheme[scheme]; dup {
			return nil, fmt.Errorf("secrets: two drivers for scheme %q", scheme)
		}
		byScheme[scheme] = d
	}
	failures, err := otel.GetMeterProvider().Meter(meterName).Int64Counter("bothy.secrets.refresh.failures",
		metric.WithUnit("{refresh}"),
		metric.WithDescription("Failed re-reads of a cached secret; the previous value stays in use."))
	if err != nil {
		return nil, fmt.Errorf("secrets: create counter bothy.secrets.refresh.failures: %w", err)
	}
	clk := opts.Clock
	if clk == nil {
		clk = clock.System()
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Provider{
		dir: opts.Dir, keyDir: opts.SigningKeyDir, ttl: opts.CacheTTL, clock: clk, logger: logger, drivers: byScheme,
		failures: failures, entries: map[Ref]*entry{},
	}, nil
}

// checkOptions checks the directories and the cache TTL of opts (NewProvider).
func checkOptions(opts Options) error {
	if !isAbsClean(opts.Dir) || opts.Dir == "/" {
		// The root would turn file:<secrets.dir>/x into file://x (docs/DECISIONS.md M1-09/15).
		return errors.New("secrets: secrets.dir must be an absolute path other than the root, without . or .. elements, repeated or trailing slashes")
	}
	if opts.CacheTTL < 0 {
		return errors.New("secrets: secrets.cache_ttl must not be negative")
	}
	if opts.SigningKeyDir != "" && (!isAbsClean(opts.SigningKeyDir) || opts.SigningKeyDir == "/") {
		return errors.New("secrets: secrets.signing_key_dir must be an absolute path other than the root, without . or .. elements, repeated or trailing slashes")
	}
	return nil
}

// Get returns the value of ref and its Version.
//
// A value read less than the cache TTL ago is returned from the cache; a value whose read time
// lies in the future (the clock moved backwards) counts as expired, so it cannot stay in use until
// the clock catches up. Concurrent misses for one reference cause one read: a caller that comes
// while another caller reads the same reference waits for that read and then sees its result.
// The context is checked when the call starts; the M1 drivers read local files and the
// environment, so the wait is short. Drivers that call a network (azkv, awssm, later) need a
// refresh that does not hold waiters, and bring it with them.
//
// When a read fails and no value is cached, Get returns the error, so a start aborts (fail fast).
// When a value is cached, Get returns it, logs one error line with the constant message "secret
// refresh failed, keeping previous value" and the attribute error.type (ErrorCode only), adds one
// to the counter bothy.secrets.refresh.failures, and counts the attempt as a read, so the next
// attempt and the next line follow one cache TTL later instead of one per call. A canceled
// context is the caller giving up, not a failed rotation: Get returns the context error without
// logging or counting.
//
// Errors: the context error first; ErrInvalidRef for a hand-built reference that breaks the
// grammar; ErrUnsupportedScheme when no driver has the scheme; otherwise the error of the driver.
// The errors Get builds itself name the reference only when it is a valid file or env reference.
func (p *Provider) Get(ctx context.Context, ref Ref) (Value, Version, error) {
	if err := ctx.Err(); err != nil {
		return Value{}, "", &RefError{Ref: displayRef(ref), Err: err}
	}
	if err := checkRef(ref); err != nil {
		return Value{}, "", err
	}
	d, ok := p.drivers[ref.Scheme]
	if !ok {
		return Value{}, "", &RefError{Ref: displayRef(ref), Err: ErrUnsupportedScheme, Detail: "no driver for this scheme"}
	}

	e := p.entry(ref)
	e.mu.Lock()
	defer e.mu.Unlock()
	now := p.clock.Now()
	if e.cached {
		if age := now.Sub(e.readAt); age >= 0 && age < p.ttl {
			return e.value, e.version, nil
		}
	}
	v, ver, err := d.Get(ctx, ref)
	if err == nil {
		e.cached, e.value, e.version, e.readAt = true, v, ver, now
		return v, ver, nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return Value{}, "", &RefError{Ref: displayRef(ref), Err: ctxErr}
	}
	if !e.cached {
		return Value{}, "", err
	}
	e.readAt = now
	p.failures.Add(ctx, 1)
	p.logger.ErrorContext(ctx, refreshFailedMessage, slog.String(attrErrorType, ErrorCode(err)))
	return e.value, e.version, nil
}

// Resolve parses raw with the provider's secrets.dir (ParseRef) and returns Get of the result.
func (p *Provider) Resolve(ctx context.Context, raw string) (Value, Version, error) {
	ref, err := ParseRef(raw, p.dir)
	if err != nil {
		return Value{}, "", err
	}
	return p.Get(ctx, ref)
}

// Put stores v under ref through the driver of ref.Scheme (16 5.2, ADR 0066 rule 2). Only a
// "file:" reference whose path lies directly in secrets.signing_key_dir is writable; every other
// reference, "env:" always, and every reference of a provider without a key store is ErrReadOnly,
// checked before the driver is called. The file driver creates the file with mode 0600 and never
// overwrites (ErrExists). A cached value of ref is dropped, so the next Get reads the new file.
//
// Errors: the context error first; ErrReadOnly; ErrInvalidRef for a reference that breaks the
// grammar; ErrExists; ErrWriteFailed. They name the reference, never the value.
//
// Example:
//
//	ref := secrets.Ref{Scheme: "file", Path: "/var/lib/bothy/signing-keys/invocation-<kid>.pem"}
//	err := p.Put(ctx, ref, secrets.NewValue(pemBytes))
func (p *Provider) Put(ctx context.Context, ref Ref, v Value) error {
	w, err := p.writer(ctx, ref)
	if err != nil {
		return err
	}
	_, err = w.Put(ctx, ref, v)
	p.forget(ref)
	return err
}

// Delete removes the secret ref names, under the same rule as Put (only directly in
// secrets.signing_key_dir, else ErrReadOnly), and drops its cached value. A missing secret is
// ErrNotFound; other errors as Put.
func (p *Provider) Delete(ctx context.Context, ref Ref) error {
	w, err := p.writer(ctx, ref)
	if err != nil {
		return err
	}
	err = w.Delete(ctx, ref)
	p.forget(ref)
	return err
}

// writer returns the writing driver of ref when the write rule allows ref (Put).
func (p *Provider) writer(ctx context.Context, ref Ref) (Writer, error) {
	if err := ctx.Err(); err != nil {
		return nil, &RefError{Ref: displayRef(ref), Err: err}
	}
	if ref.Scheme != schemeFile || checkRef(ref) != nil || p.keyDir == "" || path.Dir(ref.Path) != p.keyDir {
		return nil, &RefError{Ref: displayRef(ref), Err: ErrReadOnly, Detail: "only files directly in secrets.signing_key_dir are writable"}
	}
	w, ok := p.drivers[ref.Scheme].(Writer)
	if !ok {
		return nil, &RefError{Ref: displayRef(ref), Err: ErrReadOnly, Detail: "the driver of this scheme cannot write"}
	}
	return w, nil
}

// forget drops the cache entry of ref.
func (p *Provider) forget(ref Ref) {
	p.mu.Lock()
	delete(p.entries, ref)
	p.mu.Unlock()
}

// entry returns the cache entry of ref, creating it on first use.
func (p *Provider) entry(ref Ref) *entry {
	p.mu.Lock()
	defer p.mu.Unlock()
	e, ok := p.entries[ref]
	if !ok {
		e = &entry{}
		p.entries[ref] = e
	}
	return e
}
