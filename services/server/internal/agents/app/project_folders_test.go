// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/agents/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

func (s *fakeStore) WriteProjectFolders(_ context.Context, id ids.ID, names []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.folders == nil {
		s.folders = map[ids.ID][]string{}
	}
	s.folders[id] = slices.Clone(names)
	s.folderWrites = append(s.folderWrites, slices.Clone(names))
	return nil
}

func (s *fakeStore) GetProjectFolders(_ context.Context, id ids.ID) ([]string, bool, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.rows[id]; !ok {
		return nil, false, false, nil
	}
	names, reported := s.folders[id]
	return slices.Clone(names), reported, true, nil
}

func (s *fakeStore) writes() [][]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.folderWrites)
}

func (r *fakeReadiness) BoundProjectFolders(_ context.Context, names []string) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for _, n := range names {
		if slices.ContainsFunc(r.bound, func(b string) bool { return strings.EqualFold(b, n) }) {
			out = append(out, n)
		}
	}
	return out, nil
}

// ownerAuthorizer allows agent.project_folders.read for owner and agent.heartbeat for an agent on
// itself; it records the actions it decided.
type ownerAuthorizer struct {
	allowAgents
	owner   ids.ID
	actions []authorize.Action
}

func (o *ownerAuthorizer) Authorize(ctx context.Context, p authorize.Principal, a authorize.Action, r authorize.Resource) (authorize.Decision, error) {
	o.actions = append(o.actions, a)
	if a == authorize.ActionAgentProjectFoldersRead {
		if pa, ok := p.(actor.Actor); ok && pa.ID == o.owner && r.Kind == "agent" {
			return authorize.Decision{Outcome: authorize.OutcomeAllow}, nil
		}
		return authorize.Decision{Outcome: authorize.OutcomeForbidden, Reason: "owner_only"}, nil
	}
	return o.allowAgents.Authorize(ctx, p, a, r)
}

// TestHeartbeat_ProjectFoldersWrittenOnChange: a list is written at once the first time and when
// it changes; the same list again and an absent member write nothing (09 section 4.8, M4-44).
func TestHeartbeat_ProjectFoldersWrittenOnChange(t *testing.T) {
	t.Parallel()
	w := newTracker(t)
	w.beat(`{"state":"online","project_folders":["docs","website"]}`)
	w.beat(`{"state":"online","project_folders":["docs","website"]}`)
	w.beat(`{"state":"online"}`)
	if got := w.store.writes(); len(got) != 1 || !slices.Equal(got[0], []string{"docs", "website"}) {
		t.Fatalf("writes %q, want the first list once", got)
	}
	w.beat(`{"state":"online","project_folders":["docs"]}`)
	w.beat(`{"state":"online","project_folders":[]}`)
	got := w.store.writes()
	if len(got) != 3 || !slices.Equal(got[1], []string{"docs"}) || got[2] == nil || len(got[2]) != 0 {
		t.Fatalf("writes %q, want the changed and the empty list", got)
	}
}

// codeOf returns the problem code of err, "" for none.
func codeOf(err error) errs.Code {
	if e, ok := errs.As(err); ok {
		return e.Code
	}
	return ""
}

// useAuthorizer replaces the tracker of w by one over the same fakes with a.
func (w *tracker) useAuthorizer(t *testing.T, a authorize.Authorizer) {
	t.Helper()
	tr, err := app.NewTracker(app.PresenceConfig{PresenceTimeout: tTimeout, PersistInterval: tPersist, MismatchAlertAfter: tAlert,
		HeartbeatInterval: tInterval}, app.PresenceDeps{Store: w.store, Principals: w.pr, UoW: runUoW{}, Authorizer: a,
		Audit: w.rec, Publisher: w.pub, Readiness: w.rd, Clock: w.clk, Timers: w.tm})
	if err != nil {
		t.Fatal(err)
	}
	w.tr = tr
}

// listFolders is the tracker with an owner who reads the folders of the agent.
func listFolders(t *testing.T, w *tracker, who actor.Actor, agent ids.ID) (agents.ProjectFolders, error) {
	t.Helper()
	return w.tr.ProjectFolders(context.Background(), who, agent)
}

// TestProjectFolders_LeavesOutBoundNames: the owner gets the stored names without those a
// binding names, compared without case, in the stored order; reported is true (R-DM-56).
func TestProjectFolders_LeavesOutBoundNames(t *testing.T) {
	t.Parallel()
	w := newTracker(t)
	owner := actor.Actor{ID: ids.New(), Kind: actor.KindUser}
	auth := &ownerAuthorizer{owner: owner.ID}
	w.useAuthorizer(t, auth)
	w.rd.bound = []string{"website", "Other"}
	w.beat(`{"state":"online","project_folders":["api","docs","Website"]}`)
	got, err := listFolders(t, w, owner, w.agent.ID)
	if err != nil || !got.Reported || !slices.Equal(got.Names, []string{"api", "docs"}) {
		t.Fatalf("folders %+v err %v", got, err)
	}
	if !slices.Contains(auth.actions, authorize.ActionAgentProjectFoldersRead) {
		t.Fatalf("actions %v, want agent.project_folders.read", auth.actions)
	}
}

// TestProjectFolders_NotReported: before the first report the list is empty and reported false.
func TestProjectFolders_NotReported(t *testing.T) {
	t.Parallel()
	w := newTracker(t)
	owner := actor.Actor{ID: ids.New(), Kind: actor.KindUser}
	w.useAuthorizer(t, &ownerAuthorizer{owner: owner.ID})
	got, err := listFolders(t, w, owner, w.agent.ID)
	if err != nil || got.Reported || got.Names == nil || len(got.Names) != 0 {
		t.Fatalf("folders %+v err %v", got, err)
	}
}

// TestProjectFolders_OwnerOnlyAndUnknown: a person who is not the owner gets the decision of
// authorize; an unknown agent is not_found.
func TestProjectFolders_OwnerOnlyAndUnknown(t *testing.T) {
	t.Parallel()
	w := newTracker(t)
	owner := actor.Actor{ID: ids.New(), Kind: actor.KindUser}
	w.useAuthorizer(t, &ownerAuthorizer{owner: owner.ID})
	w.beat(`{"state":"online","project_folders":["docs"]}`)
	other := actor.Actor{ID: ids.New(), Kind: actor.KindUser}
	if _, err := listFolders(t, w, other, w.agent.ID); codeOf(err) != errs.CodeForbidden {
		t.Fatalf("other person: %v, want forbidden", err)
	}
	if _, err := listFolders(t, w, owner, ids.New()); codeOf(err) != errs.CodeNotFound {
		t.Fatalf("unknown agent: %v, want not_found", err)
	}
}
