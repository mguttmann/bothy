// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/netip"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi-validator/schema_validation"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
)

// The configuration of 07 section 14 that the expiry tests use.
const (
	sweepInterval = time.Minute
	reminderLead  = 48 * time.Hour
)

// g3Expiry is the expiry of G-3 (Robin on Projekt-A) in the seed.
var g3Expiry = time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)

// leserExpiry is the expiry of Sam's membership in Leser in the seed.
var leserExpiry = pgStart.Add(48 * time.Hour)

// expiry returns the expiry service of the world with batch as authz.expiry_sweep_batch_max.
func (w *pgWorld) expiry(t *testing.T, batch int) *app.ExpiryService {
	t.Helper()
	svc, err := app.NewExpiryService(app.ExpiryConfig{BatchMax: batch, ReminderLead: reminderLead}, app.ExpiryDeps{
		UoW: w.uow, Expiry: w.store, Rights: w.hook, Principals: w.principals,
		Events: w.appender, Clock: w.clk, Log: slog.New(slog.DiscardHandler),
	})
	mustOK(t, err)
	return svc
}

// sweepAt sets the clock to at and runs one sweep with the default batch.
func (w *pgWorld) sweepAt(t *testing.T, at time.Time) app.ExpiryResult {
	t.Helper()
	w.clk.Set(at)
	res, err := w.expiry(t, 500).Sweep(context.Background())
	mustOK(t, err)
	return res
}

// remindAt sets the clock to at and runs one reminder run with the default batch.
func (w *pgWorld) remindAt(t *testing.T, at time.Time) app.ExpiryResult {
	t.Helper()
	w.clk.Set(at)
	res, err := w.expiry(t, 500).Remind(context.Background())
	mustOK(t, err)
	return res
}

// logRow is one row of event_log of any type.
type logRow struct {
	Type, ResourceKind, ActorKind, Receive, Payload string
	Resource, Audience                              ids.ID
	Space, Channel                                  string
	Version                                         int64
}

// eventsOf returns the rows of type typ after seq, ascending.
func (w *pgWorld) eventsOf(t *testing.T, typ string, afterSeq int64) []logRow {
	t.Helper()
	rows, err := w.db.SuperConn(t).Query(context.Background(), `SELECT type, resource_kind, coalesce(actor_kind, ''), receive_action,
		payload::text, coalesce(resource_id::text, ''), coalesce(audience_principal_id::text, ''), coalesce(space_id::text, ''),
		coalesce(channel_id::text, ''), permission_version FROM event_log WHERE type = $1 AND seq > $2 ORDER BY seq`, typ, afterSeq)
	mustOK(t, err)
	defer rows.Close()
	var out []logRow
	for rows.Next() {
		var r logRow
		var res, aud string
		mustOK(t, rows.Scan(&r.Type, &r.ResourceKind, &r.ActorKind, &r.Receive, &r.Payload, &res, &aud, &r.Space, &r.Channel, &r.Version))
		if res != "" {
			r.Resource = ids.MustParse(res)
		}
		if aud != "" {
			r.Audience = ids.MustParse(aud)
		}
		out = append(out, r)
	}
	mustOK(t, rows.Err())
	return out
}

// validateAgainst validates p against the schema name of the contract and refuses null.
func validateAgainst(t *testing.T, name, p string) {
	t.Helper()
	doc, err := libopenapi.NewDocument(rest.Spec)
	mustOK(t, err)
	model, err := doc.BuildV3Model()
	mustOK(t, err)
	proxy, ok := model.Model.Components.Schemas.Get(name)
	if !ok {
		t.Fatalf("no schema %s", name)
	}
	if valid, problems := schema_validation.NewSchemaValidator().ValidateSchemaString(proxy.Schema(), p); !valid {
		for _, pr := range problems {
			t.Errorf("schema: %s %s", pr.Message, pr.Reason)
		}
		t.Fatalf("payload does not match %s: %s", name, p)
	}
	if strings.Contains(p, "null") {
		t.Fatalf("null in %s", p)
	}
}

// reportedAt reads expiry_reported_at of a grant or membership; zero for NULL.
func (w *pgWorld) reportedAt(t *testing.T, table string, id ids.ID) time.Time {
	t.Helper()
	var at *time.Time
	mustOK(t, w.db.SuperConn(t).QueryRow(context.Background(), `SELECT expiry_reported_at FROM `+table+` WHERE id = $1`, id.String()).Scan(&at))
	if at == nil {
		return time.Time{}
	}
	return at.UTC()
}

// membershipOf returns the id of the membership of user in group.
func (w *pgWorld) membershipOf(t *testing.T, group, user ids.ID) ids.ID {
	t.Helper()
	var id string
	mustOK(t, w.db.SuperConn(t).QueryRow(context.Background(), `SELECT id::text FROM group_memberships WHERE group_id = $1 AND user_id = $2`,
		group.String(), user.String()).Scan(&id))
	return ids.MustParse(id)
}

// allowsOf returns the allow rows of action after seq.
func (w *pgWorld) allowsOf(t *testing.T, action string, after int64) []allowRow {
	t.Helper()
	var out []allowRow
	for _, r := range w.allows(t, after) {
		if r.Action == action {
			out = append(out, r)
		}
	}
	return out
}

// removedIDs returns the ids of the removed entries of an access.changed payload.
func removedIDs(t *testing.T, p string) []string {
	t.Helper()
	var out []string
	for _, e := range decodePayload(t, p).Removed {
		out = append(out, e.ID)
	}
	return out
}

func TestAZ_P10_ExpiryWithoutJob(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	ctx := context.Background()
	v, seq := w.version(t), w.lastSeq(t)
	check := func(p ids.ID, channel ids.ID, at time.Time, allowed bool) {
		t.Helper()
		w.clk.Set(at)
		d, err := w.az.Authorize(ctx, w.user(p), authorize.ActionChannelRead, authorize.Resource{Kind: authorize.ResourceChannel, ID: channel})
		mustOK(t, err)
		if allowed != d.Allowed() || (!allowed && d.Outcome != authorize.OutcomeNotFound) {
			t.Fatalf("at %v: %+v, want allowed=%v", at, d, allowed)
		}
	}
	// The same Authorizer, so the second call is answered from a cached snapshot unless
	// valid_until ends it.
	check(w.sam, w.allgemein, leserExpiry.Add(-time.Microsecond), true)
	check(w.sam, w.allgemein, leserExpiry, false)
	check(w.robin, w.projektA, g3Expiry.Add(-time.Microsecond), true)
	check(w.robin, w.projektA, g3Expiry, false)
	if w.version(t) != v || w.lastSeq(t) != seq {
		t.Fatal("an expiry without job changed the rights version")
	}
}

func TestExpirySweep_Grant(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	w.sweepAt(t, leserExpiry.Add(sweepInterval)) // reports the Leser membership first
	seq, audit, v := w.lastSeq(t), w.auditSeq(t), w.version(t)
	at := g3Expiry.Add(sweepInterval)
	res := w.sweepAt(t, at)
	if res.Grants != 1 || res.Memberships != 0 {
		t.Fatalf("result %+v", res)
	}
	changed := w.eventsOf(t, "access.changed", seq)
	if len(changed) != 1 || changed[0].Audience != w.robin || changed[0].ActorKind != "system" ||
		!slices.Contains(removedIDs(t, changed[0].Payload), w.projektA.String()) {
		t.Fatalf("access.changed %+v", changed)
	}
	gc := w.eventsOf(t, "grant.changed", seq)
	if len(gc) != 1 || gc[0].Resource != w.g3 || gc[0].ResourceKind != "grant" || gc[0].ActorKind != "system" ||
		gc[0].Receive != "grant.read" || gc[0].Channel != w.projektA.String() || gc[0].Space != w.engineering.String() {
		t.Fatalf("grant.changed %+v", gc)
	}
	validateAgainst(t, "GrantChangedPayload", gc[0].Payload)
	var p struct {
		Change string `json:"change"`
		Grant  struct {
			ID, SubjectID, TargetID string
		} `json:"grant"`
	}
	mustOK(t, json.Unmarshal([]byte(gc[0].Payload), &p))
	if p.Change != "expired" || p.Grant.ID != w.g3.String() {
		t.Fatalf("payload %s", gc[0].Payload)
	}
	rows := w.allowsOf(t, "grant.expire", audit)
	if len(rows) != 1 || !rows[0].Actor.IsZero() || rows[0].ResourceKind != "grant" || rows[0].ResourceID != w.g3 ||
		!strings.Contains(rows[0].Details, `"subject_kind": "user"`) || !strings.Contains(rows[0].Details, w.robin.String()) ||
		!strings.Contains(rows[0].Details, w.projektA.String()) || !strings.Contains(rows[0].Details, `"expires_at"`) {
		t.Fatalf("audit %+v", rows)
	}
	if got := w.reportedAt(t, "grants", w.g3); !got.Equal(at) {
		t.Fatalf("expiry_reported_at %v, want %v", got, at)
	}
	if w.version(t) != v+1 {
		t.Fatalf("version %d, want %d", w.version(t), v+1)
	}
	if pub := w.published(); len(pub) == 0 || pub[len(pub)-1].Action != authorize.ActionGrantExpire {
		t.Fatalf("published %+v", pub)
	}
}

func TestExpirySweep_GroupGrant(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	ctx := context.Background()
	g, err := w.admin.CreateGroup(ctx, w.user(w.owner), authz.CreateGroupCmd{Name: "Team"})
	mustOK(t, err)
	for _, p := range []ids.ID{w.robin, w.alex} {
		_, err := w.admin.AddGroupMember(ctx, w.user(w.owner), g.ID, authz.AddMemberCmd{UserID: p})
		mustOK(t, err)
	}
	exp := pgStart.Add(time.Hour)
	kampagnen := domain.Target{Kind: domain.TargetChannel, ChannelID: w.kampagnen}
	gg := w.grantRow(t, domain.Subject{Kind: domain.SubjectGroup, GroupID: g.ID}, kampagnen, domain.LevelRead, exp)
	w.grantRow(t, domain.Subject{Kind: domain.SubjectUser, PrincipalID: w.alex}, kampagnen, domain.LevelRead, time.Time{})
	seq, audit := w.lastSeq(t), w.auditSeq(t)
	if res := w.sweepAt(t, exp.Add(sweepInterval)); res.Grants != 1 {
		t.Fatalf("result %+v", res)
	}
	changed := w.eventsOf(t, "access.changed", seq)
	if len(changed) != 1 || changed[0].Audience != w.robin || !slices.Contains(removedIDs(t, changed[0].Payload), w.kampagnen.String()) {
		t.Fatalf("access.changed %+v", changed)
	}
	rows := w.allowsOf(t, "grant.expire", audit)
	if len(rows) != 1 || rows[0].ResourceID != gg || !strings.Contains(rows[0].Details, `"subject_kind": "group"`) ||
		!strings.Contains(rows[0].Details, g.ID.String()) {
		t.Fatalf("audit %+v", rows)
	}
}

func TestExpirySweep_Membership(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	seq, audit, v := w.lastSeq(t), w.auditSeq(t), w.version(t)
	res := w.sweepAt(t, leserExpiry.Add(sweepInterval))
	if res.Memberships != 1 || res.Grants != 0 {
		t.Fatalf("result %+v", res)
	}
	changed := w.eventsOf(t, "access.changed", seq)
	if len(changed) != 1 || changed[0].Audience != w.sam || !slices.Contains(removedIDs(t, changed[0].Payload), w.allgemein.String()) {
		t.Fatalf("access.changed %+v", changed)
	}
	gc := w.eventsOf(t, "group.changed", seq)
	if len(gc) != 1 || gc[0].Resource != w.leser || gc[0].ResourceKind != "group" || gc[0].Receive != "group.read" ||
		gc[0].ActorKind != "system" || gc[0].Space != "" || gc[0].Channel != "" {
		t.Fatalf("group.changed %+v", gc)
	}
	validateAgainst(t, "GroupChangedPayload", gc[0].Payload)
	var p struct {
		Change string `json:"change"`
		UserID string `json:"user_id"`
		Group  struct {
			ID          string `json:"id"`
			MemberCount int    `json:"member_count"`
		} `json:"group"`
	}
	mustOK(t, json.Unmarshal([]byte(gc[0].Payload), &p))
	if p.Change != "member_expired" || p.UserID != w.sam.String() || p.Group.ID != w.leser.String() || p.Group.MemberCount != 0 {
		t.Fatalf("payload %s", gc[0].Payload)
	}
	m := w.membershipOf(t, w.leser, w.sam)
	rows := w.allowsOf(t, "group.member.expire", audit)
	if len(rows) != 1 || !rows[0].Actor.IsZero() || rows[0].ResourceKind != "group" || rows[0].ResourceID != w.leser ||
		!strings.Contains(rows[0].Details, m.String()) || !strings.Contains(rows[0].Details, w.sam.String()) ||
		!strings.Contains(rows[0].Details, `"expires_at"`) {
		t.Fatalf("audit %+v", rows)
	}
	if w.reportedAt(t, "group_memberships", m).IsZero() || w.version(t) != v+1 {
		t.Fatal("membership not marked or version not raised once")
	}
}

func TestExpirySweep_ExactlyOnce(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	at := leserExpiry.Add(sweepInterval)
	w.clk.Set(at)
	before := w.snapshot(t)
	// A subscriber of authorize.AccessChanged that fails stands for any failure in the unit.
	w.setVeto(errors.New("subscriber down"))
	if _, err := w.expiry(t, 500).Sweep(context.Background()); err == nil {
		t.Fatal("a sweep with a failing subscriber succeeded")
	}
	w.setVeto(nil)
	if after := w.snapshot(t); after != before {
		t.Fatalf("a failed sweep left something behind:\nbefore %+v\nafter  %+v", before, after)
	}
	seq, audit := w.lastSeq(t), w.auditSeq(t)
	w.sweepAt(t, at)
	w.sweepAt(t, at.Add(sweepInterval))
	if n := len(w.eventsOf(t, "group.changed", seq)); n != 1 {
		t.Fatalf("%d group.changed, want 1", n)
	}
	if n := len(w.eventsOf(t, "access.changed", seq)); n != 1 {
		t.Fatalf("%d access.changed, want 1", n)
	}
	if n := len(w.allowsOf(t, "group.member.expire", audit)); n != 1 {
		t.Fatalf("%d audit rows, want 1", n)
	}
}

func TestExpirySweep_NoDueRowsKeepsVersion(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	before := w.snapshot(t)
	if res := w.sweepAt(t, pgStart.Add(sweepInterval)); res != (app.ExpiryResult{}) {
		t.Fatalf("result %+v", res)
	}
	if after := w.snapshot(t); after != before {
		t.Fatalf("a sweep without due rows changed something:\nbefore %+v\nafter  %+v", before, after)
	}
}

func TestExpirySweep_RevokedBeforeSweep(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	_, err := w.admin.RevokeGrant(context.Background(), w.user(w.owner), w.g3, "")
	mustOK(t, err)
	audit, seq := w.auditSeq(t), w.lastSeq(t)
	if res := w.sweepAt(t, g3Expiry.Add(sweepInterval)); res.Grants != 0 {
		t.Fatalf("result %+v", res)
	}
	if len(w.eventsOf(t, "grant.changed", seq)) != 0 || len(w.allowsOf(t, "grant.expire", audit)) != 0 || !w.reportedAt(t, "grants", w.g3).IsZero() {
		t.Fatal("a revoked grant was reported as expired")
	}
}

func TestExpirySweep_Batches(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	exp := pgStart.Add(time.Hour)
	var due []ids.ID
	for _, c := range []ids.ID{w.allgemein, w.eskalation, w.kampagnen, w.werkzeuge, w.projektA} {
		due = append(due, w.grantRow(t, domain.Subject{Kind: domain.SubjectUser, PrincipalID: w.alex},
			domain.Target{Kind: domain.TargetChannel, ChannelID: c}, domain.LevelRead, exp))
	}
	v, audit := w.version(t), w.auditSeq(t)
	w.clk.Set(exp.Add(sweepInterval))
	res, err := w.expiry(t, 2).Sweep(context.Background())
	mustOK(t, err)
	if res.Grants != 5 || res.Batches != 3 {
		t.Fatalf("result %+v", res)
	}
	if w.version(t) != v+3 {
		t.Fatalf("version %d, want %d", w.version(t), v+3)
	}
	if n := len(w.allowsOf(t, "grant.expire", audit)); n != 5 {
		t.Fatalf("%d audit rows", n)
	}
	for _, id := range due {
		if w.reportedAt(t, "grants", id).IsZero() {
			t.Fatalf("grant %v not reported", id)
		}
	}
}

func TestExpirySweep_WorkerWasDown(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	res := w.sweepAt(t, g3Expiry.Add(90*24*time.Hour))
	if res.Grants != 1 || res.Memberships != 1 {
		t.Fatalf("result %+v", res)
	}
	if w.reportedAt(t, "grants", w.g3).IsZero() || w.reportedAt(t, "group_memberships", w.membershipOf(t, w.leser, w.sam)).IsZero() {
		t.Fatal("an old expiry was not reported")
	}
}

func TestExpirySweep_InvariantStillVerified(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	// Sam with trigger on Notizen, which the personal Assistent serves: I1 is broken already.
	w.rawGrant(t, "user", w.sam, w.notizen, time.Time{})
	w.clk.Set(leserExpiry.Add(sweepInterval))
	before := w.snapshot(t)
	_, err := w.expiry(t, 500).Sweep(context.Background())
	var inv *authz.InvariantError
	if !errors.As(err, &inv) {
		t.Fatalf("error %v, want the InvariantError", err)
	}
	mustOK(t, w.rec.Aggregator().Flush(context.Background()))
	after := w.snapshot(t)
	after.denies = before.denies
	if after != before {
		t.Fatalf("a rejected sweep left something behind:\nbefore %+v\nafter  %+v", before, after)
	}
	if !w.reportedAt(t, "group_memberships", w.membershipOf(t, w.leser, w.sam)).IsZero() {
		t.Fatal("the membership was marked")
	}
}

// expiringPayload is an AccessExpiringPayload.
type expiringPayload struct {
	Kind      string `json:"kind"`
	GrantID   string `json:"grant_id"`
	GroupID   string `json:"group_id"`
	SubjectID string `json:"subject_id"`
	ExpiresAt string `json:"expires_at"`
	Target    *struct {
		Kind         string   `json:"kind"`
		ID           string   `json:"id"`
		Level        string   `json:"level"`
		Capabilities []string `json:"capabilities"`
	} `json:"target"`
}

// expiring returns the access.expiring rows after seq whose payload names grant or group id
// (grant_id for a grant, group_id with kind group_membership for a membership), validated.
func (w *pgWorld) expiring(t *testing.T, seq int64, kind string, id ids.ID) map[ids.ID]expiringPayload {
	t.Helper()
	out := map[ids.ID]expiringPayload{}
	for _, r := range w.eventsOf(t, "access.expiring", seq) {
		validateAgainst(t, "AccessExpiringPayload", r.Payload)
		if r.ResourceKind != "principal" || r.Resource != r.Audience || r.Receive != "self" || r.ActorKind != "" {
			t.Fatalf("envelope %+v", r)
		}
		var p expiringPayload
		mustOK(t, json.Unmarshal([]byte(r.Payload), &p))
		match := (kind == "grant" && p.GrantID == id.String()) || (kind == "group_membership" && p.Kind == kind && p.GroupID == id.String())
		if !match {
			continue
		}
		if _, dup := out[r.Audience]; dup {
			t.Fatalf("two reminders for %v", r.Audience)
		}
		out[r.Audience] = p
	}
	return out
}

// recipients returns the keys of m, sorted.
func recipients(m map[ids.ID]expiringPayload) []ids.ID {
	var out []ids.ID
	for k := range m {
		out = append(out, k)
	}
	return domain.SortIDs(out)
}

func TestExpiryReminder_GrantOnceAtLead(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	seq := w.lastSeq(t)
	w.remindAt(t, g3Expiry.Add(-reminderLead-time.Second))
	if got := w.expiring(t, seq, "grant", w.g3); len(got) != 0 {
		t.Fatalf("reminded too early: %v", got)
	}
	w.remindAt(t, g3Expiry.Add(-reminderLead+time.Second))
	got := w.expiring(t, seq, "grant", w.g3)
	if want := domain.SortIDs([]ids.ID{w.robin, w.owner}); !slices.Equal(recipients(got), want) {
		t.Fatalf("recipients %v, want %v", recipients(got), want)
	}
	p := got[w.robin]
	if p.Kind != "grant" || p.SubjectID != w.robin.String() || p.GroupID != "" || p.Target == nil || p.Target.Kind != "channel" ||
		p.Target.ID != w.projektA.String() || p.Target.Level != "trigger" || !slices.Equal(p.Target.Capabilities, []string{"upload"}) ||
		p.ExpiresAt != "2026-12-31T00:00:00.000Z" {
		t.Fatalf("payload %+v", p)
	}
	seq = w.lastSeq(t)
	w.remindAt(t, g3Expiry.Add(-reminderLead+time.Minute))
	if n := len(w.eventsOf(t, "access.expiring", seq)); n != 0 {
		t.Fatalf("a second run sent %d reminders", n)
	}
}

func TestExpiryReminder_ShortGrantOnNextRun(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	w.clk.Set(pgStart.Add(72 * time.Hour))
	g := w.grantRow(t, domain.Subject{Kind: domain.SubjectAgent, PrincipalID: w.lotse}, domain.Target{Kind: domain.TargetInstance},
		domain.LevelRead, w.clk.Now().Add(time.Hour))
	seq := w.lastSeq(t)
	w.remindAt(t, w.clk.Now().Add(sweepInterval))
	got := w.expiring(t, seq, "grant", g)
	if want := domain.SortIDs([]ids.ID{w.lotse, w.owner}); !slices.Equal(recipients(got), want) {
		t.Fatalf("recipients %v, want %v", recipients(got), want)
	}
	if got[w.lotse].Target != nil {
		t.Fatal("an instance grant carries a target")
	}
	// A host grant omits the target too (07 section 9.4); hosts come with M2-39.
	host := w.grantRow(t, domain.Subject{Kind: domain.SubjectUser, PrincipalID: w.alex}, domain.Target{Kind: domain.TargetHost, HostID: ids.New()},
		domain.LevelNone, w.clk.Now().Add(time.Hour), domain.CapTerminal)
	seq = w.lastSeq(t)
	w.remindAt(t, w.clk.Now().Add(sweepInterval))
	if p, ok := w.expiring(t, seq, "grant", host)[w.alex]; !ok || p.Target != nil {
		t.Fatalf("host reminder %+v, %v", p, ok)
	}
}

func TestExpiryReminder_GroupGrantReachesOwnersAndGranter(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	exp := pgStart.Add(72 * time.Hour)
	gg := w.grantRow(t, domain.Subject{Kind: domain.SubjectGroup, GroupID: w.leser}, domain.Target{Kind: domain.TargetSpace, SpaceID: w.marketing},
		domain.LevelRead, exp)
	// A second owner, added as a fixture after the grant: the reminder is no rights change, so
	// I1 (an owner triggers the personal Assistent) does not concern it.
	w.makeOwner(t, w.alex)
	seq := w.lastSeq(t)
	w.remindAt(t, exp.Add(-reminderLead+time.Second))
	got := w.expiring(t, seq, "grant", gg)
	if want := domain.SortIDs([]ids.ID{w.owner, w.alex}); !slices.Equal(recipients(got), want) {
		t.Fatalf("recipients %v, want %v", recipients(got), want)
	}
	if p := got[w.alex]; p.SubjectID != w.leser.String() || p.GroupID != w.leser.String() || p.Target == nil || p.Target.Kind != "space" {
		t.Fatalf("payload %+v", p)
	}
}

func TestExpiryReminder_MembershipReachesMemberAndAdder(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	seq := w.lastSeq(t)
	w.remindAt(t, pgStart.Add(time.Second))
	got := w.expiring(t, seq, "group_membership", w.leser)
	if want := domain.SortIDs([]ids.ID{w.sam, w.owner}); !slices.Equal(recipients(got), want) {
		t.Fatalf("recipients %v, want %v", recipients(got), want)
	}
	if p := got[w.sam]; p.SubjectID != w.sam.String() || p.GrantID != "" || p.Target != nil {
		t.Fatalf("payload %+v", p)
	}
}

func TestExpiryReminder_DeduplicatesAndSkipsInactive(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	exp := pgStart.Add(72 * time.Hour)
	// The owner grants itself: subject and granter are one recipient.
	self := w.grantRow(t, domain.Subject{Kind: domain.SubjectUser, PrincipalID: w.owner}, domain.Target{Kind: domain.TargetSpace, SpaceID: w.resea},
		domain.LevelRead, exp)
	// Robin is disabled: only the granter is reminded.
	robins := w.grantRow(t, domain.Subject{Kind: domain.SubjectUser, PrincipalID: w.robin}, domain.Target{Kind: domain.TargetSpace, SpaceID: w.resea},
		domain.LevelRead, exp)
	_, err := w.db.SuperConn(t).Exec(context.Background(), `UPDATE principals SET status = 'disabled' WHERE id = $1`, w.robin.String())
	mustOK(t, err)
	seq := w.lastSeq(t)
	w.remindAt(t, exp.Add(-reminderLead+time.Second))
	if got := recipients(w.expiring(t, seq, "grant", self)); !slices.Equal(got, []ids.ID{w.owner}) {
		t.Fatalf("self grant recipients %v", got)
	}
	if got := recipients(w.expiring(t, seq, "grant", robins)); !slices.Equal(got, []ids.ID{w.owner}) {
		t.Fatalf("disabled subject recipients %v", got)
	}
}

// robinRemovals counts the access.changed rows after seq that remove target for Robin.
func (w *pgWorld) robinRemovals(t *testing.T, seq int64, target ids.ID) int {
	t.Helper()
	n := 0
	for _, ev := range w.eventsOf(t, "access.changed", seq) {
		if ev.Audience == w.robin && slices.Contains(removedIDs(t, ev.Payload), target.String()) {
			n++
		}
	}
	return n
}

// TestExpirySweep_MembershipAndGroupGrantTogether: Robin reaches Kampagnen only through a group
// whose membership and grant both expire before one run (red team M1-30 round 1). Exactly one
// access.changed removes Kampagnen, whichever row expires first.
func TestExpirySweep_MembershipAndGroupGrantTogether(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                string
		memberExp, grantExp time.Duration
	}{{"same instant", time.Hour, time.Hour}, {"membership first", time.Hour, 2 * time.Hour}, {"grant first", 2 * time.Hour, time.Hour}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w := newPGWorld(t)
			g := w.rawGroup(t, "Team", w.robin, pgStart.Add(tc.memberExp))
			w.grantRow(t, domain.Subject{Kind: domain.SubjectGroup, GroupID: g},
				domain.Target{Kind: domain.TargetChannel, ChannelID: w.kampagnen}, domain.LevelRead, pgStart.Add(tc.grantExp))
			seq := w.lastSeq(t)
			w.sweepAt(t, pgStart.Add(3*time.Hour))
			w.sweepAt(t, pgStart.Add(3*time.Hour+sweepInterval))
			if n := w.robinRemovals(t, seq, w.kampagnen); n != 1 {
				t.Fatalf("%d removals of Kampagnen for Robin, want 1", n)
			}
		})
	}
}

// TestExpirySweep_TwoGrantsOnOneTargetInTwoBatches: two expired grants give Robin the same
// channel and land in different batches; the removal is reported once.
func TestExpirySweep_TwoGrantsOnOneTargetInTwoBatches(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	exp := pgStart.Add(time.Hour)
	for range 2 {
		w.grantRow(t, domain.Subject{Kind: domain.SubjectUser, PrincipalID: w.robin},
			domain.Target{Kind: domain.TargetChannel, ChannelID: w.kampagnen}, domain.LevelRead, exp)
	}
	seq := w.lastSeq(t)
	w.clk.Set(exp.Add(sweepInterval))
	res, err := w.expiry(t, 1).Sweep(context.Background())
	mustOK(t, err)
	if res.Grants != 2 || res.Batches != 2 {
		t.Fatalf("result %+v", res)
	}
	if n := w.robinRemovals(t, seq, w.kampagnen); n != 1 {
		t.Fatalf("%d removals of Kampagnen for Robin, want 1", n)
	}
}

// The window between an expiry and the sweep that reports it (red team M1-30 round 2): a rights
// change in the window sees the expired but unreported row as the client still does, so the
// loss is reported exactly once, by the change or by the sweep.

// windowSweeps runs two sweeps after the expiry at exp.
func (w *pgWorld) windowSweeps(t *testing.T, exp time.Time) {
	t.Helper()
	w.sweepAt(t, exp.Add(sweepInterval))
	w.sweepAt(t, exp.Add(2*sweepInterval))
}

func TestExpiryWindow_RevokeAfterExpiry(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	seq := w.lastSeq(t)
	w.clk.Set(g3Expiry.Add(10 * time.Second))
	_, err := w.admin.RevokeGrant(context.Background(), w.user(w.owner), w.g3, "")
	mustOK(t, err)
	if n := w.robinRemovals(t, seq, w.projektA); n != 1 {
		t.Fatalf("the revocation wrote %d removals, want 1", n)
	}
	w.windowSweeps(t, g3Expiry)
	if n := w.robinRemovals(t, seq, w.projektA); n != 1 {
		t.Fatalf("%d removals of Projekt-A for Robin, want 1", n)
	}
}

func TestExpiryWindow_GroupGrantRevokedAfterMembershipExpiry(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	exp := pgStart.Add(time.Hour)
	g := w.rawGroup(t, "Team", w.robin, exp)
	gr := w.grantRow(t, domain.Subject{Kind: domain.SubjectGroup, GroupID: g},
		domain.Target{Kind: domain.TargetChannel, ChannelID: w.kampagnen}, domain.LevelRead, time.Time{})
	seq := w.lastSeq(t)
	w.clk.Set(exp.Add(10 * time.Second))
	_, err := w.admin.RevokeGrant(context.Background(), w.user(w.owner), gr, "")
	mustOK(t, err)
	w.windowSweeps(t, exp)
	if n := w.robinRemovals(t, seq, w.kampagnen); n != 1 {
		t.Fatalf("%d removals of Kampagnen for Robin, want 1", n)
	}
}

func TestExpiryWindow_MemberRemovedAfterGroupGrantExpiry(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	exp := pgStart.Add(time.Hour)
	g := w.rawGroup(t, "Team", w.robin, time.Time{})
	w.grantRow(t, domain.Subject{Kind: domain.SubjectGroup, GroupID: g},
		domain.Target{Kind: domain.TargetChannel, ChannelID: w.kampagnen}, domain.LevelRead, exp)
	seq := w.lastSeq(t)
	w.clk.Set(exp.Add(10 * time.Second))
	mustOK(t, w.admin.RemoveGroupMember(context.Background(), w.user(w.owner), g, w.robin))
	w.windowSweeps(t, exp)
	if n := w.robinRemovals(t, seq, w.kampagnen); n != 1 {
		t.Fatalf("%d removals of Kampagnen for Robin, want 1", n)
	}
}

func TestExpiryWindow_NewGrantAfterExpiry(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	exp := pgStart.Add(time.Hour)
	old := w.grantRow(t, domain.Subject{Kind: domain.SubjectUser, PrincipalID: w.robin},
		domain.Target{Kind: domain.TargetChannel, ChannelID: w.kampagnen}, domain.LevelWrite, exp)
	seq, audit := w.lastSeq(t), w.auditSeq(t)
	w.clk.Set(exp.Add(10 * time.Second))
	w.grantRow(t, domain.Subject{Kind: domain.SubjectUser, PrincipalID: w.robin},
		domain.Target{Kind: domain.TargetChannel, ChannelID: w.kampagnen}, domain.LevelRead, time.Time{})
	w.windowSweeps(t, exp)
	// The new grant folds the expiry in: one access.changed from write to read, and the expiry is
	// reported with its event and audit entry by that change.
	evs := w.eventsOf(t, "access.changed", seq)
	if len(evs) != 1 || evs[0].Audience != w.robin {
		t.Fatalf("access.changed %+v", evs)
	}
	p := decodePayload(t, evs[0].Payload)
	if len(p.Changed) != 1 || p.Changed[0].ID != w.kampagnen.String() || p.Changed[0].Level != "read" || p.Changed[0].PreviousLevel != "write" {
		t.Fatalf("payload %s", evs[0].Payload)
	}
	if gc := w.eventsOf(t, "grant.changed", seq); len(gc) != 1 || gc[0].Resource != old || len(w.allowsOf(t, "grant.expire", audit)) != 1 {
		t.Fatalf("grant.changed %+v", gc)
	}
}

// TestExpiryWindow_NoPhantomGain: Robin's membership in Team expired unreported; a new grant to
// Team reports no gain to Robin, because authorize denies it (red team M1-30 round 3).
func TestExpiryWindow_NoPhantomGain(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	exp := pgStart.Add(time.Hour)
	g := w.rawGroup(t, "Team", w.robin, exp)
	seq := w.lastSeq(t)
	w.clk.Set(exp.Add(10 * time.Second))
	_, err := w.admin.CreateGrant(context.Background(), w.user(w.owner), authz.CreateGrantCmd{
		SubjectKind: "group", SubjectID: g, TargetKind: "channel", TargetID: w.kampagnen, Level: "read", Reason: "campaign",
	})
	mustOK(t, err)
	w.windowSweeps(t, exp)
	for _, ev := range w.eventsOf(t, "access.changed", seq) {
		if ev.Audience == w.robin {
			t.Fatalf("Robin was told %s", ev.Payload)
		}
	}
	if gc := w.eventsOf(t, "group.changed", seq); len(gc) != 1 || !strings.Contains(gc[0].Payload, "member_expired") {
		t.Fatalf("group.changed %+v", gc)
	}
}

// TestExpiryWindow_GroupDeleteAndReAdd: deleting the group after the membership expired reports
// the loss once; adding the person again in the window reports nothing.
func TestExpiryWindow_GroupDeleteAndReAdd(t *testing.T) {
	t.Parallel()
	for _, readd := range []bool{false, true} {
		w := newPGWorld(t)
		exp := pgStart.Add(time.Hour)
		g := w.rawGroup(t, "Team", w.robin, exp)
		w.grantRow(t, domain.Subject{Kind: domain.SubjectGroup, GroupID: g},
			domain.Target{Kind: domain.TargetChannel, ChannelID: w.kampagnen}, domain.LevelRead, time.Time{})
		seq := w.lastSeq(t)
		w.clk.Set(exp.Add(10 * time.Second))
		want := 1
		if readd {
			_, err := w.admin.AddGroupMember(context.Background(), w.user(w.owner), g, authz.AddMemberCmd{UserID: w.robin})
			mustOK(t, err)
			want = 0
		} else {
			mustOK(t, w.admin.DeleteGroup(context.Background(), w.user(w.owner), g, ""))
		}
		w.windowSweeps(t, exp)
		if n := w.robinRemovals(t, seq, w.kampagnen); n != want {
			t.Fatalf("readd=%v: %d removals, want %d", readd, n, want)
		}
		if readd && len(w.eventsOf(t, "access.changed", seq)) != 0 {
			t.Fatalf("re-adding told something: %+v", w.eventsOf(t, "access.changed", seq))
		}
	}
}

// TestExpiryWindow_FoldAuditIsSystem: an expiry folded into a person's rights change is audited
// as the system action it is: no actor and no IP pseudonym (05 R-AZ-15, red team M1-30 round 4).
func TestExpiryWindow_FoldAuditIsSystem(t *testing.T) {
	t.Parallel()
	w := newPGWorld(t)
	exp := pgStart.Add(time.Hour)
	old := w.grantRow(t, domain.Subject{Kind: domain.SubjectUser, PrincipalID: w.robin},
		domain.Target{Kind: domain.TargetChannel, ChannelID: w.kampagnen}, domain.LevelRead, exp)
	g := w.rawGroup(t, "Team", w.robin, exp)
	audit := w.auditSeq(t)
	w.clk.Set(exp.Add(10 * time.Second))
	ctx := reqmeta.WithClientIP(context.Background(), netip.MustParseAddr("192.0.2.1"))
	created, err := w.admin.CreateGrant(ctx, w.user(w.owner), authz.CreateGrantCmd{
		SubjectKind: "group", SubjectID: g, TargetKind: "channel", TargetID: w.kampagnen, Level: "read", Reason: "campaign",
	})
	mustOK(t, err)
	expired := append(w.allowsOf(t, "grant.expire", audit), w.allowsOf(t, "group.member.expire", audit)...)
	if len(expired) != 2 || expired[0].ResourceID != old || expired[1].ResourceID != g {
		t.Fatalf("expiry audit %+v", expired)
	}
	for _, r := range expired {
		if !r.Actor.IsZero() || strings.Contains(r.Details, "ip_hmac") {
			t.Fatalf("a folded expiry is audited as a person's action: %+v", r)
		}
	}
	// The change itself keeps its actor and IP pseudonym.
	rows := w.allowsOf(t, "grant.create", audit)
	if len(rows) != 1 || rows[0].ResourceID != created.ID || rows[0].Actor != w.owner || !strings.Contains(rows[0].Details, "ip_hmac") {
		t.Fatalf("grant.create audit %+v", rows)
	}
}
