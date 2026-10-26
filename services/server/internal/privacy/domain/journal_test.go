// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/privacy/domain"
)

// TestEntryHasNoContent: the JSON of an entry has exactly the five members of brief M7-07
// requirement 12 and nothing else, for every kind (20 4.6: no content, names or e-mails).
func TestEntryHasNoContent(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 10, 26, 9, 30, 0, 123, time.FixedZone("x", 3600))
	for _, e := range []domain.Entry{
		{ResourceKind: domain.ResourceMessage, ResourceID: ids.New(), DeletedAt: at, Reason: domain.ReasonDeleted, Seq: 7},
		{ResourceKind: domain.ResourceFile, ResourceID: ids.New(), DeletedAt: at, Reason: domain.ReasonDeleted, Seq: 8},
		{ResourceKind: domain.ResourceAccount, ResourceID: ids.New(), DeletedAt: at, Reason: domain.ReasonDeleteContent, Seq: 9},
		{ResourceKind: domain.ResourceAccount, ResourceID: ids.New(), DeletedAt: at, Reason: domain.ReasonKeepPseudonymized, Seq: 10},
	} {
		b, err := e.Marshal()
		if err != nil {
			t.Fatalf("%s: %v", e.ResourceKind, err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		var keys []string
		for k := range m {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		if want := []string{"deleted_at", "reason", "resource_id", "resource_kind", "seq"}; !slices.Equal(keys, want) {
			t.Fatalf("%s: members %v, want %v", e.ResourceKind, keys, want)
		}
		if !strings.HasSuffix(m["deleted_at"].(string), "Z") {
			t.Errorf("%s: deleted_at %v is not UTC", e.ResourceKind, m["deleted_at"])
		}
		back, err := domain.ParseEntry(b)
		if err != nil {
			t.Fatalf("%s: ParseEntry: %v", e.ResourceKind, err)
		}
		if back.ResourceID != e.ResourceID || !back.DeletedAt.Equal(e.DeletedAt) || back.Seq != e.Seq || back.Reason != e.Reason {
			t.Errorf("%s: round trip %+v, want %+v", e.ResourceKind, back, e)
		}
		again, err := back.Marshal()
		if err != nil || string(again) != string(b) {
			t.Errorf("%s: the same entry gives other bytes: %s vs %s", e.ResourceKind, again, b)
		}
	}
}

// TestEntryValidate refuses entries outside the rules and objects that are no entry.
func TestEntryValidate(t *testing.T) {
	t.Parallel()
	ok := domain.Entry{ResourceKind: domain.ResourceMessage, ResourceID: ids.New(), DeletedAt: time.Now(), Reason: domain.ReasonDeleted, Seq: 1}
	bad := map[string]func(e *domain.Entry){
		"kind":            func(e *domain.Entry) { e.ResourceKind = "thread" },
		"id":              func(e *domain.Entry) { e.ResourceID = ids.ID{} },
		"time":            func(e *domain.Entry) { e.DeletedAt = time.Time{} },
		"seq":             func(e *domain.Entry) { e.Seq = 0 },
		"message reason":  func(e *domain.Entry) { e.Reason = "delete" },
		"account reason":  func(e *domain.Entry) { e.ResourceKind = domain.ResourceAccount },
		"content in text": func(e *domain.Entry) { e.Reason = "Hello Sam" },
	}
	for name, change := range bad {
		e := ok
		change(&e)
		if _, err := e.Marshal(); !errors.Is(err, domain.ErrInvalidEntry) {
			t.Errorf("%s: Marshal = %v, want ErrInvalidEntry", name, err)
		}
	}
	b, err := ok.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	for name, raw := range map[string]string{
		"unknown member": strings.Replace(string(b), `"seq"`, `"text":"x","seq"`, 1),
		"trailing":       string(b) + "{}",
		"array":          "[]",
		"empty":          "",
	} {
		if _, err := domain.ParseEntry([]byte(raw)); !errors.Is(err, domain.ErrInvalidEntry) {
			t.Errorf("%s: ParseEntry = %v, want ErrInvalidEntry", name, err)
		}
	}
}

// TestJournalKey: dense numbers from 1, zero-padded so the keys sort like the numbers, only the
// characters the object key grammar of files allows.
func TestJournalKey(t *testing.T) {
	t.Parallel()
	k, err := domain.JournalKey(7)
	if err != nil || k != "deletion-journal/0000000000000000007" {
		t.Fatalf("JournalKey(7) = %q, %v", k, err)
	}
	k9, _ := domain.JournalKey(9)
	k10, _ := domain.JournalKey(10)
	if k9 >= k10 {
		t.Errorf("%q does not sort before %q", k9, k10)
	}
	if _, err := domain.JournalKey(0); !errors.Is(err, domain.ErrInvalidEntry) {
		t.Errorf("JournalKey(0) = %v", err)
	}
	if _, err := domain.JournalKey(1<<63 - 1); err != nil {
		t.Errorf("JournalKey(max) = %v", err)
	}
}

// TestRetentionComparison (DP-T08): the journal retention must reach the longest of the three
// backup periods; each of them is the longest once. The pgBackRest period is the count of full
// backups times the interval of the full schedule, rounded up to whole days.
func TestRetentionComparison(t *testing.T) {
	t.Parallel()
	const d = 24 * time.Hour
	week := 7 * d
	cases := []struct {
		name    string
		p       domain.BackupPeriods
		longest time.Duration
	}{
		{"logical longest", domain.BackupPeriods{Logical: 100 * d, Objects: 90 * d, FullCount: 4, FullInterval: week}, 100 * d},
		{"objects longest", domain.BackupPeriods{Logical: 30 * d, Objects: 90 * d, FullCount: 4, FullInterval: week}, 90 * d},
		{"pgbackrest longest", domain.BackupPeriods{Logical: 30 * d, Objects: 90 * d, FullCount: 20, FullInterval: week}, 140 * d},
		{"pgbackrest rounded up", domain.BackupPeriods{Logical: d, Objects: d, FullCount: 3, FullInterval: 30 * time.Hour}, 4 * d},
	}
	for _, c := range cases {
		if got := c.p.Longest(); got != c.longest {
			t.Errorf("%s: Longest = %v, want %v", c.name, got, c.longest)
		}
		if !domain.JournalRetentionCovers(c.longest, c.p) {
			t.Errorf("%s: a journal retention equal to the longest period must pass", c.name)
		}
		if domain.JournalRetentionCovers(c.longest-time.Second, c.p) {
			t.Errorf("%s: a journal retention below the longest period must fail", c.name)
		}
	}
	if got := domain.FullBackupPeriod(0, week); got != 0 {
		t.Errorf("FullBackupPeriod(0) = %v", got)
	}
}
