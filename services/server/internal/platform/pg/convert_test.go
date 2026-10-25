// SPDX-License-Identifier: Apache-2.0

package pg_test

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

func TestUUIDConversions(t *testing.T) {
	id := ids.New()
	if u := pg.UUID(id); !u.Valid || ids.ID(u.Bytes) != id {
		t.Errorf("UUID(%v) = %+v", id, u)
	}
	if u := pg.UUID(ids.ID{}); !u.Valid {
		t.Errorf("UUID(zero) is NULL, want a value")
	}
	if u := pg.NullableUUID(ids.ID{}); u.Valid {
		t.Errorf("NullableUUID(zero) = %+v, want NULL", u)
	}
	if u := pg.NullableUUID(id); !u.Valid || ids.ID(u.Bytes) != id {
		t.Errorf("NullableUUID(%v) = %+v", id, u)
	}
	if got := pg.IDOf(pgtype.UUID{}); !got.IsZero() {
		t.Errorf("IDOf(NULL) = %v, want the zero ID", got)
	}
	if got := pg.IDOf(pg.UUID(id)); got != id {
		t.Errorf("IDOf = %v, want %v", got, id)
	}
}

func TestTimeOf(t *testing.T) {
	if got := pg.TimeOf(pgtype.Timestamptz{}); !got.IsZero() {
		t.Errorf("TimeOf(NULL) = %v, want the zero time", got)
	}
	at := time.Date(2026, 10, 20, 11, 0, 0, 0, time.FixedZone("UTC+2", 2*60*60))
	got := pg.TimeOf(pgtype.Timestamptz{Time: at, Valid: true})
	if !got.Equal(at) || got.Location() != time.UTC {
		t.Errorf("TimeOf = %v, want %v in UTC", got, at)
	}
}

func TestArrayAndTextConversions(t *testing.T) {
	a, b := ids.New(), ids.New()
	us := pg.UUIDs([]ids.ID{a, b})
	if len(us) != 2 || !us[0].Valid || ids.ID(us[0].Bytes) != a || ids.ID(us[1].Bytes) != b {
		t.Errorf("UUIDs = %+v", us)
	}
	if us := pg.UUIDs(nil); us == nil || len(us) != 0 {
		t.Errorf("UUIDs(nil) = %#v, want an empty array (never NULL)", us)
	}
	at := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	if ts := pg.Timestamptz(at); !ts.Valid || !ts.Time.Equal(at) {
		t.Errorf("Timestamptz = %+v", ts)
	}
	tss := pg.Timestamptzs([]time.Time{at, at.Add(time.Hour)})
	if len(tss) != 2 || !tss[1].Valid || !tss[1].Time.Equal(at.Add(time.Hour)) {
		t.Errorf("Timestamptzs = %+v", tss)
	}
	if got := pg.UUIDText(pgtype.UUID{}); got != "" {
		t.Errorf("UUIDText(NULL) = %q, want empty", got)
	}
	if got := pg.UUIDText(pg.UUID(a)); got != a.String() {
		t.Errorf("UUIDText = %q, want %q", got, a.String())
	}
}
