// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
)

// Log messages of the health monitor; constant text.
const (
	msgScannerUnreachable = "virus scanner unreachable"
	msgScannerBack        = "virus scanner reachable again"
	msgScannerOff         = "virus scan is off (file.scan.mode = off)"
	msgScanQueueFailed    = "file scan queue count failed"
)

// ScanHealth is the health monitor of the worker (docs/spec/12-dateien.md 5.3, brief M3-04 design
// note 11): every file.scan.health_interval it pings clamd (mode required only), keeps the result
// in memory for files.ScannerHealth (per process, docs/DECISIONS.md D-M3-04-11) and records
// bothy.file.scan.queue. It never changes the readiness of the process: a missing clamd keeps
// files pending while the worker serves its other queues. It is safe for concurrent use.
type ScanHealth struct {
	mode     string
	interval time.Duration
	scanner  Scanner
	scans    ScanRepository
	metrics  ScanMetrics
	clock    clock.Clock
	log      *slog.Logger

	mu    sync.Mutex
	state files.ScannerState
}

// Compile-time check: ScanHealth is the ScannerHealth of the port.
var _ files.ScannerHealth = (*ScanHealth)(nil)

// NewScanHealth returns the monitor for cfg.Mode and cfg.HealthInterval. scanner may be nil with
// mode off; metrics nil records nothing, clk nil means clock.System(), log nil slog.Default().
func NewScanHealth(cfg ScanConfig, scanner Scanner, scans ScanRepository, metrics ScanMetrics, clk clock.Clock, log *slog.Logger) (*ScanHealth, error) {
	if err := cfg.check(); err != nil {
		return nil, err
	}
	if scans == nil || (scanner == nil && cfg.Mode == ScanModeRequired) {
		return nil, errors.New("files: NewScanHealth needs the scanner and the repository")
	}
	if metrics == nil {
		metrics = noScanMetrics{}
	}
	if clk == nil {
		clk = clock.System()
	}
	if log == nil {
		log = slog.Default()
	}
	h := &ScanHealth{mode: cfg.Mode, interval: cfg.HealthInterval, scanner: scanner, scans: scans, metrics: metrics, clock: clk, log: log,
		state: files.ScannerState{Status: files.ScannerUnknown}}
	if cfg.Mode == ScanModeOff {
		h.state.Status = files.ScannerOff
	}
	return h, nil
}

// ScannerState implements files.ScannerHealth.
func (h *ScanHealth) ScannerState() files.ScannerState {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.state
}

// Check runs one round: PING with the interval as deadline (mode required), the state, one log
// line per change of status (Warn when clamd becomes unreachable, Info when it is back), and the
// queue count.
func (h *ScanHealth) Check(ctx context.Context) {
	if h.mode == ScanModeRequired {
		pctx, cancel := context.WithTimeout(ctx, h.interval)
		err := h.scanner.Ping(pctx)
		cancel()
		status := files.ScannerOK
		if err != nil {
			status = files.ScannerUnreachable
		}
		h.mu.Lock()
		prev := h.state.Status
		h.state = files.ScannerState{Status: status, CheckedAt: h.clock.Now().UTC()}
		h.mu.Unlock()
		switch {
		case status == prev:
		case status == files.ScannerUnreachable:
			h.log.WarnContext(ctx, msgScannerUnreachable)
		case prev == files.ScannerUnreachable:
			h.log.InfoContext(ctx, msgScannerBack)
		}
	}
	n, err := h.scans.CountAwaitingScan(ctx)
	if err != nil {
		if ctx.Err() == nil {
			h.log.WarnContext(ctx, msgScanQueueFailed, slog.String(logKeyErrorType, "database"))
		}
		return
	}
	h.metrics.ScanQueue(ctx, n)
}

// Run logs once that the scan is off (mode off), then runs Check at once and every
// file.scan.health_interval until ctx ends.
func (h *ScanHealth) Run(ctx context.Context) {
	if h.mode == ScanModeOff {
		h.log.WarnContext(ctx, msgScannerOff)
	}
	t := time.NewTicker(h.interval)
	defer t.Stop()
	for {
		h.Check(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
