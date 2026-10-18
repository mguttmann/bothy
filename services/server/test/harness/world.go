// SPDX-License-Identifier: Apache-2.0

package harness

import (
	"net/http"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// World is the M1 subset of the example world of docs/spec/07 "Examples" and "Test matrix",
// built through the public API. Names are the keys: spaces by name, channels as "Space/Channel",
// people and agents by display name, grants by their label G-1 to G-5.
type World struct {
	// Owner is the first owner of the server, the creator of the world.
	Owner *Client
	// Spaces maps a space name to its ID.
	Spaces map[string]string
	// Channels maps "Space/Channel" to the channel ID.
	Channels map[string]string
	// People are the signed-in persons Alex, Sam and Robin.
	People map[string]*Client
	// Agents are the agents Lotse, Texter, Runner and Assistent with DPoP-bound API tokens.
	Agents map[string]*Client
	// Groups maps a group name (Leser) to its ID.
	Groups map[string]string
	// Grants maps G-1 to G-5 to the grant IDs.
	Grants map[string]string
	// Threads maps "Space/Channel" to the thread created there (the forum post for Projekt-A).
	Threads map[string]string
	// Messages maps "Space/Channel" to the IDs of the messages of that thread, oldest first.
	Messages map[string][]string
}

// The relative expiry times of the world (never calendar dates).
const (
	membershipTTL = 30 * 24 * time.Hour
	grantTTL      = 14 * 24 * time.Hour
)

// worldChannels are the channels of the world with their kind, in creation order.
var worldChannels = []struct{ space, name, kind string }{
	{"Support", "Allgemein", "text"},
	{"Support", "Eskalation", "text"},
	{"Marketing", "Kampagnen", "text"},
	{"Engineering", "Projekt-A", "forum"},
	{"Engineering", "Werkzeuge", "text"},
	{"Research", "Notizen", "text"},
}

// BuildMatrixWorld builds the world of 07 "Examples" for M1 on s as the owner: spaces Support,
// Marketing, Engineering, Research with their channels; people Alex, Sam, Robin; agents Lotse,
// Texter and Runner (organization) and Assistent (personal, the owner holds the credentials) as
// operators of Support, Marketing, Engineering/Werkzeuge and Research; group Leser with Sam
// (membership with expiry); grants G-1 to G-5; one thread with two messages per text channel and
// one forum post in Projekt-A. The app Tracker, G-6, the DM Alex-Sam, project binding and tool
// class host come with later milestones. Expiry times are relative to the harness clock.
func BuildMatrixWorld(tb testing.TB, s *Server) *World {
	tb.Helper()
	w := &World{
		Owner: s.Owner, Spaces: map[string]string{}, Channels: map[string]string{}, People: map[string]*Client{}, Agents: map[string]*Client{},
		Groups: map[string]string{}, Grants: map[string]string{}, Threads: map[string]string{}, Messages: map[string][]string{},
	}
	o := s.Owner
	for _, name := range []string{"Support", "Marketing", "Engineering", "Research"} {
		w.Spaces[name] = created(tb, MustDo(tb, o, Request{Method: http.MethodPost, Path: "/spaces", Body: map[string]any{"name": name}}, http.StatusCreated))
	}
	mainThreads := map[string]string{}
	for _, c := range worldChannels {
		var ch struct {
			ID           string `json:"id"`
			MainThreadID string `json:"main_thread_id"`
		}
		MustDo(tb, o, Request{Method: http.MethodPost, Path: "/spaces/" + w.Spaces[c.space] + "/channels",
			Body: map[string]any{"kind": c.kind, "name": c.name}}, http.StatusCreated).JSON(tb, &ch)
		w.Channels[c.space+"/"+c.name], mainThreads[c.space+"/"+c.name] = ch.ID, ch.MainThreadID
	}
	for _, p := range []UserSpec{
		{Handle: "alex", DisplayName: "Alex", Email: "alex@example.com"},
		{Handle: "sam", DisplayName: "Sam", Email: "sam@example.com"},
		{Handle: "robin", DisplayName: "Robin", Email: "robin@example.com"},
	} {
		w.People[p.DisplayName] = s.NewUser(tb, p)
	}
	for _, a := range []AgentSpec{
		{Handle: "lotse", DisplayName: "Lotse", BillingMode: "organization"},
		{Handle: "texter", DisplayName: "Texter", BillingMode: "organization"},
		{Handle: "runner", DisplayName: "Runner", BillingMode: "organization"},
		{Handle: "assistent", DisplayName: "Assistent", BillingMode: "personal", CredentialHolder: o.PrincipalID},
	} {
		w.Agents[a.DisplayName] = s.NewAgent(tb, a)
	}
	setOperator(tb, o, "/spaces/"+w.Spaces["Support"], w.Agents["Lotse"].PrincipalID)
	setOperator(tb, o, "/spaces/"+w.Spaces["Marketing"], w.Agents["Texter"].PrincipalID)
	setOperator(tb, o, "/channels/"+w.Channels["Engineering/Werkzeuge"], w.Agents["Runner"].PrincipalID)
	setOperator(tb, o, "/spaces/"+w.Spaces["Research"], w.Agents["Assistent"].PrincipalID)

	now := s.clk.Now()
	w.Groups["Leser"] = created(tb, MustDo(tb, o, Request{Method: http.MethodPost, Path: "/groups", Body: map[string]any{"name": "Leser"}}, http.StatusCreated))
	MustDo(tb, o, Request{Method: http.MethodPost, Path: "/groups/" + w.Groups["Leser"] + "/members", Body: map[string]any{
		"user_id": w.People["Sam"].PrincipalID, "expires_at": timestamp(now.Add(membershipTTL)),
	}}, http.StatusCreated)
	for _, g := range []struct {
		label, subjectKind, subject, targetKind, target, level string
		caps                                                   []string
		expires                                                bool
	}{
		{"G-1", "user", w.People["Alex"].PrincipalID, "space", w.Spaces["Support"], "trigger", []string{"upload", "approve_actions"}, false},
		{"G-2", "user", w.People["Sam"].PrincipalID, "space", w.Spaces["Marketing"], "trigger", []string{"upload"}, false},
		{"G-3", "user", w.People["Robin"].PrincipalID, "channel", w.Channels["Engineering/Projekt-A"], "trigger", []string{"upload"}, true},
		{"G-4", "group", w.Groups["Leser"], "channel", w.Channels["Support/Allgemein"], "read", nil, false},
		{"G-5", "agent", w.Agents["Lotse"].PrincipalID, "space", w.Spaces["Support"], "write", []string{"upload"}, false},
	} {
		body := map[string]any{"subject_kind": g.subjectKind, "subject_id": g.subject, "target_kind": g.targetKind, "target_id": g.target,
			"level": g.level, "reason": "matrix world " + g.label}
		if len(g.caps) > 0 {
			body["capabilities"] = g.caps
		}
		if g.expires {
			body["expires_at"] = timestamp(now.Add(grantTTL))
		}
		w.Grants[g.label] = created(tb, MustDo(tb, o, Request{Method: http.MethodPost, Path: "/grants", Body: body}, http.StatusCreated))
	}
	for _, c := range worldChannels {
		key := c.space + "/" + c.name
		w.Threads[key] = buildThread(tb, o, w.Channels[key], mainThreads[key], c.name)
		if c.kind == "forum" {
			continue
		}
		postMessage(tb, o, w.Threads[key], "Zweite Nachricht in "+c.name)
		var msgs struct {
			Items []struct {
				ID string `json:"id"`
			} `json:"items"`
		}
		MustDo(tb, o, Request{Method: http.MethodGet, Path: "/threads/" + w.Threads[key] + "/messages"}, http.StatusOK).JSON(tb, &msgs)
		for _, m := range msgs.Items {
			w.Messages[key] = append(w.Messages[key], m.ID)
		}
	}
	return w
}

// buildThread creates a thread in the channel: a forum post with its first message (no main
// thread), or in a text channel a thread on a new message of its main thread (03 "threads": a thread of a text channel
// hangs on a parent message).
func buildThread(tb testing.TB, o *Client, channel, mainThread, name string) string {
	tb.Helper()
	body := map[string]any{"title": "Start " + name, "first_message": map[string]any{"client_msg_id": ids.New().String(), "content_md": "Hallo " + name}}
	if mainThread != "" {
		body["parent_message_id"] = postMessage(tb, o, mainThread, "Frage in "+name)
	}
	var th struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	MustDo(tb, o, Request{Method: http.MethodPost, Path: "/channels/" + channel + "/threads", Body: body}, http.StatusCreated).JSON(tb, &th)
	return th.Thread.ID
}

// postMessage posts a text message into the thread and returns its ID.
func postMessage(tb testing.TB, o *Client, thread, text string) string {
	tb.Helper()
	return created(tb, MustDo(tb, o, Request{Method: http.MethodPost, Path: "/threads/" + thread + "/messages", Body: map[string]any{
		"client_msg_id": ids.New().String(), "content_md": text,
	}}, http.StatusCreated))
}

// created returns the member id of a creation response.
func created(tb testing.TB, r Response) string {
	tb.Helper()
	var body struct {
		ID string `json:"id"`
	}
	r.JSON(tb, &body)
	if body.ID == "" {
		tb.Fatalf("harness: no id in %s", r.Body)
	}
	return body.ID
}

// setOperator makes agent the operator of the space or channel at path (setSpaceOperator,
// setChannelOperator) with the ETag of the current state as If-Match.
func setOperator(tb testing.TB, o *Client, path, agent string) {
	tb.Helper()
	cur := MustDo(tb, o, Request{Method: http.MethodGet, Path: path}, http.StatusOK)
	h := http.Header{}
	h.Set("If-Match", cur.Header.Get("ETag"))
	MustDo(tb, o, Request{Method: http.MethodPut, Path: path + "/operator", Header: h,
		Body: map[string]any{"mode": "agent", "agent_id": agent, "reason": "matrix world"}}, http.StatusOK)
}

// timestamp formats t as RFC 3339 in UTC with milliseconds (06 "Basic rules").
func timestamp(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000Z") }
