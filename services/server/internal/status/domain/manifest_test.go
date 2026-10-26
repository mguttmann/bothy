// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/status/domain"
)

// validManifest returns a manifest of kind that Validate accepts. The ids are EXAMPLE values.
func validManifest(kind domain.Kind) domain.Manifest {
	m := domain.Manifest{
		Schema:        domain.ManifestSchema,
		BackupID:      "0192f4a0-7b1c-7000-8a00-000000000001",
		Kind:          kind,
		StartedAt:     time.Date(2026, 10, 18, 2, 0, 0, 0, time.UTC),
		FinishedAt:    time.Date(2026, 10, 18, 2, 7, 30, 0, time.UTC),
		ServerVersion: "1.4.0-rc.1+build.7",
		SchemaVersion: 49,
		InstanceID:    "0192f4a0-7b1c-7000-8a00-000000000002",
		SyncEpoch:     "0192f4a0-7b1c-7000-8a00-000000000003",
		EventLog:      domain.EventLogBounds{HeadSeq: 120, MinRetainedSeq: 1},
		Audit:         domain.AuditHead{LastSeq: 42, LastHash: strings.Repeat("ab", 32)},
		RowCounts:     map[string]int64{"messages": 100, "audit_events": 42, "event_log": 120},
		Files:         domain.FileTotals{Count: 3, TotalBytes: 4096},
		// M7-07: the position of the deletion journal.
		DeletionJournal: &domain.DeletionJournalRef{NextNumber: 8},
	}
	switch kind {
	case domain.KindPGFull:
		m.PgBackRest = &domain.PgBackRestRef{Label: "20261018-020000F"}
	case domain.KindPGDiff:
		m.PgBackRest = &domain.PgBackRestRef{Label: "20261018-020000F_20261019-020000D"}
	case domain.KindLogical, domain.KindObjects:
		m.Restic = &domain.ResticRef{SnapshotID: strings.Repeat("0f", 32)}
	case domain.KindRestoreTest:
		// No backup kind: the restore test writes no manifest.
	}
	return m
}

// manifestKeys are the JSON keys of backup_manifest.v1: the fields of
// docs/spec/03-data-model.md 4.12 and docs/operations/backup-restore.md 3, nested keys with a
// dot.
var manifestKeys = []string{
	"schema", "backup_id", "kind", "started_at", "finished_at", "server_version", "schema_version",
	"instance_id", "sync_epoch", "event_log", "event_log.head_seq", "event_log.min_retained_seq",
	"audit", "audit.last_seq", "audit.last_hash", "row_counts", "files", "files.count",
	"files.total_bytes", "pgbackrest", "pgbackrest.label", "restic", "restic.snapshot_id",
	"deletion_journal", "deletion_journal.next_number",
}

// jsonKeys returns the keys of the object b, nested objects with a dot, except below row_counts
// (its keys are table names).
func jsonKeys(t *testing.T, b []byte) []string {
	t.Helper()
	var top map[string]json.RawMessage
	if err := json.Unmarshal(b, &top); err != nil {
		t.Fatal(err)
	}
	var keys []string
	for k, v := range top {
		keys = append(keys, k)
		if k == "row_counts" {
			continue
		}
		var nested map[string]json.RawMessage
		if json.Unmarshal(v, &nested) == nil {
			for n := range nested {
				keys = append(keys, k+"."+n)
			}
		}
	}
	slices.Sort(keys)
	return keys
}

// TestManifestSchema holds backup_manifest.v1 (03 4.12, backup-restore.md 3, brief M7-02 rule 13):
// a marshalled manifest has exactly the listed keys, ParseManifest returns what Marshal wrote,
// and a field with message text, an extra key or a reference of the wrong kind fails without
// echoing the value.
func TestManifestSchema(t *testing.T) {
	t.Parallel()

	// pg_full carries pgbackrest, logical carries restic: together they show every key once.
	seen := map[string]bool{}
	for _, kind := range domain.Kinds() {
		m := validManifest(kind)
		b, err := m.Marshal()
		if err != nil {
			t.Fatalf("%s: Marshal: %v", kind, err)
		}
		for _, k := range jsonKeys(t, b) {
			if !slices.Contains(manifestKeys, k) {
				t.Errorf("%s: key %q is not a field of backup_manifest.v1", kind, k)
			}
			seen[k] = true
		}
		back, err := domain.ParseManifest(b)
		if err != nil {
			t.Fatalf("%s: ParseManifest: %v", kind, err)
		}
		if !back.StartedAt.Equal(m.StartedAt) || back.BackupID != m.BackupID || !maps.Equal(back.RowCounts, m.RowCounts) ||
			(m.PgBackRest != nil) != (back.PgBackRest != nil) || (m.Restic != nil) != (back.Restic != nil) {
			t.Errorf("%s: round trip changed the manifest:\n got %+v\nwant %+v", kind, back, m)
		}
	}
	got := slices.Sorted(maps.Keys(seen))
	want := slices.Clone(manifestKeys)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("marshalled keys over all kinds:\n got %q\nwant %q", got, want)
	}

	// The objects backup of the driver s3 has no restic reference.
	s3 := validManifest(domain.KindObjects)
	s3.Restic = nil
	if err := s3.Validate(); err != nil {
		t.Errorf("objects without restic: %v", err)
	}
	// An empty audit log has seq 0 and no hash; an empty event log has 0 and 0.
	empty := validManifest(domain.KindLogical)
	empty.Audit = domain.AuditHead{}
	empty.EventLog = domain.EventLogBounds{}
	empty.RowCounts = map[string]int64{}
	if err := empty.Validate(); err != nil {
		t.Errorf("empty logs: %v", err)
	}

	leak := "disk full on /data"
	bad := []struct {
		name, field string
		mut         func(m *domain.Manifest)
	}{
		{"schema", "schema", func(m *domain.Manifest) { m.Schema = "backup_manifest.v2" }},
		{"backup_id text", "backup_id", func(m *domain.Manifest) { m.BackupID = leak }},
		{"instance_id", "instance_id", func(m *domain.Manifest) { m.InstanceID = "" }},
		{"sync_epoch", "sync_epoch", func(m *domain.Manifest) { m.SyncEpoch = leak }},
		{"kind", "kind", func(m *domain.Manifest) { m.Kind = "restore_test" }},
		{"started_at", "started_at", func(m *domain.Manifest) { m.StartedAt = time.Time{} }},
		{"finished_at", "finished_at", func(m *domain.Manifest) { m.FinishedAt = time.Time{} }},
		{"finished before started", "finished_at", func(m *domain.Manifest) { m.FinishedAt = m.StartedAt.Add(-time.Second) }},
		{"server_version text", "server_version", func(m *domain.Manifest) { m.ServerVersion = leak }},
		{"server_version empty", "server_version", func(m *domain.Manifest) { m.ServerVersion = "" }},
		{"server_version long", "server_version", func(m *domain.Manifest) { m.ServerVersion = strings.Repeat("1", 65) }},
		{"schema_version", "schema_version", func(m *domain.Manifest) { m.SchemaVersion = -1 }},
		{"head_seq", "event_log.head_seq", func(m *domain.Manifest) { m.EventLog.HeadSeq = -1 }},
		{"min_retained_seq", "event_log.min_retained_seq", func(m *domain.Manifest) { m.EventLog.MinRetainedSeq = -1 }},
		{"min above head", "event_log.min_retained_seq", func(m *domain.Manifest) { m.EventLog.MinRetainedSeq = m.EventLog.HeadSeq + 1 }},
		{"audit seq", "audit.last_seq", func(m *domain.Manifest) { m.Audit.LastSeq = -1 }},
		{"audit hash text", "audit.last_hash", func(m *domain.Manifest) { m.Audit.LastHash = leak }},
		{"audit hash upper", "audit.last_hash", func(m *domain.Manifest) { m.Audit.LastHash = strings.Repeat("AB", 32) }},
		{"audit hash missing", "audit.last_hash", func(m *domain.Manifest) { m.Audit.LastHash = "" }},
		{"audit hash without seq", "audit.last_hash", func(m *domain.Manifest) { m.Audit = domain.AuditHead{LastHash: strings.Repeat("ab", 32)} }},
		{"row_counts key", "row_counts", func(m *domain.Manifest) { m.RowCounts[leak] = 1 }},
		{"row_counts value", "row_counts", func(m *domain.Manifest) { m.RowCounts["messages"] = -1 }},
		{"row_counts nil", "row_counts", func(m *domain.Manifest) { m.RowCounts = nil }},
		{"files.count", "files.count", func(m *domain.Manifest) { m.Files.Count = -1 }},
		{"files.total_bytes", "files.total_bytes", func(m *domain.Manifest) { m.Files.TotalBytes = -1 }},
		{"pgbackrest on logical", "pgbackrest", func(m *domain.Manifest) { m.PgBackRest = &domain.PgBackRestRef{Label: "20261018-020000F"} }},
		{"restic missing on logical", "restic", func(m *domain.Manifest) { m.Restic = nil }},
		{"restic id text", "restic.snapshot_id", func(m *domain.Manifest) { m.Restic.SnapshotID = leak }},
		{"restic id short", "restic.snapshot_id", func(m *domain.Manifest) { m.Restic.SnapshotID = "0f0f0f0f" }},
		{"journal number zero", "deletion_journal.next_number", func(m *domain.Manifest) { m.DeletionJournal.NextNumber = 0 }},
	}
	for _, c := range bad {
		m := validManifest(domain.KindLogical)
		m.RowCounts = maps.Clone(m.RowCounts)
		c.mut(&m)
		assertInvalid(t, c.name, c.field, leak, m)
	}
	physical := []struct {
		name, field string
		mut         func(m *domain.Manifest)
	}{
		{"pgbackrest missing", "pgbackrest", func(m *domain.Manifest) { m.PgBackRest = nil }},
		{"label text", "pgbackrest.label", func(m *domain.Manifest) { m.PgBackRest.Label = leak }},
		{"label incremental of full", "pgbackrest.label", func(m *domain.Manifest) { m.PgBackRest.Label = "20261018-020000I" }},
		{"restic on physical", "restic", func(m *domain.Manifest) { m.Restic = &domain.ResticRef{SnapshotID: strings.Repeat("0f", 32)} }},
	}
	for _, c := range physical {
		m := validManifest(domain.KindPGFull)
		c.mut(&m)
		assertInvalid(t, c.name, c.field, leak, m)
	}
	for _, label := range []string{"20261018-020000F", "20261018-020000F_20261019-020000D", "20261018-020000F_20261019-020000I"} {
		m := validManifest(domain.KindPGDiff)
		m.PgBackRest.Label = label
		if err := m.Validate(); err != nil {
			t.Errorf("label %s: %v", label, err)
		}
	}

	// ParseManifest is strict: an extra key, a trailing value or a field of message text fail.
	b, err := validManifest(domain.KindLogical).Marshal()
	if err != nil {
		t.Fatal(err)
	}
	var obj map[string]any
	if err := json.Unmarshal(b, &obj); err != nil {
		t.Fatal(err)
	}
	obj["note"] = leak
	extra, _ := json.Marshal(obj)
	for name, in := range map[string][]byte{
		"extra key":      extra,
		"trailing value": append(slices.Clone(b), []byte(` {}`)...),
		"not an object":  []byte(`[]`),
		"message text":   []byte(strings.Replace(string(b), `"1.4.0-rc.1+build.7"`, `"`+leak+`"`, 1)),
		"nested key":     []byte(strings.Replace(string(b), `"head_seq":120`, `"head_seq":120,"note":1`, 1)),
	} {
		_, err := domain.ParseManifest(in)
		if !errors.Is(err, domain.ErrInvalidManifest) {
			t.Errorf("ParseManifest(%s) = %v, want ErrInvalidManifest", name, err)
			continue
		}
		if strings.Contains(err.Error(), leak) {
			t.Errorf("ParseManifest(%s) = %q echoes the value", name, err)
		}
	}
}

// assertInvalid checks that m fails Validate and Marshal with ErrInvalidManifest naming field,
// without the text leak.
func assertInvalid(t *testing.T, name, field, leak string, m domain.Manifest) {
	t.Helper()
	err := m.Validate()
	if !errors.Is(err, domain.ErrInvalidManifest) {
		t.Errorf("%s: Validate = %v, want ErrInvalidManifest", name, err)
		return
	}
	if !strings.Contains(err.Error(), field) {
		t.Errorf("%s: Validate = %q, want the field %q named", name, err, field)
	}
	if strings.Contains(err.Error(), leak) || strings.Contains(err.Error(), "data") {
		t.Errorf("%s: Validate = %q echoes the value", name, err)
	}
	if b, err := m.Marshal(); !errors.Is(err, domain.ErrInvalidManifest) || b != nil {
		t.Errorf("%s: Marshal = %q, %v, want no bytes and ErrInvalidManifest", name, b, err)
	}
}
