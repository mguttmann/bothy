// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// hostsList is the list name of the cursors of listHosts.
const hostsList = "terminal.hosts"

// hostsCursor is the keyset position of a page: name and id of its last item; the query compares
// lower(name), so the stored name is enough.
type hostsCursor struct {
	Name string `json:"n"`
	ID   string `json:"id"`
}

// List implements terminal.Hosts (07 section 7.4 rules 1 and 3). Scope(host.read) gives the hosts
// the caller reads (every host for an owner, the own and the terminal hosts otherwise) and refuses
// agents, apps and personal tokens; the query takes candidates after the keyset position ordered
// by lower(name) then id, each filtered with authorize(host.read, host) in filter mode, until one
// more than the page size is visible or the candidates end. Hosts in every state are listed,
// retired ones included. There are no totals.
func (s *Service) List(ctx context.Context, a actor.Actor, p terminal.Page) (terminal.HostPage, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return terminal.HostPage{}, err
	}
	sc, err := s.d.Authorizer.Scope(ctx, a, authorize.ActionHostRead)
	if err != nil {
		return terminal.HostPage{}, err
	}
	binding := []byte(a.ID.String() + "\x00hosts")
	afterName, afterID, err := s.after(binding, p.Cursor)
	if err != nil {
		return terminal.HostPage{}, err
	}
	limit := p.Limit
	if limit < 1 {
		limit = s.cfg.PageSizeDefault
	}
	limit = min(limit, s.cfg.PageSizeMax)
	visible, err := s.visible(authorize.Filter(ctx), a, sc, afterName, afterID, limit+1)
	if err != nil {
		return terminal.HostPage{}, err
	}
	full := len(visible) > limit
	visible = visible[:min(len(visible), limit)]
	list := make([]ids.ID, 0, len(visible))
	for _, h := range visible {
		list = append(list, h.ID)
	}
	users, err := s.d.Store.ValidSessionUsers(ctx, list)
	if err != nil {
		return terminal.HostPage{}, err
	}
	out := terminal.HostPage{Items: make([]terminal.HostView, 0, len(visible))}
	for _, h := range visible {
		out.Items = append(out.Items, s.view(ctx, hostState{host: h, users: users[h.ID]}))
	}
	if full {
		last := visible[len(visible)-1]
		if out.NextCursor, err = s.d.Cursors.Encode(hostsList, binding, hostsCursor{Name: last.Name, ID: last.ID.String()}); err != nil {
			return terminal.HostPage{}, errs.Wrap(errs.CodeInternalError, err, "terminal: list cursor")
		}
	}
	return out, nil
}

// after decodes the keyset position of cursor; "" is the first page. Errors: cursor_invalid.
func (s *Service) after(binding []byte, cursor string) (string, ids.ID, error) {
	if cursor == "" {
		return "", ids.ID{}, nil
	}
	var k hostsCursor
	if err := s.d.Cursors.Decode(hostsList, binding, cursor, &k); err != nil {
		return "", ids.ID{}, err
	}
	id, err := ids.Parse(k.ID)
	if err != nil || id.IsZero() || k.Name == "" {
		return "", ids.ID{}, errs.New(errs.CodeCursorInvalid, "")
	}
	return k.Name, id, nil
}

// readable appends to out the hosts of batch the caller may read (host.read in filter mode), up
// to want in total.
func (s *Service) readable(ctx context.Context, a actor.Actor, batch, out []domain.Host, want int) ([]domain.Host, error) {
	for _, h := range batch {
		if len(out) == want {
			break
		}
		d, err := s.d.Authorizer.Authorize(ctx, a, authorize.ActionHostRead, hostResource(h.ID))
		if err != nil {
			return nil, err
		}
		if d.Allowed() {
			out = append(out, h)
		}
	}
	return out, nil
}

// visible reads candidates of the scope after the keyset position in batches of want and keeps
// those the caller may read, until want are visible or the candidates end.
func (s *Service) visible(ctx context.Context, a actor.Actor, sc authorize.Scope, afterName string, afterID ids.ID, want int) ([]domain.Host, error) {
	var out []domain.Host
	if !sc.AllHosts && len(sc.HostIDs) == 0 {
		return out, nil
	}
	for len(out) < want {
		batch, err := s.d.Store.ListHosts(ctx, sc.AllHosts, sc.HostIDs, afterName, afterID, want)
		if err != nil {
			return nil, err
		}
		if out, err = s.readable(ctx, a, batch, out, want); err != nil {
			return nil, err
		}
		if len(batch) < want {
			break
		}
		last := batch[len(batch)-1]
		afterName, afterID = last.Name, last.ID
	}
	return out, nil
}
