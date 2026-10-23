// SPDX-License-Identifier: Apache-2.0

// Package httpapi holds the REST handlers of the importer module (M8-03, M8-11): listImports,
// createImport, getImport, listImportItems, startImport, pauseImport, cancelImport,
// completeImport, setImportLead, applyImportBatch, setImportCursor, createImportFile,
// uploadImportFilePart and completeImportFile of docs/api/openapi.yaml as part of
// rest.StrictServerInterface. The use cases of importer.Imports authorize; the handlers
// only convert and pass errors to the problem writer of the chain unchanged.
//
// Example:
//
//	h := httpapi.New(svc)
//	built, err := routes.Build(wire.Handlers{Imports: h}, wire.RouteOptions(), cat)
package httpapi

import (
	"context"
	"encoding/json"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/importer"
	"github.com/OWNER/bothy/services/server/internal/importer/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
)

// Imports implements the fourteen operations of the tag Import.
type Imports struct {
	imports importer.Imports
}

// New returns the handlers over the use cases.
func New(i importer.Imports) *Imports { return &Imports{imports: i} }

// Cursors is the codec of the opaque list cursors of the importer (platform/httpserver).
type Cursors struct{}

// Compile-time check: the codec is the port of the use cases.
var _ app.Cursors = Cursors{}

// Encode implements app.Cursors with httpserver.EncodeCursor.
func (Cursors) Encode(list string, binding []byte, key any) (string, error) {
	return httpserver.EncodeCursor(list, binding, key)
}

// Decode implements app.Cursors with httpserver.DecodeCursor (400 cursor_invalid on a mismatch).
func (Cursors) Decode(list string, binding []byte, s string, key any) error {
	return httpserver.DecodeCursor(list, binding, s, key)
}

// principal returns the actor of the request; anonymous when the chain set none.
func principal(ctx context.Context) actor.Actor {
	if a, ok := actor.FromContext(ctx); ok {
		return a
	}
	return actor.Anonymous()
}

// page converts the query of a list.
func page(cursor *rest.Cursor, limit *rest.Limit) importer.Page {
	var p importer.Page
	if cursor != nil {
		p.Cursor = *cursor
	}
	if limit != nil {
		p.Limit = int(*limit)
	}
	return p
}

// raw marshals a JSON object of a request.
func raw(o rest.JsonObject) (json.RawMessage, error) {
	b, err := json.Marshal(o)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, err, "importer: request object")
	}
	return b, nil
}

// object decodes a stored JSON object for an answer; {} when empty.
func object(r json.RawMessage) rest.JsonObject {
	o := rest.JsonObject{}
	if len(r) > 0 {
		_ = json.Unmarshal(r, &o)
	}
	return o
}

func optionalTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	v := t.UTC().Truncate(time.Millisecond)
	return &v
}

// sourceREST converts a source (schema ImportSource) with its cursor.
func sourceREST(s importer.Source) rest.ImportSource {
	cursor := object(s.Cursor)
	return rest.ImportSource{
		Id: rest.Uuid(s.ID), Kind: rest.ImportKind(s.Kind), Name: s.Name, Config: object(s.Config), State: rest.ImportSourceState(s.State),
		Stats: object(s.Stats), Cursor: &cursor, CreatedAt: s.CreatedAt.UTC().Truncate(time.Millisecond),
		StartedAt: optionalTime(s.StartedAt), FinishedAt: optionalTime(s.FinishedAt),
	}
}

// itemREST converts an item (schema ImportItem); empty values are absent.
func itemREST(i importer.Item) rest.ImportItem {
	out := rest.ImportItem{
		Id: rest.Uuid(i.ID), OriginKey: i.OriginKey, ItemKind: rest.ImportItemItemKind(i.ItemKind), State: rest.ImportItemState(i.State),
		CreatedAt: i.CreatedAt.UTC().Truncate(time.Millisecond),
	}
	if i.TargetKind != "" {
		k := i.TargetKind
		out.TargetKind = &k
	}
	if !i.TargetID.IsZero() {
		t := rest.Uuid(i.TargetID)
		out.TargetId = &t
	}
	if i.Error != "" {
		e := i.Error
		out.Error = &e
	}
	if i.SourceChanged {
		b := true
		out.SourceChanged = &b
	}
	return out
}

// ListImports implements rest.StrictServerInterface (GET /imports).
func (h *Imports) ListImports(ctx context.Context, req rest.ListImportsRequestObject) (rest.ListImportsResponseObject, error) {
	pg, err := h.imports.ListSources(ctx, principal(ctx), page(req.Params.Cursor, req.Params.Limit))
	if err != nil {
		return nil, err
	}
	out := rest.ImportSourcePage{Items: make([]rest.ImportSource, 0, len(pg.Items))}
	for _, s := range pg.Items {
		out.Items = append(out.Items, sourceREST(s))
	}
	if pg.NextCursor != "" {
		c := pg.NextCursor
		out.NextCursor = &c
	}
	return rest.ListImports200JSONResponse{Body: out}, nil
}

// CreateImport implements rest.StrictServerInterface (POST /imports).
func (h *Imports) CreateImport(ctx context.Context, req rest.CreateImportRequestObject) (rest.CreateImportResponseObject, error) {
	config, err := raw(req.Body.Config)
	if err != nil {
		return nil, err
	}
	s, err := h.imports.CreateSource(ctx, principal(ctx), importer.CreateSourceCmd{Kind: string(req.Body.Kind), Name: req.Body.Name, Config: config})
	if err != nil {
		return nil, err
	}
	return rest.CreateImport201JSONResponse{Body: sourceREST(s)}, nil
}

// GetImport implements rest.StrictServerInterface (GET /imports/{import_id}).
func (h *Imports) GetImport(ctx context.Context, req rest.GetImportRequestObject) (rest.GetImportResponseObject, error) {
	s, err := h.imports.GetSource(ctx, principal(ctx), ids.ID(req.ImportId))
	if err != nil {
		return nil, err
	}
	return rest.GetImport200JSONResponse{Body: sourceREST(s)}, nil
}

// ListImportItems implements rest.StrictServerInterface (GET /imports/{import_id}/items).
func (h *Imports) ListImportItems(ctx context.Context, req rest.ListImportItemsRequestObject) (rest.ListImportItemsResponseObject, error) {
	q := importer.ItemQuery{Page: page(req.Params.Cursor, req.Params.Limit)}
	if req.Params.State != nil {
		q.State = string(*req.Params.State)
	}
	pg, err := h.imports.ListItems(ctx, principal(ctx), ids.ID(req.ImportId), q)
	if err != nil {
		return nil, err
	}
	out := rest.ImportItemPage{Items: make([]rest.ImportItem, 0, len(pg.Items))}
	for _, i := range pg.Items {
		out.Items = append(out.Items, itemREST(i))
	}
	if pg.NextCursor != "" {
		c := pg.NextCursor
		out.NextCursor = &c
	}
	return rest.ListImportItems200JSONResponse{Body: out}, nil
}

// StartImport implements rest.StrictServerInterface (POST /imports/{import_id}/start).
func (h *Imports) StartImport(ctx context.Context, req rest.StartImportRequestObject) (rest.StartImportResponseObject, error) {
	s, err := h.imports.Start(ctx, principal(ctx), ids.ID(req.ImportId))
	if err != nil {
		return nil, err
	}
	return rest.StartImport202JSONResponse{Body: sourceREST(s)}, nil
}

// PauseImport implements rest.StrictServerInterface (POST /imports/{import_id}/pause).
func (h *Imports) PauseImport(ctx context.Context, req rest.PauseImportRequestObject) (rest.PauseImportResponseObject, error) {
	s, err := h.imports.Pause(ctx, principal(ctx), ids.ID(req.ImportId))
	if err != nil {
		return nil, err
	}
	return rest.PauseImport200JSONResponse{Body: sourceREST(s)}, nil
}

// CancelImport implements rest.StrictServerInterface (POST /imports/{import_id}/cancel).
func (h *Imports) CancelImport(ctx context.Context, req rest.CancelImportRequestObject) (rest.CancelImportResponseObject, error) {
	s, err := h.imports.Cancel(ctx, principal(ctx), ids.ID(req.ImportId))
	if err != nil {
		return nil, err
	}
	return rest.CancelImport200JSONResponse{Body: sourceREST(s)}, nil
}

// CompleteImport implements rest.StrictServerInterface (POST /imports/{import_id}/complete).
func (h *Imports) CompleteImport(ctx context.Context, req rest.CompleteImportRequestObject) (rest.CompleteImportResponseObject, error) {
	s, err := h.imports.Complete(ctx, principal(ctx), ids.ID(req.ImportId))
	if err != nil {
		return nil, err
	}
	return rest.CompleteImport200JSONResponse{Body: sourceREST(s)}, nil
}

// SetImportLead implements rest.StrictServerInterface (PUT /imports/{import_id}/leads).
func (h *Imports) SetImportLead(ctx context.Context, req rest.SetImportLeadRequestObject) (rest.SetImportLeadResponseObject, error) {
	s, err := h.imports.SetLead(ctx, principal(ctx), ids.ID(req.ImportId), importer.LeadCmd{
		SourceChannel: req.Body.SourceChannel, Lead: string(req.Body.Lead),
	})
	if err != nil {
		return nil, err
	}
	return rest.SetImportLead200JSONResponse{Body: sourceREST(s)}, nil
}

// ApplyImportBatch implements rest.StrictServerInterface (POST /imports/{import_id}/batches).
func (h *Imports) ApplyImportBatch(ctx context.Context, req rest.ApplyImportBatchRequestObject) (rest.ApplyImportBatchResponseObject, error) {
	b := importer.Batch{Items: make([]importer.BatchObject, 0, len(req.Body.Items))}
	if req.Body.ThreadOriginKey != nil {
		b.ThreadOriginKey, b.ThreadOriginKeySet = *req.Body.ThreadOriginKey, true
	}
	for _, o := range req.Body.Items {
		data, err := raw(o.Data)
		if err != nil {
			return nil, err
		}
		obj := importer.BatchObject{OriginKey: o.OriginKey, ItemKind: string(o.ItemKind), Data: data}
		if o.ContentSha256 != nil {
			obj.ContentSHA256 = *o.ContentSha256
		}
		b.Items = append(b.Items, obj)
	}
	res, err := h.imports.ApplyBatch(ctx, principal(ctx), ids.ID(req.ImportId), b)
	if err != nil {
		return nil, err
	}
	out := rest.ImportBatchResult{Items: make([]rest.ImportItem, 0, len(res.Items)), UnreadFloorSeq: res.UnreadFloorSeq}
	for _, i := range res.Items {
		out.Items = append(out.Items, itemREST(i))
	}
	return rest.ApplyImportBatch200JSONResponse{Body: out}, nil
}

// SetImportCursor implements rest.StrictServerInterface (PUT /imports/{import_id}/cursor).
func (h *Imports) SetImportCursor(ctx context.Context, req rest.SetImportCursorRequestObject) (rest.SetImportCursorResponseObject, error) {
	cursor, err := raw(req.Body.Cursor)
	if err != nil {
		return nil, err
	}
	cmd := importer.CursorCmd{Cursor: cursor}
	if req.Body.Stats != nil {
		if cmd.Stats, err = raw(*req.Body.Stats); err != nil {
			return nil, err
		}
	}
	s, err := h.imports.SetCursor(ctx, principal(ctx), ids.ID(req.ImportId), cmd)
	if err != nil {
		return nil, err
	}
	return rest.SetImportCursor200JSONResponse{Body: sourceREST(s)}, nil
}
