// SPDX-License-Identifier: Apache-2.0

package canary

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity/adapters/oidc/oidctest"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/webauthn/virtualauthn"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// Scenario drives the server of a Run on the matrix world. Its helpers fail the test with
// t.Errorf on an unexpected status, so one run reports every step that did not work.
type Scenario struct {
	*Run
	W   *harness.World
	Ctx context.Context

	// Observers are the principals that must not see the canaries of the scenario's scopes.
	Observers map[string]*harness.Client
	// Readable are the scopes each observer may read (AllowRules.Readable).
	Readable map[string]map[string]bool
	// GatewaySent counts the client gateway ops the scenario sent.
	GatewaySent map[string]int
	// start is when the scenario's clients got their network canaries.
	start time.Time
	// gw are the observer gateway connections, open for the whole scenario.
	gw map[string]*harness.GatewayConn
	// threads and messages are IDs the later steps reuse.
	threads, messages []string
	// newUser is the person the people step created.
	newUser string
	// enrollee is the device of the passkey enrollment, alexPhone the device of the password
	// login, authn the virtual authenticator of the last passkey registration.
	enrollee, alexPhone *harness.Client
	authn               *virtualauthn.Authenticator
	// owner is the local credential of the owner, consoleCookie the cookie of its console tab,
	// ownerLogin the provider login linked to the owner, lastRaw the body of the last raw answer.
	owner         *localAccount
	consoleCookie *http.Cookie
	ownerLogin    oidctest.Login
	lastRaw       map[string]any
	// TerminalIO is what the terminal step with Noise saw (scenario_terminal.go).
	TerminalIO TerminalIO
}

// harnessResponse is the response of a step.
type harnessResponse = harness.Response

// jsonText renders a decoded JSON scalar as text ("" for nil).
func jsonText(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	default:
		return fmt.Sprint(x)
	}
}

// req is one request of a step.
type req struct {
	method, path string
	body         any
	header       http.Header
}

// call sends r as c (with a step-up on step_up_required) and reports a status outside want.
func (sc *Scenario) call(c *harness.Client, r req, want ...int) harness.Response {
	sc.T.Helper()
	resp, err := harness.DoWithStepUp(sc.Ctx, c, harness.Request{Method: r.method, Path: r.path, Body: r.body, Header: r.header})
	if err != nil {
		sc.T.Errorf("%s %s: %v", r.method, routeOf(r.method, r.path), err)
		return resp
	}
	if len(want) > 0 && !slices.Contains(want, resp.Status) {
		sc.T.Errorf("%s %s (%s) = %d %s, want %v", r.method, routeOf(r.method, r.path), r.path, resp.Status, clip(resp.Body), want)
	}
	return resp
}

// get, post, put, patch and del are call with the method.
func (sc *Scenario) get(c *harness.Client, path string, want ...int) harness.Response {
	sc.T.Helper()
	return sc.call(c, req{method: http.MethodGet, path: path}, want...)
}

func (sc *Scenario) post(c *harness.Client, path string, body any, want ...int) harness.Response {
	sc.T.Helper()
	return sc.call(c, req{method: http.MethodPost, path: path, body: body}, want...)
}

func (sc *Scenario) put(c *harness.Client, path string, body any, want ...int) harness.Response {
	sc.T.Helper()
	return sc.call(c, req{method: http.MethodPut, path: path, body: body}, want...)
}

func (sc *Scenario) patch(c *harness.Client, path string, body any, want ...int) {
	sc.T.Helper()
	sc.call(c, req{method: http.MethodPatch, path: path, body: body}, want...)
}

func (sc *Scenario) del(c *harness.Client, path string, want ...int) {
	sc.T.Helper()
	sc.call(c, req{method: http.MethodDelete, path: path}, want...)
}

// matched sends r with If-Match set to the ETag of a GET of etagPath.
func (sc *Scenario) matched(c *harness.Client, etagPath string, r req, want ...int) {
	sc.T.Helper()
	cur := sc.get(c, etagPath, http.StatusOK)
	h := r.header.Clone()
	if h == nil {
		h = http.Header{}
	}
	h.Set("If-Match", cur.Header.Get("ETag"))
	r.header = h
	sc.versioned(c, r, want...)
}

// versioned sends r and, on a 412 whose problem names the current version, repeats it once
// with that version as If-Match (the version of a sub-resource can differ from the ETag of the
// resource the step read).
func (sc *Scenario) versioned(c *harness.Client, r req, want ...int) {
	sc.T.Helper()
	resp, err := harness.DoWithStepUp(sc.Ctx, c, harness.Request{Method: r.method, Path: r.path, Body: r.body, Header: r.header})
	if err == nil && resp.Status == http.StatusPreconditionFailed {
		if cur, isObj := obj(resp)["current"].(map[string]any); isObj && jsonText(cur["version"]) != "" {
			r.header = r.header.Clone()
			r.header.Set("If-Match", `"`+jsonText(cur["version"])+`"`)
			sc.call(c, r, want...)
			return
		}
	}
	if err != nil {
		sc.T.Errorf("%s %s: %v", r.method, routeOf(r.method, r.path), err)
	} else if len(want) > 0 && !contains(want, resp.Status) {
		sc.T.Errorf("%s %s (%s) = %d %s, want %v", r.method, routeOf(r.method, r.path), r.path, resp.Status, clip(resp.Body), want)
	}
}

// obj decodes a JSON object body; a body that is no object gives an empty map.
func obj(r harness.Response) map[string]any {
	m := map[string]any{}
	_ = json.Unmarshal(r.Body, &m)
	return m
}

// str returns the string at the dotted path of m ("thread.id"), or "".
func str(m map[string]any, path string) string {
	var cur any = m
	for p := range strings.SplitSeq(path, ".") {
		mm, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = mm[p]
	}
	s, _ := cur.(string)
	return s
}

// id returns the member id of a creation response.
func id(r harness.Response) string { return str(obj(r), "id") }

// items returns the member items of a list response.
func items(r harness.Response) []map[string]any {
	var body struct {
		Items []map[string]any `json:"items"`
	}
	_ = json.Unmarshal(r.Body, &body)
	return body.Items
}

// clip shortens a body for an error message.
func clip(b []byte) string {
	const max = 400
	if len(b) > max {
		return string(b[:max]) + "..."
	}
	return string(b)
}

// header builds a header from name, value pairs.
func header(kv ...string) http.Header {
	h := http.Header{}
	for i := 0; i+1 < len(kv); i += 2 {
		h.Set(kv[i], kv[i+1])
	}
	return h
}

// ifMatch is the If-Match header of version.
func ifMatch(version any) http.Header {
	return header("If-Match", fmt.Sprintf(`"%v"`, version))
}

// firstID returns the first non-empty string at one of the dotted paths of the body of r.
func firstID(r harness.Response, paths ...string) string {
	m := obj(r)
	for _, p := range paths {
		if v := str(m, p); v != "" {
			return v
		}
	}
	return ""
}
