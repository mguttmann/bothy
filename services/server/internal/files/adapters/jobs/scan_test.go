// SPDX-License-Identifier: Apache-2.0

package jobs_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"

	filesjobs "github.com/OWNER/bothy/services/server/internal/files/adapters/jobs"
	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
)

type runner struct {
	res app.PassResult
	err error
	got []int64
}

func (r *runner) Run(_ context.Context, _ ids.ID, round int64) (app.PassResult, error) {
	r.got = append(r.got, round)
	return r.res, r.err
}

func TestScanArgsAndRegistration(t *testing.T) {
	t.Parallel()
	if err := jobs.ValidateArgsType(reflect.TypeFor[filesjobs.Scan]()); err != nil {
		t.Fatal(err)
	}
	reg := jobs.NewRegistry()
	if err := filesjobs.Register(reg, &runner{}); err != nil {
		t.Fatal(err)
	}
	if q, ok := reg.Queue("files.scan"); !ok || q != jobs.QueueFiles {
		t.Fatalf("queue %q %v", q, ok)
	}
}

func TestScanHandlerSpan(t *testing.T) {
	t.Parallel()
	rec := tracetest.NewSpanRecorder()
	tp := trace.NewTracerProvider(trace.WithSpanProcessor(rec))
	r := &runner{res: app.PassResult{Outcome: "clean"}}
	h := filesjobs.Handler{Pass: r, Tracer: tp}
	id := ids.New()
	if err := h.Work(context.Background(), jobs.Job[filesjobs.Scan]{Args: filesjobs.Scan{FileID: id, Round: 3}}); err != nil {
		t.Fatal(err)
	}
	spans := rec.Ended()
	if len(spans) != 1 || spans[0].Name() != "file.scan" || spans[0].SpanKind() != oteltrace.SpanKindClient || r.got[0] != 3 {
		t.Fatalf("spans %v rounds %v", spans, r.got)
	}
	want := map[attribute.Key]string{"bothy.file.id": id.String(), "bothy.outcome": "clean"}
	if len(spans[0].Attributes()) != len(want) {
		t.Errorf("attributes %v", spans[0].Attributes())
	}
	for _, kv := range spans[0].Attributes() {
		if want[kv.Key] != kv.Value.AsString() {
			t.Errorf("attribute %v", kv)
		}
	}
	dbErr := errors.New("database down")
	h.Pass = &runner{err: dbErr}
	if err := h.Work(context.Background(), jobs.Job[filesjobs.Scan]{Args: filesjobs.Scan{FileID: id}}); !errors.Is(err, dbErr) {
		t.Fatalf("Work = %v", err)
	}
	if err := (filesjobs.Handler{}).Work(context.Background(), jobs.Job[filesjobs.Scan]{}); err == nil {
		t.Fatal("a handler without pass succeeded")
	}
}

func TestQueueUnbound(t *testing.T) {
	t.Parallel()
	var q filesjobs.Queue
	if err := q.EnqueueScan(context.Background(), ids.New(), 0, time.Time{}); err == nil {
		t.Fatal("an unbound queue inserted")
	}
}
