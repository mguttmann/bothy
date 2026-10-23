// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// The values of the payloads the consumer reads (docs/api/openapi.yaml).
const (
	channelKindDM    = "dm"
	originAgent      = "agent"
	originImport     = "import"
	originSystem     = "system"
	messageKindRun   = "run"
	messageKindSys   = "system"
	runCompleted     = "completed"
	runFailed        = "failed"
	runCancelled     = "cancelled"
	runWaitingInput  = "waiting_input"
	expiryGrant      = "grant"
	expiryMembership = "group_membership"
)

// messagePayload is the part of MessagePayload the consumer reads: IDs, flags and kinds, never
// the content.
type messagePayload struct {
	Message struct {
		ID        ids.ID `json:"id"`
		ThreadID  ids.ID `json:"thread_id"`
		ChannelID ids.ID `json:"channel_id"`
		ThreadSeq int64  `json:"thread_seq"`
		Author    struct {
			ID   ids.ID `json:"id"`
			Kind string `json:"kind"`
		} `json:"author"`
		Origin     string   `json:"origin"`
		Kind       string   `json:"kind"`
		Silent     bool     `json:"silent"`
		MentionIDs []ids.ID `json:"mention_ids"`
	} `json:"message"`
}

// runPayload is the part of RunStateChangedPayload the consumer reads.
type runPayload struct {
	Run struct {
		ID          ids.ID `json:"id"`
		ThreadID    ids.ID `json:"thread_id"`
		ChannelID   ids.ID `json:"channel_id"`
		AgentID     ids.ID `json:"agent_id"`
		TriggeredBy ids.ID `json:"triggered_by"`
		State       string `json:"state"`
	} `json:"run"`
}

// expiringPayload is the part of AccessExpiringPayload the consumer reads.
type expiringPayload struct {
	Kind    string `json:"kind"`
	GrantID ids.ID `json:"grant_id"`
	GroupID ids.ID `json:"group_id"`
}

// devicePayload is DevicePendingPayload.
type devicePayload struct {
	DeviceID ids.ID `json:"device_id"`
	Platform string `json:"platform"`
}

// readStatePayload is the part of ReadStatePayload the consumer reads.
type readStatePayload struct {
	ReadState struct {
		ThreadID    ids.ID `json:"thread_id"`
		LastReadSeq int64  `json:"last_read_seq"`
	} `json:"read_state"`
}

// decode reads the payload of ev into v; a payload that does not decode is an internal error,
// because the appender validated it against its schema.
func decode(ev sync.Event, v any) error {
	if err := json.Unmarshal(ev.Payload, v); err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "notify: a payload of "+ev.Type+" does not decode")
	}
	return nil
}

// group is one kind of hint for a set of candidates of one event (design note 4).
type group struct {
	kind domain.Kind
	// users are the candidates, ascending, without duplicates.
	users []ids.ID
	// threadID, channelID and spaceID are the place of a kind with thread.
	threadID, channelID, spaceID ids.ID
	// messageID and targetID are the pointers of the hint.
	messageID, targetID ids.ID
	// audience is the receiver of a personal event (security, admin).
	audience ids.ID
	// excludeDevice is the pending device of a security hint.
	excludeDevice ids.ID
	// directMessage marks a kind dm (its level step uses the DM level).
	directMessage bool
	// ev is the triggering event (t0 of 13 9 is its occurred_at).
	ev sync.Event
}

// onMessage maps message.created (design note 4 a): nothing for silent, import, system, anchors
// and system lines; dm in a DM channel; otherwise mention to the mentioned people and message to
// the other readers, except an agent message during an active run (note 5).
func (p *Push) onMessage(ctx context.Context, ev sync.Event) error {
	if ev.Redacted {
		return nil
	}
	var pl messagePayload
	if err := decode(ev, &pl); err != nil {
		return err
	}
	m := pl.Message
	if m.Silent || m.Origin == originImport || m.Origin == originSystem || m.Kind == messageKindRun || m.Kind == messageKindSys {
		return nil
	}
	info, err := p.d.Threads.Info(ctx, m.ThreadID)
	if err != nil {
		return err
	}
	base := group{threadID: m.ThreadID, channelID: info.ChannelID, spaceID: info.SpaceID, messageID: m.ID, ev: ev}
	author := m.Author.ID
	if info.ChannelKind == channelKindDM {
		readers, err := p.d.Readers.Readers(ctx, info.ChannelID)
		if err != nil {
			return err
		}
		g := base
		g.kind, g.directMessage, g.users = domain.KindDM, true, without(readers, author)
		return p.run(ctx, g)
	}
	mentioned := without(uniqueSorted(m.MentionIDs), author)
	mg := base
	mg.kind, mg.users = domain.KindMention, mentioned
	if err := p.run(ctx, mg); err != nil {
		return err
	}
	if m.Origin == originAgent {
		during, err := p.duringRun(ctx, m.ThreadID, m.ThreadSeq, ev.Seq, author)
		if err != nil || during {
			return err
		}
	}
	readers, err := p.d.Readers.Readers(ctx, info.ChannelID)
	if err != nil {
		return err
	}
	g := base
	g.kind = domain.KindMessage
	g.users = slices.DeleteFunc(without(readers, author), func(u ids.ID) bool { return slices.Contains(mentioned, u) })
	return p.run(ctx, g)
}

// onRun maps run.state_changed (design note 4 b): input_needed, run_done and run_failed to the
// starter, and at completed, failed and cancelled the run's single message hint to the other
// readers (at cancelled the starter included), all pointing at the last agent message of the run.
func (p *Push) onRun(ctx context.Context, ev sync.Event) error {
	var pl runPayload
	if err := decode(ev, &pl); err != nil {
		return err
	}
	r := pl.Run
	var starterKind domain.Kind
	switch r.State {
	case runWaitingInput:
		starterKind = domain.KindInputNeeded
	case runCompleted:
		starterKind = domain.KindRunDone
	case runFailed:
		starterKind = domain.KindRunFailed
	case runCancelled:
	default:
		return nil
	}
	info, err := p.d.Threads.Info(ctx, r.ThreadID)
	if err != nil {
		return err
	}
	last, found, err := p.lastAgentMessage(ctx, r.ID, r.ThreadID, r.AgentID, ev.Seq)
	if err != nil {
		return err
	}
	base := group{threadID: r.ThreadID, channelID: info.ChannelID, spaceID: info.SpaceID, ev: ev}
	if found {
		base.messageID = last
	}
	if starterKind != "" {
		g := base
		g.kind, g.users = starterKind, []ids.ID{r.TriggeredBy}
		if err := p.run(ctx, g); err != nil {
			return err
		}
	}
	if r.State == runWaitingInput || !found {
		return nil
	}
	readers, err := p.d.Readers.Readers(ctx, info.ChannelID)
	if err != nil {
		return err
	}
	others := without(readers, r.AgentID)
	if r.State != runCancelled {
		others = without(others, r.TriggeredBy)
	}
	g := base
	g.kind, g.users = domain.KindMessage, others
	if info.ChannelKind == channelKindDM {
		// A DM channel knows only the kind dm for messages (13 2); the run's message hint there
		// goes as dm to the participants.
		g.kind, g.directMessage = domain.KindDM, true
	}
	return p.run(ctx, g)
}

// onAccessExpiring maps access.expiring (design note 4 c): admin to the audience, with the grant
// or group as target.
func (p *Push) onAccessExpiring(ctx context.Context, ev sync.Event) error {
	var pl expiringPayload
	if err := decode(ev, &pl); err != nil {
		return err
	}
	target := pl.GrantID
	if pl.Kind == expiryMembership {
		target = pl.GroupID
	}
	if ev.AudiencePrincipalID.IsZero() || target.IsZero() {
		return nil
	}
	return p.run(ctx, group{kind: domain.KindAdmin, users: []ids.ID{ev.AudiencePrincipalID}, targetID: target,
		audience: ev.AudiencePrincipalID, ev: ev})
}

// onDevicePending maps device.pending (design note 4 d, R-AZ-31): security to the human who owns
// the device, routed only to the other active devices of that human.
func (p *Push) onDevicePending(ctx context.Context, ev sync.Event) error {
	var pl devicePayload
	if err := decode(ev, &pl); err != nil {
		return err
	}
	if ev.AudiencePrincipalID.IsZero() || pl.DeviceID.IsZero() {
		return nil
	}
	return p.run(ctx, group{kind: domain.KindSecurity, users: []ids.ID{ev.AudiencePrincipalID}, targetID: pl.DeviceID,
		audience: ev.AudiencePrincipalID, excludeDevice: pl.DeviceID, ev: ev})
}

// activeHumans keeps the active humans among users (agents, apps, disabled and deleted people
// are no recipients, 13 2); the order is kept.
func (p *Push) activeHumans(ctx context.Context, users []ids.ID) ([]ids.ID, error) {
	if len(users) == 0 {
		return nil, nil
	}
	views, err := p.d.Principals.Principals(ctx, users)
	if err != nil {
		return nil, err
	}
	out := make([]ids.ID, 0, len(users))
	for _, u := range users {
		if v, ok := views[u]; ok && v.Kind == actor.KindUser && v.Status == identity.StatusActive {
			out = append(out, u)
		}
	}
	return out, nil
}

// uniqueSorted returns list ascending without duplicates and zero IDs, as a fresh slice.
func uniqueSorted(list []ids.ID) []ids.ID {
	out := slices.DeleteFunc(slices.Clone(list), ids.ID.IsZero)
	slices.SortFunc(out, compareIDs)
	return slices.Compact(out)
}

// without returns list without x, as a fresh slice.
func without(list []ids.ID, x ids.ID) []ids.ID {
	return slices.DeleteFunc(slices.Clone(list), func(u ids.ID) bool { return u == x })
}

// internal returns an internal error with a fixed text.
func internal(msg string) error { return errs.New(errs.CodeInternalError, msg) }
