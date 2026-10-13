// SPDX-License-Identifier: Apache-2.0

package pgtest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// leaseConnections counts the lease connections of the cluster that conn belongs to.
func leaseConnections(t *testing.T, conn *pgx.Conn) int {
	t.Helper()
	var n int
	q := "SELECT count(*) FROM pg_stat_activity WHERE application_name = $1"
	if err := conn.QueryRow(t.Context(), q, leaseApp).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// waitLeaseConnections polls until the cluster that conn belongs to has want lease connections.
func waitLeaseConnections(t *testing.T, conn *pgx.Conn, want int) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		n := leaseConnections(t, conn)
		if n == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d lease connections, want %d", n, want)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestHoldLeaseKeepsItsConnectionUntilStop(t *testing.T) {
	if *childMode != "" {
		return
	}
	db := New(t, WithoutMigrations())
	conn := db.SuperConn(t)
	// The lease of the test binary itself, taken when the cluster started.
	waitLeaseConnections(t, conn, 1)
	stop, err := db.c.holdLease(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(stop) // stop may run twice
	waitLeaseConnections(t, conn, 2)
	// The lease outlives a few checks of its goroutine.
	time.Sleep(3 * leaseInterval)
	if n := leaseConnections(t, conn); n != 2 {
		t.Fatalf("%d lease connections while the lease is held, want 2", n)
	}
	stop()
	waitLeaseConnections(t, conn, 1)
}

func TestKeepLease(t *testing.T) {
	if *childMode != "" {
		return
	}
	db := New(t, WithoutMigrations())
	ctx := t.Context()
	connect := func(t *testing.T) *pgx.Conn {
		t.Helper()
		conn, err := db.c.superConn(ctx, superDatabase)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = conn.Close(context.WithoutCancel(ctx)) })
		return conn
	}
	refuse := func(context.Context) (*pgx.Conn, error) { return nil, errors.New("no connection") }

	t.Run("a live connection stays", func(t *testing.T) {
		conn := connect(t)
		opened := false
		got := keepLease(ctx, conn, func(context.Context) (*pgx.Conn, error) { opened = true; return nil, nil })
		if got != conn || opened {
			t.Fatalf("keepLease replaced a live connection (opened %v)", opened)
		}
	})
	t.Run("a broken connection is replaced", func(t *testing.T) {
		broken := connect(t)
		if err := broken.Close(ctx); err != nil {
			t.Fatal(err)
		}
		next := connect(t)
		got := keepLease(ctx, broken, func(context.Context) (*pgx.Conn, error) { return next, nil })
		if got != next {
			t.Fatal("keepLease kept a broken connection")
		}
	})
	t.Run("no connection and no new one", func(t *testing.T) {
		if got := keepLease(ctx, nil, refuse); got != nil {
			t.Fatal("keepLease returned a connection although open failed")
		}
	})
	t.Run("no connection gets a new one", func(t *testing.T) {
		next := connect(t)
		got := keepLease(ctx, nil, func(context.Context) (*pgx.Conn, error) { return next, nil })
		if got != next {
			t.Fatal("keepLease did not open a connection")
		}
	})
}

func TestHoldLeaseFailsWithoutAConnection(t *testing.T) {
	if *childMode != "" {
		return
	}
	db := New(t, WithoutMigrations())
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if stop, err := db.c.holdLease(ctx); err == nil {
		stop()
		t.Fatal("holdLease succeeded with a canceled context")
	}
}

func TestReapLegacyReportsAFailedListing(t *testing.T) {
	if *childMode != "" {
		return
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := reapLegacy(ctx, label+".reap-"+randomHex(6), time.Now()); err == nil {
		t.Fatal("reapLegacy reported no error with a canceled context")
	}
}
