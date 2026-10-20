// SPDX-License-Identifier: Apache-2.0

package images_test

import (
	"bytes"
	"encoding/json"
	"io"
	"maps"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/test/harness"
)

// The paths of the five operations of brief M3-10 below /api/v1.
const (
	pathAvatar = "/me/avatar"
	pathLogo   = "/instance/branding/logo"
)

// part is one part of a multipart body.
type part struct {
	name, ctype string
	data        []byte
}

// imageBody builds a multipart/form-data body of parts and returns it with its Content-Type.
func imageBody(t *testing.T, parts ...part) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for _, p := range parts {
		h := textproto.MIMEHeader{}
		h.Set("Content-Disposition", `form-data; name="`+p.name+`"; filename="canary-name-me-at-example-com.png"`)
		if p.ctype != "" {
			h.Set("Content-Type", p.ctype)
		}
		pw, err := w.CreatePart(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pw.Write(p.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes(), w.FormDataContentType()
}

// putImage sends PUT path as c with the multipart body of parts; a step-up is performed when the
// server asks for one.
func putImage(t *testing.T, c *harness.Client, path string, parts ...part) harness.Response {
	t.Helper()
	body, ctype := imageBody(t, parts...)
	h := http.Header{}
	h.Set("Content-Type", ctype)
	resp, err := harness.DoWithStepUp(t.Context(), c, harness.Request{Method: http.MethodPut, Path: path, Body: body, Header: h})
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

// file is one part "file" with the declared type ctype.
func file(ctype string, data []byte) part { return part{name: "file", ctype: ctype, data: data} }

// blockingBody is a request body that sends nothing until the test ends: a server that reads the
// body before it answers never answers, so an answer proves the body was not read.
type blockingBody struct {
	r *io.PipeReader
	w *io.PipeWriter
}

// newBlockingBody returns a body that is closed when the test ends.
func newBlockingBody(t *testing.T) *blockingBody {
	r, w := io.Pipe()
	t.Cleanup(func() { _ = w.Close() })
	return &blockingBody{r: r, w: w}
}

func (b *blockingBody) Read(p []byte) (int, error) { return b.r.Read(p) }

// Close ends the body, as the transport does after an early answer.
func (b *blockingBody) Close() error { return b.r.Close() }

// heldBodyLength is the declared length of the held body: above the 256 KiB that net/http drains
// after a handler that left the body unread (so the answer is not held by the drain) and below
// file.max_upload_bytes of the test server (so the chain does not refuse it by its length).
const heldBodyLength = 512 << 10

// putUnread sends PUT path as c (anonymous when anon) with a multipart Content-Type and a body that
// never arrives, after a GET that fetches a DPoP nonce, and returns the answer.
func putUnread(t *testing.T, c *harness.Client, path string, anon bool) harness.Response {
	t.Helper()
	c.Do(t.Context(), harness.Request{Method: http.MethodGet, Path: "/me"})
	h := http.Header{}
	h.Set("Content-Type", "multipart/form-data; boundary=canaryboundary")
	r := harness.Request{Method: http.MethodPut, Path: path, Body: newBlockingBody(t), Header: h, ContentLength: heldBodyLength}
	done := make(chan harness.Response, 1)
	go func() {
		if anon {
			done <- c.DoAnonymous(t.Context(), r)
			return
		}
		done <- c.Do(t.Context(), r)
	}()
	select {
	case resp := <-done:
		return resp
	case <-time.After(waitJob / 4):
		t.Fatalf("PUT %s: no answer while the body is held, so the server reads it before it decides", path)
	}
	return harness.Response{}
}

// problem is the part of a problem body the tests compare.
type problem struct {
	Code   string `json:"code"`
	Reason string `json:"reason"`
	Detail string `json:"detail"`
	Errors []struct {
		Pointer string `json:"pointer"`
		Code    string `json:"code"`
	} `json:"errors"`
}

// problemOf decodes the problem body of r.
func problemOf(t *testing.T, r harness.Response) problem {
	t.Helper()
	var p problem
	if err := json.Unmarshal(r.Body, &p); err != nil {
		t.Fatalf("problem %d %s: %v", r.Status, r.Body, err)
	}
	return p
}

// sameAnswer fails the test unless a and b have the same status, the same body apart from the
// correlation id and the same headers apart from Correlation-Id, Date and the counters of the rate
// limit (AZ-P4: an unknown and an invisible resource answer alike).
func sameAnswer(t *testing.T, what string, a, b harness.Response) {
	t.Helper()
	if a.Status != b.Status {
		t.Fatalf("%s: status %d and %d", what, a.Status, b.Status)
	}
	if !bytes.Equal(withoutCorrelation(t, a.Body), withoutCorrelation(t, b.Body)) {
		t.Errorf("%s: bodies differ:\n%s\n%s", what, a.Body, b.Body)
	}
	skip := map[string]bool{"Correlation-Id": true, "Date": true, "Ratelimit": true, "Ratelimit-Policy": true}
	ha, hb := map[string][]string{}, map[string][]string{}
	for k, v := range a.Header {
		if !skip[k] {
			ha[k] = v
		}
	}
	for k, v := range b.Header {
		if !skip[k] {
			hb[k] = v
		}
	}
	if !maps.EqualFunc(ha, hb, slices.Equal) {
		t.Errorf("%s: headers differ:\n%v\n%v", what, ha, hb)
	}
}

// withoutCorrelation returns body with every member naming the correlation id removed.
func withoutCorrelation(t *testing.T, body []byte) []byte {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("body %s: %v", body, err)
	}
	delete(m, "correlation_id")
	delete(m, "instance")
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// user is the part of the schema User the tests read.
type user struct {
	ID           string `json:"id"`
	AvatarFileID string `json:"avatar_file_id"`
}

// branding is the schema Branding.
type branding struct {
	ProductName string `json:"product_name"`
	AccentColor string `json:"accent_color"`
	LogoFileID  string `json:"logo_file_id"`
	Version     int64  `json:"version"`
}

// rowCount counts the rows q finds for args.
func (e *env) rowCount(t *testing.T, q string, args ...any) int {
	t.Helper()
	var n int
	if err := e.db.QueryRow(t.Context(), q, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// avatarOf reads principals.avatar_file_id of id, "" for NULL.
func (e *env) avatarOf(t *testing.T, id string) string {
	t.Helper()
	var v *string
	if err := e.db.QueryRow(t.Context(), `SELECT avatar_file_id::text FROM principals WHERE id = $1`, id).Scan(&v); err != nil {
		t.Fatal(err)
	}
	if v == nil {
		return ""
	}
	return *v
}

// eventRow is one event_log row with the columns the tests check.
type eventRow struct {
	Seq                         int64
	Type, ResourceKind, Receive string
	Audience, Space, Channel    *string
	Payload                     []byte
}

// eventsOf returns the events of type typ on resource id in seq order.
func (e *env) eventsOf(t *testing.T, typ, id string) []eventRow {
	t.Helper()
	rows, err := e.db.Query(t.Context(), `SELECT seq, type, resource_kind, receive_action, audience_principal_id::text, space_id::text,
		channel_id::text, payload FROM event_log WHERE type = $1 AND resource_id = $2 ORDER BY seq`, typ, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []eventRow
	for rows.Next() {
		var r eventRow
		if err := rows.Scan(&r.Seq, &r.Type, &r.ResourceKind, &r.Receive, &r.Audience, &r.Space, &r.Channel, &r.Payload); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// noNull fails the test when the JSON document b holds a null anywhere (ADR 0053).
func noNull(t *testing.T, what string, b []byte) {
	t.Helper()
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("%s: %v", what, err)
	}
	var walk func(any) bool
	walk = func(x any) bool {
		switch y := x.(type) {
		case nil:
			return true
		case map[string]any:
			for _, z := range y {
				if walk(z) {
					return true
				}
			}
		case []any:
			for _, z := range y {
				if walk(z) {
					return true
				}
			}
		}
		return false
	}
	if walk(v) {
		t.Errorf("%s carries a JSON null: %s", what, b)
	}
}
