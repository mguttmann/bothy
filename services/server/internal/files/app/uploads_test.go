// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// problem returns code and reason of err, or "ok".
func problem(err error) string {
	if err == nil {
		return "ok"
	}
	e, ok := errs.As(err)
	if !ok {
		return "untyped: " + err.Error()
	}
	if e.Reason != "" {
		return string(e.PublicCode()) + "/" + e.Reason
	}
	return string(e.PublicCode())
}

// content is a file of 10 bytes: three parts of 4, 4 and 2 bytes with testCfg.
var content = []byte("0123456789")

func createCmd(name string) files.CreateUpload {
	return files.CreateUpload{ChannelID: ids.New(), Name: name, SizeBytes: int64(len(content)), SHA256: sha256.Sum256(content)}
}

// TestCreateUploadCheckOrder holds the order of design note 5 (D-M3-03-4): when several checks
// fail at once, the first in the order authorize, size, type, quota, open sessions, free space
// decides.
func TestCreateUploadCheckOrder(t *testing.T) {
	type fails struct{ notFound, forbidden, size, denied, quota, sessions, space bool }
	all := fails{true, true, true, true, true, true, true}
	for _, c := range []struct {
		name string
		f    fails
		want string
	}{
		{"everything fails", all, "not_found"},
		{"visible but not allowed", fails{false, true, true, true, true, true, true}, "forbidden/capability_missing"},
		{"size first after authorize", fails{false, false, true, true, true, true, true}, "payload_too_large"},
		{"type before the database checks", fails{false, false, false, true, true, true, true}, "unsupported_media_type/file_type_denied"},
		{"quota before sessions", fails{false, false, false, false, true, true, true}, "conflict/quota_exceeded"},
		{"sessions before space", fails{false, false, false, false, false, true, true}, "rate_limited"},
		{"space last", fails{false, false, false, false, false, false, true}, "unavailable/storage_full"},
		{"nothing fails", fails{}, "ok"},
	} {
		t.Run(c.name, func(t *testing.T) {
			cfg := testCfg
			cfg.QuotaBytes = 100
			cfg.DeniedMIMETypes = []string{"image/png"}
			if c.f.space {
				cfg.MinFreeBytes = 1 << 40
			}
			e := newEnv(t, "fs", cfg)
			switch {
			case c.f.notFound:
				e.az.rules[authorize.ActionFileUpload] = authorize.Decision{Outcome: authorize.OutcomeNotFound, Reason: "no_grant"}
			case c.f.forbidden:
				e.az.rules[authorize.ActionFileUpload] = authorize.Decision{Outcome: authorize.OutcomeForbidden, Reason: "capability_missing"}
			}
			if c.f.quota {
				used := int64(95)
				e.repo.usedBytes = &used
			}
			if c.f.sessions {
				n := cfg.MaxOpenSessions
				e.repo.openCount = &n
			}
			cmd := createCmd("report.txt")
			if c.f.size {
				cmd.SizeBytes = cfg.MaxUploadBytes + 1
			}
			if c.f.denied {
				cmd.Name = "picture.png"
			}
			_, err := e.svc.Create(e.ctx, cmd)
			if got := problem(err); got != c.want {
				t.Fatalf("Create = %s, want %s", got, c.want)
			}
		})
	}
}

// TestCreateUploadRetryAfter holds the 429 of design note 8: Retry-After until the earliest
// expiry, rounded up to whole seconds and never zero.
func TestCreateUploadRetryAfter(t *testing.T) {
	cfg := testCfg
	cfg.MaxOpenSessions = 1
	e := newEnv(t, "fs", cfg)
	if _, err := e.svc.Create(e.ctx, createCmd("a.txt")); err != nil {
		t.Fatal(err)
	}
	e.clk.Advance(cfg.SessionTTL - 1500e6)
	_, err := e.svc.Create(e.ctx, createCmd("b.txt"))
	pe, ok := errs.As(err)
	if !ok || pe.PublicCode() != errs.CodeRateLimited || pe.RetryAfter.Seconds() != 2 {
		t.Fatalf("Create = %v (retry after %v), want rate_limited after 2s", err, pe)
	}
}

// TestDeniedTypeEarlyCheck holds design note 7 (D-M3-03-5).
func TestDeniedTypeEarlyCheck(t *testing.T) {
	for _, c := range []struct {
		name, file, declared string
		denied               []string
		want                 string
	}{
		{"declared type with parameters and other case", "a.bin", "Image/PNG; charset=binary", []string{"image/png"}, "unsupported_media_type/file_type_denied"},
		{"extension of a denied type", "a.PNG", "", []string{"image/png"}, "unsupported_media_type/file_type_denied"},
		{"denied list in other case", "a.png", "", []string{" IMAGE/PNG"}, "unsupported_media_type/file_type_denied"},
		{"unknown type has no extensions", "a.xyz", "application/x-unknown", []string{"application/x-other"}, "ok"},
		{"allowed type", "a.txt", "text/plain", []string{"image/png"}, "ok"},
		{"empty list checks nothing", "a.png", "not a type", nil, "ok"},
		{"unparsable declared type", "a.txt", "not a type", []string{"image/png"}, "validation_failed"},
	} {
		t.Run(c.name, func(t *testing.T) {
			cfg := testCfg
			cfg.DeniedMIMETypes = c.denied
			e := newEnv(t, "fs", cfg)
			cmd := createCmd(c.file)
			cmd.DeclaredMIMEType = c.declared
			s, err := e.svc.Create(e.ctx, cmd)
			if got := problem(err); got != c.want {
				t.Fatalf("Create = %s, want %s", got, c.want)
			}
			if err != nil {
				if e.store.total() != 0 || len(e.repo.files) != 0 {
					t.Fatal("a refused upload left rows or objects")
				}
				return
			}
			f := e.repo.files[s.FileID]
			if f.MIMEType != "application/octet-stream" {
				t.Errorf("mime_type = %q before detection", f.MIMEType)
			}
			if c.declared == "Image/PNG; charset=binary" && f.DeclaredMIMEType != "image/png" {
				t.Errorf("declared_mime_type = %q", f.DeclaredMIMEType)
			}
		})
	}
}

// uploadAll creates an upload of content and sends its three parts.
func uploadAll(t *testing.T, e *env, ctx context.Context) (files.UploadSession, []files.PartRef) {
	t.Helper()
	s, err := e.svc.Create(ctx, createCmd("report.txt"))
	if err != nil {
		t.Fatal(err)
	}
	var refs []files.PartRef
	for n := 1; n <= s.PartCount; n++ {
		lo := int64(n-1) * s.PartSizeBytes
		hi := min(lo+s.PartSizeBytes, int64(len(content)))
		part := content[lo:hi]
		ref, err := e.svc.PutPart(e.ctx, files.PutPart{FileID: s.FileID, PartNumber: n, ContentDigest: digestOf(part),
			ContentLength: int64(len(part)), Body: bytes.NewReader(part)})
		if err != nil {
			t.Fatalf("part %d: %v", n, err)
		}
		refs = append(refs, ref)
	}
	return s, refs
}

// TestNoObjectStoreCallInUnitOfWork: the fake store fails on any call while txn.InUnit(ctx) is
// true; create, parts, state, complete and abort pass with both strategies, also when create and
// complete run in the ambient unit of a keyed request (design note 4, 02 8.5).
func TestNoObjectStoreCallInUnitOfWork(t *testing.T) {
	for _, driver := range []string{"fs", "s3"} {
		t.Run(driver, func(t *testing.T) {
			e := newEnv(t, driver, testCfg)
			var s files.UploadSession
			var refs []files.PartRef
			err := e.uow.Do(e.ctx, func(ctx context.Context) error {
				var err error
				s, err = e.svc.Create(ctx, createCmd("report.txt"))
				return err
			})
			if err != nil {
				t.Fatal(err)
			}
			for n := 1; n <= s.PartCount; n++ {
				lo := int64(n-1) * s.PartSizeBytes
				part := content[lo:min(lo+s.PartSizeBytes, int64(len(content)))]
				ref, err := e.svc.PutPart(e.ctx, files.PutPart{FileID: s.FileID, PartNumber: n, ContentDigest: digestOf(part),
					ContentLength: -1, Body: bytes.NewReader(part)})
				if err != nil {
					t.Fatal(err)
				}
				refs = append(refs, ref)
			}
			if _, err := e.svc.State(e.ctx, s.FileID); err != nil {
				t.Fatal(err)
			}
			before := e.store.total()
			err = e.uow.Do(e.ctx, func(ctx context.Context) error {
				_, err := e.svc.Complete(ctx, files.CompleteUpload{FileID: s.FileID, Parts: refs})
				return err
			})
			if err != nil {
				t.Fatal(err)
			}
			if e.store.total() != before {
				t.Error("completion called the object store")
			}
			second, _ := uploadAll(t, e, e.ctx)
			if err := e.svc.Abort(e.ctx, second.FileID); err != nil {
				t.Fatal(err)
			}
			if _, err := e.svc.PutPart(e.uowCtx(t), files.PutPart{FileID: s.FileID, PartNumber: 1}); errs.KindOf(err) != errs.KindInternal {
				t.Errorf("PutPart inside a unit of work: %v, want an internal error", err)
			}
		})
	}
}

// uowCtx returns a context inside a unit of work of e.
func (e *env) uowCtx(t *testing.T) context.Context {
	t.Helper()
	var inner context.Context
	_ = e.uow.Do(e.ctx, func(ctx context.Context) error { inner = ctx; return nil })
	return inner
}

// TestS3PartStrategy holds the S3 strategy of design note 4: the first part creates the multipart
// upload once, a request that loses the race aborts its own upload and uses the stored one,
// UploadPart gets the stored id and its ETag is kept, abort calls AbortMultipart and completion
// calls nothing.
func TestS3PartStrategy(t *testing.T) {
	e := newEnv(t, "s3", testCfg)
	s, err := e.svc.Create(e.ctx, createCmd("report.txt"))
	if err != nil {
		t.Fatal(err)
	}
	stale, _, _ := e.repo.Session(e.ctx, s.FileID, e.alex.ID)
	ref, err := e.svc.PutPart(e.ctx, files.PutPart{FileID: s.FileID, PartNumber: 1, ContentDigest: digestOf(content[:4]),
		ContentLength: 4, Body: bytes.NewReader(content[:4])})
	if err != nil {
		t.Fatal(err)
	}
	if e.store.count("CreateMultipart") != 1 {
		t.Fatalf("CreateMultipart called %d times", e.store.count("CreateMultipart"))
	}
	stored, _, _ := e.repo.Session(e.ctx, s.FileID, e.alex.ID)
	if stored.StorageUploadID == "" {
		t.Fatal("no storage_upload_id stored")
	}
	if p, _, _ := e.repo.Part(e.ctx, s.FileID, 1); p.ETag == "" || p.ETag != ref.ETag {
		t.Fatalf("stored etag %q, answered %q", p.ETag, ref.ETag)
	}
	// A parallel first part that read the session before the id was stored loses the race.
	parts := app.NewS3PartStore(e.store, e.repo, e.uow)
	if _, err := parts.Put(e.ctx, stale, 2, bytes.NewReader(content[4:8]), 4); err != nil {
		t.Fatal(err)
	}
	if e.store.count("CreateMultipart") != 2 || e.store.count("AbortMultipart") != 1 {
		t.Fatalf("create %d, abort %d; want the loser to abort its own upload", e.store.count("CreateMultipart"), e.store.count("AbortMultipart"))
	}
	if again, _, _ := e.repo.Session(e.ctx, s.FileID, e.alex.ID); again.StorageUploadID != stored.StorageUploadID {
		t.Fatal("the stored upload id changed")
	}
	if len(e.store.uploads[stored.StorageUploadID]) != 2 {
		t.Fatalf("the stored upload holds %d parts, want 2", len(e.store.uploads[stored.StorageUploadID]))
	}
	if err := e.svc.Abort(e.ctx, s.FileID); err != nil {
		t.Fatal(err)
	}
	if e.store.count("AbortMultipart") != 2 {
		t.Fatalf("abort called AbortMultipart %d times in total", e.store.count("AbortMultipart"))
	}
	if _, ok := e.store.uploads[stored.StorageUploadID]; ok {
		t.Fatal("the multipart upload survived the abort")
	}

	done, refs := uploadAll(t, e, e.ctx)
	before := e.store.total()
	if _, err := e.svc.Complete(e.ctx, files.CompleteUpload{FileID: done.FileID, Parts: refs}); err != nil {
		t.Fatal(err)
	}
	if e.store.total() != before {
		t.Fatal("completion called the object store")
	}
}

// TestAbortWritesUploaderEvent holds design note 16 at the unit level: the abort appends one
// file.updated for the uploader with receive action file.read and the failed state.
func TestAbortWritesUploaderEvent(t *testing.T) {
	e := newEnv(t, "fs", testCfg)
	s, _ := uploadAll(t, e, e.ctx)
	if err := e.svc.Abort(e.ctx, s.FileID); err != nil {
		t.Fatal(err)
	}
	if len(e.events.list) != 1 {
		t.Fatalf("%d events", len(e.events.list))
	}
	ev := e.events.list[0]
	if ev.Type != "file.updated" || ev.AudiencePrincipalID != e.alex.ID || ev.ReceiveAction != "file.read" || ev.ResourceKind != "file" ||
		!bytes.Contains(ev.Payload, []byte(`"upload_state":"failed"`)) || bytes.Contains(ev.Payload, []byte(`"sha256"`)) {
		t.Fatalf("event %+v %s", ev, ev.Payload)
	}
	if err := e.svc.Abort(e.ctx, s.FileID); err != nil || len(e.events.list) != 1 {
		t.Fatalf("second abort: %v, %d events", err, len(e.events.list))
	}
	if got := len(e.store.objects); got != 0 {
		t.Fatalf("%d part objects survive the abort", got)
	}
	if rec, _, _ := e.repo.Session(e.ctx, s.FileID, e.alex.ID); rec.State != domain.SessionAborted {
		t.Fatalf("state %s", rec.State)
	}
}

// TestPutPartLengthAndDigest holds design note 9 g: a short, a long and a wrongly hashed body and
// a wrong Content-Length store nothing and answer part_digest_mismatch.
func TestPutPartLengthAndDigest(t *testing.T) {
	e := newEnv(t, "fs", testCfg)
	s, err := e.svc.Create(e.ctx, createCmd("report.txt"))
	if err != nil {
		t.Fatal(err)
	}
	good := content[:4]
	for _, c := range []struct {
		name    string
		body    []byte
		digest  string
		length  int64
		pointer string
	}{
		{"short", good[:3], digestOf(good[:3]), -1, ""},
		{"long", append([]byte{}, content[:5]...), digestOf(content[:5]), -1, ""},
		{"wrong digest", good, digestOf([]byte("abcd")), -1, "#header/Content-Digest"},
		{"wrong Content-Length", good, digestOf(good), 5, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := e.svc.PutPart(e.ctx, files.PutPart{FileID: s.FileID, PartNumber: 1, ContentDigest: c.digest, ContentLength: c.length,
				Body: bytes.NewReader(c.body)})
			pe, ok := errs.As(err)
			if !ok || pe.PublicCode() != errs.CodeValidationFailed || len(pe.Fields) != 1 || pe.Fields[0].Code != "part_digest_mismatch" ||
				pe.Fields[0].Pointer != c.pointer {
				t.Fatalf("PutPart = %v (%+v)", err, pe)
			}
			if len(e.store.objects) != 0 || len(e.repo.parts) != 0 {
				t.Fatal("a refused part was stored")
			}
		})
	}
}
