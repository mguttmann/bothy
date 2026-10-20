// SPDX-License-Identifier: Apache-2.0

package images_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// errCrash is the error the hook BeforeCommit returns once in testJobCrashBeforeCommit.
var errCrash = errors.New("images test: T1 fails once")

// heldRun is how long a run of testJobConcurrencyLimit keeps its slot, so a second run that the
// limiter let through would overlap it.
const heldRun = 500 * time.Millisecond

// waitClosed waits until ch is closed or waitJob passed.
func waitClosed(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(waitJob):
		t.Fatalf("%s: not reached after %v", what, waitJob)
	}
}

// testJobDeletedMidway: deleted_at is set between the object write and T1. The row stays as it
// was, no event of the job, and neither revision 2 nor a variant remains on disk.
func testJobDeletedMidway(t *testing.T, e *env) {
	u := e.create(t, "deleted.jpg", "", plainJPEG(t))
	var once sync.Once
	var hookErr error
	e.hooks.afterPut.Store(u.FileID, func(ctx context.Context) {
		once.Do(func() {
			_, hookErr = e.db.Exec(ctx, `UPDATE files SET deleted_at = now() WHERE id = $1`, u.FileID)
		})
	})
	defer e.hooks.afterPut.Delete(u.FileID)
	e.complete(t, u)
	r := e.waitJobDone(t, u.FileID)
	if hookErr != nil {
		t.Fatal(hookErr)
	}
	if !r.Deleted || r.UploadState != statePending || r.ScanStatus != scanSkipped || r.revision() != 1 {
		t.Fatalf("row %+v", r)
	}
	for _, ev := range e.events(t, u.FileID) {
		if ev.Payload.File.UploadState != statePending {
			t.Errorf("event of the job: %s", ev.Raw)
		}
	}
	poll(t, "objects of the deleted file", func() bool { return slices.Equal(e.objects(t, u.FileID), []string{"orig-1"}) })
	if n := e.thumbnails(t, u.FileID); n != 0 {
		t.Errorf("%d thumbnails", n)
	}
}

// testJobCrashBeforeCommit: T1 fails once after orig-2 was written; River's retry ends with
// revision 2 referenced, revision 1 deleted, no other object and no duplicate event.
func testJobCrashBeforeCommit(t *testing.T, e *env) {
	u := e.create(t, "crash.jpg", "", plainJPEG(t))
	var calls atomic.Int32
	e.hooks.beforeCommit.Store(u.FileID, func(context.Context) error {
		if calls.Add(1) == 1 {
			return errCrash
		}
		return nil
	})
	defer e.hooks.beforeCommit.Delete(u.FileID)
	e.complete(t, u)
	// River retries after a backoff of about a second; waitJob covers it.
	poll(t, "thumbnails after the retry", func() bool { return e.thumbnails(t, u.FileID) == 3 })
	r := e.waitJobDone(t, u.FileID)
	if r.UploadState != stateComplete || r.revision() != 2 || calls.Load() < 3 {
		t.Fatalf("row %+v after %d commits", r, calls.Load())
	}
	if n := e.count(t, `SELECT max(attempt) FROM jobs.river_job WHERE kind = 'files.thumbnail' AND args->>'file_id' = $1`, u.FileID); n < 2 {
		t.Errorf("attempt %d, want a retry", n)
	}
	if got := e.objects(t, u.FileID); !slices.Equal(got, processedObjects) {
		t.Errorf("objects %v, want %v", got, processedObjects)
	}
	checkImageEvents(t, e, u.FileID)
}

// snapshot is what testJobRunTwice compares before and after the wait.
type snapshot struct {
	row     fileRow
	events  int
	objects []string
	thumbs  int
}

// take reads the snapshot of file id.
func (e *env) take(t *testing.T, id string) snapshot {
	t.Helper()
	return snapshot{row: e.readFile(t, id), events: len(e.events(t, id)), objects: e.objects(t, id), thumbs: e.thumbnails(t, id)}
}

// testJobRunTwice: a done file stays unchanged. A second files.thumbnail job cannot be inserted
// from the test without River's client (a row INSERTed into jobs.river_job by hand would bypass
// River's insert path and its unique and state rules), so this checks the observable half: one
// job per file, and after the job is done no further event, object or row change appears while
// the test waits. The idempotence of a second run itself is covered by the unit tests of
// internal/files/app and by testJobCrashBeforeCommit, whose retry runs the job again.
func testJobRunTwice(t *testing.T, e *env) {
	id := e.send(t, "twice.jpg", "", plainJPEG(t))
	e.waitJobDone(t, id)
	poll(t, "thumbnails", func() bool { return e.thumbnails(t, id) == 3 })
	before := e.take(t, id)
	time.Sleep(stillFor)
	after := e.take(t, id)
	if !sameRow(before.row, after.row) || before.events != after.events || !slices.Equal(before.objects, after.objects) ||
		before.thumbs != after.thumbs {
		t.Errorf("changed: %+v -> %+v", before, after)
	}
	if n := e.jobs(t, id); n != 1 {
		t.Errorf("%d jobs", n)
	}
}

// sameRow compares two rows field by field (fileRow holds slices and pointers).
func sameRow(a, b fileRow) bool {
	return a.UploadState == b.UploadState && a.ScanStatus == b.ScanStatus && a.MIMEType == b.MIMEType && a.ObjectKey == b.ObjectKey &&
		a.SizeBytes == b.SizeBytes && string(a.SHA256) == string(b.SHA256) && a.Deleted == b.Deleted &&
		eqPtr(a.Width, b.Width) && eqPtr(a.Height, b.Height) && eqPtr(a.DurationMS, b.DurationMS)
}

// eqPtr compares two optional values.
func eqPtr[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// testJobConcurrencyLimit: with file.image.max_concurrent = 1 two queued images never run at
// the same time; the hooks count the runs holding a slot over the whole server life.
func testJobConcurrencyLimit(t *testing.T, e *env) {
	a := e.create(t, "first.jpg", "", plainJPEG(t))
	b := e.create(t, "second.jpg", "", plainJPEG(t))
	hold := func(ctx context.Context) error {
		select {
		case <-time.After(heldRun):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	for _, u := range []upload{a, b} {
		e.hooks.beforeRun.Store(u.FileID, hold)
		defer e.hooks.beforeRun.Delete(u.FileID)
	}
	e.complete(t, a)
	e.complete(t, b)
	for _, u := range []upload{a, b} {
		if r := e.waitJobDone(t, u.FileID); r.UploadState != stateComplete {
			t.Fatalf("row %+v", r)
		}
	}
	// Both jobs were queued before the first one finished, so the limiter had a second run to hold
	// back.
	var overlap bool
	if err := e.db.QueryRow(t.Context(), `SELECT bool_and(created_at < (SELECT min(finalized_at) FROM jobs.river_job
		WHERE kind = 'files.thumbnail' AND args->>'file_id' = ANY($1))) FROM jobs.river_job WHERE kind = 'files.thumbnail' AND args->>'file_id' = ANY($1)`,
		[]string{a.FileID, b.FileID}).Scan(&overlap); err != nil {
		t.Fatal(err)
	}
	if !overlap {
		t.Error("the two jobs were never queued at the same time")
	}
	if m := e.hooks.maxActive.Load(); m != 1 {
		t.Errorf("%d runs at the same time, want at most 1", m)
	}
}

// testJobLogsNoMetadata (SEC-LOG-4): the captured logs of the whole flow (every case before and
// the metadata JPEG once more) carry neither an original name, nor a GPS value, nor XMP text, nor
// any other metadata canary of the test images.
func testJobLogsNoMetadata(t *testing.T, e *env) {
	in, _ := metadataJPEG(t)
	e.waitJobDone(t, e.send(t, "logs.jpg", "", in))
	out := e.logs.String()
	if !strings.Contains(out, jobKind) && e.hooks.runs.Load() == 0 {
		t.Fatal("no run of files.thumbnail seen")
	}
	if out == "" {
		t.Fatal("no log lines captured")
	}
	for _, c := range []string{canaryNamePrefix, canaryGPS, canaryXMP, canaryComment, canaryIPTC} {
		if strings.Contains(out, c) {
			t.Errorf("the logs carry %q", c)
		}
	}
}
