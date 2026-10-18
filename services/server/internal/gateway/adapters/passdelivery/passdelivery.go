// SPDX-License-Identifier: Apache-2.0

// Package passdelivery is the binding of the delivery hook of agent consumers (gateway/app
// DeliveryHook, brief M2-07 note 7) until M2-08: every envelope is delivered unchanged. M2-08
// binds runs.Delivery there instead, which checks invocation events and adds a fresh invocation
// token before each delivery.
//
// Example:
//
//	engine, err := app.NewConsumerEngine(cfg, app.ConsumerDeps{Hook: passdelivery.Hook{}, ...})
package passdelivery

import (
	"context"

	gatewayapp "github.com/OWNER/bothy/services/server/internal/gateway/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// Hook implements gatewayapp.DeliveryHook and delivers every envelope unchanged.
type Hook struct{}

// Compile-time check: the hook of the consumer engine.
var _ gatewayapp.DeliveryHook = Hook{}

// Prepare implements gatewayapp.DeliveryHook: ev, true, nil.
func (Hook) Prepare(_ context.Context, _ ids.ID, ev sync.Envelope) (sync.Envelope, bool, error) {
	return ev, true, nil
}
