// SPDX-License-Identifier: Apache-2.0

package fakeadapter

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/gateway/adapters/wstest"
)

// heartbeatPath is sendHeartbeat.
const heartbeatPath = "/agents/me/heartbeat"

// The billing mode and tool class the fake adapter reports and uses: the defaults of an agent
// (03 agents), which the measured path needs.
const (
	billingMode = "organization"
	toolClass   = "chat"
)

// HeartbeatBody returns the body of sendHeartbeat (design note 3 g; 09 section 4.8): state
// online, billing_modes_supported [organization], reported_tool_class chat and details.active_runs
// with the runs active lists (sorted, so equal states give equal bodies).
func HeartbeatBody(active []string) rest.Heartbeat {
	runs := slices.Clone(active)
	slices.Sort(runs)
	if runs == nil {
		runs = []string{}
	}
	modes := []rest.BillingMode{billingMode}
	class := rest.ToolClass(toolClass)
	return rest.Heartbeat{State: rest.HeartbeatStateOnline, BillingModesSupported: &modes, ReportedToolClass: &class,
		Details: &rest.JsonObject{"active_runs": runs}}
}

// heartbeat sends one heartbeat with the running runs of the adapter and returns the interval of
// HeartbeatAck.interval_ms, after which the next one is due. The error reports a status other
// than 200 (with its problem code) or an answer that does not decode strictly.
func (a *Adapter) heartbeat(ctx context.Context) (time.Duration, error) {
	status, body := a.api.Do(ctx, http.MethodPost, heartbeatPath, HeartbeatBody(a.Active()))
	a.count()
	if status != http.StatusOK {
		return 0, fmt.Errorf("sendHeartbeat = %d %s", status, problemCode(body))
	}
	var ack rest.HeartbeatAck
	if err := wstest.Strict(body, &ack); err != nil {
		return 0, fmt.Errorf("sendHeartbeat answer: %w", err)
	}
	if ack.IntervalMs <= 0 {
		return 0, fmt.Errorf("sendHeartbeat answer: interval_ms %d", ack.IntervalMs)
	}
	return time.Duration(ack.IntervalMs) * time.Millisecond, nil
}

// heartbeatLoop sends a heartbeat every interval of the last answer until ctx ends.
func (a *Adapter) heartbeatLoop(ctx context.Context, interval time.Duration) {
	defer a.wg.Done()
	for {
		if waitUntil(ctx, time.Now().Add(interval)) != nil {
			return
		}
		next, err := a.heartbeat(ctx)
		if err != nil {
			if ctx.Err() == nil {
				a.fail("%v", err)
			}
			return
		}
		interval = next
	}
}
