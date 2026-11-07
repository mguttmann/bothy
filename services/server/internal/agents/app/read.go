// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/agents/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Get implements agents.Agents: agent.read on the agent, then 404 for an unknown id. An owner
// also sees the stored capability hosts_agents (ownerExtras).
func (s *Service) Get(ctx context.Context, a actor.Actor, id ids.ID) (agents.Agent, error) {
	rec, err := s.load(ctx, a, authorize.ActionAgentRead, id)
	if err != nil {
		return agents.Agent{}, err
	}
	v, err := s.view(ctx, rec)
	if err != nil {
		return agents.Agent{}, err
	}
	owner, err := s.isOwner(ctx, a)
	if err != nil {
		return agents.Agent{}, err
	}
	if owner {
		ownerExtras(&v, rec)
	}
	return v, nil
}

// isOwner reports whether a is a person with an active owners row. agent.read already checked
// the liveness of a, so this only decides which members of the answer a sees. Errors: the
// directory's, except not_found, which is no owner.
func (s *Service) isOwner(ctx context.Context, a actor.Actor) (bool, error) {
	if a.Kind != actor.KindUser {
		return false, nil
	}
	u, err := s.d.Directory.User(ctx, a.ID)
	if errs.KindOf(err) == errs.KindNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return u.IsOwner, nil
}

// ownerExtras adds to v the members only an owner reads: the capability hosts_agents of the last
// stored heartbeat, which the "New agent" entry of the apps needs (docs/spec/26-builtin-agents.md
// 8.1). Readers of a served channel and the agent itself see the agent as before A1-03b, and
// agent.updated never carries it, because its audience includes them.
func ownerExtras(v *agents.Agent, rec domain.Agent) {
	v.Presence.HostsAgents = string(rec.Reported.HostsAgents)
}

// agentsList is the list name of the cursors of listAgents.
const agentsList = "agents"

// agentsCursor is the keyset position of a page: the id of its last item.
type agentsCursor struct {
	ID ids.ID `json:"id"`
}

// listGate is the descriptor of the list as a whole: kind agent without an id. Authorize answers
// it with the liveness of the caller (401 for an anonymous or disabled principal, 403 for a
// pending one) and allows only an owner; every other caller then sees the items authorize allows
// one by one (07 section 7.4 rule 3, docs/DECISIONS.md M1-51).
var listGate = authorize.Resource{Kind: authorize.ResourceAgent}

// List implements agents.Agents (07 section 7.4 rule 3): the candidates come from the table in
// ascending id, each filtered with authorize(p, agent.read, agent) in filter mode, until one more
// than the page size is visible or the table ends. There are no totals.
func (s *Service) List(ctx context.Context, a actor.Actor, p agents.Page) (agents.AgentPage, error) {
	fctx := authorize.Filter(ctx)
	if err := s.check(fctx, a, authorize.ActionAgentRead, listGate); err != nil {
		if e, ok := errs.As(err); !ok || e.Kind() != errs.KindNotFound {
			return agents.AgentPage{}, err
		}
	}
	binding := []byte(a.ID.String() + "\x00agents")
	after, err := s.after(binding, p.Cursor)
	if err != nil {
		return agents.AgentPage{}, err
	}
	limit := p.Limit
	if limit < 1 {
		limit = s.cfg.PageSizeDefault
	}
	limit = min(limit, s.cfg.PageSizeMax)
	visible, err := s.visible(fctx, a, after, limit+1)
	if err != nil {
		return agents.AgentPage{}, err
	}
	owner, err := s.isOwner(ctx, a)
	if err != nil {
		return agents.AgentPage{}, err
	}
	out, err := s.page(ctx, visible, limit, owner)
	if err != nil || len(visible) <= limit {
		return out, err
	}
	if out.NextCursor, err = s.d.Cursors.Encode(agentsList, binding, agentsCursor{ID: visible[limit-1].ID}); err != nil {
		return agents.AgentPage{}, errs.Wrap(errs.CodeInternalError, err, "agents: list cursor")
	}
	return out, nil
}

// after decodes the keyset position of cursor; "" is the first page. Errors: cursor_invalid.
func (s *Service) after(binding []byte, cursor string) (ids.ID, error) {
	if cursor == "" {
		return ids.ID{}, nil
	}
	var k agentsCursor
	if err := s.d.Cursors.Decode(agentsList, binding, cursor, &k); err != nil {
		return ids.ID{}, err
	}
	if k.ID.IsZero() {
		return ids.ID{}, errs.New(errs.CodeCursorInvalid, "")
	}
	return k.ID, nil
}

// page joins the first limit visible records with their principals; owner adds ownerExtras.
func (s *Service) page(ctx context.Context, visible []domain.Agent, limit int, owner bool) (agents.AgentPage, error) {
	visible = visible[:min(len(visible), limit)]
	out := agents.AgentPage{Items: make([]agents.Agent, 0, len(visible))}
	list := make([]ids.ID, 0, len(visible))
	for _, rec := range visible {
		list = append(list, rec.ID)
	}
	principals, err := s.d.Directory.Principals(ctx, list)
	if err != nil {
		return agents.AgentPage{}, err
	}
	for _, rec := range visible {
		pv, ok := principals[rec.ID]
		if !ok {
			return agents.AgentPage{}, errs.New(errs.CodeInternalError, "agents: a record without its principal")
		}
		v := s.agentView(rec, pv)
		if owner {
			ownerExtras(&v, rec)
		}
		out.Items = append(out.Items, v)
	}
	return out, nil
}

// visible reads candidates after the keyset position in batches of want and keeps those the
// caller may read, until want are visible or the table ends.
func (s *Service) visible(ctx context.Context, a actor.Actor, after ids.ID, want int) ([]domain.Agent, error) {
	var out []domain.Agent
	for len(out) < want {
		batch, err := s.d.Store.ListAgents(ctx, after, want)
		if err != nil {
			return nil, err
		}
		if out, err = s.readable(ctx, a, batch, out, want); err != nil {
			return nil, err
		}
		if len(batch) < want {
			break
		}
		after = batch[len(batch)-1].ID
	}
	return out, nil
}

// readable appends to out the records of batch that a may read, up to want in total.
func (s *Service) readable(ctx context.Context, a actor.Actor, batch, out []domain.Agent, want int) ([]domain.Agent, error) {
	for _, rec := range batch {
		if len(out) == want {
			break
		}
		d, err := s.d.Authorizer.Authorize(ctx, a, authorize.ActionAgentRead, agentResource(rec.ID))
		if err != nil {
			return nil, err
		}
		if d.Allowed() {
			out = append(out, rec)
		}
	}
	return out, nil
}
