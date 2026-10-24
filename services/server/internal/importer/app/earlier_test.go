// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/importer"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// earlierSessions fakes agents.ImportedSessions: it records the calls and answers taken_over for
// the first call of a thread and reference, already_present after it, or the outcome or error set
// in next.
type earlierSessions struct {
	mu    sync.Mutex
	calls []agents.EarlierSession
	seen  map[string]bool
	next  string
	err   error
}

func (s *earlierSessions) CreateEarlier(_ context.Context, e agents.EarlierSession) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, e)
	if s.err != nil {
		return "", s.err
	}
	if s.next != "" {
		return s.next, nil
	}
	if s.seen == nil {
		s.seen = map[string]bool{}
	}
	key := e.ThreadID.String() + "/" + e.ExternalSessionRef
	if s.seen[key] {
		return agents.EarlierAlreadyPresent, nil
	}
	s.seen[key] = true
	return agents.EarlierTakenOver, nil
}

// TestThreadItemEarlierSession holds M8-06 design note 3: a new thread item with earlier_session
// writes the earlier row after the thread and answers session taken_over; the same item again (a
// later run with the file) reaches the posted thread and answers the outcome of agents; the
// outcomes are counted under stats.server; a validation error of agents names
// /items/0/data/earlier_session/...; an item without the member calls nothing.
func TestThreadItemEarlierSession(t *testing.T) {
	t.Parallel()
	c := newContentEnv(t)
	ctx := context.Background()
	data := strings.TrimSuffix(textThread, "}") + `,"earlier_session":{"agent_id":"` + c.lotse.String() +
		`","external_session_ref":"sess-alex-1","cost_baseline_micros":1500,"currency":"USD"}}`
	apply := func(key, body string) (importer.BatchResult, error) {
		return c.svc.ApplyBatch(ctx, c.owner, c.src.ID, thread(key, o(key, "thread", body)))
	}

	res, err := apply("thread:41", data)
	if err != nil || res.Items[0].Session != agents.EarlierTakenOver {
		t.Fatalf("new thread %+v %v", res, err)
	}
	if len(c.sessions.calls) != 1 {
		t.Fatalf("calls %+v", c.sessions.calls)
	}
	call := c.sessions.calls[0]
	if call.ThreadID != res.Items[0].TargetID || call.AgentID != c.lotse || call.ExternalSessionRef != "sess-alex-1" ||
		call.CostBaselineMicros != 1500 || call.Currency != "USD" {
		t.Errorf("call %+v, thread %s", call, res.Items[0].TargetID)
	}
	if res, err = apply("thread:41", data); err != nil || res.Items[0].Session != agents.EarlierAlreadyPresent {
		t.Fatalf("posted thread %+v %v", res, err)
	}
	c.sessions.next = agents.EarlierDroppedLate
	if res, err = apply("thread:41", data); err != nil || res.Items[0].Session != agents.EarlierDroppedLate {
		t.Fatalf("late line %+v %v", res, err)
	}
	src, err := c.svc.GetSource(ctx, c.owner, c.src.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"sessions_taken_over":1`, `"sessions_already_present":1`, `"sessions_dropped_late":1`} {
		if !strings.Contains(string(src.Stats), want) {
			t.Errorf("stats %s miss %s", src.Stats, want)
		}
	}

	c.sessions.next = ""
	c.sessions.err = errs.New(errs.CodeValidationFailed, "x").WithFields(errs.FieldError{Pointer: "/agent_id", Code: "invalid", Detail: "x"})
	_, err = apply("thread:42", strings.Replace(data, "thread:41", "thread:42", 1))
	if e, ok := errs.As(err); !ok || len(e.Fields) != 1 || e.Fields[0].Pointer != "/items/0/data/earlier_session/agent_id" {
		t.Fatalf("validation error %v", err)
	}
	c.sessions.err = nil
	calls := len(c.sessions.calls)
	if res, err = apply("thread:43", textThread); err != nil || res.Items[0].Session != "" || len(c.sessions.calls) != calls {
		t.Fatalf("thread without the member %+v %v, calls %d", res, err, len(c.sessions.calls)-calls)
	}
}
