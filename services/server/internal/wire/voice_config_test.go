// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/transcription/transcriptiontest"
	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// TestVoiceConfigKeys (M3-09 design note 16): every voice key with the default of the table of
// docs/spec/12-files.md section 18; the cross-check voice_service_url refuses mode service with an
// empty URL, a relative URL, a scheme other than http and https, user info and a fragment, and
// its problem names keys only.
func TestVoiceConfigKeys(t *testing.T) {
	t.Parallel()
	got := map[string]config.Key{}
	for _, k := range schema(t).Keys() {
		got[k.Name] = k
	}
	const ch = "12-files.md"
	want := map[string]config.Key{
		"voice.max_duration":               {Kind: config.KindDuration, Default: "15m", Min: "1s", Chapter: ch},
		"voice.transcription_mode":         {Kind: config.KindEnum, Default: "agent", Enum: []string{"off", "agent", "service"}, Chapter: ch},
		"voice.transcription_url":          {Kind: config.KindString, Default: "", Chapter: ch},
		"voice.transcription_api_key_ref":  {Kind: config.KindSecretRef, Default: "", Chapter: ch},
		"voice.transcription_timeout":      {Kind: config.KindDuration, Default: "120s", Min: "1s", Chapter: ch},
		"voice.transcript_hold_max":        {Kind: config.KindDuration, Default: "60s", Min: "1s", Chapter: ch},
		"voice.transcription_model":        {Kind: config.KindString, Default: "", Chapter: ch},
		"voice.transcription_max_attempts": {Kind: config.KindInt, Default: "5", Min: "1", Chapter: ch},
	}
	for name, w := range want {
		g, ok := got[name]
		if !ok {
			t.Errorf("key %s missing", name)
			continue
		}
		if g.Doc == "" || !strings.HasSuffix(g.Doc, ".") {
			t.Errorf("%s: Doc %q is not one sentence", name, g.Doc)
		}
		g.Name, g.Doc = "", ""
		if !reflect.DeepEqual(g, w) {
			t.Errorf("%s = %+v, want %+v", name, g, w)
		}
	}

	const mode = "BOTHY_VOICE__TRANSCRIPTION_MODE=service"
	for name, c := range map[string]struct{ url, secret string }{
		"empty":     {"", ""},
		"relative":  {"/v1/audio/transcriptions", "/v1/audio/transcriptions"},
		"ftp":       {"ftp://stt.example.com/v1", "stt.example.com"},
		"user info": {"https://alex:hunter2@stt.example.com/v1", "hunter2"},
		"fragment":  {"https://stt.example.com/v1#part", "#part"},
	} {
		problems := load(t, mode, "BOTHY_VOICE__TRANSCRIPTION_URL="+c.url)
		if len(problems) != 1 || !strings.Contains(problems[0].String(), ": voice_service_url: ") ||
			!strings.Contains(problems[0].String(), "voice.transcription_url") {
			t.Errorf("%s: problems %v, want voice_service_url naming voice.transcription_url", name, problems)
			continue
		}
		if c.secret != "" {
			noValues(t, problems, c.secret)
		}
	}
	for name, env := range map[string][]string{
		"service with https":     {mode, "BOTHY_VOICE__TRANSCRIPTION_URL=https://stt.example.com/v1/audio/transcriptions"},
		"service with http":      {mode, "BOTHY_VOICE__TRANSCRIPTION_URL=http://stt.example.com:9000/v1/audio/transcriptions"},
		"agent without url":      {"BOTHY_VOICE__TRANSCRIPTION_MODE=agent"},
		"off with any url":       {"BOTHY_VOICE__TRANSCRIPTION_MODE=off", "BOTHY_VOICE__TRANSCRIPTION_URL=ftp://stt.example.com"},
		"default mode and model": {"BOTHY_VOICE__TRANSCRIPTION_MODEL=whisper-1"},
	} {
		if got := load(t, env...); got != nil {
			t.Errorf("%s: problems %v, want none", name, got)
		}
	}
}

// TestFilesTranscriber: outside mode service and in the role api there is no transcriber; in
// mode service the worker gets the adapter over an egress client for voice.transcription_url,
// which reaches the fake service with the bearer key of the secret file; a key reference that
// cannot be read stops the start and names the key, never the reference.
func TestFilesTranscriber(t *testing.T) {
	t.Parallel()
	fake := transcriptiontest.New(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "stt_key"), []byte("sk-wire-test-key"), 0o600); err != nil {
		t.Fatal(err)
	}
	sp, err := secrets.NewProvider(secrets.Options{Dir: dir, CacheTTL: 0})
	if err != nil {
		t.Fatal(err)
	}
	deps := wire.TranscriptionDeps{Secrets: sp, Egress: egress.Deps{}}
	serviceWithKey := func(file string) []string {
		return []string{"BOTHY_VOICE__TRANSCRIPTION_MODE=service", "BOTHY_VOICE__TRANSCRIPTION_URL=" + fake.URL(),
			"BOTHY_EGRESS__ALLOWED_HOSTS=" + fake.Rule(), "BOTHY_VOICE__TRANSCRIPTION_MODEL=whisper-1",
			"BOTHY_VOICE__TRANSCRIPTION_API_KEY_REF=file:" + filepath.Join(dir, file)}
	}
	service := serviceWithKey("stt_key")
	ctx := testkit.Context(t)

	for name, c := range map[string]struct {
		env  []string
		role wire.Role
	}{
		"default mode agent": {nil, wire.RoleAll},
		"mode off":           {[]string{"BOTHY_VOICE__TRANSCRIPTION_MODE=off"}, wire.RoleWorker},
		"service in api":     {service, wire.RoleAPI},
	} {
		tr, err := wire.FilesTranscriber(ctx, loadEnv(t, c.env), c.role, deps)
		if err != nil || tr != nil {
			t.Errorf("%s: transcriber %v, err %v, want none", name, tr, err)
		}
	}

	for _, role := range []wire.Role{wire.RoleWorker, wire.RoleAll} {
		tr, err := wire.FilesTranscriber(ctx, loadEnv(t, service), role, deps)
		if err != nil || tr == nil {
			t.Fatalf("%s: transcriber %v, err %v", role, tr, err)
		}
		res, err := tr.Transcribe(ctx, app.TranscribeRequest{FileID: ids.New(), MIMEType: "audio/mp4", Audio: bytes.NewReader([]byte("audio"))})
		if err != nil || res.Text != transcriptiontest.DefaultText {
			t.Fatalf("%s: Transcribe = %+v, %v", role, res, err)
		}
	}
	for _, r := range fake.Requests() {
		if r.Authorization != "Bearer sk-wire-test-key" || r.Fields["model"] != "whisper-1" {
			t.Errorf("request fields %v, bearer key sent %t", r.Fields, r.Authorization == "Bearer sk-wire-test-key")
		}
	}

	_, err = wire.FilesTranscriber(ctx, loadEnv(t, serviceWithKey("absent_key")), wire.RoleWorker, deps)
	if err == nil || !strings.Contains(err.Error(), "voice.transcription_api_key_ref") || strings.Contains(err.Error(), "absent_key") {
		t.Errorf("missing key: err %v, want one naming the key only", err)
	}
}
