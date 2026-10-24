// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"log/slog"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/terminal/app"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// The constant log record of a dropped signal (M2-47 Design note 14): never a payload, an ID or an
// address.
const (
	msgSignalDropped = "terminal signal dropped"
	reasonMalformed  = "malformed"
)

// Reconciler runs the reconciliation of a node after its LISTEN connection came back (M2-47
// Design note 9); the wsrelay hub implements it.
type Reconciler interface {
	Reconcile(ctx context.Context)
}

// SignalRoute is the terminal's route of the one LISTEN ephemeral connection of an api process
// (pg.EphemeralListener; M2-47 Design note 5, shared since M7-17): it decodes each payload
// strictly and hands each terminal signal to the handler; a malformed terminal payload and a
// payload that is no JSON object with a kind are dropped with one log record (the gateway's route
// logs only its own kinds), a foreign kind (the gateway's) is ignored. After the listener's connection
// came back it runs the reconciliation.
type SignalRoute struct {
	handler   app.SignalHandler
	reconcile Reconciler
	log       *slog.Logger
}

// Compile-time check.
var _ pg.EphemeralRoute = (*SignalRoute)(nil)

// NewSignalRoute returns the route over handler and reconcile; log nil means slog.Default().
// Errors: a missing collaborator.
func NewSignalRoute(handler app.SignalHandler, reconcile Reconciler, log *slog.Logger) (*SignalRoute, error) {
	if handler == nil || reconcile == nil {
		return nil, errors.New("postgres: the terminal signal route needs the handler and the reconciler")
	}
	if log == nil {
		log = slog.Default()
	}
	return &SignalRoute{handler: handler, reconcile: reconcile, log: log}, nil
}

// HandleEphemeral implements pg.EphemeralRoute.
func (r *SignalRoute) HandleEphemeral(ctx context.Context, payload []byte) {
	s, ours, err := domain.DecodeSignal(payload)
	switch {
	case err != nil:
		r.log.LogAttrs(ctx, slog.LevelWarn, msgSignalDropped, slog.String(attr.Reason, reasonMalformed))
	case ours:
		r.handler.HandleSignal(ctx, s)
	}
}

// Reconcile implements pg.EphemeralRoute.
func (r *SignalRoute) Reconcile(ctx context.Context) { r.reconcile.Reconcile(ctx) }
