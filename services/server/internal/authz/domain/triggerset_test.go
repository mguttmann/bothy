// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"math/rand/v2"
	"os"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The world of the TriggerSet tests: one personal agent that serves channel served (in space S1)
// and, by its default operator, the space S2 without a channel; channel other in space S3 is not
// served.
var (
	tsOwner   = ids.MustParse("019a0000-0000-7000-8000-000000000001")
	tsAlex    = ids.MustParse("019a0000-0000-7000-8000-000000000002")
	tsSam     = ids.MustParse("019a0000-0000-7000-8000-000000000003")
	tsRobin   = ids.MustParse("019a0000-0000-7000-8000-000000000004")
	tsAgent   = ids.MustParse("019a0000-0000-7000-8000-0000000000a1")
	tsOther   = ids.MustParse("019a0000-0000-7000-8000-0000000000a2")
	tsApp     = ids.MustParse("019a0000-0000-7000-8000-0000000000b1")
	tsS1      = ids.MustParse("019a0000-0000-7000-8000-000000000101")
	tsS2      = ids.MustParse("019a0000-0000-7000-8000-000000000102")
	tsS3      = ids.MustParse("019a0000-0000-7000-8000-000000000103")
	tsServed  = ids.MustParse("019a0000-0000-7000-8000-000000000201")
	tsUnrel   = ids.MustParse("019a0000-0000-7000-8000-000000000202")
	tsGroup   = ids.MustParse("019a0000-0000-7000-8000-000000000301")
	tsPersAge = domain.PersonalAgent{AgentID: tsAgent, CredentialHolderID: tsOwner}
)

// tsWorld returns the base world: the owner, three active people, an agent and an app, and the
// served targets of tsAgent.
func tsWorld() domain.World {
	return domain.World{
		Principals: map[ids.ID]domain.PrincipalState{
			tsOwner: {Kind: domain.KindUser, Active: true}, tsAlex: {Kind: domain.KindUser, Active: true},
			tsSam: {Kind: domain.KindUser, Active: true}, tsRobin: {Kind: domain.KindUser, Active: true},
			tsOther: {Kind: domain.KindAgent, Active: true}, tsApp: {Kind: domain.KindApp, Active: true},
		},
		Owners: []ids.ID{tsOwner},
		Served: map[ids.ID][]domain.ServedTarget{tsAgent: {{ChannelID: tsServed, SpaceID: tsS1}, {SpaceID: tsS2}}},
	}
}

func userGrant(p ids.ID, tg domain.Target, l domain.Level, exp time.Time) domain.Grant {
	return domain.Grant{ID: ids.New(), Subject: domain.Subject{Kind: domain.SubjectUser, PrincipalID: p}, Target: tg, Level: l, ExpiresAt: exp, GrantedAt: t0}
}

func groupGrant(g ids.ID, tg domain.Target) domain.Grant {
	return domain.Grant{ID: ids.New(), Subject: domain.Subject{Kind: domain.SubjectGroup, GroupID: g}, Target: tg, Level: domain.LevelTrigger, GrantedAt: t0}
}

func member(user ids.ID, exp, removed, deleted time.Time) domain.MembershipWithGroup {
	return domain.MembershipWithGroup{
		Membership: domain.Membership{ID: ids.New(), GroupID: tsGroup, UserID: user, ExpiresAt: exp, RemovedAt: removed, AddedAt: t0},
		Group:      domain.Group{ID: tsGroup, Name: "G", DeletedAt: deleted},
	}
}

var (
	instanceT = domain.Target{Kind: domain.TargetInstance}
	servedCh  = domain.Target{Kind: domain.TargetChannel, ChannelID: tsServed}
	servedSp  = domain.Target{Kind: domain.TargetSpace, SpaceID: tsS1}
	defaultSp = domain.Target{Kind: domain.TargetSpace, SpaceID: tsS2}
	otherSp   = domain.Target{Kind: domain.TargetSpace, SpaceID: tsS3}
	otherCh   = domain.Target{Kind: domain.TargetChannel, ChannelID: tsUnrel}
)

// TestTriggerSet_Table: the cases of brief M1-29 test 1.
func TestTriggerSet_Table(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		edit  func(w *domain.World)
		agent domain.PersonalAgent
		want  []ids.ID
	}{
		{"owners only", func(*domain.World) {}, tsPersAge, []ids.ID{tsOwner}},
		{"user grant on the instance", func(w *domain.World) {
			w.Grants = []domain.Grant{userGrant(tsAlex, instanceT, domain.LevelTrigger, time.Time{})}
		}, tsPersAge, []ids.ID{tsOwner, tsAlex}},
		{"user grant on the space of a served channel", func(w *domain.World) {
			w.Grants = []domain.Grant{userGrant(tsAlex, servedSp, domain.LevelTrigger, time.Time{})}
		}, tsPersAge, []ids.ID{tsOwner, tsAlex}},
		{"user grant on a space with the agent as default operator", func(w *domain.World) {
			w.Grants = []domain.Grant{userGrant(tsSam, defaultSp, domain.LevelTrigger, time.Time{})}
		}, tsPersAge, []ids.ID{tsOwner, tsSam}},
		{"user grant on a served channel", func(w *domain.World) {
			w.Grants = []domain.Grant{userGrant(tsAlex, servedCh, domain.LevelTrigger, time.Time{})}
		}, tsPersAge, []ids.ID{tsOwner, tsAlex}},
		{"grants elsewhere or below trigger do not count", func(w *domain.World) {
			w.Grants = []domain.Grant{userGrant(tsAlex, otherSp, domain.LevelTrigger, time.Time{}), userGrant(tsAlex, otherCh, domain.LevelTrigger, time.Time{}),
				userGrant(tsSam, servedCh, domain.LevelWrite, time.Time{})}
		}, tsPersAge, []ids.ID{tsOwner}},
		{"group grant with a valid membership", func(w *domain.World) {
			w.Grants = []domain.Grant{groupGrant(tsGroup, servedCh)}
			w.Memberships = []domain.MembershipWithGroup{member(tsSam, time.Time{}, time.Time{}, time.Time{})}
		}, tsPersAge, []ids.ID{tsOwner, tsSam}},
		{"group grant with a membership expired exactly at t", func(w *domain.World) {
			w.Grants = []domain.Grant{groupGrant(tsGroup, servedCh)}
			w.Memberships = []domain.MembershipWithGroup{member(tsSam, t0, time.Time{}, time.Time{})}
		}, tsPersAge, []ids.ID{tsOwner}},
		{"group grant with a removed membership", func(w *domain.World) {
			w.Grants = []domain.Grant{groupGrant(tsGroup, servedCh)}
			w.Memberships = []domain.MembershipWithGroup{member(tsSam, time.Time{}, t0.Add(-time.Hour), time.Time{})}
		}, tsPersAge, []ids.ID{tsOwner}},
		{"deleted group", func(w *domain.World) {
			w.Grants = []domain.Grant{groupGrant(tsGroup, servedCh)}
			w.Memberships = []domain.MembershipWithGroup{member(tsSam, time.Time{}, time.Time{}, t0.Add(-time.Hour))}
		}, tsPersAge, []ids.ID{tsOwner}},
		{"disabled user", func(w *domain.World) {
			w.Principals[tsAlex] = domain.PrincipalState{Kind: domain.KindUser, Active: false}
			w.Grants = []domain.Grant{userGrant(tsAlex, servedCh, domain.LevelTrigger, time.Time{})}
		}, tsPersAge, []ids.ID{tsOwner}},
		{"disabled group member", func(w *domain.World) {
			w.Principals[tsSam] = domain.PrincipalState{Kind: domain.KindUser, Active: false}
			w.Grants = []domain.Grant{groupGrant(tsGroup, servedCh)}
			w.Memberships = []domain.MembershipWithGroup{member(tsSam, time.Time{}, time.Time{}, time.Time{})}
		}, tsPersAge, []ids.ID{tsOwner}},
		{"revoked grant", func(w *domain.World) {
			g := userGrant(tsAlex, servedCh, domain.LevelTrigger, time.Time{})
			g.RevokedAt = t0.Add(-time.Minute)
			w.Grants = []domain.Grant{g}
		}, tsPersAge, []ids.ID{tsOwner}},
		{"grant expired exactly at t", func(w *domain.World) { w.Grants = []domain.Grant{userGrant(tsAlex, servedCh, domain.LevelTrigger, t0)} }, tsPersAge, []ids.ID{tsOwner}},
		{"channel served only through a closed thread with fixed operator (BR-02)", func(w *domain.World) {
			// The served target list of chat holds the channel of the thread; another agent
			// operates the channel itself.
			w.Served[tsAgent] = []domain.ServedTarget{{ChannelID: tsUnrel, SpaceID: tsS3}}
			w.Grants = []domain.Grant{userGrant(tsRobin, otherCh, domain.LevelTrigger, time.Time{})}
		}, tsPersAge, []ids.ID{tsOwner, tsRobin}},
		{"agent and app subjects never", func(w *domain.World) {
			w.Grants = []domain.Grant{
				{ID: ids.New(), Subject: domain.Subject{Kind: domain.SubjectAgent, PrincipalID: tsOther}, Target: instanceT, Level: domain.LevelTrigger},
				{ID: ids.New(), Subject: domain.Subject{Kind: domain.SubjectApp, PrincipalID: tsApp}, Target: servedCh, Level: domain.LevelTrigger},
				// A row whose subject kind lies about the real kind does not count either.
				userGrant(tsOther, servedCh, domain.LevelTrigger, time.Time{}),
			}
		}, tsPersAge, []ids.ID{tsOwner}},
		{"an agent without served targets still serves the instance", func(w *domain.World) {
			w.Grants = []domain.Grant{userGrant(tsAlex, instanceT, domain.LevelTrigger, time.Time{}), userGrant(tsSam, servedCh, domain.LevelTrigger, time.Time{})}
		}, domain.PersonalAgent{AgentID: ids.New(), CredentialHolderID: tsOwner}, []ids.ID{tsOwner, tsAlex}},
		{"no owner", func(w *domain.World) { w.Owners = nil }, tsPersAge, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			w := tsWorld()
			c.edit(&w)
			got := domain.TriggerSet(w, c.agent, t0)
			if !slices.Equal(got, domain.SortIDs(slices.Clone(c.want))) {
				t.Fatalf("TriggerSet = %v, want %v", got, c.want)
			}
		})
	}
}

// TestTriggerSet_SortedUnique: a principal with several trigger paths appears once, the result is
// ascending.
func TestTriggerSet_SortedUnique(t *testing.T) {
	t.Parallel()
	w := tsWorld()
	w.Owners = []ids.ID{tsSam, tsOwner}
	w.Grants = []domain.Grant{userGrant(tsSam, instanceT, domain.LevelTrigger, time.Time{}), userGrant(tsAlex, servedCh, domain.LevelTrigger, time.Time{}),
		userGrant(tsAlex, servedSp, domain.LevelTrigger, time.Time{}), groupGrant(tsGroup, servedCh)}
	w.Memberships = []domain.MembershipWithGroup{member(tsAlex, time.Time{}, time.Time{}, time.Time{})}
	if got := domain.TriggerSet(w, tsPersAge, t0); !slices.Equal(got, []ids.ID{tsOwner, tsAlex, tsSam}) {
		t.Fatalf("TriggerSet = %v", got)
	}
}

// seed returns the seed of a property test: BOTHY_TEST_SEED repeats a run.
func seed(t *testing.T) uint64 {
	t.Helper()
	if s := os.Getenv("BOTHY_TEST_SEED"); s != "" { //nolint:forbidigo // test input (seed), not server configuration
		n, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	n := uint64(time.Now().UnixNano())
	t.Logf("BOTHY_TEST_SEED=%d", n)
	return n
}

// TestTriggerSet_MatchesAuthorizeReference (brief M1-29 test 2, idea of AZ-P8): in seeded random
// worlds TriggerSet equals the set of people for which the rights snapshot and Effective of
// M1-28 give trigger on a served channel, its space, a served space or the instance.
func TestTriggerSet_MatchesAuthorizeReference(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewPCG(seed(t), 29)) //nolint:gosec // G404: a test world, no secret
	for range 300 {
		w, agent, people := randomWorld(r)
		var want []ids.ID
		for _, p := range people {
			if reachesTrigger(w, agent, p) {
				want = append(want, p)
			}
		}
		got := domain.TriggerSet(w, agent, t0)
		if !slices.Equal(got, domain.SortIDs(want)) {
			t.Fatalf("TriggerSet = %v, reference %v (%d grants, %d memberships, owners %v)", got, want, len(w.Grants), len(w.Memberships), w.Owners)
		}
	}
}

// reachesTrigger is the reference: snapshot plus Effective on every served path and the instance.
func reachesTrigger(w domain.World, agent domain.PersonalAgent, p ids.ID) bool {
	st := w.Principals[p]
	if st.Kind != domain.KindUser || !st.Active {
		return false
	}
	var own, groupGrants []domain.Grant
	var mems []domain.MembershipWithGroup
	for _, g := range w.Grants {
		if g.Subject.Kind != domain.SubjectGroup && g.Subject.PrincipalID == p {
			own = append(own, g)
		}
		if g.Subject.Kind == domain.SubjectGroup {
			groupGrants = append(groupGrants, g)
		}
	}
	for _, m := range w.Memberships {
		if m.Membership.UserID == p {
			mems = append(mems, m)
		}
	}
	snap := domain.BuildSnapshot(domain.SnapshotInput{
		PrincipalID: p, At: t0, Active: true, Owner: slices.Contains(w.Owners, p), Grants: own, Memberships: mems, GroupGrants: groupGrants,
	})
	paths := []domain.Path{{Kind: domain.PathInstance}}
	for _, s := range w.Served[agent.AgentID] {
		paths = append(paths, domain.Path{Kind: domain.PathSpace, SpaceID: s.SpaceID})
		if !s.ChannelID.IsZero() {
			paths = append(paths, domain.Path{Kind: domain.PathChannel, SpaceID: s.SpaceID, ChannelID: s.ChannelID})
		}
	}
	for _, path := range paths {
		if domain.Effective(snap, domain.KindUser, p, path).Level >= domain.LevelTrigger {
			return true
		}
	}
	return false
}

// randomWorld builds a world of a few people, groups, spaces and channels with random grants,
// memberships, states and served targets.
func randomWorld(r *rand.Rand) (domain.World, domain.PersonalAgent, []ids.ID) {
	spaces := []ids.ID{ids.New(), ids.New(), ids.New()}
	type ch struct{ id, space ids.ID }
	var chans []ch
	for range 5 {
		chans = append(chans, ch{ids.New(), spaces[r.IntN(len(spaces))]})
	}
	groups := []ids.ID{ids.New(), ids.New()}
	w := domain.World{Principals: map[ids.ID]domain.PrincipalState{}, Served: map[ids.ID][]domain.ServedTarget{}}
	var people []ids.ID
	for range 5 {
		p := ids.New()
		people = append(people, p)
		active := r.IntN(5) > 0
		w.Principals[p] = domain.PrincipalState{Kind: domain.KindUser, Active: active}
		// World.Owners holds active owners only (PrincipalSource.ActiveOwnerIDs).
		if active && r.IntN(5) == 0 {
			w.Owners = append(w.Owners, p)
		}
	}
	bot := ids.New()
	w.Principals[bot] = domain.PrincipalState{Kind: domain.KindAgent, Active: true}
	agent := domain.PersonalAgent{AgentID: ids.New(), CredentialHolderID: people[0]}
	for _, c := range chans {
		if r.IntN(3) == 0 {
			w.Served[agent.AgentID] = append(w.Served[agent.AgentID], domain.ServedTarget{ChannelID: c.id, SpaceID: c.space})
		}
	}
	if r.IntN(3) == 0 {
		w.Served[agent.AgentID] = append(w.Served[agent.AgentID], domain.ServedTarget{SpaceID: spaces[r.IntN(len(spaces))]})
	}
	levels := []domain.Level{domain.LevelRead, domain.LevelWrite, domain.LevelTrigger}
	times := []time.Time{{}, t0, t0.Add(-time.Hour), t0.Add(time.Hour)}
	target := func() domain.Target {
		switch r.IntN(3) {
		case 0:
			return domain.Target{Kind: domain.TargetInstance}
		case 1:
			return domain.Target{Kind: domain.TargetSpace, SpaceID: spaces[r.IntN(len(spaces))]}
		}
		return domain.Target{Kind: domain.TargetChannel, ChannelID: chans[r.IntN(len(chans))].id}
	}
	for range 8 {
		g := domain.Grant{ID: ids.New(), Target: target(), Level: levels[r.IntN(3)], ExpiresAt: times[r.IntN(len(times))], GrantedAt: t0}
		switch r.IntN(4) {
		case 0:
			g.Subject = domain.Subject{Kind: domain.SubjectGroup, GroupID: groups[r.IntN(len(groups))]}
		case 1:
			g.Subject = domain.Subject{Kind: domain.SubjectAgent, PrincipalID: bot}
		default:
			g.Subject = domain.Subject{Kind: domain.SubjectUser, PrincipalID: people[r.IntN(len(people))]}
		}
		if r.IntN(6) == 0 {
			g.RevokedAt = t0.Add(-time.Minute)
		}
		if g.ExpiresAt.Equal(t0.Add(-time.Hour)) {
			g.ExpiresAt = time.Time{}
		}
		w.Grants = append(w.Grants, g)
	}
	for _, gid := range groups {
		deleted := time.Time{}
		if r.IntN(5) == 0 {
			deleted = t0.Add(-time.Minute)
		}
		for _, p := range people {
			if r.IntN(2) == 0 {
				continue
			}
			m := domain.Membership{ID: ids.New(), GroupID: gid, UserID: p, AddedAt: t0, ExpiresAt: times[r.IntN(len(times))]}
			if m.ExpiresAt.Equal(t0.Add(-time.Hour)) {
				m.ExpiresAt = time.Time{}
			}
			if r.IntN(5) == 0 {
				m.RemovedAt = t0.Add(-time.Minute)
			}
			w.Memberships = append(w.Memberships, domain.MembershipWithGroup{Membership: m, Group: domain.Group{ID: gid, DeletedAt: deleted}})
		}
	}
	return w, agent, people
}
