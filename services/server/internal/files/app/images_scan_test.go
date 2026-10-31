// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

// fixedType detects one media type for every input.
type fixedType string

func (f fixedType) Detect([]byte) string { return string(f) }

// TestScanPassQueuesThumbnail (brief M3-06 design note 1): a clean or skipped JPEG, PNG, GIF or
// WebP stays pending with files.thumbnail queued and without FileFinalized; an MP4 video or
// QuickTime file is complete and gets the job; MP4 audio stays pending for the container phase
// (M3-09 design note 5); other types and blocked files get none.
func TestScanPassQueuesThumbnail(t *testing.T) {
	t.Parallel()
	content := []byte("bytes of a file of Alex")
	for _, c := range []struct {
		mime     string
		verdict  domain.VerdictKind
		state    string
		job, fin bool
	}{
		{"image/jpeg", domain.VerdictClean, "pending", true, false},
		{"image/png", domain.VerdictClean, "pending", true, false},
		{"image/apng", domain.VerdictClean, "pending", true, false},
		{"image/gif", domain.VerdictClean, "pending", true, false},
		{"image/webp", domain.VerdictClean, "pending", true, false},
		{"video/mp4", domain.VerdictClean, "complete", true, true},
		{"video/quicktime", domain.VerdictClean, "complete", true, true},
		{"audio/mp4", domain.VerdictClean, "pending", true, false},
		{"audio/x-m4a", domain.VerdictClean, "pending", true, false},
		{"image/svg+xml", domain.VerdictClean, "complete", false, true},
		{"image/heic", domain.VerdictClean, "complete", false, true},
		{"application/pdf", domain.VerdictClean, "complete", false, true},
		{"image/jpeg", domain.VerdictInfected, "complete", false, true},
		{"image/jpeg", domain.VerdictError, "complete", false, true},
	} {
		e := newScanEnvTypes(t, scanCfg, fixedType(c.mime))
		e.scanner.verdict = domain.Verdict{Kind: c.verdict, Signature: "Local.Test.Sig", Reason: domain.ReasonMalwareDetected}
		s := e.seed(t, content, content, true)
		if _, err := e.pass.Run(context.Background(), s.FileID, 0); err != nil {
			t.Fatal(err)
		}
		f := e.file(s.FileID)
		jobs := e.queue.thumbnails()
		if f.UploadState != c.state || (len(jobs) == 1) != c.job || len(jobs) > 1 || (e.finalCount(s.FileID) == 1) != c.fin {
			t.Errorf("%s %s: state %s, jobs %d, finalized %d", c.mime, c.verdict, f.UploadState, len(jobs), e.finalCount(s.FileID))
		}
	}
	// skipped takes the same path as clean.
	cfg := scanCfg
	cfg.Mode = app.ScanModeOff
	e := newScanEnvTypes(t, cfg, fixedType("image/jpeg"))
	s := e.seed(t, content, content, true)
	if _, err := e.pass.Run(context.Background(), s.FileID, 0); err != nil {
		t.Fatal(err)
	}
	if f := e.file(s.FileID); f.UploadState != "pending" || f.ScanStatus != "skipped" || len(e.queue.thumbnails()) != 1 {
		t.Errorf("skipped JPEG: %+v", f)
	}
}

// TestScanConcurrentRunsKeepImageOriginal: of two concurrent scans of a clean JPEG, the losing run
// keeps the original that the winner left pending for files.thumbnail (review finding of M3-06).
func TestScanConcurrentRunsKeepImageOriginal(t *testing.T) {
	t.Parallel()
	e := newScanEnvTypes(t, scanCfg, fixedType("image/jpeg"))
	s := e.seed(t, []byte("0123456789"), []byte("0123456789"), false)
	var arrived sync.WaitGroup
	arrived.Add(2)
	e.scanner.during = func() {
		arrived.Done()
		arrived.Wait()
	}
	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() {
			if _, err := e.pass.Run(context.Background(), s.FileID, 0); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if f := e.file(s.FileID); f.UploadState != "pending" || f.ScanStatus != "clean" || len(e.queue.thumbnails()) != 1 {
		t.Fatalf("file %+v, jobs %d", f, len(e.queue.thumbnails()))
	}
	if !e.store.has(s.File.ObjectKey) {
		t.Error("the losing run deleted the original awaiting files.thumbnail")
	}
}

// TestScanPassRefinesDeclaredMP4Audio (FIX-voice-send): an MP4 the detection names video/mp4 (a
// generic brand such as the mp42 of the Windows recorder) that the uploader declared as MP4 audio
// is stored as audio/mp4 and stays pending for the container phase; declared as video, or without
// an audio declaration, it keeps video/mp4 and completes.
func TestScanPassRefinesDeclaredMP4Audio(t *testing.T) {
	t.Parallel()
	content := []byte("bytes of a recording of Alex")
	for _, c := range []struct {
		declared, mime, state string
		fin                   bool
	}{
		{"audio/mp4", "audio/mp4", "pending", false},
		{"audio/x-m4a", "audio/mp4", "pending", false},
		{"video/mp4", "video/mp4", "complete", true},
		{"text/plain", "video/mp4", "complete", true},
	} {
		e := newScanEnvTypes(t, scanCfg, fixedType("video/mp4"))
		s := e.seed(t, content, content, true)
		e.repo.mu.Lock()
		f := e.repo.files[s.FileID]
		f.DeclaredMIMEType = c.declared
		e.repo.files[s.FileID] = f
		e.repo.mu.Unlock()
		if _, err := e.pass.Run(context.Background(), s.FileID, 0); err != nil {
			t.Fatal(err)
		}
		got := e.file(s.FileID)
		if got.MIMEType != c.mime || got.UploadState != c.state || len(e.queue.thumbnails()) != 1 || (e.finalCount(s.FileID) == 1) != c.fin {
			t.Errorf("declared %s: mime %s, state %s, jobs %d, finalized %d", c.declared, got.MIMEType, got.UploadState,
				len(e.queue.thumbnails()), e.finalCount(s.FileID))
		}
	}
}

// TestScanPassDenyListSeesDetectedMP4 (FIX-voice-send): a denied video/mp4 stays denied when the
// uploader declares MP4 audio, so the declaration cannot pass the deny list of 12 6.1.
func TestScanPassDenyListSeesDetectedMP4(t *testing.T) {
	t.Parallel()
	cfg := scanCfg
	cfg.DeniedMIMETypes = []string{"video/mp4"}
	e := newScanEnvTypes(t, cfg, fixedType("video/mp4"))
	content := []byte("bytes of a clip of Alex")
	s := e.seed(t, content, content, true)
	e.repo.mu.Lock()
	f := e.repo.files[s.FileID]
	f.DeclaredMIMEType = "audio/mp4"
	e.repo.files[s.FileID] = f
	e.repo.mu.Unlock()
	if _, err := e.pass.Run(context.Background(), s.FileID, 0); err != nil {
		t.Fatal(err)
	}
	if got := e.file(s.FileID); got.UploadState != "failed" {
		t.Fatalf("denied video/mp4 declared as audio: %+v", got)
	}
}
