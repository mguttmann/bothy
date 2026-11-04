// SPDX-License-Identifier: Apache-2.0

package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/search"
	searchapp "github.com/OWNER/bothy/services/server/internal/search/app"
)

// The fixture of the owner's example (14 4.10): the session where the steering
// session moved from Discord to Larkspur, among sessions that share single words with the questions.
// language-check: off

const (
	takeoverTitle   = "Übernahme Larkspur von Discord"
	takeoverMessage = "Die Steuersitzung zieht von Discord nach Larkspur um, Hermes übernimmt ab jetzt die Steuerung."
	askTakeover     = "Ich suche die Übernahme von Larkspur auf Hermes"
	askMove         = "Umzug der Steuersitzung von Discord nach Larkspur"
)

var assistDistractors = [][2]string{
	{"Larkspur Release 1.4 vorbereiten", "Changelog und Tags für das nächste Larkspur Release."},
	{"Discord Bot Rechte prüfen", "Der Discord Bot braucht Leserechte im Kanal."},
	{"Hermes Gateway Neustart", "Nach dem Update startet das Hermes Gateway neu."},
	{"Larkspur Sidebar Sortierung", "Sortierung der Kanäle in der Seitenleiste von Larkspur."},
	{"Steuerberater Termine", "Die Sitzung mit dem Steuerberater ist am Freitag."},
}

// language-check: on

// assistAnswer is the decoded SearchAssistResult.
type assistAnswer struct {
	Mode           string   `json:"mode"`
	FallbackReason string   `json:"fallback_reason"`
	Terms          []string `json:"terms"`
	Hits           []struct {
		ThreadID    string `json:"thread_id"`
		MessageID   string `json:"message_id"`
		ThreadTitle string `json:"thread_title"`
		ChannelName string `json:"channel_name"`
		Source      string `json:"source"`
	} `json:"hits"`
}

// assistHTTP sends POST /search/assist as name and decodes the 200 answer.
func (w *world) assistHTTP(name, question string) assistAnswer {
	w.t.Helper()
	q, err := json.Marshal(question)
	must(w.t, err)
	r := w.do(name, http.MethodPost, "/api/v1/search/assist", `{"q":`+string(q)+`}`)
	w.want(r, http.StatusOK, "")
	var a assistAnswer
	must(w.t, json.Unmarshal(r.Body, &a))
	return a
}

// rank returns the 1-based position of thread among the hits, 0 when absent.
func (a assistAnswer) rank(thread ids.ID) int {
	for i, h := range a.Hits {
		if h.ThreadID == thread.String() {
			return i + 1
		}
	}
	return 0
}

// takeoverFixture posts the fixture as Robin into Engineering/Projekt-A (Robin's grant G-3) and
// returns the takeover thread and its message.
func (w *world) takeoverFixture() (ids.ID, ids.ID) {
	w.t.Helper()
	sw := &searchWorld{world: w}
	for _, d := range assistDistractors[:2] {
		sw.forumPost("robin", w.projektA, d[0], d[1])
	}
	thread, msg := sw.forumPost("robin", w.projektA, takeoverTitle, takeoverMessage)
	for _, d := range assistDistractors[2:] {
		sw.forumPost("robin", w.projektA, d[0], d[1])
	}
	return thread, msg
}

// TestSearchAssist_OwnersExampleFindsTheTakeoverThread is the ranking test of the ticket: both
// wordings of the owner's example find the takeover thread among the first hits on the real
// full-text search, also without an assistant, and the hit jumps to its message.
func TestSearchAssist_OwnersExampleFindsTheTakeoverThread(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	thread, msg := w.takeoverFixture()
	for _, question := range []string{askTakeover, askMove} {
		a := w.assistHTTP("robin", question)
		if a.Mode != search.ModeFulltext || a.FallbackReason != search.FallbackNotConfigured {
			t.Fatalf("%q: mode %s %s, want full text without an assistant", question, a.Mode, a.FallbackReason)
		}
		r := a.rank(thread)
		t.Logf("%q: terms %q, takeover thread at %d of %d", question, a.Terms, r, len(a.Hits))
		if r < 1 || r > 3 {
			t.Fatalf("%q: the takeover thread is at %d of %+v, want among the first three", question, r, a.Hits)
		}
		h := a.Hits[r-1]
		if h.MessageID != msg.String() || h.ThreadTitle != takeoverTitle || h.ChannelName != "Projekt-A" {
			t.Fatalf("%q: hit %+v, want the message as jump target", question, h)
		}
	}
}

// TestSearchAssist_NobodyFindsSessionsTheyMayNotRead: Alex and Sam ask the same questions and get
// nothing of Projekt-A, which only Robin reads; Alex gets his own thread of Support.
func TestSearchAssist_NobodyFindsSessionsTheyMayNotRead(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.takeoverFixture()
	own := w.say("alex", w.eskalation, "Larkspur takeover notes for Hermes")
	for _, name := range []string{"alex", "sam"} {
		for _, question := range []string{askTakeover, askMove} {
			r := w.do(name, http.MethodPost, "/api/v1/search/assist", `{"q":"`+question+`"}`)
			w.want(r, http.StatusOK, "")
			body := string(r.Body)
			for _, foreign := range []string{"Projekt-A", "Engineering", w.projektA.String(), "Steuersitzung", "Discord Bot"} {
				if strings.Contains(body, foreign) {
					t.Fatalf("%s found %q: %s", name, foreign, body)
				}
			}
		}
	}
	a := w.assistHTTP("alex", askTakeover)
	if len(a.Hits) != 1 || a.Hits[0].MessageID != own.String() {
		t.Fatalf("Alex got %+v, want only his own message", a.Hits)
	}
}

// TestSearchAssist_AgentsMayNotAsk: the assistant is for people; an agent's token gets 403.
func TestSearchAssist_AgentsMayNotAsk(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.setActor("lotse-token", actor.Actor{ID: w.lotse, Kind: actor.KindAgent})
	w.want(w.do("lotse-token", http.MethodPost, "/api/v1/search/assist", `{"q":"larkspur"}`), http.StatusForbidden, "")
}

// capturingRelay records the payloads and answers each request through answer.
type capturingRelay struct {
	mu       sync.Mutex
	payloads []string
	answer   func(req ids.ID)
}

func (c *capturingRelay) AskAssistant(_ context.Context, _ ids.ID, ev eventlog.EphemeralEvent) (bool, error) {
	raw, _ := ev.Payload.(json.RawMessage)
	var p struct {
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return false, err
	}
	c.mu.Lock()
	c.payloads = append(c.payloads, string(raw))
	c.mu.Unlock()
	go c.answer(ids.MustParse(p.RequestID))
	return true, nil
}

// TestSearchAssist_TheAssistantNeverSeesWhatItMayNotRead is the rights test of the model path on
// the real Authorizer: the assistant agent Lotse reads only Support. Robin's question finds the
// takeover thread of Projekt-A, but its payload holds nothing of it, Lotse cannot pick it, and
// Robin still gets it as a full-text hit. Alex's thread of Support reaches the payload and Lotse's
// pick of it comes first with its reason.
func TestSearchAssist_TheAssistantNeverSeesWhatItMayNotRead(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	thread, _ := w.takeoverFixture()
	own := w.say("alex", w.eskalation, "Larkspur takeover notes for Hermes")
	cfg := searchAssistConfig()
	cfg.AgentID = w.lotse.String()
	as, err := searchapp.NewAssistant(cfg, searchapp.AssistDeps{Search: w.search.Coordinator, Authorizer: w.az, Clock: w.clk})
	must(t, err)
	lotse := actor.Actor{ID: w.lotse, Kind: actor.KindAgent}
	answers := make(chan error, 4)
	relay := &capturingRelay{}
	relay.answer = func(req ids.ID) {
		// Every request: a guess at a reference that was not sent, then the first candidate.
		answers <- as.AnswerAssist(context.Background(), lotse, search.AssistAnswer{RequestID: req,
			Picks: []search.AssistPick{{Ref: "c9", Reason: "Guess."}}})
		answers <- as.AnswerAssist(context.Background(), lotse, search.AssistAnswer{RequestID: req, Picks: firstPick(relay.last())})
	}
	must(t, as.BindRelay(relay, nil))

	res, err := as.Assist(context.Background(), w.actor("robin"), search.AssistRequest{Q: askTakeover})
	must(t, err)
	if e := <-answers; e == nil {
		t.Fatal("a pick of a reference that was not sent was accepted")
	}
	must(t, <-answers)
	payload := relay.last()
	for _, leak := range []string{"Steuersitzung", "Discord", "Projekt-A", "Engineering", thread.String()} {
		if strings.Contains(payload, leak) {
			t.Fatalf("the payload to Lotse leaks %q: %s", leak, payload)
		}
	}
	if res.Mode != search.ModeAssisted || !hasThread(res, thread, search.SourceFulltext) {
		t.Fatalf("Robin got %+v, want the takeover thread as full text", res)
	}

	res, err = as.Assist(context.Background(), w.actor("alex"), search.AssistRequest{Q: askTakeover})
	must(t, err)
	<-answers
	must(t, <-answers)
	if !strings.Contains(relay.last(), "Larkspur takeover notes") {
		t.Fatalf("Alex's own readable thread is missing from the payload: %s", relay.last())
	}
	if len(res.Hits) != 1 || res.Hits[0].MessageID != own || res.Hits[0].Source != search.SourceAssistant || res.Hits[0].Reason == "" {
		t.Fatalf("Alex got %+v, want Lotse's pick of his thread", res.Hits)
	}
}

// last returns the newest payload.
func (c *capturingRelay) last() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.payloads) == 0 {
		return ""
	}
	return c.payloads[len(c.payloads)-1]
}

// firstPick picks the first candidate of payload, none when it has none.
func firstPick(payload string) []search.AssistPick {
	var p struct {
		Candidates []struct {
			Ref string `json:"ref"`
		} `json:"candidates"`
	}
	if json.Unmarshal([]byte(payload), &p) != nil || len(p.Candidates) == 0 {
		return nil
	}
	return []search.AssistPick{{Ref: p.Candidates[0].Ref, Reason: "The notes about the takeover."}}
}

// hasThread reports whether res has a hit of thread with source.
func hasThread(res search.AssistResult, thread ids.ID, source string) bool {
	for _, h := range res.Hits {
		if h.ThreadID == thread && h.Source == source {
			return true
		}
	}
	return false
}
