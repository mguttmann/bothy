// SPDX-License-Identifier: Apache-2.0

package pushhandoff

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/notify/adapters/apns/apnstest"
)

// Send is one createMessage of the load: the message, its thread (the main thread of the
// sender's channel), the principal IDs it mentions, its scheduled time and the answer.
type Send struct {
	MessageID  string
	Thread     string
	Recipients []string
	Scheduled  time.Time
	Status     int
	// Body is the answer body of a failed send (a problem document, never a token).
	Body string
}

// Registration is a seeded push registration: its ID and the principal it belongs to.
type Registration struct{ ID, Recipient string }

// Run is everything the checks of design note 9 look at after Drain.
type Run struct {
	// Sends are all sends of the run, warm-up included.
	Sends []Send
	// Registrations are the seeded registrations by device token.
	Registrations map[string]Registration
	// Requests are the recorded requests of Fake-APNs.
	Requests []apnstest.Request
	// Occurred is occurred_at of message.created by message ID; Offset how far the database clock
	// runs ahead of the host clock.
	Occurred map[string]time.Time
	Offset   time.Duration
	// From and To bound the measured window of scheduled sends, [From, To).
	From, To time.Time
	// Canary is the per-run string in every content_md; no request may carry it.
	Canary string
	// Unhealthy are the IDs of registrations with disabled_at set or failure_count above zero.
	Unhealthy []string
	// MinSamples is the fewest samples the result may rest on.
	MinSamples int
}

// Result is the outcome of Check: the samples, counts and every problem found. A problem names
// counts, message IDs and registration IDs, never a token.
type Result struct {
	Samples []time.Duration
	// Expected are the (message, recipient) pairs of the window; Matched those with exactly one
	// matching accepted request; Received all requests at the Fake.
	Expected, Matched, Received int
	Problems                    []string
}

// OK reports whether the run behaved correctly, so its value counts (D-M3-24-5).
func (r Result) OK() bool { return len(r.Problems) == 0 }

// problems collects failures by kind, with a count and the first example of each.
type problems struct {
	order []string
	count map[string]int
	first map[string]string
}

func (p *problems) add(kind, format string, args ...any) {
	if p.count == nil {
		p.count, p.first = map[string]int{}, map[string]string{}
	}
	if p.count[kind] == 0 {
		p.order = append(p.order, kind)
		p.first[kind] = fmt.Sprintf(format, args...)
	}
	p.count[kind]++
}

func (p *problems) list() []string {
	out := make([]string, 0, len(p.order))
	for _, k := range p.order {
		out = append(out, fmt.Sprintf("%s: %d, first: %s", k, p.count[k], p.first[k]))
	}
	return out
}

// pair is one expected push: a mention message and a mentioned recipient.
type pair struct{ message, recipient string }

// Check runs the failure conditions of design note 9 over a run and returns the samples of the
// window: one per (mention message scheduled in [From, To), mentioned recipient) with exactly one
// matching accepted request. A problem is: an answer other than 201; an expected pair of the
// window without a request; two requests for one pair or with one nid; a request that does not
// decode, whose b.m is no mention message of the run, whose token belongs to no seeded
// registration, whose recipient was not mentioned, whose b.k is not mention, whose b.t or
// apns-collapse-id is not the thread of the message, or whose aps.badge is missing or below one;
// a badge sync request; the canary in a body or a header; an answer other than 200; an unhealthy
// registration; fewer than MinSamples samples.
func Check(run Run) Result {
	var p problems
	res := Result{Received: len(run.Requests)}
	mentions := map[string]Send{}
	for _, s := range run.Sends {
		if s.Status != http.StatusCreated {
			p.add("answer not 201", "status %d scheduled %s: %s", s.Status, s.Scheduled.Format(time.RFC3339Nano), s.Body)
			continue
		}
		if len(s.Recipients) > 0 {
			mentions[s.MessageID] = s
		}
	}
	seen := map[pair][]Hint{}
	nids := map[string]int{}
	for i, r := range run.Requests {
		reg, known := run.Registrations[r.Token]
		name := "request " + fmt.Sprint(i)
		if known {
			name += " registration " + reg.ID
		}
		if run.Canary != "" && carries(r, run.Canary) {
			p.add("canary in a request", "%s", name)
		}
		h, err := DecodeRequest(r)
		if err != nil {
			p.add("request does not decode", "%s: %v", name, err)
			continue
		}
		if h.AcceptedAt.IsZero() {
			p.add("Fake answer other than 200", "%s message %s", name, h.Message)
		}
		if !known {
			p.add("token of no seeded registration", "%s apns-id %s", name, h.APNsID)
			continue
		}
		if h.Sync {
			p.add("badge sync request", "%s", name)
			continue
		}
		if nids[h.NID]++; nids[h.NID] == 2 {
			p.add("two requests with one nid", "nid %s", h.NID)
		}
		m, ok := mentions[h.Message]
		switch {
		case !ok:
			p.add("b.m is no mention message of the run", "%s message %q", name, h.Message)
			continue
		case !slices.Contains(m.Recipients, reg.Recipient):
			p.add("recipient not mentioned", "%s message %s", name, h.Message)
			continue
		}
		if h.Kind != "mention" {
			p.add("b.k is not mention", "%s message %s kind %q", name, h.Message, h.Kind)
		}
		if h.Thread != m.Thread || h.CollapseID != m.Thread {
			p.add("b.t or apns-collapse-id is not the thread", "%s message %s", name, h.Message)
		}
		if h.Badge == nil || *h.Badge < 1 {
			p.add("aps.badge missing or below one", "%s message %s", name, h.Message)
		}
		k := pair{h.Message, reg.Recipient}
		seen[k] = append(seen[k], h)
	}
	for k, hs := range seen {
		if len(hs) > 1 {
			p.add("two requests for one pair", "message %s recipient %s: %d", k.message, k.recipient, len(hs))
		}
	}
	for _, s := range run.Sends {
		if s.Status != http.StatusCreated || len(s.Recipients) == 0 || !inWindow(s.Scheduled, run.From, run.To) {
			continue
		}
		occurred, ok := run.Occurred[s.MessageID]
		if !ok {
			p.add("no message.created in event_log", "message %s", s.MessageID)
		}
		for _, rcp := range s.Recipients {
			res.Expected++
			hs := seen[pair{s.MessageID, rcp}]
			switch {
			case len(hs) == 0:
				p.add("expected pair without a request", "message %s recipient %s", s.MessageID, rcp)
			case len(hs) == 1 && ok && !hs[0].AcceptedAt.IsZero():
				res.Matched++
				res.Samples = append(res.Samples, Sample(hs[0].AcceptedAt, occurred, run.Offset))
			}
		}
	}
	for _, id := range run.Unhealthy {
		p.add("registration disabled or failing after Drain", "registration %s", id)
	}
	if len(res.Samples) < run.MinSamples {
		p.add("fewer samples than MinSamples", "%d samples, MinSamples %d", len(res.Samples), run.MinSamples)
	}
	res.Problems = p.list()
	return res
}

// carries reports whether the canary appears in the body, a header name or a header value of r.
func carries(r apnstest.Request, canary string) bool {
	if strings.Contains(string(r.Body), canary) {
		return true
	}
	for k, vs := range r.Header {
		if strings.Contains(k, canary) {
			return true
		}
		for _, v := range vs {
			if strings.Contains(v, canary) {
				return true
			}
		}
	}
	return false
}
