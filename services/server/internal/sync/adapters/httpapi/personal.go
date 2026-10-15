// SPDX-License-Identifier: Apache-2.0

// Package httpapi holds the REST handlers of the sync module as part of
// rest.StrictServerInterface: the twelve operations of the own read states, drafts, settings and
// sidebar (M1-61) of docs/api/openapi.yaml. The use cases authorize; the handlers only convert,
// set the ETag of markThreadUnread and pass errors to the problem writer of the chain unchanged.
//
// Example:
//
//	h := httpapi.New(personal, personal.Drafts(), personal.Settings())
//	built, err := routes.Build(wire.Handlers{Personal: h}, wire.RouteOptions(), cat)
package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// Personal implements the REST operations of the personal data of sync.
type Personal struct {
	readStates sync.ReadStates
	drafts     sync.Drafts
	settings   sync.Settings
}

// New returns the handlers over the ports of the read states rs, the drafts d and the settings s.
func New(rs sync.ReadStates, d sync.Drafts, s sync.Settings) *Personal {
	return &Personal{readStates: rs, drafts: d, settings: s}
}

// ListMyReadStates implements rest.StrictServerInterface (GET /me/read-states).
func (h *Personal) ListMyReadStates(ctx context.Context, req rest.ListMyReadStatesRequestObject) (rest.ListMyReadStatesResponseObject, error) {
	list, err := h.readStates.ListForChannel(ctx, ids.ID(req.Params.ChannelId))
	if err != nil {
		return nil, err
	}
	out := rest.ReadStateList{Items: make([]rest.ReadState, 0, len(list))}
	for _, rs := range list {
		out.Items = append(out.Items, ReadStateREST(rs))
	}
	return rest.ListMyReadStates200JSONResponse{Body: out}, nil
}

// PutReadState implements rest.StrictServerInterface (PUT /threads/{thread_id}/read-state).
func (h *Personal) PutReadState(ctx context.Context, req rest.PutReadStateRequestObject) (rest.PutReadStateResponseObject, error) {
	rs, err := h.readStates.Advance(ctx, sync.AdvanceReadState{ThreadID: ids.ID(req.ThreadId), LastReadSeq: req.Body.LastReadSeq})
	if err != nil {
		return nil, err
	}
	return rest.PutReadState200JSONResponse{Body: ReadStateREST(rs)}, nil
}

// MarkThreadUnread implements rest.StrictServerInterface (POST
// /threads/{thread_id}/read-state/mark-unread): 200 with the ETag of the version. The chain
// answers a missing If-Match with 428 before the handler and a repeated Idempotency-Key with the
// stored response.
func (h *Personal) MarkThreadUnread(ctx context.Context, req rest.MarkThreadUnreadRequestObject) (rest.MarkThreadUnreadResponseObject, error) {
	rs, err := h.readStates.MarkUnread(ctx, sync.MarkUnread{
		ThreadID: ids.ID(req.ThreadId), LastReadSeq: req.Body.LastReadSeq, IfMatch: precondition(req.Params.IfMatch),
	})
	if err != nil {
		return nil, err
	}
	tag := httpserver.FormatETag(rs.Version)
	return rest.MarkThreadUnread200JSONResponse{Body: ReadStateREST(rs), Headers: rest.MarkThreadUnread200ResponseHeaders{ETag: &tag}}, nil
}

// PutThreadDraft implements rest.StrictServerInterface (PUT /threads/{thread_id}/draft).
func (h *Personal) PutThreadDraft(ctx context.Context, req rest.PutThreadDraftRequestObject) (rest.PutThreadDraftResponseObject, error) {
	cmd := draftCmd(*req.Body)
	cmd.ThreadID = ids.ID(req.ThreadId)
	res, err := h.drafts.PutThreadDraft(ctx, cmd)
	if err != nil {
		return nil, err
	}
	return rest.PutThreadDraft200JSONResponse{Body: draftResult(res)}, nil
}

// DeleteThreadDraft implements rest.StrictServerInterface (DELETE /threads/{thread_id}/draft):
// 204, also without a draft.
func (h *Personal) DeleteThreadDraft(ctx context.Context, req rest.DeleteThreadDraftRequestObject) (rest.DeleteThreadDraftResponseObject, error) {
	if err := h.drafts.DeleteThreadDraft(ctx, ids.ID(req.ThreadId)); err != nil {
		return nil, err
	}
	return rest.DeleteThreadDraft204Response{}, nil
}

// PutChannelDraft implements rest.StrictServerInterface (PUT /channels/{channel_id}/draft).
func (h *Personal) PutChannelDraft(ctx context.Context, req rest.PutChannelDraftRequestObject) (rest.PutChannelDraftResponseObject, error) {
	cmd := draftCmd(*req.Body)
	cmd.ChannelID = ids.ID(req.ChannelId)
	res, err := h.drafts.PutChannelDraft(ctx, cmd)
	if err != nil {
		return nil, err
	}
	return rest.PutChannelDraft200JSONResponse{Body: draftResult(res)}, nil
}

// DeleteChannelDraft implements rest.StrictServerInterface (DELETE /channels/{channel_id}/draft):
// 204, also without a draft.
func (h *Personal) DeleteChannelDraft(ctx context.Context, req rest.DeleteChannelDraftRequestObject) (rest.DeleteChannelDraftResponseObject, error) {
	if err := h.drafts.DeleteChannelDraft(ctx, ids.ID(req.ChannelId)); err != nil {
		return nil, err
	}
	return rest.DeleteChannelDraft204Response{}, nil
}

// ListMyDrafts implements rest.StrictServerInterface (GET /me/drafts).
func (h *Personal) ListMyDrafts(ctx context.Context, _ rest.ListMyDraftsRequestObject) (rest.ListMyDraftsResponseObject, error) {
	list, err := h.drafts.List(ctx)
	if err != nil {
		return nil, err
	}
	out := rest.DraftList{Items: make([]rest.Draft, 0, len(list))}
	for _, d := range list {
		out.Items = append(out.Items, DraftREST(d))
	}
	return rest.ListMyDrafts200JSONResponse{Body: out}, nil
}

// ListMySettings implements rest.StrictServerInterface (GET /me/settings).
func (h *Personal) ListMySettings(ctx context.Context, _ rest.ListMySettingsRequestObject) (rest.ListMySettingsResponseObject, error) {
	list, err := h.settings.List(ctx)
	if err != nil {
		return nil, err
	}
	out := rest.UserSettingList{Items: make([]rest.UserSetting, 0, len(list))}
	for _, st := range list {
		v, err := SettingREST(st)
		if err != nil {
			return nil, err
		}
		out.Items = append(out.Items, v)
	}
	return rest.ListMySettings200JSONResponse{Body: out}, nil
}

// PutMySetting implements rest.StrictServerInterface (PUT /me/settings/{key}).
func (h *Personal) PutMySetting(ctx context.Context, req rest.PutMySettingRequestObject) (rest.PutMySettingResponseObject, error) {
	raw, err := json.Marshal(req.Body.Value)
	if err != nil {
		return nil, errs.New(errs.CodeValidationFailed, "value").WithFields(errs.FieldError{Pointer: "/value", Code: "invalid", Detail: "not a JSON value"})
	}
	st, err := h.settings.Put(ctx, req.Key, raw)
	if err != nil {
		return nil, err
	}
	v, err := SettingREST(st)
	if err != nil {
		return nil, err
	}
	return rest.PutMySetting200JSONResponse{Body: v}, nil
}

// DeleteMySetting implements rest.StrictServerInterface (DELETE /me/settings/{key}): 204, also
// for a known key that is not stored.
func (h *Personal) DeleteMySetting(ctx context.Context, req rest.DeleteMySettingRequestObject) (rest.DeleteMySettingResponseObject, error) {
	if err := h.settings.Delete(ctx, req.Key); err != nil {
		return nil, err
	}
	return rest.DeleteMySetting204Response{}, nil
}

// GetMySidebar implements rest.StrictServerInterface (GET /me/sidebar).
func (h *Personal) GetMySidebar(ctx context.Context, _ rest.GetMySidebarRequestObject) (rest.GetMySidebarResponseObject, error) {
	list, err := h.settings.ListSidebar(ctx)
	if err != nil {
		return nil, err
	}
	out := rest.SidebarStateList{Items: make([]rest.SidebarState, 0, len(list))}
	for _, st := range list {
		v := rest.SidebarState{SpaceId: rest.Uuid(st.SpaceID), Collapsed: st.Collapsed, Version: st.Version,
			UpdatedByDeviceId: optionalID(st.UpdatedByDeviceID)}
		if st.SortKey != "" {
			k := st.SortKey
			v.SortKey = &k
		}
		out.Items = append(out.Items, v)
	}
	return rest.GetMySidebar200JSONResponse{Body: out}, nil
}

// precondition parses the If-Match header value of a request.
func precondition(v string) sync.Precondition {
	hd := http.Header{}
	if v != "" {
		hd.Set("If-Match", v)
	}
	im := httpserver.ParseIfMatch(hd)
	return sync.Precondition{Present: im.Present, Valid: im.Valid, Version: im.Version}
}

// draftCmd converts the body DraftPut.
func draftCmd(b rest.DraftPut) sync.PutDraft {
	cmd := sync.PutDraft{ContentMD: b.ContentMd, BaseVersion: b.BaseVersion}
	if b.ReplyToMessageId != nil {
		r := ids.ID(*b.ReplyToMessageId)
		cmd.ReplyToID = &r
	}
	if b.FileIds != nil {
		cmd.FileIDs = make([]ids.ID, 0, len(*b.FileIds))
		for _, f := range *b.FileIds {
			cmd.FileIDs = append(cmd.FileIDs, ids.ID(f))
		}
	}
	return cmd
}

// draftResult converts the result of a draft write (schema DraftPutResult).
func draftResult(r sync.DraftPutResult) rest.DraftPutResult {
	return rest.DraftPutResult{Draft: DraftREST(r.Draft), OverwroteNewer: r.OverwroteNewer}
}

// ReadStateREST converts a read state (schema ReadState): empty values become absent members,
// never null (ADR 0053).
func ReadStateREST(rs sync.ReadState) rest.ReadState {
	return rest.ReadState{
		ThreadId: rest.Uuid(rs.ThreadID), ChannelId: rest.Uuid(rs.ChannelID), LastReadSeq: rs.LastReadSeq, MentionCount: rs.MentionCount,
		Version: rs.Version, UpdatedByDeviceId: optionalID(rs.UpdatedByDeviceID), UpdatedAt: optionalTime(rs.UpdatedAt),
	}
}

// DraftREST converts a draft (schema Draft); file_ids is never null.
func DraftREST(d sync.Draft) rest.Draft {
	files := make([]rest.Uuid, 0, len(d.FileIDs))
	for _, f := range d.FileIDs {
		files = append(files, rest.Uuid(f))
	}
	return rest.Draft{
		Id: rest.Uuid(d.ID), ThreadId: optionalID(d.ThreadID), ChannelId: rest.Uuid(d.ChannelID), ContentMd: d.ContentMD,
		ReplyToMessageId: optionalID(d.ReplyToID), FileIds: files, Version: d.Version, UpdatedByDeviceId: optionalID(d.UpdatedByDeviceID),
		UpdatedAt: optionalTime(d.UpdatedAt),
	}
}

// SettingREST converts a setting (schema UserSetting). Errors: internal for a stored value that
// is not JSON, which the jsonb column rules out.
func SettingREST(st sync.UserSetting) (rest.UserSetting, error) {
	// json.Number keeps integers of layout.column_widths exact; float64 would round them.
	dec := json.NewDecoder(bytes.NewReader(st.Value))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return rest.UserSetting{}, errs.Wrap(errs.CodeInternalError, err, "sync: a stored setting is not JSON")
	}
	return rest.UserSetting{Key: st.Key, Value: v, Version: st.Version, UpdatedByDeviceId: optionalID(st.UpdatedByDeviceID),
		UpdatedAt: optionalTime(st.UpdatedAt)}, nil
}

// optionalID returns nil for the zero ID.
func optionalID(id ids.ID) *rest.Uuid {
	if id.IsZero() {
		return nil
	}
	u := rest.Uuid(id)
	return &u
}

// optionalTime returns nil for the zero time; times are UTC with milliseconds (06 section 2).
func optionalTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	u := t.UTC().Truncate(time.Millisecond)
	return &u
}
