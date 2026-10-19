// SPDX-License-Identifier: Apache-2.0

package uploads_test

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"github.com/OWNER/bothy/services/server/test/harness"
)

func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }

// partBytes is file.upload_part_bytes of the test servers: small, so a file of a few bytes has
// several parts.
const partBytes = 4

// baseConfig are the keys every test server of this package sets; extra entries win.
func baseConfig(extra map[string]string) map[string]string {
	cfg := harness.RaisedRateLimits()
	cfg["file.upload_part_bytes"] = "4"
	cfg["file.max_upload_bytes"] = "40000"
	cfg["file.upload_max_open_sessions"] = "1000"
	for k, v := range extra {
		cfg[k] = v
	}
	return cfg
}

// content is a file of 10 bytes: parts of 4, 4 and 2 bytes.
var content = []byte("0123456789")

// sha is the lowercase hex SHA-256 of b.
func sha(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// digest is the Content-Digest header of b (RFC 9530).
func digest(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha-256=:" + base64.StdEncoding.EncodeToString(sum[:]) + ":"
}

// part returns part n (from 1) of b.
func part(b []byte, n int) []byte {
	lo := (n - 1) * partBytes
	return b[lo:min(lo+partBytes, len(b))]
}

// session is the answer of createUpload.
type session struct {
	FileID        string `json:"file_id"`
	Mode          string `json:"mode"`
	PartSizeBytes int64  `json:"part_size_bytes"`
	ExpiresAt     string `json:"expires_at"`
	Parts         []struct {
		PartNumber int    `json:"part_number"`
		URL        string `json:"url"`
		Headers    []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"headers"`
	} `json:"parts"`
}

// create asks for an upload of b named name on channel and returns the answer.
func create(t *testing.T, c *harness.Client, channel, name string, b []byte, extra map[string]any) harness.Response {
	t.Helper()
	body := map[string]any{"name": name, "size_bytes": len(b), "sha256": sha(b)}
	for k, v := range extra {
		body[k] = v
	}
	return c.Do(t.Context(), harness.Request{Method: http.MethodPost, Path: "/channels/" + channel + "/uploads", Body: body})
}

// mustCreate is create with a 201 answer, decoded.
func mustCreate(t *testing.T, c *harness.Client, channel, name string, b []byte) session {
	t.Helper()
	r := create(t, c, channel, name, b, nil)
	if r.Status != http.StatusCreated {
		t.Fatalf("createUpload: %d %s", r.Status, r.Body)
	}
	var s session
	r.JSON(t, &s)
	return s
}

// put sends body as part n of the upload with the Content-Digest header digestHeader ("" sends
// none).
func put(t *testing.T, c *harness.Client, fileID string, n int, body []byte, digestHeader string) harness.Response {
	t.Helper()
	h := http.Header{}
	h.Set("Content-Type", "application/octet-stream")
	if digestHeader != "" {
		h.Set("Content-Digest", digestHeader)
	}
	return c.Do(t.Context(), harness.Request{Method: http.MethodPut, Path: "/uploads/" + fileID + "/parts/" + itoa(n), Body: body, Header: h})
}

// mustPut sends part n of b with its digest and wants 200; it returns the etag.
func mustPut(t *testing.T, c *harness.Client, fileID string, n int, b []byte) string {
	t.Helper()
	r := put(t, c, fileID, n, part(b, n), digest(part(b, n)))
	if r.Status != http.StatusOK {
		t.Fatalf("uploadPart %d: %d %s", n, r.Status, r.Body)
	}
	var out struct {
		PartNumber int    `json:"part_number"`
		ETag       string `json:"etag"`
	}
	r.JSON(t, &out)
	return out.ETag
}

// uploadAll creates an upload of b and sends every part; it returns the session and the etags.
func uploadAll(t *testing.T, c *harness.Client, channel, name string, b []byte) (session, []map[string]any) {
	t.Helper()
	s := mustCreate(t, c, channel, name, b)
	var parts []map[string]any
	for n := 1; n <= len(s.Parts); n++ {
		parts = append(parts, map[string]any{"part_number": n, "etag": mustPut(t, c, s.FileID, n, b)})
	}
	return s, parts
}

// complete calls completeUpload with parts.
func complete(t *testing.T, c *harness.Client, fileID string, parts []map[string]any) harness.Response {
	t.Helper()
	if parts == nil {
		parts = []map[string]any{}
	}
	return c.Do(t.Context(), harness.Request{Method: http.MethodPost, Path: "/uploads/" + fileID + "/complete", Body: map[string]any{"parts": parts}})
}

// get reads path as c.
func get(t *testing.T, c *harness.Client, path string) harness.Response {
	t.Helper()
	return c.Do(t.Context(), harness.Request{Method: http.MethodGet, Path: path})
}

// abort calls abortUpload.
func abort(t *testing.T, c *harness.Client, fileID string) harness.Response {
	t.Helper()
	return c.Do(t.Context(), harness.Request{Method: http.MethodDelete, Path: "/uploads/" + fileID})
}

// problem is the decoded problem body.
type problem struct {
	Code   string `json:"code"`
	Reason string `json:"reason"`
	Detail string `json:"detail"`
	Errors []struct {
		Pointer string `json:"pointer"`
		Code    string `json:"code"`
	} `json:"errors"`
}

// wantProblem checks status, code and reason of r and returns the problem.
func wantProblem(t *testing.T, r harness.Response, status int, code, reason string) problem {
	t.Helper()
	var p problem
	if r.Status != status || json.Unmarshal(r.Body, &p) != nil || p.Code != code || p.Reason != reason {
		t.Fatalf("got %d %s, want %d %s/%s", r.Status, r.Body, status, code, reason)
	}
	return p
}

// wantField checks a 400 validation_failed with one field error of code at pointer.
func wantField(t *testing.T, r harness.Response, pointer, code string) {
	t.Helper()
	p := wantProblem(t, r, http.StatusBadRequest, "validation_failed", "")
	for _, e := range p.Errors {
		if e.Pointer == pointer && e.Code == code {
			return
		}
	}
	t.Fatalf("errors %+v, want %s at %q", p.Errors, code, pointer)
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}
