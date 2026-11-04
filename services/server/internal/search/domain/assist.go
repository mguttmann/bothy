// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"cmp"
	"embed"
	"encoding/json"
	"errors"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

// The search assistant (docs/spec/14-search.md 4.10, docs/design/search-assistant.md): the terms
// of a question, the query text, the fusion of the two result lists per thread, the excerpt, the
// payload of search.assist_requested, the rules of an answer and the state of one open request.
// Pure; the app layer runs the searches and holds the requests.

// The field error codes of an answer (06 section 5 point 3; the rulings of the search assistant).
const (
	// FieldUnknownRef: a pick names a reference the request did not send.
	FieldUnknownRef = "unknown_ref"
	// FieldDuplicate: a reference is picked twice.
	FieldDuplicate = "duplicate"
	// FieldTooManyItems: more picks than max_picks, more terms than max_terms or more turns than
	// search.assist.max_turns.
	FieldTooManyItems = "too_many_items"
	// FieldTooShort: an empty reason or term.
	FieldTooShort = "too_short"
	// FieldTooLong: a reason longer than search.assist.reason_max_chars or a term longer than
	// TermMaxChars.
	FieldTooLong = "too_long"
)

// TermMaxChars is the longest term of an answer, in runes.
const TermMaxChars = 64

// rrfK is the constant of the reciprocal rank fusion: 60, the value of the original paper, which
// keeps the first ranks of either list close together.
const rrfK = 60

// assistTimestamp is RFC 3339 in UTC with milliseconds (04 section 2.2).
const assistTimestamp = "2006-01-02T15:04:05.000Z"

// stopwordFiles holds the stop word lists, one file per language: reading it touches only memory.
//
//go:embed stopwords/*.txt
var stopwordFiles embed.FS

// stopwords is the union of the lists, set once at initialisation.
var stopwords = map[string]bool{}

func init() {
	entries, err := stopwordFiles.ReadDir("stopwords")
	if err != nil {
		panic(err)
	}
	for _, e := range entries {
		raw, err := stopwordFiles.ReadFile(path.Join("stopwords", e.Name()))
		if err != nil {
			panic(err)
		}
		for line := range strings.SplitSeq(string(raw), "\n") {
			w := strings.TrimSpace(line)
			if w != "" && !strings.HasPrefix(w, "#") {
				stopwords[w] = true
			}
		}
	}
}

// AssistTerms returns the search terms of a question (14 4.10 point 2): the words of q (runs of
// letters and digits, lower case), without stop words and words shorter than minChars
// (search.prefix_min_chars), then the earlier terms of the popup, without duplicates, at most
// maxTerms (search.assist.max_terms). Never nil.
//
// Example:
//
//	domain.AssistTerms("the takeover of the tool", nil, 3, 12) // ["takeover", "tool"]
func AssistTerms(q string, prior []string, minChars, maxTerms int) []string {
	out := []string{}
	add := func(w string) {
		if len(out) < maxTerms && utf8.RuneCountInString(w) >= minChars && !stopwords[w] && !slices.Contains(out, w) {
			out = append(out, w)
		}
	}
	for w := range strings.FieldsFuncSeq(strings.ToLower(q), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		add(w)
	}
	for _, p := range prior {
		for w := range strings.FieldsFuncSeq(strings.ToLower(p), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
			add(w)
		}
	}
	return out
}

// AssistQueryText returns the query of the terms in web search syntax: the terms joined with
// "or", so a row matches with any of them (14 4.1). The terms hold only letters and digits, so
// the text has no other operator. "" for no terms.
func AssistQueryText(terms []string) string {
	return strings.Join(terms, " or ")
}

// Ranked is one hit of a result list: its thread and its time.
type Ranked struct {
	Thread ids.ID
	At     time.Time
}

// FuseThreads returns the threads of both lists in the order of their reciprocal rank fusion
// (14 4.10 point 3): the rank of a thread in a list is its position among the distinct threads of
// that list, from 1; it scores 1/(60+rank) per list and the title list counts twice, because a
// title hit names the session. Equal scores order the newer thread first, then the higher id.
// Never nil.
//
// Example:
//
//	order := domain.FuseThreads(titleHits, messageHits)
func FuseThreads(titles, messages []Ranked) []ids.ID {
	type entry struct {
		score float64
		at    time.Time
	}
	scores := map[ids.ID]*entry{}
	order := []ids.ID{}
	add := func(list []Ranked, weight float64) {
		rank := 0
		seen := map[ids.ID]bool{}
		for _, h := range list {
			if seen[h.Thread] {
				continue
			}
			seen[h.Thread] = true
			rank++
			e := scores[h.Thread]
			if e == nil {
				e = &entry{}
				scores[h.Thread] = e
				order = append(order, h.Thread)
			}
			e.score += weight / float64(rrfK+rank)
			if h.At.After(e.at) {
				e.at = h.At
			}
		}
	}
	add(titles, 2)
	add(messages, 1)
	slices.SortStableFunc(order, func(a, b ids.ID) int {
		ea, eb := scores[a], scores[b]
		if c := cmp.Compare(eb.score, ea.score); c != 0 {
			return c
		}
		if c := eb.at.Compare(ea.at); c != 0 {
			return c
		}
		return strings.Compare(b.String(), a.String())
	})
	return order
}

// AssistExcerpt returns text as one line: white space collapsed to single spaces, cut to maxChars
// runes with "..." appended when it was longer.
func AssistExcerpt(text string, maxChars int) string {
	s := strings.Join(strings.Fields(text), " ")
	if utf8.RuneCountInString(s) <= maxChars {
		return s
	}
	r := []rune(s)
	return strings.TrimSpace(string(r[:maxChars])) + "..."
}

// Pick is one session the assistant chose: the reference of a candidate and one sentence why.
type Pick struct {
	Ref, Reason string
}

// Cost is the usage of the assistant's model call (schema CostReport).
type Cost struct {
	Provider, Model, Currency                                            string
	TokensIn, TokensOut, TokensCacheRead, TokensCacheWrite, CacheWrite1h int64
	// AmountMicros is the adapter's estimate; nil when it reported none.
	AmountMicros *int64
}

// AssistAnswer is an answer of the assistant agent (schema SearchAssistAnswer).
type AssistAnswer struct {
	Picks []Pick
	Terms []string
	Costs []Cost
}

// ValidateAssistAnswer checks an answer (14 4.10 point 6): at most maxPicks picks (/picks
// too_many_items), each with a reference of refs (/picks/<i>/ref unknown_ref), none twice
// (duplicate), a non-empty reason (too_short) of at most reasonMax runes (too_long) that is a
// display text (the code of the violated SEC-IN-4 rule); at most maxTerms terms (/terms
// too_many_items), each non-empty, at most TermMaxChars runes and a display text. No error repeats
// a value. Errors: validation_failed with one field error.
func ValidateAssistAnswer(a AssistAnswer, refs map[string]bool, maxPicks, maxTerms, reasonMax int) error {
	if len(a.Picks) > maxPicks {
		return assistField("/picks", FieldTooManyItems, "more picks than max_picks")
	}
	seen := map[string]bool{}
	for i, p := range a.Picks {
		base := "/picks/" + strconv.Itoa(i)
		switch {
		case !refs[p.Ref]:
			return assistField(base+"/ref", FieldUnknownRef, "the reference was not sent")
		case seen[p.Ref]:
			return assistField(base+"/ref", FieldDuplicate, "the reference is picked twice")
		}
		seen[p.Ref] = true
		if err := displayField(base+"/reason", p.Reason, reasonMax); err != nil {
			return err
		}
	}
	if len(a.Terms) > maxTerms {
		return assistField("/terms", FieldTooManyItems, "more terms than max_terms")
	}
	for i, t := range a.Terms {
		if err := displayField("/terms/"+strconv.Itoa(i), t, TermMaxChars); err != nil {
			return err
		}
	}
	return nil
}

// displayField checks one non-empty display text of at most maxChars runes.
func displayField(pointer, text string, maxChars int) error {
	switch {
	case text == "":
		return assistField(pointer, FieldTooShort, "the member must not be empty")
	case utf8.RuneCountInString(text) > maxChars:
		return assistField(pointer, FieldTooLong, "the member is too long")
	}
	var te *textcheck.Error
	if err := textcheck.DisplayText(text); errors.As(err, &te) {
		return assistField(pointer, string(te.Rule), "the member is no display text")
	}
	return nil
}

// TooManyTurns is 400 validation_failed on /history with too_many_items: more earlier questions
// than search.assist.max_turns.
func TooManyTurns() error {
	return assistField("/history", FieldTooManyItems, "more turns than search.assist.max_turns")
}

// assistField returns validation_failed with one field error.
func assistField(pointer, code, detail string) *errs.Error {
	return errs.New(errs.CodeValidationFailed, "").WithFields(errs.FieldError{Pointer: pointer, Code: code, Detail: detail})
}

// AssistCandidate is one candidate of the payload (schema SearchAssistCandidate): no ids.
type AssistCandidate struct {
	Ref, Title, ChannelName, SpaceName string
	At                                 time.Time
	Excerpt                            string
	ShownBefore                        bool
}

// AssistPayloadInput are the members of search.assist_requested.
type AssistPayloadInput struct {
	RequestID          ids.ID
	Q                  string
	History            []string
	Candidates         []AssistCandidate
	MaxPicks, MaxTerms int
	ExpiresAt          time.Time
}

type assistCandidateJSON struct {
	Ref         string `json:"ref"`
	Title       string `json:"title,omitempty"`
	ChannelName string `json:"channel_name"`
	SpaceName   string `json:"space_name,omitempty"`
	At          string `json:"at"`
	Excerpt     string `json:"excerpt"`
	ShownBefore bool   `json:"shown_before"`
}

type assistPayloadJSON struct {
	RequestID  string                `json:"request_id"`
	Q          string                `json:"q"`
	History    []string              `json:"history,omitempty"`
	Candidates []assistCandidateJSON `json:"candidates"`
	MaxPicks   int                   `json:"max_picks"`
	MaxTerms   int                   `json:"max_terms"`
	ExpiresAt  string                `json:"expires_at"`
}

// ErrPayloadTooLarge reports that not even a payload without candidates fits.
var ErrPayloadTooLarge = errors.New("search: search.assist_requested does not fit sync.event_payload_max_bytes")

// AssistPayload encodes the payload of search.assist_requested (14 4.10 point 4): no null, no ids
// of threads or messages, times in UTC with milliseconds. Candidates are dropped from the end
// until the payload is shorter than maxBytes (sync.event_payload_max_bytes); kept is the number
// that stayed. Errors: ErrPayloadTooLarge when not even the payload without candidates fits.
func AssistPayload(in AssistPayloadInput, maxBytes int) (payload []byte, kept int, err error) {
	p := assistPayloadJSON{
		RequestID: in.RequestID.String(), Q: in.Q, History: in.History, MaxPicks: in.MaxPicks, MaxTerms: in.MaxTerms,
		ExpiresAt: in.ExpiresAt.UTC().Format(assistTimestamp), Candidates: make([]assistCandidateJSON, 0, len(in.Candidates)),
	}
	for _, c := range in.Candidates {
		p.Candidates = append(p.Candidates, assistCandidateJSON{
			Ref: c.Ref, Title: c.Title, ChannelName: c.ChannelName, SpaceName: c.SpaceName,
			At: c.At.UTC().Format(assistTimestamp), Excerpt: c.Excerpt, ShownBefore: c.ShownBefore,
		})
	}
	for {
		b, err := json.Marshal(p)
		if err != nil {
			return nil, 0, err
		}
		if len(b) < maxBytes {
			return b, len(p.Candidates), nil
		}
		if len(p.Candidates) == 0 {
			return nil, 0, ErrPayloadTooLarge
		}
		p.Candidates = p.Candidates[:len(p.Candidates)-1]
	}
}

// AssistState is the state of one open request.
type AssistState uint8

// The states of a request: open until the first valid answer or the end of the wait.
const (
	AssistOpen AssistState = iota
	AssistAnswered
	AssistExpired
)

// AssistRequest is the state machine open -> answered | expired of one request. The caller guards
// it with a mutex; every transition is a compare-and-swap out of open, so a request has exactly one
// outcome.
type AssistRequest struct {
	expiresAt time.Time
	state     AssistState
	answer    AssistAnswer
}

// NewAssistRequest returns an open request that waits until expiresAt.
func NewAssistRequest(expiresAt time.Time) AssistRequest { return AssistRequest{expiresAt: expiresAt} }

// State returns the state.
func (r *AssistRequest) State() AssistState { return r.state }

// Answered returns the accepted answer; the zero answer unless answered.
func (r *AssistRequest) Answered() AssistAnswer { return r.answer }

// Answer moves open -> answered with a; false in any other state.
func (r *AssistRequest) Answer(a AssistAnswer) bool {
	if r.state != AssistOpen {
		return false
	}
	r.state, r.answer = AssistAnswered, a
	return true
}

// Expire moves open -> expired; false in any other state.
func (r *AssistRequest) Expire() bool {
	if r.state != AssistOpen {
		return false
	}
	r.state = AssistExpired
	return true
}

// Forgotten reports whether the request may be forgotten at now: from expires_at plus timeout
// (search.assist.timeout) on. Until then an answer learns 410 gone or 204 instead of 404.
func (r *AssistRequest) Forgotten(now time.Time, timeout time.Duration) bool {
	return !now.Before(r.expiresAt.Add(timeout))
}
