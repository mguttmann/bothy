// SPDX-License-Identifier: Apache-2.0

package jobs_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	filesjobs "github.com/OWNER/bothy/services/server/internal/files/adapters/jobs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
)

type purgeRunner struct{ got []ids.ID }

func (p *purgeRunner) Run(_ context.Context, id ids.ID) error {
	p.got = append(p.got, id)
	return nil
}

type cleanupRunner struct{ err error }

func (c cleanupRunner) Run(context.Context) error { return c.err }

// TestCleanupJobsRegistration holds both kinds on the queue maintenance, arguments of only the
// file id, and the periodic files.cleanup_pending with file.gc_interval and RunOnStart.
func TestCleanupJobsRegistration(t *testing.T) {
	t.Parallel()
	for _, typ := range []reflect.Type{reflect.TypeFor[filesjobs.PurgeObjects](), reflect.TypeFor[filesjobs.CleanupPending]()} {
		if err := jobs.ValidateArgsType(typ); err != nil {
			t.Fatal(err)
		}
	}
	reg := jobs.NewRegistry()
	if err := filesjobs.RegisterCleanup(reg, &purgeRunner{}, cleanupRunner{}, 0); err == nil {
		t.Fatal("a zero interval was accepted")
	}
	if err := filesjobs.RegisterCleanup(reg, &purgeRunner{}, cleanupRunner{}, time.Hour); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"files.purge_objects", "files.cleanup_pending"} {
		if q, ok := reg.Queue(kind); !ok || q != jobs.QueueMaintenance {
			t.Errorf("%s: queue %q %v", kind, q, ok)
		}
	}
	p := reg.Periodic()
	if len(p) != 1 || p[0].Args.Kind() != "files.cleanup_pending" || p[0].Interval != time.Hour || !p[0].RunOnStart {
		t.Fatalf("periodic %+v", p)
	}
}

// TestCleanupHandlers runs the use cases, returns their error and refuses a missing one or a
// cancelled context.
func TestCleanupHandlers(t *testing.T) {
	t.Parallel()
	r := &purgeRunner{}
	id := ids.New()
	if err := (filesjobs.PurgeHandler{Purge: r}).Work(context.Background(), jobs.Job[filesjobs.PurgeObjects]{Args: filesjobs.PurgeObjects{FileID: id}}); err != nil || len(r.got) != 1 || r.got[0] != id {
		t.Fatalf("Work = %v, got %v", err, r.got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := (filesjobs.PurgeHandler{Purge: r}).Work(ctx, jobs.Job[filesjobs.PurgeObjects]{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled Work = %v", err)
	}
	if err := (filesjobs.PurgeHandler{}).Work(context.Background(), jobs.Job[filesjobs.PurgeObjects]{}); err == nil {
		t.Fatal("a handler without purge succeeded")
	}
	boom := errors.New("step failed")
	if err := (filesjobs.CleanupHandler{Cleanup: cleanupRunner{err: boom}}).Work(context.Background(), jobs.Job[filesjobs.CleanupPending]{}); !errors.Is(err, boom) {
		t.Fatalf("cleanup Work = %v", err)
	}
	if err := (filesjobs.CleanupHandler{}).Work(context.Background(), jobs.Job[filesjobs.CleanupPending]{}); err == nil {
		t.Fatal("a handler without cleanup succeeded")
	}
	if err := (filesjobs.Purges{Q: &filesjobs.Queue{}}).Enqueue(context.Background(), id); err == nil {
		t.Fatal("an unbound purge queue inserted")
	}
}
