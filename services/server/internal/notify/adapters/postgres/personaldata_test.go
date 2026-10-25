// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	stdsync "sync"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
	"github.com/OWNER/bothy/services/server/internal/notify/app"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// sectionSchema is the path of the schema of section key of the object file with stem.
func sectionSchema(stem, key string) string {
	return filepath.Join("..", "..", "..", "..", "..", "..", "docs", "api", "gdpr-export", "v1", "sections", stem+"."+key+".schema.json")
}

// validateSchema checks raw against the JSON Schema at path with format assertions (uuid,
// date-time).
func validateSchema(t *testing.T, path string, raw []byte) {
	t.Helper()
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	sch, err := c.Compile(abs)
	if err != nil {
		t.Fatalf("schema %s: %v", path, err)
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if err := sch.Validate(inst); err != nil {
		t.Fatalf("%s does not hold %s: %v", path, raw, err)
	}
}

// recordingAppender is the appender of sync as far as the participant sees it: it refuses an
// append outside a unit of work and keeps every event in order.
type recordingAppender struct {
	mu     stdsync.Mutex
	events []eventlog.Event
}

func (a *recordingAppender) Append(ctx context.Context, events ...eventlog.Event) error {
	if _, err := pg.Tx(ctx); err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.events = append(a.events, events...)
	return nil
}

func (a *recordingAppender) all() []eventlog.Event {
	a.mu.Lock()
	defer a.mu.Unlock()
	return slices.Clone(a.events)
}

// pdWorld is one database with Alex (the subject) and Sam, each with two notification levels,
// one quiet-hour rule and two push registrations (one active, one disabled).
type pdWorld struct {
	env
	p          *app.PersonalData
	events     *recordingAppender
	alex, sam  ids.ID
	alexDevice ids.ID
	alexRegs   []ids.ID
}

func newPDWorld(t *testing.T) pdWorld {
	t.Helper()
	e := newEnv(t)
	w := pdWorld{env: e, events: &recordingAppender{}}
	p, err := app.NewPersonalData(app.PersonalDataDeps{Store: e.repo, Registrations: e.regs, UoW: e.uow, Events: w.events})
	if err != nil {
		t.Fatal(err)
	}
	w.p = p
	var alexSession, samDevice, samSession ids.ID
	w.alex, w.alexDevice, alexSession = e.person(t, "alex", "ios")
	w.sam, samDevice, samSession = e.person(t, "sam", "android")
	for _, u := range []ids.ID{w.alex, w.sam} {
		e.exec(t, `INSERT INTO notification_preferences (user_id, target_kind, target_id, level, muted_until)
			VALUES ($1, 'space', $2, 'mute', '2026-11-01T08:00:00Z'), ($1, 'thread', $3, 'all', NULL)`, u.String(), ids.New().String(), ids.New().String())
		e.exec(t, `INSERT INTO quiet_hours (id, user_id, weekdays, start_local, end_local, time_zone)
			VALUES ($1, $2, 31, '22:00', '07:00', 'Europe/Berlin')`, ids.New().String(), u.String())
	}
	w.alexRegs = []ids.ID{
		e.registration(t, w.alex, w.alexDevice, alexSession, "apns", "alex-token-1", ""),
		e.registration(t, w.alex, w.alexDevice, alexSession, "fcm", "alex-token-2", "logout"),
	}
	e.registration(t, w.sam, samDevice, samSession, "fcm", "sam-token-1", "")
	return w
}

// count returns the rows of user in table (a fixed table name of this test).
func (w pdWorld) count(t *testing.T, table string, user ids.ID) int {
	t.Helper()
	var n int
	if err := w.owner.QueryRow(context.Background(), "SELECT count(*) FROM "+table+" WHERE user_id = $1", user.String()).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// members returns the member names of every object of the array raw, sorted and joined.
func members(t *testing.T, raw json.RawMessage) []string {
	t.Helper()
	var list []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &list); err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(list))
	for _, o := range list {
		keys := make([]string, 0, len(o))
		for k := range o {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		out = append(out, strings.Join(keys, ","))
	}
	return out
}

// TestPersonalDataExport (M7-06, 20 section 5.2): personal.json gets Alex's two levels and one
// rule, devices.json Alex's two registrations with the documented members; nothing of Sam, never
// a token or a hash, each section valid against its schema, and nothing else is written.
func TestPersonalDataExport(t *testing.T) {
	t.Parallel()
	w := newPDWorld(t)
	mw := personaldata.NewMemoryWriter()
	if err := w.p.Export(context.Background(), personaldata.Export{Subject: w.alex}, mw); err != nil {
		t.Fatal(err)
	}
	if len(mw.Lines) != 0 || len(mw.Contents) != 0 || len(mw.Sections) != 2 || len(mw.Sections[personaldata.FilePersonal]) != 2 ||
		len(mw.Sections[personaldata.FileDevices]) != 1 {
		t.Fatalf("written: sections %v, lines %d, contents %d", mw.Sections, len(mw.Lines), len(mw.Contents))
	}
	prefs := mw.Sections[personaldata.FilePersonal]["notification_preferences"]
	rules := mw.Sections[personaldata.FilePersonal]["quiet_hours"]
	regs := mw.Sections[personaldata.FileDevices]["push_registrations"]
	validateSchema(t, sectionSchema("personal", "notification_preferences"), prefs)
	validateSchema(t, sectionSchema("personal", "quiet_hours"), rules)
	validateSchema(t, sectionSchema("devices", "push_registrations"), regs)

	wantMembers := func(name string, raw json.RawMessage, want ...string) {
		t.Helper()
		got := members(t, raw)
		slices.Sort(got)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("%s members %q, want %q", name, got, want)
		}
	}
	wantMembers("notification_preferences", prefs, "level,muted_until,target_id,target_kind,version", "level,target_id,target_kind,version")
	wantMembers("quiet_hours", rules, "enabled,end_local,id,start_local,time_zone,version,weekdays")
	wantMembers("push_registrations", regs, "app_id,created_at,device_id,environment,id,provider,state",
		"app_id,created_at,device_id,disabled_at,disabled_reason,environment,id,provider,state")

	all := string(prefs) + string(rules) + string(regs)
	for _, secret := range []string{"alex-token", "sam-token", "token", "hash"} {
		if strings.Contains(all, secret) {
			t.Errorf("the export holds %q: %s", secret, all)
		}
	}
	var regList []struct {
		ID       string `json:"id"`
		DeviceID string `json:"device_id"`
	}
	if err := json.Unmarshal(regs, &regList); err != nil {
		t.Fatal(err)
	}
	var gotIDs []string
	for _, r := range regList {
		gotIDs = append(gotIDs, r.ID)
		if r.DeviceID != w.alexDevice.String() {
			t.Errorf("registration %s on device %s, want Alex's", r.ID, r.DeviceID)
		}
	}
	slices.Sort(gotIDs)
	wantIDs := []string{w.alexRegs[0].String(), w.alexRegs[1].String()}
	slices.Sort(wantIDs)
	if !slices.Equal(gotIDs, wantIDs) {
		t.Errorf("registrations %v, want %v", gotIDs, wantIDs)
	}

	// A person without any row gets three empty arrays.
	empty := personaldata.NewMemoryWriter()
	if err := w.p.Export(context.Background(), personaldata.Export{Subject: ids.New()}, empty); err != nil {
		t.Fatal(err)
	}
	for f, keys := range map[personaldata.File][]string{
		personaldata.FilePersonal: {"notification_preferences", "quiet_hours"}, personaldata.FileDevices: {"push_registrations"},
	} {
		for _, k := range keys {
			if got := string(empty.Sections[f][k]); got != "[]" {
				t.Errorf("%s %s of nobody = %s, want []", f, k, got)
			}
		}
	}
}

// TestPersonalDataErase (M7-06, 20 section 4.4): run twice in full (the second run is the
// resume after a crash), step security removes Alex's registrations and step personal Alex's
// levels and rules, with one notification_pref.updated (deleted, version plus one) per level and
// one quiet_hours.updated with an empty list, all as the system; Sam keeps everything and the
// second run appends nothing. notify has no content step, so both content modes act alike.
func TestPersonalDataErase(t *testing.T) {
	t.Parallel()
	for _, mode := range []personaldata.ContentMode{personaldata.KeepPseudonymized, personaldata.DeleteContent} {
		t.Run(string(mode), func(t *testing.T) {
			t.Parallel()
			w := newPDWorld(t)
			ctx := context.Background()
			er := personaldata.Erasure{Subject: w.alex, RequestID: ids.New(), Content: mode}
			for _, step := range personaldata.ParticipantSteps() {
				if err := w.p.Erase(ctx, step, er); err != nil {
					t.Fatalf("step %s: %v", step, err)
				}
				switch step {
				case personaldata.StepSecurity:
					if n := w.count(t, "push_registrations", w.alex); n != 0 {
						t.Errorf("after security: %d registrations of Alex", n)
					}
					if n := w.count(t, "notification_preferences", w.alex); n != 2 {
						t.Errorf("after security: %d levels of Alex, want 2 (step personal removes them)", n)
					}
				case personaldata.StepPersonal:
					if n := w.count(t, "notification_preferences", w.alex) + w.count(t, "quiet_hours", w.alex); n != 0 {
						t.Errorf("after personal: %d levels and rules of Alex", n)
					}
				case personaldata.StepDisable, personaldata.StepAccess, personaldata.StepContent, personaldata.StepPseudonymize,
					personaldata.StepAudit:
				default:
				}
			}
			for table, want := range map[string]int{"notification_preferences": 2, "quiet_hours": 1, "push_registrations": 1} {
				if n := w.count(t, table, w.sam); n != want {
					t.Errorf("%s of Sam: %d, want %d", table, n, want)
				}
			}

			evs := w.events.all()
			var prefs, quiet int
			for _, ev := range evs {
				if ev.ActorKind != eventlog.ActorSystem || !ev.ActorPrincipalID.IsZero() || ev.AudiencePrincipalID != w.alex || ev.ResourceID != w.alex {
					t.Errorf("event %s: actor %s %s, audience %s, resource %s", ev.Type, ev.ActorKind, ev.ActorPrincipalID, ev.AudiencePrincipalID, ev.ResourceID)
				}
				switch ev.Type {
				case string(authorize.EventNotificationPrefUpdated):
					prefs++
					var p struct {
						Preference struct {
							Version int64 `json:"version"`
						} `json:"preference"`
						Deleted bool `json:"deleted"`
					}
					if err := json.Unmarshal(ev.Payload, &p); err != nil || !p.Deleted || p.Preference.Version != 2 {
						t.Errorf("notification_pref.updated %s (%v), want deleted with version 2", ev.Payload, err)
					}
				case string(authorize.EventQuietHoursUpdated):
					quiet++
					if string(ev.Payload) != `{"rules":[]}` {
						t.Errorf("quiet_hours.updated %s, want the empty list", ev.Payload)
					}
				default:
					t.Errorf("unexpected event %s", ev.Type)
				}
			}
			if prefs != 2 || quiet != 1 {
				t.Fatalf("events: %d notification_pref.updated, %d quiet_hours.updated; want 2 and 1", prefs, quiet)
			}

			for _, step := range personaldata.ParticipantSteps() {
				if err := w.p.Erase(ctx, step, er); err != nil {
					t.Fatalf("second run, step %s: %v", step, err)
				}
			}
			if n := len(w.events.all()); n != len(evs) {
				t.Errorf("the second run appended %d events", n-len(evs))
			}
		})
	}
}

// TestPersonalDataIdentity: the participant names notify and its three tables of 20 section 2,
// and a missing collaborator is refused at the start.
func TestPersonalDataIdentity(t *testing.T) {
	t.Parallel()
	if _, err := app.NewPersonalData(app.PersonalDataDeps{}); err == nil {
		t.Fatal("NewPersonalData without collaborators succeeded")
	}
	var p *app.PersonalData
	if p.Module() != "notify" || !slices.Equal(p.Tables(), []string{"notification_preferences", "quiet_hours", "push_registrations"}) {
		t.Fatalf("Module %q, Tables %v", p.Module(), p.Tables())
	}
}
