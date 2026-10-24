// SPDX-License-Identifier: Apache-2.0

package transcription_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/transcription"
	"github.com/OWNER/bothy/services/server/internal/files/adapters/transcription/transcriptiontest"
	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// Canaries of the content that must never reach a log line or an error text.
const (
	canaryText = "canary-transcript-7f3a"
	canaryBody = "canary-error-body-91c2"
)

// TestOpenAICompatible_RequestForm (M3-09 design note 11 b): one POST to the configured path with
// multipart/form-data; the part file is named audio.m4a or audio.mp3 and carries the stored type;
// response_format is json; model only with voice.transcription_model, language only with a hint,
// Authorization only with voice.transcription_api_key_ref (read from a secret file).
func TestOpenAICompatible_RequestForm(t *testing.T) {
	t.Parallel()
	t.Run("minimal", func(t *testing.T) {
		t.Parallel()
		s := newSetup(t, options{})
		res, err := s.call(t, "")
		if err != nil {
			t.Fatalf("Transcribe: %v", err)
		}
		if res.Text != transcriptiontest.DefaultText || res.Language != "en" {
			t.Errorf("result = %+v", res)
		}
		reqs := s.fake.Requests()
		if len(reqs) != 1 {
			t.Fatalf("%d requests, want 1", len(reqs))
		}
		r := reqs[0]
		if r.Method != http.MethodPost || r.Path != transcriptiontest.Path || r.RawQuery != "" {
			t.Errorf("request line %s %s?%s", r.Method, r.Path, r.RawQuery)
		}
		if !strings.HasPrefix(r.ContentType, "multipart/form-data; boundary=") || r.ParseError != "" {
			t.Errorf("content type %q, parse error %q", r.ContentType, r.ParseError)
		}
		if want := map[string]string{"response_format": "json"}; !equalFields(r.Fields, want) {
			t.Errorf("fields %v, want %v", r.Fields, want)
		}
		if r.FileName != "audio.m4a" || r.FileType != "audio/mp4" || !bytes.Equal(r.FileBytes, testAudio) {
			t.Errorf("file part %q %q (%d bytes)", r.FileName, r.FileType, len(r.FileBytes))
		}
		if r.Authorization != "" {
			t.Error("Authorization sent without a key reference")
		}
	})
	t.Run("model, hint and key", func(t *testing.T) {
		t.Parallel()
		s := newSetup(t, options{cfg: transcription.Config{Model: "whisper-1"}, withKey: true})
		if _, err := s.call(t, "en"); err != nil {
			t.Fatalf("Transcribe: %v", err)
		}
		r := s.fake.Requests()[0]
		if want := map[string]string{"model": "whisper-1", "language": "en", "response_format": "json"}; !equalFields(r.Fields, want) {
			t.Errorf("fields %v, want %v", r.Fields, want)
		}
		if r.Authorization != "Bearer "+testKey {
			t.Error("Authorization is not the bearer key of the secret file")
		}
	})
	t.Run("mp3", func(t *testing.T) {
		t.Parallel()
		s := newSetup(t, options{})
		ctx := testkit.Context(t)
		if _, err := s.adapter.Transcribe(ctx, app.TranscribeRequest{FileID: ids.New(), MIMEType: "audio/mpeg",
			Audio: bytes.NewReader(testAudio)}); err != nil {
			t.Fatalf("Transcribe: %v", err)
		}
		if r := s.fake.Requests()[0]; r.FileName != "audio.mp3" || r.FileType != "audio/mpeg" {
			t.Errorf("file part %q %q", r.FileName, r.FileType)
		}
	})
}

// equalFields compares form fields.
func equalFields(got, want map[string]string) bool {
	if len(got) != len(want) {
		return false
	}
	for k, v := range want {
		if got[k] != v {
			return false
		}
	}
	return true
}

// class is the expected outcome of a call.
type class string

const (
	classOK        class = "ok"
	classTransient class = "transient"
	classPermanent class = "permanent"
)

// errorRow is one row of the error class table.
type errorRow struct {
	name    string
	program func(t *testing.T, s setup)
	timeout time.Duration
	want    class
	status  int // the status code the Warn line names; 0 for none
}

// respond programs one answer.
func respond(status int, body string) func(*testing.T, setup) {
	return func(_ *testing.T, s setup) { s.fake.Respond(status, body) }
}

// errorRows is the table of design note 11 c and d, shared by the error class and the log tests.
func errorRows() []errorRow {
	padding := strings.Repeat("p", testMaxResponseBytes)
	return []errorRow{
		{name: "200 with text", program: respond(http.StatusOK, `{"text":"`+canaryText+`","language":"de","other":1}`), want: classOK},
		{name: "200 with text at the limit", program: respond(http.StatusOK, `{"text":"`+strings.Repeat("a", testMaxTextBytes)+`"}`), want: classOK},
		{name: "200 without text", program: respond(http.StatusOK, `{"language":"en"}`), want: classPermanent, status: http.StatusOK},
		{name: "200 with null text", program: respond(http.StatusOK, `{"text":null}`), want: classPermanent, status: http.StatusOK},
		{name: "invalid JSON", program: respond(http.StatusOK, `{"text":"`+canaryText), want: classPermanent, status: http.StatusOK},
		{name: "text with U+0000", program: respond(http.StatusOK, `{"text":"a\u0000`+canaryText+`"}`), want: classPermanent, status: http.StatusOK},
		{name: "text over the limit", program: respond(http.StatusOK, `{"text":"`+strings.Repeat("a", testMaxTextBytes+1)+`"}`), want: classPermanent, status: http.StatusOK},
		{name: "307 to another server", program: func(t *testing.T, s setup) {
			other := transcriptiontest.New(t)
			s.fake.RespondFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, other.URL(), http.StatusTemporaryRedirect)
			})
			t.Cleanup(func() {
				if n := len(other.Requests()); n != 0 {
					t.Errorf("redirect target got %d requests", n)
				}
			})
		}, want: classPermanent, status: http.StatusTemporaryRedirect},
		{name: "400", program: respond(http.StatusBadRequest, `{"error":"`+canaryBody+`"}`), want: classPermanent, status: http.StatusBadRequest},
		{name: "401", program: respond(http.StatusUnauthorized, `{"error":"`+canaryBody+`"}`), want: classPermanent, status: http.StatusUnauthorized},
		{name: "408", program: respond(http.StatusRequestTimeout, `{"error":"`+canaryBody+`"}`), want: classTransient, status: http.StatusRequestTimeout},
		{name: "429", program: respond(http.StatusTooManyRequests, `{"error":"`+canaryBody+`"}`), want: classTransient, status: http.StatusTooManyRequests},
		{name: "500", program: respond(http.StatusInternalServerError, `{"error":"`+canaryBody+`"}`), want: classTransient, status: http.StatusInternalServerError},
		{name: "503", program: respond(http.StatusServiceUnavailable, `{"error":"`+canaryBody+`"}`), want: classTransient, status: http.StatusServiceUnavailable},
		{name: "body over egress.max_response_bytes", program: respond(http.StatusOK, `{"text":"`+canaryText+`","pad":"`+padding+`"}`), want: classPermanent, status: http.StatusOK},
		// A short voice.transcription_timeout stands in for the configured one; the fake never
		// answers, so the client gives up after it.
		{name: "hang beyond the timeout", program: func(_ *testing.T, s setup) { s.fake.Hang() }, timeout: 250 * time.Millisecond, want: classTransient},
	}
}

// classify maps the result of a call to its class.
func classify(err error) class {
	switch {
	case err == nil:
		return classOK
	case errors.Is(err, app.ErrTranscriptionPermanent):
		return classPermanent
	default:
		return classTransient
	}
}

// warnLines returns the Warn records of the adapter in the captured log.
func warnLines(t *testing.T, logs string) []map[string]any {
	t.Helper()
	var out []map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(logs), "\n") {
		if line == "" {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("log line %q: %v", line, err)
		}
		if rec["msg"] == "transcription failed" {
			out = append(out, rec)
		}
	}
	return out
}

// TestOpenAICompatible_ErrorClasses (design note 11 d): timeouts, connection errors, 408, 429 and
// 5xx are transient; a 3xx (never followed), every other 4xx, a body over
// egress.max_response_bytes and a malformed or invalid body are permanent. Every failed call
// writes exactly one Warn line with bothy.file.id, the class and the status code.
func TestOpenAICompatible_ErrorClasses(t *testing.T) {
	t.Parallel()
	for _, row := range errorRows() {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			s := newSetup(t, options{timeout: row.timeout})
			row.program(t, s)
			res, err := s.call(t, "")
			if got := classify(err); got != row.want {
				t.Fatalf("class %s (err %v), want %s", got, err, row.want)
			}
			// A hanging service may lose the race to record the request before the client gives
			// up on a loaded machine; the class is what the row proves, so at most one request.
			if n := len(s.fake.Requests()); n != 1 && (row.timeout == 0 || n > 1) {
				t.Errorf("%d requests, want 1", n)
			}
			lines := warnLines(t, s.logs.String())
			if row.want == classOK {
				if len(lines) != 0 || res.Text == "" {
					t.Errorf("success: result %+v, Warn lines %v", res, lines)
				}
				return
			}
			if len(lines) != 1 {
				t.Fatalf("%d Warn lines, want 1:\n%s", len(lines), s.logs.String())
			}
			l := lines[0]
			if l["level"] != "WARN" || l["bothy.reason"] != string(row.want) || l["bothy.file.id"] == nil {
				t.Errorf("Warn line %v", l)
			}
			code, ok := l["http.response.status_code"].(float64)
			if row.status == 0 && ok || row.status != 0 && int(code) != row.status {
				t.Errorf("Warn line status %v, want %d", l["http.response.status_code"], row.status)
			}
			for k := range l {
				if !slices.Contains([]string{"time", "level", "msg", "bothy.file.id", "bothy.reason", "http.response.status_code"}, k) {
					t.Errorf("Warn line has attribute %s", k)
				}
			}
		})
	}
}

// TestOpenAICompatible_OnlyConfiguredHost (SEC-T09 part transcription, SEC-NET-2): a URL whose
// loopback address has no entry in egress.allowed_hosts is blocked before any connection; the
// failure is permanent and the fake records no request.
func TestOpenAICompatible_OnlyConfiguredHost(t *testing.T) {
	t.Parallel()
	for name, o := range map[string]func(fake string) options{
		"no entry":              func(string) options { return options{rules: []string{}} },
		"entry of another port": func(string) options { return options{rules: []string{"127.0.0.1:1;allow_private"}} },
		"host name of the address": func(u string) options {
			return options{url: strings.Replace(u, "127.0.0.1", "localhost", 1)}
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			probe := transcriptiontest.New(t)
			s := newSetup(t, o(probe.URL()))
			_, err := s.call(t, "")
			if !errors.Is(err, egress.ErrBlocked) || classify(err) != classPermanent {
				t.Fatalf("err %v, want a permanent egress block", err)
			}
			if n := len(s.fake.Requests()) + len(probe.Requests()); n != 0 {
				t.Errorf("%d requests reached a server", n)
			}
		})
	}
}

// TestOpenAICompatible_LogsNoContent (SEC-LOG-4, 12 14): for every row of the error table, with a
// key, a model and a hint, the captured logs and the error text contain neither the transcript,
// the response body, the key nor the URL path.
func TestOpenAICompatible_LogsNoContent(t *testing.T) {
	t.Parallel()
	for _, row := range errorRows() {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			s := newSetup(t, options{timeout: row.timeout, withKey: true, cfg: transcription.Config{Model: "whisper-1"}})
			row.program(t, s)
			_, err := s.call(t, "de")
			logs := s.logs.String()
			forbidden := []string{canaryText, canaryBody, testKey, transcriptiontest.Path, "transcriptions", "audio.m4a"}
			noneOf(t, "log", logs, forbidden...)
			if err != nil {
				noneOf(t, "error", err.Error(), forbidden...)
			}
			if row.status != 0 && !strings.Contains(logs, strconv.Itoa(row.status)) {
				t.Errorf("log lacks the status code %d", row.status)
			}
		})
	}
}
