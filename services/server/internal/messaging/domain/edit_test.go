// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi-validator/schema_validation"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
)

// TestEditRules (M1-58): an edit with the same content and the same mention set (in any order) is
// no change, as is an empty body, which keeps both; a run anchor stores no revision, every other
// kind does.
func TestEditRules(t *testing.T) {
	t.Parallel()
	a, b := ids.New(), ids.New()
	cases := []struct {
		name                   string
		oldContent, newContent string
		oldMentions, newMent   []ids.ID
		want                   bool
	}{
		{"same content and mentions", "@alex hi", "@alex hi", []ids.ID{a, b}, []ids.ID{b, a}, true},
		{"empty body keeps both", "hi", "hi", []ids.ID{}, nil, true},
		{"other content", "hi", "hi!", nil, nil, false},
		{"other mentions", "@alex hi", "@alex hi", []ids.ID{a}, []ids.ID{a, b}, false},
		{"mention gone", "@alex hi", "@alex hi", []ids.ID{a}, nil, false},
		{"byte for byte", "é", "é", nil, nil, false},
	}
	for _, c := range cases {
		if got := domain.Unchanged(c.oldContent, c.oldMentions, c.newContent, c.newMent); got != c.want {
			t.Errorf("%s: Unchanged %v, want %v", c.name, got, c.want)
		}
	}
	for k, want := range map[domain.Kind]bool{
		domain.KindText: true, domain.KindVoice: true, domain.KindCard: true, domain.KindSystem: true, domain.KindRun: false,
	} {
		if got := domain.StoresRevision(k); got != want {
			t.Errorf("StoresRevision(%s) %v, want %v", k, got, want)
		}
	}
}

// TestTombstone (M1-58): the tombstone keeps every stored field except the content, which it
// empties, and sets the deletion and the next version.
func TestTombstone(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	m := domain.Message{ID: ids.New(), ThreadID: ids.New(), ThreadSeq: 7, AuthorID: ids.New(), ContentMD: "secret", Version: 3,
		Kind: domain.KindText, EditedAt: at.Add(-time.Hour), CreatedAt: at.Add(-2 * time.Hour)}
	by := ids.New()
	got := domain.Tombstone(m, by, at)
	want := m
	want.ContentMD, want.DeletedAt, want.DeletedBy, want.Version = "", at, by, 4
	if got.ContentMD != "" || !got.DeletedAt.Equal(at) || got.DeletedBy != by || got.Version != 4 || got.ThreadSeq != 7 ||
		got.ID != m.ID || !got.EditedAt.Equal(m.EditedAt) {
		t.Fatalf("tombstone %+v, want %+v", got, want)
	}
	if m.ContentMD != "secret" {
		t.Fatal("Tombstone changed its argument")
	}
}

// tombstoneJSON is the event form of a deleted message (schema Message) the use case passes.
const tombstoneJSON = `{"id":"0192f4a0-7b1c-7353-8d8c-dea659467151","thread_id":"0192f4a0-7b1c-78f9-8074-fa94e84bab7d",` +
	`"channel_id":"0192f4a0-7b1c-72d4-8bae-7661d2b2f203","thread_seq":42,"author":{"id":"0192f4a0-7b1c-7f89-8d6b-6960453241bc",` +
	`"kind":"user","handle":"alex","display_name":"Alex"},"client_msg_id":"0192f4a0-7b1c-7f4e-8959-c3574b6ad93b","origin":"human",` +
	`"kind":"text","content_md":"","content_format":1,"silent":false,"attachments":[],"mention_ids":[],"reactions":[],"pinned":false,` +
	`"components":[],"version":4,"deleted_at":"2026-10-14T09:00:00.000Z","created_at":"2026-10-14T08:00:00.000Z"}`

// validate checks p against the schema name of docs/api/openapi.yaml.
func validate(t *testing.T, name string, p json.RawMessage) {
	t.Helper()
	doc, err := libopenapi.NewDocument(rest.Spec)
	if err != nil {
		t.Fatal(err)
	}
	model, err := doc.BuildV3Model()
	if err != nil {
		t.Fatal(err)
	}
	proxy, ok := model.Model.Components.Schemas.Get(name)
	if !ok {
		t.Fatalf("no schema %s", name)
	}
	if valid, problems := schema_validation.NewSchemaValidator().ValidateSchemaString(proxy.Schema(), string(p)); !valid {
		for _, pr := range problems {
			t.Errorf("schema %s: %s %s", name, pr.Message, pr.Reason)
		}
		t.Fatalf("payload does not match %s: %s", name, p)
	}
	if strings.Contains(string(p), "null") {
		t.Fatalf("null in %s", p)
	}
}

// TestRedactPayload (M1-58, 04 section 2.2 rule 5): message.created and message.updated carry the
// tombstone, reactions keep their ids with an empty emoji and count 0, pins stay unchanged, and
// any other type or schema version fails; every result validates against its schema.
func TestRedactPayload(t *testing.T) {
	t.Parallel()
	fn := domain.RedactPayload(json.RawMessage(tombstoneJSON))
	created := `{"message":{"id":"0192f4a0-7b1c-7353-8d8c-dea659467151","content_md":"secret canary","content_truncated":true}}`
	for _, typ := range []string{"message.created", "message.updated"} {
		out, err := fn(typ, 1, json.RawMessage(created))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(out), "canary") || string(out) != `{"message":`+tombstoneJSON+`}` {
			t.Fatalf("%s: %s", typ, out)
		}
		validate(t, "MessagePayload", out)
	}
	reaction := `{"message_id":"0192f4a0-7b1c-7353-8d8c-dea659467151","thread_id":"0192f4a0-7b1c-78f9-8074-fa94e84bab7d",` +
		`"emoji":"👍","principal_id":"0192f4a0-7b1c-7f89-8d6b-6960453241bc","count":3}`
	for _, typ := range []string{"reaction.added", "reaction.removed"} {
		out, err := fn(typ, 1, json.RawMessage(reaction))
		if err != nil {
			t.Fatal(err)
		}
		var got map[string]any
		if err := json.Unmarshal(out, &got); err != nil {
			t.Fatal(err)
		}
		if got["emoji"] != "" || got["count"] != float64(0) || got["message_id"] != "0192f4a0-7b1c-7353-8d8c-dea659467151" ||
			got["principal_id"] != "0192f4a0-7b1c-7f89-8d6b-6960453241bc" || got["thread_id"] != "0192f4a0-7b1c-78f9-8074-fa94e84bab7d" {
			t.Fatalf("%s: %s", typ, out)
		}
		validate(t, "ReactionPayload", out)
	}
	pin := `{"pin":{"message_id":"0192f4a0-7b1c-7353-8d8c-dea659467151"}}`
	for _, typ := range []string{"pin.added", "pin.removed"} {
		if out, err := fn(typ, 1, json.RawMessage(pin)); err != nil || string(out) != pin {
			t.Fatalf("%s: %s %v", typ, out, err)
		}
	}
	for _, c := range []struct {
		typ string
		v   int
		p   string
	}{
		{"message.deleted", 1, `{}`}, {"thread.updated", 1, `{}`}, {"message.created", 2, created},
		{"reaction.added", 1, `{"emoji":"x"}`}, {"reaction.added", 1, `[]`},
	} {
		_, err := fn(c.typ, c.v, json.RawMessage(c.p))
		if e, ok := errs.As(err); !ok || e.Code != errs.CodeInternalError {
			t.Errorf("%s v%d %s: %v, want internal", c.typ, c.v, c.p, err)
		}
	}
}
