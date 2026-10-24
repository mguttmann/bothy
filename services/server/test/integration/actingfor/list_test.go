// SPDX-License-Identifier: Apache-2.0

package actingfor_test

import (
	"encoding/json"
	"maps"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/OWNER/bothy/services/server/api/routes"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// scopeGolden is the file of the operations whose requests call Scope (brief M2-14
// "TestSR9_EveryScopedList"): one operationId per line, sorted, after a header of comment lines.
const scopeGolden = "testdata/sr9_scope_operations.golden"

// engineeringParams are the path parameters the fixture fills with an ID of Engineering; a request
// with T on such a path must answer like one with an unknown ID.
var engineeringParams = []string{"space_id", "channel_id", "thread_id", "message_id", "run_id"}

// listQueries are the query strings an operation needs to answer at all: listEvents requires the
// cursor after (04 section 7.3; 0 is the start of the log).
var listQueries = map[string]string{"listEvents": "after=0"}

// deliveryMode are the operations that decide in delivery mode, where the header never narrows
// (brief design note 6): pollAgentEvents admits the events of the consumer with the agent's own
// grants. They are left out of the operations that called Scope: whether a poll reads a page (and
// so calls Scope) depends on events that arrived after the last poll, not on the operation.
var deliveryMode = []string{"pollAgentEvents"}

// mustContainEngineering are the operations whose answer without the header lists something of
// Engineering in this world, so the check with T is not vacuous. The brief names listMyMentions
// too, but mention.list admits people only: an agent gets 403 forbidden subject_kind_not_allowed,
// with and without the header, so it lists nothing for Lotse either way.
var mustContainEngineering = []string{"listSpaces", "getSnapshot", "listEvents", "listActiveRuns"}

// paramPattern matches a path parameter of an operation path.
var paramPattern = regexp.MustCompile(`\{([a-z_]+)\}`)

// everyScopedList (brief "TestSR9_EveryScopedList"): every enabled GET operation an agent may call
// is requested as Lotse without and with T. For every operation whose requests called Scope, no
// page answered with T holds an ID of Engineering (space, channels, threads, messages, runs),
// except the delivery-mode poll; without the header the operations of mustContainEngineering hold
// one; a path that names Engineering answers with T like an unknown ID; and the operations that
// called Scope equal the golden file.
func (w *readsWorld) everyScopedList(e *env) {
	runnerRun := e.text(`SELECT id::text FROM runs WHERE agent_id = $1 ORDER BY queued_at, id LIMIT 1`, e.w.Agents["Runner"].PrincipalID)
	e.post(e.owner, w.post, "@lotse bitte Projekt-A ansehen")
	e.page(e.lotse, "sr9list")
	fixture := map[string]string{
		"space_id": e.w.Spaces["Engineering"], "channel_id": e.w.Channels["Engineering/Projekt-A"], "thread_id": w.post,
		"message_id": w.postMsg, "run_id": runnerRun, "agent_id": e.lotse.PrincipalID, "principal_id": e.lotse.PrincipalID,
		"user_id": e.alex.PrincipalID, "consumer": "sr9list", "group_id": e.w.Groups["Leser"], "grant_id": e.lotseRead, "code": "not_found",
	}
	eng := e.engineeringIDs()
	called := map[string]bool{}
	var skipped, random []string
	for _, op := range routes.Enabled() {
		if op.Method != http.MethodGet {
			continue
		}
		if op.Public {
			// Discovery, keys, problem types, the logo and the OIDC callback: no principal, no Scope.
			skipped = append(skipped, op.ID)
			continue
		}
		path, randomParams, addressed := fill(op, fixture)
		if len(randomParams) > 0 {
			random = append(random, op.ID+"("+strings.Join(randomParams, ",")+")")
		}
		w.scopes.take()
		plain := e.pages(op.ID, path)
		bound := e.pages(op.ID, path, w.r.token)
		// A delivery-mode poll calls Scope only when events wait after its window's read (the
		// feed reads a page in filter mode), which depends on the background events of the world,
		// so it is never part of the comparison with the golden file.
		if w.scopes.take()[op.ID] && !slices.Contains(deliveryMode, op.ID) {
			called[op.ID] = true
		}
		if called[op.ID] {
			for _, r := range bound {
				if id := firstID(withoutInstance(r), eng); id != "" {
					e.t.Errorf("%s with T answers %d with %s of Engineering", op.ID, r.Status, id)
				}
			}
		}
		if slices.Contains(mustContainEngineering, op.ID) && !anyContains(plain, eng) {
			e.t.Errorf("%s without the header lists nothing of Engineering; the check is vacuous", op.ID)
		}
		if addressed {
			unknown := fixtureWith(fixture, engineeringParams)
			upath, _, _ := fill(op, unknown)
			got, want := bound[0], e.do(e.lotse, http.MethodGet, upath+query(op.ID), nil, w.r.token)
			if got.Status < 400 {
				e.t.Errorf("%s on Engineering with T answers %d, but an unknown ID answers %d", op.ID, got.Status, want.Status)
				continue
			}
			e.sameAnswer(got, want, op.ID+" on Engineering with T")
		}
	}
	e.t.Logf("operations skipped (public, no principal): %s", strings.Join(skipped, ", "))
	e.t.Logf("operations requested with a random ID (the world has no such object for Lotse): %s", strings.Join(random, ", "))
	got := slices.Sorted(maps.Keys(called))
	want := readGolden(e, got)
	if !slices.Equal(got, want) {
		e.t.Fatalf("the operations that called Scope are\n%s\nbut %s lists\n%s\n(add a new list operation to the golden file after checking it with T)",
			strings.Join(got, "\n"), scopeGolden, strings.Join(want, "\n"))
	}
}

// fill returns the path of op relative to /api/v1 with the parameters of fixture, a random UUIDv7
// for every other parameter (named in the second result) and whether a parameter of
// engineeringParams names an object of Engineering.
func fill(op routes.Operation, fixture map[string]string) (string, []string, bool) {
	var randomParams []string
	addressed := false
	path := paramPattern.ReplaceAllStringFunc(strings.TrimPrefix(op.Path, "/api/v1"), func(m string) string {
		name := m[1 : len(m)-1]
		if slices.Contains(engineeringParams, name) && fixture[name] != "" && !strings.HasPrefix(fixture[name], unknownMark) {
			addressed = true
		}
		if v, ok := fixture[name]; ok {
			return strings.TrimPrefix(v, unknownMark)
		}
		randomParams = append(randomParams, name)
		return ids.New().String()
	})
	return path, randomParams, addressed
}

// unknownMark prefixes a fixture value that is an unknown ID, so fill does not count it as
// Engineering.
const unknownMark = "unknown:"

// fixtureWith returns fixture with a fresh unknown UUIDv7 for every parameter of names.
func fixtureWith(fixture map[string]string, names []string) map[string]string {
	out := map[string]string{}
	for k, v := range fixture {
		out[k] = v
	}
	for _, n := range names {
		out[n] = unknownMark + ids.New().String()
	}
	return out
}

// query returns the query string of listQueries for op with its separator, or "".
func query(op string) string {
	if q := listQueries[op]; q != "" {
		return "?" + q
	}
	return ""
}

// maxPages bounds the pages of one list request.
const maxPages = 50

// pages requests path for the operation op as Lotse with tokens and follows every page:
// next_cursor as text for the cursor lists, the numeric next_cursor of listEvents up to head_seq.
func (e *env) pages(op, path string, tokens ...string) []harness.Response {
	e.t.Helper()
	var out []harness.Response
	q := query(op)
	for range maxPages {
		r := e.do(e.lotse, http.MethodGet, path+q, nil, tokens...)
		out = append(out, r)
		if r.Status != http.StatusOK {
			return out
		}
		var page struct {
			NextCursor json.RawMessage `json:"next_cursor"`
			HeadSeq    int64           `json:"head_seq"`
			Events     []any           `json:"events"`
		}
		if json.Unmarshal(r.Body, &page) != nil || len(page.NextCursor) == 0 {
			return out
		}
		var text string
		var seq int64
		switch {
		case op == "listEvents" && json.Unmarshal(page.NextCursor, &seq) == nil:
			if len(page.Events) == 0 || seq >= page.HeadSeq {
				return out
			}
			q = "?after=" + itoa(int(seq))
		case json.Unmarshal(page.NextCursor, &text) == nil && text != "":
			q = "?cursor=" + url.QueryEscape(text)
			if base := query(op); base != "" {
				q = base + "&cursor=" + url.QueryEscape(text)
			}
		default:
			return out
		}
	}
	e.t.Fatalf("%s has more than %d pages", op, maxPages)
	return nil
}

// withoutInstance returns the body of r without the member instance of a problem: instance repeats
// the request path, so a request on an ID of Engineering finds it there, which is no answer of
// the server about Engineering.
func withoutInstance(r harness.Response) []byte {
	if r.Status < 400 {
		return r.Body
	}
	var m map[string]any
	if json.Unmarshal(r.Body, &m) != nil {
		return r.Body
	}
	delete(m, "instance")
	b, _ := json.Marshal(m)
	return b
}

// anyContains reports whether one of the bodies holds one of list.
func anyContains(rs []harness.Response, list []string) bool {
	for _, r := range rs {
		if firstID(r.Body, list) != "" {
			return true
		}
	}
	return false
}

// readGolden returns the operation IDs of the golden file without its comment lines; a missing
// file fails with got, the observed set, so it can be reviewed and written once.
func readGolden(e *env, got []string) []string {
	e.t.Helper()
	b, err := os.ReadFile(scopeGolden)
	if err != nil {
		e.t.Fatalf("read %s: %v; the operations that called Scope are\n%s", scopeGolden, err, strings.Join(got, "\n"))
	}
	var out []string
	for line := range strings.Lines(string(b)) {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			out = append(out, line)
		}
	}
	return out
}
