// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
)

// The ephemeral types that cross nodes (02 section 11.6, the table "Ephemeral events") and the
// resource kinds of their envelopes.
const (
	eventAgentPresence  = string(authorize.EventAgentPresenceChanged)
	eventHint           = string(authorize.EventNotificationCreated)
	resourceKindAgent   = "agent"
	resourceKindUser    = "user"
	presenceSchema      = 1
	hintSchema          = 1
	msgSignalDropped    = "gateway signal dropped"
	msgSignalNotSent    = "gateway signal not sent"
	msgSignalUnreadable = "gateway signal not applied"
	reasonMalformedSig  = "malformed"
	logKeySignalKind    = "bothy.signal.kind"
)

// ClusterConfig holds the configuration of the gateway across several api nodes (M7-17).
type ClusterConfig struct {
	// Node is the process instance (a UUIDv7 new at every start, gateway_presence.node_id); every
	// signal carries it and a node ignores its own.
	Node ids.ID
	// TypingTTL is sync.typing_ttl: expires_at of a typing.started rebuilt from a signal.
	TypingTTL time.Duration
}

// ClusterDeps are the collaborators of the cluster.
type ClusterDeps struct {
	// Sender sends the signals (NOTIFY ephemeral).
	Sender SignalSender
	// Threads gives the place of a thread for the scope of a rebuilt event (chat); required on a
	// node that receives (a process with a hub).
	Threads Threads
	// Clock stamps rebuilt events; nil means clock.System().
	Clock clock.Clock
	// Log receives dropped and unsent signals, never a payload; nil means slog.Default().
	Log *slog.Logger
}

// Cluster is the gateway across several api nodes (M7-17, docs/spec/02-architecture.md sections
// 11.6 and 16; docs/decisions.d/M7-17.md D-M7-17-1 to D-M7-17-5): the publisher decorator that
// forwards the ephemeral events of 02 11.6 as signals, the sender of the device and consumer
// signals, and the route of the shared LISTEN ephemeral connection, which rebuilds each signal of
// another node from the database and hands it to the hub, the coalescer, the session store or the
// consumer engine of this node. It is safe for concurrent use.
type Cluster struct {
	cfg ClusterConfig
	d   ClusterDeps

	hub    *Hub
	gw     *Gateway
	engine *ConsumerEngine
}

// NewCluster returns the cluster of the node cfg.Node. Errors: a zero node, a missing sender or a
// typing ttl that is not positive.
func NewCluster(cfg ClusterConfig, d ClusterDeps) (*Cluster, error) {
	if cfg.Node.IsZero() || d.Sender == nil || cfg.TypingTTL <= 0 {
		return nil, errors.New("gateway: the cluster needs the node, the sender and a positive sync.typing_ttl")
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &Cluster{cfg: cfg, d: d}, nil
}

// Bind makes the hub, the gateway and the consumer engine of this process the receivers of the
// signals of other nodes (nil for a process without them: the worker). internal/wire calls it
// once before the listener starts. Errors: a hub without the threads port.
func (c *Cluster) Bind(hub *Hub, gw *Gateway, engine *ConsumerEngine) error {
	if hub != nil && c.d.Threads == nil {
		return errors.New("gateway: a receiving cluster needs the threads of chat")
	}
	c.hub, c.gw, c.engine = hub, gw, engine
	return nil
}

// Node returns the node of the process.
func (c *Cluster) Node() ids.ID { return c.cfg.Node }

// Publisher returns the EphemeralPublisher of the process with cluster.mode multi (and of a
// worker): every event goes to local first (nil in a process without a hub), then the four types
// of 02 11.6 (typing.started, run.updated, agent.presence_changed, notification.created) go to
// the other nodes as a signal with IDs only. The terminal's own types stay local (M2-47 sends its
// signals itself). Errors: only those of local.
func (c *Cluster) Publisher(local eventlog.EphemeralPublisher) eventlog.EphemeralPublisher {
	return clusterPublisher{c: c, local: local}
}

// clusterPublisher is the decorator of Cluster.Publisher.
type clusterPublisher struct {
	c     *Cluster
	local eventlog.EphemeralPublisher
}

// PublishEphemeral implements eventlog.EphemeralPublisher.
func (p clusterPublisher) PublishEphemeral(ctx context.Context, ev eventlog.EphemeralEvent) error {
	if p.local != nil {
		if err := p.local.PublishEphemeral(ctx, ev); err != nil {
			return err
		}
	}
	if s, ok := signalOf(ev); ok {
		p.c.send(ctx, s)
	}
	return nil
}

// signalOf returns the signal of ev, false for a type that does not cross nodes or an event
// without its IDs.
func signalOf(ev eventlog.EphemeralEvent) (domain.Signal, bool) {
	switch ev.Type {
	case eventTypingStarted:
		if ev.Actor == nil || ev.Actor.PrincipalID.IsZero() || ev.Scope.ThreadID.IsZero() {
			return domain.Signal{}, false
		}
		return domain.Signal{Kind: domain.SignalTyping, ThreadID: ev.Scope.ThreadID, PrincipalID: ev.Actor.PrincipalID}, true
	case eventRunUpdated:
		u, ok := ev.Payload.(runs.RunUpdated)
		if !ok {
			return domain.Signal{}, false
		}
		return domain.Signal{Kind: domain.SignalRun, RunID: u.RunID, ThreadID: u.ThreadID, Version: u.Version}, true
	case eventAgentPresence:
		var p agentPresenceBody
		if !reencode(ev.Payload, &p) {
			return domain.Signal{}, false
		}
		s := domain.Signal{Kind: domain.SignalAgentPresence, AgentID: ev.Resource.ID, State: p.State, BillingMode: p.ReportedBillingMode}
		if ev.Actor != nil {
			s.ActorID = ev.Actor.PrincipalID
		}
		return s, true
	case eventHint:
		var p hintBody
		if !reencode(ev.Payload, &p) || ev.Audience.IsZero() {
			return domain.Signal{}, false
		}
		return domain.Signal{Kind: domain.SignalHint, RecipientID: ev.Audience, NotificationID: p.NID, HintKind: p.Kind, ThreadID: p.ThreadID,
			MessageID: p.MessageID, TargetID: p.TargetID, Quiet: p.Quiet, Count: p.Count}, true
	}
	return domain.Signal{}, false
}

// reencode reads the payload v of another module into dst through its JSON form; only the members
// of dst are kept, so a signal never carries a member it does not name.
func reencode(v any, dst any) bool {
	b, err := json.Marshal(v)
	return err == nil && json.Unmarshal(b, dst) == nil
}

// agentPresenceBody is the schema AgentPresencePayload (agents, M2-20).
type agentPresenceBody struct {
	AgentID             string `json:"agent_id"`
	State               string `json:"state"`
	ReportedBillingMode string `json:"reported_billing_mode,omitempty"`
}

// hintBody is NotificationCreatedPayload as M3-23 fills it (notify): the nid, the kind, the
// pointers, the quiet flag and the count; absent members are omitted, never null (ADR 0053).
type hintBody struct {
	NID       ids.ID `json:"nid"`
	Kind      string `json:"kind"`
	ThreadID  ids.ID `json:"thread_id,omitzero"`
	MessageID ids.ID `json:"message_id,omitzero"`
	TargetID  ids.ID `json:"target_id,omitzero"`
	Quiet     bool   `json:"quiet"`
	Count     int64  `json:"count"`
}

// DeviceBound announces that the session of a device is now bound to a connection of this node
// (identify or resume, 04 6.1); bind orders the bindings of one device across nodes. The node that
// holds an older connection of the device closes it with 4017.
func (c *Cluster) DeviceBound(ctx context.Context, device, session, bind ids.ID) {
	c.send(ctx, domain.Signal{Kind: domain.SignalDevice, DeviceID: device, SessionID: session, BindID: bind})
}

// ConsumerTaken announces that session took a consumer of agent over on this node (04 15.1): the
// other nodes check their connections of the agent against the cursor row at once instead of at
// their next save (02 16, "Agent consumers").
func (c *Cluster) ConsumerTaken(ctx context.Context, agent, session ids.ID) {
	c.send(ctx, domain.Signal{Kind: domain.SignalConsumer, AgentID: agent, SessionID: session})
}

// send encodes s with the node and sends it; a failure is logged with the kind only and dropped
// (at most once, 04 9 point 4).
func (c *Cluster) send(ctx context.Context, s domain.Signal) {
	s.Node = c.cfg.Node
	b, err := domain.EncodeSignal(s)
	if err == nil {
		err = c.d.Sender.Send(context.WithoutCancel(ctx), b)
	}
	if err != nil {
		c.d.Log.LogAttrs(ctx, slog.LevelWarn, msgSignalNotSent, slog.String(logKeySignalKind, string(s.Kind)),
			slog.String(logKeyErrorType, errorTypeSignal))
	}
}

// errorTypeSignal is error.type of a signal that was not sent or not applied.
const errorTypeSignal = "cluster_signal"

// HandleEphemeral implements the route of the shared LISTEN ephemeral connection
// (platform/pg.EphemeralRoute): it decodes one payload; a foreign kind (the terminal's) is
// ignored, a malformed gateway payload is dropped with one log record, the node's own signal is
// ignored, and every other signal is applied. It never blocks on a connection.
func (c *Cluster) HandleEphemeral(ctx context.Context, payload []byte) {
	s, ours, err := domain.DecodeSignal(payload)
	switch {
	case !ours:
		return
	case err != nil:
		c.d.Log.LogAttrs(ctx, slog.LevelWarn, msgSignalDropped, slog.String(logKeyReason, reasonMalformedSig))
		return
	case s.Node == c.cfg.Node:
		return
	}
	c.Apply(ctx, s)
}

// Reconcile implements the route: ephemeral events lost while the LISTEN connection was down stay
// lost (at most once); the log tail delivers every durable event, and the next refresh of
// gateway_presence and the next save of a consumer restore the rest. So it does nothing.
func (*Cluster) Reconcile(context.Context) {}

// Apply applies the signal s of another node on this node (HandleEphemeral after decoding; tests
// call it directly).
func (c *Cluster) Apply(ctx context.Context, s domain.Signal) {
	var err error
	switch s.Kind {
	case domain.SignalTyping:
		err = c.applyTyping(ctx, s)
	case domain.SignalRun:
		err = c.applyRun(ctx, s)
	case domain.SignalAgentPresence:
		err = c.applyAgentPresence(ctx, s)
	case domain.SignalHint:
		err = c.applyHint(ctx, s)
	case domain.SignalDevice:
		if c.gw != nil {
			c.gw.DeviceTaken(s.DeviceID, s.SessionID, s.BindID)
		}
	case domain.SignalConsumer:
		if c.engine != nil {
			c.engine.Fence(ctx, s.AgentID, s.SessionID)
		}
	}
	if err != nil {
		c.d.Log.LogAttrs(ctx, slog.LevelWarn, msgSignalUnreadable, slog.String(logKeySignalKind, string(s.Kind)),
			slog.String(logKeyErrorType, errorTypeSignal))
	}
}

// applyTyping rebuilds typing.started for the connections of this node that focus the thread:
// the place from chat, occurred_at now and expires_at now plus sync.typing_ttl.
func (c *Cluster) applyTyping(ctx context.Context, s domain.Signal) error {
	if c.hub == nil || len(c.hub.focusing(s.ThreadID)) == 0 {
		return nil
	}
	info, err := c.d.Threads.Info(ctx, s.ThreadID)
	if err != nil {
		return err
	}
	now := c.d.Clock.Now().UTC().Truncate(time.Millisecond)
	return c.hub.PublishEphemeral(ctx, eventlog.EphemeralEvent{
		Type: eventTypingStarted, SchemaVersion: typingSchemaVersion,
		Actor:      &eventlog.ActorRef{Kind: eventlog.ActorUser, PrincipalID: s.PrincipalID},
		Scope:      eventlog.Scope{SpaceID: info.SpaceID, ChannelID: info.ChannelID, ThreadID: s.ThreadID},
		Resource:   eventlog.ResourceRef{Kind: resourceKindThread, ID: s.ThreadID},
		OccurredAt: now,
		Payload:    typingStartedBody{ThreadID: s.ThreadID, PrincipalID: s.PrincipalID, ExpiresAt: now.Add(c.cfg.TypingTTL).Format(timestampMillisUTC)},
	})
}

// applyRun hands a version step without ops to the coalescer when a connection of this node
// focuses the run's thread: the window end reads the run document and sends it as run.updated
// with snapshot (02 11.6, M2-10 design note 9).
func (c *Cluster) applyRun(ctx context.Context, s domain.Signal) error {
	if c.hub == nil || len(c.hub.focusing(s.ThreadID)) == 0 {
		return nil
	}
	lb := c.hub.liveBox()
	if lb == nil {
		return nil
	}
	info, err := c.d.Threads.Info(ctx, s.ThreadID)
	if err != nil {
		return err
	}
	lb.Changed(ctx, eventlog.EphemeralEvent{
		Type: eventRunUpdated, SchemaVersion: runUpdatedSchema,
		Scope:    eventlog.Scope{SpaceID: info.SpaceID, ChannelID: info.ChannelID, ThreadID: s.ThreadID},
		Resource: eventlog.ResourceRef{Kind: resourceKindRun, ID: s.RunID, Version: s.Version},
		Payload:  runs.RunUpdated{RunID: s.RunID, ThreadID: s.ThreadID, BaseVersion: s.Version - 1, Version: s.Version, Truncated: true},
	})
	return nil
}

// applyAgentPresence publishes agent.presence_changed on this node.
func (c *Cluster) applyAgentPresence(ctx context.Context, s domain.Signal) error {
	if c.hub == nil {
		return nil
	}
	ev := eventlog.EphemeralEvent{
		Type: eventAgentPresence, SchemaVersion: presenceSchema,
		Resource: eventlog.ResourceRef{Kind: resourceKindAgent, ID: s.AgentID},
		Payload:  agentPresenceBody{AgentID: s.AgentID.String(), State: s.State, ReportedBillingMode: s.BillingMode},
	}
	if !s.ActorID.IsZero() {
		ev.Actor = &eventlog.ActorRef{Kind: eventlog.ActorAgent, PrincipalID: s.ActorID}
	}
	return c.hub.PublishEphemeral(ctx, ev)
}

// applyHint publishes notification.created to the connections of the recipient on this node,
// when it holds any: the job notify.gateway_send checked the rights after its commit, and the hub
// admits each connection again (13 4.6 point 1, 6.3). A hint with a thread takes the thread's
// place now as its scope.
func (c *Cluster) applyHint(ctx context.Context, s domain.Signal) error {
	if c.hub == nil || len(c.hub.membersOf(s.RecipientID)) == 0 {
		return nil
	}
	ev := eventlog.EphemeralEvent{
		Type: eventHint, SchemaVersion: hintSchema, Audience: s.RecipientID,
		Payload: hintBody{NID: s.NotificationID, Kind: s.HintKind, ThreadID: s.ThreadID, MessageID: s.MessageID, TargetID: s.TargetID,
			Quiet: s.Quiet, Count: s.Count},
		Resource: eventlog.ResourceRef{Kind: resourceKindUser, ID: s.RecipientID},
	}
	if !s.ThreadID.IsZero() {
		info, err := c.d.Threads.Info(ctx, s.ThreadID)
		if err != nil {
			return err
		}
		ev.Resource = eventlog.ResourceRef{Kind: resourceKindThread, ID: s.ThreadID}
		ev.Scope = eventlog.Scope{SpaceID: info.SpaceID, ChannelID: info.ChannelID, ThreadID: s.ThreadID}
	}
	return c.hub.PublishEphemeral(ctx, ev)
}
