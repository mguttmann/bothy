// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"io"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// stageCfg allows images of up to 8 bytes: the part size is the cap.
var stageCfg = func() app.Config {
	c := testCfg
	c.PartBytes = 8
	return c
}()

// stage stages body as an avatar of Alex with the declared type.
func (e *env) stage(declared string, body io.Reader) (ids.ID, error) {
	return e.svc.StageImage(context.Background(), app.ImageUpload{Purpose: domain.PurposeAvatar, UploadedBy: e.alex.ID,
		DeclaredType: declared, Body: body})
}

// TestStageImage holds design note 2 f: the file row of purpose avatar without channel with the
// fixed name and the counted size and digest, an open session of one part, the part stored and
// confirmed, no event and no scan job before the link.
func TestStageImage(t *testing.T) {
	e := newEnv(t, "fs", stageCfg)
	body := []byte("pngbytes")
	id, err := e.stage("image/PNG; x=y", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	f := e.repo.files[id]
	sum := sha256.Sum256(body)
	if f.Purpose != "avatar" || !f.ScopeChannelID.IsZero() || f.OriginalName != "avatar" || f.UploadedBy != e.alex.ID ||
		f.SizeBytes != 8 || f.SHA256 != sum || f.DeclaredMIMEType != "image/png" || f.UploadState != "pending" || f.ScanStatus != "pending" {
		t.Fatalf("file row: %+v", f)
	}
	s := e.repo.sessions[id]
	if s.State != domain.SessionOpen || s.PartCount != 1 || s.DeclaredSizeBytes != 8 || s.DeclaredSHA256 != sum || s.Mode != "direct" {
		t.Fatalf("session: %+v", s)
	}
	if _, ok := e.repo.parts[id][1]; !ok || e.store.count("Put") != 1 {
		t.Fatalf("part not stored and confirmed: %v, %d puts", e.repo.parts[id], e.store.count("Put"))
	}
	if len(e.events.list) != 0 || len(e.scans.list()) != 0 {
		t.Fatal("staging wrote an event or a scan job")
	}
}

// failingBody fails the test when it is read.
type failingBody struct{ t *testing.T }

func (b failingBody) Read([]byte) (int, error) {
	b.t.Error("the body was read")
	return 0, io.EOF
}

// TestStageImageRejections holds the checks of design note 2 b to e: nothing is stored on any.
func TestStageImageRejections(t *testing.T) {
	errPart := errs.New(errs.CodeValidationFailed, "a further part")
	cases := []struct {
		name     string
		declared string
		body     func(t *testing.T) io.Reader
		mutate   func(e *env)
		want     errs.Code
		reason   string
		detail   string
	}{
		{name: "gif", declared: "image/gif", body: func(t *testing.T) io.Reader { return failingBody{t} }, want: errs.CodeUnsupportedMediaType},
		{name: "svg", declared: "image/svg+xml", body: func(t *testing.T) io.Reader { return failingBody{t} }, want: errs.CodeUnsupportedMediaType},
		{name: "no type", declared: "", body: func(t *testing.T) io.Reader { return failingBody{t} }, want: errs.CodeUnsupportedMediaType},
		{name: "denied", declared: "image/png", body: func(t *testing.T) io.Reader { return failingBody{t} },
			mutate: func(e *env) {}, want: errs.CodeUnsupportedMediaType, reason: "file_type_denied"},
		{name: "one byte over the cap", declared: "image/png", body: func(*testing.T) io.Reader { return strings.NewReader("123456789") },
			want: errs.CodePayloadTooLarge, detail: "file.upload_part_bytes"},
		{name: "empty", declared: "image/png", body: func(*testing.T) io.Reader { return strings.NewReader("") }, want: errs.CodeValidationFailed},
		{name: "further part", declared: "image/png",
			body: func(*testing.T) io.Reader { return io.MultiReader(strings.NewReader("abc"), errReader{errPart}) }, want: errs.CodeValidationFailed},
		{name: "broken body", declared: "image/png",
			body: func(*testing.T) io.Reader {
				return io.MultiReader(strings.NewReader("abc"), errReader{io.ErrUnexpectedEOF})
			},
			want: errs.CodeValidationFailed},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := stageCfg
			if c.mutate != nil {
				cfg.DeniedMIMETypes = []string{"image/png"}
			}
			e := newEnv(t, "fs", cfg)
			_, err := e.stage(c.declared, c.body(t))
			ee, ok := errs.As(err)
			if !ok || ee.Code != c.want || ee.Reason != c.reason || !strings.Contains(ee.Detail, c.detail) {
				t.Fatalf("got %v, want %s %q %q", err, c.want, c.reason, c.detail)
			}
			if len(e.repo.files) != 0 || len(e.repo.sessions) != 0 || e.store.total() != 0 {
				t.Fatal("a refused image left a row or an object")
			}
		})
	}
}

// errReader returns err on every read.
type errReader struct{ err error }

func (r errReader) Read([]byte) (int, error) { return 0, r.err }

// TestStageImageLimits holds design note 2 e: open sessions and free space apply, the quota does
// not, and staging inside a unit of work is an internal error.
func TestStageImageLimits(t *testing.T) {
	e := newEnv(t, "fs", stageCfg)
	open := 3
	e.repo.openCount = &open
	if _, err := e.stage("image/png", strings.NewReader("abc")); errs.KindOf(err) != errs.KindRateLimited {
		t.Fatalf("open sessions: %v", err)
	}
	cfg := stageCfg
	cfg.QuotaBytes = 1
	e = newEnv(t, "fs", cfg)
	used := int64(1 << 20)
	e.repo.usedBytes = &used
	if _, err := e.stage("image/png", strings.NewReader("abc")); err != nil {
		t.Fatalf("the quota was checked: %v", err)
	}
	err := e.uow.Do(context.Background(), func(ctx context.Context) error {
		_, err := e.svc.StageImage(ctx, app.ImageUpload{Purpose: domain.PurposeAvatar, UploadedBy: e.alex.ID, DeclaredType: "image/png",
			Body: strings.NewReader("abc")})
		return err
	})
	if errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("inside a unit: %v", err)
	}
}

// TestStageImageFreeSpace holds 503 storage_full below file.fs.min_free_bytes.
func TestStageImageFreeSpace(t *testing.T) {
	e := newEnv(t, "fs", stageCfg)
	svc, err := app.New(stageCfg, app.Deps{Repo: e.repo, Parts: app.NewFSPartStore(e.store), UoW: e.uow, Authorizer: e.az, Events: e.events,
		Channels: channels{}, Names: names{}, Types: types{}, FreeSpace: freeSpace{free: 5}, ScanQueue: e.scans, Store: e.store,
		Purge: e.purges, Redactor: e.redact, Cleanup: e.repo})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.StageImage(context.Background(), app.ImageUpload{Purpose: domain.PurposeBranding, UploadedBy: e.alex.ID,
		DeclaredType: "image/jpeg", Body: strings.NewReader("abc")})
	ee, ok := errs.As(err)
	if !ok || ee.Code != errs.CodeUnavailable || ee.Reason != "storage_full" || len(e.repo.files) != 0 {
		t.Fatalf("got %v", err)
	}
}

// TestCompleteStagedImage holds the link of design note 3: inside a unit of work the session is
// completed and files.scan inserted; a foreign principal, another purpose or a second call is an
// internal error, and so is a call outside a unit.
func TestCompleteStagedImage(t *testing.T) {
	e := newEnv(t, "fs", stageCfg)
	id, err := e.stage("image/webp", strings.NewReader("riff"))
	if err != nil {
		t.Fatal(err)
	}
	complete := func(by ids.ID, p domain.Purpose) error {
		return e.uow.Do(context.Background(), func(ctx context.Context) error { return e.svc.CompleteStagedImage(ctx, id, by, p) })
	}
	if err := e.svc.CompleteStagedImage(context.Background(), id, e.alex.ID, domain.PurposeAvatar); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("outside a unit: %v", err)
	}
	if err := complete(ids.New(), domain.PurposeAvatar); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("foreign principal: %v", err)
	}
	if err := complete(e.alex.ID, domain.PurposeBranding); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("other purpose: %v", err)
	}
	if err := complete(e.alex.ID, domain.PurposeAvatar); err != nil {
		t.Fatal(err)
	}
	if s := e.repo.sessions[id]; s.State != domain.SessionCompleted {
		t.Fatalf("session %s", s.State)
	}
	if jobs := e.scans.list(); len(jobs) != 1 || jobs[0].fileID != id {
		t.Fatalf("scan jobs %v", jobs)
	}
	if err := complete(e.alex.ID, domain.PurposeAvatar); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("second link: %v", err)
	}
}

// TestReleaseImage holds the release of design note 3: only a file of the purpose (and of the
// uploader when named) gets the deleted mark.
func TestReleaseImage(t *testing.T) {
	e := newEnv(t, "fs", stageCfg)
	id, err := e.stage("image/png", strings.NewReader("abc"))
	if err != nil {
		t.Fatal(err)
	}
	release := func(by ids.ID, p domain.Purpose) error {
		return e.uow.Do(context.Background(), func(ctx context.Context) error { return e.svc.ReleaseImage(ctx, id, by, p) })
	}
	if err := release(e.alex.ID, domain.PurposeBranding); err != nil || e.repo.files[id].Deleted {
		t.Fatalf("other purpose released: %v", err)
	}
	if err := release(ids.New(), domain.PurposeAvatar); err != nil || e.repo.files[id].Deleted {
		t.Fatalf("foreign uploader released: %v", err)
	}
	if err := release(e.alex.ID, domain.PurposeAvatar); err != nil || !e.repo.files[id].Deleted {
		t.Fatalf("not released: %v", err)
	}
	if err := release(e.alex.ID, domain.PurposeAvatar); err != nil {
		t.Fatalf("second release: %v", err)
	}
	if err := e.svc.ReleaseImage(context.Background(), id, e.alex.ID, domain.PurposeAvatar); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("outside a unit: %v", err)
	}
}

// TestOpenImageVariant holds design note 8: only the variant of a deliverable file of the purpose;
// every other case is the same not_found.
func TestOpenImageVariant(t *testing.T) {
	e := newEnv(t, "fs", stageCfg)
	id := ids.New()
	key, _ := domain.VariantKey(id, domain.VariantMedium)
	e.store.objects[key] = []byte("medium")
	base := app.FileRecord{ID: id, Purpose: "branding", UploadState: "complete", ScanStatus: "clean"}
	e.repo.thumbs[id] = []app.ThumbnailRecord{{Variant: domain.VariantMedium, ObjectKey: key, MIMEType: "image/png", SizeBytes: 6}}
	e.repo.files[id] = base
	v, err := e.svc.OpenImageVariant(context.Background(), id, domain.PurposeBranding, domain.VariantMedium)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(v.Body)
	_ = v.Body.Close()
	if string(b) != "medium" || v.MIMEType != "image/png" || v.SizeBytes != 6 {
		t.Fatalf("variant %q %+v", b, v)
	}
	for name, mutate := range map[string]func(f *app.FileRecord){
		"pending": func(f *app.FileRecord) { f.UploadState = "pending" },
		"failed":  func(f *app.FileRecord) { f.UploadState = "failed" },
		"blocked": func(f *app.FileRecord) { f.ScanStatus = "infected" },
		"deleted": func(f *app.FileRecord) { f.Deleted = true },
		"avatar":  func(f *app.FileRecord) { f.Purpose = "avatar" },
	} {
		t.Run(name, func(t *testing.T) {
			f := base
			mutate(&f)
			e.repo.files[id] = f
			if _, err := e.svc.OpenImageVariant(context.Background(), id, domain.PurposeBranding, domain.VariantMedium); errs.KindOf(err) != errs.KindNotFound {
				t.Fatalf("got %v", err)
			}
		})
	}
	e.repo.files[id] = base
	if _, err := e.svc.OpenImageVariant(context.Background(), id, domain.PurposeBranding, domain.VariantLarge); errs.KindOf(err) != errs.KindNotFound {
		t.Fatalf("no such variant: %v", err)
	}
	if _, err := e.svc.OpenImageVariant(context.Background(), ids.New(), domain.PurposeBranding, domain.VariantMedium); errs.KindOf(err) != errs.KindNotFound {
		t.Fatalf("unknown: %v", err)
	}
	delete(e.store.objects, key)
	if _, err := e.svc.OpenImageVariant(context.Background(), id, domain.PurposeBranding, domain.VariantMedium); errs.KindOf(err) != errs.KindNotFound {
		t.Fatalf("missing object: %v", err)
	}
}

// TestSessionRoutesRefuseImageFiles: the session routes of attachments (getUpload, uploadPart,
// completeUpload, abortUpload) answer an avatar or logo of the caller like an unknown upload, 404,
// and change nothing (review finding of M3-10: a zero scope channel reached authz as an internal
// error, and abortUpload would have aborted the staged session).
func TestSessionRoutesRefuseImageFiles(t *testing.T) {
	e := newEnv(t, "fs", stageCfg)
	id, err := e.stage("image/png", strings.NewReader("abc"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.State(e.ctx, id); errs.KindOf(err) != errs.KindNotFound {
		t.Errorf("getUpload: %v", err)
	}
	if _, err := e.svc.PutPart(e.ctx, files.PutPart{FileID: id, PartNumber: 1, ContentDigest: digestOf([]byte("abc")),
		ContentLength: 3, Body: strings.NewReader("abc")}); errs.KindOf(err) != errs.KindNotFound {
		t.Errorf("uploadPart: %v", err)
	}
	if _, err := e.svc.Complete(e.ctx, files.CompleteUpload{FileID: id}); errs.KindOf(err) != errs.KindNotFound {
		t.Errorf("completeUpload: %v", err)
	}
	if err := e.svc.Abort(e.ctx, id); errs.KindOf(err) != errs.KindNotFound {
		t.Errorf("abortUpload: %v", err)
	}
	if s := e.repo.sessions[id]; s.State != domain.SessionOpen || e.repo.files[id].UploadState != "pending" {
		t.Errorf("the staged image changed: %s %s", s.State, e.repo.files[id].UploadState)
	}
}
