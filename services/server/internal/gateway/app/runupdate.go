// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"strconv"

	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
)

// opRunUpdate is the op of a live-box patch over the gateway (04 "Ops", agents only).
const opRunUpdate = "run.update"

// RunPatcher is the part of runs.Runs the op run.update calls: the use case of patchRun, so both
// paths are one (M2-10 design note 1).
type RunPatcher interface {
	Patch(ctx context.Context, in runs.PatchRun) (runs.PatchResult, error)
}

// RunUpdate is the body of run.update (schema GatewayRunUpdate) with the values of the ops as
// received.
type RunUpdate struct {
	RunID       ids.ID
	BaseVersion int64
	Ops         []runs.PatchOp
	Ref         string
}

// runUpdateErrorKinds are the kinds of the problem codes run.update answers with an error frame
// while the connection stays (design note 11): validation_failed, forbidden, not_found, conflict
// and base_version_conflict, rate_limited.
var runUpdateErrorKinds = map[errs.Kind]bool{
	errs.KindBadRequest: true, errs.KindForbidden: true, errs.KindNotFound: true, errs.KindConflict: true,
	errs.KindRateLimited: true,
}

// runUpdate is the handler of run.update (M2-10 design note 11): rl.agent_runtime keyed by the
// agent before anything else, then runs.Runs.Patch with the actor of the connection, which
// authorizes run.update (agent-self) and audits a denial. A success sends no frame (asyncapi
// defines no reply; the adapter's next base is base_version + 1). A failure answers error {op,
// code, detail, ref} and keeps the connection: detail names the pointer and code of the first
// field error of validation_failed (for example "/ops/0: patch_path_forbidden") and the reason of
// a conflict, never a value; GatewayError has no current, so after base_version_conflict the
// adapter loads getRun (ruling D-M2-10-14). A credential revoked since identify closes with 4012.
func (g *Gateway) runUpdate(ctx context.Context, c *Connection, d any) error {
	in, ok := d.(RunUpdate)
	if !ok {
		return closeWith(domain.CloseDecodeError)
	}
	if g.d.Runs == nil {
		return errNoRuns
	}
	_, a := c.current()
	ctx = actor.WithActor(ctx, a)
	if g.d.AgentLimiter != nil {
		allowed, retry, err := g.d.AgentLimiter.AllowAgent(a)
		if err != nil {
			return err
		}
		if !allowed {
			return &OpError{Code: string(errs.CodeRateLimited), Detail: "retry_after=" + strconv.Itoa(retry), Ref: in.Ref}
		}
	}
	_, err := g.d.Runs.Patch(ctx, runs.PatchRun{RunID: in.RunID, BaseVersion: in.BaseVersion, Ops: in.Ops})
	if err == nil {
		return nil
	}
	e, ok := errs.As(err)
	switch {
	case ok && runUpdateErrorKinds[e.Kind()]:
		return &OpError{Code: string(e.PublicCode()), Detail: runUpdateDetail(e), Ref: in.Ref}
	case ok && e.Kind() == errs.KindUnauthenticated:
		return closeWith(domain.CloseRevoked)
	}
	return err
}

// errNoRuns ends run.update on a gateway built without the runs module (a wiring fault).
var errNoRuns = errors.New("gateway: run.update without the runs module")

// runUpdateDetail is the detail of an error frame of run.update: pointer and code of the first
// field error, otherwise the reason; never a value or a document.
func runUpdateDetail(e *errs.Error) string {
	if len(e.Fields) > 0 {
		f := e.Fields[0]
		return f.Pointer + ": " + f.Code
	}
	return e.Reason
}
