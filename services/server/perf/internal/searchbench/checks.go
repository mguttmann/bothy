// SPDX-License-Identifier: Apache-2.0

package searchbench

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/OWNER/bothy/services/server/api/rest"
)

// Answer is what a check sees of a response: status and body. Checks never print the body: it can
// hold message content.
type Answer struct {
	Status int
	Body   []byte
}

// CheckError is a failed check of design note 10. Its message names the phase, the query class,
// the query (synthetic words), counts and ids, never message content.
type CheckError struct {
	Phase string
	Query Query
	What  string
}

func (e *CheckError) Error() string {
	return fmt.Sprintf("%s, searcher %d, class %s, band %q, query '%s': %s", e.Phase, e.Query.Searcher, e.Query.Class, e.Query.Band, e.Query.Text, e.What)
}

// fail returns a CheckError.
func fail(phase string, q Query, format string, args ...any) error {
	return &CheckError{Phase: phase, Query: q, What: fmt.Sprintf(format, args...)}
}

// problem returns the code and reason of a problem body, nothing else of it.
func problem(body []byte) string {
	var p struct {
		Code   string `json:"code"`
		Reason string `json:"reason"`
	}
	if json.Unmarshal(body, &p) != nil {
		return "a body that is no problem document"
	}
	if p.Reason != "" {
		return "code " + p.Code + ", reason " + p.Reason
	}
	return "code " + p.Code
}

// decodeStrict decodes body into v and rejects unknown members and trailing data.
func decodeStrict(body []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.More() {
		return errors.New("data after the document")
	}
	return nil
}

// decodeError names the error of a strict decode without the body: a json.SyntaxError or
// UnmarshalTypeError carries offsets and field names only, the unknown-field error the member
// name.
func decodeError(err error) string {
	var syntax *json.SyntaxError
	var typ *json.UnmarshalTypeError
	switch {
	case errors.As(err, &syntax):
		return fmt.Sprintf("syntax error at offset %d", syntax.Offset)
	case errors.As(err, &typ):
		return fmt.Sprintf("type %s at %s", typ.Value, typ.Field)
	default:
		return err.Error()
	}
}

// CheckSearch checks an answer of phase messages (design note 10): status 200, a body that decodes
// strictly into SearchHitPage, every hit of kind message from a channel visible reports readable,
// and at least one hit when the query must find something. It returns whether the answer reported
// candidates_capped.
func CheckSearch(phase string, q Query, a Answer, visible func(channelID string) bool) (bool, error) {
	if a.Status != http.StatusOK {
		return false, fail(phase, q, "status %d (%s)", a.Status, problem(a.Body))
	}
	var page rest.SearchHitPage
	if err := decodeStrict(a.Body, &page); err != nil {
		return false, fail(phase, q, "the body is no SearchHitPage: %s", decodeError(err))
	}
	for i, h := range page.Items {
		if h.Kind != "message" || h.Message == nil {
			return false, fail(phase, q, "hit %d has kind %q", i, h.Kind)
		}
		if ch := h.Message.ChannelId.String(); !visible(ch) {
			return false, fail(phase, q, "hit %d, message %s, comes from channel %s the searcher may not read", i, h.Message.Id, ch)
		}
	}
	if q.MustFind && len(page.Items) == 0 {
		return false, fail(phase, q, "no hit for a query built from a visible message")
	}
	return page.CandidatesCapped != nil && *page.CandidatesCapped, nil
}

// CheckPrefix checks an answer of phase prefix: status 200, a body that decodes strictly into
// ThreadTitleHitList, every title hit from a visible channel, and at least one hit when the query
// must find something.
func CheckPrefix(phase string, q Query, a Answer, visible func(channelID string) bool) error {
	if a.Status != http.StatusOK {
		return fail(phase, q, "status %d (%s)", a.Status, problem(a.Body))
	}
	var list rest.ThreadTitleHitList
	if err := decodeStrict(a.Body, &list); err != nil {
		return fail(phase, q, "the body is no ThreadTitleHitList: %s", decodeError(err))
	}
	for i, h := range list.Items {
		if !visible(h.ChannelId.String()) {
			return fail(phase, q, "title hit %d, thread %s, comes from channel %s the searcher may not read", i, h.ThreadId, h.ChannelId)
		}
	}
	if q.MustFind && len(list.Items) == 0 {
		return fail(phase, q, "no title hit for a prefix of a visible title")
	}
	return nil
}

// CheckProbe checks an answer of phase probe (design note 11): the probe term and the absent term
// must give the same answer, a 200 SearchHitPage without an item, with candidates_capped false and
// without next_cursor.
func CheckProbe(phase string, q Query, a Answer) error {
	if a.Status != http.StatusOK {
		return fail(phase, q, "status %d (%s)", a.Status, problem(a.Body))
	}
	var page rest.SearchHitPage
	if err := decodeStrict(a.Body, &page); err != nil {
		return fail(phase, q, "the body is no SearchHitPage: %s", decodeError(err))
	}
	switch {
	case len(page.Items) > 0:
		return fail(phase, q, "%d items for a term the searcher cannot read", len(page.Items))
	case page.CandidatesCapped != nil && *page.CandidatesCapped:
		return fail(phase, q, "candidates_capped is true")
	case page.NextCursor != nil:
		return fail(phase, q, "the answer has a next_cursor")
	}
	return nil
}
