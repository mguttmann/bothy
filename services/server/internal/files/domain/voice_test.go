// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"bytes"
	"regexp"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/filestest"
	"github.com/OWNER/bothy/services/server/internal/files/adapters/mimetype"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

// TestIsVoiceFormat pins the detected names of the generated fixtures (brief M3-09 design note
// 4 a) and the voice formats per author kind.
func TestIsVoiceFormat(t *testing.T) {
	t.Parallel()
	det := mimetype.Detector{}
	m4a := det.Detect(filestest.M4A(1000, 4500, filestest.AudioOptions{}))
	m4b := det.Detect(filestest.M4A(1000, 4500, filestest.AudioOptions{Brand: "M4B "}))
	isom := det.Detect(filestest.M4A(1000, 4500, filestest.AudioOptions{Brand: "isom"}))
	mp3 := det.Detect(filestest.MP3(4))
	wav := det.Detect(filestest.WAV())
	if m4a != "audio/x-m4a" || m4b != "audio/mp4" || isom != "video/mp4" || mp3 != "audio/mpeg" || wav != "audio/wav" {
		t.Fatalf("detected names changed: m4a %q, m4b %q, isom %q, mp3 %q, wav %q", m4a, m4b, isom, mp3, wav)
	}
	cases := []struct {
		mime   string
		author string
		want   bool
	}{
		{m4a, "user", true}, {m4a, "agent", true}, {m4a, "app", true},
		{m4b, "user", true}, {m4b, "agent", true}, {m4b, "app", true},
		{mp3, "agent", true}, {mp3, "user", false}, {mp3, "app", false},
		{wav, "agent", false}, {"audio/ogg", "agent", false}, {"video/mp4", "user", false},
		{isom, "agent", false}, {"application/pdf", "user", false}, {"", "user", false},
	}
	for _, c := range cases {
		if got := domain.IsVoiceFormat(c.mime, c.author); got != c.want {
			t.Errorf("IsVoiceFormat(%q, %q) = %v, want %v", c.mime, c.author, got, c.want)
		}
	}
}

func TestDeferContainerComplete(t *testing.T) {
	t.Parallel()
	for mime, want := range map[string]bool{"audio/x-m4a": true, "audio/mp4": true, "video/mp4": false,
		"video/quicktime": false, "video/x-m4v": false, "image/png": false, "audio/mpeg": false} {
		if got := domain.DeferContainerComplete(mime); got != want {
			t.Errorf("DeferContainerComplete(%q) = %v, want %v", mime, got, want)
		}
	}
}

func TestVoiceCheck(t *testing.T) {
	t.Parallel()
	maxD := 15 * time.Minute
	ok := domain.VoiceFacts{UploadState: domain.UploadComplete, ScanStatus: domain.ScanClean, MimeType: "audio/x-m4a",
		AuthorKind: "user", DurationMs: 4500, HasDuration: true, MaxDuration: maxD}
	with := func(change func(*domain.VoiceFacts)) domain.VoiceFacts {
		f := ok
		change(&f)
		return f
	}
	cases := []struct {
		name string
		f    domain.VoiceFacts
		want domain.VoiceVerdict
	}{
		{"ok", ok, domain.VoiceOK},
		{"upload pending", with(func(f *domain.VoiceFacts) { f.UploadState = domain.UploadPending }), domain.VoiceNotReady},
		{"scan pending", with(func(f *domain.VoiceFacts) { f.ScanStatus = domain.ScanPending }), domain.VoiceNotReady},
		{"pending before wrong type", with(func(f *domain.VoiceFacts) { f.ScanStatus = domain.ScanPending; f.MimeType = "application/pdf" }), domain.VoiceNotReady},
		{"infected", with(func(f *domain.VoiceFacts) { f.ScanStatus = domain.ScanInfected }), domain.VoiceBlocked},
		{"error", with(func(f *domain.VoiceFacts) { f.ScanStatus = domain.ScanError }), domain.VoiceBlocked},
		{"failed upload", with(func(f *domain.VoiceFacts) { f.UploadState = domain.UploadFailed }), domain.VoiceBlocked},
		{"blocked before wrong type", with(func(f *domain.VoiceFacts) { f.ScanStatus = domain.ScanInfected; f.MimeType = "audio/wav" }), domain.VoiceBlocked},
		{"wrong type", with(func(f *domain.VoiceFacts) { f.MimeType = "audio/wav" }), domain.VoiceFormatUnsupported},
		{"mp3 from a person", with(func(f *domain.VoiceFacts) { f.MimeType = "audio/mpeg"; f.HasDuration = false }), domain.VoiceFormatUnsupported},
		{"mp4 with width", with(func(f *domain.VoiceFacts) { f.HasWidth = true }), domain.VoiceFormatUnsupported},
		{"mp4 with height", with(func(f *domain.VoiceFacts) { f.HasHeight = true }), domain.VoiceFormatUnsupported},
		{"mp4 without duration", with(func(f *domain.VoiceFacts) { f.HasDuration = false; f.DurationMs = 0 }), domain.VoiceFormatUnsupported},
		{"wrong type before too long", with(func(f *domain.VoiceFacts) { f.MimeType = "audio/wav"; f.DurationMs = maxD.Milliseconds() + 1 }), domain.VoiceFormatUnsupported},
		{"duration equal", with(func(f *domain.VoiceFacts) { f.DurationMs = maxD.Milliseconds() }), domain.VoiceOK},
		{"one millisecond above", with(func(f *domain.VoiceFacts) { f.DurationMs = maxD.Milliseconds() + 1 }), domain.VoiceTooLong},
		{"mp3 from an agent without duration", with(func(f *domain.VoiceFacts) {
			f.MimeType = "audio/mpeg"
			f.AuthorKind = "agent"
			f.HasDuration = false
			f.DurationMs = 0
		}), domain.VoiceOK},
		{"skipped scan", with(func(f *domain.VoiceFacts) { f.ScanStatus = domain.ScanSkipped }), domain.VoiceOK},
		{"deleted", with(func(f *domain.VoiceFacts) { f.Deleted = true }), domain.VoiceBlocked},
	}
	for _, c := range cases {
		if got := domain.VoiceCheck(c.f); got != c.want {
			t.Errorf("%s: VoiceCheck = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestInvocationFinal(t *testing.T) {
	t.Parallel()
	final := domain.StateFacts{UploadState: domain.UploadComplete, ScanStatus: domain.ScanClean}
	open := domain.StateFacts{UploadState: domain.UploadPending, ScanStatus: domain.ScanPending}
	for _, ts := range []string{"none", "pending", "done", "failed"} {
		f, o := final, open
		f.TranscriptStatus, o.TranscriptStatus = ts, ts
		if got, want := domain.InvocationFinal(f), ts != "pending"; got != want {
			t.Errorf("InvocationFinal(final, %s) = %v, want %v", ts, got, want)
		}
		if domain.InvocationFinal(o) {
			t.Errorf("InvocationFinal(open, %s) = true", ts)
		}
	}
}

// TestBecameFinalTranscript: every pair of Final times transcript status (brief M3-09 design
// note 10).
func TestBecameFinalTranscript(t *testing.T) {
	t.Parallel()
	states := []domain.StateFacts{}
	for _, base := range []domain.StateFacts{
		{UploadState: domain.UploadPending, ScanStatus: domain.ScanPending},
		{UploadState: domain.UploadComplete, ScanStatus: domain.ScanClean},
		{UploadState: domain.UploadComplete, ScanStatus: domain.ScanInfected},
	} {
		for _, ts := range []string{"none", "pending", "done", "failed"} {
			s := base
			s.TranscriptStatus = ts
			states = append(states, s)
		}
	}
	for _, b := range states {
		for _, a := range states {
			want := !domain.InvocationFinal(b) && domain.InvocationFinal(a)
			if got := domain.BecameFinal(b, a); got != want {
				t.Errorf("BecameFinal(%+v, %+v) = %v, want %v", b, a, got, want)
			}
		}
	}
	clean := domain.StateFacts{UploadState: domain.UploadComplete, ScanStatus: domain.ScanClean, TranscriptStatus: "pending"}
	done := clean
	done.TranscriptStatus = "done"
	if !domain.BecameFinal(clean, done) {
		t.Error("pending to done on a final file must publish")
	}
	none := clean
	none.TranscriptStatus = "none"
	if domain.BecameFinal(none, clean) {
		t.Error("setting pending must publish nothing")
	}
}

func TestTranscriptETag(t *testing.T) {
	t.Parallel()
	base := domain.TranscriptFacts{Status: "done", Source: "agent", Language: "en", Text: "hello"}
	e := domain.TranscriptETag(base)
	if e != domain.TranscriptETag(base) {
		t.Fatal("not stable")
	}
	if !regexp.MustCompile(`^"[A-Za-z0-9_-]{22}"$`).MatchString(e) {
		t.Fatalf("form %q", e)
	}
	for name, f := range map[string]domain.TranscriptFacts{
		"status":   {Status: "failed", Source: "agent", Language: "en", Text: "hello"},
		"source":   {Status: "done", Source: "service", Language: "en", Text: "hello"},
		"language": {Status: "done", Source: "agent", Language: "de", Text: "hello"},
		"text":     {Status: "done", Source: "agent", Language: "en", Text: "hello!"},
	} {
		if domain.TranscriptETag(f) == e {
			t.Errorf("%s does not change the ETag", name)
		}
	}
	a := domain.TranscriptETag(domain.TranscriptFacts{Status: "done", Source: "a", Language: "bc"})
	b := domain.TranscriptETag(domain.TranscriptFacts{Status: "done", Source: "ab", Language: "c"})
	if a == b {
		t.Error("the separator does not keep fields apart")
	}
	if bytes.Equal([]byte(a), []byte(b)) {
		t.Error("unreachable")
	}
}

func TestLanguageHint(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{"en-GB": "en", "de": "de", "DE-at": "de", "": "", "x": "", "english": "",
		"en_GB": "", "en-toolongsubtag": "", "fil-PH": "fil"} {
		if got := domain.LanguageHint(in); got != want {
			t.Errorf("LanguageHint(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidLanguageTag(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]bool{"en-GB": true, "de": true, "zh-Hant-TW": true, "fil": true, "es-419": true,
		"": false, "x": false, "english": false, "en_GB": false, "en-toolongsubtag": false, "en-": false, "-en": false,
		"e1": false, "en--GB": false, "en-G": false} {
		if got := domain.ValidLanguageTag(in); got != want {
			t.Errorf("ValidLanguageTag(%q) = %v, want %v", in, got, want)
		}
	}
}
