// SPDX-License-Identifier: Apache-2.0

// Package download_test holds the integration tests of brief M3-05: the real server of
// internal/wire in role all through test/harness, PostgreSQL 18 from pgtest, the file system
// driver in a temporary root, the matrix world (space Support with channel Allgemein, Alex with
// trigger and upload, Sam reading through the group Leser, Robin without a grant on Support),
// uploads through the real HTTP chain as Alex, and a scripted clamd (clamdtest.Fake) whose answer
// each test sets.
package download_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/clamd/clamdtest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// The fixed header values of a delivery (12 9.3, SEC-HDR-4).
const (
	wantCSP  = "default-src 'none'; sandbox"
	octet    = "application/octet-stream"
	noStore  = "private, no-store"
	inlineAt = "inline; "
)

// TestDownload runs the cases of brief M3-05 on one server.
func TestDownload(t *testing.T) {
	// A short DPoP nonce lifetime makes stale nonces, and with them a DPoP-Nonce header on any
	// response, happen in every run instead of only when a run crosses a rotation.
	e := start(t, harness.Options{}, map[string]string{"auth.dpop_nonce_ttl": "1s"})
	maxAge := e.duration(t, "client.cache_max_age")
	authCache := "private, max-age=" + strconv.FormatInt(int64(maxAge/time.Second), 10) + ", immutable"
	png := e.ready(t, "Beach day.png", pngBytes)
	pngPath := "/files/" + png + "/content"

	t.Run("InlineImageHeadersAndBody", func(t *testing.T) {
		r := get(t, e.alex, pngPath, nil)
		sum := sha256.Sum256(pngBytes)
		want := map[string]string{
			"Content-Type":                 "image/png",
			"Content-Disposition":          `inline; filename="Beach day.png"; filename*=UTF-8''Beach%20day.png`,
			"X-Content-Type-Options":       "nosniff",
			"Content-Security-Policy":      wantCSP,
			"Cross-Origin-Resource-Policy": "same-origin",
			"Cache-Control":                authCache,
			"Etag":                         `"` + hex.EncodeToString(sum[:]) + `"`,
			"Repr-Digest":                  "sha-256=:" + base64.StdEncoding.EncodeToString(sum[:]) + ":",
			"Accept-Ranges":                "bytes",
			"Content-Length":               strconv.Itoa(len(pngBytes)),
		}
		if r.Status != http.StatusOK || !bytes.Equal(r.Body, pngBytes) {
			t.Fatalf("status %d, body equal %v", r.Status, bytes.Equal(r.Body, pngBytes))
		}
		for k, v := range want {
			if got := r.Header.Get(k); got != v {
				t.Errorf("%s = %q, want %q", k, got, v)
			}
		}
	})

	t.Run("SECT03_HTMLAndSVGAsAttachment", func(t *testing.T) {
		for _, c := range []struct{ name, body string }{
			{"page.html", "<!doctype html><html><body><script>alert(1)</script></body></html>"},
			{"logo.svg", `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`},
			{"report.pdf", "%PDF-1.4\n%%EOF\n"},
		} {
			id := e.ready(t, c.name, []byte(c.body))
			r := get(t, e.alex, "/files/"+id+"/content", nil)
			if r.Status != http.StatusOK || r.Header.Get("Content-Type") != octet ||
				!strings.HasPrefix(r.Header.Get("Content-Disposition"), "attachment; ") ||
				r.Header.Get("X-Content-Type-Options") != "nosniff" || r.Header.Get("Content-Security-Policy") != wantCSP {
				t.Errorf("%s: %d %v", c.name, r.Status, r.Header)
			}
		}
	})

	t.Run("ContentDispositionSanitizedName", func(t *testing.T) {
		// 12 6.2 (M3-03) cleans the name at upload: bidi controls and control characters go, a
		// path keeps only its last element; the response carries the cleaned name in both forms.
		id := e.ready(t, "../Résumé\u202Egpj.exe\x01 for Sam.txt", []byte("plain text of Sam"))
		var f struct {
			OriginalName string `json:"original_name"`
		}
		harness.MustDo(t, e.alex, harness.Request{Method: http.MethodGet, Path: "/files/" + id}, http.StatusOK).JSON(t, &f)
		cd := get(t, e.alex, "/files/"+id+"/content", nil).Header.Get("Content-Disposition")
		if strings.ContainsAny(f.OriginalName, "\u202E\x01/") || strings.ContainsAny(cd, "\r\n\u202E") {
			t.Fatalf("name %q, disposition %q", f.OriginalName, cd)
		}
		fallback, encoded, ok := strings.Cut(strings.TrimPrefix(cd, "attachment; filename=\""), "\"; filename*=UTF-8''")
		if !ok || strings.ContainsFunc(fallback, func(r rune) bool { return r < 0x20 || r > 0x7e || strings.ContainsRune("\"\\%;", r) }) {
			t.Fatalf("disposition %q", cd)
		}
		if !strings.Contains(encoded, "R%C3%A9sum%C3%A9") || strings.ContainsAny(encoded, " \"") {
			t.Errorf("encoded name %q", encoded)
		}
	})

	t.Run("Range", func(t *testing.T) {
		size := len(pngBytes)
		etag := get(t, e.alex, pngPath, nil).Header.Get("ETag")
		for _, c := range []struct {
			rng         string
			first, last int
		}{{"bytes=0-99", 0, 99}, {"bytes=100-", 100, size - 1}, {"bytes=-10", size - 10, size - 1}} {
			r := get(t, e.alex, pngPath, header("Range", c.rng))
			want := fmt.Sprintf("bytes %d-%d/%d", c.first, c.last, size)
			if r.Status != http.StatusPartialContent || r.Header.Get("Content-Range") != want ||
				r.Header.Get("Content-Length") != strconv.Itoa(c.last-c.first+1) || !bytes.Equal(r.Body, pngBytes[c.first:c.last+1]) {
				t.Errorf("%s: %d %q", c.rng, r.Status, r.Header.Get("Content-Range"))
			}
		}
		if r := get(t, e.alex, pngPath, header("Range", "bytes=0-1,5-9")); r.Status != http.StatusOK || !bytes.Equal(r.Body, pngBytes) {
			t.Errorf("two ranges: %d", r.Status)
		}
		r := get(t, e.alex, pngPath, header("Range", "bytes="+strconv.Itoa(size)+"-"))
		if r.Status != http.StatusRequestedRangeNotSatisfiable || r.Header.Get("Content-Range") != "bytes */"+strconv.Itoa(size) ||
			!strings.Contains(string(r.Body), `"range_not_satisfiable"`) {
			t.Errorf("range at the size: %d %q %s", r.Status, r.Header.Get("Content-Range"), r.Body)
		}
		if r := get(t, e.alex, pngPath, header("Range", "bytes=x-y")); r.Status != http.StatusOK {
			t.Errorf("invalid range: %d", r.Status)
		}
		h := header("Range", "bytes=0-9")
		h.Set("If-Range", etag)
		if r := get(t, e.alex, pngPath, h); r.Status != http.StatusPartialContent {
			t.Errorf("If-Range with the ETag: %d", r.Status)
		}
		h.Set("If-Range", `"other"`)
		if r := get(t, e.alex, pngPath, h); r.Status != http.StatusOK {
			t.Errorf("If-Range with another value: %d", r.Status)
		}
	})

	thumb := bytes.Repeat([]byte{0xff, 0xd8}, 40)
	e.seedThumbnail(t, png, "small", "image/jpeg", thumb)

	t.Run("ForeignAndUnknownIDSame404", func(t *testing.T) {
		unknown := ids.New().String()
		calls := []struct {
			name string
			do   func(c *harness.Client, id string) harness.Response
		}{
			{"downloadFile", func(c *harness.Client, id string) harness.Response { return get(t, c, "/files/"+id+"/content", nil) }},
			{"downloadThumbnail", func(c *harness.Client, id string) harness.Response {
				return get(t, c, "/files/"+id+"/thumbnails/small", nil)
			}},
			{"getTranscript", func(c *harness.Client, id string) harness.Response { return get(t, c, "/files/"+id+"/transcript", nil) }},
			{"createFileTicket", func(c *harness.Client, id string) harness.Response {
				return c.Do(t.Context(), harness.Request{Method: http.MethodPost, Path: "/files/" + id + "/tickets", Body: map[string]any{"variant": "original"}})
			}},
		}
		for _, call := range calls {
			ref := call.do(e.sam, unknown)
			if ref.Status != http.StatusNotFound {
				t.Fatalf("%s unknown: %d %s", call.name, ref.Status, ref.Body)
			}
			for who, c := range map[string]*harness.Client{"Sam": e.sam, "Robin": e.robin} {
				for _, id := range []string{png, unknown} {
					r := call.do(c, id)
					if r.Status != ref.Status || problemOf(t, r) != problemOf(t, ref) || fmt.Sprint(headerSet(r)) != fmt.Sprint(headerSet(ref)) {
						t.Errorf("%s by %s on Alex file %v: %d %v %s", call.name, who, id == png, r.Status, headerSet(r), r.Body)
					}
				}
			}
		}
	})

	t.Run("NotReadyAndBlocked", func(t *testing.T) {
		retry := strconv.FormatInt(int64(e.duration(t, "file.not_ready_retry_after")/time.Second), 10)
		ops := func(c *harness.Client, id string) []harness.Response {
			return []harness.Response{
				get(t, c, "/files/"+id+"/content", nil), get(t, c, "/files/"+id+"/thumbnails/small", nil), get(t, c, "/files/"+id+"/transcript", nil),
				c.Do(t.Context(), harness.Request{Method: http.MethodPost, Path: "/files/" + id + "/tickets", Body: map[string]any{"variant": "original"}}),
			}
		}
		e.clamd.SetScript(clamdtest.Script{Stall: true})
		pending := e.upload(t, e.alex, "pending.txt", []byte("waiting for clamd"), []byte("waiting for clamd"))
		time.Sleep(1500 * time.Millisecond)
		if st := e.state(t, pending); st != "pending/pending" {
			t.Fatalf("with clamd stalled the file is %s", st)
		}
		for i, r := range ops(e.alex, pending) {
			if r.Status != http.StatusConflict || !strings.Contains(string(r.Body), `"file_not_ready"`) || r.Header.Get("Retry-After") != retry {
				t.Errorf("pending op %d: %d %q %s", i, r.Status, r.Header.Get("Retry-After"), r.Body)
			}
		}
		for i, r := range ops(e.sam, pending) {
			if r.Status != http.StatusNotFound {
				t.Errorf("pending op %d for Sam: %d", i, r.Status)
			}
		}
		e.clamd.SetScript(clamdtest.Script{})
		if st := e.waitFinal(t, pending); st != "complete/clean" {
			t.Fatalf("after clamd answered OK the file is %s", st)
		}
		if r := get(t, e.alex, "/files/"+pending+"/content", nil); r.Status != http.StatusOK {
			t.Errorf("deliverable file: %d", r.Status)
		}

		blocked := map[string]string{}
		e.clamd.SetScript(clamdtest.Script{Reply: "stream: Eicar-Test-Signature FOUND"})
		blocked["infected"] = e.upload(t, e.alex, "eicar.txt", []byte("infected body"), []byte("infected body"))
		e.waitFinal(t, blocked["infected"])
		e.clamd.SetScript(clamdtest.Script{Reply: "stream: Can't allocate memory ERROR"})
		blocked["error"] = e.upload(t, e.alex, "error.txt", []byte("error body"), []byte("error body"))
		e.waitFinal(t, blocked["error"])
		e.clamd.SetScript(clamdtest.Script{})
		blocked["failed"] = e.upload(t, e.alex, "mismatch.txt", []byte("these bytes"), []byte("other bytes"))
		e.waitFinal(t, blocked["failed"])
		for kind, id := range blocked {
			for i, r := range ops(e.alex, id) {
				if r.Status != http.StatusConflict || !strings.Contains(string(r.Body), `"file_blocked"`) || r.Header.Get("Retry-After") != "" {
					t.Errorf("%s (%s) op %d: %d %q %s", kind, e.state(t, id), i, r.Status, r.Header.Get("Retry-After"), r.Body)
				}
			}
		}
	})

	t.Run("DeletedFileIs404", func(t *testing.T) {
		id := e.ready(t, "deleted.txt", []byte("soon gone"))
		ref := get(t, e.alex, "/files/"+ids.New().String()+"/content", nil)
		e.exec(t, `UPDATE files SET deleted_at = now() WHERE id = $1`, id)
		r := get(t, e.alex, "/files/"+id+"/content", nil)
		if r.Status != http.StatusNotFound || problemOf(t, r) != problemOf(t, ref) {
			t.Errorf("deleted file: %d %s", r.Status, r.Body)
		}
	})

	t.Run("TicketOnlyQueryParameterOnlyDownloads", func(t *testing.T) {
		tk := newTicket(t, e.alex, png, "original")
		raw := strings.SplitN(tk.URL, "?ticket=", 2)[1]
		if r := e.fetch(t, "/api/v1"+pngPath, header("Authorization", "Bearer "+raw)); r.Status != http.StatusUnauthorized {
			t.Errorf("ticket as bearer: %d", r.Status)
		}
		if r := e.fetch(t, "/api/v1/files/"+png+"/transcript?ticket="+raw, nil); r.Status != http.StatusUnauthorized {
			t.Errorf("ticket on getTranscript: %d", r.Status)
		}
		req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, e.s.PublicURL+"/api/v1/files/"+png+"/tickets?ticket="+raw,
			strings.NewReader(`{"variant":"original"}`))
		req.Header.Set("Content-Type", "application/json")
		if resp, err := e.plain.Do(req); err != nil || resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("ticket on createFileTicket: %v %v", resp, err)
		} else {
			_ = resp.Body.Close()
		}
		other := e.ready(t, "other.txt", []byte("file B"))
		small := newTicket(t, e.alex, png, "small")
		smallRaw := strings.SplitN(small.URL, "?ticket=", 2)[1]
		tampered := []byte(raw)
		tampered[10] ^= 1
		ref := e.fetch(t, "/api/v1/files/"+ids.New().String()+"/content?ticket="+raw, nil)
		for name, ref2 := range map[string]string{
			"file A on B":         "/api/v1/files/" + other + "/content?ticket=" + raw,
			"small on content":    "/api/v1/files/" + png + "/content?ticket=" + smallRaw,
			"original on a thumb": "/api/v1/files/" + png + "/thumbnails/small?ticket=" + raw,
			"tampered":            "/api/v1" + pngPath + "?ticket=" + string(tampered),
		} {
			r := e.fetch(t, ref2, nil)
			if r.Status != http.StatusNotFound || problemOf(t, r) != problemOf(t, ref) {
				t.Errorf("%s: %d %s", name, r.Status, r.Body)
			}
		}
		r := get(t, e.alex, pngPath+"?ticket="+smallRaw, nil)
		if r.Status != http.StatusOK || r.Header.Get("Cache-Control") != authCache {
			t.Errorf("Authorization with a ticket: %d %q", r.Status, r.Header.Get("Cache-Control"))
		}
	})

	t.Run("TicketThumbnailVariant", func(t *testing.T) {
		tk := newTicket(t, e.alex, png, "small")
		if !strings.HasPrefix(tk.URL, "/api/v1/files/"+png+"/thumbnails/small?ticket=") {
			t.Fatalf("url prefix %q", strings.SplitN(tk.URL, "?", 2)[0])
		}
		r := e.fetch(t, tk.URL, nil)
		sum := sha256.Sum256(thumb)
		if r.Status != http.StatusOK || !bytes.Equal(r.Body, thumb) || r.Header.Get("Content-Type") != "image/jpeg" ||
			r.Header.Get("Content-Disposition") != `inline; filename="Beach day-small.jpg"; filename*=UTF-8''Beach%20day-small.jpg` ||
			r.Header.Get("ETag") != `"`+hex.EncodeToString(sum[:])+`"` ||
			r.Header.Get("Repr-Digest") != "sha-256=:"+base64.StdEncoding.EncodeToString(sum[:])+":" || r.Header.Get("Cache-Control") != noStore {
			t.Errorf("thumbnail with ticket: %d %v", r.Status, r.Header)
		}
		// The job of M3-06 renders small, medium and large of an image, never a poster.
		if r := e.alex.Do(t.Context(), harness.Request{Method: http.MethodPost, Path: "/files/" + png + "/tickets", Body: map[string]any{"variant": "poster"}}); r.Status != http.StatusNotFound {
			t.Errorf("ticket of a missing variant: %d", r.Status)
		}
	})

	t.Run("CreateFileTicketNeedsSession", func(t *testing.T) {
		if r, err := harness.DoWithStepUp(t.Context(), e.w.Owner, harness.Request{Method: http.MethodPut, Path: "/feature-flags/tokens.personal_enabled",
			Body: map[string]any{"enabled": true, "reason": "personal tokens for the download tests"}}); err != nil || r.Status != http.StatusOK {
			t.Fatalf("putFeatureFlag: %v %d %s", err, r.Status, r.Body)
		}
		resp, err := harness.DoWithStepUp(t.Context(), e.alex, harness.Request{Method: http.MethodPost, Path: "/principals/" + e.alex.PrincipalID + "/tokens",
			Body: map[string]any{"name": "alex tool", "expires_at": time.Now().Add(time.Hour).UTC().Format("2006-01-02T15:04:05.000Z")}})
		if err != nil || resp.Status != http.StatusCreated {
			t.Fatalf("createToken: %v %d %s", err, resp.Status, resp.Body)
		}
		var tok struct {
			Secret string `json:"secret"`
		}
		resp.JSON(t, &tok)
		post := func(id string) harness.Response {
			req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, e.s.APIBase+"/files/"+id+"/tickets", strings.NewReader(`{"variant":"original"}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+tok.Secret)
			r, err := e.plain.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = r.Body.Close() }()
			var b bytes.Buffer
			_, _ = b.ReadFrom(r.Body)
			return harness.Response{Status: r.StatusCode, Header: r.Header, Body: b.Bytes()}
		}
		if r := post(png); r.Status != http.StatusForbidden || !strings.Contains(string(r.Body), `"step_up_not_possible"`) {
			t.Errorf("API token on a readable file: %d %s", r.Status, r.Body)
		}
		robins := e.seedFile(t, e.robin.PrincipalID, "gdpr_export", "export.zip", octet, []byte("robin export"))
		if r := post(robins); r.Status != http.StatusNotFound {
			t.Errorf("API token on an invisible file: %d %s", r.Status, r.Body)
		}
	})

	t.Run("PurposeRules", func(t *testing.T) {
		avatar := e.seedFile(t, e.alex.PrincipalID, "avatar", "alex.png", "image/png", pngBytes)
		branding := e.seedFile(t, e.w.Owner.PrincipalID, "branding", "logo.png", "image/png", pngBytes)
		export := e.seedFile(t, e.alex.PrincipalID, "gdpr_export", "export.zip", octet, []byte("alex export"))
		if r := get(t, e.robin, "/files/"+avatar+"/content", nil); r.Status != http.StatusOK {
			t.Errorf("avatar for Robin: %d %s", r.Status, r.Body)
		}
		for who, c := range map[string]*harness.Client{"Robin": e.robin, "Sam": e.sam, "Alex": e.alex} {
			if r := get(t, c, "/files/"+branding+"/content", nil); r.Status != http.StatusOK {
				t.Errorf("branding for %s: %d %s", who, r.Status, r.Body)
			}
		}
		if r := get(t, e.alex, "/files/"+export+"/content", nil); r.Status != http.StatusNotFound {
			t.Errorf("gdpr_export for its uploader: %d", r.Status)
		}
	})

	t.Run("TranscriptRead", func(t *testing.T) {
		voice := e.ready(t, "voice.m4a", []byte("not really audio"))
		r := get(t, e.alex, "/files/"+voice+"/transcript", nil)
		if r.Status != http.StatusOK || strings.Contains(string(r.Body), "null") || strings.TrimSpace(string(r.Body)) != `{"file_id":"`+voice+`","status":"none"}` {
			t.Errorf("status none: %d %s", r.Status, r.Body)
		}
		e.exec(t, `UPDATE files SET transcript_status = 'done', transcript_text = 'Please call Sam back.', transcript_language = 'en', transcript_source = 'agent' WHERE id = $1`, voice)
		var tr map[string]any
		harness.MustDo(t, e.alex, harness.Request{Method: http.MethodGet, Path: "/files/" + voice + "/transcript"}, http.StatusOK).JSON(t, &tr)
		if tr["status"] != "done" || tr["text"] != "Please call Sam back." || tr["language"] != "en" || tr["source"] != "agent" {
			t.Errorf("status done: %v", tr)
		}
	})

	t.Run("AnonymousWithoutTicketIs401", func(t *testing.T) {
		for _, p := range []string{pngPath, "/files/" + ids.New().String() + "/content"} {
			if r := e.fetch(t, "/api/v1"+p, nil); r.Status != http.StatusUnauthorized {
				t.Errorf("anonymous %s: %d", p, r.Status)
			}
		}
	})

	t.Run("SECT03_FileResponseHeaders", func(t *testing.T) {
		tk := newTicket(t, e.alex, png, "original")
		pending := ""
		e.clamd.SetScript(clamdtest.Script{Stall: true})
		pending = e.upload(t, e.alex, "pending2.txt", []byte("pending again"), []byte("pending again"))
		file := []string{"Accept-Ranges", "Cache-Control", "Content-Disposition", "Content-Security-Policy", "Content-Type",
			"Cross-Origin-Resource-Policy", "Etag", "Referrer-Policy", "Repr-Digest", "X-Content-Type-Options"}
		problem := []string{"Cache-Control", "Content-Type", "Cross-Origin-Resource-Policy", "Referrer-Policy", "X-Content-Type-Options"}
		cases := []struct {
			name string
			r    harness.Response
			want []string
		}{
			{"200", get(t, e.alex, pngPath, nil), file},
			{"200 ticket", e.fetch(t, tk.URL, nil), file},
			{"206", get(t, e.alex, pngPath, header("Range", "bytes=0-1")), append([]string{"Content-Range"}, file...)},
			{"416", get(t, e.alex, pngPath, header("Range", "bytes=99999-")), append([]string{"Content-Range"}, problem...)},
			{"409", get(t, e.alex, "/files/"+pending+"/content", nil), append([]string{"Retry-After"}, problem...)},
			{"404", get(t, e.sam, pngPath, nil), problem},
		}
		e.clamd.SetScript(clamdtest.Script{})
		for _, c := range cases {
			got := headerSet(c.r)
			delete(got, "Strict-Transport-Security")
			var names []string
			for k := range got {
				names = append(names, k)
			}
			if fmt.Sprint(sorted(names)) != fmt.Sprint(sorted(c.want)) || got["X-Content-Type-Options"] != "nosniff" ||
				got["Referrer-Policy"] != "no-referrer" || c.r.Header.Get("Server") != "" {
				t.Errorf("%s: headers %v, want %v", c.name, sorted(names), sorted(c.want))
			}
			if c.r.Status < 300 && (got["Content-Security-Policy"] != wantCSP || !strings.HasPrefix(got["Cache-Control"], "private")) {
				t.Errorf("%s: CSP %q Cache-Control %q", c.name, got["Content-Security-Policy"], got["Cache-Control"])
			}
		}
		e.waitFinal(t, pending)
	})
}

// sorted returns s sorted.
func sorted(s []string) []string {
	out := append([]string(nil), s...)
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}
