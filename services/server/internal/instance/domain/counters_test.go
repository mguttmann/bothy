// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/instance/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

var phases = []string{"none", "rights_begin", "work", "rights_check", "sync", "audit", "phase(42)"}

func TestCheckAdvance_PhaseRule(t *testing.T) {
	t.Parallel()
	allowed := map[string]string{
		domain.CounterPermissionVersion: "rights_begin",
		domain.CounterEventSeq:          "sync",
		domain.CounterAuditSeq:          "audit",
	}
	for counter, only := range allowed {
		for _, p := range phases {
			err := domain.CheckAdvance(counter, p)
			if p == only {
				if err != nil {
					t.Errorf("CheckAdvance(%s, %s) = %v, want nil", counter, p, err)
				}
				continue
			}
			if errs.KindOf(err) != errs.KindInternal || !strings.Contains(err.Error(), counter) || !strings.Contains(err.Error(), p) {
				t.Errorf("CheckAdvance(%s, %s) = %v, want kind internal naming counter and phase", counter, p, err)
			}
		}
	}
}

func TestCheckRead_PhaseRule(t *testing.T) {
	t.Parallel()
	hooks := map[string]bool{"rights_begin": true, "rights_check": true, "sync": true, "audit": true}
	for _, counter := range domain.CounterNames() {
		for _, p := range phases {
			err := domain.CheckRead(counter, p)
			if hooks[p] != (err == nil) {
				t.Errorf("CheckRead(%s, %s) = %v, want ok %v", counter, p, err, hooks[p])
			}
			if err != nil && (errs.KindOf(err) != errs.KindInternal || !strings.Contains(err.Error(), p)) {
				t.Errorf("CheckRead(%s, %s) = %v, want kind internal naming the phase", counter, p, err)
			}
		}
	}
}

func TestCounterNames_RejectUnknown(t *testing.T) {
	t.Parallel()
	if got := domain.CounterNames(); len(got) != 3 {
		t.Fatalf("CounterNames() = %v, want the three counters of 03", got)
	}
	for _, n := range []string{"", "EVENT_SEQ", "event_seq ", "thread_seq"} {
		if err := domain.CheckAdvance(n, "sync"); errs.KindOf(err) != errs.KindInternal {
			t.Errorf("CheckAdvance(%q) = %v, want kind internal", n, err)
		}
		if err := domain.CheckRead(n, "sync"); errs.KindOf(err) != errs.KindInternal {
			t.Errorf("CheckRead(%q) = %v, want kind internal", n, err)
		}
	}
}

func TestCheckAdvanceCount(t *testing.T) {
	t.Parallel()
	for _, n := range []int64{0, -1, -1 << 63} {
		if err := domain.CheckAdvanceCount(n); errs.KindOf(err) != errs.KindInternal {
			t.Errorf("CheckAdvanceCount(%d) = %v, want kind internal", n, err)
		}
	}
	if err := domain.CheckAdvanceCount(1); err != nil {
		t.Errorf("CheckAdvanceCount(1) = %v", err)
	}
}
