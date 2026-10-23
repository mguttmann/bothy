// SPDX-License-Identifier: Apache-2.0

package pushworld_test

import (
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/perf/internal/pushhandoff"
	"github.com/OWNER/bothy/services/server/perf/internal/pushworld"
	"github.com/OWNER/bothy/services/server/test/harness"
)

func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }

// TestSeedSmallWorld (brief M3-24 "Tests"): a plan with two channels, two senders and two
// recipients each; Seed writes the grants, the preferences and one registration per recipient;
// one mention per channel gives exactly one matched request per mentioned recipient, the plain
// messages give none, and the matcher passes.
func TestSeedSmallWorld(t *testing.T) {
	t.Parallel()
	srv, fake := pushworld.StartServer(t, harness.Options{})
	if got := srv.ConfigValue("ratelimit.rl.message.per_minute"); got != pushworld.DefaultOf(t, "ratelimit.rl.message.per_minute") {
		t.Fatalf("ratelimit.rl.message.per_minute = %s, want the default", got)
	}
	plan := pushhandoff.Plan{Senders: 4, Channels: 2, Recipients: 4, SendersPerChannel: 2, RecipientsPerChannel: 2, MentionEvery: 2, MentionsPerMessage: 2}
	w, err := pushworld.Seed(t.Context(), srv, plan)
	if err != nil {
		t.Fatal(err)
	}
	db := srv.AdminDB()
	var people, senders []string
	for _, c := range w.Senders {
		people, senders = append(people, c.PrincipalID), append(senders, c.PrincipalID)
	}
	for _, r := range w.Recipients {
		people = append(people, r.Client.PrincipalID)
	}
	var regs, prefs, grants int
	if err := db.QueryRow(t.Context(), `SELECT
		(SELECT count(*) FROM push_registrations r JOIN auth_sessions s ON s.id = r.auth_session_id AND s.device_id = r.device_id
		  WHERE r.provider = 'apns' AND r.environment = 'production' AND r.app_id = 'com.example.chat' AND length(r.token) = 64
		  AND r.disabled_at IS NULL AND r.failure_count = 0),
		(SELECT count(*) FROM notification_preferences WHERE target_kind = 'channel' AND level = 'mentions'),
		(SELECT count(*) FROM grants g WHERE g.target_kind = 'channel' AND g.revoked_at IS NULL AND g.subject_principal_id = ANY($1::uuid[])
		  AND g.level = CASE WHEN g.subject_principal_id = ANY($2::uuid[]) THEN 'write' ELSE 'read' END)`, people, senders).Scan(&regs, &prefs, &grants); err != nil {
		t.Fatal(err)
	}
	if regs != 4 || prefs != 4 || grants != 8 || len(w.Registrations()) != 4 {
		t.Fatalf("registrations %d, preferences %d, grants %d, tokens %d; want 4, 4, 8, 4", regs, prefs, grants, len(w.Registrations()))
	}
	sched, err := plan.Schedule(4)
	if err != nil {
		t.Fatal(err)
	}
	canary := "canary-" + ids.New().String()
	start := time.Now()
	var sends []pushhandoff.Send
	var messageIDs []string
	for _, m := range sched {
		s := w.Send(t.Context(), m, canary, time.Now())
		sends = append(sends, s)
		messageIDs = append(messageIDs, s.MessageID)
	}
	mentions := 0
	for _, s := range sends {
		mentions += len(s.Recipients)
	}
	if mentions != 4 {
		t.Fatalf("%d mentioned recipients, want two per channel", mentions)
	}
	testkit.Until(t, "a request per mentioned recipient", func() bool { return len(fake.Requests()) >= mentions })
	occurred, err := pushworld.Occurred(t.Context(), db, messageIDs)
	if err != nil {
		t.Fatal(err)
	}
	unhealthy, err := pushworld.Unhealthy(t.Context(), db)
	if err != nil {
		t.Fatal(err)
	}
	res := pushhandoff.Check(pushhandoff.Run{Sends: sends, Registrations: w.Registrations(), Requests: fake.Requests(), Occurred: occurred,
		From: start, To: time.Now(), Canary: canary, Unhealthy: unhealthy, MinSamples: mentions})
	if !res.OK() || res.Matched != mentions || res.Received != mentions {
		t.Fatalf("matched %d, received %d, want %d each; problems:\n%s", res.Matched, res.Received, mentions, strings.Join(res.Problems, "\n"))
	}
	if r := sends[1]; r.Status != http.StatusCreated || len(r.Recipients) != 0 {
		t.Fatalf("message 1 is not a plain message: %+v", r)
	}
	if _, err := pushworld.ConsumerLag(t.Context(), db); err != nil {
		t.Fatal(err)
	}
}
