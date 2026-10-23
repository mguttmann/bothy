// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// hints lists every permission hint of q for caller, following the cursor with limit.
func (w *pgWorld) hints(t *testing.T, caller actor.Actor, q authz.HintsQuery) []authz.PermissionHint {
	t.Helper()
	var out []authz.PermissionHint
	for range 1000 {
		page, err := w.admin.ListPermissionHints(ctxOf(caller), caller, q)
		mustOK(t, err)
		out = append(out, page.Items...)
		checkLabels(t, page.Items, page.Labels)
		if page.NextCursor == "" {
			return out
		}
		q.Cursor = page.NextCursor
	}
	t.Fatal("the cursor never ends")
	return nil
}

// checkLabels fails t when a page names an id without its label.
func checkLabels(t *testing.T, items []authz.PermissionHint, l authz.AccessLabels) {
	t.Helper()
	principals := map[ids.ID]bool{}
	for _, p := range l.Principals {
		principals[p.ID] = true
	}
	channels := map[ids.ID]bool{}
	for _, c := range l.Channels {
		channels[c.ID] = true
	}
	for _, h := range items {
		for _, id := range append([]ids.ID{h.AgentID, h.PersonID, h.CredentialHolderID}, h.TriggerSet...) {
			if !id.IsZero() && !principals[id] {
				t.Errorf("hint %+v names %v without a label", h, id)
			}
		}
		if !h.ChannelID.IsZero() && !channels[h.ChannelID] {
			t.Errorf("hint %+v names its channel without a label", h)
		}
	}
}

// codeOf returns the code of a kernel/errs error, or "" for none.
func codeOf(err error) errs.Code {
	if e, ok := errs.As(err); ok {
		return e.Code
	}
	return ""
}

// ofKind returns the hints of kind k.
func ofKind(list []authz.PermissionHint, k domain.HintKind) []authz.PermissionHint {
	var out []authz.PermissionHint
	for _, h := range list {
		if h.Kind == string(k) {
			out = append(out, h)
		}
	}
	return out
}

// owner is the owner in a session.
func (w *pgWorld) ownerActor() actor.Actor { return w.user(w.owner) }

// TestHints_H1 (PLAN row, brief M6-07): Texter operates Marketing and reads Marketing/Kampagnen
// and Marketing/Intern, Sam has trigger on Kampagnen only: the owner's list holds exactly one H1
// (Texter, Sam, Intern); "who has access" on Intern shows the same as via_agent (one function);
// granting Sam read on Intern removes both.
func TestHints_H1(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	ctx := ctxOf(w.ownerActor())
	_, err := w.admin.RevokeGrant(ctx, w.ownerActor(), w.g2, "narrow")
	mustOK(t, err)
	intern := ids.New()
	w.structure.PutChannel(authz.ChannelFacts{ID: intern, SpaceID: w.marketing, Kind: authz.ChannelText})
	w.grantRow(t, domain.Subject{Kind: domain.SubjectAgent, PrincipalID: w.texter}, domain.Target{Kind: domain.TargetChannel, ChannelID: intern}, domain.LevelRead, time.Time{})
	w.grantRow(t, domain.Subject{Kind: domain.SubjectUser, PrincipalID: w.sam}, domain.Target{Kind: domain.TargetChannel, ChannelID: w.kampagnen}, domain.LevelTrigger, time.Time{})
	h1 := ofKind(w.hints(t, w.ownerActor(), authz.HintsQuery{}), domain.HintH1)
	if len(h1) != 1 || h1[0].AgentID != w.texter || h1[0].PersonID != w.sam || h1[0].ChannelID != intern || h1[0].SpaceID != w.marketing {
		t.Fatalf("H1 %+v", h1)
	}
	filtered := w.hints(t, w.ownerActor(), authz.HintsQuery{Kind: string(domain.HintH1)})
	if len(filtered) != 1 || filtered[0].Kind != string(domain.HintH1) {
		t.Fatalf("kind filter %+v", filtered)
	}
	if e := w.explain(t, "channel", intern)[w.sam]; !domain.HasSource(srcs(e), domain.SourceViaAgent) {
		t.Fatalf("explain lost via_agent: %+v", e)
	}
	w.grantRow(t, domain.Subject{Kind: domain.SubjectUser, PrincipalID: w.sam}, domain.Target{Kind: domain.TargetChannel, ChannelID: intern}, domain.LevelRead, time.Time{})
	if h1 := ofKind(w.hints(t, w.ownerActor(), authz.HintsQuery{}), domain.HintH1); len(h1) != 0 {
		t.Fatalf("H1 after the read grant %+v", h1)
	}
	if e, ok := w.explain(t, "channel", intern)[w.sam]; ok && domain.HasSource(srcs(e), domain.SourceViaAgent) {
		t.Fatalf("explain still shows via_agent: %+v", e)
	}
}

// TestHints_OtherKinds (brief M6-07): a grant to a disabled person, a trigger grant blocked by
// SR2 (Robin on Projekt-A, operator Runner of class host) and by SR3 (a channel with a project
// binding, until Sam's dev environment is ready), and the agent Assistent in mode personal with
// its trigger set.
func TestHints_OtherKinds(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	w.sql(t, `UPDATE principals SET status = 'disabled', disabled_at = now() WHERE id = $1`, w.alex.String())
	bound := ids.New()
	w.structure.PutChannel(authz.ChannelFacts{ID: bound, SpaceID: w.support, Kind: authz.ChannelText, EffectiveOperatorID: w.lotse, ProjectBound: true, ProjectReady: true})
	sr3 := w.grantRow(t, domain.Subject{Kind: domain.SubjectUser, PrincipalID: w.sam}, domain.Target{Kind: domain.TargetChannel, ChannelID: bound}, domain.LevelTrigger, time.Time{})
	all := w.hints(t, w.ownerActor(), authz.HintsQuery{})
	disabled := ofKind(all, domain.HintGrantToDisabled)
	if len(disabled) != 1 || disabled[0].Grant == nil || disabled[0].Grant.ID != w.g1 || disabled[0].Grant.SubjectID != w.alex {
		t.Fatalf("grant_to_disabled %+v", disabled)
	}
	blocked := ofKind(all, domain.HintTriggerBlocked)
	if len(blocked) != 2 {
		t.Fatalf("trigger_blocked %+v", blocked)
	}
	byGrant := map[ids.ID]authz.PermissionHint{}
	for _, h := range blocked {
		byGrant[h.Grant.ID] = h
	}
	if h := byGrant[w.g3]; !slices.Equal(h.Rules, []string{domain.RuleSR2}) {
		t.Errorf("SR2 %+v", h)
	}
	if h := byGrant[sr3]; !slices.Equal(h.Rules, []string{domain.RuleSR3}) {
		t.Errorf("SR3 %+v", h)
	}
	personal := ofKind(all, domain.HintAgentPersonal)
	if len(personal) != 1 || personal[0].AgentID != w.assistent || personal[0].CredentialHolderID != w.owner ||
		!slices.Equal(personal[0].TriggerSet, []ids.ID{w.owner}) {
		t.Fatalf("agent_personal %+v", personal)
	}
	kinds := make([]string, 0, len(all))
	for _, h := range all {
		kinds = append(kinds, h.Kind)
	}
	if !slices.IsSortedFunc(kinds, func(a, b string) int {
		return domain.KindRank(domain.HintKind(a)) - domain.KindRank(domain.HintKind(b))
	}) {
		t.Errorf("kinds out of order: %v", kinds)
	}
	w.policies.SetDevReady(w.sam, bound, true)
	for _, h := range ofKind(w.hints(t, w.ownerActor(), authz.HintsQuery{}), domain.HintTriggerBlocked) {
		if h.Grant.ID == sr3 {
			t.Fatalf("a ready dev environment still blocks: %+v", h)
		}
	}
}

// TestHints_Scope (brief M6-07, 07 sections 2.6 and 4.1): Sam as manager of Support sees only
// Support's hints and expiring access, no tokens; Engineering, which Sam cannot read, is 404;
// Robin with read but without manage on Support gets 403; without a space only owners pass.
func TestHints_Scope(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	w.sql(t, `UPDATE principals SET status = 'disabled', disabled_at = now() WHERE id = $1`, w.alex.String())
	w.grantRow(t, domain.Subject{Kind: domain.SubjectUser, PrincipalID: w.sam}, domain.Target{Kind: domain.TargetSpace, SpaceID: w.support}, domain.LevelRead, time.Time{}, domain.CapManage)
	w.grantRow(t, domain.Subject{Kind: domain.SubjectUser, PrincipalID: w.robin}, domain.Target{Kind: domain.TargetSpace, SpaceID: w.support}, domain.LevelRead, time.Time{})
	w.tokenExp.Put(authz.ExpiringToken{ID: ids.New(), PrincipalID: w.lotse, ExpiresAt: pgStart.Add(time.Hour)})
	sam := w.user(w.sam)
	got := w.hints(t, sam, authz.HintsQuery{SpaceID: w.support})
	if len(got) != 1 || got[0].Kind != string(domain.HintGrantToDisabled) || got[0].Grant.ID != w.g1 {
		t.Fatalf("Sam on Support %+v", got)
	}
	owner := w.hints(t, w.ownerActor(), authz.HintsQuery{SpaceID: w.support})
	if len(owner) != 1 || owner[0].Grant.ID != w.g1 {
		t.Fatalf("owner on Support %+v", owner)
	}
	exp := w.expiringList(t, sam, authz.ExpiringQuery{SpaceID: w.support})
	if len(exp) != 1 || exp[0].Kind != "membership" || exp[0].GroupID != w.leser || exp[0].UserID != w.sam {
		t.Fatalf("Sam's expiring on Support %+v", exp)
	}
	for _, c := range []struct {
		who   actor.Actor
		space ids.ID
		code  errs.Code
	}{
		{sam, w.engineering, errs.CodeNotFound},
		{sam, ids.New(), errs.CodeNotFound},
		{w.user(w.robin), w.support, errs.CodeForbidden},
		{sam, ids.ID{}, errs.CodeForbidden},
	} {
		_, err := w.admin.ListPermissionHints(ctxOf(c.who), c.who, authz.HintsQuery{SpaceID: c.space})
		if e, ok := errs.As(err); !ok || e.Code != c.code {
			t.Errorf("hints of %v on %v: %v, want %s", c.who.ID, c.space, err, c.code)
		}
		_, err = w.admin.ListExpiringAccess(ctxOf(c.who), c.who, authz.ExpiringQuery{SpaceID: c.space})
		if e, ok := errs.As(err); !ok || e.Code != c.code {
			t.Errorf("expiring of %v on %v: %v, want %s", c.who.ID, c.space, err, c.code)
		}
	}
	if _, err := w.admin.ListPermissionHints(ctxOf(sam), sam, authz.HintsQuery{Kind: "other"}); codeOf(err) != errs.CodeValidationFailed {
		t.Errorf("unknown kind: %v", err)
	}
}

// expiringList lists "Expiring soon" of q for caller, following the cursor.
func (w *pgWorld) expiringList(t *testing.T, caller actor.Actor, q authz.ExpiringQuery) []authz.ExpiringItem {
	t.Helper()
	var out []authz.ExpiringItem
	for range 1000 {
		page, err := w.admin.ListExpiringAccess(ctxOf(caller), caller, q)
		mustOK(t, err)
		out = append(out, page.Items...)
		if page.NextCursor == "" {
			return out
		}
		q.Cursor = page.NextCursor
	}
	t.Fatal("the cursor never ends")
	return nil
}

// TestExpiring (brief M6-07, 07 section 9.4): with the fake clock at the start, grants,
// memberships and agent and app tokens expiring within authz.expiry_reminder_lead are listed by
// expiry; the boundaries (now and now plus the lead) and revoked grants follow 07 section 2.5;
// the item of a token carries ids and the expiry only.
func TestExpiring(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	user := func(id ids.ID) domain.Subject { return domain.Subject{Kind: domain.SubjectUser, PrincipalID: id} }
	space := domain.Target{Kind: domain.TargetSpace, SpaceID: w.resea}
	soon := w.grantRow(t, user(w.robin), space, domain.LevelRead, pgStart.Add(time.Hour))
	edge := w.grantRow(t, user(w.alex), space, domain.LevelRead, pgStart.Add(reminderLead))
	w.grantRow(t, user(w.sam), space, domain.LevelRead, pgStart.Add(reminderLead+time.Second))
	gone := w.grantRow(t, user(w.sam), domain.Target{Kind: domain.TargetSpace, SpaceID: w.engineering}, domain.LevelRead, pgStart.Add(2*time.Hour))
	_, err := w.admin.RevokeGrant(ctxOf(w.ownerActor()), w.ownerActor(), gone, "done")
	mustOK(t, err)
	agentTok, appTok := ids.New(), ids.New()
	w.tokenExp.Put(authz.ExpiringToken{ID: agentTok, PrincipalID: w.lotse, ExpiresAt: pgStart.Add(30 * time.Minute)})
	w.tokenExp.Put(authz.ExpiringToken{ID: appTok, PrincipalID: w.tracker, ExpiresAt: pgStart.Add(3 * time.Hour)})
	w.tokenExp.Put(authz.ExpiringToken{ID: ids.New(), PrincipalID: w.lotse, ExpiresAt: pgStart.Add(reminderLead + time.Hour)})
	got := w.expiringList(t, w.ownerActor(), authz.ExpiringQuery{})
	var seq []string
	for _, it := range got {
		switch it.Kind {
		case "grant":
			seq = append(seq, "grant:"+it.Grant.ID.String())
		case "membership":
			seq = append(seq, "membership:"+it.UserID.String())
		case "token":
			if it.Grant != nil || !it.MembershipID.IsZero() {
				t.Errorf("a token item with other fields %+v", it)
			}
			seq = append(seq, "token:"+it.TokenID.String())
		}
	}
	// Sam's Leser membership and Alex's grant both end exactly at now plus the lead: grant first.
	want := []string{"token:" + agentTok.String(), "grant:" + soon.String(), "token:" + appTok.String(),
		"grant:" + edge.String(), "membership:" + w.sam.String()}
	if !slices.Equal(seq, want) {
		t.Fatalf("expiring\n got %v\nwant %v", seq, want)
	}
	tokens := got[0]
	if !tokens.ExpiresAt.Equal(pgStart.Add(30*time.Minute)) || tokens.PrincipalID != w.lotse {
		t.Errorf("token %+v", tokens)
	}
	w.clk.Set(pgStart.Add(time.Hour))
	after := w.expiringList(t, w.ownerActor(), authz.ExpiringQuery{})
	for _, it := range after {
		if it.Grant != nil && it.Grant.ID == soon {
			t.Errorf("a grant expiring at now is listed: %+v", it)
		}
	}
}

// TestHints_Paging (brief M6-07 design note 2): every hint of every kind appears on exactly one
// page, in the order of the full list, for every page size; a cursor of another query is refused.
func TestHints_Paging(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	w.sql(t, `UPDATE principals SET status = 'disabled', disabled_at = now() WHERE id = $1`, w.alex.String())
	w.sql(t, `UPDATE principals SET status = 'disabled', disabled_at = now() WHERE id = $1`, w.tracker.String())
	_, err := w.admin.RevokeGrant(ctxOf(w.ownerActor()), w.ownerActor(), w.g2, "narrow")
	mustOK(t, err)
	w.grantRow(t, domain.Subject{Kind: domain.SubjectUser, PrincipalID: w.sam}, domain.Target{Kind: domain.TargetChannel, ChannelID: w.kampagnen}, domain.LevelTrigger, time.Time{})
	for range 3 {
		ch := ids.New()
		w.structure.PutChannel(authz.ChannelFacts{ID: ch, SpaceID: w.marketing, Kind: authz.ChannelText})
		w.grantRow(t, domain.Subject{Kind: domain.SubjectAgent, PrincipalID: w.texter}, domain.Target{Kind: domain.TargetChannel, ChannelID: ch}, domain.LevelRead, time.Time{})
	}
	full := w.hints(t, w.ownerActor(), authz.HintsQuery{PageQuery: authz.PageQuery{Limit: 3}})
	if len(ofKind(full, domain.HintH1)) != 3 || len(ofKind(full, domain.HintGrantToDisabled)) != 2 {
		t.Fatalf("the world %+v", full)
	}
	key := func(list []authz.PermissionHint) []string {
		var out []string
		for _, h := range list {
			g := ""
			if h.Grant != nil {
				g = h.Grant.ID.String()
			}
			out = append(out, fmt.Sprint(h.Kind, h.AgentID, h.PersonID, h.ChannelID, g))
		}
		return out
	}
	for _, limit := range []int{1, 2, 3} {
		got := w.hints(t, w.ownerActor(), authz.HintsQuery{PageQuery: authz.PageQuery{Limit: limit}})
		if !slices.Equal(key(got), key(full)) {
			t.Errorf("limit %d\n got %v\nwant %v", limit, key(got), key(full))
		}
	}
	page, err := w.admin.ListPermissionHints(ctxOf(w.ownerActor()), w.ownerActor(), authz.HintsQuery{PageQuery: authz.PageQuery{Limit: 1}})
	mustOK(t, err)
	_, err = w.admin.ListPermissionHints(ctxOf(w.ownerActor()), w.ownerActor(), authz.HintsQuery{PageQuery: authz.PageQuery{Cursor: page.NextCursor}, Kind: string(domain.HintH1)})
	if codeOf(err) != errs.CodeCursorInvalid {
		t.Errorf("a cursor of another query: %v", err)
	}
	exp := w.expiringList(t, w.ownerActor(), authz.ExpiringQuery{PageQuery: authz.PageQuery{Limit: 1}})
	if len(exp) != 1 {
		t.Errorf("expiring with limit 1 %+v", exp)
	}
}

// TestHints_ScaledWorld (brief M6-07 design note 2) measures the computation for 50 agents and
// 500 people: each agent operates its own channel and reads one more, ten people per agent hold
// trigger on the operated channel. It checks the count of H1 hints (one per person and agent) and
// logs the time of the full list; the time is a measurement, never an assertion.
func TestHints_ScaledWorld(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	space := w.resea
	var people []ids.ID
	for i := range 500 {
		people = append(people, w.person(t, fmt.Sprintf("p%03d", i)))
	}
	for a := range 50 {
		agent := w.bot(t, "agent", fmt.Sprintf("a%02d", a))
		w.policies.Put(agent, authz.AgentPolicy{BillingMode: domain.BillingOrganization, ToolClass: domain.ToolChat})
		op, extra := ids.New(), ids.New()
		w.structure.PutChannel(authz.ChannelFacts{ID: op, SpaceID: space, Kind: authz.ChannelText, EffectiveOperatorID: agent})
		w.structure.PutChannel(authz.ChannelFacts{ID: extra, SpaceID: space, Kind: authz.ChannelText})
		w.sql(t, `INSERT INTO grants (id, subject_kind, subject_principal_id, target_kind, target_channel_id, level, capabilities, reason, granted_by, granted_at)
			VALUES ($1, 'agent', $2, 'channel', $3, 'read', '{}', 'setup', $4, $5)`, ids.New().String(), agent.String(), extra.String(), w.owner.String(), pgStart)
		for _, p := range people[a*10 : a*10+10] {
			w.sql(t, `INSERT INTO grants (id, subject_kind, subject_principal_id, target_kind, target_channel_id, level, capabilities, reason, granted_by, granted_at)
				VALUES ($1, 'user', $2, 'channel', $3, 'trigger', '{}', 'setup', $4, $5)`, ids.New().String(), p.String(), op.String(), w.owner.String(), pgStart)
		}
	}
	cfg := w.adminDeps
	cfg.Config.PageSizeDefault, cfg.Config.PageSizeMax = 1000, 1000
	big, err := app.NewAdminService(cfg)
	mustOK(t, err)
	start := time.Now()
	page, err := big.ListPermissionHints(ctxOf(w.ownerActor()), w.ownerActor(), authz.HintsQuery{Kind: string(domain.HintH1)})
	mustOK(t, err)
	elapsed := time.Since(start)
	if len(page.Items) != 500 || page.NextCursor != "" {
		t.Fatalf("H1 count %d, want 500 on one page", len(page.Items))
	}
	start = time.Now()
	_, err = w.admin.ListPermissionHints(ctxOf(w.ownerActor()), w.ownerActor(), authz.HintsQuery{PageQuery: authz.PageQuery{Limit: 3}})
	mustOK(t, err)
	t.Logf("scaled world (50 agents, 500 people): the whole list of 500 H1 in one request %v, a first page of 3 %v", elapsed, time.Since(start))
}
