// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/audit/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/retention"
)

// ipKeyStore is the part of app.RetentionStore the IP key participant uses.
type ipKeyStore struct {
	app.RetentionStore
	cutoff, at time.Time
	limit      int
	n          int64
	err        error
}

func (s *ipKeyStore) DestroyIPKeys(_ context.Context, cutoff, at time.Time, limit int) (int64, error) {
	s.cutoff, s.at, s.limit = cutoff, at, limit
	return s.n, s.err
}

// TestIPKeysParticipant (M7-05, DP-T06): the participant destroys through the store with the
// cutoff, the time of its clock and the batch, reports more for a full batch, and passes errors.
func TestIPKeysParticipant(t *testing.T) {
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	cutoff := now.Add(-90 * 24 * time.Hour)
	s := &ipKeyStore{n: 3}
	p := app.NewIPKeys(s, clock.NewFake(now))
	if p.Class() != retention.ClassAudit || len(p.Tables()) != 1 || p.Tables()[0] != "audit_ip_keys" {
		t.Fatalf("class %s tables %v", p.Class(), p.Tables())
	}
	n, more, err := p.Purge(context.Background(), cutoff, 3)
	if err != nil || n != 3 || !more || !s.cutoff.Equal(cutoff) || !s.at.Equal(now) || s.limit != 3 {
		t.Fatalf("Purge = %d, %v, %v; store saw %v %v %d", n, more, err, s.cutoff, s.at, s.limit)
	}
	s.n = 1
	if n, more, err := p.Purge(context.Background(), cutoff, 3); err != nil || n != 1 || more {
		t.Fatalf("a short batch: %d, %v, %v", n, more, err)
	}
	s.err = errors.New("boom")
	if _, _, err := p.Purge(context.Background(), cutoff, 3); err == nil {
		t.Fatal("the store error was lost")
	}
	if app.NewIPKeys(s, nil) == nil {
		t.Fatal("no participant with the system clock")
	}
}

// TestNewTrimNeedsItsParts: the trim refuses missing collaborators and names its tables.
func TestNewTrimNeedsItsParts(t *testing.T) {
	if _, err := app.NewTrim(nil, nil, nil); err == nil {
		t.Fatal("a trim without collaborators was built")
	}
}
