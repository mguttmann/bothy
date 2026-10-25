// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"log/slog"
	"sync"
	"testing"

	gatewaydomain "github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
)

// sentSignals records the payloads a SignalSender sends.
type sentSignals struct {
	mu   sync.Mutex
	sent [][]byte
}

// Send implements gatewayapp.SignalSender.
func (s *sentSignals) Send(_ context.Context, payload []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent = append(s.sent, payload)
	return nil
}

// TestRunsPublisher_WorkerSendsRunSignal: in role worker, where runs.stall_check runs and no
// gateway hub exists, the run.updated of the runs module goes to the api nodes as the gateway
// signal of the run with its version (M2-51 red team finding; 02 11.6, D-M7-17-7); in roles api
// and all the gateway binds its own publisher, so runsPublisher binds none.
func TestRunsPublisher_WorkerSendsRunSignal(t *testing.T) {
	t.Parallel()
	newServer := func(role Role) *server {
		return &server{role: role, values: loadValues(t), log: newSwitchLogger(slog.New(slog.DiscardHandler))}
	}
	for _, role := range []Role{RoleAPI, RoleAll} {
		pub, err := newServer(role).runsPublisher(&sentSignals{})
		if err != nil || pub != nil {
			t.Fatalf("role %s: %v %v", role, pub, err)
		}
	}
	sender := &sentSignals{}
	pub, err := newServer(RoleWorker).runsPublisher(sender)
	if err != nil || pub == nil {
		t.Fatalf("role worker: %v %v", pub, err)
	}
	run, thread := ids.New(), ids.New()
	err = pub.PublishEphemeral(context.Background(), eventlog.EphemeralEvent{
		Type: "run.updated", SchemaVersion: 1,
		Scope:    eventlog.Scope{SpaceID: ids.New(), ChannelID: ids.New(), ThreadID: thread},
		Resource: eventlog.ResourceRef{Kind: "run", ID: run, Version: 5},
		Payload: runs.RunUpdated{RunID: run, ThreadID: thread, BaseVersion: 4, Version: 5,
			Ops: []runs.PatchOp{{Op: "add", Path: "/stall_warned_at", Value: []byte(`"2026-10-25T09:00:00.000Z"`)}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(sender.sent) != 1 {
		t.Fatalf("%d signals sent, want 1", len(sender.sent))
	}
	sig, _, err := gatewaydomain.DecodeSignal(sender.sent[0])
	if err != nil || sig.Kind != gatewaydomain.SignalRun || sig.RunID != run || sig.ThreadID != thread || sig.Version != 5 {
		t.Fatalf("signal %+v %v", sig, err)
	}
}
