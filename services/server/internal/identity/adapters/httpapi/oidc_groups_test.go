// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	authzdomain "github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/oidc/oidctest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// groupsClaim is groups_claim of the provider of the claim group worlds.
const groupsClaim = "groups"

// TestGroupClaimSync: the claim groups follow the group claim at every OIDC login (brief M1-42
// "Integration tests"), against the fake provider with the real rights path of authz.
func TestGroupClaimSync(t *testing.T) {
	runOIDCCases(t, oidcWorldOpts{groupsClaim: groupsClaim}, []oidcCase{
		{"TestGroupClaimSync_AddsWithExpiry", caseClaimAddsWithExpiry},
		{"TestGroupClaimSync_ExtendsOnLogin", caseClaimExtendsOnLogin},
		{"TestGroupClaimSync_RemovesMissing", caseClaimRemovesMissing},
		{"TestGroupClaimSync_AbsentClaimNoChange", caseClaimAbsentNoChange},
		{"TestGroupClaimSync_EntraOverageNoChange", caseClaimOverageNoChange},
		{"TestGroupClaimSync_WrongTypeFailsCallback", caseClaimWrongType},
		{"TestGroupClaimSync_ConcurrentLogins", caseClaimConcurrentLogins},
		{"TestGroupClaimSync_ValuesNeverLogged", caseClaimValuesNeverLogged},
	})
}

// TestGroupClaimSync_I1: the AZ-P7 case of group.member.sync through the login.
func TestGroupClaimSync_I1(t *testing.T) {
	runOIDCCases(t, oidcWorldOpts{groupsClaim: groupsClaim}, []oidcCase{
		{"TestGroupClaimSync_I1ViolationFailsLoginNothingChanges", caseClaimI1ViolationFailsLogin},
	})
}

// TestGroupClaimSync_NoGroupsClaim: a provider without groups_claim never syncs.
func TestGroupClaimSync_NoGroupsClaim(t *testing.T) {
	runOIDCCases(t, oidcWorldOpts{}, []oidcCase{
		{"TestGroupClaimSync_ProviderWithoutGroupsClaimNoChange", caseClaimProviderWithoutGroupsClaim},
	})
}

// claimOwner returns the owner of the world, creating it on first use: every rights change needs
// an active owner (SR8), and claim groups need a creator.
func (w *oworld) claimOwner() ids.ID {
	w.t.Helper()
	var id string
	w.scalar(&id, `SELECT coalesce((SELECT user_id::text FROM owners WHERE revoked_at IS NULL ORDER BY id LIMIT 1), '')`)
	if id != "" {
		return ids.MustParse(id)
	}
	return w.person(w.handle("owner"), true)
}

// claimGroup inserts a live claim group of the world's provider for the value ref, with a read
// grant on a new channel, and returns the group.
func (w *oworld) claimGroup(name, ref string) ids.ID {
	w.t.Helper()
	owner := w.claimOwner()
	g, space, ch := ids.New(), ids.New(), ids.New()
	w.sql(`INSERT INTO groups (id, name, source, provider_id, external_ref, created_by) VALUES ($1, $2, 'oidc_claim', $3, $4, $5)`,
		g.String(), name, w.prov.String(), ref, owner.String())
	w.structure.PutSpace(space)
	w.structure.PutChannel(authz.ChannelFacts{ID: ch, SpaceID: space, Kind: authz.ChannelText})
	w.sql(`INSERT INTO grants (id, subject_kind, subject_group_id, target_kind, target_channel_id, level, reason, granted_by)
		VALUES ($1, 'group', $2, 'channel', $3, 'read', 'claim group', $4)`, ids.New().String(), g.String(), ch.String(), owner.String())
	return g
}

// claimLogin runs a whole login of l with the group claim values (nil: no claim), which must
// succeed.
func (w *oworld) claimLogin(l oidctest.Login, values []string) {
	w.t.Helper()
	if values == nil {
		w.idp.DropClaim(groupsClaim)
	} else {
		w.idp.SetGroups(groupsClaim, values...)
	}
	defer w.idp.Reset()
	f, code := w.loginCode(l, nil)
	w.want(w.complete(w.phoneKeys(), code, f.verifier), http.StatusCreated, "")
}

// membership is one row of group_memberships as these tests read it.
type membership struct {
	expires, reminded, removed time.Time
	addedBy, removedBy         string
}

// memberships returns the memberships of user in group, ascending by id.
func (w *oworld) memberships(group, user ids.ID) []membership {
	w.t.Helper()
	rows, err := w.owner.Query(w.t.Context(), `SELECT expires_at, expiry_reminded_at, removed_at, added_by::text, coalesce(removed_by::text, '')
		FROM group_memberships WHERE group_id = $1 AND user_id = $2 ORDER BY id`, group.String(), user.String())
	if err != nil {
		w.t.Fatal(err)
	}
	defer rows.Close()
	var out []membership
	for rows.Next() {
		var m membership
		var exp, rem, gone *time.Time
		if err := rows.Scan(&exp, &rem, &gone, &m.addedBy, &m.removedBy); err != nil {
			w.t.Fatal(err)
		}
		for _, p := range []struct {
			src *time.Time
			dst *time.Time
		}{{exp, &m.expires}, {rem, &m.reminded}, {gone, &m.removed}} {
			if p.src != nil {
				*p.dst = p.src.UTC()
			}
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		w.t.Fatal(err)
	}
	return out
}

// rightsState is what a login without change must leave alone.
type rightsState struct {
	version, events int
	memberships     string
}

// rightsNow reads permission_version, the number of rights events and a hash of the memberships.
func (w *oworld) rightsNow() rightsState {
	w.t.Helper()
	var s rightsState
	s.version = w.count(`SELECT value FROM sequence_counters WHERE name = 'permission_version'`)
	s.events = w.count(`SELECT count(*) FROM event_log WHERE type IN ('access.changed', 'group.changed')`)
	w.scalar(&s.memberships, `SELECT coalesce(md5(string_agg(m::text, ',' ORDER BY id)), '') FROM group_memberships m`)
	return s
}

// eventsFor counts the events of type typ for the audience or with resource id.
func (w *oworld) eventsFor(typ string, id ids.ID) int {
	w.t.Helper()
	return w.count(`SELECT count(*) FROM event_log WHERE type = $1 AND (audience_principal_id = $2 OR resource_id = $2)`, typ, id.String())
}

func caseClaimAddsWithExpiry(t *testing.T, w *oworld) {
	support := w.claimGroup("Support", "support-ref")
	sam, l := w.knownPerson()
	before := w.rightsNow()
	w.claimLogin(l, []string{"support-ref", "unmapped-ref"})
	ms := w.memberships(support, sam)
	if len(ms) != 1 || !ms[0].expires.Equal(w.clk.Now().Add(sessCfg.GroupClaimMaxAge)) || ms[0].addedBy != sam.String() || !ms[0].removed.IsZero() {
		t.Fatalf("memberships %+v", ms)
	}
	if after := w.rightsNow(); after.version != before.version+1 {
		t.Fatalf("permission_version %d, want %d", after.version, before.version+1)
	}
	if n := w.eventsFor("access.changed", sam); n != 1 {
		t.Fatalf("access.changed for Sam: %d", n)
	}
	if n := w.count(`SELECT count(*) FROM event_log WHERE type = 'group.changed' AND resource_id = $1 AND payload->>'change' = 'member_added'
		AND payload->>'user_id' = $2`, support.String(), sam.String()); n != 1 {
		t.Fatalf("group.changed member_added: %d", n)
	}
	var syncs []sauditRow
	for _, a := range w.audits("group.member.sync") {
		if a.ResourceID == sam.String() {
			syncs = append(syncs, a)
		}
	}
	if len(syncs) != 1 || syncs[0].Decision != "allow" || syncs[0].Details["provider_id"] != w.prov.String() {
		t.Fatalf("audit %+v", syncs)
	}
}

func caseClaimExtendsOnLogin(t *testing.T, w *oworld) {
	support := w.claimGroup("Support Extend", "support-extend")
	sam, l := w.knownPerson()
	w.claimLogin(l, []string{"support-extend"})
	w.sql(`UPDATE group_memberships SET expiry_reminded_at = now() WHERE group_id = $1`, support.String())
	w.clk.Advance(2 * time.Hour)
	w.claimLogin(l, []string{"support-extend"})
	ms := w.memberships(support, sam)
	if len(ms) != 1 || !ms[0].expires.Equal(w.clk.Now().Add(sessCfg.GroupClaimMaxAge)) || !ms[0].reminded.IsZero() {
		t.Fatalf("memberships %+v", ms)
	}
}

func caseClaimRemovesMissing(t *testing.T, w *oworld) {
	support := w.claimGroup("Support Remove", "support-remove")
	sam, l := w.knownPerson()
	w.claimLogin(l, []string{"support-remove"})
	w.claimLogin(l, []string{})
	ms := w.memberships(support, sam)
	if len(ms) != 1 || ms[0].removed.IsZero() || ms[0].removedBy != sam.String() {
		t.Fatalf("memberships %+v", ms)
	}
	if n := w.count(`SELECT count(*) FROM event_log WHERE type = 'group.changed' AND resource_id = $1 AND payload->>'change' = 'member_removed'`,
		support.String()); n != 1 {
		t.Fatalf("group.changed member_removed: %d", n)
	}
}

func caseClaimAbsentNoChange(t *testing.T, w *oworld) {
	support := w.claimGroup("Support Absent", "support-absent")
	sam, l := w.knownPerson()
	w.claimLogin(l, []string{"support-absent"})
	before := w.rightsNow()
	w.claimLogin(l, nil)
	if after := w.rightsNow(); after != before {
		t.Fatalf("an absent claim changed rights:\nbefore %+v\nafter  %+v", before, after)
	}
	if ms := w.memberships(support, sam); len(ms) != 1 || !ms[0].removed.IsZero() {
		t.Fatalf("memberships %+v", ms)
	}
}

func caseClaimOverageNoChange(t *testing.T, w *oworld) {
	support := w.claimGroup("Support Overage", "support-overage")
	sam, l := w.knownPerson()
	w.claimLogin(l, []string{"support-overage"})
	before := w.rightsNow()
	w.idp.SetGroupsOverage(groupsClaim)
	f, code := w.loginCode(l, nil)
	w.idp.Reset()
	w.want(w.complete(w.phoneKeys(), code, f.verifier), http.StatusCreated, "")
	if after := w.rightsNow(); after != before {
		t.Fatalf("an overage changed rights:\nbefore %+v\nafter  %+v", before, after)
	}
	if ms := w.memberships(support, sam); len(ms) != 1 || !ms[0].removed.IsZero() {
		t.Fatalf("memberships %+v", ms)
	}
}

func caseClaimWrongType(t *testing.T, w *oworld) {
	_, l := w.knownPerson()
	w.idp.SetClaim(groupsClaim, "support-ref")
	defer w.idp.Reset()
	f, r := w.begin(oidcBeginPath, nil, nil)
	w.want(r, http.StatusOK, "")
	w.want(w.callback(f, l), http.StatusFound, "")
	if e := f.redirect.Query().Get("error"); e != "login_failed" {
		t.Fatalf("redirect %s", f.redirect)
	}
	if st, e := w.flowStatus(f.attempt); st != "failed" || e != "login_failed" {
		t.Fatalf("flow %s %s", st, e)
	}
}

func caseClaimConcurrentLogins(t *testing.T, w *oworld) {
	support := w.claimGroup("Support Parallel", "support-parallel")
	sam, l := w.knownPerson()
	w.idp.SetGroups(groupsClaim, "support-parallel")
	defer w.idp.Reset()
	f1, c1 := w.loginCode(l, nil)
	f2, c2 := w.loginCode(l, nil)
	var wg sync.WaitGroup
	codes := make([]int, 2)
	for i, fc := range []struct {
		f    *flow
		code string
	}{{f1, c1}, {f2, c2}} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes[i] = w.complete(w.phoneKeys(), fc.code, fc.f.verifier).StatusCode
		}()
	}
	wg.Wait()
	if codes[0] != http.StatusCreated || codes[1] != http.StatusCreated {
		t.Fatalf("statuses %v", codes)
	}
	ms := w.memberships(support, sam)
	if len(ms) != 1 || !ms[0].removed.IsZero() {
		t.Fatalf("memberships %+v", ms)
	}
}

func caseClaimValuesNeverLogged(t *testing.T, w *oworld) {
	w.claimGroup("Research", "research-ref-7f3a")
	_, l := w.knownPerson()
	w.claimLogin(l, []string{"research-ref-7f3a", "unmapped-ref-9c1d"})
	var stored, events string
	w.scalar(&stored, `SELECT coalesce(string_agg(details::text, ' '), '') FROM audit_events`)
	w.scalar(&events, `SELECT coalesce(string_agg(payload::text, ' '), '') FROM event_log`)
	for where, text := range map[string]string{"logs": w.logText(), "audit": stored} {
		for _, v := range []string{"research-ref-7f3a", "unmapped-ref-9c1d"} {
			if strings.Contains(text, v) {
				t.Errorf("the claim value %s is in the %s", v, where)
			}
		}
	}
	// group.changed carries the Group of the contract, whose external_ref is the owner's mapping
	// (readable with group.read); a value the login alone delivered never reaches an event.
	if strings.Contains(events, "unmapped-ref-9c1d") {
		t.Error("an unmapped claim value is in the events")
	}
}

// caseClaimI1ViolationFailsLogin is AZ-P7 for group.member.sync (M1-29 leaves it to M1-42): the
// owner's personal agent operates a channel on which Engineering has trigger; Robin's claim maps
// him into Engineering, so the login is refused with 409 i1_violation and leaves no membership,
// session, device, identity link or used enrollment code, only the deny entry.
func caseClaimI1ViolationFailsLogin(t *testing.T, w *oworld) {
	owner := w.claimOwner()
	agent, space, ch := ids.New(), ids.New(), ids.New()
	w.policies.Put(agent, authz.AgentPolicy{BillingMode: authzdomain.BillingPersonal, CredentialHolderID: owner, ToolClass: authzdomain.ToolScoped})
	w.structure.PutSpace(space)
	w.structure.PutChannel(authz.ChannelFacts{ID: ch, SpaceID: space, Kind: authz.ChannelText, EffectiveOperatorID: agent})
	eng := ids.New()
	w.sql(`INSERT INTO groups (id, name, source, provider_id, external_ref, created_by) VALUES ($1, 'Engineering', 'oidc_claim', $2, 'engineering', $3)`,
		eng.String(), w.prov.String(), owner.String())
	w.sql(`INSERT INTO grants (id, subject_kind, subject_group_id, target_kind, target_channel_id, level, reason, granted_by)
		VALUES ($1, 'group', $2, 'channel', $3, 'trigger', 'engineering', $4)`, ids.New().String(), eng.String(), ch.String(), owner.String())
	robin, c := w.linkCode()
	l := w.login()
	w.idp.SetGroups(groupsClaim, "engineering")
	defer w.idp.Reset()
	f, code := w.loginCode(l, map[string]any{"enrollment_code": c.Code})
	before := w.rightsNow()
	ph := w.phoneKeys()
	m := w.want(w.complete(ph, code, f.verifier), http.StatusConflict, "i1_violation")
	if m["hint"] != authz.HintI1ClaimSync {
		t.Fatalf("answer %v", m)
	}
	if after := w.rightsNow(); after != before {
		t.Fatalf("a refused sync changed rights:\nbefore %+v\nafter  %+v", before, after)
	}
	for what, q := range map[string]string{
		"membership":    `SELECT count(*) FROM group_memberships WHERE user_id = $1`,
		"session":       `SELECT count(*) FROM auth_sessions WHERE user_id = $1`,
		"device":        `SELECT count(*) FROM devices WHERE user_id = $1`,
		"identity link": `SELECT count(*) FROM user_identities WHERE user_id = $1`,
	} {
		if n := w.count(q, robin.String()); n != 0 {
			t.Fatalf("a %s of Robin remains", what)
		}
	}
	if n := w.count(`SELECT count(*) FROM enrollment_codes WHERE id = $1 AND used_at IS NULL`, c.ID.String()); n != 1 {
		t.Fatal("the enrollment code was used")
	}
	if st, _ := w.flowStatus(f.attempt); st != "failed" {
		t.Fatalf("flow %s", st)
	}
	var denies []sauditRow
	for _, a := range w.audits("group.member.sync") {
		if a.Decision == "deny" && a.ResourceID == robin.String() {
			denies = append(denies, a)
		}
	}
	if len(denies) != 1 || denies[0].Reason != "i1_violation" || denies[0].Details["provider_id"] != w.prov.String() ||
		!strings.Contains(denies[0].Raw, eng.String()) || !strings.Contains(denies[0].Raw, agent.String()) {
		t.Fatalf("deny entries %+v", denies)
	}
}

func caseClaimProviderWithoutGroupsClaim(t *testing.T, w *oworld) {
	support := w.claimGroup("Support", "support-ref")
	sam, l := w.knownPerson()
	w.idp.SetGroups(groupsClaim, "support-ref")
	defer w.idp.Reset()
	before := w.rightsNow()
	f, code := w.loginCode(l, nil)
	var ext string
	w.scalar(&ext, `SELECT external_identity::text FROM auth_login_flows WHERE id = $1`, f.attempt)
	if strings.Contains(ext, "groups") {
		t.Fatalf("external identity %s", ext)
	}
	w.want(w.complete(w.phoneKeys(), code, f.verifier), http.StatusCreated, "")
	if after := w.rightsNow(); after != before {
		t.Fatalf("a provider without groups_claim changed rights")
	}
	if ms := w.memberships(support, sam); len(ms) != 0 {
		t.Fatalf("memberships %+v", ms)
	}
}
