// SPDX-License-Identifier: Apache-2.0

package integration_test

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// The regression tests of the lock cycle of M2-21 (docs/DECISIONS.md D-M2-21-11): a request that
// holds the agents row or a channel row in its unit of work and reads or updates the presence
// tracker, against a storm of heartbeats of the same agent that wait for that row. Before the fix
// both sides waited until a lock timeout and answered 5xx after 15 s. The worlds run one after
// the other (cb2a50f21: parallel worlds overload the shared runner).

// lockCycleRounds is the number of storms per case; the cycle showed in 5 to 8 of 8 rounds.
const lockCycleRounds = 3

// heartbeatStorm sends heartbeats of name with the bodies in turn until stop and returns every
// answer of 500 or more and, unless allow4xx, every other answer that is not 200.
func (w *world) heartbeatStorm(name string, bodies []string, allow4xx bool) (stop func() []string) {
	done := make(chan struct{})
	var wg sync.WaitGroup
	var mu sync.Mutex
	var bad []string
	wg.Add(1)
	go func() {
		defer wg.Done()
		for n := 0; ; n++ {
			select {
			case <-done:
				return
			default:
			}
			s := time.Now()
			r := w.h.Do(w.h.NewRequest(http.MethodPost, "/api/v1/agents/me/heartbeat", bodies[n%len(bodies)], "Authorization", "Bearer "+name))
			if r.StatusCode >= 500 || (!allow4xx && r.StatusCode != http.StatusOK) {
				mu.Lock()
				bad = append(bad, fmt.Sprintf("heartbeat %d after %v: %s", r.StatusCode, time.Since(s), r.Body))
				mu.Unlock()
			}
		}
	}()
	time.Sleep(20 * time.Millisecond)
	return func() []string {
		close(done)
		wg.Wait()
		mu.Lock()
		defer mu.Unlock()
		return bad
	}
}

// underStorm runs req once against a storm of heartbeats of name and requires want within 5 s and
// no failed heartbeat.
func underStorm(t *testing.T, name string, bodies []string, allow4xx bool, want int, req func(w *world) *httpservertest.Response) {
	t.Helper()
	for i := range lockCycleRounds {
		w := newWorld(t)
		w.beatOK(name, bodies[len(bodies)-1])
		stop := w.heartbeatStorm(name, bodies, allow4xx)
		s := time.Now()
		r := req(w)
		took := time.Since(s)
		bad := stop()
		if r.StatusCode != want || len(bad) > 0 || took > 5*time.Second {
			t.Fatalf("round %d: %d after %v: %s\nheartbeats: %v", i, r.StatusCode, took, r.Body, bad)
		}
	}
}

// TestLockCycle_GuidedGrantVsHeartbeatStorm: a guided createGrant switches Assistent in its unit
// and builds agent.updated through the tracker (red team round 1 of M2-21).
func TestLockCycle_GuidedGrantVsHeartbeatStorm(t *testing.T) {
	underStorm(t, "assistent", []string{beatOrganization, beatPersonal}, false, http.StatusCreated, func(w *world) *httpservertest.Response {
		return w.h.Do(w.h.NewRequest(http.MethodPost, "/api/v1/grants", with(w.samNotizen(), resolveJSON("Guided", w.assistent)),
			append(key(), "Authorization", "Bearer owner")...))
	})
}

// TestLockCycle_DisableWithKeyVsHeartbeatStorm: disableAgent under Idempotency-Key holds the agents
// row in the outer unit and calls Tracker.Changed (delta red team of M2-21).
func TestLockCycle_DisableWithKeyVsHeartbeatStorm(t *testing.T) {
	underStorm(t, "lotse", []string{beatOrganization, beatPersonal}, true, http.StatusOK, func(w *world) *httpservertest.Response {
		return w.do("owner", http.MethodPost, "/api/v1/agents/"+w.lotse.String()+"/disable", `{"reason":"Pause"}`, key()...)
	})
}

// TestLockCycle_GuidedOperatorVsReadinessStorm: a guided channel.operator.set holds the channel row
// that the project readiness of the heartbeats takes.
func TestLockCycle_GuidedOperatorVsReadinessStorm(t *testing.T) {
	ready := func(w *world, v string) string {
		return `{"state":"online","reported_billing_mode":"personal","billing_modes_supported":["personal","organization"],` +
			`"billing_evidence":{"key_type":"oauth","method":"resolved_credentials"},"project_readiness":[{"channel_id":"` +
			w.allgemein.String() + `","ready":` + v + `}]}`
	}
	for i := range lockCycleRounds {
		w := newWorld(t)
		w.beatOK("assistent", ready(w, "false"))
		stop := w.heartbeatStorm("assistent", []string{ready(w, "true"), ready(w, "false")}, false)
		s := time.Now()
		r := w.do("owner", http.MethodPut, "/api/v1/channels/"+w.allgemein.String()+"/operator",
			with(agentBody(w.assistent), resolveJSON("Guided", w.assistent)), ifMatch(w.channelVersion(w.allgemein))...)
		took := time.Since(s)
		bad := stop()
		if r.StatusCode != http.StatusOK || len(bad) > 0 || took > 5*time.Second {
			t.Fatalf("round %d: guided operator.set %d after %v: %s\nheartbeats: %v", i, r.StatusCode, took, r.Body, bad)
		}
		if p := w.policy(w.assistent); !strings.HasPrefix(p, "organization") {
			t.Fatalf("round %d: assistent policy %q after the guided flow", i, p)
		}
	}
}

// TestLockCycle_SwitchVsHeartbeatStorm: setAgentBillingMode and set_all to organization hold the
// agents row while they build agent.updated.
func TestLockCycle_SwitchVsHeartbeatStorm(t *testing.T) {
	underStorm(t, "assistent", []string{beatOrganization, beatPersonal}, false, http.StatusOK, func(w *world) *httpservertest.Response {
		return w.do("owner", http.MethodPut, "/api/v1/agents/"+w.assistent.String()+"/billing-mode",
			`{"billing_mode":"organization","reason":"Org"}`, ifMatch(w.agentVersion(w.assistent))...)
	})
	underStorm(t, "assistent", []string{beatOrganization, beatPersonal}, false, http.StatusOK, func(w *world) *httpservertest.Response {
		return w.do("owner", http.MethodPut, "/api/v1/instance/agent-billing-mode", `{"billing_mode":"organization","reason":"Emergency"}`)
	})
}
