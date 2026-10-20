// SPDX-License-Identifier: Apache-2.0

package pg_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// TestCheckTextSearch holds the start check of the text search (docs/DECISIONS.md D-M3-13-6): the
// chapter defaults pass on the pgtest database; an unknown configuration fails with its position
// and not its name; a database with the character classification C fails the case-folding probe.
func TestCheckTextSearch(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	app := db.Pool(t, pg.RoleApp)
	ctx := context.Background()
	if err := pg.CheckTextSearch(ctx, app, []string{"english", "german", "simple"}); err != nil {
		t.Fatalf("defaults: %v", err)
	}
	err := pg.CheckTextSearch(ctx, app, []string{"english", "klingonisch", "simple"})
	var missing *pg.TextSearchConfigMissingError
	if !errors.As(err, &missing) || missing.Position != 2 {
		t.Fatalf("unknown configuration: %v, want position 2", err)
	}
	if msg := err.Error(); strings.Contains(msg, "klingonisch") || !strings.Contains(msg, "2") {
		t.Errorf("message %q names the configuration or not its position", msg)
	}

	super := db.SuperConn(t)
	const name = "bothy_ctype_c"
	if _, err := super.Exec(ctx, "CREATE DATABASE "+name+" TEMPLATE template0 LC_COLLATE 'C' LC_CTYPE 'C'"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := super.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err != nil {
			t.Errorf("drop %s: %v", name, err)
		}
	})
	cfg := db.SuperConfig()
	cfg.Name = name
	cPool := db.OpenPool(t, cfg)
	err = pg.CheckTextSearch(ctx, cPool, []string{"english", "german", "simple"})
	if !errors.Is(err, pg.ErrTextSearchCaseFolding) {
		t.Fatalf("database with LC_CTYPE C: %v, want ErrTextSearchCaseFolding", err)
	}
	cPool.Close()
}
