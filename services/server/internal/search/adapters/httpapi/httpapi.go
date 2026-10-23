// SPDX-License-Identifier: Apache-2.0

// Package httpapi holds the REST handlers of the search module: search and searchThreadTitles of
// docs/api/openapi.yaml as part of rest.StrictServerInterface (brief M3-14 design note 9). The
// coordinator authorizes and pages; the handlers convert the parameters, render the hit objects
// through the read operations and the REST renderers of the owning modules (so a hit's message,
// thread or file is exactly what listThreadMessages, getThread or getFile give) and pass errors to
// the problem writer of the chain unchanged.
//
// Example:
//
//	h := httpapi.New(coord, httpapi.Objects{Messages: m, Threads: c, Files: f}, httpapi.Renderers{
//		Message: messaginghttp.MessageREST, Thread: chathttp.ThreadREST, File: fileshttp.FileREST,
//	})
package httpapi

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/searchkit"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/search"
	"github.com/OWNER/bothy/services/server/internal/search/app"
)

// Objects are the read operations that render the objects of the hits.
type Objects struct {
	Messages interface {
		HitMessages(ctx context.Context, messageIDs, channelIDs []ids.ID) ([]messaging.Message, error)
	}
	Threads interface {
		HitThreads(ctx context.Context, threadIDs, channelIDs []ids.ID) ([]chat.Thread, error)
	}
	Files interface {
		HitFiles(ctx context.Context, fileIDs, channelIDs []ids.ID) ([]files.File, error)
	}
}

// Renderers are the REST renderers of the owning modules (messaging, chat, files), passed in by
// internal/wire, so search renders an object exactly as its module does without importing another
// module's adapters.
type Renderers struct {
	Message func(messaging.Message) rest.Message
	Thread  func(chat.Thread) rest.Thread
	File    func(files.File) rest.File
}

// Search implements search and searchThreadTitles.
type Search struct {
	search  search.Search
	objects Objects
	render  Renderers
}

// New returns the handlers. Errors: a missing collaborator.
func New(s search.Search, o Objects, r Renderers) (*Search, error) {
	if s == nil || o.Messages == nil || o.Threads == nil || o.Files == nil || r.Message == nil || r.Thread == nil || r.File == nil {
		return nil, errors.New("httpapi: search needs the coordinator, the three read operations and the three renderers")
	}
	return &Search{search: s, objects: o, render: r}, nil
}

// Cursors is the codec of the list cursors (platform/httpserver, M1-18), the outgoing port
// app.Cursors of the coordinator.
type Cursors struct{}

// Compile-time check: the codec is the port of the coordinator.
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

// Search implements rest.StrictServerInterface (GET /search).
func (h *Search) Search(ctx context.Context, req rest.SearchRequestObject) (rest.SearchResponseObject, error) {
	res, err := h.search.Query(ctx, principal(ctx), request(req.Params))
	if err != nil {
		return nil, err
	}
	items, err := h.items(ctx, res.Hits)
	if err != nil {
		return nil, err
	}
	capped := res.CandidatesCapped
	out := rest.SearchHitPage{Items: items, CandidatesCapped: &capped}
	if res.NextCursor != "" {
		next := res.NextCursor
		out.NextCursor = &next
	}
	return rest.Search200JSONResponse{Body: out}, nil
}

// SearchThreadTitles implements rest.StrictServerInterface (GET /search/thread-titles).
func (h *Search) SearchThreadTitles(ctx context.Context, req rest.SearchThreadTitlesRequestObject) (rest.SearchThreadTitlesResponseObject, error) {
	r := search.PrefixRequest{Q: req.Params.Q}
	if req.Params.Limit != nil {
		r.Limit = *req.Params.Limit
	}
	hits, err := h.search.ThreadTitles(ctx, principal(ctx), r)
	if err != nil {
		return nil, err
	}
	out := rest.ThreadTitleHitList{Items: make([]struct {
		ChannelId      rest.Uuid      `json:"channel_id"`
		LastActivityAt rest.Timestamp `json:"last_activity_at"`
		ThreadId       rest.Uuid      `json:"thread_id"`
		Title          string         `json:"title"`
	}, 0, len(hits))}
	for _, t := range hits {
		out.Items = append(out.Items, struct {
			ChannelId      rest.Uuid      `json:"channel_id"`
			LastActivityAt rest.Timestamp `json:"last_activity_at"`
			ThreadId       rest.Uuid      `json:"thread_id"`
			Title          string         `json:"title"`
		}{ChannelId: rest.Uuid(t.ChannelID), LastActivityAt: t.LastActivityAt.UTC().Truncate(time.Millisecond), ThreadId: rest.Uuid(t.ThreadID), Title: t.Title})
	}
	return rest.SearchThreadTitles200JSONResponse{Body: out}, nil
}

// request maps the query parameters to the port request; the coordinator validates them.
func request(p rest.SearchParams) search.Request {
	r := search.Request{
		Q: p.Q, SpaceIDs: idList(p.SpaceId), ChannelIDs: idList(p.ChannelId), TagIDs: idList(p.TagId),
		ThreadID: optID(p.ThreadId), AuthorID: optID(p.AuthorId), AgentID: optID(p.AgentId),
		After: p.After, Before: p.Before, HasFile: p.HasFile,
	}
	if p.Type != nil {
		r.Type = search.Type(*p.Type)
	}
	if p.Sort != nil {
		r.Sort = searchkit.Sort(*p.Sort)
	}
	if p.ThreadState != nil {
		r.ThreadState = string(*p.ThreadState)
	}
	if p.Limit != nil {
		r.Limit = *p.Limit
	}
	if p.Cursor != nil {
		r.Cursor = *p.Cursor
	}
	return r
}

// idList converts an optional id list; nil for none.
func idList(in *[]rest.Uuid) []ids.ID {
	if in == nil {
		return nil
	}
	out := make([]ids.ID, 0, len(*in))
	for _, u := range *in {
		out = append(out, ids.ID(u))
	}
	return out
}

// optID converts an optional id.
func optID(in *rest.Uuid) *ids.ID {
	if in == nil {
		return nil
	}
	id := ids.ID(*in)
	return &id
}

// segment is the element type of SearchHit.segments.
type segment = struct {
	Match bool   `json:"match"`
	Text  string `json:"text"`
}

// items renders the hits in page order with the object of each from its module (design note 9);
// a hit whose object vanished since the query is dropped. Never nil.
func (h *Search) items(ctx context.Context, hits []search.ResultHit) ([]rest.SearchHit, error) {
	out := make([]rest.SearchHit, 0, len(hits))
	if len(hits) == 0 {
		return out, nil
	}
	objs, err := h.objectsOf(ctx, hits)
	if err != nil {
		return nil, err
	}
	for _, hit := range hits {
		item, ok := objs[hit.ID]
		if !ok {
			continue
		}
		item.Kind = rest.SearchHitKind(hit.Kind)
		item.Score = hit.Score
		item.Segments = make([]segment, 0, len(hit.Segments))
		for _, s := range hit.Segments {
			item.Segments = append(item.Segments, segment{Match: s.Match, Text: s.Text})
		}
		item.ThreadTitle, item.ChannelName, item.SpaceName = optString(hit.ThreadTitle), optString(hit.ChannelName), optString(hit.SpaceName)
		out = append(out, item)
	}
	return out, nil
}

// objectsOf loads the objects of hits (all of one kind) through the module's read operation,
// called with the ids and channels of the hits, and returns each as a SearchHit with its object
// set, keyed by the hit id.
func (h *Search) objectsOf(ctx context.Context, hits []search.ResultHit) (map[ids.ID]rest.SearchHit, error) {
	var idsOf, channels []ids.ID
	seen := map[ids.ID]bool{}
	for _, hit := range hits {
		idsOf = append(idsOf, hit.ID)
		if !seen[hit.ChannelID] {
			seen[hit.ChannelID] = true
			channels = append(channels, hit.ChannelID)
		}
	}
	out := make(map[ids.ID]rest.SearchHit, len(hits))
	switch hits[0].Kind {
	case string(rest.SearchHitKindThread):
		list, err := h.objects.Threads.HitThreads(ctx, idsOf, channels)
		if err != nil {
			return nil, err
		}
		for _, t := range list {
			v := h.render.Thread(t)
			out[t.ID] = rest.SearchHit{Thread: &v}
		}
	case string(rest.SearchHitKindFile):
		list, err := h.objects.Files.HitFiles(ctx, idsOf, channels)
		if err != nil {
			return nil, err
		}
		for _, f := range list {
			v := h.render.File(f)
			out[f.ID] = rest.SearchHit{File: &v}
		}
	default:
		list, err := h.objects.Messages.HitMessages(ctx, idsOf, channels)
		if err != nil {
			return nil, err
		}
		for _, m := range list {
			v := h.render.Message(m)
			out[m.ID] = rest.SearchHit{Message: &v}
		}
	}
	return out, nil
}

// optString returns nil for "", so the member is absent (no JSON null, ADR 0053).
func optString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
