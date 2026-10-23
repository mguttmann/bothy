// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/apps/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The UUIDs of the reply examples.
const (
	replyMsgID   = "0190a1b2-c3d4-7e5f-8a6b-7c8d9e0f1a2b"
	replyQuoteID = "0190a1b2-c3d4-7e5f-8a6b-7c8d9e0f1a2c"
	replyFileID  = "0190a1b2-c3d4-7e5f-8a6b-7c8d9e0f1a2d"
	replyAlexID  = "0190a1b2-c3d4-7e5f-8a6b-7c8d9e0f1a2e"
)

// fullReply is a MessageCreate with every member of the schema.
var fullReply = `{"client_msg_id":"` + replyMsgID + `","kind":"card","content_md":"Ticket for @alex created.",` +
	`"reply_to_message_id":"` + replyQuoteID + `","attachment_file_ids":["` + replyFileID + `"],` +
	`"mention_ids":["` + replyAlexID + `"],"silent":true,"clear_draft":false,"run_id":"` + replyQuoteID + `",` +
	`"components":[{"custom_id":"close","type":"button","definition":{"label":"Close","style":"primary"},` +
	`"single_use":true,"required_capability":"approve_actions","restricted_to":"owner","expires_at":"2026-10-23T12:00:00.000Z"}]}`

// TestDecodeAppReply holds the strict decoding of the answers of an interaction address (M2-25
// design note 9 b 5 and d): a valid MessageCreate and AutocompleteAnswer decode; an unknown member,
// a duplicate key at any depth, a JSON null, invalid UTF-8, a missing client_msg_id and a body that
// is no JSON are invalid; the M2-17 rules of an answer hold for the choices.
func TestDecodeAppReply(t *testing.T) {
	t.Parallel()
	t.Run("valid MessageCreate", func(t *testing.T) {
		t.Parallel()
		r, err := domain.DecodeAppReply([]byte(fullReply))
		if err != nil {
			t.Fatalf("DecodeAppReply(full) = %v", err)
		}
		want := domain.Reply{
			ClientMsgID: ids.MustParse(replyMsgID), Kind: ptr("card"), ContentMD: ptr("Ticket for @alex created."),
			ReplyToMessageID: ptr(ids.MustParse(replyQuoteID)), AttachmentFileIDs: []ids.ID{ids.MustParse(replyFileID)},
			MentionIDs: []ids.ID{ids.MustParse(replyAlexID)}, Silent: true, RunID: ptr(ids.MustParse(replyQuoteID)),
		}
		if r.ClientMsgID != want.ClientMsgID || *r.Kind != *want.Kind || *r.ContentMD != *want.ContentMD ||
			*r.ReplyToMessageID != *want.ReplyToMessageID || fmt.Sprint(r.AttachmentFileIDs) != fmt.Sprint(want.AttachmentFileIDs) ||
			fmt.Sprint(r.MentionIDs) != fmt.Sprint(want.MentionIDs) || !r.Silent || r.ClearDraft || *r.RunID != *want.RunID {
			t.Fatalf("DecodeAppReply(full) members differ")
		}
		if len(r.Components) != 1 {
			t.Fatalf("components = %d, want 1", len(r.Components))
		}
		c := r.Components[0]
		if c.CustomID != "close" || c.Type != "button" || c.SingleUse == nil || !*c.SingleUse || c.RequiredCapability != "approve_actions" ||
			c.RestrictedTo != "owner" || c.ExpiresAt == nil || c.ExpiresAt.UTC().Format("15:04") != "12:00" {
			t.Fatalf("component members differ")
		}
		var def map[string]string
		if err := json.Unmarshal(c.Definition, &def); err != nil || def["label"] != "Close" {
			t.Fatalf("definition not kept as JSON object: %v", err)
		}
	})
	t.Run("minimal MessageCreate keeps absent members absent", func(t *testing.T) {
		t.Parallel()
		r, err := domain.DecodeAppReply([]byte(`{"client_msg_id":"` + replyMsgID + `","content_md":"ok"}`))
		if err != nil {
			t.Fatalf("DecodeAppReply(minimal) = %v", err)
		}
		if r.Kind != nil || r.ReplyToMessageID != nil || r.AttachmentFileIDs != nil || r.MentionIDs != nil || r.RunID != nil ||
			r.Components != nil || r.Silent || r.ClearDraft {
			t.Fatalf("absent members became present")
		}
	})
	invalid := map[string]string{
		"unknown member":                `{"client_msg_id":"` + replyMsgID + `","content_md":"ok","origin":"system"}`,
		"unknown member in a component": `{"client_msg_id":"` + replyMsgID + `","kind":"card","components":[{"custom_id":"a","type":"button","definition":{},"run_id":"x"}]}`,
		"member in another case":        `{"Client_Msg_ID":"` + replyMsgID + `","content_md":"ok"}`,
		"duplicate key":                 `{"client_msg_id":"` + replyMsgID + `","content_md":"a","content_md":"b"}`,
		"duplicate key in a definition": `{"client_msg_id":"` + replyMsgID + `","kind":"card","components":[{"custom_id":"a","type":"button","definition":{"label":"x","label":"y"}}]}`,
		"JSON null":                     `{"client_msg_id":"` + replyMsgID + `","content_md":null}`,
		"JSON null in a definition":     `{"client_msg_id":"` + replyMsgID + `","kind":"card","components":[{"custom_id":"a","type":"button","definition":{"label":null}}]}`,
		"JSON null in a list":           `{"client_msg_id":"` + replyMsgID + `","mention_ids":[null]}`,
		"top-level null":                `null`,
		"missing client_msg_id":         `{"content_md":"ok"}`,
		"client_msg_id no uuid":         `{"client_msg_id":"ticket-1","content_md":"ok"}`,
		"not JSON":                      `<html>ok</html>`,
		"empty body":                    ``,
		"two values":                    `{"client_msg_id":"` + replyMsgID + `"} {}`,
		"array":                         `[]`,
		"wrong type":                    `{"client_msg_id":"` + replyMsgID + `","silent":"yes"}`,
		"kind outside the enum":         `{"client_msg_id":"` + replyMsgID + `","kind":"system","content_md":"ok"}`,
		"component type outside enum":   `{"client_msg_id":"` + replyMsgID + `","kind":"card","components":[{"custom_id":"a","type":"slider","definition":{}}]}`,
		"capability outside the enum":   `{"client_msg_id":"` + replyMsgID + `","kind":"card","components":[{"custom_id":"a","type":"button","definition":{},"required_capability":"root"}]}`,
		"restriction outside the enum":  `{"client_msg_id":"` + replyMsgID + `","kind":"card","components":[{"custom_id":"a","type":"button","definition":{},"restricted_to":"me"}]}`,
		"definition no object":          `{"client_msg_id":"` + replyMsgID + `","kind":"card","components":[{"custom_id":"a","type":"button","definition":[1]}]}`,
		"component without definition":  `{"client_msg_id":"` + replyMsgID + `","kind":"card","components":[{"custom_id":"a","type":"button"}]}`,
		"component without custom_id":   `{"client_msg_id":"` + replyMsgID + `","kind":"card","components":[{"type":"button","definition":{}}]}`,
		"component without type":        `{"client_msg_id":"` + replyMsgID + `","kind":"card","components":[{"custom_id":"a","definition":{}}]}`,
		"expires_at no timestamp":       `{"client_msg_id":"` + replyMsgID + `","kind":"card","components":[{"custom_id":"a","type":"button","definition":{},"expires_at":"tomorrow"}]}`,
	}
	for name, body := range invalid {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := domain.DecodeAppReply([]byte(body)); !errors.Is(err, domain.ErrInvalidReply) {
				t.Fatalf("DecodeAppReply = %v, want ErrInvalidReply", err)
			}
		})
	}
	t.Run("invalid UTF-8", func(t *testing.T) {
		t.Parallel()
		body := []byte(`{"client_msg_id":"` + replyMsgID + `","content_md":"ok"}`)
		body[len(body)-4] = 0xff
		if _, err := domain.DecodeAppReply(body); !errors.Is(err, domain.ErrInvalidReply) {
			t.Fatalf("DecodeAppReply(invalid UTF-8) = %v, want ErrInvalidReply", err)
		}
	})
	t.Run("the error repeats no content", func(t *testing.T) {
		t.Parallel()
		_, err := domain.DecodeAppReply([]byte(`{"client_msg_id":"` + replyMsgID + `","secret-member":"canary-value"}`))
		if err == nil || strings.Contains(err.Error(), "canary") || strings.Contains(err.Error(), "secret-member") {
			t.Fatalf("error repeats the body: %v", err)
		}
	})

	const maxChoices = 3
	t.Run("valid AutocompleteAnswer", func(t *testing.T) {
		t.Parallel()
		got, err := domain.DecodeAutocompleteReply([]byte(`{"choices":[{"name":"First line","value":"first"},{"value":"second","name":"Second line"}]}`), maxChoices)
		want := []domain.Choice{{Name: "First line", Value: "first"}, {Name: "Second line", Value: "second"}}
		if err != nil || fmt.Sprint(got) != fmt.Sprint(want) {
			t.Fatalf("DecodeAutocompleteReply = %v %v, want %v", got, err, want)
		}
		empty, err := domain.DecodeAutocompleteReply([]byte(`{"choices":[]}`), maxChoices)
		if err != nil || empty == nil || len(empty) != 0 {
			t.Fatalf("DecodeAutocompleteReply(empty) = %#v %v, want an empty list", empty, err)
		}
	})
	invalidChoices := map[string]string{
		"too many choices":         `{"choices":[{"name":"a","value":"a"},{"name":"b","value":"b"},{"name":"c","value":"c"},{"name":"d","value":"d"}]}`,
		"empty name":               `{"choices":[{"name":"","value":"a"}]}`,
		"empty value":              `{"choices":[{"name":"a","value":""}]}`,
		"bidi character in a name": "{\"choices\":[{\"name\":\"a\u202eb\",\"value\":\"a\"}]}",
		"control character":        `{"choices":[{"name":"a","value":"a\u0007"}]}`,
		"missing choices":          `{}`,
		"choice without value":     `{"choices":[{"name":"a"}]}`,
		"unknown member":           `{"choices":[],"more":true}`,
		"unknown member in choice": `{"choices":[{"name":"a","value":"a","icon":"x"}]}`,
		"duplicate key":            `{"choices":[{"name":"a","name":"b","value":"a"}]}`,
		"JSON null":                `{"choices":null}`,
		"MessageCreate":            `{"client_msg_id":"` + replyMsgID + `","content_md":"ok"}`,
		"not JSON":                 `choices`,
	}
	for name, body := range invalidChoices {
		t.Run("autocomplete "+name, func(t *testing.T) {
			t.Parallel()
			if _, err := domain.DecodeAutocompleteReply([]byte(body), maxChoices); !errors.Is(err, domain.ErrInvalidReply) {
				t.Fatalf("DecodeAutocompleteReply = %v, want ErrInvalidReply", err)
			}
		})
	}
}

// TestStatusToOutcome holds the table from the answer of an interaction address to its outcome
// (M2-25 design note 9 b 5): 200 is answered (the body decides then), 202 deferred, a 3xx redirect,
// every other status http_status; a deadline is timeout, a refusal of the egress client and a host
// outside webhook.allowed_hosts are blocked, a body above http.max_json_body_bytes too_large, any
// other transport error transport.
func TestStatusToOutcome(t *testing.T) {
	t.Parallel()
	wrapped := func(err error) error { return fmt.Errorf("post: %w", err) }
	cases := []struct {
		name   string
		status int
		err    error
		want   string
	}{
		{name: "200", status: 200, want: domain.ReplyAnswered},
		{name: "202", status: 202, want: domain.ReplyDeferred},
		{name: "204", status: 204, want: domain.ReasonHTTPStatus},
		{name: "201", status: 201, want: domain.ReasonHTTPStatus},
		{name: "302", status: 302, want: domain.ReasonRedirect},
		{name: "307", status: 307, want: domain.ReasonRedirect},
		{name: "404", status: 404, want: domain.ReasonHTTPStatus},
		{name: "500", status: 500, want: domain.ReasonHTTPStatus},
		{name: "deadline", err: wrapped(context.DeadlineExceeded), want: domain.ReasonTimeout},
		{name: "net timeout", err: wrapped(timeoutError{}), want: domain.ReasonTimeout},
		{name: "deadline while reading a 200", status: 200, err: context.DeadlineExceeded, want: domain.ReasonTimeout},
		{name: "blocked by the egress client", err: wrapped(&domain.BlockedError{Reason: "address_blocked"}), want: domain.ReasonBlocked},
		{name: "host not in webhook.allowed_hosts", err: domain.ErrHostNotAllowed, want: domain.ReasonBlocked},
		{name: "invalid stored address", err: domain.ErrInvalidTarget, want: domain.ReasonBlocked},
		{name: "too large", status: 200, err: wrapped(domain.ErrReplyTooLarge), want: domain.ReasonTooLarge},
		{name: "connection refused", err: errors.New("dial tcp: connection refused"), want: domain.ReasonTransport},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := domain.OutcomeOf(c.status, c.err); got != c.want {
				t.Fatalf("OutcomeOf(%d, %v) = %q, want %q", c.status, c.err, got, c.want)
			}
		})
	}
}

// TestReplyReasons: the closed list holds every reason once and in the order of the specification.
func TestReplyReasons(t *testing.T) {
	t.Parallel()
	want := []string{"inactive", "no_url", "denied", "timeout", "transport", "blocked", "redirect", "http_status", "too_large",
		"invalid_reply", "no_thread", "reply_refused"}
	if got := domain.ReplyReasons(); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("ReplyReasons() = %v, want %v", got, want)
	}
}

// ptr returns a pointer to v.
func ptr[T any](v T) *T { return &v }
