// SPDX-License-Identifier: Apache-2.0

package voice_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/filestest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// threadSeq reads the last sequence number of a thread, so a refused send is seen to give it back.
func (w *world) threadSeq(thread string) int64 {
	return w.count(`SELECT coalesce(max(thread_seq), 0) FROM messages WHERE thread_id = $1`, thread)
}

// TestVoice_KindVoiceEnabled (plan; replaces the voice row of M1-56's not_enabled test): a voice
// message with a ready .m4a below voice.max_duration is stored with kind voice and the duration
// from mvhd, through createMessage and through createThread with first_message.
func TestVoice_KindVoiceEnabled(t *testing.T) {
	t.Parallel()
	w := newWorld(t, worldOptions{})
	f := w.m4a(4500 * time.Millisecond)
	r := w.send(w.alex, w.main, "voice", "", f)
	if r.Status != http.StatusCreated {
		t.Fatalf("createMessage voice: %d %s", r.Status, r.Body)
	}
	noNull(t, "createMessage", r.Body)
	var m message
	r.JSON(t, &m)
	if m.Kind != "voice" || len(m.Attachments) != 1 || m.Attachments[0].File.DurationMS == nil || *m.Attachments[0].File.DurationMS != 4500 ||
		m.Attachments[0].File.TranscriptStatus != "none" {
		t.Fatalf("voice message %s", r.Body)
	}
	f2 := w.m4a(3 * time.Second)
	tr := w.createThread(map[string]any{"client_msg_id": ids.New().String(), "kind": "voice", "content_md": "", "attachment_file_ids": []string{f2}})
	if tr.Status != http.StatusCreated {
		t.Fatalf("createThread with a voice first_message: %d %s", tr.Status, tr.Body)
	}
	if n := w.count(`SELECT count(*) FROM messages WHERE kind = 'voice'`); n != 2 {
		t.Fatalf("%d voice messages stored, want 2", n)
	}
}

// TestVoice_FormatUnsupported400 (plan): WAV, MP3 and PDF from a person, an MP4 with a video track
// and one without mvhd give voice_format_unsupported at /attachment_file_ids/0 and store nothing;
// as text attachments they pass; Lotse's MP3 passes without duration_ms.
func TestVoice_FormatUnsupported400(t *testing.T) {
	t.Parallel()
	w := newWorld(t, worldOptions{})
	bodies := map[string][]byte{
		"note.wav": filestest.WAV(), "note.mp3": filestest.MP3(8), "note.pdf": []byte("%PDF-1.4\n%voice\n1 0 obj<<>>endobj\ntrailer<<>>\n%%EOF\n"),
		"video.m4a":  filestest.M4A(1000, 3000, filestest.AudioOptions{VideoTrack: true}),
		"nomvhd.m4a": filestest.M4A(1000, 3000, filestest.AudioOptions{NoMovieHeader: true}),
	}
	for name, body := range bodies {
		f := w.ready(w.alex, w.allgemein, name, body)
		seq := w.threadSeq(w.main)
		msgs := w.count(`SELECT count(*) FROM messages`)
		wantField(t, name, w.send(w.alex, w.main, "voice", "", f), "/attachment_file_ids/0", "voice_format_unsupported")
		if n := w.count(`SELECT count(*) FROM messages`); n != msgs || w.threadSeq(w.main) != seq ||
			w.count(`SELECT count(*) FROM message_attachments WHERE file_id = $1`, f) != 0 {
			t.Errorf("%s: something was stored", name)
		}
		if r := w.send(w.alex, w.main, "", "the file "+name, f); r.Status != http.StatusCreated {
			t.Errorf("%s as a text attachment: %d %s", name, r.Status, r.Body)
		}
	}
	// Lotse sends an MP3 voice message in its own thread (12 8.1: MP3 for agents).
	thread := w.lotseThread()
	mp3 := w.ready(w.lotse, w.allgemein, "answer.mp3", filestest.MP3(8))
	r := w.send(w.lotse, thread, "voice", "", mp3)
	if r.Status != http.StatusCreated {
		t.Fatalf("Lotse's MP3 voice message: %d %s", r.Status, r.Body)
	}
	var m message
	r.JSON(t, &m)
	if len(m.Attachments) != 1 || m.Attachments[0].File.DurationMS != nil {
		t.Fatalf("MP3 voice message %s", r.Body)
	}
}

// createThread creates a thread of Alex in Support/Allgemein under a new parent message with
// first as its first message.
func (w *world) createThread(first map[string]any) harness.Response {
	w.t.Helper()
	parent := w.send(w.alex, w.main, "", "Question")
	if parent.Status != http.StatusCreated {
		w.t.Fatalf("parent message: %d %s", parent.Status, parent.Body)
	}
	var p message
	parent.JSON(w.t, &p)
	r, err := harness.DoWithStepUp(w.t.Context(), w.alex, harness.Request{Method: http.MethodPost, Path: "/channels/" + w.allgemein + "/threads",
		Body: map[string]any{"parent_message_id": p.ID, "first_message": first}})
	if err != nil {
		w.t.Fatal(err)
	}
	return r
}

// lotseThread opens a thread with Lotse as its operator through Alex's mention (auto_thread) and
// returns it, so Lotse may write in it.
func (w *world) lotseThread() string {
	w.t.Helper()
	r := w.send(w.alex, w.main, "", "@lotse hello")
	if r.Status != http.StatusCreated {
		w.t.Fatalf("mention: %d %s", r.Status, r.Body)
	}
	var m message
	r.JSON(w.t, &m)
	return w.runOf(m.ID).Thread
}

// TestVoice_TooLong400: one second more than voice.max_duration gives voice_too_long; exactly the
// maximum passes.
func TestVoice_TooLong400(t *testing.T) {
	t.Parallel()
	w := newWorld(t, worldOptions{})
	wantField(t, "too long", w.send(w.alex, w.main, "voice", "", w.m4a(maxDuration+time.Second)), "/attachment_file_ids/0", "voice_too_long")
	if r := w.send(w.alex, w.main, "voice", "", w.m4a(maxDuration)); r.Status != http.StatusCreated {
		t.Fatalf("exactly voice.max_duration: %d %s", r.Status, r.Body)
	}
}

// TestVoice_BodyRules: voice without an attachment and with two gives the codes of design note 7.
func TestVoice_BodyRules(t *testing.T) {
	t.Parallel()
	w := newWorld(t, worldOptions{})
	wantField(t, "no attachment", w.send(w.alex, w.main, "voice", "hello"), "/attachment_file_ids", "required")
	wantField(t, "two attachments", w.send(w.alex, w.main, "voice", "", w.m4a(time.Second), w.m4a(2*time.Second)),
		"/attachment_file_ids", "limit_reached")
	r := w.createThread(map[string]any{"client_msg_id": ids.New().String(), "kind": "voice", "content_md": "x"})
	wantField(t, "createThread without an attachment", r, "/first_message/attachment_file_ids", "required")
}

// TestAudioMP4CompleteAfterContainerPhase (design note 5): after the scan the .m4a is pending with
// files.thumbnail queued; the container phase completes it with duration_ms in one transaction
// with one file.updated; a broken box tree ends complete without duration_ms.
func TestAudioMP4CompleteAfterContainerPhase(t *testing.T) {
	t.Parallel()
	w := newWorld(t, worldOptions{})
	f := w.upload(w.alex, w.allgemein, "note.m4a", filestest.M4A(1000, 4500, filestest.AudioOptions{}))
	w.mustScan(f)
	if got := w.file(f); got.Upload != "pending" || got.Scan != "skipped" || got.DurationMS != 0 {
		t.Fatalf("after the scan: %+v", got)
	}
	if n := w.count(`SELECT count(*) FROM jobs.river_job WHERE kind = 'files.thumbnail' AND args->>'file_id' = $1`, f); n != 1 {
		t.Fatalf("%d files.thumbnail jobs", n)
	}
	before := len(w.fileUpdated(f))
	w.mustThumbnail(f)
	if got := w.file(f); got.Upload != "complete" || got.DurationMS != 4500 {
		t.Fatalf("after the container phase: %+v", got)
	}
	if n := len(w.fileUpdated(f)) - before; n != 1 {
		t.Fatalf("%d file.updated for the container phase, want 1", n)
	}
	w.mustThumbnail(f)
	if n := len(w.fileUpdated(f)) - before; n != 1 {
		t.Fatalf("a repeated run wrote %d file.updated", n-1)
	}
	broken := w.upload(w.alex, w.allgemein, "broken.m4a", filestest.M4A(1000, 4500, filestest.AudioOptions{Broken: true}))
	w.mustScan(broken)
	w.mustThumbnail(broken)
	if got := w.file(broken); got.Upload != "complete" || got.DurationMS != 0 {
		t.Fatalf("a broken box tree: %+v", got)
	}
}
