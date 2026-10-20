// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/host/internal/clock/clocktest"
	"github.com/OWNER/bothy/services/host/internal/config"
	"github.com/OWNER/bothy/services/host/internal/doctor"
	"github.com/OWNER/bothy/services/host/internal/state"
)

// gateRunner counts the runs of the gate.
type gateRunner struct {
	mu        sync.Mutex
	res       *doctor.Result
	light     int
	lightUser map[string]int
}

func (g *gateRunner) RunLight(context.Context) *doctor.Result {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.light++
	return g.res
}

func (g *gateRunner) RunLightUser(_ context.Context, u string) *doctor.Result {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.lightUser[u]++
	return g.res
}

func (g *gateRunner) Last() *doctor.Result {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.res
}

func passing() *doctor.Result {
	res := &doctor.Result{Declaration: state.DeclOrganization, Workdirs: map[string][]doctor.Workdir{},
		Profiles: []doctor.Profile{{Name: "shell", DisplayName: "Shell", Kind: config.KindShell}}}
	for _, c := range doctor.Checks {
		res.Checks = append(res.Checks, doctor.CheckResult{Check: c})
	}
	return res
}

func with(check string, e ...doctor.Entry) *doctor.Result {
	res := passing()
	for i := range res.Checks {
		if res.Checks[i].Check == check {
			res.Checks[i].Entries = e
		}
	}
	return res
}

func TestGateConsequences(t *testing.T) {
	type row struct {
		name   string
		res    *doctor.Result
		peer   bool
		host   string
		ends   bool
		alex   string
		samRef string
	}
	fail := func(code, user string) doctor.Entry {
		return doctor.Entry{Status: doctor.Fail, Code: code, SessionUser: user}
	}
	rows := []row{
		{"passing", passing(), false, "", false, "", ""},
		{"HD6 subscription login of one session user", with(doctor.HD6, fail(doctor.CodeSubscriptionLogin, alexOSU)), false, DetailSubscriptionLogin, true, DetailSubscriptionLogin, DetailSubscriptionLogin},
		{"HD6 warning only", with(doctor.HD6, doctor.Entry{Status: doctor.Warn, Code: doctor.CodeAuthStatusUnchecked}), false, "", false, "", ""},
		{"HD11 at a place of Alex", with(doctor.HD11, fail(doctor.CodeSharedLocation, alexOSU)), false, "", false, DetailSharedLocation, ""},
		{"HD11 entry scan running for Alex", with(doctor.HD11, fail(doctor.CodeEntryScanRunning, alexOSU)), false, "", false, DetailSharedLocation, ""},
		{"HD11 foreign entry of Alex", with(doctor.HD11, fail(doctor.CodeForeignEntry, alexOSU)), false, "", false, DetailSharedLocation, ""},
		{"HD11 home unresolvable of Alex", with(doctor.HD11, fail(doctor.CodeHomeUnresolvable, alexOSU)), false, "", false, DetailSharedLocation, ""},
		{"HD11 at a place for all", with(doctor.HD11, fail(doctor.CodeProgramLocationUntrusted, "")), false, DetailSharedLocation, true, DetailSharedLocation, DetailSharedLocation},
		{"HD10 peer mismatch", with(doctor.HD10, fail(doctor.CodePeerMismatch, alexOSU)), false, DetailHelperUntrusted, true, DetailHelperUntrusted, DetailHelperUntrusted},
		{"peer mismatch on the session channel", passing(), true, DetailHelperUntrusted, true, DetailHelperUntrusted, DetailHelperUntrusted},
		{"another start refusing finding", with(doctor.HD2, fail(doctor.CodeForbiddenGroup, "")), false, DetailPTYFailed, false, DetailPTYFailed, DetailPTYFailed},
		{"no result", nil, false, DetailPTYFailed, false, DetailPTYFailed, DetailPTYFailed},
	}
	for _, r := range rows {
		p := PolicyOf(r.res, r.peer)
		if p.Host != r.host || p.Ends != r.ends || p.Refusal(alexOSU) != r.alex || p.Refusal(samOSU) != r.samRef {
			t.Errorf("%s: %+v", r.name, p)
		}
	}
}

func TestGateRuns(t *testing.T) {
	clk := clocktest.New(now)
	gr := &gateRunner{res: passing(), lightUser: map[string]int{}}
	var mu sync.Mutex
	var published int
	var enforced []Policy
	g := NewGate(gr, clk, time.Minute, func(context.Context, *doctor.Result) {
		mu.Lock()
		published++
		mu.Unlock()
	}, func(_ context.Context, p Policy) {
		mu.Lock()
		enforced = append(enforced, p)
		mu.Unlock()
	})
	// Each opening: exactly one Light run in the helper of osu (and HD11 for all inside it).
	if d := g.Admit(context.Background(), alexOSU); d != "" || gr.lightUser[alexOSU] != 1 || gr.light != 0 {
		t.Fatalf("admit %q: %v %d", d, gr.lightUser, gr.light)
	}
	// The interval triggers one Light run in every helper.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go g.Loop(ctx)
	for clk.Pending() == 0 {
		time.Sleep(time.Millisecond)
	}
	gr.mu.Lock()
	gr.res = with(doctor.HD6, doctor.Entry{Status: doctor.Fail, Code: doctor.CodeSubscriptionLogin, SessionUser: samOSU})
	gr.mu.Unlock()
	clk.Advance(time.Minute)
	for {
		gr.mu.Lock()
		n := gr.light
		gr.mu.Unlock()
		mu.Lock()
		e := len(enforced)
		mu.Unlock()
		if n == 1 && e == 2 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	mu.Lock()
	last := enforced[len(enforced)-1]
	pub := published
	mu.Unlock()
	if last.Host != DetailSubscriptionLogin || !last.Ends || pub != 2 {
		t.Errorf("periodic run: %+v, reports %d", last, pub)
	}
	if d := g.Admit(context.Background(), alexOSU); d != DetailSubscriptionLogin {
		t.Errorf("admit after HD6 finding: %q", d)
	}
	// A passing run lifts HD6 and a recorded peer mismatch.
	g.PeerMismatch(context.Background())
	if g.Current().Host != DetailHelperUntrusted {
		t.Error("peer mismatch not recorded")
	}
	gr.mu.Lock()
	gr.res = passing()
	gr.mu.Unlock()
	// The Light run of another session user answers the other helpers from the cache: it lifts
	// HD6 but not the peer mismatch (red team M3).
	if d := g.Admit(context.Background(), samOSU); d != DetailHelperUntrusted {
		t.Errorf("a Light run of one session user lifted the peer mismatch: %q", d)
	}
	// Only a periodic run, which asks every helper, lifts it.
	g.RunOnce(context.Background())
	if d := g.Admit(context.Background(), alexOSU); d != "" {
		t.Errorf("a passing periodic run lifts the findings: %q", d)
	}
}
