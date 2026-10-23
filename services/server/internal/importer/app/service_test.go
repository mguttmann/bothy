// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/importer"
	"github.com/OWNER/bothy/services/server/internal/importer/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// mapping is the fixture mapping of the unit tests.
func (e *env) mapping() json.RawMessage {
	m := map[string]any{
		"version": 1, "guild_id": "1",
		"defaults": map[string]any{"unmapped_human": "placeholder", "unmapped_bot": "skip", "uncategorized_space": "10"},
		"spaces": []any{
			map[string]any{"discord_category": "10", "target": map[string]any{"create": map[string]any{"name": "Imported"}}},
			map[string]any{"discord_category": "11", "target": map[string]any{"existing": e.space.String()}},
			map[string]any{"discord_category": "12", "target": "skip"},
		},
		"channels": []any{
			map[string]any{"discord_channel": "20", "target": map[string]any{"create": map[string]any{"name": "tickets"}}, "planned_operator": "agent:lotse"},
			map[string]any{"discord_channel": "21", "target": map[string]any{"create": map[string]any{"name": "forum"}}, "lead": "bothy"},
			map[string]any{"discord_channel": "22", "target": map[string]any{"existing": e.channel.String()}},
			map[string]any{"discord_channel": "23", "target": "skip"},
		},
		"users": []any{
			map[string]any{"discord_user": "30", "target": "user:alex"},
			map[string]any{"discord_user": "31", "target": "agent:lotse"},
		},
	}
	b, _ := json.Marshal(m)
	return b
}

func (e *env) create(t *testing.T) importer.Source {
	t.Helper()
	s, err := e.svc.CreateSource(context.Background(), e.owner, importer.CreateSourceCmd{Kind: "discord", Name: "Old", Config: e.mapping()})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func (e *env) running(t *testing.T) importer.Source {
	t.Helper()
	s := e.create(t)
	s, err := e.svc.Start(context.Background(), e.owner, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func o(key, kind, data string) importer.BatchObject {
	return importer.BatchObject{OriginKey: key, ItemKind: kind, Data: json.RawMessage(data)}
}

func structureBatch() importer.Batch {
	return importer.Batch{Items: []importer.BatchObject{
		o("space:10", "space", `{"name":"Imported","sort_key":"a0"}`),
		o("channel:20", "channel", `{"space_origin_key":"space:10","name":"tickets","kind":"text","sort_key":"a0"}`),
		o("channel:21", "channel", `{"name":"forum","kind":"forum","sort_key":"a1"}`),
		o("forum_tag:21:5", "forum_tag", `{"channel_origin_key":"channel:21","name":"Bug"}`),
		o("user:30", "user", `{"discord_kind":"user","username":"alex"}`),
		o("user:31", "user", `{"discord_kind":"bot","username":"lotse"}`),
		o("user:32", "user", `{"discord_kind":"user","username":"Sam","global_name":"Sam E."}`),
		o("user:33", "user", `{"discord_kind":"bot","username":"other"}`),
		o("user:34", "user", `{"discord_kind":"webhook","username":"hook"}`),
	}}
}

func byKey(items []importer.Item) map[string]importer.Item {
	m := map[string]importer.Item{}
	for _, it := range items {
		m[it.OriginKey] = it
	}
	return m
}

func TestNew(t *testing.T) {
	t.Parallel()
	if _, err := app.New(testConfig(), app.Deps{}); err == nil {
		t.Error("missing collaborators accepted")
	}
	e := newEnv(t)
	if _, err := app.New(app.Config{}, app.Deps{Store: e.store, UoW: uow{}, Authorizer: e.az, Audit: e.rec, Events: e.events,
		Structure: e.structure, People: e.people, Cursors: cursors{}}); err == nil {
		t.Error("limits below 1 accepted")
	}
	if _, err := app.NewGuard(nil); err == nil {
		t.Error("guard without store")
	}
}

// TestCreateSource covers design note 2 over the fakes: a valid mapping is stored normalized with
// audit and import.progress; violations of the references are all reported; archive is
// not_enabled; a denied caller writes nothing.
func TestCreateSource(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	s := e.create(t)
	if s.State != importer.StateDraft || !strings.Contains(string(s.Config), `"lead":"discord"`) {
		t.Errorf("source %+v %s", s, s.Config)
	}
	if len(e.rec.entries) != 1 || len(e.events.evs) != 1 || e.events.evs[0].Type != "import.progress" || len(*e.published) != 1 {
		t.Errorf("audit %d, events %+v", len(e.rec.entries), e.events.evs)
	}
	bad := `{"version":1,"guild_id":"1","defaults":{"unmapped_human":"skip","unmapped_bot":"skip","uncategorized_space":"skip"},
		"spaces":[{"discord_category":"1","target":{"existing":"` + ids.New().String() + `"}}],
		"channels":[{"discord_channel":"2","target":"skip","planned_operator":"agent:alex"}],
		"users":[{"discord_user":"3","target":"user:sam"},{"discord_user":"4","target":"user:nobody"}]}`
	_, err := e.svc.CreateSource(context.Background(), e.owner, importer.CreateSourceCmd{Kind: "discord", Name: "x", Config: json.RawMessage(bad)})
	ee := fieldsOf(t, err)
	for _, f := range []struct{ p, c string }{
		{"/config/spaces/0/target/existing", "unknown_space"}, {"/config/channels/0/planned_operator", "unknown_handle"},
		{"/config/users/0/target", "not_active"}, {"/config/users/1/target", "unknown_handle"},
	} {
		if !hasField(ee, f.p, f.c) {
			t.Errorf("missing %s %s in %+v", f.p, f.c, ee.Fields)
		}
	}
	_, err = e.svc.CreateSource(context.Background(), e.owner, importer.CreateSourceCmd{Kind: "archive", Name: "x", Config: json.RawMessage(`{}`)})
	if ee := fieldsOf(t, err); !hasField(ee, "/kind", "not_enabled") {
		t.Errorf("%+v", ee.Fields)
	}
	_, err = e.svc.CreateSource(context.Background(), e.owner, importer.CreateSourceCmd{Kind: "discord", Name: " ", Config: e.mapping()})
	if ee := fieldsOf(t, err); !hasField(ee, "/name", "required") {
		t.Errorf("%+v", ee.Fields)
	}
	_, err = e.svc.CreateSource(context.Background(), e.owner, importer.CreateSourceCmd{Kind: "discord", Name: "a\u202eb", Config: e.mapping()})
	if ee := fieldsOf(t, err); !hasField(ee, "/name", "invalid") {
		t.Errorf("%+v", ee.Fields)
	}
	e.az.deny[authorize.ActionImportManage] = authorize.Decision{Outcome: authorize.OutcomeStepUpRequired, Reason: "step_up_required"}
	_, err = e.svc.CreateSource(context.Background(), e.owner, importer.CreateSourceCmd{Kind: "discord", Name: "x", Config: e.mapping()})
	wantErr(t, err, errs.CodeStepUpRequired, "")
}

// TestTransitions covers design note 3 over the fakes.
func TestTransitions(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	s := e.create(t)
	_, err := e.svc.Pause(ctx, e.owner, s.ID)
	wantErr(t, err, errs.CodeConflict, importer.ReasonImportState)
	_, err = e.svc.Start(ctx, e.owner, ids.New())
	wantErr(t, err, errs.CodeNotFound, "")
	s, err = e.svc.Start(ctx, e.owner, s.ID)
	if err != nil || s.State != importer.StateRunning || s.StartedAt.IsZero() {
		t.Fatalf("start %+v %v", s, err)
	}
	evs := len(e.events.evs)
	if s, err = e.svc.Start(ctx, e.owner, s.ID); err != nil || len(e.events.evs) != evs {
		t.Fatalf("repeat %+v %v", s, err)
	}
	if last := e.rec.entries[len(e.rec.entries)-1]; last.Attrs["no_change"].Data() != true {
		t.Errorf("no_change missing: %+v", last)
	}
	if _, err = e.svc.ApplyBatch(ctx, e.owner, s.ID, structureBatch()); err != nil {
		t.Fatal(err)
	}
	_, err = e.svc.Complete(ctx, e.owner, s.ID)
	wantErr(t, err, errs.CodeConflict, importer.ReasonLeadNotHere)
	if _, err = e.svc.SetLead(ctx, e.owner, s.ID, importer.LeadCmd{SourceChannel: "20", Lead: "bothy"}); err != nil {
		t.Fatal(err)
	}
	if s, err = e.svc.Complete(ctx, e.owner, s.ID); err != nil || s.State != importer.StateCompleted || s.FinishedAt.IsZero() {
		t.Fatalf("complete %+v %v", s, err)
	}
	other := e.create(t)
	if other, err = e.svc.Cancel(ctx, e.owner, other.ID); err != nil || other.State != importer.StateFailed ||
		!strings.Contains(string(other.Stats), `"cancelled":true`) {
		t.Fatalf("cancel %+v %v", other, err)
	}
}

// TestApplyBatch covers design note 5 over the fakes: targets, skips, mapped people, placeholders
// with the counter, replay without event, source_changed and the counters.
func TestApplyBatch(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	s := e.running(t)
	res, err := e.svc.ApplyBatch(ctx, e.owner, s.ID, structureBatch())
	if err != nil {
		t.Fatal(err)
	}
	got := byKey(res.Items)
	if got["user:30"].TargetID != e.alex || got["user:31"].TargetID != e.lotse || got["user:31"].TargetKind != "agent" ||
		got["user:33"].State != importer.ItemSkipped || got["user:32"].State != importer.ItemPosted || got["forum_tag:21:5"].State != importer.ItemPosted {
		t.Errorf("items %+v", got)
	}
	if len(e.people.created) != 2 || e.people.created[0].Handle != "imported-sam-2" || e.people.created[0].DisplayName != "Sam E." ||
		e.people.created[1].Handle != "imported-hook" {
		t.Errorf("placeholders %+v", e.people.created)
	}
	ch := e.structure.channels[got["channel:20"].TargetID]
	if !e.structure.readOnly[got["channel:20"].TargetID] || e.structure.readOnly[got["channel:21"].TargetID] || ch.SpaceID != got["space:10"].TargetID {
		t.Errorf("channels %+v %+v", ch, e.structure.readOnly)
	}
	evs := len(e.events.evs)
	again, err := e.svc.ApplyBatch(ctx, e.owner, s.ID, structureBatch())
	if err != nil {
		t.Fatal(err)
	}
	for i, it := range again.Items {
		if it.ID != res.Items[i].ID || it.TargetID != res.Items[i].TargetID {
			t.Errorf("replay %d: %+v then %+v", i, res.Items[i], it)
		}
	}
	if len(e.events.evs) != evs {
		t.Errorf("a replay appended events")
	}
	b := importer.Batch{Items: []importer.BatchObject{o("space:12", "space", `{"name":"x","sort_key":"a0"}`)}}
	b.Items[0].ContentSHA256 = strings.Repeat("a", 64)
	if _, err := e.svc.ApplyBatch(ctx, e.owner, s.ID, b); err != nil {
		t.Fatal(err)
	}
	b.Items[0].ContentSHA256 = strings.Repeat("b", 64)
	r, err := e.svc.ApplyBatch(ctx, e.owner, s.ID, b)
	if err != nil || !r.Items[0].SourceChanged || r.Items[0].State != importer.ItemSkipped {
		t.Errorf("source changed %+v %v", r, err)
	}
	cur, _ := e.svc.GetSource(ctx, e.owner, s.ID)
	if !strings.Contains(string(cur.Stats), `"source_changed":1`) || !strings.Contains(string(cur.Stats), `"channel":{"posted":2}`) {
		t.Errorf("stats %s", cur.Stats)
	}
	if o, found, err := e.svc.SourceOfTarget(ctx, "channel", got["channel:20"].TargetID); err != nil || !found || o.OriginKey != "channel:20" {
		t.Errorf("SourceOfTarget %+v %v %v", o, found, err)
	}
	g, _ := app.NewGuard(e.store)
	if c, err := g.ImportControlled(ctx, got["channel:20"].TargetID); err != nil || !c {
		t.Errorf("guard %v %v", c, err)
	}
}

// TestApplyBatchRefusals: not running, parse errors, outside targets, adoption and collisions.
func TestApplyBatchRefusals(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	draft := e.create(t)
	_, err := e.svc.ApplyBatch(ctx, e.owner, draft.ID, structureBatch())
	wantErr(t, err, errs.CodeConflict, importer.ReasonImportNotRunning)
	if len(e.rec.denied) != 1 {
		t.Errorf("denials %+v", e.rec.denied)
	}
	_, err = e.svc.SetCursor(ctx, e.owner, draft.ID, importer.CursorCmd{Cursor: json.RawMessage(`{}`)})
	wantErr(t, err, errs.CodeConflict, importer.ReasonImportNotRunning)
	s := e.running(t)
	_, err = e.svc.ApplyBatch(ctx, e.owner, s.ID, importer.Batch{ThreadOriginKeySet: true, Items: []importer.BatchObject{
		o("message:1", "message", `{}`), o("space:x", "space", `{"sort_key":"a0","name":"a"}`)}})
	ee := fieldsOf(t, err)
	if !hasField(ee, "/thread_origin_key", "not_enabled") || !hasField(ee, "/items/0/item_kind", "not_enabled") || !hasField(ee, "/items/1/origin_key", "invalid") {
		t.Errorf("%+v", ee.Fields)
	}
	for _, bad := range []importer.BatchObject{
		o("space:99", "space", `{"name":"x","sort_key":"a0"}`),
		o("space:11", "space", `{"existing_space_id":"`+ids.New().String()+`","sort_key":"a0"}`),
		o("space:10", "space", `{"existing_space_id":"`+e.space.String()+`","sort_key":"a0"}`),
		o("channel:99", "channel", `{"name":"x","kind":"text","sort_key":"a0"}`),
		o("channel:22", "channel", `{"existing_channel_id":"`+ids.New().String()+`","kind":"text","sort_key":"a0"}`),
		o("channel:20", "channel", `{"existing_channel_id":"`+e.channel.String()+`","kind":"text","sort_key":"a0"}`),
		o("channel:20", "channel", `{"space_origin_key":"space:10","name":"tickets","kind":"text","sort_key":"a0"}`),
		o("forum_tag:21:1", "forum_tag", `{"channel_origin_key":"channel:21","name":"x"}`),
		o("forum_tag:21:1", "forum_tag", `{"channel_origin_key":"channel:20","name":"x"}`),
	} {
		_, err := e.svc.ApplyBatch(ctx, e.owner, s.ID, importer.Batch{Items: []importer.BatchObject{bad}})
		if ee, ok := errs.As(err); !ok || ee.Reason != importer.ReasonTargetOutsideSource {
			t.Errorf("%s %s: %v", bad.OriginKey, bad.Data, err)
		}
	}
	adopt := importer.Batch{Items: []importer.BatchObject{
		o("space:11", "space", `{"existing_space_id":"`+e.space.String()+`","sort_key":"a0"}`),
		o("channel:22", "channel", `{"existing_channel_id":"`+e.channel.String()+`","kind":"text","sort_key":"a0"}`),
		o("channel:23", "channel", `{"name":"x","kind":"text","sort_key":"a0"}`),
	}}
	r, err := e.svc.ApplyBatch(ctx, e.owner, s.ID, adopt)
	if err != nil || r.Items[0].TargetID != e.space || r.Items[1].TargetID != e.channel || r.Items[2].State != importer.ItemSkipped ||
		!e.structure.readOnly[e.channel] {
		t.Fatalf("adopt %+v %v", r, err)
	}
	_, err = e.svc.ApplyBatch(ctx, e.owner, s.ID, importer.Batch{Items: []importer.BatchObject{o("space:10", "space", `{"name":"Imported","sort_key":"!"}`)}})
	if ee := fieldsOf(t, err); !hasField(ee, "/items/0/data/sort_key", "invalid") {
		t.Errorf("%+v", ee.Fields)
	}
}

// TestSetLeadAndCursor covers design notes 4 and 10 over the fakes.
func TestSetLeadAndCursor(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	s := e.running(t)
	_, err := e.svc.SetLead(ctx, e.owner, s.ID, importer.LeadCmd{SourceChannel: "20", Lead: "bothy"})
	wantErr(t, err, errs.CodeConflict, importer.ReasonChannelNotImported)
	_, err = e.svc.SetLead(ctx, e.owner, s.ID, importer.LeadCmd{SourceChannel: "23", Lead: "bothy"})
	wantErr(t, err, errs.CodeNotFound, "")
	res, err := e.svc.ApplyBatch(ctx, e.owner, s.ID, structureBatch())
	if err != nil {
		t.Fatal(err)
	}
	target := byKey(res.Items)["channel:20"].TargetID
	if _, err := e.svc.SetLead(ctx, e.owner, s.ID, importer.LeadCmd{SourceChannel: "20", Lead: "bothy"}); err != nil || e.structure.readOnly[target] {
		t.Fatalf("lead here %v %v", err, e.structure.readOnly[target])
	}
	evs := len(e.events.evs)
	if _, err := e.svc.SetLead(ctx, e.owner, s.ID, importer.LeadCmd{SourceChannel: "20", Lead: "bothy"}); err != nil || len(e.events.evs) != evs {
		t.Fatalf("equal lead %v", err)
	}
	cur, err := e.svc.SetCursor(ctx, e.owner, s.ID, importer.CursorCmd{Cursor: json.RawMessage(`{"run":{"id":"a"}}`), Stats: json.RawMessage(`{"messages":2}`)})
	if err != nil || string(cur.Cursor) != `{"run":{"id":"a"}}` || !strings.Contains(string(cur.Stats), `"tool":{"messages":2}`) {
		t.Fatalf("cursor %+v %v", cur, err)
	}
	_, err = e.svc.SetCursor(ctx, e.owner, s.ID, importer.CursorCmd{Cursor: json.RawMessage(`{}`), Stats: json.RawMessage(`[1]`)})
	wantErr(t, err, errs.CodeValidationFailed, "")
	if _, err := e.svc.Cancel(ctx, e.owner, s.ID); err != nil {
		t.Fatal(err)
	}
	_, err = e.svc.SetLead(ctx, e.owner, s.ID, importer.LeadCmd{SourceChannel: "20", Lead: "discord"})
	wantErr(t, err, errs.CodeConflict, importer.ReasonImportState)
}

// TestLists covers the pages of listImports and listImportItems.
func TestLists(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	var created []ids.ID
	for range 3 {
		created = append(created, e.create(t).ID)
		e.clk.Advance(1)
	}
	p, err := e.svc.ListSources(ctx, e.owner, importer.Page{})
	if err != nil || len(p.Items) != 2 || p.Items[0].ID != created[2] || p.NextCursor == "" {
		t.Fatalf("first page %+v %v", p, err)
	}
	p, err = e.svc.ListSources(ctx, e.owner, importer.Page{Cursor: p.NextCursor, Limit: 10})
	if err != nil || len(p.Items) != 1 || p.Items[0].ID != created[0] || p.NextCursor != "" {
		t.Fatalf("second page %+v %v", p, err)
	}
	_, err = e.svc.ListSources(ctx, e.owner, importer.Page{Cursor: "bad"})
	wantErr(t, err, errs.CodeCursorInvalid, "")
	s := e.running(t)
	if _, err := e.svc.ApplyBatch(ctx, e.owner, s.ID, structureBatch()); err != nil {
		t.Fatal(err)
	}
	seen, cursor := 0, ""
	for {
		ip, err := e.svc.ListItems(ctx, e.owner, s.ID, importer.ItemQuery{Page: importer.Page{Cursor: cursor}})
		if err != nil {
			t.Fatal(err)
		}
		seen += len(ip.Items)
		if cursor = ip.NextCursor; cursor == "" {
			break
		}
	}
	if seen != len(structureBatch().Items) {
		t.Errorf("listed %d items", seen)
	}
	ip, err := e.svc.ListItems(ctx, e.owner, s.ID, importer.ItemQuery{State: importer.ItemSkipped})
	if err != nil || len(ip.Items) != 1 {
		t.Errorf("skipped %+v %v", ip, err)
	}
	_, err = e.svc.ListItems(ctx, e.owner, ids.New(), importer.ItemQuery{})
	wantErr(t, err, errs.CodeNotFound, "")
}

// TestPlaceholderHandleRace (design note 8, red team finding M1): a handle that a concurrent batch
// takes between the check and the insert makes the next counter win instead of failing the batch.
func TestPlaceholderHandleRace(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.people.race = map[string]bool{"imported-hook": true, "imported-hook-2": true}
	s := e.running(t)
	r, err := e.svc.ApplyBatch(context.Background(), e.owner, s.ID, importer.Batch{Items: []importer.BatchObject{
		o("user:34", "user", `{"discord_kind":"webhook","username":"hook"}`)}})
	if err != nil || r.Items[0].State != importer.ItemPosted {
		t.Fatalf("batch %+v %v", r, err)
	}
	if len(e.people.created) != 1 || e.people.created[0].Handle != "imported-hook-3" {
		t.Errorf("placeholders %+v", e.people.created)
	}
}
