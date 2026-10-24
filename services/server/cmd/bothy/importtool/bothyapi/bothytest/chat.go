// SPDX-License-Identifier: Apache-2.0

package bothytest

import (
	"net/http"
	"strconv"
)

// The read model of chat in the fake (M8-07): the channel, its threads and the origin and kind of
// their messages, so the commands revert and status can read what was written in Bothy. Imported
// messages are not mirrored into it; a test adds the messages it needs with AddThreadMessage.

// chatThread is a thread of a channel with its messages in thread_seq order.
type chatThread struct {
	id, channelID, kind string
	messages            []chatMessage
}

// chatMessage is one message of a chatThread.
type chatMessage struct {
	id, origin, kind string
}

// The thread kinds of the read model.
const (
	threadMain   = "main"
	threadThread = "thread"
)

// channelByID returns the channel id of any space, nil when unknown.
func (im *imports) channelByID(id string) *channel {
	for _, sp := range im.spaces {
		for _, ch := range sp.channels {
			if ch.id == id {
				return ch
			}
		}
	}
	return nil
}

// threadByID returns the thread id of any channel, nil when unknown.
func (im *imports) threadByID(id string) *chatThread {
	for _, sp := range im.spaces {
		for _, ch := range sp.channels {
			if ch.mainThread != nil && ch.mainThread.id == id {
				return ch.mainThread
			}
			for _, t := range ch.threads {
				if t.id == id {
					return t
				}
			}
		}
	}
	return nil
}

// main returns the main thread of a text channel, created on first use; nil for a forum.
func (ch *channel) main() *chatThread {
	if ch.kind == "forum" {
		return nil
	}
	if ch.mainThread == nil {
		ch.mainThread = &chatThread{id: newID(), channelID: ch.id, kind: threadMain}
	}
	return ch.mainThread
}

// MainThread returns the UUID of the main thread of the channel (a text channel of AddChannel or
// of an import batch); "" for an unknown channel or a forum.
func (s *Server) MainThread(channelID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ch := s.imp.channelByID(channelID); ch != nil {
		if m := ch.main(); m != nil {
			return m.id
		}
	}
	return ""
}

// AddThread adds a thread of kind thread to the channel and returns its UUID; "" for an unknown
// channel.
func (s *Server) AddThread(channelID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch := s.imp.channelByID(channelID)
	if ch == nil {
		return ""
	}
	t := &chatThread{id: newID(), channelID: ch.id, kind: threadThread}
	ch.threads = append(ch.threads, t)
	return t.id
}

// AddThreadMessage appends a message with origin (human, agent, app, system or import) and kind
// (text, system, ...) to the thread and returns its UUID; "" for an unknown thread.
func (s *Server) AddThreadMessage(threadID, origin, kind string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	t := s.imp.threadByID(threadID)
	if t == nil {
		return ""
	}
	id := newID()
	t.messages = append(t.messages, chatMessage{id: id, origin: origin, kind: kind})
	return id
}

// SetChannelOperator sets the operator of the channel: an agent UUID for mode agent, "" for mode
// none (setChannelOperator of the web console).
func (s *Server) SetChannelOperator(channelID, agentID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ch := s.imp.channelByID(channelID); ch != nil {
		ch.operator = agentID
	}
}

// SetChannelReadOnly sets the read-only flag of the channel, as setImportLead does with the lead.
func (s *Server) SetChannelReadOnly(channelID string, readOnly bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ch := s.imp.channelByID(channelID); ch != nil {
		ch.readOnly = readOnly
	}
}

// channelJSON is the schema Channel of ch in space.
func channelJSON(ch *channel, spaceID string) map[string]any {
	op := map[string]any{memberMode: "none"}
	if ch.operator != "" {
		op = map[string]any{memberMode: "agent", "agent_id": ch.operator, "effective_agent_id": ch.operator}
	}
	out := map[string]any{"id": ch.id, memberName: ch.name, memberKind: ch.kind, "space_id": spaceID, memberSortKey: ch.sortKey,
		"topic": ch.topic, "read_only": ch.readOnly, "operator": op, "settings_version": 1, memberVersion: 1,
		memberCreatedAt: stamp(ch.created)}
	if m := ch.main(); m != nil {
		out["main_thread_id"] = m.id
	}
	return out
}

// spaceOf returns the space ID of the channel.
func (im *imports) spaceOf(ch *channel) string {
	for _, sp := range im.spaces {
		for _, c := range sp.channels {
			if c == ch {
				return sp.id
			}
		}
	}
	return ""
}

// getChannel answers getChannel (ChannelView); 404 for an unknown channel.
func (s *Server) getChannel(_ *http.Request, _ []byte, params []string) (int, any, *apiError) {
	ch := s.imp.channelByID(params[0])
	if ch == nil {
		return 0, nil, notFound()
	}
	return http.StatusOK, view("channel", channelJSON(ch, s.imp.spaceOf(ch))), nil
}

// threadJSON is the schema Thread of t.
func threadJSON(t *chatThread) map[string]any {
	return map[string]any{"id": t.id, "channel_id": t.channelID, memberKind: t.kind, "message_count": len(t.messages),
		"last_message_seq": len(t.messages), memberState: "open"}
}

// listChannelThreads answers listChannelThreads: the threads of kind thread (never the main
// thread, like the server) in pages of limit and cursor; 404 for an unknown channel.
func (s *Server) listChannelThreads(r *http.Request, _ []byte, params []string) (int, any, *apiError) {
	ch := s.imp.channelByID(params[0])
	if ch == nil {
		return 0, nil, notFound()
	}
	items := make([]any, 0, len(ch.threads))
	for _, t := range ch.threads {
		items = append(items, threadJSON(t))
	}
	return pageOf(r, items)
}

// listThreadMessages answers listThreadMessages with after_seq and limit (thread_seq is the
// 1-based position); 404 for an unknown thread.
func (s *Server) listThreadMessages(r *http.Request, _ []byte, params []string) (int, any, *apiError) {
	t := s.imp.threadByID(params[0])
	if t == nil {
		return 0, nil, notFound()
	}
	q := r.URL.Query()
	after, limit := 0, pageSizeDefault
	if v := q.Get("after_seq"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return 0, nil, invalid(fieldError{Pointer: "#query/after_seq", Code: codeInvalid})
		}
		after = min(n, len(t.messages))
	}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return 0, nil, invalid(fieldError{Pointer: "#query/limit", Code: codeInvalid})
		}
		limit = min(n, pageSizeMax)
	}
	to := min(after+limit, len(t.messages))
	items := make([]any, 0, to-after)
	for i := after; i < to; i++ {
		m := t.messages[i]
		items = append(items, map[string]any{"id": m.id, "thread_id": t.id, "channel_id": t.channelID, "origin": m.origin,
			memberKind: m.kind, "thread_seq": i + 1, "content_md": "", "attachments": []any{}, "components": []any{},
			"mention_ids": []any{}, "reactions": []any{}})
	}
	return http.StatusOK, map[string]any{memberItems: items, "has_more_after": to < len(t.messages), "has_more_before": after > 0}, nil
}

// view is a SpaceView or ChannelView of the person of the session: level trigger, no capabilities,
// and obj under key (space or channel).
func view(key string, obj map[string]any) map[string]any {
	return map[string]any{"level": "trigger", memberCaps: []any{}, key: obj}
}
