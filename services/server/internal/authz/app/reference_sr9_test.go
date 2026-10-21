// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"os"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The SR9 dimension of the slow reference (AZ-P8, AZ-P11; brief M2-14 design note 4i), written
// from 07 "SR9" and the mode table, not from the implementation: an agent in a run has the lower
// level and the common capabilities of itself and the triggering person; the person has no rights
// when disabled and the owner rights without a session; replies in the run's own thread use the
// agent's rights alone; an invalid header denies everything, an ended run every action of the
// reference (none of them is unbound); delivery mode ignores the header.

// refRun is a run of an agent triggered by a person in the main thread of a channel.
type refRun struct {
	id, agent, person, thread ids.ID
	ended                     bool
}

// refBinding is the Invocation-Token header of one request: none (""), bound, run_inactive or
// invalid, with its run.
type refBinding struct {
	status string
	run    *refRun
}

// addRun adds a run of a random agent for a random person in the thread of a random channel.
func (m *refWorld) addRun() {
	var agents, people []*refPrincipal
	for _, p := range m.principals {
		switch p.kind {
		case domain.KindAgent:
			agents = append(agents, p)
		case domain.KindUser:
			people = append(people, p)
		}
	}
	c := m.channels[m.rnd.IntN(len(m.channels))]
	m.runs = append(m.runs, &refRun{id: ids.New(), agent: agents[m.rnd.IntN(len(agents))].id,
		person: people[m.rnd.IntN(len(people))].id, thread: c.thread})
}

// bindingFor returns a random binding for the agent p: none, one of its runs (bound or ended), or
// an invalid header.
func (m *refWorld) bindingFor(p *refPrincipal) refBinding {
	var own []*refRun
	for _, r := range m.runs {
		if r.agent == p.id {
			own = append(own, r)
		}
	}
	switch k := m.rnd.IntN(8); {
	case k == 0:
		return refBinding{}
	case k == 1:
		return refBinding{status: "invalid"}
	case len(own) == 0:
		return refBinding{}
	default:
		r := own[m.rnd.IntN(len(own))]
		if r.ended {
			return refBinding{status: "run_inactive", run: r}
		}
		return refBinding{status: "bound", run: r}
	}
}

// actorWith is the actor of p with the header b.
func (m *refWorld) actorWith(p *refPrincipal, b refBinding) actor.Actor {
	a := m.actorOf(p)
	switch b.status {
	case "invalid":
		a.ActingFor = actor.ActingFor{Status: actor.ActingForInvalid}
	case "bound", "run_inactive":
		a.ActingFor = actor.ActingFor{Status: actor.ActingForStatus(b.status), UserID: b.run.person, RunID: b.run.id, ThreadID: b.run.thread, JTI: "jti"}
	}
	return a
}

// principalByID returns the model principal id.
func (m *refWorld) principalByID(id ids.ID) *refPrincipal {
	for _, p := range m.principals {
		if p.id == id {
			return p
		}
	}
	return nil
}

// refRights are a level with the three capabilities of the reference.
type refRights struct {
	level                  int
	upload, manage, export bool
}

// agentRights and personRights are the reference rights; a disabled person has none (SR7).
func (m *refWorld) agentRights(p *refPrincipal, space ids.ID, c *refChannel) refRights {
	l, u, mg, e := m.rights(p, space, c)
	return refRights{l, u, mg, e}
}

func (m *refWorld) personRights(person *refPrincipal, space ids.ID, c *refChannel) refRights {
	if person == nil || person.disabled {
		return refRights{}
	}
	l, u, mg, e := m.rights(person, space, c)
	return refRights{l, u, mg, e}
}

// meet is the intersection of 05 section 2.3 point 6: lower level, common capabilities.
func meet(a, b refRights) refRights {
	return refRights{min(a.level, b.level), a.upload && b.upload, a.manage && b.manage, a.export && b.export}
}

// spaceVisible is the visibility of a space with the rights function f (07 section 4.3).
func (m *refWorld) spaceVisible(space ids.ID, f func(ids.ID, *refChannel) refRights) bool {
	if f(space, nil).level >= 1 {
		return true
	}
	for _, ch := range m.channels {
		if ch.space == space && !ch.dm && f(space, ch).level >= 1 {
			return true
		}
	}
	return false
}

// requirementReason evaluates the requirement of the reference actions with r: "" when it holds,
// else the reason of the first failing part.
func requirementReason(a authorize.Action, r refRights) string {
	//exhaustive:ignore // the reference covers only its actions; every other action has no requirement here.
	switch a {
	case authorize.ActionMessageCreate:
		if r.level < 2 {
			return "level_insufficient"
		}
	case authorize.ActionFileUpload:
		if r.level < 2 {
			return "level_insufficient"
		}
		if !r.upload {
			return "capability_missing"
		}
	case authorize.ActionChannelUpdate:
		if !r.manage {
			return "capability_missing"
		}
	case authorize.ActionExportChannel:
		if !r.export {
			return "capability_missing"
		}
	}
	return ""
}

// decideSR9 is the reference decision of the agent p with the header b in normal (request) mode.
// detail is the Detail of invocation_token_invalid.
func (m *refWorld) decideSR9(p *refPrincipal, b refBinding, a authorize.Action, c *refChannel, space ids.ID) (o authorize.Outcome, reason, detail string) {
	if p.disabled {
		return authorize.OutcomeUnauthenticated, "principal_inactive", ""
	}
	switch b.status {
	case "":
		o, r := m.decide(p, a, c, space)
		return o, r, ""
	case "invalid", "run_inactive":
		return authorize.OutcomeForbidden, "invocation_token_invalid", b.status
	}
	if a == authorize.ActionMessageCreate && c != nil && c.thread == b.run.thread {
		// The reply in the run's own thread uses the agent's rights alone (07 "SR9").
		o, r := m.decide(p, a, c, space)
		return o, r, ""
	}
	person := m.principalByID(b.run.person)
	own := func(s ids.ID, ch *refChannel) refRights { return m.agentRights(p, s, ch) }
	both := func(s ids.ID, ch *refChannel) refRights { return meet(own(s, ch), m.personRights(person, s, ch)) }
	if a == authorize.ActionSpaceRead {
		switch {
		case m.spaceVisible(space, both):
			return authorize.OutcomeAllow, "", ""
		case m.spaceVisible(space, own):
			return authorize.OutcomeNotFound, "acting_for_insufficient", ""
		}
		return authorize.OutcomeNotFound, "no_grant", ""
	}
	base, eff := own(c.space, c), both(c.space, c)
	switch {
	case base.level < 1:
		return authorize.OutcomeNotFound, "no_grant", ""
	case eff.level < 1:
		return authorize.OutcomeNotFound, "acting_for_insufficient", ""
	}
	if r := requirementReason(a, base); r != "" {
		return authorize.OutcomeForbidden, r, ""
	}
	if requirementReason(a, eff) != "" {
		return authorize.OutcomeForbidden, "acting_for_insufficient", ""
	}
	if (a == authorize.ActionMessageCreate || a == authorize.ActionFileUpload) && c.readOnly {
		return authorize.OutcomeForbidden, "channel_read_only", ""
	}
	return authorize.OutcomeAllow, "", ""
}

// scopeSR9 is the reference of Scope(message.read) for the agent p with a bound header: the
// channels both read.
func (m *refWorld) scopeSR9(p *refPrincipal, b refBinding) []ids.ID {
	person := m.principalByID(b.run.person)
	var out []ids.ID
	for _, c := range m.channels {
		if meet(m.agentRights(p, c.space, c), m.personRights(person, c.space, c)).level >= 1 {
			out = append(out, c.id)
		}
	}
	return domain.SortIDs(out)
}

// endRun ends a random run (completed, failed or cancelled are alike for SR9).
func (m *refWorld) endRun() string {
	if len(m.runs) == 0 {
		return "end run"
	}
	m.runs[m.rnd.IntN(len(m.runs))].ended = true
	return "end run"
}

// checkSR9 compares every decision and the scope of the agents with random headers with the
// reference at one step, also in delivery mode, and returns the bound requests that were allowed
// (for the property of AZ-P11).
func (m *refWorld) checkSR9(t *testing.T, where string, actions []authorize.Action, trail []string) {
	t.Helper()
	for _, p := range m.principals {
		if p.kind != domain.KindAgent {
			continue
		}
		b := m.bindingFor(p)
		a := m.actorWith(p, b)
		for _, act := range actions {
			c := m.channels[m.rnd.IntN(len(m.channels))]
			if b.run != nil && m.rnd.IntN(3) == 0 {
				c = m.channelOfThread(b.run.thread)
			}
			r := m.resourceFor(act, c)
			wantO, wantR, wantD := m.decideSR9(p, b, act, c, r.ID)
			d, err := m.az.Authorize(context.Background(), a, act, r)
			if err != nil || d.Outcome != wantO || string(d.Reason) != wantR || d.Detail != wantD {
				t.Fatalf("%s, %s %s with header %q: got %s/%s/%s %v, want %s/%s/%s\ntrail %v",
					where, p.kind, act, b.status, d.Outcome, d.Reason, d.Detail, err, wantO, wantR, wantD, trail)
			}
			if act == authorize.ActionMessageRead {
				// Delivery mode ignores the header (07 section 8.1): the same decision as without it.
				// Delivery snapshots follow the log tail (Observe), which this model does not feed,
				// so the comparison is with the request without the header, not with the model.
				del := authorize.Delivery(context.Background())
				want, err1 := m.az.Authorize(del, m.actorOf(p), act, r)
				dd, err := m.az.Authorize(del, a, act, r)
				if err != nil || err1 != nil || dd != want {
					t.Fatalf("%s, delivery %s with header %q: got %+v %v, want %+v %v\ntrail %v", where, act, b.status, dd, err, want, err1, trail)
				}
			}
		}
		if p.disabled {
			continue
		}
		sc, err := m.az.Scope(context.Background(), a, authorize.ActionMessageRead)
		switch b.status {
		case "":
			if want := m.visibleChannels(p); err != nil || !slices.Equal(sc.ChannelIDs, want) {
				t.Fatalf("%s, scope without header: got %v %v, want %v\ntrail %v", where, sc.ChannelIDs, err, want, trail)
			}
		case "bound":
			if want := m.scopeSR9(p, b); err != nil || !slices.Equal(sc.ChannelIDs, want) {
				t.Fatalf("%s, bound scope: got %v %v, want %v\ntrail %v", where, sc.ChannelIDs, err, want, trail)
			}
		default:
			if e, ok := errs.As(err); !ok || e.Code != errs.CodeInvocationTokenInvalid || e.Reason != b.status {
				t.Fatalf("%s, scope with header %q: %v\ntrail %v", where, b.status, err, trail)
			}
		}
	}
}

// channelOfThread returns the channel whose main thread is thread.
func (m *refWorld) channelOfThread(thread ids.ID) *refChannel {
	for _, c := range m.channels {
		if c.thread == thread {
			return c
		}
	}
	return m.channels[0]
}

// testSeed is the seed of a random test: BOTHY_TEST_SEED or the clock.
func testSeed(t *testing.T) uint64 {
	t.Helper()
	seed := uint64(time.Now().UnixNano())
	if s := os.Getenv("BOTHY_TEST_SEED"); s != "" { //nolint:forbidigo // test input (seed), not server configuration
		n, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		seed = n
	}
	return seed
}

// p11Reads and p11Writes are the content reads and channel writes of AZ-P11 with their resource
// on the main thread of a channel; every write needs write on the path.
var (
	p11Reads  = []authorize.Action{authorize.ActionMessageRead, authorize.ActionThreadRead, authorize.ActionChannelRead, authorize.ActionRunRead, authorize.ActionMessageRevisionsRead}
	p11Writes = []authorize.Action{authorize.ActionMessageCreate, authorize.ActionMessageUpdate, authorize.ActionMessageDelete, authorize.ActionReactionAdd,
		authorize.ActionThreadCreate, authorize.ActionThreadClose}
)

// p11Resource is the resource of act on channel c; the message actions name a message of the
// agent itself, so the author rule never decides.
func p11Resource(act authorize.Action, c *refChannel, agent ids.ID) authorize.Resource {
	//exhaustive:ignore // the property names only its actions; the default covers the rest.
	switch act {
	case authorize.ActionMessageRead, authorize.ActionMessageRevisionsRead, authorize.ActionMessageUpdate, authorize.ActionMessageDelete,
		authorize.ActionReactionAdd:
		kind := authorize.ResourceMessage
		if act == authorize.ActionReactionAdd {
			kind = authorize.ResourceMessage
		}
		return authorize.Resource{Kind: kind, ID: ids.New(), ThreadID: c.thread, OwnerID: agent}
	case authorize.ActionRunRead:
		return authorize.Resource{Kind: authorize.ResourceRun, ID: ids.New(), ThreadID: c.thread, OwnerID: agent}
	case authorize.ActionChannelRead, authorize.ActionThreadCreate:
		return authorize.Resource{Kind: authorize.ResourceChannel, ID: c.id}
	}
	return authorize.Resource{Kind: authorize.ResourceThread, ID: c.thread}
}

// TestAZP11_ActingForProperty (AZ-P11): in random worlds of people, agents, groups, grants with
// expiries, operators, runs and statuses, no agent in a run reads or writes what the triggering
// person may not: an allowed content read implies read of the person on the path, an allowed
// channel write (a reply outside the run's own thread) write of the person; the agent's Scope is a
// subset of the person's; and every decision equals the SR9 reference.
func TestAZP11_ActingForProperty(t *testing.T) {
	seed := testSeed(t)
	actions := []authorize.Action{authorize.ActionMessageRead, authorize.ActionMessageCreate, authorize.ActionThreadRead,
		authorize.ActionChannelRead, authorize.ActionSpaceRead, authorize.ActionChannelUpdate, authorize.ActionExportChannel,
		authorize.ActionFileUpload}
	checked := 0
	for w := range 20 {
		ws := seed + uint64(w)
		m := newRefWorld(t, ws)
		var trail []string
		for step := range 40 {
			trail = append(trail, m.mutate())
			where := "seed " + strconv.FormatUint(seed, 10) + " (BOTHY_TEST_SEED), world " + strconv.Itoa(w) + ", step " + strconv.Itoa(step)
			m.checkSR9(t, where, actions, trail)
			for _, p := range m.principals {
				if p.kind != domain.KindAgent || p.disabled {
					continue
				}
				b := m.bindingFor(p)
				if b.status != "bound" {
					continue
				}
				a := m.actorWith(p, b)
				person := m.principalByID(b.run.person)
				for _, act := range append(slices.Clone(p11Reads), p11Writes...) {
					c := m.channels[m.rnd.IntN(len(m.channels))]
					d, err := m.az.Authorize(context.Background(), a, act, p11Resource(act, c, p.id))
					if err != nil {
						t.Fatalf("%s, %s: %v", where, act, err)
					}
					if !d.Allowed() {
						continue
					}
					if act == authorize.ActionMessageCreate && c.thread == b.run.thread {
						continue // the reply exception of SR9
					}
					need := 1
					if slices.Contains(p11Writes, act) {
						need = 2
					}
					checked++
					if got := m.personRights(person, c.space, c).level; got < need {
						t.Fatalf("%s, %s allowed for the agent in a run of a person with level %d (need %d)\ntrail %v", where, act, got, need, trail)
					}
				}
				sc, err := m.az.Scope(context.Background(), a, authorize.ActionMessageRead)
				if err != nil {
					t.Fatal(err)
				}
				var personSees []ids.ID
				for _, c := range m.channels {
					if m.personRights(person, c.space, c).level >= 1 {
						personSees = append(personSees, c.id)
					}
				}
				for _, id := range sc.ChannelIDs {
					if !slices.Contains(personSees, id) {
						t.Fatalf("%s, Scope of the agent names a channel the person cannot read\ntrail %v", where, trail)
					}
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("the property never saw an allowed request (vacuous)")
	}
	t.Logf("seed %d, %d allowed bound requests checked", seed, checked)
}
