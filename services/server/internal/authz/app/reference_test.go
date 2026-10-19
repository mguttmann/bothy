// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"os"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/adapters/fake"
	"github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The slow reference of AZ-P8: decisions computed straight from a model of the world, following
// 07 sections 2.5 and 2.7 to 2.10 and 4.2, without a snapshot, a cache or any type of the
// implementation's rights code. The model is mirrored into the fakes the Authorizer reads.

type refGrant struct {
	id                 ids.ID
	subject            ids.ID // principal or group
	group              bool
	targetKind         string // instance, space, channel, host
	target             ids.ID
	level              int // 1 read, 2 write, 3 trigger
	upload, manage, ex bool
	expires            time.Time
	revoked            bool
}

type refMember struct {
	id, group, user ids.ID
	expires         time.Time
	removed         bool
}

type refPrincipal struct {
	id       ids.ID
	kind     string
	owner    bool
	disabled bool
	scopes   []string
	session  ids.ID
	device   ids.ID
	token    ids.ID
	install  ids.ID
}

type refChannel struct {
	id, space ids.ID
	dm        bool
	members   []ids.ID
	operator  ids.ID
	readOnly  bool
	thread    ids.ID
}

// refHost is a terminal host with its holder (the assignee, zero for none, M2-39) and the facts of
// M2-40: a personal LLM login with its credential holder, the server host flags and the
// assignments of session users (person to whether the host reports the name).
type refHost struct {
	id, holder     ids.ID
	personal       bool
	credHolder     ids.ID
	declaredServer bool
	reportedServer bool
	assigned       map[ids.ID]bool
}

type refWorld struct {
	rnd        *rand.Rand
	hosts      []*refHost
	now        time.Time
	principals []*refPrincipal
	spaces     []ids.ID
	channels   []*refChannel
	groups     map[ids.ID]bool // id -> deleted
	members    []*refMember
	grants     []*refGrant
	expiries   []time.Time

	clk        *clock.Fake
	fPrincipal *fake.Principals
	fStructure *fake.Structure
	fCreds     *fake.Credentials
	fStore     *fake.Store
	fVersions  *fake.Versions
	fHosts     *fake.Hosts
	az         *app.Authorizer
}

func (m *refWorld) inEffect(exp time.Time) bool { return exp.IsZero() || exp.After(m.now) }

// rights is the reference of 05 section 2.3 for p on channel c (nil: space or instance path).
func (m *refWorld) rights(p *refPrincipal, space ids.ID, c *refChannel) (level int, upload, manage, export bool) {
	if c != nil && c.dm {
		if p.kind == domain.KindUser && slices.Contains(c.members, p.id) {
			return 2, true, false, false
		}
		return 0, false, false, false
	}
	if p.owner && p.kind == domain.KindUser && !p.disabled {
		return 3, true, true, true
	}
	groups := map[ids.ID]bool{}
	if p.kind == domain.KindUser {
		for _, mb := range m.members {
			if mb.user == p.id && !mb.removed && !m.groups[mb.group] && m.inEffect(mb.expires) {
				groups[mb.group] = true
			}
		}
	}
	for _, g := range m.grants {
		if g.revoked || !m.inEffect(g.expires) {
			continue
		}
		if g.group && !groups[g.subject] || !g.group && g.subject != p.id {
			continue
		}
		applies := g.targetKind == "instance" || (g.targetKind == "space" && g.target == space) ||
			(g.targetKind == "channel" && c != nil && g.target == c.id)
		if !applies {
			continue
		}
		level = max(level, g.level)
		upload, manage, export = upload || g.upload, manage || g.manage, export || g.ex
	}
	if p.kind == domain.KindAgent && c != nil && c.operator == p.id {
		level = max(level, 2)
	}
	if p.kind != domain.KindUser {
		level, manage, export = min(level, 2), false, false
	}
	return level, upload, manage, export
}

// groupsOf returns the groups of p with a membership in effect.
func (m *refWorld) groupsOf(p *refPrincipal) map[ids.ID]bool {
	groups := map[ids.ID]bool{}
	for _, mb := range m.members {
		if mb.user == p.id && !mb.removed && !m.groups[mb.group] && m.inEffect(mb.expires) {
			groups[mb.group] = true
		}
	}
	return groups
}

// terminalOn is the reference of 07 section 2.12 point 3 for a person p on host h: an active owner,
// or a grant terminal in effect on h to p or a group in effect of p.
func (m *refWorld) terminalOn(p *refPrincipal, h *refHost) bool {
	if p.owner {
		return true
	}
	groups := m.groupsOf(p)
	for _, g := range m.grants {
		if g.targetKind != "host" || g.target != h.id || g.revoked || !m.inEffect(g.expires) {
			continue
		}
		if g.group && groups[g.subject] || !g.group && g.subject == p.id {
			return true
		}
	}
	return false
}

// decideHost is the reference of host.read on h (07 section 2.12 point 5, 05 section 4.14): the
// gate refuses agents and apps, then owners, the holder and a terminal grant see the host.
func (m *refWorld) decideHost(p *refPrincipal, h *refHost) (authorize.Outcome, string) {
	if p.disabled {
		return authorize.OutcomeUnauthenticated, "principal_inactive"
	}
	if p.kind != domain.KindUser {
		return authorize.OutcomeForbidden, "subject_kind_not_allowed"
	}
	if h.holder == p.id || m.terminalOn(p, h) {
		return authorize.OutcomeAllow, ""
	}
	return authorize.OutcomeNotFound, "no_grant"
}

// hostScope is the reference of Scope(host.read) for a person: every host for an owner, else the
// hosts it holds or has terminal on.
func (m *refWorld) hostScope(p *refPrincipal) (all bool, list []ids.ID) {
	if p.owner {
		return true, nil
	}
	for _, h := range m.hosts {
		if h.holder == p.id || m.terminalOn(p, h) {
			list = append(list, h.id)
		}
	}
	return false, domain.SortIDs(list)
}

// syncHost mirrors a host into the fake source.
func (m *refWorld) syncHost(h *refHost) {
	f := authz.HostFacts{ID: h.id, State: "active", AssignedUserID: h.holder, LLMCredentials: "organization",
		DeclaredServerHost: h.declaredServer, ReportedIsServerHost: h.reportedServer}
	if h.personal {
		f.LLMCredentials, f.CredentialHolderID = "personal", h.credHolder
	}
	m.fHosts.Put(f)
	for _, p := range m.principals {
		if reported, ok := h.assigned[p.id]; ok {
			m.fHosts.PutSessionUser(p.id, authz.SessionUserFact{HostID: h.id, HostSessionUserID: ids.New(), OSUser: "bothy-s-x", Reported: reported})
		} else {
			m.fHosts.RemoveSessionUser(p.id, h.id)
		}
	}
}

// decideOpen is the reference of terminal.open on h with a fresh step-up (07 section 4.2, SR10
// to SR12; M2-40): the gate, the visibility of the host, the right terminal, then SR10 server
// host, SR10 assignee, SR11 and SR12.
func (m *refWorld) decideOpen(p *refPrincipal, h *refHost) (authorize.Outcome, string) {
	if o, r := m.decideHost(p, h); o != authorize.OutcomeAllow {
		return o, r
	}
	if !m.terminalOn(p, h) {
		return authorize.OutcomeForbidden, "capability_missing"
	}
	switch reported, assigned := h.assigned[p.id]; {
	case !p.owner && (h.declaredServer || h.reportedServer):
		return authorize.OutcomeForbidden, "terminal_server_host_owner_only"
	case !p.owner && h.holder != p.id:
		return authorize.OutcomeForbidden, "terminal_not_host_assignee"
	case h.personal && h.credHolder != p.id:
		return authorize.OutcomeForbidden, "i6_holder_only"
	case !assigned || !reported:
		return authorize.OutcomeForbidden, "terminal_no_session_user"
	}
	return authorize.OutcomeAllow, ""
}

// decideAttach is the reference of terminal.attach of a session of owner on h with a fresh
// step-up: the gate, the session visible to its person and to effective owners (not_self), the
// requirement Self and terminal on the host, then the guards of terminal.open.
func (m *refWorld) decideAttach(p *refPrincipal, h *refHost, owner ids.ID) (authorize.Outcome, string) {
	if p.disabled {
		return authorize.OutcomeUnauthenticated, "principal_inactive"
	}
	if p.kind != domain.KindUser {
		return authorize.OutcomeForbidden, "subject_kind_not_allowed"
	}
	if owner != p.id && !p.owner {
		return authorize.OutcomeNotFound, "not_self"
	}
	if owner != p.id {
		return authorize.OutcomeForbidden, "not_self"
	}
	if !m.terminalOn(p, h) {
		return authorize.OutcomeForbidden, "capability_missing"
	}
	return m.decideOpen(p, h)
}

// mutateTerminal changes a terminal fact of a random host (M2-40): the declaration and the
// assignments are rights state and raise the version; the report of the server host does not.
func (m *refWorld) mutateTerminal(kind int) string {
	h := m.hosts[m.rnd.IntN(len(m.hosts))]
	switch kind {
	case 0:
		h.personal = !h.personal
		h.credHolder = ids.ID{}
		if h.personal {
			if h.credHolder = m.randomHolder(); h.credHolder.IsZero() {
				h.credHolder = m.principals[0].id
			}
		}
		m.syncHost(h)
		m.fVersions.Bump()
		return "toggle declaration"
	case 1:
		h.declaredServer = !h.declaredServer
		m.syncHost(h)
		m.fVersions.Bump()
		return "toggle declared server host"
	case 2:
		h.reportedServer = !h.reportedServer
		m.syncHost(h)
		return "toggle reported server host"
	}
	p := m.principals[m.rnd.IntN(4)]
	if _, ok := h.assigned[p.id]; ok && m.rnd.IntN(2) == 0 {
		delete(h.assigned, p.id)
	} else {
		h.assigned[p.id] = m.rnd.IntN(3) != 0
	}
	m.syncHost(h)
	m.fVersions.Bump()
	return "change session user"
}

// refAppScope is the scope table of 05 section 2.6 for the actions of the test.
var refAppScope = map[authorize.Action]string{
	authorize.ActionMessageRead: "messages:read", authorize.ActionThreadRead: "messages:read",
	authorize.ActionChannelRead: "messages:read", authorize.ActionSpaceRead: "messages:read",
	authorize.ActionMessageCreate: "messages:write", authorize.ActionFileUpload: "files:write",
}

// decide is the reference decision for p, a and the channel c (space for space.read).
func (m *refWorld) decide(p *refPrincipal, a authorize.Action, c *refChannel, space ids.ID) (authorize.Outcome, string) {
	if p.disabled {
		return authorize.OutcomeUnauthenticated, "principal_inactive"
	}
	if a == authorize.ActionSpaceRead {
		lvl, _, _, _ := m.rights(p, space, nil)
		visible := lvl >= 1
		for _, ch := range m.channels {
			if ch.space == space && !ch.dm {
				if l, _, _, _ := m.rights(p, space, ch); l >= 1 {
					visible = true
				}
			}
		}
		if !visible {
			return authorize.OutcomeNotFound, "no_grant"
		}
		return m.scope(p, a)
	}
	lvl, upload, manage, export := m.rights(p, c.space, c)
	if lvl < 1 {
		return authorize.OutcomeNotFound, "no_grant"
	}
	needWrite := a == authorize.ActionMessageCreate || a == authorize.ActionFileUpload
	if needWrite && lvl < 2 {
		return authorize.OutcomeForbidden, "level_insufficient"
	}
	if (a == authorize.ActionFileUpload && !upload) || (a == authorize.ActionChannelUpdate && !manage) ||
		(a == authorize.ActionExportChannel && !export) {
		return authorize.OutcomeForbidden, "capability_missing"
	}
	if o, r := m.scope(p, a); o != authorize.OutcomeAllow {
		return o, r
	}
	if needWrite && c.readOnly {
		return authorize.OutcomeForbidden, "channel_read_only"
	}
	if a == authorize.ActionExportChannel && p.kind != domain.KindUser {
		return authorize.OutcomeForbidden, "step_up_not_possible"
	}
	return authorize.OutcomeAllow, ""
}

// scope applies the app scope rule.
func (m *refWorld) scope(p *refPrincipal, a authorize.Action) (authorize.Outcome, string) {
	if p.kind != domain.KindApp {
		return authorize.OutcomeAllow, ""
	}
	need, ok := refAppScope[a]
	if !ok {
		return authorize.OutcomeForbidden, "subject_kind_not_allowed"
	}
	if !slices.Contains(p.scopes, need) {
		return authorize.OutcomeForbidden, "scope_missing"
	}
	return authorize.OutcomeAllow, ""
}

// visibleChannels is the reference of Scope(message.read).
func (m *refWorld) visibleChannels(p *refPrincipal) []ids.ID {
	if p.kind == domain.KindApp && !slices.Contains(p.scopes, "messages:read") {
		return nil
	}
	var out []ids.ID
	for _, c := range m.channels {
		if l, _, _, _ := m.rights(p, c.space, c); l >= 1 {
			out = append(out, c.id)
		}
	}
	return domain.SortIDs(out)
}

// sync mirrors the model into the fakes.
func (m *refWorld) syncPrincipal(p *refPrincipal) {
	st := authz.PrincipalActive
	if p.disabled {
		st = authz.PrincipalDisabled
	}
	m.fPrincipal.Put(authz.PrincipalFacts{ID: p.id, Kind: actor.Kind(p.kind), Status: st, IsOwner: p.owner, AppInstallationActive: !p.install.IsZero()})
}

func (m *refWorld) syncChannel(c *refChannel) {
	f := authz.ChannelFacts{ID: c.id, SpaceID: c.space, Kind: authz.ChannelText, EffectiveOperatorID: c.operator, ReadOnly: c.readOnly}
	if c.dm {
		f.Kind, f.SpaceID, f.DMParticipants, f.EffectiveOperatorID = authz.ChannelDM, ids.ID{}, c.members, ids.ID{}
	}
	m.fStructure.PutChannel(f)
}

func (m *refWorld) actorOf(p *refPrincipal) actor.Actor {
	a := actor.Actor{ID: p.id, Kind: actor.Kind(p.kind), SessionID: p.session, DeviceID: p.device, TokenID: p.token,
		InstallationID: p.install, GrantedScopes: p.scopes, Scopes: p.scopes, StepUpAt: m.now}
	if p.kind == domain.KindUser {
		a.StepUpMethod, a.ClientComponent = "device_key", "app"
	}
	return a
}

func (m *refWorld) expiry() time.Time {
	if m.rnd.IntN(3) == 0 {
		return time.Time{}
	}
	e := m.now.Add(time.Duration(1+m.rnd.IntN(120)) * time.Minute)
	m.expiries = append(m.expiries, e)
	return e
}

func (m *refWorld) pick() *refPrincipal { return m.principals[m.rnd.IntN(len(m.principals))] }

func (m *refWorld) addChannel(space ids.ID) {
	c := &refChannel{id: ids.New(), space: space, thread: ids.New()}
	if m.rnd.IntN(3) == 0 {
		for _, p := range m.principals {
			if p.kind == domain.KindAgent {
				c.operator = p.id
				break
			}
		}
	}
	m.channels = append(m.channels, c)
	m.syncChannel(c)
	m.fStructure.PutThread(authz.ThreadFacts{ID: c.thread, ChannelID: c.id, Main: true})
}

func (m *refWorld) addDM() {
	var users []ids.ID
	for _, p := range m.principals {
		if p.kind == domain.KindUser && m.rnd.IntN(2) == 0 {
			users = append(users, p.id)
		}
	}
	c := &refChannel{id: ids.New(), dm: true, members: domain.SortIDs(users), thread: ids.New()}
	m.channels = append(m.channels, c)
	m.syncChannel(c)
	m.fStructure.PutThread(authz.ThreadFacts{ID: c.thread, ChannelID: c.id, Main: true})
}

func (m *refWorld) addGrant() {
	g := &refGrant{id: ids.New(), level: 1 + m.rnd.IntN(3), expires: m.expiry(),
		upload: m.rnd.IntN(2) == 0, manage: m.rnd.IntN(3) == 0, ex: m.rnd.IntN(3) == 0}
	groups := slices.Collect(func(yield func(ids.ID) bool) {
		for id := range m.groups {
			if !yield(id) {
				return
			}
		}
	})
	if len(groups) > 0 && m.rnd.IntN(3) == 0 {
		g.group, g.subject = true, groups[m.rnd.IntN(len(groups))]
	} else {
		g.subject = m.pick().id
	}
	if m.rnd.IntN(5) == 0 && (g.group || m.kindOf(g.subject) == domain.KindUser) {
		m.addHostGrant(g)
		return
	}
	switch m.rnd.IntN(4) {
	case 0:
		g.targetKind = "instance"
	case 1:
		g.targetKind, g.target = "space", m.spaces[m.rnd.IntN(len(m.spaces))]
	default:
		var ch []*refChannel
		for _, c := range m.channels {
			if !c.dm {
				ch = append(ch, c)
			}
		}
		g.targetKind, g.target = "channel", ch[m.rnd.IntN(len(ch))].id
	}
	m.grants = append(m.grants, g)
	sub := domain.Subject{Kind: domain.SubjectUser, PrincipalID: g.subject}
	if g.group {
		sub = domain.Subject{Kind: domain.SubjectGroup, GroupID: g.subject}
	}
	tg := domain.Target{Kind: domain.TargetKind(g.targetKind)}
	switch g.targetKind {
	case "space":
		tg.SpaceID = g.target
	case "channel":
		tg.ChannelID = g.target
	}
	var caps []domain.Capability
	if g.upload {
		caps = append(caps, domain.CapUpload)
	}
	if g.manage {
		caps = append(caps, domain.CapManage)
	}
	if g.ex {
		caps = append(caps, domain.CapExport)
	}
	_ = m.fStore.InsertGrant(context.Background(), domain.Grant{ID: g.id, Subject: sub, Target: tg, Level: domain.Level(g.level), //nolint:gosec // G115: g.level is 1 to 3
		Caps: domain.CapsOf(caps...), ExpiresAt: g.expires, GrantedAt: m.now})
}

// kindOf returns the kind of principal id; "" for a group.
func (m *refWorld) kindOf(id ids.ID) string {
	for _, p := range m.principals {
		if p.id == id {
			return p.kind
		}
	}
	return ""
}

// addHostGrant turns g into a grant terminal on a random host (people and groups only, 07 section
// 2.12) and stores it.
func (m *refWorld) addHostGrant(g *refGrant) {
	h := m.hosts[m.rnd.IntN(len(m.hosts))]
	g.targetKind, g.target, g.level, g.upload, g.manage, g.ex = "host", h.id, 0, false, false, false
	m.grants = append(m.grants, g)
	sub := domain.Subject{Kind: domain.SubjectUser, PrincipalID: g.subject}
	if g.group {
		sub = domain.Subject{Kind: domain.SubjectGroup, GroupID: g.subject}
	}
	_ = m.fStore.InsertGrant(context.Background(), domain.Grant{ID: g.id, Subject: sub, Target: domain.Target{Kind: domain.TargetHost, HostID: h.id},
		Caps: domain.CapsOf(domain.CapTerminal), ExpiresAt: g.expires, GrantedAt: m.now})
}

// randomHolder returns a person of the world or zero (no holder).
func (m *refWorld) randomHolder() ids.ID {
	var people []ids.ID
	for _, p := range m.principals {
		if p.kind == domain.KindUser {
			people = append(people, p.id)
		}
	}
	if m.rnd.IntN(3) == 0 {
		return ids.ID{}
	}
	return people[m.rnd.IntN(len(people))]
}

func newRefWorld(t *testing.T, seed uint64) *refWorld {
	t.Helper()
	rnd := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)) //nolint:gosec // G404: a reproducible test world needs a seeded generator
	m := &refWorld{rnd: rnd, now: worldStart, groups: map[ids.ID]bool{},
		clk: clock.NewFake(worldStart), fPrincipal: fake.NewPrincipals(), fStructure: fake.NewStructure(),
		fCreds: fake.NewCredentials(), fStore: fake.NewStore(), fVersions: &fake.Versions{}, fHosts: fake.NewHosts()}
	m.fVersions.Set(1)
	kinds := []string{domain.KindUser, domain.KindUser, domain.KindUser, domain.KindUser, domain.KindAgent, domain.KindAgent, domain.KindApp}
	for i, k := range kinds {
		p := &refPrincipal{id: ids.New(), kind: k, owner: k == domain.KindUser && i == 0}
		switch k {
		case domain.KindUser:
			p.session, p.device = ids.New(), ids.New()
			m.fCreds.PutDevice(p.device, fake.Device{PrincipalID: p.id, State: authz.DeviceActive})
			m.fCreds.PutSession(p.session, fake.Session{PrincipalID: p.id, DeviceID: p.device, ExpiresAt: worldStart.Add(10 * 365 * 24 * time.Hour)})
		default:
			p.token = ids.New()
			m.fCreds.PutToken(p.token, fake.Token{PrincipalID: p.id})
		}
		if k == domain.KindApp {
			p.install = ids.New()
			for _, s := range []string{"messages:read", "messages:write", "files:write"} {
				if m.rnd.IntN(2) == 0 {
					p.scopes = append(p.scopes, s)
				}
			}
		}
		m.principals = append(m.principals, p)
		m.syncPrincipal(p)
	}
	for range 3 {
		s := ids.New()
		m.spaces = append(m.spaces, s)
		m.fStructure.PutSpace(s)
		for range 2 {
			m.addChannel(s)
		}
	}
	m.addDM()
	for range 3 {
		h := &refHost{id: ids.New(), holder: m.randomHolder(), assigned: map[ids.ID]bool{}}
		for _, p := range m.principals[:4] {
			if m.rnd.IntN(2) == 0 {
				h.assigned[p.id] = m.rnd.IntN(4) != 0
			}
		}
		m.hosts = append(m.hosts, h)
		m.syncHost(h)
	}
	for range 2 {
		g := ids.New()
		m.groups[g] = false
		_ = m.fStore.InsertGroup(context.Background(), domain.Group{ID: g, Name: "g" + g.String()[:8], CreatedAt: m.now})
		for _, p := range m.principals {
			if p.kind == domain.KindUser && m.rnd.IntN(2) == 0 {
				mb := &refMember{id: ids.New(), group: g, user: p.id, expires: m.expiry()}
				m.members = append(m.members, mb)
				_ = m.fStore.InsertMembership(context.Background(), domain.Membership{ID: mb.id, GroupID: g, UserID: p.id, AddedAt: m.now, ExpiresAt: mb.expires})
			}
		}
	}
	for range 8 {
		m.addGrant()
	}
	az, err := app.NewAuthorizer(app.AuthorizerConfig{SnapshotCacheMaxEntries: 4, StepUpMaxAge: 5 * time.Minute, TerminalStepUpMaxAge: time.Minute,
		QuickReattachWindow: time.Minute}, app.AuthorizerDeps{
		Catalog: app.NewCatalog(slog.New(slog.DiscardHandler)), Store: m.fStore, Principals: m.fPrincipal, Credentials: m.fCreds,
		Structure: m.fStructure, Policies: fake.NewPolicies(), Hosts: m.fHosts, Versions: m.fVersions, Recorder: &recorder{}, Clock: m.clk,
		Log: slog.New(slog.DiscardHandler),
	})
	if err != nil {
		t.Fatal(err)
	}
	m.az = az
	return m
}

// mutate applies one random change; rights changes raise the version, structure changes do not.
func (m *refWorld) mutate() string {
	ctx := context.Background()
	if k := m.rnd.IntN(16); k >= 12 {
		return m.mutateTerminal(k - 12)
	}
	switch m.rnd.IntN(12) {
	case 0:
		m.addGrant()
		m.fVersions.Bump()
		return "add grant"
	case 1:
		if len(m.grants) > 0 {
			g := m.grants[m.rnd.IntN(len(m.grants))]
			if !g.revoked {
				g.revoked = true
				_, _, _ = m.fStore.RevokeGrant(ctx, g.id, ids.ID{}, "", m.now)
				m.fVersions.Bump()
			}
		}
		return "revoke grant"
	case 2:
		if len(m.members) > 0 {
			mb := m.members[m.rnd.IntN(len(m.members))]
			if !mb.removed {
				mb.removed = true
				_, _ = m.fStore.RemoveMembership(ctx, mb.id, ids.ID{}, m.now)
				m.fVersions.Bump()
			}
		}
		return "remove membership"
	case 3:
		p := m.pick()
		p.disabled = !p.disabled
		m.syncPrincipal(p)
		m.fVersions.Bump()
		return "toggle disabled"
	case 4:
		p := m.principals[m.rnd.IntN(4)]
		p.owner = !p.owner
		m.syncPrincipal(p)
		m.fVersions.Bump()
		return "toggle owner"
	case 5:
		m.addChannel(m.spaces[m.rnd.IntN(len(m.spaces))])
		return "create channel"
	case 6:
		s := ids.New()
		m.spaces = append(m.spaces, s)
		m.fStructure.PutSpace(s)
		return "create space"
	case 7:
		m.addDM()
		return "create dm"
	case 8:
		if len(m.expiries) > 0 && m.rnd.IntN(2) == 0 {
			e := m.expiries[m.rnd.IntN(len(m.expiries))]
			if e.After(m.now) {
				m.now = e
				m.clk.Set(e)
				return "clock to an expiry"
			}
		}
		m.now = m.now.Add(time.Duration(m.rnd.IntN(40)) * time.Minute)
		m.clk.Set(m.now)
		return "clock advance"
	case 11:
		// A holder change is no rights change (07 section 2.12 point 3): the source is read fresh.
		h := m.hosts[m.rnd.IntN(len(m.hosts))]
		h.holder = m.randomHolder()
		m.syncHost(h)
		return "change holder"
	case 9:
		c := m.channels[m.rnd.IntN(len(m.channels))]
		if !c.dm {
			c.readOnly = !c.readOnly
			m.syncChannel(c)
		}
		return "toggle read-only"
	default:
		c := m.channels[m.rnd.IntN(len(m.channels))]
		if !c.dm {
			c.operator = m.pick().id
			m.syncChannel(c)
		}
		return "change operator"
	}
}

// resourceFor is the resource of act on channel c (a random space for space.read).
func (m *refWorld) resourceFor(act authorize.Action, c *refChannel) authorize.Resource {
	byAction := map[authorize.Action]authorize.Resource{
		authorize.ActionMessageRead:   {Kind: authorize.ResourceMessage, ID: ids.New(), ThreadID: c.thread},
		authorize.ActionMessageCreate: {Kind: authorize.ResourceThread, ID: c.thread},
		authorize.ActionThreadRead:    {Kind: authorize.ResourceThread, ID: c.thread},
		authorize.ActionSpaceRead:     {Kind: authorize.ResourceSpace, ID: m.spaces[m.rnd.IntN(len(m.spaces))]},
	}
	if r, ok := byAction[act]; ok {
		return r
	}
	return authorize.Resource{Kind: authorize.ResourceChannel, ID: c.id}
}

// TestAZP8_CacheMatchesReference (AZ-P8) compares the cached decisions with the reference; since
// M2-40 it also covers terminal.open and terminal.attach over random declarations, server host
// flags, assignments and reports (the test TestAZP8_TerminalMatchesReference of the brief M2-40).
func TestAZP8_CacheMatchesReference(t *testing.T) {
	seed := uint64(time.Now().UnixNano())
	if s := os.Getenv("BOTHY_TEST_SEED"); s != "" { //nolint:forbidigo // test input (seed), not server configuration
		n, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		seed = n
	}
	actions := []authorize.Action{authorize.ActionMessageRead, authorize.ActionMessageCreate, authorize.ActionThreadRead,
		authorize.ActionChannelRead, authorize.ActionSpaceRead, authorize.ActionChannelUpdate, authorize.ActionExportChannel,
		authorize.ActionFileUpload}
	outcomes := map[string]int{}
	for w := range 20 {
		ws := seed + uint64(w)
		m := newRefWorld(t, ws)
		var trail []string
		for step := range 60 {
			trail = append(trail, m.mutate())
			for _, p := range m.principals {
				a := m.actorOf(p)
				for _, act := range actions {
					c := m.channels[m.rnd.IntN(len(m.channels))]
					r := m.resourceFor(act, c)
					wantO, wantR := m.decide(p, act, c, r.ID)
					outcomes[string(wantO)+"/"+wantR]++
					d, err := m.az.Authorize(context.Background(), a, act, r)
					if err != nil || d.Outcome != wantO || string(d.Reason) != wantR {
						t.Fatalf("seed %d (BOTHY_TEST_SEED=%d), world %d, step %d, %s %s: got %s/%s %v, want %s/%s\ntrail %v",
							seed, ws, w, step, p.kind, act, d.Outcome, d.Reason, err, wantO, wantR, trail)
					}
				}
				h := m.hosts[m.rnd.IntN(len(m.hosts))]
				wantO, wantR := m.decideHost(p, h)
				outcomes["host.read "+string(wantO)+"/"+wantR]++
				d, err := m.az.Authorize(context.Background(), a, authorize.ActionHostRead, authorize.Resource{Kind: authorize.ResourceHost, ID: h.id})
				if err != nil || d.Outcome != wantO || string(d.Reason) != wantR {
					t.Fatalf("seed %d, world %d, step %d, %s host.read: got %s/%s %v, want %s/%s\ntrail %v",
						ws, w, step, p.kind, d.Outcome, d.Reason, err, wantO, wantR, trail)
				}
				wantO, wantR = m.decideOpen(p, h)
				outcomes["terminal.open "+string(wantO)+"/"+wantR]++
				d, err = m.az.Authorize(context.Background(), a, authorize.ActionTerminalOpen, authorize.Resource{Kind: authorize.ResourceHost, ID: h.id})
				if err != nil || d.Outcome != wantO || string(d.Reason) != wantR {
					t.Fatalf("seed %d, world %d, step %d, %s terminal.open: got %s/%s %v, want %s/%s\ntrail %v",
						ws, w, step, p.kind, d.Outcome, d.Reason, err, wantO, wantR, trail)
				}
				owner := m.principals[m.rnd.IntN(4)]
				wantO, wantR = m.decideAttach(p, h, owner.id)
				outcomes["terminal.attach "+string(wantO)+"/"+wantR]++
				ses := authorize.Resource{Kind: authorize.ResourceTerminalSession, ID: ids.New(), HostID: h.id, OwnerID: owner.id,
					Terminal: &authorize.TerminalFacts{SessionState: "active", SessionDeviceID: owner.device, SessionAuthSessionID: owner.session}}
				d, err = m.az.Authorize(context.Background(), a, authorize.ActionTerminalAttach, ses)
				if err != nil || d.Outcome != wantO || string(d.Reason) != wantR {
					t.Fatalf("seed %d, world %d, step %d, %s terminal.attach: got %s/%s %v, want %s/%s\ntrail %v",
						ws, w, step, p.kind, d.Outcome, d.Reason, err, wantO, wantR, trail)
				}
				if p.disabled {
					continue
				}
				if p.kind == domain.KindUser {
					all, list := m.hostScope(p)
					hs, err := m.az.Scope(context.Background(), a, authorize.ActionHostRead)
					if err != nil || hs.AllHosts != all || !slices.Equal(hs.HostIDs, list) {
						t.Fatalf("seed %d, world %d, step %d, host scope: got %v %v %v, want %v %v\ntrail %v", ws, w, step, hs.AllHosts, hs.HostIDs, err, all, list, trail)
					}
				}
				sc, err := m.az.Scope(context.Background(), a, authorize.ActionMessageRead)
				if want := m.visibleChannels(p); err != nil || !slices.Equal(sc.ChannelIDs, want) {
					t.Fatalf("seed %d, world %d, step %d, %s scope: got %v %v, want %v\ntrail %v", ws, w, step, p.kind, sc.ChannelIDs, err, want, trail)
				}
			}
		}
	}
	t.Logf("seed %d, decisions by outcome %v", seed, outcomes)
}
