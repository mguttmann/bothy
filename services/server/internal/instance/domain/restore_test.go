// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/instance/domain"
)

// TestIdentityMismatch: the same identity matches; another timeline (a point-in-time restore or
// a failover) and another system identifier (a logical restore into a fresh cluster) are
// mismatches; an empty stored identity (an instance older than M7-04) never is (R-DM-50,
// brief M7-04 design note 3).
func TestIdentityMismatch(t *testing.T) {
	t.Parallel()
	stored := domain.Identity{SystemIdentifier: "7698675516970025", TimelineID: 1}
	cases := map[string]struct {
		stored, current domain.Identity
		want            bool
	}{
		"same":                     {stored, stored, false},
		"other timeline":           {stored, domain.Identity{SystemIdentifier: stored.SystemIdentifier, TimelineID: 2}, true},
		"other system identifier":  {stored, domain.Identity{SystemIdentifier: "7698675516970026", TimelineID: 1}, true},
		"both differ":              {stored, domain.Identity{SystemIdentifier: "1", TimelineID: 3}, true},
		"empty stored":             {domain.Identity{}, stored, false},
		"empty stored and current": {domain.Identity{}, domain.Identity{}, false},
	}
	for name, c := range cases {
		if got := domain.IdentityMismatch(c.stored, c.current); got != c.want {
			t.Errorf("%s: IdentityMismatch(%+v, %+v) = %v, want %v", name, c.stored, c.current, got, c.want)
		}
	}
	if !(domain.Identity{}).Empty() || stored.Empty() {
		t.Error("Empty reports the zero identity only")
	}
}

// TestRelocationDifference: a manifest equal to the restored database has no difference; each
// differing field is named, head_seq first, then the audit head and the epoch (design note 3).
func TestRelocationDifference(t *testing.T) {
	t.Parallel()
	same := domain.RelocationFacts{HeadSeq: 42, AuditLastSeq: 7, AuditLastHash: "ab01", SyncEpoch: "0199f0c2-0000-7000-8000-000000000001"}
	if field, ok := domain.RelocationDifference(same, same); !ok || field != "" {
		t.Fatalf("equal facts = %q, %v; want no difference", field, ok)
	}
	cases := map[string]func(f *domain.RelocationFacts){
		domain.FieldHeadSeq:       func(f *domain.RelocationFacts) { f.HeadSeq++ },
		domain.FieldAuditLastSeq:  func(f *domain.RelocationFacts) { f.AuditLastSeq++ },
		domain.FieldAuditLastHash: func(f *domain.RelocationFacts) { f.AuditLastHash = "ab02" },
		domain.FieldSyncEpoch:     func(f *domain.RelocationFacts) { f.SyncEpoch = "0199f0c2-0000-7000-8000-000000000002" },
	}
	for want, change := range cases {
		restored := same
		change(&restored)
		if field, ok := domain.RelocationDifference(same, restored); ok || field != want {
			t.Errorf("changed %s: RelocationDifference = %q, %v", want, field, ok)
		}
	}
	all := domain.RelocationFacts{HeadSeq: 1, AuditLastSeq: 1, AuditLastHash: "ff", SyncEpoch: "x"}
	if field, _ := domain.RelocationDifference(same, all); field != domain.FieldHeadSeq {
		t.Errorf("every field differs: first named %q, want %s", field, domain.FieldHeadSeq)
	}
}
