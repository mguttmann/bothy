// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestScanPassPurposeRule holds brief M3-10 design note 4 in the scan pass: an avatar or logo whose
// detected type would be stored unchanged or as a re-encoded GIF fails with file_type_denied and
// loses its object before it can become deliverable; JPEG, PNG and WebP wait for files.thumbnail;
// attachments are unchanged.
func TestScanPassPurposeRule(t *testing.T) {
	t.Parallel()
	content := []byte("bytes of an image of Alex")
	for _, c := range []struct {
		purpose, mime string
		state         string
		failed, job   bool
	}{
		{"avatar", "text/html", "failed", true, false},
		{"branding", "image/gif", "failed", true, false},
		{"avatar", "image/svg+xml", "failed", true, false},
		{"branding", "image/heic", "failed", true, false},
		{"avatar", "image/jpeg", "pending", false, true},
		{"branding", "image/png", "pending", false, true},
		{"avatar", "image/webp", "pending", false, true},
		{"attachment", "image/gif", "pending", false, true},
		{"attachment", "text/html", "complete", false, false},
	} {
		e := newScanEnvTypes(t, scanCfg, fixedType(c.mime))
		s := e.seed(t, content, content, true)
		f := e.repo.files[s.FileID]
		f.Purpose = c.purpose
		if c.purpose != "attachment" {
			f.ScopeChannelID = ids.ID{}
		}
		e.repo.files[s.FileID] = f
		s.File = f
		e.repo.sessions[s.FileID] = s
		if _, err := e.pass.Run(context.Background(), s.FileID, 0); err != nil {
			t.Fatal(err)
		}
		got := e.file(s.FileID)
		reason := e.repo.failure[s.FileID]
		if got.UploadState != c.state || (reason == domain.ReasonFileTypeDenied) != c.failed || (len(e.queue.thumbnails()) == 1) != c.job {
			t.Errorf("%s %s: state %s, reason %q, jobs %d", c.purpose, c.mime, got.UploadState, reason, len(e.queue.thumbnails()))
		}
		if c.failed {
			if _, err := e.store.Head(context.Background(), f.ObjectKey); err == nil {
				t.Errorf("%s %s: the object of a refused image stays", c.purpose, c.mime)
			}
			if evs := e.events.list; len(evs) != 1 || evs[0].AudiencePrincipalID != f.UploadedBy || !evs[0].SpaceID.IsZero() {
				t.Errorf("%s %s: file.updated %+v", c.purpose, c.mime, evs)
			}
		}
	}
}

// TestProcessImagePurposeRule holds design note 4 in files.thumbnail: a WebP of an avatar or logo
// that the job keeps unchanged (animated, above file.image.max_pixels or undecodable) fails with
// file_type_denied and loses its object; an attachment WebP stays complete unchanged.
func TestProcessImagePurposeRule(t *testing.T) {
	t.Parallel()
	for _, cleanErr := range []error{app.ErrImageAnimated, app.ErrTooManyPixels, app.ErrImageUndecodable} {
		for _, purpose := range []string{"avatar", "branding", "attachment"} {
			e := newImageEnv(t, nil)
			e.codec.cleanErr = cleanErr
			f := e.seed(t, "image/webp", "pending", []byte("RIFFxxxxWEBP"))
			f.Purpose = purpose
			e.repo.files[f.ID] = f
			if err := e.proc.ProcessImage(context.Background(), f.ID); err != nil {
				t.Fatal(err)
			}
			got, _ := e.repo.imageFile(f.ID)
			if purpose == "attachment" {
				if got.UploadState != "complete" || got.ObjectKey != f.ObjectKey {
					t.Errorf("attachment %v: %+v", cleanErr, got)
				}
				continue
			}
			if got.UploadState != "failed" || e.repo.failure[f.ID] != domain.ReasonFileTypeDenied || len(e.events.list) != 1 || len(e.finalized) != 1 {
				t.Errorf("%s %v: state %s, reason %q, events %d", purpose, cleanErr, got.UploadState, e.repo.failure[f.ID], len(e.events.list))
			}
			if k := e.keys(); len(k) != 0 {
				t.Errorf("%s %v: objects %v", purpose, cleanErr, k)
			}
		}
	}
}
