// SPDX-License-Identifier: Apache-2.0

package agents_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/test/harness"
)

// defaultToolClass is the tool class of each agent of the world (07 10): world_test.go sets it
// through setAgentToolClass, and a Fake-Agent reports it unless its runtime says otherwise.
var defaultToolClass = map[string]string{"Lotse": "chat", "Texter": "scoped", "Runner": "host", "Assistent": "scoped"}

// caseWorld is the matrix world of M1-71 with the extensions of brief Design note 10, built
// through public operations only.
type caseWorld struct {
	srv        *harness.Server
	w          *harness.World
	instanceID string
	// tracker is the client of the app Tracker (extra "tracker").
	tracker   *harness.Client
	trackerID string
	// webhookID and webhookToken name the incoming webhook (extra "webhook"); the token lives
	// only here.
	webhookID, webhookToken string
	// mains maps "Space/Channel" to the main thread of the channel.
	mains map[string]string
}

// buildCaseWorld builds BuildMatrixWorld and the extensions the scenario asks for.
func buildCaseWorld(t testing.TB, srv *harness.Server, sc *Scenario) *caseWorld {
	t.Helper()
	cw := &caseWorld{srv: srv, w: harness.BuildMatrixWorld(t, srv)}
	o := cw.w.Owner
	var disc struct {
		InstanceID string `json:"instance_id"`
	}
	harness.MustDo(t, o, harness.Request{Method: http.MethodGet, Path: "/instance/discovery"}, http.StatusOK).JSON(t, &disc)
	cw.instanceID = disc.InstanceID
	cw.mains = map[string]string{}
	for key, id := range cw.w.Channels {
		var view struct {
			Channel struct {
				MainThreadID string `json:"main_thread_id"`
			} `json:"channel"`
		}
		harness.MustDo(t, o, harness.Request{Method: http.MethodGet, Path: "/channels/" + id}, http.StatusOK).JSON(t, &view)
		cw.mains[key] = view.Channel.MainThreadID
	}
	for _, name := range []string{"Lotse", "Texter", "Runner", "Assistent"} {
		cw.setToolClass(t, name, defaultToolClass[name])
	}
	for _, extra := range sc.Extras {
		switch extra {
		case "tracker":
			cw.installTracker(t)
		case "webhook":
			cw.createWebhook(t)
		}
	}
	return cw
}

// setToolClass sets the configured tool class of an agent when it differs (setAgentToolClass).
func (cw *caseWorld) setToolClass(t testing.TB, agent, class string) {
	t.Helper()
	o := cw.w.Owner
	path := "/agents/" + cw.w.Agents[agent].PrincipalID
	cur := harness.MustDo(t, o, harness.Request{Method: http.MethodGet, Path: path}, http.StatusOK)
	var a struct {
		ToolClass string `json:"tool_class"`
	}
	cur.JSON(t, &a)
	if a.ToolClass == class {
		return
	}
	h := http.Header{}
	h.Set("If-Match", cur.Header.Get("ETag"))
	harness.MustDo(t, o, harness.Request{Method: http.MethodPut, Path: path + "/tool-class", Header: h,
		Body: map[string]any{"tool_class": class, "reason": "matrix world"}}, http.StatusOK)
}

// installTracker installs the app Tracker with scope messages:write, grants it write on
// Support/Eskalation (G-6) and creates its DPoP-bound token.
func (cw *caseWorld) installTracker(t testing.TB) {
	t.Helper()
	o := cw.w.Owner
	resp := harness.MustDo(t, o, harness.Request{Method: http.MethodPost, Path: "/apps", Body: map[string]any{
		"manifest": map[string]any{
			"schema_version": 1, "slug": "tracker", "name": "Tracker", "description": "Posts ticket updates.",
			"publisher": "Example", "version": "1.0.0", "min_api_version": "0.1",
			"scopes": map[string]any{"required": []string{"messages:write"}, "optional": []string{}},
		},
		"granted_scopes": []string{"messages:write"}, "reason": "matrix world",
	}}, http.StatusCreated)
	var app struct {
		App struct {
			ID string `json:"id"`
		} `json:"app"`
		ID          string `json:"id"`
		PrincipalID string `json:"principal_id"`
	}
	resp.JSON(t, &app)
	id := app.App.ID
	if id == "" {
		id = app.PrincipalID
	}
	if id == "" {
		id = app.ID
	}
	cw.trackerID = id
	harness.MustDo(t, o, harness.Request{Method: http.MethodPost, Path: "/grants", Body: map[string]any{
		"subject_kind": "app", "subject_id": id, "target_kind": "channel", "target_id": cw.w.Channels["Support/Eskalation"],
		"level": "write", "reason": "matrix world G-6",
	}}, http.StatusCreated)
	cw.w.Grants["G-6"] = id
	cw.tracker = cw.srv.NewTokenClient(t, &harness.Client{PrincipalID: id})
}

// createWebhook creates an incoming webhook of format native on Support/Eskalation for Tracker
// (installing Tracker when the scenario did not ask for it).
func (cw *caseWorld) createWebhook(t testing.TB) {
	t.Helper()
	if cw.trackerID == "" {
		cw.installTracker(t)
	}
	resp := harness.MustDo(t, cw.w.Owner, harness.Request{Method: http.MethodPost, Path: "/channels/" + cw.w.Channels["Support/Eskalation"] + "/incoming-webhooks",
		Body: map[string]any{"app_id": cw.trackerID, "name": "Alerts"}}, http.StatusCreated)
	var created struct {
		Webhook struct {
			ID string `json:"id"`
		} `json:"webhook"`
		URL string `json:"url"`
	}
	resp.JSON(t, &created)
	cw.webhookID = created.Webhook.ID
	cw.webhookToken = created.URL[strings.LastIndexByte(created.URL, '/')+1:]
}

// postWebhook sends postIncomingWebhook to the world's webhook with its token.
func (cw *caseWorld) postWebhook(ctx context.Context, body any) harness.Response {
	raw, err := json.Marshal(body)
	if err != nil {
		return harness.Response{Body: []byte(err.Error())}
	}
	url := cw.srv.APIBase + "/hooks/" + cw.webhookID + "/" + cw.webhookToken
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return harness.Response{Body: []byte(err.Error())}
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return harness.Response{Body: []byte(err.Error())}
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return harness.Response{Status: resp.StatusCode, Header: resp.Header, Body: b}
}

// subject returns the client of a subject of a human step.
func (cw *caseWorld) subject(name string) (*harness.Client, error) {
	switch name {
	case "owner":
		return cw.w.Owner, nil
	case "tracker":
		if cw.tracker == nil {
			return nil, fmt.Errorf("the scenario has no extra tracker")
		}
		return cw.tracker, nil
	case "webhook":
		if cw.webhookID == "" {
			return nil, fmt.Errorf("the scenario has no extra webhook")
		}
		return cw.w.Owner, nil
	}
	c, ok := cw.w.People[strings.ToUpper(name[:1])+name[1:]]
	if !ok {
		return nil, fmt.Errorf("unknown subject %s", name)
	}
	return c, nil
}

// ref resolves a world reference: kind channel, thread, space, user, agent, grant, group,
// message ("<Space/Channel>.<n>", the n-th message of the world's thread there), app, webhook.
func (cw *caseWorld) ref(kind, name string) (string, bool) {
	if cw == nil || cw.w == nil {
		return "", false
	}
	w := cw.w
	var id string
	switch kind {
	case "channel":
		id = w.Channels[name]
	case "thread":
		id = w.Threads[name]
	case "main":
		id = cw.mains[name]
	case "space":
		id = w.Spaces[name]
	case "user":
		if c, ok := w.People[name]; ok {
			id = c.PrincipalID
		} else if name == "Owner" {
			id = w.Owner.PrincipalID
		}
	case "agent":
		if c, ok := w.Agents[name]; ok {
			id = c.PrincipalID
		}
	case "grant":
		id = w.Grants[name]
	case "group":
		id = w.Groups[name]
	case "app":
		if name == "Tracker" {
			id = cw.trackerID
		}
	case "webhook":
		id = cw.webhookID
	case "message":
		i := strings.LastIndexByte(name, '.')
		if i < 0 {
			return "", false
		}
		n, err := strconv.Atoi(name[i+1:])
		msgs := w.Messages[name[:i]]
		if err != nil || n < 1 || n > len(msgs) {
			return "", false
		}
		id = msgs[n-1]
	default:
		return "", false
	}
	return id, id != ""
}
