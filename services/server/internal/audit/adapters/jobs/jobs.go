// SPDX-License-Identifier: Apache-2.0

// Package jobs is the adapter of the audit module to platform/jobs: the periodic job
// audit.chain_verify (docs/spec/02-architecture.md section 12.3, 19 SEC-AUD-2), which recomputes
// the chain since the last checkpoint, signs the next one and hands it to the sink.
package jobs

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
)

// KindChainVerify is the kind of the job.
const KindChainVerify = "audit.chain_verify"

// ChainVerify are the arguments of the job; it has none, every run reads the log, the last
// checkpoint and the clock (02 "Rules for jobs").
type ChainVerify struct{}

// Kind returns KindChainVerify.
func (ChainVerify) Kind() string { return KindChainVerify }

// Verifier is the use case the job runs (app.Log.RunCheckpoint).
type Verifier interface {
	// RunCheckpoint runs one verification; a detected break is no error, a storage, signing or
	// sink error is.
	RunCheckpoint(ctx context.Context) error
}

// Handler works the job: one run of the verification. A run signs at most the head it verified
// and exports only what is not exported, so a retry after an error is safe.
type Handler struct {
	// Verifier is the use case.
	Verifier Verifier
}

// Work runs the use case once; its error makes River retry the job.
func (h Handler) Work(ctx context.Context, _ jobs.Job[ChainVerify]) error {
	return h.Verifier.RunCheckpoint(ctx)
}

// Register adds the handler on the queue maintenance and the periodic job every interval
// (audit.checkpoint_interval). Every role builds the same registry; only worker and all run River,
// so the job runs there. The error is the first rejected registration or an interval that is not
// positive.
func Register(reg *jobs.Registry, v Verifier, interval time.Duration) error {
	if interval <= 0 {
		return errors.New("audit: audit.checkpoint_interval must be positive")
	}
	if err := jobs.Register[ChainVerify](reg, Handler{Verifier: v}, jobs.HandlerOptions{Queue: jobs.QueueMaintenance}); err != nil {
		return err
	}
	return reg.AddPeriodic(jobs.PeriodicJob{Args: ChainVerify{}, Interval: interval})
}
