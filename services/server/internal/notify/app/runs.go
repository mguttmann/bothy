// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"math"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// The resource kinds of the history reads.
const (
	resourceRun     = "run"
	resourceMessage = "message"
	resourceThread  = "thread"
)

// duringRun reports whether the agent message at thread position threadSeq, committed with the
// log seq seq by agent, was written during an active run of that agent in the thread (13 2.1,
// brief M3-19 design note 5 a, D-M3-19-4). The answer comes from the commit order, never from
// the run's state at processing time: the newest anchor before the message names the only
// candidate run (R-DM-14), and its last run.state_changed before seq must be active and name the
// same agent. No anchor, no retained event or another agent: not during a run.
func (p *Push) duringRun(ctx context.Context, threadID ids.ID, threadSeq, seq int64, agent ids.ID) (bool, error) {
	anchor, found, err := p.d.Messages.LastAnchorBefore(ctx, threadID, threadSeq)
	if err != nil || !found || anchor.RunID.IsZero() {
		return false, err
	}
	events, err := p.d.History.ResourceEvents(ctx, sync.ResourceQuery{
		ResourceKind: resourceRun, ResourceID: anchor.RunID, Types: []string{string(authorize.EventRunStateChanged)}, Before: seq,
	})
	if err != nil || len(events) == 0 {
		return false, err
	}
	var pl runPayload
	if err := decode(events[len(events)-1], &pl); err != nil {
		return false, err
	}
	return domain.RunActive(pl.Run.State) && pl.Run.AgentID == agent, nil
}

// lastAgentMessage returns the last agent message of run runID of agent in thread, as of the
// event with seq end that ended the run (design note 5 b): the newest message of the agent after
// the run's anchor whose message.created was committed before end. Messages written after the end
// are skipped; the loop ends at the anchor. found is false when the run wrote none. The ref
// carries the message's thread_seq (Hint.MessageSeq, M3-23).
func (p *Push) lastAgentMessage(ctx context.Context, runID, threadID, agent ids.ID, end int64) (messaging.MessageRef, bool, error) {
	anchor, found, err := p.d.Messages.AnchorOf(ctx, runID)
	if err != nil || !found {
		return messaging.MessageRef{}, false, err
	}
	before := int64(math.MaxInt64)
	for {
		m, found, err := p.d.Messages.LastAgentMessageBefore(ctx, threadID, agent, anchor.ThreadSeq, before)
		if err != nil || !found {
			return messaging.MessageRef{}, false, err
		}
		created, err := p.d.History.ResourceEvents(ctx, sync.ResourceQuery{
			ResourceKind: resourceMessage, ResourceID: m.ID, Types: []string{string(authorize.EventMessageCreated)}, Before: end,
		})
		if err != nil {
			return messaging.MessageRef{}, false, err
		}
		if len(created) > 0 {
			return m, true, nil
		}
		before = m.ThreadSeq
	}
}
