// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestJournalObjects (M7-07): the ports of the deletion journal store, read and delete objects
// only under files.JournalKeyPrefix, report a missing object as not found and refuse an object
// above files.JournalObjectMaxBytes.
func TestJournalObjects(t *testing.T) {
	e := newEnv(t, "fs", testCfg)
	j, err := app.NewJournal(e.svc)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	key := files.JournalKeyPrefix + strings.Repeat("0", 18) + "1"
	if _, found, err := j.Get(ctx, key); err != nil || found {
		t.Fatalf("Get of a missing object = %v, %v", found, err)
	}
	body := []byte(`{"resource_kind":"message"}`)
	if err := j.Put(ctx, key, body); err != nil {
		t.Fatal(err)
	}
	got, found, err := j.Get(ctx, key)
	if err != nil || !found || !bytes.Equal(got, body) {
		t.Fatalf("Get = %q, %v, %v", got, found, err)
	}
	if err := j.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := j.Get(ctx, key); found {
		t.Fatal("the object stays after Delete")
	}
	for _, bad := range []string{"f/aa/bb/x/orig-1", files.JournalKeyPrefix + "../x", "deletion-journal", files.JournalKeyPrefix + "A"} {
		if err := j.Put(ctx, bad, body); errs.KindOf(err) != errs.KindInternal {
			t.Errorf("Put(%q) = %v, want internal", bad, err)
		}
		if _, _, err := j.Get(ctx, bad); errs.KindOf(err) != errs.KindInternal {
			t.Errorf("Get(%q) = %v, want internal", bad, err)
		}
		if err := j.Delete(ctx, bad); errs.KindOf(err) != errs.KindInternal {
			t.Errorf("Delete(%q) = %v, want internal", bad, err)
		}
	}
	big := bytes.Repeat([]byte("x"), files.JournalObjectMaxBytes+1)
	if err := j.Put(ctx, key, big); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("Put of a large object = %v", err)
	}
	e.store.mu.Lock()
	e.store.objects[key] = big
	e.store.mu.Unlock()
	if _, _, err := j.Get(ctx, key); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("Get of a large object = %v", err)
	}
	if _, err := app.NewJournal(nil); err == nil {
		t.Fatal("NewJournal without the service")
	}
}

// TestDeleteForJournal (M7-07): an unattached attachment is marked deleted with its redaction and
// purge; a deleted, an attached, an unknown file and a file of another purpose are left alone.
func TestDeleteForJournal(t *testing.T) {
	e := newEnv(t, "fs", testCfg)
	j, err := app.NewJournal(e.svc)
	if err != nil {
		t.Fatal(err)
	}
	p := newPlace()
	free := e.seedFile(e.alex.ID, p.channel, domain.SessionCompleted, domain.UploadComplete, domain.ScanClean)
	attached := e.seedFile(e.alex.ID, p.channel, domain.SessionCompleted, domain.UploadComplete, domain.ScanClean)
	if _, err := e.attach(p, e.alex.ID, attached.ID); err != nil {
		t.Fatal(err)
	}
	avatar := e.seedFile(e.alex.ID, p.channel, domain.SessionCompleted, domain.UploadComplete, domain.ScanClean)
	e.repo.mu.Lock()
	a := e.repo.files[avatar.ID]
	a.Purpose = string(domain.PurposeAvatar)
	e.repo.files[avatar.ID] = a
	e.repo.mu.Unlock()
	ctx := context.Background()
	done, err := j.DeleteForJournal(ctx, free.ID)
	if err != nil || !done {
		t.Fatalf("DeleteForJournal(free) = %v, %v", done, err)
	}
	if again, err := j.DeleteForJournal(ctx, free.ID); err != nil || again {
		t.Fatalf("second DeleteForJournal = %v, %v", again, err)
	}
	for name, id := range map[string]ids.ID{"attached": attached.ID, "avatar": avatar.ID, "unknown": ids.New()} {
		if done, err := j.DeleteForJournal(ctx, id); err != nil || done {
			t.Errorf("%s: DeleteForJournal = %v, %v", name, done, err)
		}
	}
	if got := e.purges.list(); len(got) != 1 || got[0] != free.ID {
		t.Fatalf("purges %v, want only %s", got, free.ID)
	}
}
