// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Type is the result type of a search (messages, threads, files).
type Type string

// The three result types of docs/spec/14-search.md section 11.
const (
	TypeMessages Type = "messages"
	TypeThreads  Type = "threads"
	TypeFiles    Type = "files"
)

// Filter is the query parameter name of a filter of 14 4.4 (plus thread_id of openapi.yaml).
type Filter string

// The filters of 14 4.4 and thread_id.
const (
	FilterSpaceID     Filter = "space_id"
	FilterChannelID   Filter = "channel_id"
	FilterThreadID    Filter = "thread_id"
	FilterAuthorID    Filter = "author_id"
	FilterAgentID     Filter = "agent_id"
	FilterAfter       Filter = "after"
	FilterBefore      Filter = "before"
	FilterHasFile     Filter = "has_file"
	FilterThreadState Filter = "thread_state"
	FilterTagID       Filter = "tag_id"
)

// The field error codes of q and of filters (06 section 5, docs/DECISIONS.md D-M3-14-8).
const (
	codeTooLong       = "too_long"
	codeEmpty         = "empty"
	codeNotApplicable = "not_applicable"
	pointerQ          = "#query/q"
)

// filterTypes is the column "Types" of 14 4.4; thread_id applies to every type.
var filterTypes = map[Filter][]Type{
	FilterSpaceID:     {TypeMessages, TypeThreads, TypeFiles},
	FilterChannelID:   {TypeMessages, TypeThreads, TypeFiles},
	FilterThreadID:    {TypeMessages, TypeThreads, TypeFiles},
	FilterAuthorID:    {TypeMessages, TypeFiles},
	FilterAgentID:     {TypeMessages, TypeThreads, TypeFiles},
	FilterAfter:       {TypeMessages, TypeThreads, TypeFiles},
	FilterBefore:      {TypeMessages, TypeThreads, TypeFiles},
	FilterHasFile:     {TypeMessages},
	FilterThreadState: {TypeMessages, TypeThreads},
	FilterTagID:       {TypeMessages, TypeThreads},
}

// Filters returns every filter in the order of 14 4.4.
func Filters() []Filter {
	return []Filter{FilterSpaceID, FilterChannelID, FilterThreadID, FilterAuthorID, FilterAgentID, FilterAfter,
		FilterBefore, FilterHasFile, FilterThreadState, FilterTagID}
}

// CheckFilter returns nil when the filter f applies to the type t, otherwise 400
// validation_failed with pointer #query/<f> and code not_applicable (D-M3-14-8): a filter that
// cannot narrow the type would be ignored silently, so the client learns of its mistake.
func CheckFilter(f Filter, t Type) error {
	if slices.Contains(filterTypes[f], t) {
		return nil
	}
	return errs.New(errs.CodeValidationFailed, "").WithFields(errs.FieldError{
		Pointer: "#query/" + string(f), Code: codeNotApplicable,
		Detail: fmt.Sprintf("the filter %s does not apply to the type %s", f, t),
	})
}

// ValidateQ checks the length of the raw query raw and returns it cleaned (14 4.1, D-M3-14-9).
// The length counts runes of the raw value, an invalid byte as one; more than maxChars
// (search.query_max_chars) is 400 validation_failed on #query/q with code too_long. Invalid
// UTF-8 and control characters become a space: they cannot be part of a term, and PostgreSQL
// rejects a NUL byte in text.
//
// Example:
//
//	q, err := domain.ValidateQ("invoice\x00run", 256) // "invoice run", nil
func ValidateQ(raw string, maxChars int) (string, error) {
	if utf8.RuneCountInString(raw) > maxChars {
		return "", errs.New(errs.CodeValidationFailed, "").WithFields(errs.FieldError{
			Pointer: pointerQ, Code: codeTooLong, Detail: "the query is longer than search.query_max_chars",
		})
	}
	var b strings.Builder
	b.Grow(len(raw))
	for i := 0; i < len(raw); {
		r, size := utf8.DecodeRuneInString(raw[i:])
		if (r == utf8.RuneError && size == 1) || unicode.IsControl(r) {
			r = ' '
		}
		b.WriteRune(r)
		i += size
	}
	return b.String(), nil
}

// EmptyQuery is 400 validation_failed on #query/q with code empty: no term remains of q (14 4.1).
func EmptyQuery() error {
	return errs.New(errs.CodeValidationFailed, "").WithFields(errs.FieldError{
		Pointer: pointerQ, Code: codeEmpty, Detail: "the query has no search term",
	})
}

// TooLong is 400 validation_failed on #query/q with code too_long for a raw query of more than
// maxChars runes, nil otherwise; the prefix search checks only the length.
func TooLong(raw string, maxChars int) error {
	_, err := ValidateQ(raw, maxChars)
	return err
}

// PageSize returns the page size of a request (06 section 6 item 3): def
// (search.page_size_default) when limit is not positive, otherwise limit capped at maxSize
// (search.page_size_max).
func PageSize(limit, def, maxSize int) int {
	if limit < 1 {
		return min(def, maxSize)
	}
	return min(limit, maxSize)
}

// Binding is the canonical form of a search request that a cursor is bound to (14 4.7,
// docs/DECISIONS.md D-M3-14-5): type, raw q, sort and every filter with defaults applied. limit is
// not part of it, so a client may change the page size between pages.
type Binding struct {
	Type        Type
	Q           string
	Sort        string
	SpaceIDs    []ids.ID
	ChannelIDs  []ids.ID
	ThreadID    *ids.ID
	AuthorID    *ids.ID
	AgentID     *ids.ID
	After       *time.Time
	Before      *time.Time
	HasFile     *bool
	ThreadState string
	TagIDs      []ids.ID
}

// bindingJSON is the encoded form: id lists sorted and without duplicates, times in UTC with
// millisecond precision.
type bindingJSON struct {
	Type        Type     `json:"type"`
	Q           string   `json:"q"`
	Sort        string   `json:"sort"`
	SpaceIDs    []string `json:"space_ids"`
	ChannelIDs  []string `json:"channel_ids"`
	ThreadID    string   `json:"thread_id"`
	AuthorID    string   `json:"author_id"`
	AgentID     string   `json:"agent_id"`
	After       string   `json:"after"`
	Before      string   `json:"before"`
	HasFile     string   `json:"has_file"`
	ThreadState string   `json:"thread_state"`
	TagIDs      []string `json:"tag_ids"`
}

// Encode returns the canonical bytes of b. Equal requests give equal bytes whatever the order or
// repetition of ids and the zone or sub-millisecond part of times; any other difference gives
// other bytes.
func (b Binding) Encode() []byte {
	out := bindingJSON{
		Type: b.Type, Q: b.Q, Sort: b.Sort, SpaceIDs: idSet(b.SpaceIDs), ChannelIDs: idSet(b.ChannelIDs),
		ThreadID: optID(b.ThreadID), AuthorID: optID(b.AuthorID), AgentID: optID(b.AgentID),
		After: optTime(b.After), Before: optTime(b.Before), ThreadState: b.ThreadState, TagIDs: idSet(b.TagIDs),
	}
	if b.HasFile != nil {
		out.HasFile = fmt.Sprint(*b.HasFile)
	}
	enc, err := json.Marshal(out)
	if err != nil {
		// Strings and string slices always marshal.
		panic(err)
	}
	return enc
}

// idSet returns the ids as sorted strings without duplicates; never nil.
func idSet(in []ids.ID) []string {
	out := make([]string, 0, len(in))
	for _, id := range in {
		out = append(out, id.String())
	}
	slices.Sort(out)
	return slices.Compact(out)
}

func optID(id *ids.ID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

func optTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Truncate(time.Millisecond).Format("2006-01-02T15:04:05.000Z")
}
