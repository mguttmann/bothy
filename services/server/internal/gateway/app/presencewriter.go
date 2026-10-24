// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"log/slog"
	gosync "sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The log record of a failed write of gateway_presence; never a person, device or thread.
const (
	msgPresenceWrite       = "gateway presence not written, retrying at the next interval"
	errorTypePresenceWrite = "presence_write"
)

// PresenceMetrics counts the failed writes of gateway_presence (bothy.gateway.presence.write_failures,
// signals.md 4.1); adapters/otelmetrics implements it.
type PresenceMetrics interface {
	PresenceWriteFailed(ctx context.Context)
}

// PresenceWriterConfig holds the configuration of the presence writer.
type PresenceWriterConfig struct {
	// Node is the process instance, gateway_presence.node_id.
	Node ids.ID
	// FlushInterval is cluster.presence_flush_interval: the period of the flush.
	FlushInterval time.Duration
	// TTL is cluster.presence_ttl: rows of any node older than it are deleted at a flush.
	TTL time.Duration
}

// PresenceWriter writes the counted connections of the hub of this process into gateway_presence
// (M7-17 Design note 2, R-DM-51, docs/decisions.d/M7-17.md D-M7-17-6): every
// cluster.presence_flush_interval one flush upserts the rows of all live connections with a fresh
// updated_at (so a connection that became live, changed its focus or idle flag, and a row lost with
// the UNLOGGED table, are written within one interval), deletes the rows of its connections that
// ended, and deletes the rows of nodes that stopped refreshing; a clean shutdown deletes all its
// rows. A failed write is logged and counted and never blocks a connection: the hub is only read
// under its own short locks. It is safe for concurrent use.
type PresenceWriter struct {
	cfg     PresenceWriterConfig
	hub     *Hub
	store   PresenceStore
	clk     clock.Clock
	log     *slog.Logger
	metrics PresenceMetrics

	mu     gosync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

// NewPresenceWriter returns the writer over the hub and store; clk nil means clock.System(), log
// nil slog.Default(), metrics nil counts nothing. Errors: a missing collaborator, a zero node or an
// interval or ttl that is not positive.
func NewPresenceWriter(cfg PresenceWriterConfig, hub *Hub, store PresenceStore, clk clock.Clock, log *slog.Logger,
	metrics PresenceMetrics) (*PresenceWriter, error) {
	if hub == nil || store == nil || cfg.Node.IsZero() || cfg.FlushInterval <= 0 || cfg.TTL <= 0 {
		return nil, errors.New("gateway: the presence writer needs the hub, the store, the node and positive " +
			"cluster.presence_flush_interval and cluster.presence_ttl")
	}
	if clk == nil {
		clk = clock.System()
	}
	if log == nil {
		log = slog.Default()
	}
	return &PresenceWriter{cfg: cfg, hub: hub, store: store, clk: clk, log: log, metrics: metrics}, nil
}

// Flush writes the current rows of the hub once (see PresenceWriter). Errors: those of the store,
// after they were logged and counted.
func (w *PresenceWriter) Flush(ctx context.Context) error {
	now := w.clk.Now()
	err := w.store.Flush(ctx, w.cfg.Node, w.hub.PresenceRows(), now, now.Add(-w.cfg.TTL))
	if err != nil {
		w.failed(ctx)
	}
	return err
}

// failed logs and counts one failed write.
func (w *PresenceWriter) failed(ctx context.Context) {
	w.log.LogAttrs(ctx, slog.LevelWarn, msgPresenceWrite, slog.String(logKeyErrorType, errorTypePresenceWrite))
	if w.metrics != nil {
		w.metrics.PresenceWriteFailed(ctx)
	}
}

// Start runs the flush every cluster.presence_flush_interval until Stop, the first one at once.
func (w *PresenceWriter) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	w.mu.Lock()
	w.cancel, w.done = cancel, done
	w.mu.Unlock()
	go func() {
		defer close(done)
		t := time.NewTicker(w.cfg.FlushInterval)
		defer t.Stop()
		for {
			_ = w.Flush(ctx)
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
		}
	}()
}

// Stop is the stop step of the writer (after the connections closed, 02 15.3): it ends the loop
// and deletes every row of the node within ctx. Errors: those of the delete, after they were
// logged and counted (the rows expire after cluster.presence_ttl).
func (w *PresenceWriter) Stop(ctx context.Context) error {
	w.mu.Lock()
	cancel, done := w.cancel, w.done
	w.mu.Unlock()
	if cancel != nil {
		cancel()
		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if err := w.store.DeleteNode(ctx, w.cfg.Node); err != nil {
		w.failed(ctx)
		return err
	}
	return nil
}
