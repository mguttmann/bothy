// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sort"
	gosync "sync"

	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// OpHandler handles one client op on c with its decoded body d. It returns nil after a success,
// which the dispatcher accepts only when the handler called authorize with the op's action; an
// *OpError for a failure that keeps the connection (error frame); a *CloseError for a failure
// that ends it; any other error ends the connection with 4000.
type OpHandler func(ctx context.Context, c *Connection, d any) error

// The ops that open a session; every other registered op needs one.
const (
	opIdentify = "identify"
	opResume   = "resume"
)

// Log messages of the dispatcher.
const (
	msgNotChecked = "gateway op without authorization check"
	msgOpFailed   = "gateway op failed"
)

// opEntry is one registered op.
type opEntry struct {
	action  authorize.Action
	handler OpHandler
}

// Dispatcher is the op registry of the gateway (02 "Nachweis der Prüfung", B7): every client op
// is registered with the action of 05 "WebSocket-Ops und Aktion" and its handler. Dispatch
// enforces the order of the protocol (identify or resume first, then never again), decodes the
// body through the FrameDecoder and, after a successful handler, requires a check of the op's
// action in the check log of kernel/authorize; a handler without it closes the connection with
// 4000 and raises the alarm counter. It is safe for concurrent use; register before serving.
type Dispatcher struct {
	mu      gosync.RWMutex
	ops     map[string]opEntry
	decoder FrameDecoder
	alarm   ProofAlarm
	log     *slog.Logger
}

// newDispatcher returns an empty registry over the decoder, the alarm and the log.
func newDispatcher(dec FrameDecoder, alarm ProofAlarm, log *slog.Logger) *Dispatcher {
	return &Dispatcher{ops: map[string]opEntry{}, decoder: dec, alarm: alarm, log: log}
}

// Register registers the client op op with action and handler. Errors: an empty op, a nil
// handler or an op registered before. internal/wire registers exactly the ops of
// gateway_ops.enabled (routes.GatewayOps); every other op is unknown (4001).
//
// Example:
//
//	err := d.Register("heartbeat", authorize.ActionGatewayConnect, handler)
func (d *Dispatcher) Register(op string, action authorize.Action, handler OpHandler) error {
	if op == "" || handler == nil || action == "" {
		return errors.New("gateway: Register needs an op, an action and a handler")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.ops[op]; ok {
		return fmt.Errorf("gateway: op %q is registered twice", op)
	}
	d.ops[op] = opEntry{action: action, handler: handler}
	return nil
}

// Ops returns the registered ops, sorted.
func (d *Dispatcher) Ops() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make([]string, 0, len(d.ops))
	for op := range d.ops {
		out = append(out, op)
	}
	sort.Strings(out)
	return out
}

// lookup returns the entry of op.
func (d *Dispatcher) lookup(op string) (opEntry, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	e, ok := d.ops[op]
	return e, ok
}

// Dispatch runs op with the raw body raw on c in the order of 04 section 6.4: an unregistered op
// closes with 4001 at any time; before identify or resume every other op closes with 4003; after
// it identify and resume close with 4005; a body that fails the decoder closes with 4002. Each
// op runs with its own check log and request memo of kernel/authorize, so the proof sees only
// this op's checks and the rights version is read again per op.
func (d *Dispatcher) Dispatch(ctx context.Context, c *Connection, op string, raw json.RawMessage) {
	e, ok := d.lookup(op)
	if !ok {
		c.Close(domain.CloseUnknownOp, "")
		return
	}
	opens := op == opIdentify || op == opResume
	switch identified := c.identified(); {
	case !identified && !opens:
		c.Close(domain.CloseNotIdentified, "")
		return
	case identified && opens:
		c.Close(domain.CloseAlreadyIdentified, "")
		return
	}
	v, err := d.decoder.Decode(op, raw)
	if err != nil {
		c.Close(domain.CloseDecodeError, "")
		return
	}
	ctx = authorize.WithRequestScope(authorize.WithCheckLog(ctx))
	err = e.handler(ctx, c, v)
	var ce *CloseError
	var oe *OpError
	switch {
	case err == nil:
		if !proven(ctx, e.action) {
			d.alarm.NotChecked(ctx, op, e.action)
			d.log.LogAttrs(ctx, slog.LevelError, msgNotChecked,
				slog.String(logKeyAction, string(e.action)), slog.String(logKeyReason, errs.CauseAuthzNotChecked))
			c.Close(domain.CloseUnknownError, "")
		}
	case errors.As(err, &oe):
		_ = c.conn.Send(ErrorFrame{Op: op, Code: oe.Code, Detail: oe.Detail, Ref: oe.Ref})
	case errors.As(err, &ce):
		c.Close(ce.Code, ce.Reason)
	default:
		d.log.LogAttrs(ctx, slog.LevelError, msgOpFailed, slog.String(logKeyErrorType, errs.KindOf(err).String()))
		c.Close(domain.CloseUnknownError, "")
	}
}

// proven reports whether the check log of ctx holds an authorize check of action.
func proven(ctx context.Context, action authorize.Action) bool {
	return slices.ContainsFunc(authorize.Checks(ctx), func(ch authorize.Check) bool {
		return ch.Action == string(action) && ch.Kind == authorize.CheckAuthorize
	})
}
