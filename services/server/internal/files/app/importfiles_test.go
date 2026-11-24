// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
)

// slot runs ImportFiles.Slot in a unit of work, as the importer does.
func slot(t *testing.T, e *env, s files.UploadSession) files.ImportSlot {
	t.Helper()
	var got files.ImportSlot
	if err := e.uow.Do(e.ctx, func(ctx context.Context) error {
		var err error
		got, err = e.svc.ImportFiles().Slot(ctx, s.FileID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if got.Upload.FileID != s.FileID {
		t.Fatalf("slot %+v", got)
	}
	return got
}

// TestImportSlotReopenWritesFileUpdated pins that reopening the aborted session of an import file
// announces the way back from failed to pending with file.updated for the uploader.
func TestImportSlotReopenWritesFileUpdated(t *testing.T) {
	e := newEnv(t, "fs", testCfg)
	s, _ := uploadAll(t, e, e.ctx)
	if err := e.svc.Abort(e.ctx, s.FileID); err != nil {
		t.Fatal(err)
	}
	slot(t, e, s)
	if len(e.events.list) != 2 {
		t.Fatalf("%d events, want the abort and the reopen", len(e.events.list))
	}
	ev := e.events.list[1]
	if ev.Type != "file.updated" || ev.AudiencePrincipalID != e.alex.ID || !bytes.Contains(ev.Payload, []byte(`"upload_state":"pending"`)) {
		t.Fatalf("event %+v %s", ev, ev.Payload)
	}
	if rec, _, _ := e.repo.Session(e.ctx, s.FileID, e.alex.ID); rec.State != domain.SessionOpen {
		t.Fatalf("state %s", rec.State)
	}
}

// TestImportSlotReopenAbortsTheOldMultipartUpload pins that reopening an open session past its
// expiry aborts its S3 multipart upload after the commit, so no upload is left that no row names,
// and that a file that stayed pending gets no event.
func TestImportSlotReopenAbortsTheOldMultipartUpload(t *testing.T) {
	e := newEnv(t, "s3", testCfg)
	s, err := e.svc.Create(e.ctx, createCmd("report.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.PutPart(e.ctx, files.PutPart{FileID: s.FileID, PartNumber: 1, ContentDigest: digestOf(content[:4]),
		ContentLength: 4, Body: bytes.NewReader(content[:4])}); err != nil {
		t.Fatal(err)
	}
	if len(e.store.uploads) != 1 {
		t.Fatalf("%d multipart uploads", len(e.store.uploads))
	}
	e.clk.Advance(testCfg.SessionTTL)
	slot(t, e, s)
	if e.store.count("AbortMultipart") != 1 || len(e.store.uploads) != 0 {
		t.Fatalf("abort %d, %d uploads left; want the old upload aborted", e.store.count("AbortMultipart"), len(e.store.uploads))
	}
	if len(e.events.list) != 0 {
		t.Fatalf("%d events for a file that stayed pending", len(e.events.list))
	}
}
