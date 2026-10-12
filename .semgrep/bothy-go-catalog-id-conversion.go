// SPDX-License-Identifier: Apache-2.0

// Test cases for the rule bothy-go-catalog-id-conversion (run by `make semgrep`).
package fixtures

import "github.com/OWNER/bothy/services/server/internal/kernel/authorize"

// ruleid: bothy-go-catalog-id-conversion
import . "github.com/OWNER/bothy/services/server/internal/kernel/authorize"

var _ = ActionMessageRead

func conversions(s string) {
	// ruleid: bothy-go-catalog-id-conversion
	_ = authorize.Action("message.read")
	// ruleid: bothy-go-catalog-id-conversion
	_ = authorize.EventType(s)
	// ruleid: bothy-go-catalog-id-conversion
	_ = authorize.Op("heartbeat")
	// ruleid: bothy-go-catalog-id-conversion
	_ = authorize.ResourceKind(s)
	// ruleid: bothy-go-catalog-id-conversion
	_ = authorize.ChannelOp("host_control/hello")
}

// ruleid: bothy-go-catalog-id-conversion
var declared authorize.Action = "message.read"

// ruleid: bothy-go-catalog-id-conversion
const declaredOp authorize.Op = "heartbeat"

type catalog interface {
	Spec(a authorize.Action) error
}

func implicit(c catalog, r *authorize.Registry) {
	// ruleid: bothy-go-catalog-id-conversion
	_ = c.Spec("no.such.action")
	// ruleid: bothy-go-catalog-id-conversion
	r.Event("messaging", "no.such.event")
	// ruleid: bothy-go-catalog-id-conversion
	r.Op("gateway", "no_such_op")
	// ruleid: bothy-go-catalog-id-conversion
	r.ChannelOp("terminal", "host_control/hello")
	// ok: bothy-go-catalog-id-conversion
	r.Event("messaging", authorize.EventMessageCreated)
	// ok: bothy-go-catalog-id-conversion
	_ = c.Spec(authorize.ActionMessageRead)
}

// ok: bothy-go-catalog-id-conversion
var fromConstant authorize.Action = authorize.ActionMessageRead

func allowed(s string) {
	// ok: bothy-go-catalog-id-conversion
	_ = authorize.ActionMessageRead
	// ok: bothy-go-catalog-id-conversion
	_, _ = authorize.ParseAction(s)
	// ok: bothy-go-catalog-id-conversion
	_, _ = authorize.ParseEventType(s)
	// ok: bothy-go-catalog-id-conversion
	_, _ = authorize.ParseOp(s)
	// ok: bothy-go-catalog-id-conversion
	_, _ = authorize.ParseResourceKind(s)
	// ok: bothy-go-catalog-id-conversion
	_, _ = authorize.ParseChannelOp(s)
	// ok: bothy-go-catalog-id-conversion
	_ = string(authorize.EventAccessChanged)
}
