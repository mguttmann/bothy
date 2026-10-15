// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"reflect"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// addAgent registers an active agent with an agents row in the fakes.
func (e *env) addAgent(status identity.Status, row bool) ids.ID {
	id := ids.New()
	e.dir.principals[id] = identity.PrincipalView{ID: id, Kind: actor.KindAgent, Status: status}
	e.pol.rows[id] = row
	return id
}

// match is a valid If-Match of version.
func match(v int64) chat.Precondition {
	return chat.Precondition{Present: true, Valid: true, Version: v}
}

// lastAttrs returns the attributes of the last audit entry of act.
func (e *env) lastAttrs(t *testing.T, act authorize.Action) auditing.Attrs {
	t.Helper()
	e.rec.mu.Lock()
	defer e.rec.mu.Unlock()
	for i := len(e.rec.entries) - 1; i >= 0; i-- {
		if e.rec.entries[i].Action == string(act) {
			return e.rec.entries[i].Attrs
		}
	}
	t.Fatalf("no audit entry of %s", act)
	return nil
}

// TestOperatorUseCases: both operator use cases over the fakes, with versions, events, audit
// entries, the affected principals, the rights change declaration and the refusals of D1 and D2.
func TestOperatorUseCases(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	lotse, texter := e.addAgent(identity.StatusActive, true), e.addAgent(identity.StatusActive, true)
	disabled, rowless := e.addAgent(identity.StatusDisabled, true), e.addAgent(identity.StatusActive, false)
	sp := e.create(t, "Support", ids.ID{})
	inherit, own := e.channel(t, sp.ID, "text", "inherit"), e.channel(t, sp.ID, "text", "own")

	// The channel gets its own operator.
	c, err := e.svc.SetChannelOperator(ctx, e.owner, chat.SetChannelOperatorCmd{ChannelID: own.ID, Reason: "Own",
		Operator: chat.OperatorAssignment{Mode: "agent", AgentID: texter}, IfMatch: match(own.Version)})
	if err != nil || c.Version != own.Version+1 || c.Operator.Mode != "agent" || c.Operator.EffectiveAgentID != texter {
		t.Fatalf("channel %+v %v", c, err)
	}
	if !slices.Contains(e.affected.list, texter) {
		t.Fatalf("affected %v", e.affected.list)
	}
	if a := e.lastAttrs(t, authorize.ActionChannelOperatorSet); !reflect.DeepEqual(a["new_agent_id"], auditing.ID(texter)) || !reflect.DeepEqual(a["old_mode"], auditing.Code("inherit")) {
		t.Fatalf("audit %v", a)
	}
	if o := e.uow.opts[len(e.uow.opts)-1]; !o.RightsChange || o.Action != "channel.operator.set" {
		t.Fatalf("rights change %+v", o)
	}

	// The space default bumps the inheriting channel only.
	events := len(e.events.evs)
	s, err := e.svc.SetSpaceOperator(ctx, e.owner, chat.SetSpaceOperatorCmd{SpaceID: sp.ID, Reason: "Default",
		Operator: chat.OperatorAssignment{Mode: "agent", AgentID: lotse}, IfMatch: match(sp.Version)})
	if err != nil || s.DefaultOperatorAgentID != lotse || s.Version != sp.Version+1 {
		t.Fatalf("space %+v %v", s, err)
	}
	if got := e.events.evs[events:]; len(got) != 2 || got[0].Type != "space.updated" || got[1].Type != "channel.updated" || got[1].ResourceID != inherit.ID {
		t.Fatalf("events %+v", got)
	}
	if v, _ := e.svc.GetChannel(ctx, e.owner, inherit.ID); v.Channel.Version != inherit.Version+1 || v.Channel.Operator.EffectiveAgentID != lotse {
		t.Fatalf("inheriting %+v", v.Channel)
	}
	if v, _ := e.svc.GetChannel(ctx, e.owner, own.ID); v.Channel.Version != c.Version {
		t.Fatalf("own channel touched %+v", v.Channel)
	}

	// The same assignment changes nothing and records no_change.
	events = len(e.events.evs)
	if s2, err := e.svc.SetSpaceOperator(ctx, e.owner, chat.SetSpaceOperatorCmd{SpaceID: sp.ID, Reason: "Same",
		Operator: chat.OperatorAssignment{Mode: "agent", AgentID: lotse}, IfMatch: match(s.Version)}); err != nil || s2.Version != s.Version {
		t.Fatalf("no-op space %+v %v", s2, err)
	}
	if a := e.lastAttrs(t, authorize.ActionSpaceOperatorSet); !reflect.DeepEqual(a["no_change"], auditing.Bool(true)) {
		t.Fatalf("no-op audit %v", a)
	}
	if c2, err := e.svc.SetChannelOperator(ctx, e.owner, chat.SetChannelOperatorCmd{ChannelID: own.ID, Reason: "Same",
		Operator: chat.OperatorAssignment{Mode: "agent", AgentID: texter}, IfMatch: match(c.Version)}); err != nil || c2.Version != c.Version {
		t.Fatalf("no-op channel %+v %v", c2, err)
	}
	if len(e.events.evs) != events {
		t.Fatalf("no-op appended events")
	}

	// Inherit and none on a channel, none on the space.
	c, err = e.svc.SetChannelOperator(ctx, e.owner, chat.SetChannelOperatorCmd{ChannelID: own.ID, Reason: "Back",
		Operator: chat.OperatorAssignment{Mode: "inherit"}, IfMatch: match(c.Version)})
	if err != nil || c.Operator.Mode != "inherit" || c.Operator.EffectiveAgentID != lotse {
		t.Fatalf("inherit %+v %v", c, err)
	}
	if s, err = e.svc.SetSpaceOperator(ctx, e.owner, chat.SetSpaceOperatorCmd{SpaceID: sp.ID, Reason: "None",
		Operator: chat.OperatorAssignment{Mode: "none"}, IfMatch: match(s.Version)}); err != nil || !s.DefaultOperatorAgentID.IsZero() {
		t.Fatalf("none %+v %v", s, err)
	}

	cur, _ := e.svc.GetChannel(ctx, e.owner, own.ID)
	refused := []struct {
		name string
		cmd  chat.SetChannelOperatorCmd
		code errs.Code
	}{
		{"missing If-Match", chat.SetChannelOperatorCmd{ChannelID: own.ID, Reason: "x", Operator: chat.OperatorAssignment{Mode: "none"}}, errs.CodePreconditionRequired},
		{"stale If-Match", chat.SetChannelOperatorCmd{ChannelID: own.ID, Reason: "x", Operator: chat.OperatorAssignment{Mode: "none"}, IfMatch: match(1)}, errs.CodeVersionConflict},
		{"disabled agent", chat.SetChannelOperatorCmd{ChannelID: own.ID, Reason: "x", Operator: chat.OperatorAssignment{Mode: "agent", AgentID: disabled}, IfMatch: match(cur.Channel.Version)}, errs.CodeValidationFailed},
		{"agent without row", chat.SetChannelOperatorCmd{ChannelID: own.ID, Reason: "x", Operator: chat.OperatorAssignment{Mode: "agent", AgentID: rowless}, IfMatch: match(cur.Channel.Version)}, errs.CodeValidationFailed},
		{"unknown agent", chat.SetChannelOperatorCmd{ChannelID: own.ID, Reason: "x", Operator: chat.OperatorAssignment{Mode: "agent", AgentID: ids.New()}, IfMatch: match(cur.Channel.Version)}, errs.CodeValidationFailed},
		{"blank reason", chat.SetChannelOperatorCmd{ChannelID: own.ID, Reason: " ", Operator: chat.OperatorAssignment{Mode: "none"}, IfMatch: match(cur.Channel.Version)}, errs.CodeValidationFailed},
		{"unknown channel", chat.SetChannelOperatorCmd{ChannelID: ids.New(), Reason: "x", Operator: chat.OperatorAssignment{Mode: "none"}, IfMatch: match(1)}, errs.CodeNotFound},
	}
	for _, r := range refused {
		_, err := e.svc.SetChannelOperator(ctx, e.owner, r.cmd)
		if ee, ok := errs.As(err); !ok || ee.Code != r.code {
			t.Errorf("%s: %v, want %s", r.name, err, r.code)
		}
	}
	if _, err := e.svc.SetSpaceOperator(ctx, e.owner, chat.SetSpaceOperatorCmd{SpaceID: sp.ID, Reason: "x",
		Operator: chat.OperatorAssignment{Mode: "inherit"}, IfMatch: match(s.Version)}); errs.KindOf(err) != errs.KindBadRequest {
		t.Errorf("inherit on a space: %v", err)
	}
	if _, err := e.svc.SetSpaceOperator(ctx, e.owner, chat.SetSpaceOperatorCmd{SpaceID: sp.ID, Reason: "x",
		Operator: chat.OperatorAssignment{Mode: "none"}}); errs.KindOf(err) != errs.KindPreconditionRequired {
		t.Errorf("space without If-Match: %v", err)
	}

	// Archived targets are frozen.
	if _, err := e.svc.ArchiveChannel(ctx, e.owner, own.ID, "Done"); err != nil {
		t.Fatal(err)
	}
	cur, _ = e.svc.GetChannel(ctx, e.owner, own.ID)
	_, err = e.svc.SetChannelOperator(ctx, e.owner, chat.SetChannelOperatorCmd{ChannelID: own.ID, Reason: "x",
		Operator: chat.OperatorAssignment{Mode: "none"}, IfMatch: match(cur.Channel.Version)})
	wantCode(t, err, errs.CodeConflict)
	if _, err := e.svc.ArchiveSpace(ctx, e.owner, sp.ID, "Done"); err != nil {
		t.Fatal(err)
	}
	got, _ := e.svc.GetSpace(ctx, e.owner, sp.ID)
	_, err = e.svc.SetSpaceOperator(ctx, e.owner, chat.SetSpaceOperatorCmd{SpaceID: sp.ID, Reason: "x",
		Operator: chat.OperatorAssignment{Mode: "agent", AgentID: lotse}, IfMatch: match(got.Space.Version)})
	wantCode(t, err, errs.CodeConflict)

	// authorize refuses before anything else.
	e.deny(authorize.ActionChannelOperatorSet, authorize.Decision{Outcome: authorize.OutcomeForbidden, Reason: "owner_only"})
	if _, err := e.svc.SetChannelOperator(ctx, e.owner, chat.SetChannelOperatorCmd{ChannelID: inherit.ID}); errs.KindOf(err) != errs.KindForbidden {
		t.Errorf("forbidden: %v", err)
	}
}
