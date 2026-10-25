// SPDX-License-Identifier: Apache-2.0

package transcription_test

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/transcription"
	"github.com/OWNER/bothy/services/server/internal/files/adapters/transcription/transcriptiontest"
	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// Small limits of the tests, standing in for message.max_content_bytes and
// egress.max_response_bytes.
const (
	testMaxTextBytes     = 64
	testMaxResponseBytes = 4096
)

// testKey is the API key the tests store in a secret file; it must never appear in a log.
const testKey = "sk-test-transcription-key-0123456789"

// testAudio is the audio content of every request; tests generate it, it is no real recording.
var testAudio = bytes.Repeat([]byte("m4a-audio-bytes-"), 64)

// logBuffer is a concurrency-safe log sink for the captured logs of the adapter.
type logBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *logBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *logBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

// setup is the adapter under test with its fake service and captured log.
type setup struct {
	adapter *transcription.OpenAICompatible
	fake    *transcriptiontest.Server
	logs    *logBuffer
	keyRef  string
}

// options change the defaults of newSetup.
type options struct {
	cfg     transcription.Config
	timeout time.Duration
	rules   []string // nil: the rule of the fake; set: exactly these entries
	url     string   // "": the URL of the fake
	withKey bool
}

// newSetup builds the adapter over a real egress client and a real secret provider whose key
// file lies in t.TempDir().
func newSetup(t *testing.T, o options) setup {
	t.Helper()
	fake := transcriptiontest.New(t)
	s := setup{fake: fake, logs: &logBuffer{}}
	logger := slog.New(slog.NewJSONHandler(s.logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	dir := t.TempDir()
	if o.withKey {
		if err := os.WriteFile(filepath.Join(dir, "transcription_key"), []byte(testKey), 0o600); err != nil {
			t.Fatal(err)
		}
		s.keyRef = "file:<secrets.dir>/transcription_key"
	}
	sp, err := secrets.NewProvider(secrets.Options{Dir: dir, CacheTTL: 0, Logger: logger})
	if err != nil {
		t.Fatal(err)
	}
	cfg := o.cfg
	if o.url == "" {
		o.url = fake.URL()
	}
	cfg.URL = o.url
	cfg.BearerRef = s.keyRef
	if cfg.MaxTextBytes == 0 {
		cfg.MaxTextBytes = testMaxTextBytes
	}
	rules := o.rules
	if rules == nil {
		rules = []string{fake.Rule()}
	}
	var parsed []egress.Rule
	for _, r := range rules {
		pr, err := egress.ParseRule(r)
		if err != nil {
			t.Fatal(err)
		}
		parsed = append(parsed, pr)
	}
	timeout := o.timeout
	if timeout == 0 {
		timeout = testkit.Patience
	}
	hosts, err := transcription.HostPolicy(cfg.URL)
	if err != nil {
		t.Fatal(err)
	}
	client, err := egress.New(egress.Config{AllowedHosts: parsed, Timeout: timeout, MaxResponseBytes: testMaxResponseBytes},
		egress.Options{Purpose: "transcription", Hosts: hosts, Redirects: egress.RedirectNever, Timeout: timeout},
		egress.Deps{Logger: logger})
	if err != nil {
		t.Fatal(err)
	}
	s.adapter, err = transcription.NewOpenAICompatible(cfg, transcription.Deps{Client: client, Secrets: sp, Logger: logger})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// call transcribes testAudio as audio/mp4 with the language hint.
func (s setup) call(t *testing.T, hint string) (app.TranscribeResult, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(testkit.Context(t), testkit.Patience)
	defer cancel()
	return s.adapter.Transcribe(ctx, app.TranscribeRequest{FileID: ids.New(), MIMEType: "audio/mp4",
		Audio: bytes.NewReader(testAudio), LanguageHint: hint})
}

// noneOf fails when text contains one of the values.
func noneOf(t *testing.T, what, text string, values ...string) {
	t.Helper()
	for _, v := range values {
		if v != "" && strings.Contains(text, v) {
			t.Errorf("%s contains %q:\n%s", what, v, text)
		}
	}
}
