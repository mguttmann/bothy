// SPDX-License-Identifier: Apache-2.0

package notify_test

import (
	"net/http"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/notify/adapters/fcm/fcmtest"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

// TestPushSend_OutageKeepsRegistration (finding srv-notify-1, docs/decisions.d/AUD-srv-notify-01.md
// AUD-SN01/1): an outage of the provider and of the way to it never ends a registration. With the
// default of push.max_consecutive_failures, one notify.push_send for Sam's phone runs through the
// real FCM adapter against Fake-FCM while the token endpoint answers 503 three times (no access
// token, a Go error of the adapter) and the send endpoint then answers 503, 500 and 503: after
// each of the six attempts the registration is still active with the failures counted, and the
// hint has exactly one follow-up job with the next attempt. When Fake-FCM answers again, that
// follow-up delivers the hint with its nid, the count is back at zero, and the next hint of Sam
// still gets a job for the same registration.
func TestPushSend_OutageKeepsRegistration(t *testing.T) {
	t.Parallel()
	w := newWorld(t, withFCM(), withDefaultFailureLimit())
	reg, token := w.samPhone.Registration, w.samPhone.Token
	j := w.mentionJob("sam", w.sam)
	if j.Args.RegistrationID != reg {
		t.Fatalf("job %+v, want Sam's fcm registration", j.Args)
	}
	unavailable := fcmtest.Response{Status: http.StatusServiceUnavailable, RPCStatus: "UNAVAILABLE", ErrorCode: "UNAVAILABLE"}
	broken := fcmtest.Response{Status: http.StatusInternalServerError, RPCStatus: "INTERNAL", ErrorCode: "INTERNAL"}
	down := fcmtest.TokenResponse{Status: http.StatusServiceUnavailable}
	w.fcmReal.fake.ScriptToken(down, down, down)
	w.fcmReal.fake.Script(token, unavailable, broken, unavailable)
	const outage = 6
	before := w.sent("fcm", domain.OutcomeProviderError)
	cur := j
	for attempt := int64(1); attempt <= outage; attempt++ {
		if err := w.send(cur); err != nil {
			t.Fatalf("attempt %d: the job failed: %v", attempt, err)
		}
		if s := w.registration(reg); s.Disabled || s.Failures != attempt || s.LastSuccessAt != nil {
			t.Fatalf("attempt %d: registration %+v, want active with %d failures", attempt, s, attempt)
		}
		follow := w.followUps(j.Args.NotificationID, cur.ID)
		if len(follow) != 1 || follow[0].Args.Attempt != attempt+1 || follow[0].Args.RegistrationID != reg ||
			follow[0].Args.DueAtUnixMicro != j.Args.DueAtUnixMicro {
			t.Fatalf("attempt %d: follow-up jobs %+v, want one with Attempt %d", attempt, follow, attempt+1)
		}
		cur = follow[0]
		w.clk.Set(cur.ScheduledAt)
	}
	if n := w.sent("fcm", domain.OutcomeProviderError) - before; n != outage {
		t.Fatalf("bothy.push.sent provider_error rose by %d, want %d", n, outage)
	}
	if got := w.requestsFor(token); len(got) != 3 {
		t.Fatalf("%d send requests reached Fake-FCM during the outage, want 3", len(got))
	}

	// The outage is over: the pending retry delivers the hint.
	if err := w.send(cur); err != nil {
		t.Fatalf("the retry after the outage failed: %v", err)
	}
	got := w.requestsFor(token)
	if len(got) != 4 || decodeFCM(t, got[3]).Message.Data["nid"] != j.Args.NotificationID.String() {
		t.Fatalf("%d send requests, want a fourth with the nid of the hint", len(got))
	}
	if s := w.registration(reg); s.Disabled || s.Failures != 0 || s.LastSuccessAt == nil {
		t.Fatalf("registration %+v after the outage, want active, no failures and a success", s)
	}
	if follow := w.followUps(j.Args.NotificationID, cur.ID); len(follow) != 0 {
		t.Fatalf("follow-up jobs %+v after the delivery, want none", follow)
	}

	// The registration still takes part in the fan-out.
	next := w.mentionJob("sam", w.sam)
	if next.Args.RegistrationID != reg {
		t.Fatalf("the next hint has the job %+v, want one for Sam's registration", next.Args)
	}
	if err := w.send(next); err != nil {
		t.Fatalf("the next hint failed: %v", err)
	}
	if got := w.requestsFor(token); len(got) != 5 {
		t.Fatalf("%d send requests, want the next hint as the fifth", len(got))
	}
}
