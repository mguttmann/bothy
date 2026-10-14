// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
)

// wantField checks that err is validation_failed with one field error on pointer with code.
func wantField(t *testing.T, err error, pointer, code string) {
	t.Helper()
	e, ok := errs.As(err)
	if !ok || e.Code != errs.CodeValidationFailed || len(e.Fields) != 1 || e.Fields[0].Pointer != pointer || e.Fields[0].Code != code {
		t.Fatalf("error %v, want validation_failed on %s with %s", err, pointer, code)
	}
}

// TestMentionScanner: the token rule of 03 messages "Inhaltsformat" (brief M1-56 D5).
func TestMentionScanner(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, in string
		want     []string
	}{
		{"start of text", "@alex hi", []string{"alex"}},
		{"after punctuation", "hi (@alex), [@sam]", []string{"alex", "sam"}},
		{"e-mail is no mention", "alex@example.com", nil},
		{"after slash, dot, dash, underscore or @", "a/@x b.@y c-@z d_@w @@v", nil},
		{"inline code", "see `@alex` and @sam", []string{"sam"}},
		{"fenced code block", "```\n@alex\n```\n@sam", []string{"sam"}},
		{"indented code block", "    @alex\n\n@sam", []string{"sam"}},
		{"trailing dot cut", "thanks @alex.", []string{"alex"}},
		{"trailing dash and underscore cut", "@sam-_ and @alex_.", []string{"sam", "alex"}},
		{"inner dot kept", "@support.agent please", []string{"support.agent"}},
		{"upper case handle", "@Alex and @ALEX", []string{"alex"}},
		{"duplicates once", "@sam @alex @sam", []string{"sam", "alex"}},
		{"everyone is a plain handle", "@everyone @here", []string{"everyone", "here"}},
		{"handle must start with a letter or digit", "@.alex @-x @_y @", nil},
		{"after a letter of another script", "\u00fc@alex", nil},
		{"after non-breaking space", "hi\u00a0@alex", []string{"alex"}},
	}
	for _, c := range cases {
		if got := domain.ScanMentions(c.in); !slices.Equal(got, c.want) {
			t.Errorf("%s: %q gives %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

// TestRequestHash_StableAndSensitive: the canonical hash of D3 is stable, changes with every
// field, and does not depend on the JSON form of the request.
func TestRequestHash_StableAndSensitive(t *testing.T) {
	t.Parallel()
	base := domain.HashInput{Context: "thread:a", Kind: "text", Content: "hello", ReplyTo: "r", AttachmentIDs: []string{"f1", "f2"}}
	h := domain.RequestHash(base)
	if len(h) != 32 || !bytes.Equal(h, domain.RequestHash(base)) {
		t.Fatalf("hash not stable: %x", h)
	}
	variants := []domain.HashInput{
		{Context: "thread:b", Kind: "text", Content: "hello", ReplyTo: "r", AttachmentIDs: []string{"f1", "f2"}},
		{Context: "thread:a", Kind: "voice", Content: "hello", ReplyTo: "r", AttachmentIDs: []string{"f1", "f2"}},
		{Context: "thread:a", Kind: "text", Content: "hello!", ReplyTo: "r", AttachmentIDs: []string{"f1", "f2"}},
		{Context: "thread:a", Kind: "text", Content: "hello", ReplyTo: "", AttachmentIDs: []string{"f1", "f2"}},
		{Context: "thread:a", Kind: "text", Content: "hello", ReplyTo: "r", AttachmentIDs: []string{"f2", "f1"}},
		{Context: "thread:a", Kind: "text", Content: "hello", ReplyTo: "r", AttachmentIDs: nil},
		// The length prefix keeps field boundaries: moving bytes between fields changes the hash.
		{Context: "thread:a", Kind: "text", Content: "hellor", ReplyTo: "", AttachmentIDs: []string{"f1", "f2"}},
	}
	for i, v := range variants {
		if bytes.Equal(domain.RequestHash(v), h) {
			t.Errorf("variant %d has the same hash", i)
		}
	}
	// Two JSON forms of one request (other key order and white space) decode to the same input.
	var a, b struct {
		Content string `json:"content_md"`
		Kind    string `json:"kind"`
	}
	if err := json.Unmarshal([]byte(`{"kind":"text","content_md":"hello"}`), &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte("{ \"content_md\" : \"hello\",\n \"kind\":\"text\" }"), &b); err != nil {
		t.Fatal(err)
	}
	ha := domain.RequestHash(domain.HashInput{Context: "thread:a", Kind: a.Kind, Content: a.Content})
	hb := domain.RequestHash(domain.HashInput{Context: "thread:a", Kind: b.Kind, Content: b.Content})
	if !bytes.Equal(ha, hb) {
		t.Fatal("JSON key order changes the hash")
	}
}

// TestTruncateForEvent_UTF8Boundary: the cut keeps valid UTF-8 at multi-byte characters and the
// longest prefix that fits.
func TestTruncateForEvent_UTF8Boundary(t *testing.T) {
	t.Parallel()
	content := strings.Repeat("a\u00e9\u20ac\U0001f600", 50)
	for limit := 0; limit < len(content)+2; limit++ {
		fits := func(s string) bool { return len(s) <= limit }
		got, cut := domain.TruncateForEvent(content, fits)
		if !utf8.ValidString(got) || !strings.HasPrefix(content, got) || len(got) > limit && limit < len(content) {
			t.Fatalf("limit %d: %q (cut %v)", limit, got, cut)
		}
		if cut != (limit < len(content)) {
			t.Fatalf("limit %d: cut %v", limit, cut)
		}
		if cut {
			// The next character would not fit.
			_, size := utf8.DecodeRuneInString(content[len(got):])
			if len(got)+size <= limit {
				t.Fatalf("limit %d: %d bytes, not the longest prefix", limit, len(got))
			}
		}
	}
}

// TestOriginFromPrincipalKind: the server sets origin from the principal kind (09 section 3.2).
func TestOriginFromPrincipalKind(t *testing.T) {
	t.Parallel()
	for kind, want := range map[actor.Kind]domain.Origin{
		actor.KindUser: domain.OriginHuman, actor.KindAgent: domain.OriginAgent, actor.KindApp: domain.OriginApp,
	} {
		if got, err := domain.OriginFor(string(kind)); err != nil || got != want {
			t.Errorf("%s: %q, %v", kind, got, err)
		}
	}
	for _, kind := range []actor.Kind{actor.KindHost, actor.KindAnonymous, ""} {
		if _, err := domain.OriginFor(string(kind)); err == nil {
			t.Errorf("%s: no error", kind)
		}
	}
}

// TestCheckContent: text needs a non-space character, at most maxBytes UTF-8 bytes (413 above),
// no U+0000; the bytes are kept as they are.
func TestCheckContent(t *testing.T) {
	t.Parallel()
	if err := domain.CheckContent("  x\u202e  ", 8); err != nil {
		t.Fatalf("CheckContent = %v", err)
	}
	for _, c := range []string{"", " \n\t "} {
		wantField(t, domain.CheckContent(c, 10), "/content_md", domain.FieldEmpty)
	}
	wantField(t, domain.CheckContent("a\x00b", 10), "/content_md", domain.FieldInvalid)
	err := domain.CheckContent("\u00e9\u00e9\u00e9", 5)
	if e, ok := errs.As(err); !ok || e.Code != errs.CodePayloadTooLarge || !strings.Contains(e.Detail, "message.max_content_bytes") {
		t.Fatalf("over the limit: %v", err)
	}
}

// TestCheckClientMsgID: only a UUIDv7 (04 section 5.1 point 5), the time inside is not checked.
func TestCheckClientMsgID(t *testing.T) {
	t.Parallel()
	if err := domain.CheckClientMsgID(ids.New()); err != nil {
		t.Fatal(err)
	}
	old := ids.MustParse("00000001-0000-7000-8000-000000000001")
	if err := domain.CheckClientMsgID(old); err != nil {
		t.Fatalf("old UUIDv7: %v", err)
	}
	v4 := [16]byte{0: 1, 6: 0x40, 8: 0x80}
	wantField(t, domain.CheckClientMsgID(v4), "/client_msg_id", domain.FieldNotUUIDv7)
	wantField(t, domain.CheckClientMsgID([16]byte{}), "/client_msg_id", domain.FieldNotUUIDv7)
}

// TestCheckMentionCount: more than the limit of distinct handles is too_many_mentions.
func TestCheckMentionCount(t *testing.T) {
	t.Parallel()
	if err := domain.CheckMentionCount([]string{"a", "b"}, 2); err != nil {
		t.Fatal(err)
	}
	wantField(t, domain.CheckMentionCount([]string{"a", "b", "c"}, 2), "/content_md", domain.FieldTooManyMentions)
}

// TestSystemLine: the line of a reopen (brief D6).
func TestSystemLine(t *testing.T) {
	t.Parallel()
	author, thread, channel := ids.New(), ids.New(), ids.New()
	m := domain.SystemLine(domain.SystemThreadReopened, thread, channel, author, 7)
	if m.Kind != domain.KindSystem || m.Origin != domain.OriginSystem || m.SystemCode != domain.SystemThreadReopened ||
		m.ContentMD != "" || !m.Silent || m.AuthorID != author || m.ThreadSeq != 7 || m.ThreadID != thread ||
		m.ChannelID != channel || m.ClientMsgID.IsZero() || m.ID.IsZero() || len(m.PayloadSHA256) != 32 || m.Version != 1 {
		t.Fatalf("system line %+v", m)
	}
}
