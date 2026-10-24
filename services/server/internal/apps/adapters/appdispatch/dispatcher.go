// SPDX-License-Identifier: Apache-2.0

package appdispatch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/OWNER/bothy/services/server/internal/apps/app"
	"github.com/OWNER/bothy/services/server/internal/apps/domain"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/platform/buildinfo"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// Purpose is the egress purpose of the interaction client (bothy.egress.purpose).
const Purpose = "app_interaction"

// JobKind is the job whose handler calls Post (runs.app_dispatch, M2-25 design note 8); its log
// lines carry it as bothy.job.kind. Ask runs in the person's request and carries none.
const JobKind = "runs.app_dispatch"

// The values of bothy.outcome of a request line (M2-25 design note 9 c).
const (
	outcomeAnswered    = "answered"
	outcomeDeferred    = "deferred"
	outcomeNotAnswered = "not_answered"
)

// logMessage is the message of the one line per request.
const logMessage = "app interaction request"

// The headers of Standard Webhooks (webhooks.md section 2).
const (
	headerID        = "webhook-id"
	headerTimestamp = "webhook-timestamp"
	headerSignature = "webhook-signature"
)

// timestampMillisUTC is occurred_at of an ephemeral envelope, as the gateway renders it.
const timestampMillisUTC = "2006-01-02T15:04:05.000Z07:00"

// Config are the configuration keys of the dispatcher; internal/wire binds them.
type Config struct {
	// InteractionTimeout is app.interaction_timeout: the time limit of one request.
	InteractionTimeout time.Duration `config:"app.interaction_timeout"`
	// MaxReplyBytes is http.max_json_body_bytes: a reply is a createMessage body, so it has the
	// same bound.
	MaxReplyBytes int64 `config:"http.max_json_body_bytes"`
	// AllowedHosts is webhook.allowed_hosts in the form host[:port] (D-M2-26-8).
	AllowedHosts []string `config:"webhook.allowed_hosts"`
	// MaxChoices is command.autocomplete_max_choices, the most suggestions of an answer.
	MaxChoices int `config:"command.autocomplete_max_choices"`
}

// MessageCreator writes a reply as a message (messaging.Messages.Create): authorize decides
// message.create for the actor, the message gets origin app, and client_msg_id makes a repeated
// job idempotent.
type MessageCreator interface {
	Create(ctx context.Context, a actor.Actor, cmd messaging.CreateMessageCmd) (messaging.Created, error)
}

// Deps are the collaborators of the dispatcher.
type Deps struct {
	// Installations reads the active installation and its manifest.
	Installations Installations
	// Log reads the stored event of a Post by id.
	Log sync.LogReader
	// Keys signs v1a with every key of purpose webhook_out.
	Keys app.Signers
	// State is the instance row with the sync epoch.
	State app.InstanceState
	// Messages writes the reply of a Post.
	Messages MessageCreator
	// Clock is the time of webhook-timestamp and of an ephemeral event without one; nil means
	// clock.System().
	Clock clock.Clock
	// Logger receives the line of each request; nil discards them.
	Logger *slog.Logger
}

// Dispatcher implements runs.AppDispatcher. It is safe for concurrent use.
type Dispatcher struct {
	cfg    Config
	d      Deps
	hosts  []domain.AllowedHost
	client *egress.Client
	ua     string
}

// Compile-time check: Dispatcher is the source port of runs.
var _ runs.AppDispatcher = (*Dispatcher)(nil)

// New builds the dispatcher and its one egress client: purpose app_interaction, the host policy of
// webhook.allowed_hosts (domain.AllowsHost, the rule of domain.ValidateTarget), RedirectNever,
// time limit app.interaction_timeout and body bound http.max_json_body_bytes. eg is the egress
// configuration of the server, egDeps its resolver, roots, logger and metrics (bothy.egress.blocked).
//
// Errors: a nil collaborator, a time limit, bound or choice count that is not positive, an entry of
// webhook.allowed_hosts that is no host[:port], or what egress.New refuses.
func New(cfg Config, eg egress.Config, d Deps, egDeps egress.Deps) (*Dispatcher, error) {
	if d.Installations == nil || d.Log == nil || d.Keys == nil || d.State == nil || d.Messages == nil {
		return nil, errors.New("appdispatch: a collaborator is missing")
	}
	if cfg.InteractionTimeout <= 0 || cfg.MaxReplyBytes <= 0 || cfg.MaxChoices <= 0 {
		return nil, errors.New("appdispatch: app.interaction_timeout, http.max_json_body_bytes and command.autocomplete_max_choices must be positive")
	}
	hosts, err := domain.ParseAllowedHosts(cfg.AllowedHosts)
	if err != nil {
		return nil, err
	}
	allows := domain.AllowsHost(hosts)
	client, err := egress.New(eg, egress.Options{
		Purpose: Purpose, Redirects: egress.RedirectNever, Timeout: cfg.InteractionTimeout, MaxResponseBytes: cfg.MaxReplyBytes,
		Hosts: func(_ context.Context, scheme, host string, port uint16) bool { return allows(scheme, host, port) },
	}, egDeps)
	if err != nil {
		return nil, err
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	if d.Logger == nil {
		d.Logger = slog.New(slog.DiscardHandler)
	}
	return &Dispatcher{cfg: cfg, d: d, hosts: hosts, client: client, ua: "chat-server-webhooks/" + buildinfo.Version()}, nil
}

// Way implements runs.AppDispatcher: inactive without an active installation, interactions_url
// when the manifest names delivery.interactions_url, gateway otherwise. The way follows the
// manifest, not a gateway connection, so every api node answers the same (OQ-07).
func (x *Dispatcher) Way(ctx context.Context, appID ids.ID) (runs.AppWay, error) {
	inst, found, err := x.d.Installations.ActiveInstallation(ctx, appID)
	switch {
	case err != nil:
		return "", err
	case !found:
		return runs.AppWayInactive, nil
	case interactionsURL(inst) != "":
		return runs.AppWayInteractionsURL, nil
	default:
		return runs.AppWayGateway, nil
	}
}

// interactionsURL is delivery.interactions_url of the manifest; "" when absent.
func interactionsURL(inst Installation) string {
	if inst.Manifest.Delivery == nil {
		return ""
	}
	return inst.Manifest.Delivery.InteractionsURL
}

// request is one prepared request to an interaction address.
type request struct {
	appID, webhookID ids.ID
	inst             Installation
	url, host        string
	eventType        string
	jobKind          string
	body             []byte
}

// Post implements runs.AppDispatcher (M2-25 design note 9 b): the installation and its address
// (inactive, no_url, blocked without a request), the stored event rendered as the webhook delivery
// of M2-26, one signed request, and for a 200 the reply written in the app's name into
// p.ReplyThreadID (no_thread without one, reply_refused when the create path refuses it).
//
// Errors: faults of the server only and the end of ctx; every answer of the app is an outcome. A
// create of the reply that failed with kind internal or unavailable happened after the request and
// wraps runs.ErrAfterRequest; the database, the keys and an event that is not in the log fail
// before the request.
func (x *Dispatcher) Post(ctx context.Context, p runs.AppPost) (runs.AppOutcome, error) {
	r := request{appID: p.AppID, webhookID: p.EventID, jobKind: JobKind}
	inst, out, err := x.target(ctx, &r)
	if err != nil || out != nil {
		return x.result(ctx, r, out, err)
	}
	ev, found, err := x.d.Log.EventByID(ctx, p.EventID)
	if err != nil {
		return runs.AppOutcome{}, err
	}
	if !found {
		return runs.AppOutcome{}, errs.New(errs.CodeInternalError, "apps: the event of an app dispatch is not in the log")
	}
	r.eventType = ev.Type
	epoch, err := x.epoch(ctx)
	if err != nil {
		return runs.AppOutcome{}, err
	}
	if r.body, err = json.Marshal(ev.Envelope(sync.DeliveryFields{SyncEpoch: epoch})); err != nil {
		return runs.AppOutcome{}, errs.Wrap(errs.CodeInternalError, err, "apps: envelope of an app dispatch")
	}
	status, body, sendErr, err := x.send(ctx, r)
	if err != nil {
		return runs.AppOutcome{}, err
	}
	o, err := x.postOutcome(ctx, inst, p, status, body, sendErr)
	return x.result(ctx, r, o, err)
}

// postOutcome turns the answer of a Post into its outcome; a 200 writes the reply.
func (x *Dispatcher) postOutcome(ctx context.Context, inst Installation, p runs.AppPost, status int, body []byte, sendErr error) (*runs.AppOutcome, error) {
	switch domain.OutcomeOf(status, sendErr) {
	case domain.ReplyDeferred:
		return &runs.AppOutcome{Deferred: true}, nil
	case domain.ReplyAnswered:
	default:
		return reason(domain.OutcomeOf(status, sendErr)), nil
	}
	reply, err := domain.DecodeAppReply(body)
	if err != nil {
		return reason(domain.ReasonInvalidReply), nil
	}
	if p.ReplyThreadID.IsZero() {
		return reason(domain.ReasonNoThread), nil
	}
	a := actor.Actor{ID: p.AppID, Kind: actor.KindApp, InstallationID: inst.ID, GrantedScopes: slices.Clone(inst.GrantedScopes)}
	if _, err := x.d.Messages.Create(ctx, a, createCmd(p.ReplyThreadID, reply)); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		if k := errs.KindOf(err); k == errs.KindInternal || k == errs.KindUnavailable {
			// The app got the request: the job must not send it again (runs.ErrAfterRequest).
			return nil, fmt.Errorf("%w: %w", runs.ErrAfterRequest, err)
		}
		return reason(domain.ReasonReplyRefused), nil
	}
	return &runs.AppOutcome{Answered: true}, nil
}

// Ask implements runs.AppDispatcher (M2-25 design note 9 d): the same checks as Post, the body the
// envelope of the ephemeral event with sync_epoch (no seq, no permission_version), a request that
// ends at a.Deadline, and for a 200 the suggestions of a valid AutocompleteAnswer. The ephemeral
// event has no stored id, so a fresh UUIDv7 is both its envelope id and webhook-id. No message is
// written.
//
// Errors: faults of the server (database, keys, a payload that is no JSON object or contains null)
// and the cancellation of ctx.
func (x *Dispatcher) Ask(ctx context.Context, a runs.AppAsk) (runs.AppOutcome, error) {
	r := request{appID: a.AppID, webhookID: ids.New(), eventType: a.Event.Type}
	if _, out, err := x.target(ctx, &r); err != nil || out != nil {
		return x.result(ctx, r, out, err)
	}
	epoch, err := x.epoch(ctx)
	if err != nil {
		return runs.AppOutcome{}, err
	}
	env, err := x.ephemeralEnvelope(r.webhookID, a.Event, epoch)
	if err != nil {
		return runs.AppOutcome{}, err
	}
	if r.body, err = json.Marshal(env); err != nil {
		return runs.AppOutcome{}, errs.Wrap(errs.CodeInternalError, err, "apps: envelope of an autocomplete request")
	}
	actx, cancel := context.WithDeadline(ctx, a.Deadline)
	defer cancel()
	status, body, sendErr, err := x.send(actx, r)
	if err != nil {
		return runs.AppOutcome{}, err
	}
	var out *runs.AppOutcome
	switch o := domain.OutcomeOf(status, sendErr); o {
	case domain.ReplyAnswered:
		choices, err := domain.DecodeAutocompleteReply(body, x.cfg.MaxChoices)
		if err != nil {
			out = reason(domain.ReasonInvalidReply)
			break
		}
		out = &runs.AppOutcome{Answered: true, Choices: make([]runs.Choice, 0, len(choices))}
		for _, c := range choices {
			out.Choices = append(out.Choices, runs.Choice{Name: c.Name, Value: c.Value})
		}
	case domain.ReplyDeferred:
		out = &runs.AppOutcome{Deferred: true}
	default:
		out = reason(o)
	}
	return x.result(ctx, r, out, nil)
}

// target loads the active installation of r.appID and checks its address against
// webhook.allowed_hosts. It sets r.inst, r.url and r.host and returns a non-nil outcome when no
// request may go out: inactive, no_url or blocked.
func (x *Dispatcher) target(ctx context.Context, r *request) (Installation, *runs.AppOutcome, error) {
	inst, found, err := x.d.Installations.ActiveInstallation(ctx, r.appID)
	if err != nil {
		return Installation{}, nil, err
	}
	if !found {
		return Installation{}, reason(domain.ReasonInactive), nil
	}
	r.inst, r.url = inst, interactionsURL(inst)
	if r.url == "" {
		return inst, reason(domain.ReasonNoURL), nil
	}
	r.host = domain.HostOf(r.url)
	// The check before the egress client: an egress.allowed_hosts entry would admit a host that
	// left webhook.allowed_hosts (M2-25 design note 9 b 1, SEC-NET-2).
	if _, err := domain.ValidateTarget(r.url, x.hosts); err != nil {
		return inst, reason(domain.OutcomeOf(0, err)), nil
	}
	return inst, nil, nil
}

// epoch reads the sync epoch of the instance row.
func (x *Dispatcher) epoch(ctx context.Context) (ids.ID, error) {
	rec, err := x.d.State.Load(ctx)
	if err != nil {
		return ids.ID{}, err
	}
	return rec.SyncEpoch, nil
}

// send signs and sends r once and returns the status and, for a 200, the body read up to
// http.max_json_body_bytes. sendErr is the transport error of the request or of reading the body,
// converted to the terms of domain (a refusal to *domain.BlockedError, the size limit to
// domain.ErrReplyTooLarge). err is a fault of the server: a key that cannot sign, or the
// cancellation of the caller's context (shutdown), which is no answer of the app.
func (x *Dispatcher) send(ctx context.Context, r request) (status int, body []byte, sendErr, err error) {
	ts := x.d.Clock.Now().Unix()
	signers, err := x.d.Keys.Signers(ctx, instance.PurposeWebhookOut)
	if err != nil {
		return 0, nil, nil, err
	}
	content := domain.SignedContent(r.webhookID, ts, r.body)
	v1a := make([][]byte, 0, len(signers))
	for _, s := range signers {
		sig, err := s.Sign(ctx, content)
		if err != nil {
			return 0, nil, nil, err
		}
		v1a = append(v1a, sig)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.url, bytes.NewReader(r.body))
	if err != nil {
		// The address passed ValidateTarget; a request that cannot be built is refused like an
		// address the egress client does not accept.
		return 0, nil, &domain.BlockedError{Reason: string(egress.ReasonURLNotAllowed)}, nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", x.ua)
	req.Header.Set(headerID, r.webhookID.String())
	req.Header.Set(headerTimestamp, strconv.FormatInt(ts, 10))
	req.Header.Set(headerSignature, domain.SignatureHeader(v1a, nil))
	resp, doErr := x.client.Do(req)
	if doErr != nil {
		sendErr, err := x.transportError(ctx, doErr)
		return 0, nil, sendErr, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		// Only a 200 has a body that is used (11 section 9); every other body is never read.
		return resp.StatusCode, nil, nil, nil
	}
	b, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		sendErr, err := x.transportError(ctx, readErr)
		return resp.StatusCode, nil, sendErr, err
	}
	return resp.StatusCode, b, nil, nil
}

// transportError converts an error of the egress client or of reading a body into the terms of
// domain; a cancellation of the caller's context (not a deadline) is returned as a fault.
func (x *Dispatcher) transportError(ctx context.Context, err error) (sendErr, fault error) {
	if errors.Is(ctx.Err(), context.Canceled) {
		return nil, ctx.Err()
	}
	var blocked *egress.BlockedError
	switch {
	case errors.As(err, &blocked):
		return &domain.BlockedError{Reason: string(blocked.Reason)}, nil
	case errors.Is(err, egress.ErrResponseTooLarge):
		return domain.ErrReplyTooLarge, nil
	}
	return err, nil
}

// ephemeralEnvelope renders ev like the gateway renders an ephemeral event (no seq, no
// permission_version, occurred_at in milliseconds UTC) with id and the delivery member sync_epoch
// of a webhook delivery.
func (x *Dispatcher) ephemeralEnvelope(id ids.ID, ev eventlog.EphemeralEvent, epoch ids.ID) (sync.Envelope, error) {
	payload, err := json.Marshal(ev.Payload)
	if err != nil || len(payload) == 0 || payload[0] != '{' || containsNull(payload) {
		return sync.Envelope{}, errs.New(errs.CodeInternalError, "apps: the payload of an autocomplete request is no JSON object without null")
	}
	occurred := ev.OccurredAt
	if occurred.IsZero() {
		occurred = x.d.Clock.Now()
	}
	env := sync.Envelope{
		ID: id, Type: ev.Type, SchemaVersion: ev.SchemaVersion, OccurredAt: occurred.UTC().Format(timestampMillisUTC),
		Scope:    sync.EnvelopeScope{SpaceID: ev.Scope.SpaceID, ChannelID: ev.Scope.ChannelID, ThreadID: ev.Scope.ThreadID},
		Resource: sync.EnvelopeResource{Kind: ev.Resource.Kind, ID: ev.Resource.ID},
		Payload:  payload, SyncEpoch: epoch,
	}
	if ev.Actor != nil {
		env.Actor = &sync.EnvelopeActor{ID: ev.Actor.PrincipalID, Kind: string(ev.Actor.Kind)}
	}
	if !ev.Audience.IsZero() {
		env.Audience = "self"
	}
	return env, nil
}

// containsNull reports whether the valid JSON value b contains a null literal outside strings.
func containsNull(b []byte) bool {
	dec := json.NewDecoder(bytes.NewReader(b))
	for {
		tok, err := dec.Token()
		if err != nil {
			return false
		}
		if tok == nil {
			return true
		}
	}
}

// result writes the line of a request and returns the outcome; an error passes unchanged and
// writes no line.
func (x *Dispatcher) result(ctx context.Context, r request, out *runs.AppOutcome, err error) (runs.AppOutcome, error) {
	if err != nil {
		return runs.AppOutcome{}, err
	}
	level, outcome := slog.LevelWarn, outcomeNotAnswered
	switch {
	case out.Answered:
		level, outcome = slog.LevelInfo, outcomeAnswered
	case out.Deferred:
		level, outcome = slog.LevelInfo, outcomeDeferred
	}
	attrs := make([]slog.Attr, 0, 6)
	if r.jobKind != "" {
		attrs = append(attrs, slog.String(attr.JobKind, r.jobKind))
	}
	if r.eventType != "" {
		attrs = append(attrs, slog.String(attr.EventType, r.eventType))
	}
	attrs = append(attrs, slog.String(attr.PrincipalKind, string(actor.KindApp)), slog.String(attr.Outcome, outcome))
	if out.Reason != "" {
		attrs = append(attrs, slog.String(attr.Reason, out.Reason))
	}
	if r.host != "" {
		attrs = append(attrs, slog.String(attr.ServerAddress, r.host))
	}
	x.d.Logger.LogAttrs(ctx, level, logMessage, attrs...)
	return *out, nil
}

// reason is the outcome with only code as its reason.
func reason(code string) *runs.AppOutcome { return &runs.AppOutcome{Reason: code} }

// createCmd builds the create command of a reply exactly as the REST handler of createMessage
// builds it from a MessageCreate body.
func createCmd(thread ids.ID, r domain.Reply) messaging.CreateMessageCmd {
	cmd := messaging.CreateMessageCmd{
		ThreadID: thread, ClientMsgID: r.ClientMsgID, Kind: r.Kind, ContentMD: r.ContentMD, ReplyToID: r.ReplyToMessageID,
		AttachmentIDs: r.AttachmentFileIDs, RunID: r.RunID, MentionIDs: r.MentionIDs, Silent: r.Silent, ClearDraft: r.ClearDraft,
	}
	if r.Components != nil {
		cmd.Components = make([]messaging.ComponentInput, 0, len(r.Components))
		for _, c := range r.Components {
			cmd.Components = append(cmd.Components, messaging.ComponentInput{
				CustomID: c.CustomID, Type: c.Type, Definition: c.Definition, SingleUse: c.SingleUse,
				RequiredCapability: c.RequiredCapability, RestrictedTo: c.RestrictedTo, ExpiresAt: c.ExpiresAt,
			})
		}
	}
	return cmd
}
