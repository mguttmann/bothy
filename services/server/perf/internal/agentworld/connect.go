// SPDX-License-Identifier: Apache-2.0

package agentworld

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/gateway/adapters/wstest"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs/adapters/runstest"
	"github.com/OWNER/bothy/services/server/perf/internal/agentpath"
	"github.com/OWNER/bothy/services/server/perf/internal/fakeadapter"
	"github.com/OWNER/bothy/services/server/test/harness"
)

// jwksTimeout bounds one JWKS fetch of a reference verifier.
const jwksTimeout = 30 * time.Second

// StartAdapters starts one fake adapter in mode per agent against srv (design note 3): a reference
// verifier over getJwks with the adapter keys of the plan, the first heartbeat, then the gateway
// connection as consumer agentpath.ConsumerName. The adapters stop in tb's cleanup. The error
// reports the first adapter that did not start.
func (w *World) StartAdapters(tb testing.TB, srv *harness.Server, plan agentpath.Plan, mode agentpath.Mode) ([]*fakeadapter.Adapter, error) {
	issuer, err := ids.Parse(w.InstanceID)
	if err != nil {
		return nil, err
	}
	out := make([]*fakeadapter.Adapter, len(w.Agents))
	err = parallel(tb.Context(), len(w.Agents), func(ctx context.Context, i int) error {
		ag := w.Agents[i]
		aid, err := ids.Parse(ag.ID)
		if err != nil {
			return err
		}
		ver, err := runstest.NewReferenceVerifier(runstest.ReferenceOptions{
			JWKSURL: srv.PublicURL + "/api/v1/instance/jwks", Client: &http.Client{Timeout: jwksTimeout}, Clock: clock.System(),
			Issuer: issuer, AgentID: aid, Leeway: plan.TokenLeeway, RefreshMinInterval: plan.JWKSRefreshMinInterval,
		})
		if err != nil {
			return err
		}
		// The keys are loaded before the measurement, as an adapter does at its start.
		if err := ver.Refresh(ctx); err != nil {
			return fmt.Errorf("agent %s: JWKS: %w", ag.Handle, err)
		}
		a, err := fakeadapter.New(fakeadapter.Config{Mode: mode, MaxInflight: plan.MaxInflight, AckPersistInterval: plan.AckPersistInterval,
			RunUpdateMinInterval: plan.RunUpdateMinInterval}, fakeadapter.HarnessAPI{C: ag.Client}, ver)
		if err != nil {
			return err
		}
		// The adapter outlives the seeding request context: it runs until the cleanup.
		run := context.WithoutCancel(ctx)
		if err := a.Start(run, func(dctx context.Context) (fakeadapter.Conn, error) {
			return fakeadapter.DialGateway(dctx, tb, ag.Client, srv.PublicURL, agentpath.ConsumerName)
		}); err != nil {
			return fmt.Errorf("agent %s: %w", ag.Handle, err)
		}
		tb.Cleanup(a.Stop)
		out[i] = a
		return nil
	})
	return out, err
}

// RunFrame is one run.updated a viewer received, with the run it is for.
type RunFrame struct {
	RunID string
	agentpath.ViewerFrame
}

// Watcher reads every frame of one person's gateway connection: it keeps the run.updated frames
// (decoded strictly as RunUpdatedEvent) with their receive times and records every error frame,
// every frame that does not decode and an end of the connection it did not cause. Safe for
// concurrent use.
type Watcher struct {
	g    *harness.GatewayConn
	done chan struct{}

	mu       sync.Mutex
	runs     []RunFrame
	frames   int
	errs     []string
	stopping bool
	changed  chan struct{}
}

// ConnectViewers opens a gateway connection per viewer and starts a Watcher on each; the
// connections close in tb's cleanup. The error reports the first failed connection.
func (w *World) ConnectViewers(tb testing.TB) ([]*Watcher, error) {
	out := make([]*Watcher, len(w.Viewers))
	err := parallel(tb.Context(), len(w.Viewers), func(ctx context.Context, i int) error {
		g, err := w.Viewers[i].Gateway(ctx)
		if err != nil {
			return fmt.Errorf("viewer %d: %w", i+1, err)
		}
		v := &Watcher{g: g, done: make(chan struct{}), changed: make(chan struct{})}
		go v.read() //nolint:gosec // G118: the reader lives as long as the connection
		tb.Cleanup(v.Stop)
		out[i] = v
		return nil
	})
	return out, err
}

// read collects frames until the connection ends.
func (v *Watcher) read() {
	defer close(v.done)
	for {
		f, err := v.g.Next(context.Background())
		v.mu.Lock()
		if err != nil {
			if !v.stopping {
				v.errs = append(v.errs, fmt.Sprintf("the connection ended: %v", err))
			}
			v.wake()
			v.mu.Unlock()
			return
		}
		v.frames++
		v.mu.Unlock()
		v.frame(f)
	}
}

// frame handles one frame: error frames and run.updated.
func (v *Watcher) frame(f harness.Frame) {
	switch f.Op {
	case "error":
		var e rest.GatewayError
		_ = json.Unmarshal(f.D, &e)
		v.fail(fmt.Sprintf("error frame: op %s, code %s", e.Op, e.Code))
	case "dispatch":
		var head struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(f.D, &head) != nil || head.Type != "run.updated" {
			return
		}
		var ev rest.RunUpdatedEvent
		if err := wstest.Strict(f.D, &ev); err != nil {
			v.fail(fmt.Sprintf("run.updated does not decode strictly: %v", err))
			return
		}
		p := ev.Payload
		rf := RunFrame{RunID: p.RunId.String(), ViewerFrame: agentpath.ViewerFrame{Received: f.Received, Snapshot: p.Snapshot != nil, Version: p.Version}}
		if p.BaseVersion != nil {
			rf.BaseVersion = *p.BaseVersion
		}
		v.mu.Lock()
		v.runs = append(v.runs, rf)
		v.wake()
		v.mu.Unlock()
	}
}

// fail records one finding.
func (v *Watcher) fail(s string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.errs = append(v.errs, s)
	v.wake()
}

// wake wakes WaitFor; the caller holds mu.
func (v *Watcher) wake() {
	close(v.changed)
	v.changed = make(chan struct{})
}

// Focus sends focus with the one thread (GatewayFocus).
func (v *Watcher) Focus(thread string) error {
	return v.g.Send("focus", map[string]any{"thread_ids": []string{thread}})
}

// WaitFor waits until done reports true for the run.updated frames so far or ctx ends.
func (v *Watcher) WaitFor(ctx context.Context, done func([]RunFrame) bool) error {
	for {
		v.mu.Lock()
		ch, runs := v.changed, v.runs
		v.mu.Unlock()
		if done(runs) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ch:
		}
	}
}

// RunFrames returns the run.updated frames so far, in order of receipt.
func (v *Watcher) RunFrames() []RunFrame {
	v.mu.Lock()
	defer v.mu.Unlock()
	return append([]RunFrame(nil), v.runs...)
}

// Frames returns how many frames other than heartbeat_ack arrived.
func (v *Watcher) Frames() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.frames
}

// Errors returns the findings so far.
func (v *Watcher) Errors() []string {
	v.mu.Lock()
	defer v.mu.Unlock()
	return append([]string(nil), v.errs...)
}

// Stop closes the connection (an end it caused is no finding) and waits for the reader.
func (v *Watcher) Stop() {
	v.mu.Lock()
	already := v.stopping
	v.stopping = true
	v.mu.Unlock()
	if already {
		return
	}
	v.g.Close()
	<-v.done
}
