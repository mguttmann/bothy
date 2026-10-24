// SPDX-License-Identifier: Apache-2.0

package searchbench_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/perf/internal/searchbench"
)

// secret is message content that no failure message may contain.
const secret = "Confidential message body" // #nosec G101 -- message content of a fixture, no credential

const (
	visibleChannel   = "0192f4a0-7b1c-7000-8a00-000000000001"
	invisibleChannel = "0192f4a0-7b1c-7000-8a00-000000000002"
	threadID         = "0192f4a0-7b1c-7000-8a00-000000000003"
	messageID        = "0192f4a0-7b1c-7000-8a00-000000000004"
	authorID         = "0192f4a0-7b1c-7000-8a00-000000000005"
)

// message is a Message object of the API in channel ch with the secret content.
func message(ch string) map[string]any {
	return map[string]any{
		"id": messageID, "thread_id": threadID, "channel_id": ch, "thread_seq": 1,
		"author":        map[string]any{"id": authorID, "kind": "user", "handle": "alex", "display_name": "Alex"},
		"client_msg_id": messageID, "origin": "human", "kind": "text", "content_md": secret, "content_format": 1,
		"mention_ids": []any{}, "attachments": []any{}, "components": []any{}, "reactions": []any{}, "pinned": false,
		"version": 1, "created_at": "2026-10-01T12:00:00.000Z", "silent": false,
	}
}

// body encodes v.
func body(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestChecks: every condition of design note 10 fails with a message naming phase, class and
// query and never the content; a correct answer passes.
func TestChecks(t *testing.T) {
	t.Parallel()
	visible := func(ch string) bool { return ch == visibleChannel }
	hit := func(ch string) map[string]any {
		return map[string]any{"kind": "message", "message": message(ch), "segments": []any{map[string]any{"text": secret, "match": true}}}
	}
	q := searchbench.Query{Searcher: 2, Class: searchbench.ClassPhrase, Text: `"haus baum"`, MustFind: true}
	word := searchbench.Query{Searcher: 2, Class: searchbench.ClassOneWord, Band: searchbench.BandFrequent, Text: "haus"}
	ok := searchbench.Answer{Status: http.StatusOK, Body: body(t, map[string]any{"items": []any{hit(visibleChannel)}, "candidates_capped": true})}
	if capped, err := searchbench.CheckSearch("messages", q, ok, visible); err != nil || !capped {
		t.Fatalf("a correct answer: capped %v, %v", capped, err)
	}
	fileHit := hit(visibleChannel)
	fileHit["kind"] = "file"
	unknown := hit(visibleChannel)
	unknown["secret_member"] = secret
	cases := map[string]struct {
		q searchbench.Query
		a searchbench.Answer
	}{
		"rate limited":       {word, searchbench.Answer{Status: http.StatusTooManyRequests, Body: body(t, map[string]any{"code": "rate_limited", "detail": secret})}},
		"timeout":            {word, searchbench.Answer{Status: http.StatusServiceUnavailable, Body: body(t, map[string]any{"code": "unavailable", "reason": "search_timeout"})}},
		"not json":           {word, searchbench.Answer{Status: http.StatusOK, Body: []byte("{" + secret)}},
		"unknown member":     {word, searchbench.Answer{Status: http.StatusOK, Body: body(t, map[string]any{"items": []any{unknown}})}},
		"other kind":         {word, searchbench.Answer{Status: http.StatusOK, Body: body(t, map[string]any{"items": []any{fileHit}})}},
		"leak":               {word, searchbench.Answer{Status: http.StatusOK, Body: body(t, map[string]any{"items": []any{hit(invisibleChannel)}})}},
		"phrase without hit": {q, searchbench.Answer{Status: http.StatusOK, Body: body(t, map[string]any{"items": []any{}})}},
	}
	for name, c := range cases {
		_, err := searchbench.CheckSearch("messages", c.q, c.a, visible)
		assertFailure(t, name, err, "messages", c.q)
	}
	if _, err := searchbench.CheckSearch("messages", word, searchbench.Answer{Status: http.StatusServiceUnavailable,
		Body: body(t, map[string]any{"code": "unavailable", "reason": "search_timeout"})}, visible); err == nil || !strings.Contains(err.Error(), "search_timeout") {
		t.Errorf("a timeout does not name its reason: %v", err)
	}

	prefix := searchbench.Query{Searcher: 1, Class: searchbench.ClassPrefix, Text: "hau", MustFind: true}
	title := func(ch string) map[string]any {
		return map[string]any{"thread_id": threadID, "channel_id": ch, "title": secret, "last_activity_at": "2026-10-01T12:00:00.000Z"}
	}
	if err := searchbench.CheckPrefix("prefix", prefix, searchbench.Answer{Status: http.StatusOK, Body: body(t, map[string]any{"items": []any{title(visibleChannel)}})}, visible); err != nil {
		t.Fatalf("a correct prefix answer: %v", err)
	}
	for name, a := range map[string]searchbench.Answer{
		"prefix leak":    {Status: http.StatusOK, Body: body(t, map[string]any{"items": []any{title(invisibleChannel)}})},
		"prefix empty":   {Status: http.StatusOK, Body: body(t, map[string]any{"items": []any{}})},
		"prefix status":  {Status: http.StatusTooManyRequests, Body: body(t, map[string]any{"code": "rate_limited"})},
		"prefix unknown": {Status: http.StatusOK, Body: body(t, map[string]any{"items": []any{}, "more": secret})},
	} {
		assertFailure(t, name, searchbench.CheckPrefix("prefix", prefix, a, visible), "prefix", prefix)
	}

	probe := searchbench.Query{Searcher: 0, Class: searchbench.ClassProbe, Text: "qxalor"}
	if err := searchbench.CheckProbe("probe", probe, searchbench.Answer{Status: http.StatusOK, Body: body(t, map[string]any{"items": []any{}, "candidates_capped": false})}); err != nil {
		t.Fatalf("a correct probe answer: %v", err)
	}
	for name, a := range map[string]searchbench.Answer{
		"probe item":   {Status: http.StatusOK, Body: body(t, map[string]any{"items": []any{hit(invisibleChannel)}})},
		"probe capped": {Status: http.StatusOK, Body: body(t, map[string]any{"items": []any{}, "candidates_capped": true})},
		"probe cursor": {Status: http.StatusOK, Body: body(t, map[string]any{"items": []any{}, "next_cursor": "abc"})},
		"probe status": {Status: http.StatusInternalServerError, Body: []byte(secret)},
	} {
		assertFailure(t, name, searchbench.CheckProbe("probe", probe, a), "probe", probe)
	}
}

// assertFailure fails t unless err names phase, class and query and holds no content.
func assertFailure(t *testing.T, name string, err error, phase string, q searchbench.Query) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: passed", name)
		return
	}
	msg := err.Error()
	if !strings.Contains(msg, phase) || !strings.Contains(msg, string(q.Class)) || !strings.Contains(msg, q.Text) {
		t.Errorf("%s: %q does not name phase, class and query", name, msg)
	}
	if strings.Contains(msg, secret) || strings.Contains(msg, "Confidential") {
		t.Errorf("%s: the failure prints content: %q", name, msg)
	}
}
