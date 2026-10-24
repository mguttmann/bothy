// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/importer"
	"github.com/OWNER/bothy/services/server/internal/importer/adapters/httpapi"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

var now = time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)

// fake answers every call with fixed data and records the last command.
type fake struct {
	importer.Imports
	src   importer.Source
	batch importer.Batch
	lead  importer.LeadCmd
	cur   importer.CursorCmd
	query importer.ItemQuery
	fail  error
}

func (f *fake) ListSources(context.Context, actor.Actor, importer.Page) (importer.SourcePage, error) {
	return importer.SourcePage{Items: []importer.Source{f.src}, NextCursor: "next"}, f.fail
}

func (f *fake) CreateSource(_ context.Context, _ actor.Actor, cmd importer.CreateSourceCmd) (importer.Source, error) {
	s := f.src
	s.Config = cmd.Config
	return s, f.fail
}

func (f *fake) GetSource(context.Context, actor.Actor, ids.ID) (importer.Source, error) {
	return f.src, f.fail
}

func (f *fake) ListItems(_ context.Context, _ actor.Actor, _ ids.ID, q importer.ItemQuery) (importer.ItemPage, error) {
	f.query = q
	return importer.ItemPage{Items: []importer.Item{{ID: ids.New(), OriginKey: "space:1", ItemKind: "space", TargetKind: "space",
		TargetID: ids.New(), State: "posted", Error: "e", SourceChanged: true, CreatedAt: now}}, NextCursor: "n"}, f.fail
}

func (f *fake) Start(context.Context, actor.Actor, ids.ID) (importer.Source, error) {
	return f.src, f.fail
}
func (f *fake) Pause(context.Context, actor.Actor, ids.ID) (importer.Source, error) {
	return f.src, f.fail
}
func (f *fake) Cancel(context.Context, actor.Actor, ids.ID) (importer.Source, error) {
	return f.src, f.fail
}
func (f *fake) Complete(context.Context, actor.Actor, ids.ID) (importer.Source, error) {
	return f.src, f.fail
}

func (f *fake) SetLead(_ context.Context, _ actor.Actor, _ ids.ID, c importer.LeadCmd) (importer.Source, error) {
	f.lead = c
	return f.src, f.fail
}

func (f *fake) ApplyBatch(_ context.Context, _ actor.Actor, _ ids.ID, b importer.Batch) (importer.BatchResult, error) {
	f.batch = b
	return importer.BatchResult{Items: []importer.Item{{ID: ids.New(), OriginKey: "user:1", ItemKind: "user", State: "skipped", CreatedAt: now}}}, f.fail
}

func (f *fake) SetCursor(_ context.Context, _ actor.Actor, _ ids.ID, c importer.CursorCmd) (importer.Source, error) {
	f.cur = c
	return f.src, f.fail
}

func (f *fake) CreateFile(context.Context, actor.Actor, ids.ID, importer.FileCmd) (importer.FileSlot, error) {
	return importer.FileSlot{}, f.fail
}

func (f *fake) UploadFilePart(context.Context, actor.Actor, ids.ID, importer.FilePartCmd) (importer.PartRef, error) {
	return importer.PartRef{}, f.fail
}

func (f *fake) CompleteFile(context.Context, actor.Actor, ids.ID, importer.FileCompleteCmd) (importer.FileSlot, error) {
	return importer.FileSlot{}, f.fail
}

// TestHandlers converts every operation in both directions and passes errors on.
func TestHandlers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := &fake{src: importer.Source{ID: ids.New(), Kind: "discord", Name: "Old", Config: json.RawMessage(`{"version":1}`), State: "running",
		Stats: json.RawMessage(`{"server":{}}`), Cursor: json.RawMessage(`{"run":{"id":"a"}}`), CreatedAt: now, StartedAt: now}}
	h := httpapi.New(f)
	id := rest.Uuid(ids.New())
	limit, cursor := 5, "c"
	list, err := h.ListImports(ctx, rest.ListImportsRequestObject{Params: rest.ListImportsParams{Limit: &limit, Cursor: &cursor}})
	if err != nil {
		t.Fatal(err)
	}
	page := list.(rest.ListImports200JSONResponse).Body
	if len(page.Items) != 1 || *page.NextCursor != "next" || (*page.Items[0].Cursor)["run"] == nil || page.Items[0].FinishedAt != nil {
		t.Errorf("list %+v", page)
	}
	c, err := h.CreateImport(ctx, rest.CreateImportRequestObject{Body: &rest.ImportSourceCreate{Kind: "discord", Name: "Old",
		Config: rest.JsonObject{"version": 1}}})
	if err != nil || c.(rest.CreateImport201JSONResponse).Body.Config["version"] != float64(1) {
		t.Errorf("create %+v %v", c, err)
	}
	if _, err := h.GetImport(ctx, rest.GetImportRequestObject{ImportId: id}); err != nil {
		t.Error(err)
	}
	state := rest.ImportItemState("posted")
	items, err := h.ListImportItems(ctx, rest.ListImportItemsRequestObject{ImportId: id, Params: rest.ListImportItemsParams{State: &state}})
	if err != nil || f.query.State != "posted" {
		t.Fatalf("items %v %+v", err, f.query)
	}
	it := items.(rest.ListImportItems200JSONResponse).Body.Items[0]
	if it.SourceChanged == nil || !*it.SourceChanged || it.TargetId == nil || it.Error == nil {
		t.Errorf("item %+v", it)
	}
	if _, err := h.StartImport(ctx, rest.StartImportRequestObject{ImportId: id}); err != nil {
		t.Error(err)
	}
	if _, err := h.PauseImport(ctx, rest.PauseImportRequestObject{ImportId: id}); err != nil {
		t.Error(err)
	}
	if _, err := h.CancelImport(ctx, rest.CancelImportRequestObject{ImportId: id}); err != nil {
		t.Error(err)
	}
	if _, err := h.CompleteImport(ctx, rest.CompleteImportRequestObject{ImportId: id}); err != nil {
		t.Error(err)
	}
	if _, err := h.SetImportLead(ctx, rest.SetImportLeadRequestObject{ImportId: id, Body: &rest.ImportLeadSet{SourceChannel: "1", Lead: "bothy"}}); err != nil ||
		f.lead != (importer.LeadCmd{SourceChannel: "1", Lead: "bothy"}) {
		t.Errorf("lead %+v %v", f.lead, err)
	}
	thread, sha := "thread:1", "ab"
	b, err := h.ApplyImportBatch(ctx, rest.ApplyImportBatchRequestObject{ImportId: id, Body: &rest.ImportBatch{ThreadOriginKey: &thread,
		Items: []rest.ImportObject{{OriginKey: "space:1", ItemKind: "space", ContentSha256: &sha, Data: rest.JsonObject{"sort_key": "a0"}}}}})
	if err != nil || !f.batch.ThreadOriginKeySet || f.batch.Items[0].ContentSHA256 != "ab" || string(f.batch.Items[0].Data) != `{"sort_key":"a0"}` {
		t.Fatalf("batch %+v %v", f.batch, err)
	}
	if res := b.(rest.ApplyImportBatch200JSONResponse).Body.Items[0]; res.TargetId != nil || res.SourceChanged != nil {
		t.Errorf("skipped item %+v", res)
	}
	stats := rest.JsonObject{"messages": 1}
	if _, err := h.SetImportCursor(ctx, rest.SetImportCursorRequestObject{ImportId: id, Body: &rest.ImportCursorSet{Cursor: rest.JsonObject{"a": 1},
		Stats: &stats}}); err != nil || string(f.cur.Stats) != `{"messages":1}` || string(f.cur.Cursor) != `{"a":1}` {
		t.Errorf("cursor %+v %v", f.cur, err)
	}
	f.fail = errors.New("boom")
	if _, err := h.StartImport(ctx, rest.StartImportRequestObject{ImportId: id}); err == nil {
		t.Error("an error of the use case must pass")
	}
	if _, err := h.ApplyImportBatch(ctx, rest.ApplyImportBatchRequestObject{ImportId: id, Body: &rest.ImportBatch{}}); err == nil {
		t.Error("an error of the use case must pass")
	}
	if _, err := h.CreateImportFile(ctx, rest.CreateImportFileRequestObject{ImportId: id, Body: &rest.ImportFileCreate{}}); err == nil {
		t.Error("an error of createImportFile must pass")
	}
	if _, err := h.UploadImportFilePart(ctx, rest.UploadImportFilePartRequestObject{ImportId: id}); err == nil {
		t.Error("an error of uploadImportFilePart must pass")
	}
	if _, err := h.CompleteImportFile(ctx, rest.CompleteImportFileRequestObject{ImportId: id, Body: &rest.UploadComplete{}}); err == nil {
		t.Error("an error of completeImportFile must pass")
	}
	var cs httpapi.Cursors
	s, err := cs.Encode("imports", nil, map[string]string{"id": "x"})
	if err != nil {
		t.Fatal(err)
	}
	var back map[string]string
	if err := cs.Decode("imports", nil, s, &back); err != nil || back["id"] != "x" {
		t.Errorf("cursor round trip %v %v", back, err)
	}
}
