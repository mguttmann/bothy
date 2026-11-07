// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package hostos

import "context"

// runRunner starts the programs of one run: none once the run ended, and each one ends with the
// run.
type runRunner struct {
	ctx context.Context //nolint:containedctx // the run is the scope; its values live only as long as it
	r   Runner
}

// Run implements Runner.
func (r runRunner) Run(ctx context.Context, c Cmd) (Output, error) {
	if err := context.Cause(r.ctx); err != nil {
		return Output{}, err
	}
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	stop := context.AfterFunc(r.ctx, func() { cancel(context.Cause(r.ctx)) })
	defer stop()
	return r.r.Run(ctx, c)
}
