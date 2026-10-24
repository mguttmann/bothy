// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// FieldNotImported is the field code of a reference to an item of the source that is not posted:
// a reply target, a pinned message or an attachment file (M8-11).
const FieldNotImported = "not_imported"

// Content reports the item kinds of a thread batch (M8-11 design note 1): thread, message and pin.
func Content(kind string) bool {
	switch kind {
	case KindThread, KindMessage, KindPin:
		return true
	}
	return false
}

// ThreadKey is the parsed thread_origin_key of a thread batch: Kind is KindThread for a thread or
// forum post (thread:<id>) and KindChannel for the main thread of a text channel (channel:<id>).
type ThreadKey struct {
	Kind, Key string
	// ID is the Discord ID of the thread or the channel.
	ID string
}

// ItemRef is the kind and the origin key of one item, as the shape check of a batch sees it.
type ItemRef struct {
	Kind, OriginKey string
}

// CheckThreadBatch checks the shape of a thread batch (M8-11 design note 1) and records every
// violation in p: thread_origin_key is thread:<digits> or channel:<digits>; only the first item may
// be the thread item, and only with the key of a thread: batch; every other item is a message or a
// pin. It returns the parsed key (zero after a violation of the key).
//
// Example:
//
//	k := CheckThreadBatch("thread:42", []ItemRef{{"thread", "thread:42"}, {"message", "message:43"}}, &p)
func CheckThreadBatch(key string, items []ItemRef, p *Problems) ThreadKey {
	var tk ThreadKey
	prefix, rest, _ := strings.Cut(key, ":")
	switch {
	case (prefix != KindThread && prefix != KindChannel) || !Digits(rest):
		p.Add("/thread_origin_key", FieldInvalid, "thread:<thread_id> or channel:<channel_id>")
	default:
		tk = ThreadKey{Kind: prefix, Key: key, ID: rest}
	}
	for i, it := range items {
		base := Pointer("/items", i)
		switch {
		case it.Kind == KindThread && i == 0:
			if tk.Kind != KindThread || it.OriginKey != key {
				p.Add(base+"/origin_key", FieldInvalid, "the thread item carries the thread_origin_key of its batch")
			}
		case it.Kind == KindMessage || it.Kind == KindPin:
		default:
			p.Add(base+"/item_kind", FieldInvalid, "a thread batch holds the thread item first, then messages and pins")
		}
	}
	return tk
}

// ThreadData is the data of a thread item (schema ImportThreadData).
type ThreadData struct {
	ChannelOriginKey, Name string
	CreatedAt              time.Time
	LastActivityAt         time.Time
	Closed                 bool
	// ClosedAt is set exactly when Closed.
	ClosedAt time.Time
	// StarterMessageOriginKey is "" for none; TagOriginKeys may be empty.
	StarterMessageOriginKey string
	TagOriginKeys           []string
	// EarlierSession is the member earlier_session (M8-06), nil when absent.
	EarlierSession *EarlierSessionData
}

// EarlierSessionData is the member earlier_session of a thread item (schema ImportEarlierSession,
// M8-06 design note 3). Only its shape is checked here; the operator, the reference, the baseline
// and the currency are the rules of agents (agents.ImportedSessions).
type EarlierSessionData struct {
	AgentID ids.ID
	// ExternalSessionRef is opaque and never logged.
	ExternalSessionRef string
	// CostBaselineMicros is 0 when absent; Currency "" when absent.
	CostBaselineMicros int64
	Currency           string
}

// memberEarlierSession is the member of the earlier session in a thread item.
const memberEarlierSession = "earlier_session"

// parseEarlierSession checks the shape of earlier_session at ptr: agent_id a UUID and
// external_session_ref a string, both required; cost_baseline_micros an integer and currency a
// string, both optional.
func parseEarlierSession(raw json.RawMessage, ptr string, p *Problems) *EarlierSessionData {
	m := object(raw, ptr, []string{"agent_id", "external_session_ref", "cost_baseline_micros", "currency"}, p)
	if m == nil {
		return nil
	}
	d := &EarlierSessionData{}
	if _, ok := m["agent_id"]; ok {
		d.AgentID = optionalID(m, "agent_id", ptr, p)
	} else {
		p.Add(ptr+"/agent_id", FieldRequired, "the member is required")
	}
	d.ExternalSessionRef, _ = str(m, "external_session_ref", ptr, true, p)
	d.Currency, _ = str(m, "currency", ptr, false, p)
	if raw, ok := m["cost_baseline_micros"]; ok && !isNull(raw) {
		if err := json.Unmarshal(raw, &d.CostBaselineMicros); err != nil {
			p.Add(ptr+"/cost_baseline_micros", FieldInvalid, "an integer is required")
		}
	}
	return d
}

// MessageData is the data of a message item (schema ImportMessageData).
type MessageData struct {
	AuthorOriginKey, ContentMD string
	CreatedAt                  time.Time
	// EditedAt is zero for none; ReplyToOriginKey "" for none.
	EditedAt          time.Time
	ReplyToOriginKey  string
	AttachmentFileIDs []ids.ID
}

// PinData is the data of a pin item (schema ImportPinData).
type PinData struct {
	MessageOriginKey string
}

// stamp reads the required or optional timestamp member key (RFC 3339 with offset) in the
// precision of the columns.
func stamp(m map[string]json.RawMessage, key, ptr string, required bool, p *Problems) time.Time {
	s, ok := str(m, key, ptr, required, p)
	if !ok {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		p.Add(Pointer(ptr, key), FieldInvalid, "an RFC 3339 timestamp is required")
		return time.Time{}
	}
	return t.UTC().Truncate(time.Microsecond)
}

// keyOf reads the string member key as an origin key of kind; "" when absent.
func keyOf(m map[string]json.RawMessage, key, kind, ptr string, required bool, p *Problems) string {
	s, ok := str(m, key, ptr, required, p)
	if !ok {
		return ""
	}
	if _, code := ParseOriginKey(kind, s); code != "" {
		p.Add(Pointer(ptr, key), code, kind+":<id>")
		return ""
	}
	return s
}

// ParseThreadData checks the data of a thread item at ptr: channel_origin_key, name, created_at,
// last_activity_at and closed are required; closed_at is required exactly when closed is true.
func ParseThreadData(raw json.RawMessage, ptr string, p *Problems) ThreadData {
	m := object(raw, ptr, []string{"channel_origin_key", memberName, "created_at", "last_activity_at", "closed", "closed_at",
		"starter_message_origin_key", "tag_origin_keys", memberEarlierSession}, p)
	if m == nil {
		return ThreadData{}
	}
	var d ThreadData
	d.ChannelOriginKey = keyOf(m, "channel_origin_key", KindChannel, ptr, true, p)
	d.Name, _ = str(m, memberName, ptr, true, p)
	d.CreatedAt = stamp(m, "created_at", ptr, true, p)
	d.LastActivityAt = stamp(m, "last_activity_at", ptr, true, p)
	if raw, ok := m["closed"]; !ok || isNull(raw) {
		p.Add(ptr+"/closed", FieldRequired, "the member is required")
	} else if err := json.Unmarshal(raw, &d.Closed); err != nil {
		p.Add(ptr+"/closed", FieldInvalid, "a boolean is required")
	}
	d.ClosedAt = stamp(m, "closed_at", ptr, false, p)
	switch {
	case d.Closed && d.ClosedAt.IsZero():
		p.Add(ptr+"/closed_at", FieldRequired, "a closed thread needs closed_at")
	case !d.Closed && !d.ClosedAt.IsZero():
		p.Add(ptr+"/closed_at", FieldInvalid, "an open thread has no closed_at")
	}
	d.StarterMessageOriginKey = keyOf(m, "starter_message_origin_key", KindMessage, ptr, false, p)
	if raw, ok := m["tag_origin_keys"]; ok {
		var keys []string
		if err := json.Unmarshal(raw, &keys); err != nil {
			p.Add(ptr+"/tag_origin_keys", FieldInvalid, "an array of forum_tag keys is required")
		}
		for i, k := range keys {
			if _, code := ParseOriginKey(KindForumTag, k); code != "" {
				p.Add(Pointer(ptr+"/tag_origin_keys", i), code, "forum_tag:<channel_id>:<tag_id>")
				continue
			}
			d.TagOriginKeys = append(d.TagOriginKeys, k)
		}
	}
	if raw, ok := m[memberEarlierSession]; ok && !isNull(raw) {
		d.EarlierSession = parseEarlierSession(raw, ptr+"/"+memberEarlierSession, p)
	}
	return d
}

// ParseMessageData checks the data of a message item at ptr: author_origin_key, content_md and
// created_at are required; content_md must be valid UTF-8 (its size and characters are the rules of
// messaging); attachment_file_ids are UUIDs.
func ParseMessageData(raw json.RawMessage, ptr string, p *Problems) MessageData {
	m := object(raw, ptr, []string{"author_origin_key", "content_md", "created_at", "edited_at", "reply_to_origin_key",
		"attachment_file_ids"}, p)
	if m == nil {
		return MessageData{}
	}
	var d MessageData
	d.AuthorOriginKey = keyOf(m, "author_origin_key", KindUser, ptr, true, p)
	if raw, present := m["content_md"]; !present || isNull(raw) {
		p.Add(ptr+"/content_md", FieldRequired, "the member is required")
	} else if err := json.Unmarshal(m["content_md"], &d.ContentMD); err != nil || !utf8.ValidString(d.ContentMD) {
		p.Add(ptr+"/content_md", FieldInvalid, "a string is required")
	}
	d.CreatedAt = stamp(m, "created_at", ptr, true, p)
	d.EditedAt = stamp(m, "edited_at", ptr, false, p)
	d.ReplyToOriginKey = keyOf(m, "reply_to_origin_key", KindMessage, ptr, false, p)
	if raw, ok := m["attachment_file_ids"]; ok {
		var list []string
		if err := json.Unmarshal(raw, &list); err != nil {
			p.Add(ptr+"/attachment_file_ids", FieldInvalid, "an array of UUIDs is required")
		}
		for i, s := range list {
			id, err := ids.Parse(s)
			if err != nil {
				p.Add(Pointer(ptr+"/attachment_file_ids", i), FieldInvalid, "a UUID is required")
				continue
			}
			d.AttachmentFileIDs = append(d.AttachmentFileIDs, id)
		}
	}
	return d
}

// ParsePinData checks the data of a pin item at ptr.
func ParsePinData(raw json.RawMessage, ptr string, p *Problems) PinData {
	m := object(raw, ptr, []string{"message_origin_key"}, p)
	if m == nil {
		return PinData{}
	}
	return PinData{MessageOriginKey: keyOf(m, "message_origin_key", KindMessage, ptr, true, p)}
}

// StoredThread is the stored state of a posted thread the mirror rules compare with.
type StoredThread struct {
	Title string
	// AutoTitle reports title_source auto.
	AutoTitle bool
	Closed    bool
	// LastActivityAt is the stored time of the last activity.
	LastActivityAt time.Time
}

// Mirror is what a posted thread changes in a mirror round (22 12.2 step 3); a nil member and a
// false Reopen change nothing.
type Mirror struct {
	Title          *string
	CloseAt        *time.Time
	Reopen         bool
	LastActivityAt *time.Time
}

// Empty reports a mirror without a change.
func (m Mirror) Empty() bool {
	return m.Title == nil && m.CloseAt == nil && !m.Reopen && m.LastActivityAt == nil
}

// CutTitle cuts name to maxChars code points (thread.title_max_chars), never inside a rune.
func CutTitle(name string, maxChars int) string {
	if r := []rune(name); len(r) > maxChars {
		return string(r[:maxChars])
	}
	return name
}

// MirrorChange returns the changes of a thread item whose thread is posted (22 12.2 step 3, M8-11
// design note 3 b): the title only while it is auto and the cut name differs; closed when the
// source is closed and the stored thread open, reopened in the other case; last_activity_at only
// when the source is later. Nothing else changes.
//
// Example:
//
//	m := MirrorChange(StoredThread{Title: "new thread", AutoTitle: true}, ThreadData{Name: "Login fails"}, 200)
//	// *m.Title == "Login fails"
func MirrorChange(stored StoredThread, d ThreadData, titleMaxChars int) Mirror {
	var m Mirror
	if title := CutTitle(d.Name, titleMaxChars); stored.AutoTitle && title != stored.Title {
		m.Title = &title
	}
	switch {
	case d.Closed && !stored.Closed:
		at := d.ClosedAt
		m.CloseAt = &at
	case !d.Closed && stored.Closed:
		m.Reopen = true
	}
	if d.LastActivityAt.After(stored.LastActivityAt) {
		at := d.LastActivityAt
		m.LastActivityAt = &at
	}
	return m
}

// ParseSHA256 decodes a hex SHA-256 (64 hex digits); ok is false otherwise.
func ParseSHA256(s string) (sum [32]byte, ok bool) {
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != len(sum) {
		return sum, false
	}
	copy(sum[:], b)
	return sum, true
}
