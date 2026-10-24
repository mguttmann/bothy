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

type idRunner struct {
	got []ids.ID
	err error
}

func (r *idRunner) Run(_ context.Context, id ids.ID) error {
	r.got = append(r.got, id)
	return r.err
}

// TestExtractArgsAndRegistration: both kinds carry only ids and pass the argument and kind checks;
// files.extract_text runs on the queue files, files.extract_text_refresh periodically on the queue
// maintenance with the interval and once at the start; the handlers pass ids and errors through.
func TestExtractArgsAndRegistration(t *testing.T) {
	t.Parallel()
	for _, typ := range []reflect.Type{reflect.TypeFor[filesjobs.ExtractText](), reflect.TypeFor[filesjobs.ExtractRefresh]()} {
		if err := jobs.ValidateArgsType(typ); err != nil {
			t.Fatalf("%s: %v", typ, err)
		}
	}
	if (filesjobs.ExtractText{}).Kind() != "files.extract_text" || (filesjobs.ExtractRefresh{}).Kind() != "files.extract_text_refresh" {
		t.Fatal("kinds")
	}
	if err := filesjobs.RegisterExtract(jobs.NewRegistry(), nil, nil, 0); err == nil {
		t.Fatal("zero interval accepted")
	}
	reg := jobs.NewRegistry()
	if err := filesjobs.RegisterExtract(reg, &idRunner{}, &idRunner{}, 24*time.Hour); err != nil {
		t.Fatal(err)
	}
	if q, ok := reg.Queue("files.extract_text"); !ok || q != jobs.QueueFiles {
		t.Fatalf("extract queue %q %v", q, ok)
	}
	if q, ok := reg.Queue("files.extract_text_refresh"); !ok || q != jobs.QueueMaintenance {
		t.Fatalf("refresh queue %q %v", q, ok)
	}
	p := reg.Periodic()
	if len(p) != 1 || p[0].Args.Kind() != "files.extract_text_refresh" || p[0].Interval != 24*time.Hour || !p[0].RunOnStart {
		t.Fatalf("periodic %+v", p)
	}
	r := &idRunner{err: errors.New("db down")}
	id := ids.New()
	if err := (filesjobs.ExtractTextHandler{Text: r}).Work(context.Background(), jobs.Job[filesjobs.ExtractText]{Args: filesjobs.ExtractText{FileID: id}}); !errors.Is(err, r.err) || r.got[0] != id {
		t.Fatalf("text: %v %v", err, r.got)
	}
	rr := &idRunner{}
	if err := (filesjobs.ExtractRefreshHandler{Refresh: rr}).Work(context.Background(), jobs.Job[filesjobs.ExtractRefresh]{Args: filesjobs.ExtractRefresh{After: id}}); err != nil || rr.got[0] != id {
		t.Fatalf("refresh: %v %v", err, rr.got)
	}
	if err := (filesjobs.ExtractTextHandler{}).Work(context.Background(), jobs.Job[filesjobs.ExtractText]{}); err == nil {
		t.Fatal("text handler without use case succeeded")
	}
	if err := (filesjobs.ExtractRefreshHandler{}).Work(context.Background(), jobs.Job[filesjobs.ExtractRefresh]{}); err == nil {
		t.Fatal("refresh handler without use case succeeded")
	}
	var q filesjobs.ExtractQueue
	if q.EnqueueExtract(context.Background(), id) == nil || q.EnqueueExtractRefresh(context.Background(), id) == nil {
		t.Fatal("an unbound queue inserted")
	}
}
