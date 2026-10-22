// SPDX-License-Identifier: Apache-2.0

package startup

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/host/internal/doctor"
)

// TestPublishOnChange: Publish sends a report only when it differs from the last one it sent, and
// the profile list comes from Deps.Offered (M2-45).
func TestPublishOnChange(t *testing.T) {
	x := newWorld(t)
	x.pair()
	d := x.deps()
	d.Offered = func(*doctor.Result) []doctor.Profile {
		return []doctor.Profile{{Name: "only", DisplayName: "Only", Kind: "shell"}}
	}
	s, code := Start(context.Background(), d)
	if s == nil {
		t.Fatalf("start refused %d", code)
	}
	if r := x.ctl.last(); len(r.Profiles) != 1 || r.Profiles[0].Name != "only" {
		t.Fatalf("report profiles %v", r.Profiles)
	}
	// The entry scans of the start report their results on their own; count only after them.
	s.Runner().WaitScans()
	_, before, _ := x.ctl.count()
	res := s.Runner().RunLight(context.Background())
	s.Publish(context.Background(), res)
	s.Publish(context.Background(), res)
	if _, after, _ := x.ctl.count(); after != before+1 {
		t.Errorf("an unchanged result is sent once: %d reports after %d", after, before)
	}
	x.login[sam] = true
	s.Publish(context.Background(), s.Runner().RunLight(context.Background()))
	if _, after, _ := x.ctl.count(); after != before+2 || len(hd(x.ctl.last(), doctor.HD6)) == 0 {
		t.Errorf("a changed result is sent: %d reports", after)
	}
}
