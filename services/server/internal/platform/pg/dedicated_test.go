// SPDX-License-Identifier: Apache-2.0

package pg_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// TestDialDedicated_OwnConnectionOutsideThePool: the log tail's LISTEN connection is a single
// connection with its own application_name and the session settings of the pool, and it takes
// no connection of the pool (02 "Log-Tail je api-Prozess").
func TestDialDedicated_OwnConnectionOutsideThePool(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	cfg := db.Config(pg.RoleApp)
	cfg.MaxConns = 1
	pool := db.OpenPool(t, cfg)
	conn, err := pool.DialDedicated(ctx, "bothy-event-tail")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(ctx) }()
	if got := show(t, conn, "application_name"); got != "bothy-event-tail" {
		t.Errorf("application_name = %q", got)
	}
	if got := show(t, conn, "TimeZone"); got != "UTC" {
		t.Errorf("TimeZone = %q, want the session settings of the pool", got)
	}
	if got := pool.Stat().TotalConns(); got != 0 {
		t.Errorf("the pool holds %d connections; the dedicated one must not count", got)
	}
	c, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("the only pool connection is taken: %v", err)
	}
	c.Release()
	if _, err := pool.DialDedicated(ctx, "bad\nname"); err == nil {
		t.Error("an application name with a control character was accepted")
	}
}
