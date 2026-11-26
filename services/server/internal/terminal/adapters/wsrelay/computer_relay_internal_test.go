// SPDX-License-Identifier: Apache-2.0

package wsrelay

import (
	"context"
	gosync "sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal/app"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// endRecorder records the ends the computer relay asks for.
type endRecorder struct {
	mu   gosync.Mutex
	ends []domain.ComputerEnd
}

func (e *endRecorder) AttachComputer(context.Context, ids.ID, string) (app.AttachOutcome, error) {
	return app.AttachOutcome{}, nil
}

func (e *endRecorder) ComputerDataSession(context.Context, app.Login, ids.ID) (bool, error) {
	return true, nil
}

func (e *endRecorder) EndComputerSession(_ context.Context, _ ids.ID, end domain.ComputerEnd) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ends = append(e.ends, end)
	return nil
}

// TestComputerRelay_ExpectationWithoutSideEndsNothing (review of CU-04): with several nodes every
// node expects an opening; the deadline of a node that holds no side of the session must not end
// it, because the session may run paired on another node.
func TestComputerRelay_ExpectationWithoutSideEndsNothing(t *testing.T) {
	t.Parallel()
	rec := &endRecorder{}
	r, err := NewComputerRelay(ComputerRelayConfig{OpenTimeout: time.Minute, WriteTimeout: time.Second}, rec, NewLinks(nil, nil), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	id := ids.New()
	r.ExpectComputer(app.ComputerExpectation{SessionID: id, HostID: ids.New(), Deadline: time.Now().Add(20 * time.Millisecond)})
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		r.mu.Lock()
		_, held := r.slots[id]
		r.mu.Unlock()
		if !held {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	r.mu.Lock()
	_, held := r.slots[id]
	r.mu.Unlock()
	if held {
		t.Fatal("the slot outlived its deadline")
	}
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.ends) != 0 {
		t.Fatalf("a node without any side ended the session: %+v", rec.ends)
	}
}
