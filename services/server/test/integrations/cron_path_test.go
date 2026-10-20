// SPDX-License-Identifier: Apache-2.0

package integrations_test

import (
	"net/http"
	"strings"
	"testing"
)

// cronJob is the test job of 11 section 10.4: one client_msg_id per run, reused on every retry of
// the run, a native body with a title per run.
type cronJob struct {
	h    *hw
	k    hook
	runs map[int]string
}

// run posts the report of run n (a retry of n reuses its client_msg_id).
func (j *cronJob) run(n int, content string) reply {
	j.h.t.Helper()
	id, ok := j.runs[n]
	if !ok {
		id = newID()
		j.runs[n] = id
	}
	return j.h.postBearer(j.k, native(content, "client_msg_id", id, "title", "Nightly report "+strings.Repeat("I", n)))
}

// TestCronPath_NativeIdempotentPerRun (11 10.4, exit criterion 9): run 1 gives 201; repeating run
// 1 gives 200 with Idempotent-Replay, the same message and thread and still one message.created;
// run 2 gives 201 with a new message; run 1's client_msg_id with other content and through a second
// webhook of Tracker gives 422 idempotency_key_reuse.
func TestCronPath_NativeIdempotentPerRun(t *testing.T) {
	t.Parallel()
	h := newHW(t, options{})
	job := &cronJob{h: h, k: h.create(h.tracker, h.eskalation, ""), runs: map[int]string{}}
	first := job.run(1, "Backups: 42 ok, 0 failed")
	if first.status != http.StatusCreated {
		t.Fatalf("run 1: %d %s", first.status, first.body)
	}
	msg, thread := first.result(t)
	again := job.run(1, "Backups: 42 ok, 0 failed")
	if again.status != http.StatusOK || again.header.Get("Idempotent-Replay") != "true" {
		t.Fatalf("retry of run 1: %d %s %v", again.status, again.body, again.header)
	}
	if m2, t2 := again.result(t); m2 != msg || t2 != thread {
		t.Fatalf("retry answers %s in %s, want %s in %s", m2, t2, msg, thread)
	}
	if n := h.events("message.created", msg, "app"); n != 1 {
		t.Fatalf("%d message.created events", n)
	}
	second := job.run(2, "Backups: 43 ok, 0 failed")
	if second.status != http.StatusCreated {
		t.Fatalf("run 2: %d %s", second.status, second.body)
	}
	if m3, _ := second.result(t); m3 == msg {
		t.Fatal("run 2 got the message of run 1")
	}
	if r := job.run(1, "Backups: other numbers"); r.status != http.StatusUnprocessableEntity || problemCode(r.body) != "idempotency_key_reuse" {
		t.Fatalf("run 1 with other content: %d %s", r.status, r.body)
	}
	other := &cronJob{h: h, k: h.create(h.tracker, h.eskalation, ""), runs: map[int]string{1: job.runs[1]}}
	if r := other.run(1, "Backups: 42 ok, 0 failed"); r.status != http.StatusUnprocessableEntity || problemCode(r.body) != "idempotency_key_reuse" {
		t.Fatalf("run 1 through a second webhook: %d %s", r.status, r.body)
	}
	if n := h.count(`SELECT 1 FROM messages WHERE author_principal_id = $1`, h.tracker); n != 2 {
		t.Fatalf("%d messages of Tracker, want one per run", n)
	}
}
