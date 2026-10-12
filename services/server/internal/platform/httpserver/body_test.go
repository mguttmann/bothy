// SPDX-License-Identifier: Apache-2.0

package httpserver_test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// createHarness serves createThing (POST /api/v1/things) and echoes the body it received.
func createHarness(t *testing.T, cfg *httpserver.ChainConfig) (*httpservertest.Harness, *string) {
	t.Helper()
	op := httpservertest.Op("createThing", http.MethodPost, "/api/v1/things", "thing.create")
	var got string
	h := httpservertest.New(t, httpservertest.Options{Config: cfg, Routes: []httpservertest.Route{{Op: op,
		Handler: httpservertest.Authorized(op.Action, func(w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body)
			got = string(b)
			w.WriteHeader(http.StatusCreated)
		})}}})
	return h, &got
}

func wantProblem(t *testing.T, resp *httpservertest.Response, status int, code string) map[string]any {
	t.Helper()
	p := resp.Problem(t)
	if resp.StatusCode != status || p["code"] != code {
		t.Fatalf("got %d %v, want %d %s", resp.StatusCode, p, status, code)
	}
	return p
}

func TestSECIN2_DuplicateKeysRejected(t *testing.T) {
	t.Parallel()
	h, got := createHarness(t, nil)
	for name, body := range map[string]string{
		"top level": `{"name":"ab","color":"red","name":"cd"}`,
		"nested":    `{"name":"ab","color":"red","nested":{"flag":true,"flag":false}}`,
		"escaped":   `{"name":"ab","color":"red","name":"cd"}`,
	} {
		p := wantProblem(t, h.Do(h.NewRequest(http.MethodPost, "/api/v1/things", body)), 400, "malformed_request")
		if strings.Contains(p["detail"].(string), "cd") {
			t.Errorf("%s: detail echoes content: %v", name, p["detail"])
		}
	}
	if *got != "" {
		t.Fatal("the handler ran")
	}
	h.Do(h.NewRequest(http.MethodPost, "/api/v1/things", `{"name":"ab","color":"red","tags":["a","a"]}`))
	if *got != `{"name":"ab","color":"red","tags":["a","a"]}` {
		t.Fatalf("equal array items are no duplicate members; handler got %q", *got)
	}
}

func TestSECIN2_InvalidUTF8Rejected(t *testing.T) {
	t.Parallel()
	h, _ := createHarness(t, nil)
	wantProblem(t, h.Do(h.NewRequest(http.MethodPost, "/api/v1/things", "{\"name\":\"a\xffb\",\"color\":\"red\"}")), 400, "malformed_request")
}

func TestSECIN2_DepthLimit(t *testing.T) {
	t.Parallel()
	cfg := httpserver.DefaultChainConfig(httpservertest.PublicURL)
	cfg.MaxJSONDepth = 3
	h, _ := createHarness(t, &cfg)
	p := wantProblem(t, h.Do(h.NewRequest(http.MethodPost, "/api/v1/things", `{"extra":{"a":{"b":{}}}}`)), 400, "malformed_request")
	if !strings.Contains(p["detail"].(string), "http.max_json_depth") {
		t.Fatalf("detail %v", p["detail"])
	}
	resp := h.Do(h.NewRequest(http.MethodPost, "/api/v1/things", `{"name":"ab","color":"red","extra":{"a":{}}}`))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("depth 3 must pass: %d %s", resp.StatusCode, resp.Body)
	}
	if err := httpserver.CheckJSON([]byte(`[[[1]]]`), 3); err != nil {
		t.Fatalf("arrays at depth 3: %v", err)
	}
	if err := httpserver.CheckJSON([]byte(`[[[[1]]]]`), 3); errs.KindOf(err) != errs.KindBadRequest {
		t.Fatalf("arrays at depth 4: %v", err)
	}
}

func TestSECIN3_BodyLimit413NamesKey(t *testing.T) {
	t.Parallel()
	cfg := httpserver.DefaultChainConfig(httpservertest.PublicURL)
	cfg.MaxJSONBodyBytes = 32
	h, got := createHarness(t, &cfg)
	big := `{"name":"ab","color":"red","extra":{"x":"` + strings.Repeat("y", 64) + `"}}`
	p := wantProblem(t, h.Do(h.NewRequest(http.MethodPost, "/api/v1/things", big)), 413, "payload_too_large")
	if p["detail"] != "Body exceeds http.max_json_body_bytes." {
		t.Fatalf("detail %v", p["detail"])
	}
	// Without Content-Length the limit applies while reading.
	req := h.NewRequest(http.MethodPost, "/api/v1/things", big)
	req.ContentLength = -1
	req.Body = io.NopCloser(strings.NewReader(big))
	wantProblem(t, h.Do(req), 413, "payload_too_large")
	if *got != "" {
		t.Fatal("the handler ran")
	}
	// Uploads name file.max_upload_bytes.
	cfg = httpserver.DefaultChainConfig(httpservertest.PublicURL)
	cfg.MaxUploadBytes = 4
	blob := httpservertest.Op("uploadBlob", http.MethodPost, "/api/v1/blobs", "blob.upload")
	blob.RequestBody = httpserver.BodyOctetStream
	hb := httpservertest.New(t, httpservertest.Options{Config: &cfg, Routes: []httpservertest.Route{{Op: blob,
		Handler: httpservertest.Authorized(blob.Action, func(w http.ResponseWriter, r *http.Request) {
			if _, err := io.ReadAll(r.Body); err != nil {
				httpserver.WriteProblem(w, r, err)
				return
			}
			w.WriteHeader(201)
		})}}})
	req = hb.NewRequest(http.MethodPost, "/api/v1/blobs", "123456789", "Content-Type", "application/octet-stream")
	req.ContentLength = -1
	req.Body = io.NopCloser(strings.NewReader("123456789"))
	p = wantProblem(t, hb.Do(req), 413, "payload_too_large")
	if p["detail"] != "Body exceeds file.max_upload_bytes." {
		t.Fatalf("detail %v", p["detail"])
	}
}

func TestBody_WrongMediaType415(t *testing.T) {
	t.Parallel()
	h, _ := createHarness(t, nil)
	wantProblem(t, h.Do(h.NewRequest(http.MethodPost, "/api/v1/things", `{"name":"ab","color":"red"}`, "Content-Type", "text/plain")), 415, "unsupported_media_type")
	wantProblem(t, h.Do(h.NewRequest(http.MethodPost, "/api/v1/things", `{"name":"ab","color":"red"}`, "Content-Type", "")), 415, "unsupported_media_type")
	resp := h.Do(h.NewRequest(http.MethodPost, "/api/v1/things", `{"name":"ab","color":"red"}`, "Content-Type", "application/json; charset=utf-8"))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("parameters of the media type: %d %s", resp.StatusCode, resp.Body)
	}
	// A body on an operation without one.
	op := getThing()
	hg := httpservertest.New(t, httpservertest.Options{Routes: []httpservertest.Route{{Op: op, Handler: okHandler(op.Action)}}})
	wantProblem(t, hg.Do(hg.NewRequest(http.MethodGet, "/api/v1/things/"+thingID, `{"x":1}`, "X-Mode", "fast")), 415, "unsupported_media_type")
}

func TestBody_MissingRequiredBody(t *testing.T) {
	t.Parallel()
	h, _ := createHarness(t, nil)
	p := wantProblem(t, h.Do(h.NewRequest(http.MethodPost, "/api/v1/things", "")), 400, "validation_failed")
	if fe, _ := p["errors"].([]any); len(fe) != 1 || fe[0].(map[string]any)["code"] != "required" {
		t.Fatalf("errors %v", p["errors"])
	}
}

func TestBody_RestoredForHandler(t *testing.T) {
	t.Parallel()
	h, got := createHarness(t, nil)
	const body = `{"name":"ab","color":"red","size":3}`
	if resp := h.Do(h.NewRequest(http.MethodPost, "/api/v1/things", body)); resp.StatusCode != 201 || *got != body {
		t.Fatalf("%d, handler got %q", resp.StatusCode, *got)
	}
}

// FuzzJSONGuard holds the guard against the decoder: it never panics, and whatever it accepts is
// valid UTF-8 JSON that encoding/json decodes.
func FuzzJSONGuard(f *testing.F) {
	for _, seed := range []string{`{}`, `[]`, `{"a":1,"a":2}`, `{"a":{"b":[1,{"c":null}]}}`, "\"\xff\"", `[[[[[[[[1]]]]]]]]`,
		`{"a":1} {"b":2}`, `{"a":`, `"x"`, `1e400`, `{"a":1,"a":2}`, ``, ` `, `nul`} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		err := httpserver.CheckJSON(data, 5)
		if err != nil {
			if errs.KindOf(err) != errs.KindBadRequest {
				t.Fatalf("kind %s", errs.KindOf(err))
			}
			return
		}
		if jerr := decodeNumbers(data); jerr != nil {
			t.Fatalf("guard accepted %q, decoder refused: %v", data, jerr)
		}
	})
}

// --- preconditions (06 "Concurrency", K4) ---

func ifMatchHarness(t *testing.T) *httpservertest.Harness {
	t.Helper()
	op := httpservertest.Op("updateThing", http.MethodPatch, thingPath, "thing.update")
	op.IfMatch = httpserver.IfMatchRequired
	const current = 7
	return httpservertest.New(t, httpservertest.Options{Routes: []httpservertest.Route{{Op: op,
		Handler: httpservertest.Authorized(op.Action, func(w http.ResponseWriter, r *http.Request) {
			im := httpserver.ParseIfMatch(r.Header)
			if !im.Valid || im.Version != current {
				httpserver.WriteProblem(w, r, errs.VersionConflict(map[string]any{"id": thingID, "version": current}))
				return
			}
			w.Header().Set("ETag", httpserver.FormatETag(current+1))
			httpservertest.JSON(w, http.StatusOK, map[string]any{"id": thingID, "version": current + 1})
		})}}})
}

func TestPreconditions_MissingIfMatch428(t *testing.T) {
	t.Parallel()
	h := ifMatchHarness(t)
	wantProblem(t, h.Do(h.NewRequest(http.MethodPatch, "/api/v1/things/"+thingID, `{"name":"ab"}`)), 428, "precondition_required")
}

func TestPreconditions_Wildcard428(t *testing.T) {
	t.Parallel()
	h := ifMatchHarness(t)
	wantProblem(t, h.Do(h.NewRequest(http.MethodPatch, "/api/v1/things/"+thingID, `{"name":"ab"}`, "If-Match", "*")), 428, "precondition_required")
}

func TestVersionConflict_412WithCurrent(t *testing.T) {
	t.Parallel()
	h := ifMatchHarness(t)
	for _, tag := range []string{`"6"`, `W/"7"`, `"7", "8"`, `7`} {
		p := wantProblem(t, h.Do(h.NewRequest(http.MethodPatch, "/api/v1/things/"+thingID, `{"name":"ab"}`, "If-Match", tag)), 412, "version_conflict")
		if cur, _ := p["current"].(map[string]any); cur["version"] != float64(7) {
			t.Errorf("%s: current %v", tag, p["current"])
		}
	}
	resp := h.Do(h.NewRequest(http.MethodPatch, "/api/v1/things/"+thingID, `{"name":"ab"}`, "If-Match", `"7"`))
	if resp.StatusCode != 200 || resp.Header.Get("ETag") != `"8"` {
		t.Fatalf("%d ETag %q", resp.StatusCode, resp.Header.Get("ETag"))
	}
}

func TestIfMatch_ParseAndFormat(t *testing.T) {
	t.Parallel()
	if got := httpserver.FormatETag(42); got != `"42"` {
		t.Fatalf("FormatETag = %s", got)
	}
	cases := map[string]httpserver.IfMatch{
		``:                        {},
		`"42"`:                    {Present: true, Version: 42, Valid: true},
		`"0"`:                     {Present: true, Version: 0, Valid: true},
		` "9" `:                   {Present: true, Version: 9, Valid: true},
		`W/"42"`:                  {Present: true},
		`"42", "43"`:              {Present: true},
		`*`:                       {Present: true},
		`"abc"`:                   {Present: true},
		`"-1"`:                    {Present: true},
		`"042"`:                   {Present: true},
		`""`:                      {Present: true},
		`"999999999999999999999"`: {Present: true}, // 21 digits, beyond int64
	}
	for v, want := range cases {
		h := http.Header{}
		if v != "" {
			h.Set("If-Match", v)
		}
		if got := httpserver.ParseIfMatch(h); got != want {
			t.Errorf("ParseIfMatch(%q) = %+v, want %+v", v, got, want)
		}
	}
}
