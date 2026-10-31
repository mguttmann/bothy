// SPDX-License-Identifier: Apache-2.0

package voice_test

import (
	"bytes"
	"image"
	"image/png"
	"net/http"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/files/adapters/filestest"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// windowsM4A is a recording as the app writes it on Windows (FIX-voice-send): AAC in MP4 with
// the major brand mp42 and the compatible brands mp41 and isom, which the type detection names
// video/mp4; one sound track and an mvhd duration of 4.5 seconds.
func windowsM4A(opts filestest.AudioOptions) []byte {
	opts.Brand, opts.Compatible = "mp42", []string{"mp41", "isom"}
	return filestest.M4A(1000, 4500, opts)
}

// mimeOf reads files.mime_type of the file.
func (w *world) mimeOf(fileID string) string {
	w.t.Helper()
	var m string
	if err := w.db.QueryRow(w.t.Context(), `SELECT coalesce(mime_type, '') FROM files WHERE id = $1`, fileID).Scan(&m); err != nil {
		w.t.Fatal(err)
	}
	return m
}

// wantNotReady fails unless r is 409 file_not_ready with Retry-After (12 8.1 rule 1, 9.3).
func wantNotReady(t *testing.T, what string, r harness.Response) {
	t.Helper()
	if r.Status != http.StatusConflict || problemOf(t, r).Code != "file_not_ready" || r.Header.Get("Retry-After") == "" {
		t.Fatalf("%s: %d %v %s, want 409 file_not_ready with Retry-After", what, r.Status, r.Header, r.Body)
	}
}

// TestVoice_GenericBrandFromWindows (FIX-voice-send, live report of 2026-10-31): the recording
// of the Windows app, declared audio/mp4, is a voice message. Sent right after completeUpload it
// answers 409 file_not_ready, also after the scan while the container phase has not read its
// duration; after files.thumbnail the row is audio/mp4, complete with duration_ms, and the same
// request answers 201. Before the fix the scan stored video/mp4 and the send answered 400
// voice_format_unsupported.
func TestVoice_GenericBrandFromWindows(t *testing.T) {
	t.Parallel()
	w := newWorld(t, worldOptions{})
	f := w.uploadAs(w.alex, w.allgemein, "voice-1.m4a", "audio/mp4", windowsM4A(filestest.AudioOptions{}))
	wantNotReady(t, "before the scan", w.send(w.alex, w.main, "voice", "", f))

	w.mustScan(f)
	if got, mime := w.file(f), w.mimeOf(f); mime != "audio/mp4" || got.Upload != "pending" || got.DurationMS != 0 {
		t.Fatalf("after the scan: %s %+v, want audio/mp4 pending without a duration", mime, got)
	}
	wantNotReady(t, "after the scan, before the container phase", w.send(w.alex, w.main, "voice", "", f))

	w.mustThumbnail(f)
	if got := w.file(f); got.Upload != "complete" || got.DurationMS != 4500 {
		t.Fatalf("after the container phase: %+v", got)
	}
	r := w.send(w.alex, w.main, "voice", "", f)
	if r.Status != http.StatusCreated {
		t.Fatalf("createMessage voice with the Windows recording: %d %s", r.Status, r.Body)
	}
	var m message
	r.JSON(t, &m)
	if m.Kind != "voice" || len(m.Attachments) != 1 || m.Attachments[0].File.DurationMS == nil || *m.Attachments[0].File.DurationMS != 4500 {
		t.Fatalf("voice message %s", r.Body)
	}
}

// TestVoice_GenericBrandWithVideoTrack (FIX-voice-send): the same layout with a video track,
// declared audio/mp4, is still no voice recording (12 8.1 rule 4) and passes as a text attachment.
func TestVoice_GenericBrandWithVideoTrack(t *testing.T) {
	t.Parallel()
	w := newWorld(t, worldOptions{})
	body := windowsM4A(filestest.AudioOptions{VideoTrack: true})
	f := w.uploadAs(w.alex, w.allgemein, "clip.m4a", "audio/mp4", body)
	w.mustScan(f)
	w.mustThumbnail(f)
	wantField(t, "a video track", w.send(w.alex, w.main, "voice", "", f), "/attachment_file_ids/0", "voice_format_unsupported")
	if r := w.send(w.alex, w.main, "", "the clip", f); r.Status != http.StatusCreated {
		t.Fatalf("as a text attachment: %d %s", r.Status, r.Body)
	}
	// Without the declaration the detected type stays video/mp4.
	v := w.uploadAs(w.alex, w.allgemein, "clip.mp4", "video/mp4", body)
	w.mustScan(v)
	if mime := w.mimeOf(v); mime != "video/mp4" {
		t.Fatalf("declared video/mp4 stored as %s", mime)
	}
}

// TestAttachments_PendingFilesAreAttachable (FIX-voice-send, the same class for images and files,
// M5-16 and M4-47): a text message with an image and a file whose scan has not ended is stored at
// once (12 10 rule 1); only a voice message waits for its file.
func TestAttachments_PendingFilesAreAttachable(t *testing.T) {
	t.Parallel()
	w := newWorld(t, worldOptions{})
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	var pngBody bytes.Buffer
	if err := png.Encode(&pngBody, img); err != nil {
		t.Fatal(err)
	}
	a := w.uploadAs(w.alex, w.allgemein, "photo-1.png", "image/png", pngBody.Bytes())
	b := w.uploadAs(w.alex, w.allgemein, "photo-2.png", "image/png", append([]byte(nil), pngBody.Bytes()...))
	c := w.uploadAs(w.alex, w.allgemein, "pasted.txt", "text/plain", []byte("pasted text\n"))
	r := w.send(w.alex, w.main, "", "", a, b, c)
	if r.Status != http.StatusCreated {
		t.Fatalf("text message with files being checked: %d %s", r.Status, r.Body)
	}
	var m message
	r.JSON(t, &m)
	if len(m.Attachments) != 3 {
		t.Fatalf("attachments %s", r.Body)
	}
}
