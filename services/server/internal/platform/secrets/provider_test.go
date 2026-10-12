// SPDX-License-Identifier: Apache-2.0

package secrets_test

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
)

const ttl = 5 * time.Minute

// start is the instant the fake clocks of these tests begin at.
var start = time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC)

// newProvider builds a provider over dir with the default drivers, the fake clock and logger.
func newProvider(t *testing.T, dir string, clk clock.Clock, logger *slog.Logger) *secrets.Provider {
	t.Helper()
	p, err := secrets.NewProvider(secrets.Options{Dir: dir, CacheTTL: ttl, Clock: clk, Logger: logger})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	return p
}

// discardLogger returns a logger for tests that do not look at log output.
func discardLogger() *slog.Logger { return slog.New(slog.DiscardHandler) }

func TestNewProviderRejects(t *testing.T) {
	t.Parallel()
	fake := func(scheme string) secrets.Driver { return &fakeDriver{scheme: scheme} }
	for name, opts := range map[string]secrets.Options{
		"empty dir":          {Dir: ""},
		"relative dir":       {Dir: "run/bothy/secrets"},
		"trailing slash":     {Dir: "/run/bothy/secrets/"},
		"repeated slash":     {Dir: "/run//bothy/secrets"},
		"dot dot element":    {Dir: "/run/bothy/../secrets"},
		"negative ttl":       {Dir: "/run/bothy/secrets", CacheTTL: -time.Second},
		"empty scheme":       {Dir: "/run/bothy/secrets", Drivers: []secrets.Driver{fake("")}},
		"duplicate scheme":   {Dir: "/run/bothy/secrets", Drivers: []secrets.Driver{fake("file"), secrets.NewFileDriver()}},
		"nil driver":         {Dir: "/run/bothy/secrets", Drivers: []secrets.Driver{nil}},
		"relative dot":       {Dir: "."},
		"root dir":           {Dir: "/"},
		"duplicate env only": {Dir: "/run/bothy/secrets", Drivers: []secrets.Driver{secrets.NewEnvDriver(nil), secrets.NewEnvDriver(nil)}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			p, err := secrets.NewProvider(opts)
			if err == nil || p != nil {
				t.Fatalf("NewProvider = %v, %v; want nil and an error", p, err)
			}
			if !strings.HasPrefix(err.Error(), "secrets: ") {
				t.Fatalf("error %q lacks the package prefix", err)
			}
		})
	}

	p, err := secrets.NewProvider(secrets.Options{Dir: "/run/bothy/secrets", CacheTTL: 0})
	if err != nil || p == nil {
		t.Fatalf("NewProvider with valid options = %v, %v; want a provider", p, err)
	}
}

// TestProviderConcurrentMissReadsOnce: callers that miss the cache for the same reference at the
// same time cause one read, not one each.
func TestProviderConcurrentMissReadsOnce(t *testing.T) {
	t.Parallel()
	const callers = 16
	release := make(chan struct{})
	unblock := sync.OnceFunc(func() { close(release) })
	t.Cleanup(unblock) // a failed test must not leave callers blocked in the driver
	entered := make(chan struct{}, callers)
	fake := &fakeDriver{scheme: "file", get: func(context.Context, int32, secrets.Ref) (secrets.Value, secrets.Version, error) {
		entered <- struct{}{}
		<-release
		return secrets.NewValue([]byte("EXAMPLE")), "v", nil
	}}
	p, err := secrets.NewProvider(secrets.Options{
		Dir: "/run/bothy/secrets", CacheTTL: ttl, Clock: clock.NewFake(start), Logger: discardLogger(),
		Drivers: []secrets.Driver{fake},
	})
	if err != nil {
		t.Fatal(err)
	}
	ref := secrets.Ref{Scheme: "file", Path: "/run/bothy/secrets/db_app_password"}

	var ready, done sync.WaitGroup
	ready.Add(callers)
	errs := make(chan error, callers)
	for range callers {
		done.Go(func() {
			ready.Done()
			v, _, err := p.Get(context.Background(), ref)
			if err == nil && string(v.Bytes()) != "EXAMPLE" {
				err = errors.New("wrong value")
			}
			errs <- err
		})
	}
	ready.Wait()
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("no caller reached the driver")
	}
	// Give the other callers time to arrive at the reference while the first read is blocked.
	// A slow machine only makes the test weaker, never wrong: late callers hit the cache.
	time.Sleep(50 * time.Millisecond)
	unblock()
	done.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
	}
	if n := fake.calls.Load(); n != 1 {
		t.Fatalf("driver called %d times for %d concurrent misses, want 1", n, callers)
	}
}

func TestProviderUnsupportedScheme(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p := newProvider(t, "/run/bothy/secrets", clock.NewFake(start), discardLogger())
	_, _, err := p.Resolve(ctx, "azkv://vault/secret")
	if !errors.Is(err, secrets.ErrUnsupportedScheme) || secrets.ErrorCode(err) != "unsupported_scheme" {
		t.Fatalf("Resolve azkv: error = %v, want ErrUnsupportedScheme", err)
	}

	fileOnly, err := secrets.NewProvider(secrets.Options{
		Dir: "/run/bothy/secrets", CacheTTL: ttl, Drivers: []secrets.Driver{secrets.NewFileDriver()},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = fileOnly.Get(ctx, secrets.Ref{Scheme: "env", Path: "BOTHY_SECRET_X"})
	if !errors.Is(err, secrets.ErrUnsupportedScheme) {
		t.Fatalf("Get env without env driver: error = %v, want ErrUnsupportedScheme", err)
	}

	for _, ref := range []secrets.Ref{{}, {Scheme: "file", Path: "relative"}, {Scheme: "env", Path: "lower"}} {
		if _, _, err := p.Get(ctx, ref); !errors.Is(err, secrets.ErrInvalidRef) {
			t.Errorf("Get(%#v) error = %v, want ErrInvalidRef", ref, err)
		}
	}

	// A hand-built reference of a scheme other than file or env has no grammar the provider can
	// check, so its path may be a secret pasted into the wrong place: no message repeats it, with
	// a live context as with a canceled one (docs/DECISIONS.md M1-09/6, like ParseRef).
	canary := newCanary(t)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	for _, ref := range []secrets.Ref{{Scheme: "azkv", Path: canary}, {Scheme: "foo", Path: canary}} {
		_, _, err := p.Get(ctx, ref)
		if !errors.Is(err, secrets.ErrUnsupportedScheme) {
			t.Errorf("Get(%s) error = %v, want ErrUnsupportedScheme", ref.Scheme, err)
		}
		if err != nil {
			assertNoSecret(t, "Get "+ref.Scheme, err.Error(), canary)
		}
		_, _, err = p.Get(canceled, ref)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Get(%s) with a canceled context: error = %v, want context.Canceled", ref.Scheme, err)
		}
		if err != nil {
			assertNoSecret(t, "Get "+ref.Scheme+" with a canceled context", err.Error(), canary)
		}
	}
}

func TestProviderHonoursCanceledContext(t *testing.T) {
	t.Parallel()
	ref := secrets.Ref{Scheme: "env", Path: "BOTHY_SECRET_X"}

	t.Run("before the read", func(t *testing.T) {
		t.Parallel()
		fake := &fakeDriver{scheme: "env", get: func(context.Context, int32, secrets.Ref) (secrets.Value, secrets.Version, error) {
			return secrets.NewValue([]byte("EXAMPLE")), "v", nil
		}}
		p, err := secrets.NewProvider(secrets.Options{Dir: "/run/bothy/secrets", Drivers: []secrets.Driver{fake}, Logger: discardLogger()})
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, _, err := p.Get(ctx, ref); !errors.Is(err, context.Canceled) {
			t.Fatalf("Get error = %v, want context.Canceled", err)
		}
		if n := fake.calls.Load(); n != 0 {
			t.Fatalf("driver called %d times with a canceled context, want 0", n)
		}
		// The context error comes first, and even then an invalid reference is not repeated.
		_, _, err = p.Get(ctx, secrets.Ref{Scheme: "file", Path: "EXAMPLE-relative"})
		if !errors.Is(err, context.Canceled) || strings.Contains(err.Error(), "EXAMPLE-relative") {
			t.Fatalf("Get of an invalid reference with a canceled context: error = %v, want context.Canceled without the reference", err)
		}
		_, _, err = p.Get(ctx, ref)
		if !strings.Contains(err.Error(), ref.String()) {
			t.Fatalf("message %q does not name the valid reference", err)
		}
	})

	// A caller that gives up during a refresh is no failed rotation: no log line, no counter,
	// and the next caller reads again at once.
	t.Run("during a refresh", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		fake := &fakeDriver{scheme: "env", get: func(ctx context.Context, call int32, _ secrets.Ref) (secrets.Value, secrets.Version, error) {
			if call == 2 {
				cancel()
				return secrets.Value{}, "", ctx.Err()
			}
			return secrets.NewValue([]byte("EXAMPLE")), "v", nil
		}}
		clk := clock.NewFake(start)
		counter := &countingHandler{}
		p, err := secrets.NewProvider(secrets.Options{
			Dir: "/run/bothy/secrets", CacheTTL: ttl, Clock: clk, Logger: slog.New(counter), Drivers: []secrets.Driver{fake},
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := p.Get(ctx, ref); err != nil {
			t.Fatal(err)
		}
		clk.Advance(ttl)
		if _, _, err := p.Get(ctx, ref); !errors.Is(err, context.Canceled) {
			t.Fatalf("Get error = %v, want context.Canceled", err)
		}
		if n := counter.n.Load(); n != 0 {
			t.Fatalf("a canceled refresh logged %d lines, want 0", n)
		}
		if _, _, err := p.Get(context.Background(), ref); err != nil {
			t.Fatal(err)
		}
		if n := fake.calls.Load(); n != 3 {
			t.Fatalf("driver calls = %d, want 3: the canceled refresh must not count as a read", n)
		}
	})

	// The provider cannot check the grammar of a caller-supplied driver's scheme, so the context
	// error of such a reference never repeats its path, before the read as during it.
	t.Run("custom scheme", func(t *testing.T) {
		t.Parallel()
		canary := newCanary(t)
		ref := secrets.Ref{Scheme: "foo", Path: canary}
		ctx, cancel := context.WithCancel(context.Background())
		fake := &fakeDriver{scheme: "foo", get: func(ctx context.Context, _ int32, _ secrets.Ref) (secrets.Value, secrets.Version, error) {
			cancel()
			return secrets.Value{}, "", ctx.Err()
		}}
		p, err := secrets.NewProvider(secrets.Options{Dir: "/run/bothy/secrets", Drivers: []secrets.Driver{fake}, Logger: discardLogger()})
		if err != nil {
			t.Fatal(err)
		}
		for _, when := range []string{"during the read", "before the read"} {
			_, _, err := p.Get(ctx, ref)
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("Get %s: error = %v, want context.Canceled", when, err)
			}
			assertNoSecret(t, "Get "+when, err.Error(), canary)
		}
		if n := fake.calls.Load(); n != 1 {
			t.Fatalf("driver calls = %d, want 1: the second Get starts with a canceled context", n)
		}
	})
}
