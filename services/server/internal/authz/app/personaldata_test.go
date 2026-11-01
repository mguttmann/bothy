// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// personalFixture is the pgWorld plus the access history of Alex (the subject) and Sam (another
// person) that the participant tests read and erase.
type personalFixture struct {
	*pgWorld
	p *app.PersonalData
	// readers is a live group with Alex and Sam as members; old is a group whose membership of
	// Alex expires one hour after the start.
	readers, old ids.ID
	// alexReaders and alexOld are open memberships of Alex; alexLeserGone is a membership of Alex
	// in Leser that was removed before; samReaders is Sam's open membership in readers.
	alexReaders, alexOld, alexLeserGone, samReaders ids.ID
	// alexRevoked is a grant of Alex revoked before; alexChannel is an open channel grant with an
	// expiry.
	alexRevoked, alexChannel ids.ID
}

// newPersonalFixture builds the world and the rows of Alex and Sam.
func newPersonalFixture(t *testing.T) *personalFixture {
	t.Helper()
	w := newPGWorld(t)
	f := &personalFixture{pgWorld: w}
	var err error
	f.p, err = app.NewPersonalData(app.PersonalDataDeps{
		UoW: w.uow, Store: w.store, History: w.store, Structure: w.structure, Events: w.appender, Clock: w.clk,
	})
	mustOK(t, err)
	group := func(name string) ids.ID {
		g, err := domain.NewGroup(ids.New(), name, "", domain.GroupLimits{}, w.owner, pgStart)
		mustOK(t, err)
		// group.create changes no rights (05 column Permissions): a plain unit.
		mustOK(t, w.uow.Do(context.Background(), func(ctx context.Context) error { return w.store.InsertGroup(ctx, g) }))
		return g.ID
	}
	member := func(group, user ids.ID, exp time.Time) ids.ID {
		m, err := domain.NewMembership(ids.New(), group, user, w.owner, exp, w.clk.Now())
		mustOK(t, err)
		mustOK(t, w.change(context.Background(), "group.member.add", txn.Ref{Kind: "group", ID: group}, []ids.ID{user}, func(ctx context.Context) error {
			return w.store.InsertMembership(ctx, m)
		}))
		return m.ID
	}
	f.readers, f.old = group("Readers"), group("Old")
	f.alexReaders = member(f.readers, w.alex, time.Time{})
	f.samReaders = member(f.readers, w.sam, time.Time{})
	f.alexOld = member(f.old, w.alex, pgStart.Add(time.Hour))
	f.alexLeserGone = member(w.leser, w.alex, time.Time{})
	mustOK(t, w.change(context.Background(), "group.member.remove", txn.Ref{Kind: "group", ID: w.leser}, []ids.ID{w.alex}, func(ctx context.Context) error {
		_, err := w.store.RemoveMembership(ctx, f.alexLeserGone, w.owner, w.clk.Now())
		return err
	}))
	user := domain.Subject{Kind: domain.SubjectUser, PrincipalID: w.alex}
	f.alexRevoked = w.grantRow(t, user, domain.Target{Kind: domain.TargetSpace, SpaceID: w.marketing}, domain.LevelRead, time.Time{})
	mustOK(t, w.change(context.Background(), "grant.revoke", txn.Ref{Kind: "grant", ID: f.alexRevoked}, []ids.ID{w.alex}, func(ctx context.Context) error {
		_, _, err := w.store.RevokeGrant(ctx, f.alexRevoked, w.owner, "moved to another team", w.clk.Now())
		return err
	}))
	f.alexChannel = w.grantRow(t, user, domain.Target{Kind: domain.TargetChannel, ChannelID: w.kampagnen}, domain.LevelWrite,
		pgStart.Add(30*24*time.Hour), domain.CapUpload)
	return f
}

// exportSchema compiles docs/api/gdpr-export/v1/sections/<name>.schema.json.
func exportSchema(t *testing.T, name string) *jsonschema.Schema {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(file), "../../../../../docs/api/gdpr-export/v1/sections", name+".schema.json")
	raw, err := os.ReadFile(filepath.Clean(path))
	mustOK(t, err)
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	mustOK(t, err)
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	mustOK(t, c.AddResource(name+".schema.json", doc))
	s, err := c.Compile(name + ".schema.json")
	mustOK(t, err)
	return s
}

// section validates the section key of access.json against its schema and returns its objects.
func section(t *testing.T, w *personaldata.MemoryWriter, key string) []map[string]any {
	t.Helper()
	raw, ok := w.Sections[personaldata.FileAccess][key]
	if !ok {
		t.Fatalf("access.json has no section %s: %v", key, w.Sections)
	}
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	mustOK(t, err)
	if err := exportSchema(t, "access."+key).Validate(v); err != nil {
		t.Fatalf("section %s does not match its schema: %v\n%s", key, err, raw)
	}
	if testkit.HasJSONNull(raw) {
		t.Fatalf("null in section %s: %s", key, raw)
	}
	var out []map[string]any
	mustOK(t, json.Unmarshal(raw, &out))
	return out
}

// idsOf returns the member id of every object, sorted.
func idsOf(rows []map[string]any) []string {
	var out []string
	for _, r := range rows {
		out = append(out, r["id"].(string))
	}
	slices.Sort(out)
	return out
}

// idStrings returns the strings of list, sorted.
func idStrings(list ...ids.ID) []string {
	var out []string
	for _, id := range list {
		out = append(out, id.String())
	}
	slices.Sort(out)
	return out
}

func TestPersonalDataExport(t *testing.T) {
	t.Parallel()
	f := newPersonalFixture(t)
	if f.p.Module() != "authz" {
		t.Fatalf("Module = %q", f.p.Module())
	}
	if got := f.p.Tables(); !slices.Equal(got, []string{"groups", "group_memberships", "grants"}) {
		t.Fatalf("Tables = %v", got)
	}
	w := personaldata.NewMemoryWriter()
	mustOK(t, f.p.Export(context.Background(), personaldata.Export{Subject: f.alex}, w))
	if keys := len(w.Sections[personaldata.FileAccess]); keys != 2 || len(w.Sections) != 1 || len(w.Lines) != 0 {
		t.Fatalf("written: %v %v", w.Sections, w.Lines)
	}

	grants := section(t, w, "grants")
	if got, want := idsOf(grants), idStrings(f.g1, f.alexRevoked, f.alexChannel); !slices.Equal(got, want) {
		t.Fatalf("grants = %v, want %v (only Alex's, revoked included)", got, want)
	}
	for _, g := range grants {
		switch g["id"] {
		case f.alexRevoked.String():
			if g["revoked_at"] == nil || g["revoked_by"] != f.owner.String() || g["revoke_reason"] != "moved to another team" {
				t.Fatalf("revoked grant: %v", g)
			}
		case f.alexChannel.String():
			if g["target_kind"] != "channel" || g["target_id"] != f.kampagnen.String() || g["level"] != "write" ||
				g["expires_at"] != pgStart.Add(30*24*time.Hour).Format(time.RFC3339) || g["revoked_at"] != nil {
				t.Fatalf("channel grant: %v", g)
			}
		case f.g1.String():
			if g["target_kind"] != "space" || g["target_id"] != f.support.String() || g["level"] != "trigger" ||
				g["reason"] != "setup" || g["granted_by"] != f.owner.String() || g["granted_at"] != pgStart.Format(time.RFC3339) {
				t.Fatalf("space grant: %v", g)
			}
			caps, _ := g["capabilities"].([]any)
			if len(caps) != 2 {
				t.Fatalf("capabilities: %v", g)
			}
		}
	}

	members := section(t, w, "group_memberships")
	if got, want := idsOf(members), idStrings(f.alexReaders, f.alexOld, f.alexLeserGone); !slices.Equal(got, want) {
		t.Fatalf("memberships = %v, want %v (only Alex's, removed included)", got, want)
	}
	for _, m := range members {
		switch m["id"] {
		case f.alexReaders.String():
			if m["group_id"] != f.readers.String() || m["group_name"] != "Readers" || m["added_by"] != f.owner.String() ||
				m["added_at"] != pgStart.Format(time.RFC3339) || m["removed_at"] != nil || m["expires_at"] != nil {
				t.Fatalf("open membership: %v", m)
			}
		case f.alexLeserGone.String():
			if m["group_name"] != "Leser" || m["removed_at"] == nil || m["removed_by"] != f.owner.String() {
				t.Fatalf("removed membership: %v", m)
			}
		case f.alexOld.String():
			if m["expires_at"] != pgStart.Add(time.Hour).Format(time.RFC3339) {
				t.Fatalf("expiring membership: %v", m)
			}
		}
	}

	// No row of Sam and nothing that names a secret, a hash or a key.
	all := string(w.Sections[personaldata.FileAccess]["grants"]) + string(w.Sections[personaldata.FileAccess]["group_memberships"])
	for _, bad := range []string{f.g2.String(), f.samReaders.String(), "hash", "secret", "token", "key"} {
		if strings.Contains(all, bad) {
			t.Fatalf("export contains %q: %s", bad, all)
		}
	}

	// A person without rows gets two empty arrays.
	empty := personaldata.NewMemoryWriter()
	mustOK(t, f.p.Export(context.Background(), personaldata.Export{Subject: f.robin}, empty))
	if g := string(empty.Sections[personaldata.FileAccess]["group_memberships"]); g != "[]" {
		t.Fatalf("Robin's memberships = %s", g)
	}
}

// accessRow is the state of a grant or a membership after the erasure.
type accessRow struct {
	closedAt *time.Time
	closedBy *string
	reason   *string
}

// grantState reads revoked_at, revoked_by and revoke_reason of a grant.
func (f *personalFixture) grantState(t *testing.T, id ids.ID) accessRow {
	t.Helper()
	var r accessRow
	mustOK(t, f.db.SuperConn(t).QueryRow(context.Background(),
		`SELECT revoked_at, revoked_by::text, revoke_reason FROM grants WHERE id = $1`, id.String()).Scan(&r.closedAt, &r.closedBy, &r.reason))
	return r
}

// membershipState reads removed_at and removed_by of a membership.
func (f *personalFixture) membershipState(t *testing.T, id ids.ID) accessRow {
	t.Helper()
	var r accessRow
	mustOK(t, f.db.SuperConn(t).QueryRow(context.Background(),
		`SELECT removed_at, removed_by::text FROM group_memberships WHERE id = $1`, id.String()).Scan(&r.closedAt, &r.closedBy))
	return r
}

func TestPersonalDataErase(t *testing.T) {
	t.Parallel()
	for _, mode := range []personaldata.ContentMode{personaldata.KeepPseudonymized, personaldata.DeleteContent} {
		t.Run(string(mode), func(t *testing.T) {
			t.Parallel()
			f := newPersonalFixture(t)
			// Alex's membership in Old has expired and is not reported yet: the erasure folds it in.
			f.clk.Set(pgStart.Add(2 * time.Hour))
			erasedAt := f.clk.Now()
			samG2, samReaders := f.grantState(t, f.g2), f.membershipState(t, f.samReaders)
			revokedBefore := f.grantState(t, f.alexRevoked)
			removedBefore := f.membershipState(t, f.alexLeserGone)
			seq := f.lastSeq(t)
			e := personaldata.Erasure{Subject: f.alex, RequestID: ids.New(), DecidedBy: f.owner, Content: mode}

			var afterFirst int64
			for run := range 2 {
				for _, step := range personaldata.ParticipantSteps() {
					mustOK(t, f.p.Erase(context.Background(), step, e))
				}
				if run == 0 {
					afterFirst = f.lastSeq(t)
				}
			}
			if f.lastSeq(t) != afterFirst {
				t.Fatalf("the second run appended events: %d after %d", f.lastSeq(t), afterFirst)
			}

			// Every open grant of Alex is revoked by the system; the rows stay.
			for _, id := range []ids.ID{f.g1, f.alexChannel} {
				r := f.grantState(t, id)
				if r.closedAt == nil || !r.closedAt.Equal(erasedAt) || r.closedBy != nil || r.reason == nil || *r.reason != "account_deleted" {
					t.Fatalf("grant %s after the erasure: %+v", id, r)
				}
			}
			// Every open membership of Alex is removed by the system, the expired one included.
			for _, id := range []ids.ID{f.alexReaders, f.alexOld} {
				r := f.membershipState(t, id)
				if r.closedAt == nil || !r.closedAt.Equal(erasedAt) || r.closedBy != nil {
					t.Fatalf("membership %s after the erasure: %+v", id, r)
				}
			}
			// Rows closed before and the rows of Sam are untouched.
			for name, pair := range map[string][2]accessRow{
				"revoked grant": {revokedBefore, f.grantState(t, f.alexRevoked)}, "removed membership": {removedBefore, f.membershipState(t, f.alexLeserGone)},
				"Sam's grant": {samG2, f.grantState(t, f.g2)}, "Sam's membership": {samReaders, f.membershipState(t, f.samReaders)},
			} {
				if !sameRow(pair[0], pair[1]) {
					t.Fatalf("%s changed: %+v -> %+v", name, pair[0], pair[1])
				}
			}

			// The events of a rights change, all by the system.
			grantsChanged := f.eventsOf(t, "grant.changed", seq)
			var revoked []string
			for _, ev := range grantsChanged {
				if ev.ActorKind != "system" {
					t.Fatalf("grant.changed with actor %q", ev.ActorKind)
				}
				var p struct {
					Change string `json:"change"`
				}
				mustOK(t, json.Unmarshal([]byte(ev.Payload), &p))
				if p.Change == "revoked" {
					revoked = append(revoked, ev.Resource.String())
				}
				validateAgainst(t, "GrantChangedPayload", ev.Payload)
			}
			slices.Sort(revoked)
			if want := idStrings(f.g1, f.alexChannel); !slices.Equal(revoked, want) {
				t.Fatalf("grant.changed revoked for %v, want %v", revoked, want)
			}
			changes := map[string]int{}
			for _, ev := range f.eventsOf(t, "group.changed", seq) {
				if ev.ActorKind != "system" {
					t.Fatalf("group.changed with actor %q", ev.ActorKind)
				}
				var p struct {
					Change string `json:"change"`
					UserID string `json:"user_id"`
				}
				mustOK(t, json.Unmarshal([]byte(ev.Payload), &p))
				if p.UserID != f.alex.String() {
					t.Fatalf("group.changed for %s", p.UserID)
				}
				changes[p.Change+" "+ev.Resource.String()]++
				validateAgainst(t, "GroupChangedPayload", ev.Payload)
			}
			if want := map[string]int{"member_removed " + f.readers.String(): 1, "member_expired " + f.old.String(): 1}; !mapsEqual(changes, want) {
				t.Fatalf("group.changed = %v, want %v", changes, want)
			}
			accessChanged := f.eventsOf(t, "access.changed", seq)
			if len(accessChanged) == 0 {
				t.Fatal("no access.changed")
			}
			for _, ev := range accessChanged {
				if ev.Audience != f.alex || ev.ActorKind != "system" {
					t.Fatalf("access.changed to %s by %q", ev.Audience, ev.ActorKind)
				}
			}
		})
	}
}

// sameRow reports equal states.
func sameRow(a, b accessRow) bool {
	eqT := (a.closedAt == nil) == (b.closedAt == nil) && (a.closedAt == nil || a.closedAt.Equal(*b.closedAt))
	eqS := func(x, y *string) bool { return (x == nil) == (y == nil) && (x == nil || *x == *y) }
	return eqT && eqS(a.closedBy, b.closedBy) && eqS(a.reason, b.reason)
}

// mapsEqual compares two count maps.
func mapsEqual(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
