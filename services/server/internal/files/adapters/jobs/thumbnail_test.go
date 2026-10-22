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

type imageRunner struct {
	got []ids.ID
	err error
}

func (r *imageRunner) ProcessImage(_ context.Context, id ids.ID) error {
	r.got = append(r.got, id)
	return r.err
}

// TestThumbnailArgsAndRegistration: files.thumbnail carries only the file id and runs on the
// queue files; the handler passes the id and the error through.
func TestThumbnailArgsAndRegistration(t *testing.T) {
	t.Parallel()
	if err := jobs.ValidateArgsType(reflect.TypeFor[filesjobs.ThumbnailArgs]()); err != nil {
		t.Fatal(err)
	}
	if (filesjobs.ThumbnailArgs{}).Kind() != "files.thumbnail" {
		t.Fatal("kind")
	}
	reg := jobs.NewRegistry()
	if err := filesjobs.RegisterThumbnail(reg, &imageRunner{}, time.Minute); err != nil {
		t.Fatal(err)
	}
	if q, ok := reg.Queue("files.thumbnail"); !ok || q != jobs.QueueFiles {
		t.Fatalf("queue %q %v", q, ok)
	}
	r := &imageRunner{err: errors.New("store down")}
	id := ids.New()
	h := filesjobs.ThumbnailHandler{Images: r}
	if err := h.Work(context.Background(), jobs.Job[filesjobs.ThumbnailArgs]{Args: filesjobs.ThumbnailArgs{FileID: id}}); !errors.Is(err, r.err) ||
		len(r.got) != 1 || r.got[0] != id {
		t.Fatalf("Work = %v, %v", err, r.got)
	}
	if err := (filesjobs.ThumbnailHandler{}).Work(context.Background(), jobs.Job[filesjobs.ThumbnailArgs]{}); err == nil {
		t.Fatal("a handler without use case succeeded")
	}
	var q filesjobs.Queue
	if err := q.EnqueueThumbnail(context.Background(), id); err == nil {
		t.Fatal("an unbound queue inserted")
	}
}
