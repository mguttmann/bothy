// SPDX-License-Identifier: Apache-2.0

package reconcile_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"
)

// fixtureDay is the day of the first page of the fixtures in plugins/cost-reconciler/tests/fixtures;
// the fake moves every bucket by the distance from it to the day of the test's runs.
var fixtureDay = time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)

// recorded is one request with its answer as a fake or the proxy saw it.
type recorded struct {
	Method, Path, Query string
	Header              http.Header
	Body                []byte
	Status              int
	RespHeader          http.Header
	RespBody            []byte
}

// recorder keeps requests goroutine-safe.
type recorder struct {
	mu   sync.Mutex
	list []recorded
}

// add appends one record.
func (r *recorder) add(x recorded) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.list = append(r.list, x)
}

// all returns a copy of the records.
func (r *recorder) all() []recorded {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.list)
}

// costAPI is the fake of the provider's cost endpoint: GET /v1/organizations/cost_report serves the
// fixture pages (without the query parameter page the first, with page=page_2 the second), with
// every bucket moved to the days of the test, and records every request.
type costAPI struct {
	srv   *httptest.Server
	got   recorder
	pages map[string][]byte
}

// newCostAPI starts the fake on 127.0.0.1 port 0 with the fixture pages moved by shift.
func newCostAPI(t *testing.T, shift time.Duration) *costAPI {
	t.Helper()
	dir := filepath.Join(repoRoot(t), "plugins", "cost-reconciler", "tests", "fixtures")
	c := &costAPI{pages: map[string][]byte{}}
	for key, file := range map[string]string{"": "cost_report_page1.json", "page_2": "cost_report_page2.json"} {
		raw, err := os.ReadFile(filepath.Join(dir, file))
		if err != nil {
			t.Fatal(err)
		}
		c.pages[key] = movedPage(t, raw, shift)
	}
	c.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		c.got.add(recorded{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Header: r.Header.Clone(), Body: body})
		page, ok := c.pages[r.URL.Query().Get("page")]
		if r.Method != http.MethodGet || r.URL.Path != "/v1/organizations/cost_report" || !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(page)
	}))
	t.Cleanup(c.srv.Close)
	return c
}

// movedPage moves starting_at and ending_at of every bucket of a page by shift.
func movedPage(t *testing.T, raw []byte, shift time.Duration) []byte {
	t.Helper()
	var page map[string]any
	if err := json.Unmarshal(raw, &page); err != nil {
		t.Fatal(err)
	}
	for _, b := range page["data"].([]any) {
		bucket := b.(map[string]any)
		for _, k := range []string{"starting_at", "ending_at"} {
			at, err := time.Parse(time.RFC3339, bucket[k].(string))
			if err != nil {
				t.Fatal(err)
			}
			bucket[k] = at.Add(shift).UTC().Format(time.RFC3339)
		}
	}
	out, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// recordingProxy is a reverse proxy on 127.0.0.1 port 0 in front of the server: the app talks to
// the server only through it, so it records every request and response of the app's traffic.
type recordingProxy struct {
	srv *httptest.Server
	got recorder
}

// newRecordingProxy starts the proxy to target.
func newRecordingProxy(t *testing.T, target string) *recordingProxy {
	t.Helper()
	u, err := url.Parse(target)
	if err != nil {
		t.Fatal(err)
	}
	p := &recordingProxy{}
	rp := &httputil.ReverseProxy{Rewrite: func(r *httputil.ProxyRequest) { r.SetURL(u) }}
	p.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		x := recorded{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Header: r.Header.Clone(), Body: body}
		r.Body = io.NopCloser(bytes.NewReader(body))
		rec := httptest.NewRecorder()
		rp.ServeHTTP(rec, r)
		x.Status, x.RespHeader, x.RespBody = rec.Code, rec.Header().Clone(), rec.Body.Bytes()
		p.got.add(x)
		for k, v := range rec.Header() {
			w.Header()[k] = v
		}
		w.WriteHeader(rec.Code)
		_, _ = w.Write(rec.Body.Bytes())
	}))
	t.Cleanup(p.srv.Close)
	return p
}
