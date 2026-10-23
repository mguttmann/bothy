// SPDX-License-Identifier: Apache-2.0

package pushhandoff_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/notify/adapters/apns/apnstest"
	"github.com/OWNER/bothy/services/server/perf/internal/pushhandoff"
	"github.com/OWNER/bothy/services/server/perf/internal/report"
)

// The fixture of the checks: Alex sends in the main thread of one channel and mentions Sam and
// Robin; each of them holds one registration.
const (
	thread   = "01900000-0000-7000-8000-0000000000a1"
	instance = "01900000-0000-7000-8000-0000000000f1"
	samID    = "01900000-0000-7000-8000-0000000000b1"
	robinID  = "01900000-0000-7000-8000-0000000000b2"
	samReg   = "01900000-0000-7000-8000-0000000000c1"
	robinReg = "01900000-0000-7000-8000-0000000000c2"
	canary   = "canary-7f3a"
)

// Device tokens of the fixture; no failure message may contain them.
var (
	samToken   = strings.Repeat("a1", 32)
	robinToken = strings.Repeat("b2", 32)
	strayToken = strings.Repeat("c3", 32)
)

var t0 = time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)

// body returns a hint body like the example of 13 6.1.
func body(nid, kind, threadID, msg string, badge string) []byte {
	b := `{"aps":{"alert":{"title-loc-key":"PUSH_TITLE","loc-key":"PUSH_BODY_MENTION"},"mutable-content":1,"thread-id":"` + threadID +
		`","category":"MENTION","sound":"default"` + badge + `,"interruption-level":"active"},"b":{"v":1,"nid":"` + nid + `","k":"` + kind +
		`","i":"` + instance + `","t":"` + threadID + `","m":"` + msg + `"}}`
	return []byte(b)
}

// request returns the Fake's record of a hint for token about msg, accepted at.
func request(token, nid, msg string, at time.Time) apnstest.Request {
	return apnstest.Request{Proto: "HTTP/2.0", Method: http.MethodPost, Token: token, AcceptedAt: at,
		Header: http.Header{"Apns-Id": {nid}, "Apns-Collapse-Id": {thread}, "Apns-Topic": {"com.example.chat"}},
		Body:   body(nid, "mention", thread, msg, `,"badge":1`)}
}

// run is a clean run: the window holds n mention messages of Alex, each mentioning Sam and
// Robin, committed at t0 + i seconds and accepted 100 ms later; one message lies in the warm-up.
func run(n int) pushhandoff.Run {
	r := pushhandoff.Run{
		Registrations: map[string]pushhandoff.Registration{samToken: {ID: samReg, Recipient: samID}, robinToken: {ID: robinReg, Recipient: robinID}},
		Occurred:      map[string]time.Time{}, From: t0, To: t0.Add(time.Hour), Canary: canary, MinSamples: 2 * n,
	}
	for i := -1; i < n; i++ {
		msg := fmt.Sprintf("01900000-0000-7000-8000-%012d", 100+i)
		at := t0.Add(time.Duration(i) * time.Second)
		r.Sends = append(r.Sends, pushhandoff.Send{MessageID: msg, Thread: thread, Recipients: []string{samID, robinID}, Scheduled: at, Status: http.StatusCreated})
		r.Occurred[msg] = at
		r.Requests = append(r.Requests,
			request(samToken, fmt.Sprintf("01900000-0000-7000-8001-%012d", 100+i), msg, at.Add(100*time.Millisecond)),
			request(robinToken, fmt.Sprintf("01900000-0000-7000-8002-%012d", 100+i), msg, at.Add(100*time.Millisecond)))
	}
	r.Sends = append(r.Sends, pushhandoff.Send{MessageID: "01900000-0000-7000-8000-000000000999", Thread: thread, Scheduled: t0, Status: http.StatusCreated})
	return r
}

func TestDecodeRequest(t *testing.T) {
	t.Parallel()
	h, err := pushhandoff.DecodeRequest(request(samToken, "nid-1", "msg-1", t0))
	if err != nil {
		t.Fatal(err)
	}
	if h.Sync || h.Kind != "mention" || h.Message != "msg-1" || h.Thread != thread || h.NID != "nid-1" || h.CollapseID != thread ||
		h.Badge == nil || *h.Badge != 1 || !h.AcceptedAt.Equal(t0) || h.Token != samToken {
		t.Errorf("hint %+v", h)
	}
	for name, b := range map[string]string{
		"unknown in aps": `{"aps":{"badge":1,"alert":{"title-loc-key":"PUSH_TITLE","loc-key":"PUSH_BODY_MENTION"},"body":"hello"},"b":{"v":1,"nid":"n","k":"mention","i":"i","t":"t","m":"m"}}`,
		"unknown in b":   `{"aps":{"badge":1},"b":{"v":1,"nid":"n","k":"mention","i":"i","t":"t","m":"m","text":"hello"}}`,
		"unknown top":    `{"aps":{"badge":1},"text":"hello"}`,
		"not json":       `hello`,
	} {
		if _, err := pushhandoff.DecodeRequest(apnstest.Request{Token: samToken, Body: []byte(b)}); err == nil {
			t.Errorf("%s: decoded", name)
		}
	}
	h, err = pushhandoff.DecodeRequest(apnstest.Request{Token: samToken, Body: []byte(`{"aps":{"badge":3}}`)})
	if err != nil || !h.Sync || h.Badge == nil || *h.Badge != 3 {
		t.Errorf("sync form: %+v, %v", h, err)
	}
}

func TestHintSample(t *testing.T) {
	t.Parallel()
	offset := 40 * time.Millisecond // the database clock runs 40 ms ahead
	if got := pushhandoff.Sample(t0.Add(500*time.Millisecond), t0.Add(offset), offset); got != 500*time.Millisecond {
		t.Errorf("sample %s, want 500ms", got)
	}
	r := run(3)
	r.Offset = 0
	r.From = t0.Add(time.Second) // message 0 now lies in the warm-up
	r.To = t0.Add(2 * time.Second)
	r.MinSamples = 1
	r.Requests[len(r.Requests)-1].AcceptedAt = t0.Add(time.Minute) // the last message is after the window
	res := pushhandoff.Check(r)
	if !res.OK() || len(res.Samples) != 2 || res.Expected != 2 {
		t.Fatalf("window of one message: %d samples, %d expected, problems %v", len(res.Samples), res.Expected, res.Problems)
	}
	xs := []time.Duration{5, 1, 4, 2, 3, 9, 8, 7, 6, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	got, err := pushhandoff.P95(xs)
	want, _ := report.DurationsP(xs, 95)
	if err != nil || float64(got) != want || got != 19 {
		t.Errorf("P95 = %v, %v; report says %v", got, err, want)
	}
}

func TestMatcher(t *testing.T) {
	t.Parallel()
	if res := pushhandoff.Check(run(3)); !res.OK() || len(res.Samples) != 6 || res.Matched != 6 || res.Received != 8 {
		t.Fatalf("clean run: ok %v, %d samples, %d matched, %d received, problems %v", res.OK(), len(res.Samples), res.Matched, res.Received, res.Problems)
	}
	last := func(r *pushhandoff.Run) *apnstest.Request { return &r.Requests[len(r.Requests)-1] }
	cases := map[string]func(r *pushhandoff.Run){ // name: the mutation; want[name]: the problem it must cause
		"answer not 201": func(r *pushhandoff.Run) { r.Sends[1].Status = http.StatusTooManyRequests },
		"missing pair":   func(r *pushhandoff.Run) { r.Requests = r.Requests[:len(r.Requests)-1] },
		"duplicate pair": func(r *pushhandoff.Run) {
			d := *last(r)
			d.Header = http.Header{"Apns-Id": {"other"}, "Apns-Collapse-Id": {thread}}
			d.Body = body("other", "mention", thread, r.Sends[3].MessageID, `,"badge":1`)
			r.Requests = append(r.Requests, d)
		},
		"duplicate nid": func(r *pushhandoff.Run) {
			r.Requests[2].Body = body("01900000-0000-7000-8002-000000000100", "mention", thread, r.Sends[1].MessageID, `,"badge":1`)
		},
		"unknown message": func(r *pushhandoff.Run) {
			last(r).Body = body("x", "mention", thread, "01900000-0000-7000-8000-000000000777", `,"badge":1`)
		},
		"unknown token":           func(r *pushhandoff.Run) { last(r).Token = strayToken },
		"recipient not mentioned": func(r *pushhandoff.Run) { r.Sends[3].Recipients = []string{samID} },
		"kind message": func(r *pushhandoff.Run) {
			last(r).Body = body("x", "message", thread, r.Sends[3].MessageID, `,"badge":1`)
		},
		"wrong thread": func(r *pushhandoff.Run) {
			last(r).Body = body("x", "mention", "01900000-0000-7000-8000-0000000000a2", r.Sends[3].MessageID, `,"badge":1`)
		},
		"wrong collapse id": func(r *pushhandoff.Run) {
			last(r).Header = http.Header{"Apns-Collapse-Id": {"01900000-0000-7000-8000-0000000000a2"}}
		},
		"missing badge": func(r *pushhandoff.Run) { last(r).Body = body("x", "mention", thread, r.Sends[3].MessageID, ``) },
		"badge zero": func(r *pushhandoff.Run) {
			last(r).Body = body("x", "mention", thread, r.Sends[3].MessageID, `,"badge":0`)
		},
		"sync form": func(r *pushhandoff.Run) {
			r.Requests = append(r.Requests, apnstest.Request{Token: samToken, Body: []byte(`{"aps":{"badge":2}}`), AcceptedAt: t0})
		},
		"canary in body": func(r *pushhandoff.Run) {
			last(r).Header.Set("Apns-Push-Type", "alert")
			last(r).Body = append(last(r).Body[:len(last(r).Body)-2], []byte(`,"m":"`+canary+`"}}`)...)
		},
		"canary in header":       func(r *pushhandoff.Run) { last(r).Header.Set("Apns-Topic", "com.example."+canary) },
		"fake answer 403":        func(r *pushhandoff.Run) { last(r).AcceptedAt = time.Time{} },
		"undecodable":            func(r *pushhandoff.Run) { last(r).Body = []byte(`{"aps":{"badge":1,"body":"x"}}`) },
		"unhealthy registration": func(r *pushhandoff.Run) { r.Unhealthy = []string{samReg} },
	}
	want := map[string]string{
		"answer not 201":          "answer not 201",
		"missing pair":            "expected pair without a request",
		"duplicate pair":          "two requests for one pair",
		"duplicate nid":           "two requests with one nid",
		"unknown message":         "no mention message",
		"unknown token":           "token of no seeded registration",
		"recipient not mentioned": "recipient not mentioned",
		"kind message":            "b.k is not mention",
		"wrong thread":            "is not the thread",
		"wrong collapse id":       "is not the thread",
		"missing badge":           "aps.badge missing",
		"badge zero":              "aps.badge missing",
		"sync form":               "badge sync request",
		"canary in body":          "canary",
		"canary in header":        "canary",
		"fake answer 403":         "other than 200",
		"undecodable":             "does not decode",
		"unhealthy registration":  "disabled or failing",
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := run(3)
			mutate(&r)
			res := pushhandoff.Check(r)
			if res.OK() || !strings.Contains(strings.Join(res.Problems, "\n"), want[name]) {
				t.Fatalf("problems %v, want one with %q", res.Problems, want[name])
			}
			for _, p := range res.Problems {
				for _, tok := range []string{samToken, robinToken, strayToken} {
					if strings.Contains(p, tok) {
						t.Fatalf("problem %q contains a device token", p)
					}
				}
			}
		})
	}
}

func TestMinSamples(t *testing.T) {
	t.Parallel()
	r := run(3)
	r.MinSamples = 7
	res := pushhandoff.Check(r)
	if res.OK() || !strings.Contains(strings.Join(res.Problems, "\n"), "MinSamples") {
		t.Fatalf("6 samples with MinSamples 7: ok %v, problems %v", res.OK(), res.Problems)
	}
}
