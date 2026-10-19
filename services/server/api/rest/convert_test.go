// SPDX-License-Identifier: Apache-2.0

package rest_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

func TestOptionalID(t *testing.T) {
	if got := rest.OptionalID(ids.ID{}); got != nil {
		t.Errorf("OptionalID(zero) = %v, want nil", got)
	}
	id := ids.New()
	if got := rest.OptionalID(id); got == nil || *got != rest.Uuid(id) {
		t.Errorf("OptionalID(%v) = %v", id, got)
	}
}

func TestOptionalTime(t *testing.T) {
	if got := rest.OptionalTime(time.Time{}); got != nil {
		t.Errorf("OptionalTime(zero) = %v, want nil", got)
	}
	at := time.Date(2026, 10, 20, 11, 0, 0, 123456789, time.FixedZone("UTC+2", 2*60*60))
	got := rest.OptionalTime(at)
	want := time.Date(2026, 10, 20, 9, 0, 0, 123000000, time.UTC)
	if got == nil || !got.Equal(want) || got.Location() != time.UTC {
		t.Errorf("OptionalTime = %v, want %v", got, want)
	}
}

func TestIDs(t *testing.T) {
	if got := rest.IDs(nil); got == nil || len(got) != 0 {
		t.Errorf("IDs(nil) = %#v, want an empty list", got)
	}
	a, b := ids.New(), ids.New()
	got := rest.IDs([]ids.ID{a, b})
	if len(got) != 2 || got[0] != rest.Uuid(a) || got[1] != rest.Uuid(b) {
		t.Errorf("IDs = %v", got)
	}
}
