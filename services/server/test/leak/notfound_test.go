// SPDX-License-Identifier: Apache-2.0

package leak

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"text/tabwriter"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// The timing samples of AZ-P4 design note 5: warm-up pairs, then measured pairs (A then B, or B
// then A, alternating) per route and requester.
const (
	timingWarmup  = 3
	timingSamples = 21
)

// contentParams are the path parameters of content resources: without read the answer is 404
// not_found (R-AZ-01, 06 "Authorization in the API" item 3).
var contentParams = []string{"space_id", "channel_id", "thread_id", "message_id", "agent_id"}

// requesterNames are the requesters of AZ-P4: a person with DPoP and an agent with an API token.
var requesterNames = []string{"Alex", "Lotse"}

// p4World is a harness server with raised rate limits (23 R-QA-11), the matrix world and the
// extras, plus the routes of SelectRoutes.
type p4World struct {
	w          *harness.World
	routes     []Route
	requesters map[string]*harness.Client
}

// newP4World starts the server of an AZ-P4 test.
func newP4World(t *testing.T) *p4World {
	t.Helper()
	routes, err := SelectRoutes()
	if err != nil {
		t.Fatal(err)
	}
	s := harness.Start(t, harness.Options{EgressDenyAll: true, Config: harness.RaisedRateLimits()})
	w := harness.BuildMatrixWorld(t, s)
	PrepareExtras(t, w)
	return &p4World{w: w, routes: routes, requesters: map[string]*harness.Client{"Alex": w.People["Alex"], "Lotse": w.Agents["Lotse"]}}
}

// pair is request A (foreign IDs) and request B (fresh random UUIDv7 values) of one route and
// requester, or the reason why the route has no foreign instance for the requester.
type pair struct {
	a, b       string
	foreign    []string
	random     []string
	content    bool
	notApplied string
}

// pairFor builds the pair of route r and requester c; a missing fixture or a fixture without
// reason fails t.
func (p *p4World) pairFor(t *testing.T, r Route, c *harness.Client) pair {
	t.Helper()
	a, b := map[string]string{}, map[string]string{}
	out := pair{}
	for _, name := range r.Params {
		f, ok := Fixtures[name]
		if !ok {
			t.Fatalf("%s: no fixture for path parameter %s", r.OperationID, name)
		}
		id, ok, reason := f(p.w, c)
		if !ok {
			if strings.TrimSpace(reason) == "" {
				t.Fatalf("%s: fixture %s returned ok=false without a reason", r.OperationID, name)
			}
			out.notApplied = name + ": " + reason
			return out
		}
		a[name], b[name] = id, ids.New().String()
		out.foreign, out.random = append(out.foreign, id), append(out.random, b[name])
		out.content = out.content || slices.Contains(contentParams, name)
	}
	out.a, out.b = r.Build(a), r.Build(b)
	return out
}

// get sends GET path as c.
func get(t *testing.T, c *harness.Client, path string) harness.Response {
	t.Helper()
	r := c.Do(t.Context(), harness.Request{Method: http.MethodGet, Path: path})
	if r.Status == 0 {
		t.Fatalf("GET %s: %s", path, r.Body)
	}
	return r
}

// assertEqualAnswers holds AZ-P4 design note 3 for the answers ra (foreign) and rb (unknown).
func assertEqualAnswers(t *testing.T, p pair, ra, rb harness.Response) {
	t.Helper()
	na, nb := Normalize(ra, p.foreign), Normalize(rb, p.random)
	if d := Diff(na, nb); len(d) != 0 {
		t.Errorf("foreign and unknown ID differ:\n  %s\n  A %d %s\n  B %d %s", strings.Join(d, "\n  "), ra.Status, ra.Body, rb.Status, rb.Body)
	}
	if p.content && (ra.Status != http.StatusNotFound || problemCode(ra.Body) != "not_found") {
		t.Errorf("no read on a content resource: %d %s, want 404 not_found", ra.Status, ra.Body)
	}
}

// problemCode returns the member code of a JSON body.
func problemCode(body []byte) string {
	n := Normalize(harness.Response{Body: body}, nil)
	if m, ok := n.Body.(map[string]any); ok {
		if c, ok := m["code"].(string); ok {
			return c
		}
	}
	return ""
}

// TestAZ_P4_RouteCoverage (AZ-P4): every selected route has a fixture for each UUID path
// parameter, every fixture with ok=false has a reason, and the table of routes, requesters and
// outcome is printed.
func TestAZ_P4_RouteCoverage(t *testing.T) {
	t.Parallel()
	p := newP4World(t)
	if len(p.routes) == 0 {
		t.Fatal("no enabled GET route with a uuid path parameter")
	}
	var b strings.Builder
	tw := tabwriter.NewWriter(&b, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "operationId\trequester\toutcome")
	compared := 0
	for _, r := range p.routes {
		for _, name := range requesterNames {
			pr := p.pairFor(t, r, p.requesters[name])
			outcome := "foreign vs unknown: " + strings.Join(r.Params, ", ")
			if pr.notApplied != "" {
				outcome = "not applicable (" + pr.notApplied + ")"
			} else {
				compared++
			}
			_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\n", r.OperationID, name, outcome)
		}
	}
	_ = tw.Flush()
	t.Logf("AZ-P4 coverage: %d routes, %d compared pairs\n%s", len(p.routes), compared, b.String())
	if compared == 0 {
		t.Fatal("no route compared")
	}
}

// TestAZ_P4_NotFoundEquality (AZ-P4): for every selected route and both requesters, a foreign ID
// and a fresh random ID give the same normalized status, headers, body and length, and 404
// not_found where the requester lacks read on a content resource.
func TestAZ_P4_NotFoundEquality(t *testing.T) {
	t.Parallel()
	p := newP4World(t)
	for _, r := range p.routes {
		for _, name := range requesterNames {
			c := p.requesters[name]
			t.Run("AZ-P4/"+r.OperationID+"/"+name, func(t *testing.T) {
				pr := p.pairFor(t, r, c)
				if pr.notApplied != "" {
					t.Skip("not applicable: " + pr.notApplied)
				}
				assertEqualAnswers(t, pr, get(t, c, pr.a), get(t, c, pr.b))
			})
		}
	}
}

// TestAZ_P4_TimingMedian (AZ-P4): for every selected route and both requesters, after a warm-up,
// interleaved samples of the foreign and the unknown ID have medians of Elapsed that differ by
// at most authz.not_found_timing_tolerance.
func TestAZ_P4_TimingMedian(t *testing.T) {
	t.Parallel()
	tolerance := notFoundTimingTolerance(t)
	p := newP4World(t)
	for _, r := range p.routes {
		for _, name := range requesterNames {
			c := p.requesters[name]
			t.Run("AZ-P4/"+r.OperationID+"/"+name, func(t *testing.T) {
				pr := p.pairFor(t, r, c)
				if pr.notApplied != "" {
					t.Skip("not applicable: " + pr.notApplied)
				}
				for range timingWarmup {
					get(t, c, pr.a)
					get(t, c, pr.b)
				}
				var as, bs []time.Duration
				for i := range timingSamples {
					if i%2 == 0 {
						as, bs = append(as, get(t, c, pr.a).Elapsed), append(bs, get(t, c, pr.b).Elapsed)
					} else {
						bs, as = append(bs, get(t, c, pr.b).Elapsed), append(as, get(t, c, pr.a).Elapsed)
					}
				}
				ma, mb := median(as), median(bs)
				if d := (ma - mb).Abs(); d > tolerance {
					t.Errorf("medians foreign %v, unknown %v differ by %v > %v", ma, mb, d, tolerance)
				}
			})
		}
	}
}

// TestAZ_M52_ForeignMessageEqualsUnknown (AZ-M52): Alex reading a message of
// Marketing/Kampagnen by ID gets the same answer as for a random UUIDv7, 404 not_found.
func TestAZ_M52_ForeignMessageEqualsUnknown(t *testing.T) {
	t.Parallel()
	s := harness.Start(t, harness.Options{EgressDenyAll: true, Config: harness.RaisedRateLimits()})
	w := harness.BuildMatrixWorld(t, s)
	alex := w.People["Alex"]
	foreign, unknown := w.Messages["Marketing/Kampagnen"][0], ids.New().String()
	// The message exists: the owner reads it.
	harness.MustDo(t, w.Owner, harness.Request{Method: http.MethodGet, Path: "/messages/" + foreign}, http.StatusOK)
	p := pair{foreign: []string{foreign}, random: []string{unknown}, content: true}
	assertEqualAnswers(t, p, get(t, alex, "/messages/"+foreign), get(t, alex, "/messages/"+unknown))
}

// median returns the median of d.
func median(d []time.Duration) time.Duration {
	s := slices.Clone(d)
	slices.Sort(s)
	if len(s)%2 == 1 {
		return s[len(s)/2]
	}
	return (s[len(s)/2-1] + s[len(s)/2]) / 2
}

// toleranceKey is the configuration key of the allowed median difference of AZ-P4.
const toleranceKey = "authz.not_found_timing_tolerance"

// notFoundTimingTolerance reads the standard of authz.not_found_timing_tolerance from the
// configuration reference docs/operations/configuration.md (the key is planned there, the server
// does not read it), so the test never carries the value as a literal.
func notFoundTimingTolerance(t *testing.T) time.Duration {
	t.Helper()
	d, err := toleranceFromReference(filepath.Join(moduleDir(), "..", "..", "docs", "operations", "configuration.md"))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// toleranceFromReference returns the standard column of the row of toleranceKey in the table of
// the configuration reference at path (key, environment, place, type, standard, ...).
func toleranceFromReference(path string) (time.Duration, error) {
	f, err := os.Open(path) // #nosec G304 -- the reference of this repository
	if err != nil {
		return 0, err
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		cells := strings.Split(sc.Text(), "|")
		if len(cells) < 7 || strings.TrimSpace(cells[1]) != "`"+toleranceKey+"`" {
			continue
		}
		return time.ParseDuration(strings.Trim(strings.TrimSpace(cells[5]), "`"))
	}
	if err := sc.Err(); err != nil {
		return 0, err
	}
	return 0, fmt.Errorf("%s: no row for %s", path, toleranceKey)
}

// TestToleranceFromReference: the standard of the key is read from the reference table.
func TestToleranceFromReference(t *testing.T) {
	d := notFoundTimingTolerance(t)
	if d <= 0 {
		t.Fatalf("tolerance %v", d)
	}
	dir := t.TempDir()
	bad := filepath.Join(dir, "configuration.md")
	if err := os.WriteFile(bad, []byte("| `other.key` | X | Server | duration | `1s` | x | aktiv |\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := toleranceFromReference(bad); err == nil {
		t.Fatal("a reference without the key gave a value")
	}
	if _, err := toleranceFromReference(filepath.Join(dir, "missing.md")); err == nil {
		t.Fatal("a missing reference gave a value")
	}
}
