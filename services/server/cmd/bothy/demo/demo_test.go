// SPDX-License-Identifier: Apache-2.0

package demo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/cmd/bothy/demo"
	"github.com/OWNER/bothy/services/server/internal/wire"
	"github.com/OWNER/bothy/services/server/test/harness"
)

func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }

// dataDir is the data set of the repository, seen from this package.
var dataDir = filepath.Join("..", "..", "..", "..", "..", "infra", "demo")

// loadDataSet loads the data set of the repository.
func loadDataSet(t *testing.T) *demo.DataSet {
	t.Helper()
	ds, err := demo.Load(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	return ds
}

// startEmpty starts a harness server whose setup has not run.
func startEmpty(t *testing.T) *harness.Server {
	t.Helper()
	return harness.Start(t, harness.Options{EmptyInstance: true, Config: harness.RaisedRateLimits()})
}

// bootstrapWith returns the Bootstrap of Options over the setup service of s, as bothy setup demo
// wires it.
func bootstrapWith(s *harness.Server) func(context.Context, demo.Person) (string, error) {
	return func(ctx context.Context, owner demo.Person) (string, error) {
		setup, err := s.NewSetup(ctx)
		if err != nil {
			return "", err
		}
		defer setup.Close(context.WithoutCancel(ctx))
		res, err := setup.BootstrapOwner(ctx, wire.BootstrapOwnerCmd{Handle: owner.Handle, DisplayName: owner.DisplayName, Locale: owner.Locale, TimeZone: owner.TimeZone})
		if err != nil {
			if errors.Is(err, wire.ErrSetupCompleted) {
				return "", demo.ErrNotEmpty
			}
			return "", err
		}
		return res.Code.Code, nil
	}
}

// runDemo runs the demo on s with seed and no exchange directory and returns the result and the
// enrollment code it printed.
func runDemo(t *testing.T, s *harness.Server, ds *demo.DataSet, seed uint64) (*demo.Result, string) {
	t.Helper()
	var out bytes.Buffer
	res, err := demo.Run(t.Context(), demo.Options{DataSet: ds, Seed: seed, APIURL: s.ListenURL, PublicURL: s.PublicURL, Pace: 0,
		Bootstrap: bootstrapWith(s), Stdout: &out})
	if err != nil {
		t.Fatalf("demo.Run: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
	if len(lines) != 1 || lines[0] == "" {
		t.Fatalf("stdout has %d lines, want exactly the code", len(lines))
	}
	return res, lines[0]
}

// signIn redeems code on a new device of s and returns its client.
func signIn(t *testing.T, s *harness.Server, code string) *harness.Client {
	t.Helper()
	c := s.NewDevice()
	resp := c.DoAnonymous(t.Context(), harness.Request{Method: http.MethodPost, Path: "/auth/enroll/redeem",
		Body: map[string]any{"enrollment_code": code, "device": c.Registration("Export device")}})
	if resp.Status != http.StatusCreated {
		t.Fatalf("redeemEnrollmentCode: %d %s", resp.Status, resp.Body)
	}
	if err := c.Adopt(resp.Body); err != nil {
		t.Fatal(err)
	}
	return c
}

// TestDemoDeterministic (brief M7-27 requirement 14): the same seed on two empty instances gives
// the same content and structure; the export through the public API is compared without IDs and
// times. A different seed gives a different warm-up.
func TestDemoDeterministic(t *testing.T) {
	t.Parallel()
	ds := loadDataSet(t)
	exports := make([]string, 3)
	for i, seed := range []uint64{7, 7, 8} {
		s := startEmpty(t)
		res, code := runDemo(t, s, ds, seed)
		exports[i] = export(t, signIn(t, s, code), res)
	}
	if exports[0] != exports[1] {
		t.Fatalf("same seed, different export:\n%s", firstDifference(exports[0], exports[1]))
	}
	if exports[0] == exports[2] {
		t.Fatal("seeds 7 and 8 gave the same export")
	}
	for _, want := range []string{`"Alex"`, `"Support"`, `"Engineering"`, `"Triage Bot"`, `"Password reset mail takes a long time"`} {
		if !strings.Contains(exports[0], want) {
			t.Errorf("the export misses %s", want)
		}
	}
}

// TestDemoSecondRunRefused: a second run on the same instance ends with ErrNotEmpty and writes
// nothing: the audit chain gains no entry of its own. The only entries it may gain meanwhile are
// the aggregated denials of the first run (its step-up answers), which the server writes when
// audit.deny_aggregate_window ends; such a row names the time of its last denial.
func TestDemoSecondRunRefused(t *testing.T) {
	t.Parallel()
	ds := loadDataSet(t)
	s := startEmpty(t)
	runDemo(t, s, ds, 1)
	last := auditLastSeq(t, s)
	start := time.Now()
	var out bytes.Buffer
	_, err := demo.Run(t.Context(), demo.Options{DataSet: ds, Seed: 1, APIURL: s.ListenURL, PublicURL: s.PublicURL, Pace: 0,
		Bootstrap: bootstrapWith(s), Stdout: &out})
	if !errors.Is(err, demo.ErrNotEmpty) || out.Len() != 0 {
		t.Fatalf("second run: %v, stdout %d bytes", err, out.Len())
	}
	if got := auditWrittenSince(t, s, last, start); len(got) != 0 {
		t.Fatalf("the second run wrote audit events: %v", got)
	}
}

// auditLastSeq returns the seq of the newest audit event of s, 0 without one.
func auditLastSeq(t *testing.T, s *harness.Server) int64 {
	t.Helper()
	var seq int64
	if err := s.AdminDB().QueryRow(t.Context(), `SELECT coalesce(max(seq), 0) FROM audit_events`).Scan(&seq); err != nil {
		t.Fatal(err)
	}
	return seq
}

// auditWrittenSince returns the actions of the audit events of s after seq last, without the
// aggregated denials whose last denial came before start: those belong to what ran before start
// and reach the table only when their aggregation window ends. A bucket that a denial after start
// joined counts.
func auditWrittenSince(t *testing.T, s *harness.Server, last int64, start time.Time) []string {
	t.Helper()
	rows, err := s.AdminDB().Query(t.Context(), `SELECT action || ' ' || decision FROM audit_events
		WHERE seq > $1 AND NOT (decision = 'deny' AND details ? 'aggregate'
		AND (details->'aggregate'->>'last_at')::timestamptz < $2) ORDER BY seq`, last, start)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			t.Fatal(err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// TestDemoAudited (brief M7-27 requirement 13): every record of the demo came through authorize and
// the audit: the audit chain names the owner actions of the demo, each by a person.
func TestDemoAudited(t *testing.T) {
	t.Parallel()
	ds := loadDataSet(t)
	s := startEmpty(t)
	runDemo(t, s, ds, 1)
	rows, err := s.AdminDB().Query(t.Context(), `SELECT action, count(*) FROM audit_events GROUP BY action`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[string]int{}
	for rows.Next() {
		var a string
		var n int
		if err := rows.Scan(&a, &n); err != nil {
			t.Fatal(err)
		}
		got[a] = n
	}
	people := len(ds.People) - 1
	grants := 0
	for _, sp := range ds.Spaces {
		grants += len(sp.Members)
	}
	grants += len(ds.Host.SessionUsers)
	for action, want := range map[string]int{
		"instance.bootstrap": 1, "user.create": people, "agent.create": len(ds.Agents), "grant.create": grants,
		"price.manage": len(ds.Prices), "host.manage": 1,
	} {
		if got[action] < want {
			t.Errorf("audit action %s: %d entries, want at least %d (all: %v)", action, got[action], want, got)
		}
	}
}

// export reads the instance through the public API as c (the owner) and returns it as JSON
// without IDs and times: every known ID is replaced by the name of its record.
func export(t *testing.T, c *harness.Client, res *demo.Result) string {
	t.Helper()
	names := map[string]string{res.HostID: "host"}
	for _, m := range []map[string]string{res.People, res.Agents, res.Spaces, res.Channels} {
		for name, id := range m {
			names[id] = name
		}
	}
	doc := map[string]any{
		"users": list(t, c, "/users"), "agents": list(t, c, "/agents"), "prices": list(t, c, "/prices"),
		"grants": list(t, c, "/grants"), "hosts": list(t, c, "/hosts"), "spaces": list(t, c, "/spaces"),
	}
	channels := map[string]any{}
	for _, name := range slices.Sorted(maps.Keys(res.Channels)) {
		id := res.Channels[name]
		var threads []any
		for _, th := range list(t, c, "/channels/"+id+"/threads") {
			tid := th.(map[string]any)["id"].(string)
			threads = append(threads, map[string]any{"thread": th, "messages": list(t, c, "/threads/"+tid+"/messages")})
		}
		var ch map[string]any
		harness.MustDo(t, c, harness.Request{Method: http.MethodGet, Path: "/channels/" + id}, http.StatusOK).JSON(t, &ch)
		if main, ok := ch["main_thread_id"].(string); ok {
			threads = append(threads, map[string]any{"main": list(t, c, "/threads/"+main+"/messages")})
		}
		channels[name] = map[string]any{"channel": ch, "threads": threads}
	}
	doc["channels"] = channels
	b, err := json.MarshalIndent(normalize(doc, names), "", " ")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// list reads every page of the list at path.
func list(t *testing.T, c *harness.Client, path string) []any {
	t.Helper()
	var all []any
	cursor := ""
	for {
		q := url.Values{"limit": {"100"}}
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		var page struct {
			Items      []any `json:"items"`
			NextCursor any   `json:"next_cursor"`
		}
		harness.MustDo(t, c, harness.Request{Method: http.MethodGet, Path: path + "?" + q.Encode()}, http.StatusOK).JSON(t, &page)
		all = append(all, page.Items...)
		next, ok := page.NextCursor.(string)
		if !ok || next == "" {
			return all
		}
		cursor = next
	}
}

// uuidText matches an ID in a text.
var uuidText = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

// volatile reports whether a member holds a time, a version, an order key or a cursor: the parts
// the comparison ignores (16 16.4).
func volatile(key string) bool {
	return strings.HasSuffix(key, "_at") || key == "version" || key == "settings_version" || key == "sort_key" || key == "etag" ||
		strings.HasSuffix(key, "_seq") || key == "seq" || key == "cursor" || key == "expires_in" || key == "last_activity"
}

// normalize drops the volatile members and replaces every ID by the name of its record or "id".
func normalize(v any, names map[string]string) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, val := range x {
			if volatile(k) {
				continue
			}
			out[k] = normalize(val, names)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = normalize(val, names)
		}
		return out
	case string:
		return uuidText.ReplaceAllStringFunc(x, func(id string) string {
			if n, ok := names[id]; ok {
				return "<" + n + ">"
			}
			return "<id>"
		})
	default:
		return v
	}
}

// firstDifference shows the lines around the first line where a and b differ.
func firstDifference(a, b string) string {
	al, bl := strings.Split(a, "\n"), strings.Split(b, "\n")
	for i := range min(len(al), len(bl)) {
		if al[i] != bl[i] {
			lo := max(0, i-5)
			return "first:\n" + strings.Join(al[lo:min(len(al), i+5)], "\n") + "\nsecond:\n" + strings.Join(bl[lo:min(len(bl), i+5)], "\n")
		}
	}
	return "one export is longer"
}
