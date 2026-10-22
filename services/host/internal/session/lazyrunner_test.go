// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/clock/clocktest"
	"github.com/OWNER/bothy/services/host/internal/doctor"
)

// TestLazyRunnerBeforeAndAfterSet: before Set every run gives no result, which refuses every
// session without ending one; after Set the runs reach the doctor runner.
func TestLazyRunnerBeforeAndAfterSet(t *testing.T) {
	var l LazyRunner
	ctx := context.Background()
	if l.RunLight(ctx) != nil || l.RunLightUser(ctx, alexOSU) != nil || l.Last() != nil {
		t.Fatal("a lazy runner without a runner returned a result")
	}
	g := NewGate(&l, clocktest.New(now), time.Minute, nil, nil)
	if d := g.Admit(ctx, alexOSU); d != DetailPTYFailed {
		t.Errorf("admit before Set: %q", d)
	}
	if p := g.Current(); p.Host != DetailPTYFailed || p.Ends {
		t.Errorf("policy before Set: %+v", p)
	}
	if g.Last() != nil {
		t.Error("the gate reported a result before Set")
	}

	gr := &gateRunner{res: passing(), lightUser: map[string]int{}}
	l.Set(gr)
	if d := g.Admit(ctx, alexOSU); d != "" {
		t.Errorf("admit after Set: %q", d)
	}
	if p := g.RunOnce(ctx); p.Host != "" {
		t.Errorf("periodic run after Set: %+v", p)
	}
	if gr.lightUser[alexOSU] != 1 || gr.light != 1 {
		t.Errorf("runs reached the runner: user %v, light %d", gr.lightUser, gr.light)
	}
	if g.Last() != gr.res {
		t.Error("Last is not the result of the runner")
	}
}

// TestPolicyIgnoresHD11Warnings: an HD11 entry that is not a failure refuses nobody.
func TestPolicyIgnoresHD11Warnings(t *testing.T) {
	res := with(doctor.HD11, doctor.Entry{Status: doctor.Warn, Code: doctor.CodeSharedLocation, SessionUser: alexOSU},
		doctor.Entry{Status: doctor.Warn, Code: doctor.CodeSharedLocation})
	p := PolicyOf(res, false)
	if p.Host != "" || p.Ends || p.Refusal(alexOSU) != "" || len(p.Users) != 0 {
		t.Errorf("an HD11 warning refused: %+v", p)
	}
}
