// SPDX-License-Identifier: Apache-2.0

package multinode

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// consumer is the REST consumer of the operator agent in these tests.
const consumer = "multinode"

// runOf waits until the run of thread exists and returns its ID.
func (c *cluster) runOf(thread string) string {
	c.t.Helper()
	var id string
	testkit.Until(c.t, "the run of thread "+thread, func() bool {
		err := c.nodes[nodeA].AdminDB().QueryRow(context.Background(), `SELECT id::text FROM runs WHERE thread_id = $1`, thread).Scan(&id)
		return err == nil && id != ""
	})
	return id
}

// invocationToken polls the REST consumer of agent through node a until the agent.invocation of
// run arrives and returns its token.
func (c *cluster) invocationToken(agent *harness.Client, run string) string {
	c.t.Helper()
	var tok string
	testkit.Until(c.t, "the agent.invocation of run "+run, func() bool {
		var page struct {
			Events []struct {
				Type            string `json:"type"`
				InvocationToken string `json:"invocation_token"`
				Resource        struct {
					ID string `json:"id"`
				} `json:"resource"`
			} `json:"events"`
			NextCursor int64 `json:"next_cursor"`
		}
		c.must(nodeA, agent, http.MethodGet, "/agents/me/consumers/"+consumer+"/events?limit=100", nil, http.StatusOK).JSON(c.t, &page)
		for _, ev := range page.Events {
			if ev.Type == "agent.invocation" && ev.Resource.ID == run && ev.InvocationToken != "" {
				tok = ev.InvocationToken
			}
		}
		if len(page.Events) > 0 {
			c.must(nodeA, agent, http.MethodPost, "/agents/me/consumers/"+consumer+"/ack", map[string]any{"seq": page.NextCursor}, http.StatusOK)
		}
		return tok != ""
	})
	return tok
}

// startBody is the RunStart body with the token, billing mode organization and tool class chat.
func startBody(token string) map[string]any {
	return map[string]any{"invocation_token": token, "billing_mode_used": "organization", "tool_class_used": "chat"}
}

// TestTypingAndLiveBoxCrossNode (M7-17 acceptance 1, 02 11.6): Alex types on node a and Sam, who
// focuses the thread on node b, receives typing.started; the operator Lotse patches its run
// through node a and Sam receives run.updated with the run document on node b. The latencies are
// recorded (the bound perf.fanout_cross_node_p99 is checked by perf/gateway_fanout_multi).
func TestTypingAndLiveBoxCrossNode(t *testing.T) {
	t.Parallel()
	c := newCluster(t)
	alex, sam, lotse := c.w.People["Alex"], c.w.People["Sam"], c.w.Agents["Lotse"]
	// The first poll creates the REST consumer at the head of the log, before the trigger.
	c.must(nodeA, lotse, http.MethodGet, "/agents/me/consumers/"+consumer+"/events?limit=1", nil, http.StatusOK)
	th := c.newThread(alex, "Support/Allgemein", "Export check")
	run := c.runOf(th)
	c.must(nodeA, lotse, http.MethodPost, "/runs/"+run+"/start", startBody(c.invocationToken(lotse, run)), http.StatusOK)

	samB := c.connect(nodeB, sam)
	c.focus(samB, sam.PrincipalID, false, th)
	alexA := c.connect(nodeA, alex)
	sent := time.Now()
	if err := alexA.Send("typing.start", map[string]any{"thread_id": th}); err != nil {
		t.Fatal(err)
	}
	typing, f := c.waitDispatch(samB, "typing.started", func(d dispatch) bool { return d.Actor != nil && d.Actor.ID == alex.PrincipalID })
	if typing.Scope["thread_id"] != th || typing.Scope["channel_id"] != c.w.Channels["Support/Allgemein"] {
		t.Fatalf("typing.started on node b: %+v", typing)
	}
	t.Logf("typing.started from node a to node b in %s", f.Received.Sub(sent))

	var cur struct {
		Version int64 `json:"version"`
	}
	c.must(nodeA, lotse, http.MethodGet, "/runs/"+run, nil, http.StatusOK).JSON(t, &cur)
	sent = time.Now()
	c.must(nodeA, lotse, http.MethodPatch, "/runs/"+run, map[string]any{"base_version": cur.Version,
		"ops": []any{map[string]any{"op": "replace", "path": "/status_line", "value": "Checking the export"}}}, http.StatusOK)
	upd, f := c.waitDispatch(samB, "run.updated", func(d dispatch) bool {
		return d.Resource.ID == run && strings.Contains(string(d.Payload), "Checking the export")
	})
	var body struct {
		Snapshot json.RawMessage `json:"snapshot"`
	}
	if err := json.Unmarshal(upd.Payload, &body); err != nil || len(body.Snapshot) == 0 {
		t.Fatalf("run.updated on node b without the run document: %s %v", upd.Payload, err)
	}
	t.Logf("run.updated from node a to node b in %s", f.Received.Sub(sent))
}

// TestConsumerFencing4015 (04 15.1, M7-17 acceptance 2): a second identify of the default consumer
// of Lotse on node b closes the older connection on node a with 4015; the next invocation reaches
// only the connection on node b, and its run starts once: a second startRun through node a
// changes nothing.
func TestConsumerFencing4015(t *testing.T) {
	t.Parallel()
	c := newCluster(t)
	alex, lotse := c.w.People["Alex"], c.w.Agents["Lotse"]
	onA := c.connect(nodeA, lotse)
	onB := c.connect(nodeB, lotse)
	if code := c.waitClose(onA); code != 4015 {
		t.Fatalf("the older consumer connection on node a closed with %d, want 4015", code)
	}
	th := c.newThread(alex, "Support/Allgemein", "Fencing check")
	run := c.runOf(th)
	inv, _ := c.waitDispatch(onB, "agent.invocation", func(d dispatch) bool { return d.Resource.ID == run })
	if inv.InvocationToken == "" {
		t.Fatalf("agent.invocation without a token: %+v", inv)
	}
	type started struct {
		StartedAt string `json:"started_at"`
		Version   int64  `json:"version"`
	}
	var first, again started
	c.must(nodeB, lotse, http.MethodPost, "/runs/"+run+"/start", startBody(inv.InvocationToken), http.StatusOK).JSON(t, &first)
	// The same start through the other node is the compare-and-swap of 04 15.3: it changes
	// nothing (the answer repeats the started run).
	c.must(nodeA, lotse, http.MethodPost, "/runs/"+run+"/start", startBody(inv.InvocationToken), http.StatusOK).JSON(t, &again)
	if first.StartedAt == "" || again != first {
		t.Fatalf("startRun through node b %+v, again through node a %+v: the run started twice", first, again)
	}
	if err := onB.Err(); err != nil {
		t.Fatalf("the consumer connection on node b ended: %v", err)
	}
	if n := c.count(`SELECT count(*) FROM runs WHERE id = $1 AND state = 'running'`, run); n != 1 {
		t.Fatalf("%d running rows of the run", n)
	}
}

// TestDeviceReplacement4017 (04 6.1, M7-17 acceptance 2): a second identify of Alex's device on
// node b closes the older connection on node a with 4017 and leaves the new one open.
func TestDeviceReplacement4017(t *testing.T) {
	t.Parallel()
	c := newCluster(t)
	alex := c.w.People["Alex"]
	onA := c.connect(nodeA, alex)
	onB := c.connect(nodeB, alex)
	if code := c.waitClose(onA); code != 4017 {
		t.Fatalf("the older connection on node a closed with %d, want 4017", code)
	}
	c.focus(onB, alex.PrincipalID, false)
	if err := onB.Err(); err != nil {
		t.Fatalf("the newer connection on node b ended: %v", err)
	}
}

// pushJob is one job notify.push_send of the tests.
type pushJob struct {
	RecipientID    string `json:"recipient_id"`
	NotificationID string `json:"notification_id"`
	ReleaseSeq     int64  `json:"release_seq"`
}

// pushJobsFor returns the jobs notify.push_send of recipient in id order.
func (c *cluster) pushJobsFor(recipient string) []pushJob {
	c.t.Helper()
	rows, err := c.nodes[nodeA].AdminDB().Query(context.Background(),
		`SELECT args FROM jobs.river_job WHERE kind = 'notify.push_send' AND args->>'recipient_id' = $1 ORDER BY id`, recipient)
	if err != nil {
		c.t.Fatal(err)
	}
	defer rows.Close()
	var out []pushJob
	for rows.Next() {
		var b []byte
		if err := rows.Scan(&b); err != nil {
			c.t.Fatal(err)
		}
		var j pushJob
		if err := json.Unmarshal(b, &j); err != nil {
			c.t.Fatal(err)
		}
		out = append(out, j)
	}
	if err := rows.Err(); err != nil {
		c.t.Fatal(err)
	}
	return out
}

// phone writes an active phone of user with a push registration (a fixture outside the use
// cases, as in test/integration/notify).
func (c *cluster) phone(user string) {
	c.t.Helper()
	db := c.nodes[nodeA].AdminDB()
	device, session := ids.New(), ids.New()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO devices (id, user_id, name, platform, state) VALUES ($1, $2, 'Phone', 'ios', 'active')`, []any{device, user}},
		{`INSERT INTO auth_sessions (id, user_id, device_id, auth_method, expires_at) VALUES ($1, $2, $3, 'passkey', now() + interval '1 day')`,
			[]any{session, user, device}},
		{`INSERT INTO push_registrations (id, user_id, device_id, auth_session_id, provider, token, app_id, created_at)
			VALUES ($1, $2, $3, $4, 'apns', $5, 'com.example.app', now())`, []any{ids.New(), user, device, session, "device-token-" + ids.New().String()}},
	} {
		if _, err := db.Exec(context.Background(), q.sql, q.args...); err != nil {
			c.t.Fatalf("%s: %v", q.sql, err)
		}
	}
}

// TestDesktopHintWithSeparateWorker (13 12 point 4, RK-37, M7-17 acceptance 3): Alex's desktop is
// live on node b with an empty focus; the worker decides the hint of the Owner's message through
// gateway_presence: Alex's desktop gets notification.created on node b through NOTIFY ephemeral
// and the phone's push is delayed, exactly as in role all. With the desktop idle, the next hint
// pushes at once.
func TestDesktopHintWithSeparateWorker(t *testing.T) {
	t.Parallel()
	c := newCluster(t)
	alex, owner := c.w.People["Alex"], c.w.Owner
	c.phone(alex.PrincipalID)
	threads := []string{c.w.Threads["Support/Allgemein"], c.w.Threads["Support/Eskalation"]}
	for _, th := range threads {
		c.must(nodeA, alex, http.MethodPut, "/me/notification-preferences/thread/"+th, map[string]any{"level": "all"}, http.StatusOK)
	}
	desk := c.connect(nodeB, alex)
	c.focus(desk, alex.PrincipalID, false)

	c.post(nodeA, owner, threads[0], "The export is back")
	hint, _ := c.waitDispatch(desk, "notification.created", nil)
	var p struct {
		NID string `json:"nid"`
	}
	if err := json.Unmarshal(hint.Payload, &p); err != nil || p.NID == "" {
		t.Fatalf("notification.created on node b: %s %v", hint.Payload, err)
	}
	var first []pushJob
	testkit.Until(t, "the push job of the first hint", func() bool {
		first = c.pushJobsFor(alex.PrincipalID)
		return len(first) == 1
	})
	if first[0].NotificationID != p.NID || first[0].ReleaseSeq == 0 {
		t.Fatalf("push jobs %+v, want one push delayed behind the active desktop for %s", first, p.NID)
	}

	c.focus(desk, alex.PrincipalID, true)
	c.post(nodeA, owner, threads[1], "The import is back")
	var jobs []pushJob
	testkit.Until(t, "the push job of the second hint", func() bool {
		jobs = c.pushJobsFor(alex.PrincipalID)
		return len(jobs) == 2
	})
	if jobs[1].ReleaseSeq != 0 {
		t.Fatalf("push job %+v with the desktop idle, want an immediate push", jobs[1])
	}
}

// kidOf returns the kid of the JWS header of token.
func kidOf(t *testing.T, token string) string {
	t.Helper()
	head, _, _ := strings.Cut(token, ".")
	b, err := base64.RawURLEncoding.DecodeString(head)
	if err != nil {
		t.Fatalf("token header: %v", err)
	}
	var h struct {
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(b, &h); err != nil {
		t.Fatal(err)
	}
	return h.Kid
}

// refresh refreshes the token pair of cl through node and adopts it.
func (c *cluster) refresh(node string, cl *harness.Client) {
	c.t.Helper()
	_, rt := cl.Tokens()
	r := cl.DoAnonymous(c.t.Context(), harness.Request{Method: http.MethodPost, Path: "/auth/token/refresh", Header: via(node),
		Body: map[string]any{"refresh_token": rt}})
	if r.Status != http.StatusOK {
		c.t.Fatalf("refreshToken through node %s: %d %s", node, r.Status, r.Body)
	}
	if err := cl.Adopt(r.Body); err != nil {
		c.t.Fatal(err)
	}
}

// TestSharedKeyStore (16 5.2, ADR 0066, M7-17 acceptance 4): every node uses one signing key
// store. An emergency rotation of the access token key on node a issues tokens of the new key at
// once; node b accepts such a token within signing_key.cache_ttl, and node b, which reads the
// private key from the shared store, issues tokens of the new key too.
func TestSharedKeyStore(t *testing.T) {
	t.Parallel()
	c := newCluster(t)
	alex, owner := c.w.People["Alex"], c.w.Owner
	channel := "/channels/" + c.w.Channels["Support/Allgemein"]
	r, err := harness.DoWithStepUp(t.Context(), owner, harness.Request{Method: http.MethodPost, Path: "/signing-keys/rotate", Header: via(nodeA),
		Body: map[string]any{"purpose": "access_token", "mode": "emergency", "reason": "multi-node test"}})
	if err != nil || r.Status != http.StatusOK {
		t.Fatalf("rotateSigningKey: %v %d %s", err, r.Status, r.Body)
	}
	var rot struct {
		NewKid string `json:"new_kid"`
	}
	r.JSON(t, &rot)
	c.refresh(nodeA, alex)
	if access, _ := alex.Tokens(); kidOf(t, access) != rot.NewKid {
		t.Fatalf("node a signed with %q, want the new key %q", kidOf(t, access), rot.NewKid)
	}
	testkit.Until(t, "node b accepts a token of the new key", func() bool {
		return c.do(nodeB, alex, http.MethodGet, channel, nil).Status == http.StatusOK
	})
	testkit.Until(t, "node b signs with the new key", func() bool {
		c.refresh(nodeB, alex)
		access, _ := alex.Tokens()
		return kidOf(t, access) == rot.NewKid
	})
	c.must(nodeA, alex, http.MethodGet, channel, nil, http.StatusOK)
}
