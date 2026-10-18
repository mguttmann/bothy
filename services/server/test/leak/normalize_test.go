// SPDX-License-Identifier: Apache-2.0

package leak

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/OWNER/bothy/services/server/test/harness"
)

// problem returns a 404 response for id with a correlation ID, as the server writes it.
func problem(id, correlation string) harness.Response {
	body := `{"type":"/api/v1/problems/not_found","title":"Not found","status":404,"code":"not_found","correlation_id":"` +
		correlation + `","instance":"/api/v1/messages/` + id + `"}`
	h := http.Header{}
	h.Set("Content-Type", "application/problem+json")
	h.Set("Content-Length", strconv.Itoa(len(body)))
	h.Set("Correlation-Id", correlation)
	h.Set("Date", "Sat, 17 Oct 2026 21:04:37 GMT")
	h.Set("DPoP-Nonce", "nonce-"+correlation)
	h.Set("RateLimit", `"rl.read";r=99;t=1`)
	h.Set("RateLimit-Policy", `"rl.read";q=100;w=60`)
	return harness.Response{Status: http.StatusNotFound, Header: h, Body: []byte(body)}
}

const (
	foreignID = "01a14bae-50ed-762a-b4ef-a952643faf2e"
	unknownID = "01a14bae-51b9-7a2a-b450-7052a1b17427"
)

// TestNormalizeEqualProblems: two 404 answers that differ only in the requested ID, the
// correlation ID, Date, DPoP-Nonce and the remaining quota of RateLimit normalize to equal values.
func TestNormalizeEqualProblems(t *testing.T) {
	a := Normalize(problem(foreignID, "1e676456-89da-b61e-57f1-29013708e89e"), []string{foreignID})
	b := problem(unknownID, "4b93d984-c39f-0463-d472-f7fd3e2aac05")
	b.Header.Set("RateLimit", `"rl.read";r=98;t=2`)
	nb := Normalize(b, []string{unknownID})
	if d := Diff(a, nb); len(d) != 0 {
		t.Fatalf("diff %v", d)
	}
	for _, name := range []string{"Correlation-Id", "Date", "Dpop-Nonce"} {
		if a.Header.Get(name) != "" {
			t.Errorf("%s kept", name)
		}
	}
	if got := a.Header.Get("Ratelimit"); got != `"rl.read"` {
		t.Errorf("RateLimit %q", got)
	}
	if got := a.Header.Get("Ratelimit-Policy"); got != `"rl.read";q=100;w=60` {
		t.Errorf("RateLimit-Policy %q", got)
	}
	obj := a.Body.(map[string]any)
	if _, ok := obj["correlation_id"]; ok || obj["instance"] != "/api/v1/messages/"+Placeholder {
		t.Fatalf("body %v", obj)
	}
	if len(Placeholder) != len(foreignID) || a.Length != len(problem(foreignID, "1e676456-89da-b61e-57f1-29013708e89e").Body) {
		t.Fatalf("length %d", a.Length)
	}
}

// TestNormalizeKeepsDifferences: a different status, header, body member or length stays a
// difference after normalization.
func TestNormalizeKeepsDifferences(t *testing.T) {
	base := func() harness.Response { return problem(unknownID, "4b93d984-c39f-0463-d472-f7fd3e2aac05") }
	want := Normalize(base(), []string{unknownID})
	for name, change := range map[string]func(r *harness.Response){
		"status":   func(r *harness.Response) { r.Status = http.StatusForbidden },
		"header":   func(r *harness.Response) { r.Header.Set("ETag", `"3"`) },
		"policy":   func(r *harness.Response) { r.Header.Set("RateLimit", `"rl.admin";r=99;t=1`) },
		"body":     func(r *harness.Response) { r.Body = []byte(`{"code":"forbidden","correlation_id":"x"}`) },
		"instance": func(r *harness.Response) { r.Body = []byte(`{"instance":"/api/v1/threads/` + unknownID + `"}`) },
		"raw":      func(r *harness.Response) { r.Body = []byte("not json") },
	} {
		r := base()
		change(&r)
		if d := Diff(want, Normalize(r, []string{unknownID})); len(d) == 0 {
			t.Errorf("%s: no difference", name)
		}
	}
	// The same JSON with other white space differs only in its length.
	r := base()
	r.Body = append([]byte(" "), r.Body...)
	if d := Diff(want, Normalize(r, []string{unknownID})); len(d) != 1 {
		t.Errorf("white space: %v", d)
	}
}
