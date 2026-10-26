// SPDX-License-Identifier: Apache-2.0

package load

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
)

// tokenMargin is how long the tokens of a step must stay valid beyond its planned end, so a slow
// step never meets an expired token or reauth_required (sync.reauth_lead_time).
const tokenMargin = 3 * time.Minute

// connectWorkers bounds the parallel gateway connects of a step.
const connectWorkers = 64

// drainPoll is how often a step checks whether every expected dispatch arrived during Drain.
const drainPoll = 100 * time.Millisecond

// Runner runs ramp steps against a seeded world: the driver of the server, the world of Seed, the
// generator of the texts and the targets a step must hold. Warmup, Window and Drain are the
// timing of a step (the constants of params.go, or SteadyWindow for the capacity gate).
type Runner struct {
	Driver  *Driver
	World   *World
	Gen     Generator
	Targets Targets
	Warmup  time.Duration
	Window  time.Duration
	Drain   time.Duration
}

// sent is one send of a step with what the fan-out check needs.
type sent struct {
	Send
	clientMsgID string
	group       int
}

// Step runs one step: it renews the tokens of the step's senders, connects every sender to the
// gateway (the observers keep their dispatches), sends at plan.Rate open loop for Warmup plus
// Window, waits up to Drain for the dispatches, then closes the connections and sums up the
// window. Violations is not filled here (Ramp does it).
func (r *Runner) Step(ctx context.Context, plan Step) StepResult {
	res := StepResult{Index: plan.Index, Rate: plan.Rate, Senders: plan.Senders, Connections: plan.Connections, Groups: plan.Groups}
	errs := &errorList{}
	// A step that ends before its sends still has a valid window in the result file.
	res.Start = time.Now()
	res.WindowFrom, res.WindowTo = res.Start.Add(r.Warmup), res.Start.Add(r.Warmup+r.Window)
	if plan.Senders > len(r.World.People) {
		errs.add(fmt.Errorf("load: step %d needs %d people, the world has %d", plan.Index, plan.Senders, len(r.World.People)))
		res.Errors = errs.list
		return res
	}
	people := r.World.People[:plan.Senders]
	until := time.Now().Add(r.Warmup + r.Window + r.Drain + tokenMargin)
	r.Driver.logf("step %d: %g messages/s, %d senders in %d groups", plan.Index, plan.Rate, plan.Senders, plan.Groups)
	if err := parallelN(ctx, len(people), connectWorkers, func(ctx context.Context, i int) error {
		return r.Driver.Refresh(ctx, people[i], until)
	}); err != nil {
		errs.add(err)
		res.Errors = errs.list
		return res
	}
	res.Start = time.Now()
	conns := make([]*Conn, len(people))
	var failed sync.Mutex
	_ = parallelN(ctx, len(people), connectWorkers, func(ctx context.Context, i int) error {
		c, err := r.Driver.Connect(ctx, people[i], IsObserver(i))
		if err != nil {
			failed.Lock()
			res.ConnectionErrors++
			failed.Unlock()
			errs.add(err)
			return nil
		}
		conns[i] = c
		return nil
	})
	defer func() {
		for _, c := range conns {
			if c != nil {
				c.Close()
			}
		}
	}()
	start := time.Now().Add(100 * time.Millisecond)
	res.WindowFrom, res.WindowTo = start.Add(r.Warmup), start.Add(r.Warmup+r.Window)
	sends := r.send(ctx, plan, people, start, errs)
	r.drain(ctx, sends, conns, res.WindowFrom, res.WindowTo)
	for _, c := range conns {
		if c != nil && c.Err() != nil {
			res.ConnectionErrors++
			errs.add(c.Err())
		}
	}
	plain := make([]Send, len(sends))
	for i, s := range sends {
		plain[i] = s.Send
	}
	res.Sends = SummarizeSends(plain, res.WindowFrom, res.WindowTo, r.Targets.MessageCreateP99)
	lat, expected := fanout(sends, conns, res.WindowFrom, res.WindowTo)
	res.Fanout = SummarizeFanout(lat, expected)
	res.Errors = errs.list
	return res
}

// send runs the open-loop schedule of a step: message k is due at start + k/rate, comes from
// people[k % len(people)] into the main thread of that person's group, and runs in its own
// goroutine, so a slow answer never delays the next send (no coordinated omission).
func (r *Runner) send(ctx context.Context, plan Step, people []*Person, start time.Time, errs *errorList) []sent {
	total := int(plan.Rate * (r.Warmup + r.Window).Seconds())
	out := make([]sent, total)
	interval := time.Duration(float64(time.Second) / plan.Rate)
	var wg sync.WaitGroup
	for k := range total {
		due := start.Add(time.Duration(k) * interval)
		if d := time.Until(due); d > 0 {
			select {
			case <-ctx.Done():
				wg.Wait()
				return out[:k]
			case <-time.After(d):
			}
		}
		p := people[k%len(people)]
		wg.Go(func() {
			cm := NewUUIDv7()
			text := r.Gen.Content(plan.Index, k)
			g := GroupOf(p.Index)
			status, err := r.Driver.do(ctx, p, p.key, request{op: "createMessage", method: http.MethodPost,
				path: "/threads/" + r.World.Groups[g].Thread.String() + "/messages",
				body: rest.MessageCreate{ClientMsgId: cm, ContentMd: &text}, want: http.StatusCreated})
			if err != nil {
				errs.add(err)
			}
			out[k] = sent{Send: Send{Scheduled: due, Done: time.Now(), Status: status}, clientMsgID: cm.String(), group: g}
		})
	}
	wg.Wait()
	return out
}

// drain waits until the observers received every 201 send of the window, at most Drain.
func (r *Runner) drain(ctx context.Context, sends []sent, conns []*Conn, from, to time.Time) {
	deadline := time.Now().Add(r.Drain)
	for time.Now().Before(deadline) && ctx.Err() == nil {
		if lat, expected := fanout(sends, conns, from, to); len(lat) >= expected {
			return
		}
		select {
		case <-ctx.Done():
		case <-time.After(drainPoll):
		}
	}
}

// fanout returns the latencies from occurred_at to the receipt at the observer of the group for
// every 201 send scheduled in [from, to), and how many dispatches were expected. A send whose
// observer has no connection is expected and missing.
func fanout(sends []sent, conns []*Conn, from, to time.Time) ([]time.Duration, int) {
	got := map[int]map[string]Dispatch{}
	var lat []time.Duration
	expected := 0
	for _, s := range sends {
		if s.Status != http.StatusCreated || s.Scheduled.Before(from) || !s.Scheduled.Before(to) {
			continue
		}
		expected++
		obs := s.group * GroupSize
		if obs >= len(conns) || conns[obs] == nil {
			continue
		}
		m, ok := got[obs]
		if !ok {
			m = map[string]Dispatch{}
			for _, d := range conns[obs].Dispatches() {
				m[d.ClientMsgID] = d
			}
			got[obs] = m
		}
		if d, ok := m[s.clientMsgID]; ok {
			lat = append(lat, d.Received.Sub(d.OccurredAt))
		}
	}
	return lat, expected
}

// Ramp runs steps in order and stops after the first step that misses a target (02 section 17);
// that step and the one before are the result (Result.Capacity).
func (r *Runner) Ramp(ctx context.Context, steps []Step) Result {
	res := Result{Seed: r.Gen.seed, Targets: r.Targets, Params: CurrentParams(), Stopped: "every step held"}
	for _, plan := range steps {
		s := r.Step(ctx, plan)
		s.Missed = s.Violations(r.Targets)
		res.Steps = append(res.Steps, s)
		r.Driver.logf("step %d: goodput %.1f/s, p99 %v, fan-out p99 %v, missed %d", plan.Index, s.Sends.GoodputPerS, s.Sends.P99, s.Fanout.P99, len(s.Missed))
		if len(s.Missed) > 0 {
			res.Stopped = fmt.Sprintf("step %d missed a target", plan.Index)
			break
		}
		if ctx.Err() != nil {
			res.Stopped = "cancelled"
			break
		}
	}
	return res
}

// errorList keeps the first maxErrors errors of a step.
type errorList struct {
	mu   sync.Mutex
	list []string
}

// add keeps err when the list is not full.
func (e *errorList) add(err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.list) < maxErrors {
		e.list = append(e.list, err.Error())
	}
}
