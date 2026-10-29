// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	gosync "sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/gateway/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// fakePresence records the heartbeats it takes and answers err.
type fakePresence struct {
	mu     gosync.Mutex
	bodies []string
	actors []ids.ID
	err    error
}

func (p *fakePresence) Heartbeat(ctx context.Context, body json.RawMessage) (agents.PresenceStatus, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	a, _ := actor.FromContext(ctx)
	p.bodies, p.actors = append(p.bodies, string(body)), append(p.actors, a.ID)
	return agents.PresenceStatus{State: agents.StateOnline}, p.err
}

func (p *fakePresence) Statuses(context.Context, []ids.ID) ([]agents.PresenceStatus, error) {
	return nil, nil
}

func (p *fakePresence) ProjectFolders(context.Context, actor.Actor, ids.ID) (agents.ProjectFolders, error) {
	return agents.ProjectFolders{}, nil
}

// calls returns the recorded heartbeats.
func (p *fakePresence) calls() ([]string, []ids.ID) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.bodies...), append([]ids.ID(nil), p.actors...)
}

// agentLimiter is rl.agent_runtime; deny empties the bucket.
type agentLimiter struct{ deny bool }

func (l agentLimiter) AllowAgent(actor.Actor) (bool, int, error) { return !l.deny, 3, nil }

// presenceEnv returns an env with p as the presence and the agent identified on a connection.
func presenceEnv(t *testing.T, p *fakePresence, limit bool) (*env, *app.Connection, *fakeConn) {
	t.Helper()
	e := newEnv(t, func(_ *app.Config, d *app.Deps) {
		d.Presence = p
		d.AgentLimiter = agentLimiter{deny: limit}
	})
	c, fc := e.open()
	e.send(c, "identify", map[string]any{"protocol_version": 1, "api_token": agentToken})
	wantOpen(t, fc)
	return e, c, fc
}

// TestHeartbeatAgentMember_CallsPresence: the member agent goes raw to agents.Presence in the
// session's actor context, after heartbeat_ack (brief M2-20 note 10).
func TestHeartbeatAgentMember_CallsPresence(t *testing.T) {
	t.Parallel()
	p := &fakePresence{}
	e, c, fc := presenceEnv(t, p, false)
	e.send(c, "heartbeat", map[string]any{"last_seq": 0, "agent": map[string]any{"state": "online", "runtime_version": "2.3.1"}})
	wantOpen(t, fc)
	bodies, actors := p.calls()
	if len(bodies) != 1 || bodies[0] != `{"runtime_version":"2.3.1","state":"online"}` || actors[0] != e.agent.ID {
		t.Fatalf("presence calls %v %v", bodies, actors)
	}
	if len(sent[app.HeartbeatAck](fc)) != 1 || len(sent[app.ErrorFrame](fc)) != 0 {
		t.Fatalf("frames %#v", fc.frames)
	}
}

// TestHeartbeatAgentMember_ErrorKeepsConnection: validation_failed, forbidden, rate_limited and
// internal_error of the agent part answer error and keep the connection; heartbeat_ack is still
// sent.
func TestHeartbeatAgentMember_ErrorKeepsConnection(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		err   error
		limit bool
		code  string
	}{
		"validation": {err: errs.New(errs.CodeValidationFailed, "x"), code: "validation_failed"},
		"forbidden":  {err: errs.New(errs.CodeForbidden, "").WithReason("subject_kind_not_allowed"), code: "forbidden"},
		"rate":       {limit: true, code: "rate_limited"},
		"internal":   {err: errs.New(errs.CodeInternalError, "db"), code: "internal_error"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			p := &fakePresence{err: c.err}
			e, conn, fc := presenceEnv(t, p, c.limit)
			e.send(conn, "heartbeat", map[string]any{"last_seq": 0, "agent": map[string]any{"state": "online"}})
			wantOpen(t, fc)
			got := sent[app.ErrorFrame](fc)
			if len(got) != 1 || got[0].Op != "heartbeat" || got[0].Code != c.code {
				t.Fatalf("error frames %#v, want %s", got, c.code)
			}
			if len(sent[app.HeartbeatAck](fc)) != 1 {
				t.Fatal("no heartbeat_ack")
			}
			if bodies, _ := p.calls(); c.limit && len(bodies) != 0 {
				t.Fatal("a rate-limited heartbeat reached the presence")
			}
		})
	}
}

// TestHeartbeatWithoutAgentMember_NoPresence: an agent's heartbeat without the member keeps the
// connection alive and changes no presence (D-M2-20-9).
func TestHeartbeatWithoutAgentMember_NoPresence(t *testing.T) {
	t.Parallel()
	p := &fakePresence{}
	e, c, fc := presenceEnv(t, p, false)
	e.send(c, "heartbeat", map[string]any{"last_seq": 0})
	wantOpen(t, fc)
	if bodies, _ := p.calls(); len(bodies) != 0 {
		t.Fatalf("presence called %v", bodies)
	}
	if len(sent[app.HeartbeatAck](fc)) != 1 {
		t.Fatal("no heartbeat_ack")
	}
}
