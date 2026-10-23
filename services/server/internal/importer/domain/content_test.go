// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/importer/domain"
)

// pointers returns the pointer and code of every recorded violation.
func pointers(p *domain.Problems) map[string]string {
	out := map[string]string{}
	for _, f := range p.Fields() {
		out[f.Pointer] = f.Code
	}
	return out
}

// TestThreadBatchShape holds design note 1 of M8-11: the key forms, the thread item only first and
// only with its own key in a thread: batch, then messages and pins only.
func TestThreadBatchShape(t *testing.T) {
	t.Parallel()
	msg := domain.ItemRef{Kind: "message", OriginKey: "message:11"}
	pin := domain.ItemRef{Kind: "pin", OriginKey: "pin:11"}
	cases := []struct {
		name  string
		key   string
		items []domain.ItemRef
		want  map[string]string
		kind  string
	}{
		{"thread with its item", "thread:10", []domain.ItemRef{{Kind: "thread", OriginKey: "thread:10"}, msg, pin}, map[string]string{}, "thread"},
		{"thread without its item", "thread:10", []domain.ItemRef{msg, pin}, map[string]string{}, "thread"},
		{"main thread of a channel", "channel:20", []domain.ItemRef{msg}, map[string]string{}, "channel"},
		{"thread item in a channel batch", "channel:20", []domain.ItemRef{{Kind: "thread", OriginKey: "thread:10"}},
			map[string]string{"/items/0/origin_key": domain.FieldInvalid}, "channel"},
		{"thread item of another thread", "thread:10", []domain.ItemRef{{Kind: "thread", OriginKey: "thread:12"}},
			map[string]string{"/items/0/origin_key": domain.FieldInvalid}, "thread"},
		{"thread item not first", "thread:10", []domain.ItemRef{msg, {Kind: "thread", OriginKey: "thread:10"}},
			map[string]string{"/items/1/item_kind": domain.FieldInvalid}, "thread"},
		{"structure item", "thread:10", []domain.ItemRef{{Kind: "user", OriginKey: "user:1"}},
			map[string]string{"/items/0/item_kind": domain.FieldInvalid}, "thread"},
		{"other key form", "message:10", []domain.ItemRef{msg}, map[string]string{"/thread_origin_key": domain.FieldInvalid}, ""},
		{"no digits", "thread:abc", []domain.ItemRef{msg}, map[string]string{"/thread_origin_key": domain.FieldInvalid}, ""},
		{"empty key", "", []domain.ItemRef{msg}, map[string]string{"/thread_origin_key": domain.FieldInvalid}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			var p domain.Problems
			k := domain.CheckThreadBatch(c.key, c.items, &p)
			got := pointers(&p)
			if len(got) != len(c.want) {
				t.Fatalf("problems %v, want %v", got, c.want)
			}
			for ptr, code := range c.want {
				if got[ptr] != code {
					t.Errorf("%s = %q, want %q", ptr, got[ptr], code)
				}
			}
			if k.Kind != c.kind {
				t.Errorf("kind %q, want %q", k.Kind, c.kind)
			}
		})
	}
}

// TestMirrorChange holds 22 12.2 step 3 (M8-11 design note 3 b): an auto title follows a rename, a
// manual one never; archive and reopen; an older last_activity_at is ignored.
func TestMirrorChange(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2025, 3, 1, 8, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Hour)
	cases := []struct {
		name     string
		stored   domain.StoredThread
		data     domain.ThreadData
		title    string
		closeAt  time.Time
		reopen   bool
		activity time.Time
	}{
		{"auto title renamed", domain.StoredThread{Title: "old", AutoTitle: true, LastActivityAt: t0},
			domain.ThreadData{Name: "Login fails", LastActivityAt: t0}, "Login fails", time.Time{}, false, time.Time{}},
		{"manual title untouched", domain.StoredThread{Title: "old", LastActivityAt: t0},
			domain.ThreadData{Name: "Login fails", LastActivityAt: t0}, "", time.Time{}, false, time.Time{}},
		{"same auto title", domain.StoredThread{Title: "same", AutoTitle: true, LastActivityAt: t0},
			domain.ThreadData{Name: "same", LastActivityAt: t0}, "", time.Time{}, false, time.Time{}},
		{"archived", domain.StoredThread{Title: "x", LastActivityAt: t0},
			domain.ThreadData{Name: "x", Closed: true, ClosedAt: t1, LastActivityAt: t0}, "", t1, false, time.Time{}},
		{"closed stays closed", domain.StoredThread{Title: "x", Closed: true, LastActivityAt: t0},
			domain.ThreadData{Name: "x", Closed: true, ClosedAt: t1, LastActivityAt: t0}, "", time.Time{}, false, time.Time{}},
		{"reopened", domain.StoredThread{Title: "x", Closed: true, LastActivityAt: t0},
			domain.ThreadData{Name: "x", LastActivityAt: t0}, "", time.Time{}, true, time.Time{}},
		{"later activity", domain.StoredThread{Title: "x", LastActivityAt: t0},
			domain.ThreadData{Name: "x", LastActivityAt: t1}, "", time.Time{}, false, t1},
		{"older activity ignored", domain.StoredThread{Title: "x", LastActivityAt: t1},
			domain.ThreadData{Name: "x", LastActivityAt: t0}, "", time.Time{}, false, time.Time{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m := domain.MirrorChange(c.stored, c.data, 200)
			if (m.Title == nil) != (c.title == "") || (m.Title != nil && *m.Title != c.title) {
				t.Errorf("title %v, want %q", m.Title, c.title)
			}
			if (m.CloseAt == nil) != c.closeAt.IsZero() || (m.CloseAt != nil && !m.CloseAt.Equal(c.closeAt)) {
				t.Errorf("close %v, want %v", m.CloseAt, c.closeAt)
			}
			if m.Reopen != c.reopen {
				t.Errorf("reopen %v, want %v", m.Reopen, c.reopen)
			}
			if (m.LastActivityAt == nil) != c.activity.IsZero() || (m.LastActivityAt != nil && !m.LastActivityAt.Equal(c.activity)) {
				t.Errorf("activity %v, want %v", m.LastActivityAt, c.activity)
			}
			if m.Empty() != (c.title == "" && c.closeAt.IsZero() && !c.reopen && c.activity.IsZero()) {
				t.Errorf("empty %v", m.Empty())
			}
		})
	}
}

// TestCutTitle cuts at code points, never inside a rune.
func TestCutTitle(t *testing.T) {
	t.Parallel()
	if got := domain.CutTitle("äöü", 2); got != "äö" {
		t.Errorf("CutTitle = %q", got)
	}
	if got := domain.CutTitle("short", 200); got != "short" {
		t.Errorf("CutTitle = %q", got)
	}
}

// TestParseContentData holds the schemas ImportThreadData, ImportMessageData and ImportPinData.
func TestParseContentData(t *testing.T) {
	t.Parallel()
	var p domain.Problems
	th := domain.ParseThreadData(json.RawMessage(`{"channel_origin_key":"channel:20","name":"n","created_at":"2025-03-01T08:00:00Z",
		"last_activity_at":"2025-03-01T09:00:00.123Z","closed":true,"closed_at":"2025-03-02T08:00:00Z",
		"starter_message_origin_key":"message:10","tag_origin_keys":["forum_tag:20:9"]}`), "/d", &p)
	if !p.Empty() || !th.Closed || th.ClosedAt.IsZero() || th.StarterMessageOriginKey != "message:10" || len(th.TagOriginKeys) != 1 {
		t.Fatalf("thread %+v %v", th, p.Fields())
	}
	p = domain.Problems{}
	domain.ParseThreadData(json.RawMessage(`{"channel_origin_key":"space:20","name":"n","created_at":"x","last_activity_at":"2025-03-01T09:00:00Z",
		"closed":true,"tag_origin_keys":["forum_tag:20"],"extra":1}`), "/d", &p)
	got := pointers(&p)
	for _, ptr := range []string{"/d/channel_origin_key", "/d/created_at", "/d/closed_at", "/d/tag_origin_keys/0", "/d/extra"} {
		if got[ptr] == "" {
			t.Errorf("no problem at %s: %v", ptr, got)
		}
	}
	p = domain.Problems{}
	m := domain.ParseMessageData(json.RawMessage(`{"author_origin_key":"user:1","content_md":"hi","created_at":"2025-03-01T08:00:00Z",
		"edited_at":"2025-03-01T08:05:00Z","reply_to_origin_key":"message:9","attachment_file_ids":["0192f4a0-7b1c-72d4-8bae-7661d2b2f203"]}`), "/d", &p)
	if !p.Empty() || m.EditedAt.IsZero() || m.ReplyToOriginKey != "message:9" || len(m.AttachmentFileIDs) != 1 {
		t.Fatalf("message %+v %v", m, p.Fields())
	}
	p = domain.Problems{}
	domain.ParseMessageData(json.RawMessage(`{"author_origin_key":"user:1","created_at":"2025-03-01T08:00:00Z","attachment_file_ids":["x"]}`), "/d", &p)
	got = pointers(&p)
	if got["/d/content_md"] != domain.FieldRequired || got["/d/attachment_file_ids/0"] != domain.FieldInvalid {
		t.Errorf("message problems %v", got)
	}
	p = domain.Problems{}
	if pin := domain.ParsePinData(json.RawMessage(`{"message_origin_key":"message:9"}`), "/d", &p); !p.Empty() || pin.MessageOriginKey != "message:9" {
		t.Errorf("pin %+v %v", pin, p.Fields())
	}
	if _, ok := domain.ParseSHA256("ab"); ok {
		t.Error("a short digest parsed")
	}
}
