// SPDX-License-Identifier: Apache-2.0

package httpserver_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"math/rand"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"testing/quick"

	"go.opentelemetry.io/otel/attribute"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// --- no null, 2^53 (ADR 0053) ---

// randomValue builds a random JSON-shaped Go value: recursive maps, slices, pointers, nil slices,
// nil pointers and scalars.
func randomValue(r *rand.Rand, depth int) any {
	n := r.Intn(9)
	if depth > 3 {
		n = r.Intn(4)
	}
	switch n {
	case 0:
		return nil
	case 1:
		return r.Intn(1<<20) - 1<<19
	case 2:
		return "s" + strings.Repeat("x", r.Intn(3)) + "<&>"
	case 3:
		return r.Intn(2) == 0
	case 4:
		var nilSlice []any
		return nilSlice
	case 5:
		v := randomValue(r, depth+1)
		return &v
	case 6:
		var p *int
		return p
	case 7:
		s := make([]any, r.Intn(3))
		for i := range s {
			s[i] = nonNull(r, depth+1)
		}
		return s
	}
	m := map[string]any{}
	for i := range r.Intn(4) {
		m["k"+string(rune('a'+i))] = randomValue(r, depth+1)
	}
	return m
}

// nonNull is a random value that does not encode to null (array elements must not be null).
func nonNull(r *rand.Rand, depth int) any {
	for {
		v := randomValue(r, depth)
		if b, _ := json.Marshal(v); string(b) != "null" {
			return v
		}
	}
}

// dropNull is the expected result: the decoded value without its null members.
func dropNull(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, x := range t {
			if x != nil {
				out[k] = dropNull(x)
			}
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, x := range t {
			out[i] = dropNull(x)
		}
		return out
	}
	return v
}

// TestNoNull_PropertyRandomValues: for random values that are objects at the top, SanitizeJSON
// returns a body without a null token that equals the input without its null members.
func TestNoNull_PropertyRandomValues(t *testing.T) {
	t.Parallel()
	prop := func(seed int64) bool {
		r := rand.New(rand.NewSource(seed)) //nolint:gosec // test data, not security
		in := map[string]any{"root": randomValue(r, 0), "other": randomValue(r, 0)}
		body, err := json.Marshal(in)
		if err != nil {
			return false
		}
		out, _, err := httpserver.SanitizeJSON(body)
		if err != nil {
			t.Logf("body %s: %v", body, err)
			return false
		}
		if testkit.HasJSONNull(out) {
			t.Logf("null in %s", out)
			return false
		}
		var got, want any
		_ = json.Unmarshal(out, &got)
		_ = json.Unmarshal(body, &want)
		return reflect.DeepEqual(got, dropNull(want))
	}
	if err := quick.Check(prop, &quick.Config{MaxCount: 500}); err != nil {
		t.Fatal(err)
	}
}

func FuzzSanitizeJSON(f *testing.F) {
	for _, s := range []string{`{"a":null}`, `[null]`, `null`, `{"a":[1,{"b":null}]}`, `9007199254740992`, `{"x":"\u003c"}`, `{}`, `1.5e300`, `"s"`} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		out, _, err := httpserver.SanitizeJSON(data)
		if err != nil {
			return
		}
		if decodeNumbers(out) != nil {
			t.Fatalf("output %q of %q is not JSON", out, data)
		}
		again, removed, err := httpserver.SanitizeJSON(out)
		if err != nil || removed != 0 || !bytes.Equal(again, out) {
			t.Fatalf("output %q is not stable: %q %d %v", out, again, removed, err)
		}
	})
}

// jsonHarness serves getThing with a handler that writes body as JSON with status 200.
func jsonHarness(t *testing.T, body string, allowRepair bool) *httpservertest.Harness {
	t.Helper()
	op := getThing()
	return httpservertest.New(t, httpservertest.Options{AllowNullRepair: allowRepair, Routes: []httpservertest.Route{{Op: op,
		Handler: httpservertest.Authorized(op.Action, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("ETag", `"3"`)
			_, _ = w.Write([]byte(body))
		})}}})
}

func getOne(h *httpservertest.Harness) *httpservertest.Response {
	return h.Do(h.NewRequest(http.MethodGet, "/api/v1/things/"+thingID, "", "X-Mode", "fast"))
}

func TestNoNull_NullMemberRemovedAndLogged(t *testing.T) {
	t.Parallel()
	h := jsonHarness(t, `{"a":1,"b":null,"c":{"d":null}}`, true)
	resp := getOne(h)
	if resp.StatusCode != 200 || string(resp.Body) != `{"a":1,"c":{}}` {
		t.Fatalf("%d %s", resp.StatusCode, resp.Body)
	}
	rec, ok := h.Logs.Find(httpserver.LogNullRepaired)
	if !ok || rec.Attrs["http.route"] != thingPath {
		t.Fatalf("log %+v", rec)
	}
}

func TestNoNull_NullInArrayGives500(t *testing.T) {
	t.Parallel()
	for _, body := range []string{`{"a":[1,null]}`, `null`} {
		h := jsonHarness(t, body, false)
		resp := getOne(h)
		if resp.StatusCode != 500 || resp.Problem(t)["code"] != "internal_error" || resp.Header.Get("ETag") != "" {
			t.Fatalf("%s: %d %v %s", body, resp.StatusCode, resp.Header, resp.Body)
		}
		if !h.Logs.Has(httpserver.LogResponseRefused) {
			t.Fatalf("%s: no log", body)
		}
	}
}

func TestInt53_AboveLimitGives500(t *testing.T) {
	t.Parallel()
	for body, status := range map[string]int{
		`{"seq":9007199254740991}`:   200,
		`{"seq":-9007199254740991}`:  200,
		`{"seq":9007199254740992}`:   500,
		`{"seq":-9007199254740992}`:  500,
		`[123456789012345678901234]`: 500,
		`{"f":1.5e300}`:              200,
	} {
		if resp := getOne(jsonHarness(t, body, false)); resp.StatusCode != status {
			t.Errorf("%s: %d, want %d", body, resp.StatusCode, status)
		}
	}
}

func TestNoNull_CleanBodyByteIdentical(t *testing.T) {
	t.Parallel()
	const body = "{ \"b\" : [1, 2.50, \"\\u003c&\"] ,\n \"a\":{}}\n"
	resp := getOne(jsonHarness(t, body, false))
	if resp.StatusCode != 200 || string(resp.Body) != body || resp.Header.Get("ETag") != `"3"` {
		t.Fatalf("%d %q", resp.StatusCode, resp.Body)
	}
}

func TestNoNull_ProblemBodiesClean(t *testing.T) {
	t.Parallel()
	h := jsonHarness(t, `{}`, false)
	resp := h.Do(h.NewRequest(http.MethodGet, "/api/v1/things/"+thingID, "")) // header missing: 400
	if resp.StatusCode != 400 || testkit.HasJSONNull(resp.Body) {
		t.Fatalf("%d %s", resp.StatusCode, resp.Body)
	}
}

// --- proof of the authorize check (02 9.3) ---

func authzErrors(h *httpservertest.Harness) int64 {
	return h.Counter("bothy.authz.errors", attribute.String("bothy.reason", "authz_not_checked"))
}

func TestProof_MissingCheckGives500AuthzNotChecked(t *testing.T) {
	t.Parallel()
	op := getThing()
	h := httpservertest.New(t, httpservertest.Options{Routes: []httpservertest.Route{{Op: op,
		Handler: func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("ETag", `"1"`)
			httpservertest.JSON(w, http.StatusOK, map[string]any{"secret": "content"})
		}}}})
	before := authzErrors(h)
	resp := getOne(h)
	p := resp.Problem(t)
	if resp.StatusCode != 500 || p["code"] != "internal_error" || bytes.Contains(resp.Body, []byte("secret")) ||
		bytes.Contains(resp.Body, []byte("authz_not_checked")) || resp.Header.Get("ETag") != "" {
		t.Fatalf("%d %v %s", resp.StatusCode, resp.Header, resp.Body)
	}
	if got := authzErrors(h) - before; got != 1 {
		t.Fatalf("bothy.authz.errors grew by %d, want 1", got)
	}
	rec, ok := h.Logs.Find(httpserver.LogAuthzNotChecked)
	if !ok || rec.Attrs["bothy.action"] != "thing.read" || rec.Attrs["http.route"] != thingPath || rec.Attrs["bothy.reason"] != "authz_not_checked" {
		t.Fatalf("log %+v", rec)
	}
	// An empty response is an implicit 200 and needs the proof too.
	h2 := httpservertest.New(t, httpservertest.Options{Routes: []httpservertest.Route{{Op: op, Handler: func(http.ResponseWriter, *http.Request) {}}}})
	if resp := getOne(h2); resp.StatusCode != 500 {
		t.Fatalf("empty response: %d", resp.StatusCode)
	}
}

func TestProof_OtherActionGives500(t *testing.T) {
	t.Parallel()
	op := getThing()
	h := httpservertest.New(t, httpservertest.Options{Routes: []httpservertest.Route{{Op: op, Handler: okHandler("channel.read")}}})
	if resp := getOne(h); resp.StatusCode != 500 || authzErrors(h) != 1 {
		t.Fatalf("%d, counter %d", resp.StatusCode, authzErrors(h))
	}
}

func TestProof_ScopeCounts(t *testing.T) {
	t.Parallel()
	op := getThing()
	h := httpservertest.New(t, httpservertest.Options{Routes: []httpservertest.Route{{Op: op,
		Handler: func(w http.ResponseWriter, r *http.Request) {
			authorize.RecordCheck(r.Context(), authorize.Check{Action: op.Action, Kind: authorize.CheckScope, Allowed: true})
			w.WriteHeader(http.StatusNoContent)
		}}}})
	if resp := getOne(h); resp.StatusCode != 204 || authzErrors(h) != 0 {
		t.Fatalf("%d, counter %d", resp.StatusCode, authzErrors(h))
	}
}

func TestProof_ErrorsPassThrough(t *testing.T) {
	t.Parallel()
	op := getThing()
	h := httpservertest.New(t, httpservertest.Options{Routes: []httpservertest.Route{{Op: op,
		Handler: func(w http.ResponseWriter, r *http.Request) {
			httpserver.WriteProblem(w, r, errs.New(errs.CodeNotFound, ""))
		}}}})
	if resp := getOne(h); resp.StatusCode != 404 || authzErrors(h) != 0 {
		t.Fatalf("%d, counter %d", resp.StatusCode, authzErrors(h))
	}
}

func TestProof_ConcurrentRecords(t *testing.T) {
	t.Parallel()
	op := getThing()
	h := httpservertest.New(t, httpservertest.Options{Routes: []httpservertest.Route{{Op: op,
		Handler: func(w http.ResponseWriter, r *http.Request) {
			var wg sync.WaitGroup
			for i := range 16 {
				wg.Go(func() {
					action := "other.read"
					if i == 15 {
						action = op.Action
					}
					authorize.RecordCheck(r.Context(), authorize.Check{Action: action, Kind: authorize.CheckAuthorize, Allowed: true})
				})
			}
			wg.Wait()
			w.WriteHeader(http.StatusNoContent)
		}}}})
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if resp := getOne(h); resp.StatusCode != 204 {
				t.Errorf("status %d", resp.StatusCode)
			}
		})
	}
	wg.Wait()
	if authzErrors(h) != 0 {
		t.Fatalf("counter %d", authzErrors(h))
	}
}

func TestProof_HijackNeedsCheck(t *testing.T) {
	t.Parallel()
	op := getThing()
	var hijackErr error
	h := httpservertest.New(t, httpservertest.Options{Routes: []httpservertest.Route{{Op: op,
		Handler: func(w http.ResponseWriter, _ *http.Request) {
			_, _, hijackErr = http.NewResponseController(w).Hijack()
		}}}})
	if resp := getOne(h); resp.StatusCode != 500 || hijackErr == nil || errors.Is(hijackErr, http.ErrNotSupported) {
		t.Fatalf("%d, hijack error %v", resp.StatusCode, hijackErr)
	}
}

// --- cursor codec (06 "Pagination" item 2) ---

type threadKey struct {
	CreatedAt string `json:"created_at"`
	ID        string `json:"id"`
}

func TestCursor_RoundTripAndRejections(t *testing.T) {
	t.Parallel()
	binding := []byte(thingID + "\x00state=open\x00sort=created_at")
	key := threadKey{CreatedAt: "2026-10-12T08:00:00.000Z", ID: thingID}
	c, err := httpserver.EncodeCursor("threads", binding, key)
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(c, "=+/") {
		t.Fatalf("cursor %q is not base64url without padding", c)
	}
	var got threadKey
	if err := httpserver.DecodeCursor("threads", binding, c, &got); err != nil || got != key {
		t.Fatalf("round trip: %+v %v", got, err)
	}
	otherPrincipal := []byte("0192f4a0-7b1c-7577-8bf0-000000000000\x00state=open\x00sort=created_at")
	invalid := map[string]func() error{
		"other list":      func() error { return httpserver.DecodeCursor("channels", binding, c, &got) },
		"other binding":   func() error { return httpserver.DecodeCursor("threads", []byte(thingID+"\x00state=closed"), c, &got) },
		"other principal": func() error { return httpserver.DecodeCursor("threads", otherPrincipal, c, &got) },
		"empty":           func() error { return httpserver.DecodeCursor("threads", binding, "", &got) },
		"not base64":      func() error { return httpserver.DecodeCursor("threads", binding, "!!!", &got) },
		"padded":          func() error { return httpserver.DecodeCursor("threads", binding, c+"=", &got) },
		"too long":        func() error { return httpserver.DecodeCursor("threads", binding, strings.Repeat("A", 4096), &got) },
		"wrong key type":  func() error { var n struct{ X int }; return httpserver.DecodeCursor("threads", binding, c, &n) },
	}
	for name, f := range invalid {
		if err := f(); errs.KindOf(err) != errs.KindBadRequest || err.(*errs.Error).PublicCode() != errs.CodeCursorInvalid { //nolint:errorlint // DecodeCursor returns *errs.Error directly
			t.Errorf("%s: %v", name, err)
		}
	}
	// Every changed byte of the decoded cursor is rejected.
	raw := mustDecode(t, c)
	for i := range raw {
		for _, delta := range []byte{1, 0x80} {
			tampered := append([]byte(nil), raw...)
			tampered[i] ^= delta
			if err := httpserver.DecodeCursor("threads", binding, encode(tampered), &got); errs.KindOf(err) != errs.KindBadRequest {
				t.Fatalf("byte %d xor %#x accepted", i, delta)
			}
		}
	}
	// A key string with NUL cannot reach PostgreSQL text (SQLSTATE 22021, a 500): a client that
	// forges a cursor with it gets cursor_invalid in every list (M2-39 red team).
	for _, forged := range []any{threadKey{CreatedAt: "a\x00b", ID: thingID}, map[string]any{"created_at": "x", "id": thingID, "a\x00": 1}} {
		nul, err := httpserver.EncodeCursor("threads", binding, forged)
		if err != nil {
			t.Fatal(err)
		}
		var k map[string]any
		if err := httpserver.DecodeCursor("threads", binding, nul, &k); errs.KindOf(err) != errs.KindBadRequest {
			t.Errorf("NUL in a cursor key: %v", err)
		}
	}
	if _, err := httpserver.EncodeCursor("", binding, key); err == nil {
		t.Error("empty list name accepted")
	}
	if _, err := httpserver.EncodeCursor("threads", binding, func() {}); err == nil {
		t.Error("unmarshalable key accepted")
	}
	if _, err := httpserver.EncodeCursor("threads", binding, strings.Repeat("x", 4096)); err == nil {
		t.Error("oversized key accepted")
	}
}
