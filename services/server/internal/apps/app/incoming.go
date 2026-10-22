// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"context"
	"crypto/hmac"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/apps"
	"github.com/OWNER/bothy/services/server/internal/apps/domain"
	"github.com/OWNER/bothy/services/server/internal/apps/domain/incoming"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/messaging"
)

// IncomingConfig are the configuration keys of the incoming webhooks; internal/wire binds them.
type IncomingConfig struct {
	// MaxContentBytes is message.max_content_bytes: the native content and the rendered
	// Alertmanager message stay within it.
	MaxContentBytes int `config:"message.max_content_bytes"`
	// FormatsEnabled is webhook_in.formats_enabled.
	FormatsEnabled []string `config:"webhook_in.formats_enabled"`
	// LastUsedGranularity is identity.last_active_granularity: last_used_at moves at most once
	// per window (D-M2-27-9).
	LastUsedGranularity time.Duration `config:"identity.last_active_granularity"`
	// PublicURL is server.public_url, the base of the URL createIncomingWebhook returns.
	PublicURL string `config:"server.public_url"`
}

// IncomingRecord is a row of incoming_webhooks; zero values stand for NULL.
type IncomingRecord struct {
	ID, AppID, ChannelID ids.ID
	ThreadID             ids.ID
	Name                 string
	TokenHash            []byte
	CreatedBy            ids.ID
	CreatedAt            time.Time
	LastUsedAt           time.Time
	RevokedAt            time.Time
}

// IncomingAuth is a webhook with the installation of its app that is not uninstalled (the token
// check, design note 2 b).
type IncomingAuth struct {
	Webhook           IncomingRecord
	InstallationID    ids.ID
	InstallationState domain.State
	GrantedScopes     []string
}

// IncomingStore is the outgoing port of the incoming webhooks over incoming_webhooks
// (adapters/postgres). Reads run in the ambient transaction of ctx or on the pool; writes and
// locks only inside a unit of work. Errors are kernel/errs errors without SQL.
type IncomingStore interface {
	// ListActiveIncoming returns the webhooks of channelID without revoked_at, ordered by
	// created_at, then id.
	ListActiveIncoming(ctx context.Context, channelID ids.ID) ([]IncomingRecord, error)
	// InsertIncoming inserts r and returns it with created_at of the database.
	InsertIncoming(ctx context.Context, r IncomingRecord) (IncomingRecord, error)
	// Incoming returns the webhook id; found is false for an unknown id.
	Incoming(ctx context.Context, id ids.ID) (IncomingRecord, bool, error)
	// IncomingForAuth returns the webhook id with the installation of its app that is not
	// uninstalled; found is false for an unknown id and for an app without such an installation.
	IncomingForAuth(ctx context.Context, id ids.ID) (IncomingAuth, bool, error)
	// LockIncomingForPost returns the webhook id FOR SHARE.
	LockIncomingForPost(ctx context.Context, id ids.ID) (IncomingRecord, bool, error)
	// RevokeIncoming sets revoked_at of an active webhook; changed is false when it was revoked.
	RevokeIncoming(ctx context.Context, id ids.ID, at time.Time) (changed bool, err error)
	// RevokeIncomingByApp revokes every active webhook of appID and returns their number.
	RevokeIncomingByApp(ctx context.Context, appID ids.ID, at time.Time) (int, error)
	// RevokeIncomingByChannel revokes every active webhook of channelID and returns their number.
	RevokeIncomingByChannel(ctx context.Context, channelID ids.ID, at time.Time) (int, error)
	// TouchIncomingLastUsed sets last_used_at to at when it is NULL or before olderThan.
	TouchIncomingLastUsed(ctx context.Context, id ids.ID, at, olderThan time.Time) error
}

// IncomingTokens is the consumer port of identity for the webhook tokens (identity.Tokens).
type IncomingTokens interface {
	NewIncomingWebhookToken(ctx context.Context) (identity.IncomingWebhookToken, error)
	HashIncomingWebhookToken(ctx context.Context, raw string) ([]byte, error)
}

// IncomingThreads is the consumer port of chat for the threads of a post (chat.Threads.Info).
type IncomingThreads interface {
	Info(ctx context.Context, threadID ids.ID) (chat.ThreadInfo, error)
}

// IncomingMessages is the consumer port of messaging for the message of a post
// (messaging.Messages).
type IncomingMessages interface {
	Create(ctx context.Context, a actor.Actor, cmd messaging.CreateMessageCmd) (messaging.Created, error)
	CreateThread(ctx context.Context, a actor.Actor, cmd messaging.CreateThreadCmd) (messaging.ThreadCreated, error)
}

// IncomingDeps are the collaborators of the incoming webhooks.
type IncomingDeps struct {
	Store      IncomingStore
	Repo       Repository
	UoW        txn.UnitOfWork
	Authorizer authorize.Authorizer
	Audit      auditing.Recorder
	Tokens     IncomingTokens
	// Channels are the channel facts of chat (kind, space, main thread), read without
	// authorizing; the use cases authorize before they answer anything derived from them.
	Channels chat.TriggerFacts
	Threads  IncomingThreads
	ByParent chat.ThreadsByParent
	Messages IncomingMessages
	// Clock is the time of the writes; nil means clock.System().
	Clock clock.Clock
	// NewID returns a new UUIDv7 (the server client_msg_id); nil means ids.New.
	NewID func() ids.ID
	// Logger receives the failure line of last_used_at; nil discards it.
	Logger *slog.Logger
}

// IncomingService implements apps.IncomingWebhooks and the revocation of an uninstall. It is safe
// for concurrent use.
type IncomingService struct {
	cfg IncomingConfig
	d   IncomingDeps
}

// Compile-time check: IncomingService is the incoming port.
var _ apps.IncomingWebhooks = (*IncomingService)(nil)

// NewIncoming returns the use cases of the incoming webhooks. Errors: a missing collaborator, a
// limit below 1, a granularity that is not positive, a public URL that is empty, or a format of
// webhook_in.formats_enabled that is unknown.
func NewIncoming(cfg IncomingConfig, d IncomingDeps) (*IncomingService, error) {
	if d.Store == nil || d.Repo == nil || d.UoW == nil || d.Authorizer == nil || d.Audit == nil || d.Tokens == nil ||
		d.Channels == nil || d.Threads == nil || d.ByParent == nil || d.Messages == nil {
		return nil, errors.New("apps: NewIncoming needs every collaborator")
	}
	if cfg.MaxContentBytes < 1 || cfg.LastUsedGranularity <= 0 || cfg.PublicURL == "" {
		return nil, errors.New("apps: message.max_content_bytes, identity.last_active_granularity and server.public_url must be set")
	}
	for _, f := range cfg.FormatsEnabled {
		if f != string(incoming.FormatNative) && f != string(incoming.FormatAlertmanager) {
			return nil, errors.New("apps: webhook_in.formats_enabled names an unknown format")
		}
	}
	cfg.FormatsEnabled = slices.Clone(cfg.FormatsEnabled)
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	if d.NewID == nil {
		d.NewID = ids.New
	}
	if d.Logger == nil {
		d.Logger = slog.New(slog.DiscardHandler)
	}
	return &IncomingService{cfg: cfg, d: d}, nil
}

// now is the time of the clock at the precision of PostgreSQL.
func (s *IncomingService) now() time.Time { return s.d.Clock.Now().Truncate(time.Microsecond) }

// The audit attributes of the incoming webhook actions (SEC-AUD-5): ids only, never the name,
// the token or its hash.
const (
	attrWebhookID = "webhook_id"
	attrThreadID  = "thread_id"
)

// The errors[].code values of createIncomingWebhook (D-M2-27-11).
const (
	codeAppNotInstalled = "app_not_installed"
	codeAppCannotWrite  = "app_cannot_write"
	codeNotInChannel    = "not_in_channel"
)

// hooksPath is the path of postIncomingWebhook below server.public_url.
const hooksPath = "/api/v1/hooks/"

// authorizeOn authorizes act on r for a and returns the decision as an error (nil for allow).
func (s *IncomingService) authorizeOn(ctx context.Context, a actor.Actor, act authorize.Action, r authorize.Resource) error {
	d, err := s.d.Authorizer.Authorize(ctx, a, act, r)
	if err != nil {
		return err
	}
	return d.Err()
}

// channel returns the facts of channelID and its authorize resource; an unknown channel is the
// 404 of an invisible one.
func (s *IncomingService) channel(ctx context.Context, channelID ids.ID) (chat.TriggerChannel, authorize.Resource, error) {
	ch, err := s.d.Channels.TriggerChannel(ctx, channelID)
	if err != nil {
		return chat.TriggerChannel{}, authorize.Resource{}, err
	}
	return ch, authorize.Resource{Kind: authorize.ResourceChannel, ID: ch.ChannelID, SpaceID: ch.SpaceID}, nil
}

// incomingView is the webhook as the API shows it.
func incomingView(r IncomingRecord) apps.IncomingWebhook {
	return apps.IncomingWebhook{
		ID: r.ID, AppID: r.AppID, ChannelID: r.ChannelID, ThreadID: r.ThreadID, Name: r.Name, CreatedAt: r.CreatedAt,
		LastUsedAt: r.LastUsedAt, RevokedAt: r.RevokedAt,
	}
}

// ListIncoming implements apps.IncomingWebhooks (design note 9): webhook_in.read on the channel
// (404 without read, 403 forbidden without manage), then the active webhooks; revoked ones are not
// listed (D-M2-27-12).
func (s *IncomingService) ListIncoming(ctx context.Context, a actor.Actor, channelID ids.ID) ([]apps.IncomingWebhook, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return nil, err
	}
	_, res, err := s.channel(ctx, channelID)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeOn(ctx, a, authorize.ActionWebhookInRead, res); err != nil {
		return nil, err
	}
	rows, err := s.d.Store.ListActiveIncoming(ctx, channelID)
	if err != nil {
		return nil, err
	}
	out := make([]apps.IncomingWebhook, 0, len(rows))
	for _, r := range rows {
		out = append(out, incomingView(r))
	}
	return out, nil
}

// CreateIncoming implements apps.IncomingWebhooks (design note 8): webhook_in.create on the
// channel (step-up), the body rules, then one unit of work that mints the token, inserts the row
// with only its HMAC and writes the allow entry. The URL with the token is returned once.
func (s *IncomingService) CreateIncoming(ctx context.Context, a actor.Actor, cmd apps.CreateIncomingCmd) (apps.IncomingCreated, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return apps.IncomingCreated{}, err
	}
	ch, res, err := s.channel(ctx, cmd.ChannelID)
	if err != nil {
		return apps.IncomingCreated{}, err
	}
	if err := s.authorizeOn(ctx, a, authorize.ActionWebhookInCreate, res); err != nil {
		return apps.IncomingCreated{}, err
	}
	if err := s.checkCreate(ctx, cmd, ch); err != nil {
		return apps.IncomingCreated{}, err
	}
	var out apps.IncomingCreated
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		tok, err := s.d.Tokens.NewIncomingWebhookToken(ctx)
		if err != nil {
			return err
		}
		r := IncomingRecord{ID: ids.New(), AppID: cmd.AppID, ChannelID: ch.ChannelID, Name: cmd.Name, TokenHash: tok.Hash, CreatedBy: a.ID}
		if cmd.ThreadID != nil {
			r.ThreadID = *cmd.ThreadID
		}
		if r, err = s.d.Store.InsertIncoming(ctx, r); err != nil {
			return err
		}
		attrs := auditing.Attrs{attrWebhookID: auditing.ID(r.ID), attrAppID: auditing.ID(r.AppID)}
		if !r.ThreadID.IsZero() {
			attrs[attrThreadID] = auditing.ID(r.ThreadID)
		}
		if err := s.d.Audit.Record(ctx, auditing.Entry{
			Action: string(authorize.ActionWebhookInCreate), Decision: auditing.DecisionAllow, ResourceKind: string(authorize.ResourceChannel),
			ResourceID: ch.ChannelID, Attrs: attrs,
		}); err != nil {
			return err
		}
		out = apps.IncomingCreated{Webhook: incomingView(r), URL: strings.TrimRight(s.cfg.PublicURL, "/") + hooksPath + r.ID.String() + "/" + tok.Secret.Reveal()}
		return nil
	})
	if err != nil {
		return apps.IncomingCreated{}, err
	}
	return out, nil
}

// checkCreate holds the body rules of design note 8 b in their order: the name, the app
// (checkApp) and the fixed thread, which must be a thread of the channel (not_in_channel, the same
// for an unknown thread).
func (s *IncomingService) checkCreate(ctx context.Context, cmd apps.CreateIncomingCmd, ch chat.TriggerChannel) error {
	name := strings.TrimSpace(cmd.Name)
	switch {
	case name == "":
		return fieldErr("/name", domain.CodeRequired, "A name is required.")
	case textcheck.DisplayText(cmd.Name) != nil:
		return fieldErr("/name", domain.CodeInvalid, "The name contains a forbidden character (SEC-IN-4).")
	}
	if err := s.checkApp(ctx, cmd.AppID, ch); err != nil {
		return err
	}
	if cmd.ThreadID == nil {
		return nil
	}
	info, err := s.d.Threads.Info(ctx, *cmd.ThreadID)
	if errs.KindOf(err) == errs.KindNotFound || err == nil && info.ChannelID != ch.ChannelID {
		return fieldErr("/thread_id", codeNotInChannel, "The thread is not a thread of this channel.")
	}
	return err
}

// checkApp holds the two app rules of design note 8 b: an app principal with an active
// installation (app_not_installed) that may write in the channel (app_cannot_write, checked in
// filter mode with the webhook actor, so no denial is audited).
func (s *IncomingService) checkApp(ctx context.Context, appID ids.ID, ch chat.TriggerChannel) error {
	inst, found, err := s.d.Repo.ActiveInstallation(ctx, appID)
	if err != nil {
		return err
	}
	if !found {
		return fieldErr("/app_id", codeAppNotInstalled, "The app has no active installation.")
	}
	wa := WebhookActor(appID, inst.ID, inst.GrantedScopes)
	d, err := s.d.Authorizer.Authorize(authorize.Filter(ctx), wa, authorize.ActionMessageCreate, newThreadIn(ch))
	if err != nil {
		return err
	}
	if d.Err() != nil {
		return fieldErr("/app_id", codeAppCannotWrite, "The app may not write in this channel.")
	}
	return nil
}

// newThreadIn is the resource of message.create in a channel without naming a thread (the path
// of a thread created in the same unit, messaging brief D8).
func newThreadIn(ch chat.TriggerChannel) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceThread, ChannelID: ch.ChannelID, SpaceID: ch.SpaceID}
}

// RevokeIncoming implements apps.IncomingWebhooks (design note 10): an unknown webhook is 404;
// webhook_in.revoke on the webhook (step-up); an already revoked one is without effect and audited
// with no_change; otherwise revoked_at is set and the allow entry written in one unit of work.
func (s *IncomingService) RevokeIncoming(ctx context.Context, a actor.Actor, webhookID ids.ID) error {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return err
	}
	r, found, err := s.d.Store.Incoming(ctx, webhookID)
	if err != nil {
		return err
	}
	if !found {
		return notFound()
	}
	res := authorize.Resource{Kind: authorize.ResourceIncomingWebhook, ID: r.ID, ChannelID: r.ChannelID}
	if err := s.authorizeOn(ctx, a, authorize.ActionWebhookInRevoke, res); err != nil {
		return err
	}
	return s.d.UoW.Do(ctx, func(ctx context.Context) error {
		changed, err := s.d.Store.RevokeIncoming(ctx, r.ID, s.now())
		if err != nil {
			return err
		}
		attrs := auditing.Attrs{attrWebhookID: auditing.ID(r.ID), attrAppID: auditing.ID(r.AppID)}
		if !changed {
			attrs = noChange(attrs)
		}
		return s.d.Audit.Record(ctx, auditing.Entry{
			Action: string(authorize.ActionWebhookInRevoke), Decision: auditing.DecisionAllow, ResourceKind: string(authorize.ResourceIncomingWebhook),
			ResourceID: r.ID, Attrs: attrs,
		})
	})
}

// scopeMessagesWrite is the one scope a webhook actor keeps (D-M2-27-3).
const scopeMessagesWrite = "messages:write"

// WebhookActor returns the actor of a verified post (D-M2-27-3): the app principal with its active
// installation and the granted scopes of the installation intersected with messages:write, without
// token, session or device. A stolen URL can so only post (T-TK-03).
func WebhookActor(appID, installationID ids.ID, granted []string) actor.Actor {
	scopes := []string{}
	if slices.Contains(granted, scopeMessagesWrite) {
		scopes = append(scopes, scopeMessagesWrite)
	}
	return actor.Actor{ID: appID, Kind: actor.KindApp, InstallationID: installationID, GrantedScopes: scopes}
}

// AuthenticateIncoming implements apps.IncomingWebhooks (design note 2 b): a token that is not of
// kind whi fails without a database call; otherwise one HMAC and one lookup by the primary key,
// compared with hmac.Equal. Every failure is the same 404.
func (s *IncomingService) AuthenticateIncoming(ctx context.Context, webhookID ids.ID, token string) (actor.Actor, error) {
	hash, err := s.d.Tokens.HashIncomingWebhookToken(ctx, token)
	if errors.Is(err, identity.ErrTokenMalformed) {
		return actor.Actor{}, notFound()
	}
	if err != nil {
		return actor.Actor{}, err
	}
	auth, found, err := s.d.Store.IncomingForAuth(ctx, webhookID)
	if err != nil {
		return actor.Actor{}, err
	}
	if !found || !hmac.Equal(auth.Webhook.TokenHash, hash) || !auth.Webhook.RevokedAt.IsZero() || auth.InstallationState != domain.StateActive {
		return actor.Actor{}, notFound()
	}
	return WebhookActor(auth.Webhook.AppID, auth.InstallationID, auth.GrantedScopes), nil
}

// post is one checked request: the message content, its client message id and the facts of the
// target thread rules.
type post struct {
	format      incoming.Format
	content     string
	clientMsgID ids.ID
	bodyThread  *ids.ID
	title       *string
}

// PostIncoming implements apps.IncomingWebhooks (design notes 4 to 6): the format and its body
// rules, then one unit of work that locks the webhook row FOR SHARE, chooses the target thread,
// authorizes webhook_in.post on the channel and writes the message through messaging with
// SkipMentions and the hash context of the webhook. Every refusal of the app's rights is the 404 of
// an unknown webhook. After the commit last_used_at moves at most once per
// identity.last_active_granularity.
func (s *IncomingService) PostIncoming(ctx context.Context, a actor.Actor, cmd apps.PostIncomingCmd) (apps.IncomingPosted, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return apps.IncomingPosted{}, err
	}
	p, err := s.parse(cmd.Body)
	if err != nil {
		return apps.IncomingPosted{}, err
	}
	var out apps.IncomingPosted
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.write(ctx, a, cmd.WebhookID, p)
		return err
	})
	if err != nil {
		return apps.IncomingPosted{}, asNotFound(err)
	}
	now := s.now()
	if err := s.d.Store.TouchIncomingLastUsed(ctx, cmd.WebhookID, now, now.Add(-s.cfg.LastUsedGranularity)); err != nil {
		s.d.Logger.WarnContext(ctx, "incoming webhook last_used_at not written")
	}
	return out, nil
}

// parse classifies the body and applies the rules of its format (design note 4).
func (s *IncomingService) parse(body []byte) (post, error) {
	var members map[string]json.RawMessage
	if err := json.Unmarshal(body, &members); err != nil {
		return post{}, errs.New(errs.CodeMalformedRequest, "")
	}
	var present []string
	for _, m := range append(incoming.NativeMembers(), incoming.AlertmanagerMembers()...) {
		if _, ok := members[m]; ok {
			present = append(present, m)
		}
	}
	f, err := incoming.Classify(present)
	if err != nil {
		return post{}, err
	}
	if err := incoming.CheckEnabled(f, s.cfg.FormatsEnabled); err != nil {
		return post{}, err
	}
	if f == incoming.FormatAlertmanager {
		var pl incoming.Payload
		if err := decodeStrict(body, &pl); err != nil {
			return post{}, err
		}
		n, err := incoming.ParseAlertmanager(pl)
		if err != nil {
			return post{}, err
		}
		return post{format: f, content: incoming.RenderAlertmanager(n, s.cfg.MaxContentBytes), clientMsgID: s.d.NewID()}, nil
	}
	var nb struct {
		ContentMD   *string  `json:"content_md"`
		Title       *string  `json:"title"`
		ThreadID    *anyUUID `json:"thread_id"`
		ClientMsgID *anyUUID `json:"client_msg_id"`
	}
	if err := decodeStrict(body, &nb); err != nil {
		return post{}, err
	}
	n, err := incoming.CheckNative(incoming.Native{ContentMD: nb.ContentMD, Title: nb.Title, ThreadID: nb.ThreadID.id(), ClientMsgID: nb.ClientMsgID.id()},
		s.cfg.MaxContentBytes, s.d.NewID)
	if err != nil {
		return post{}, err
	}
	return post{format: f, content: n.ContentMD, clientMsgID: n.ClientMsgID, bodyThread: n.ThreadID, title: n.Title}, nil
}

// anyUUID is a UUID of any version in the canonical form, as the schema Uuid admits it: the body
// rules, not the decoder, refuse a client_msg_id that is no UUIDv7 (not_uuid_v7), and a thread id
// of another version names no thread.
type anyUUID [16]byte

// UnmarshalJSON reads the canonical form 8-4-4-4-12 in any case.
func (u *anyUUID) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	h := strings.ReplaceAll(s, "-", "")
	if len(s) != 36 || len(h) != 32 || s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return errors.New("apps: not a UUID")
	}
	raw, err := hex.DecodeString(h)
	if err != nil {
		return errors.New("apps: not a UUID")
	}
	copy(u[:], raw)
	return nil
}

// id returns the UUID as an ids.ID, nil when absent.
func (u *anyUUID) id() *ids.ID {
	if u == nil {
		return nil
	}
	id := ids.ID(*u)
	return &id
}

// decodeStrict decodes body into v; the HTTP chain already checked it against the closed schema,
// so a failure here is a malformed request.
func decodeStrict(body []byte, v any) error {
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(v); err != nil {
		return errs.New(errs.CodeMalformedRequest, "")
	}
	return nil
}

// write is the unit of work of a post.
func (s *IncomingService) write(ctx context.Context, a actor.Actor, webhookID ids.ID, p post) (apps.IncomingPosted, error) {
	r, found, err := s.d.Store.LockIncomingForPost(ctx, webhookID)
	if err != nil {
		return apps.IncomingPosted{}, err
	}
	if !found || !r.RevokedAt.IsZero() || r.AppID != a.ID {
		return apps.IncomingPosted{}, notFound()
	}
	ch, res, err := s.channel(ctx, r.ChannelID)
	if err != nil {
		return apps.IncomingPosted{}, err
	}
	facts := incoming.TargetFacts{Format: p.format, ChannelID: ch.ChannelID, ChannelKind: ch.Kind, MainThreadID: ch.MainThreadID, Title: p.title}
	if !r.ThreadID.IsZero() {
		if facts.Fixed, err = s.threadFact(ctx, r.ThreadID); err != nil {
			return apps.IncomingPosted{}, err
		}
	}
	if p.bodyThread != nil {
		if facts.Body, err = s.threadFact(ctx, *p.bodyThread); err != nil {
			return apps.IncomingPosted{}, err
		}
	}
	target, err := incoming.ChooseTarget(facts)
	if err != nil {
		return apps.IncomingPosted{}, err
	}
	if err := s.authorizeOn(ctx, a, authorize.ActionWebhookInPost, res); err != nil {
		return apps.IncomingPosted{}, err
	}
	content := p.content
	msg := messaging.CreateMessageCmd{
		ClientMsgID: p.clientMsgID, ContentMD: &content, SkipMentions: true,
		HashContext: incoming.HashContext(webhookID, p.bodyThread, p.title),
	}
	out, err := s.send(ctx, a, ch, target, msg)
	if err != nil {
		return apps.IncomingPosted{}, err
	}
	if out.channelID != r.ChannelID {
		// A thread moved between the choice and the write (design note 6 d): the unit rolls back.
		return apps.IncomingPosted{}, notFound()
	}
	return apps.IncomingPosted{MessageID: out.messageID, ThreadID: out.threadID, Replay: out.replay}, nil
}

// sent is the outcome of the messaging calls of a post.
type sent struct {
	messageID, threadID, channelID ids.ID
	replay                         bool
}

// send writes the message into the target (design note 6 b): createMessage for an existing
// thread, createThread with the message as first message for a forum post, and createMessage into
// the main thread followed by createThread from it for a title in a text channel. A replay creates
// no thread and answers the thread of the first answer.
func (s *IncomingService) send(ctx context.Context, a actor.Actor, ch chat.TriggerChannel, t incoming.Target, msg messaging.CreateMessageCmd) (sent, error) {
	switch t.Kind {
	case incoming.TargetForumPost:
		title := t.Title
		tc, err := s.d.Messages.CreateThread(ctx, a, messaging.CreateThreadCmd{ChannelID: ch.ChannelID, Title: &title, FirstMessage: &msg})
		if err != nil {
			return sent{}, err
		}
		if tc.FirstMessage == nil {
			return sent{}, errs.New(errs.CodeInternalError, "apps: a forum post without its first message")
		}
		return sent{messageID: tc.FirstMessage.ID, threadID: tc.ThreadID, channelID: tc.FirstMessage.ChannelID, replay: tc.Replay}, nil
	case incoming.TargetTextThread:
		return s.sendTitledText(ctx, a, ch, t, msg)
	case incoming.TargetExisting:
	}
	msg.ThreadID = t.ThreadID
	c, err := s.d.Messages.Create(ctx, a, msg)
	if err != nil {
		return sent{}, err
	}
	return sent{messageID: c.Message.ID, threadID: c.Message.ThreadID, channelID: c.Message.ChannelID, replay: c.Replay}, nil
}

// sendTitledText writes a titled post into a text channel (design note 5 rule 3): the message
// goes into the main thread and a new thread starts from it. A replay creates no thread and
// answers the thread whose parent is the original message.
func (s *IncomingService) sendTitledText(ctx context.Context, a actor.Actor, ch chat.TriggerChannel, t incoming.Target, msg messaging.CreateMessageCmd) (sent, error) {
	msg.ThreadID = t.ThreadID
	c, err := s.d.Messages.Create(ctx, a, msg)
	if err != nil {
		return sent{}, err
	}
	out := sent{messageID: c.Message.ID, threadID: c.Message.ThreadID, channelID: c.Message.ChannelID, replay: c.Replay}
	if c.Replay {
		info, found, err := s.d.ByParent.ThreadByParentMessage(ctx, c.Message.ID)
		if err != nil {
			return sent{}, err
		}
		if found {
			out.threadID = info.ID
		}
		return out, nil
	}
	title, parent := t.Title, c.Message.ID
	tc, err := s.d.Messages.CreateThread(ctx, a, messaging.CreateThreadCmd{ChannelID: ch.ChannelID, Title: &title, ParentMessageID: &parent})
	if err != nil {
		return sent{}, err
	}
	out.threadID = tc.ThreadID
	return out, nil
}

// threadFact reads what chat knows about a thread named by the webhook or the body.
func (s *IncomingService) threadFact(ctx context.Context, id ids.ID) (*incoming.ThreadFact, error) {
	info, err := s.d.Threads.Info(ctx, id)
	if errs.KindOf(err) == errs.KindNotFound {
		return &incoming.ThreadFact{ID: id}, nil
	}
	if err != nil {
		return nil, err
	}
	return &incoming.ThreadFact{ID: id, ChannelID: info.ChannelID, Found: true}, nil
}

// asNotFound turns every refusal of the app's rights (not_found, forbidden with any reason,
// unauthenticated, step_up_required) into the 404 of an unknown webhook (11 section 8.4); body
// errors and internal errors stay.
func asNotFound(err error) error {
	if k := errs.KindOf(err); k == errs.KindNotFound || k == errs.KindForbidden || k == errs.KindUnauthenticated || k == errs.KindStepUpRequired {
		return notFound()
	}
	return err
}

// ArchiveSubscriber is the in-transaction subscriber of apps for chat.ChannelArchived (02 section
// 4.3, D-M2-27-14). internal/wire builds it with the txn registry, before the module, so it needs
// only the store.
type ArchiveSubscriber struct {
	store IncomingStore
}

// NewArchiveSubscriber returns the subscriber over store. Errors: none; a nil store panics on the
// first event.
func NewArchiveSubscriber(store IncomingStore) *ArchiveSubscriber {
	return &ArchiveSubscriber{store: store}
}

// OnChannelArchived revokes the active incoming webhooks of the archived channel with the archive
// time, in the archive transaction: archiving is irreversible (05 has no unarchive), so such a
// webhook could never post again. The archive is the audited action; no entry of its own.
func (s *ArchiveSubscriber) OnChannelArchived(ctx context.Context, ev chat.ChannelArchived) error {
	_, err := s.store.RevokeIncomingByChannel(ctx, ev.ChannelID, ev.ArchivedAt.Truncate(time.Microsecond))
	return err
}

// revokeIncomingForApp revokes every active webhook of appID inside the unit of the uninstall
// (11 section 4.3, design note 11 a); the entry of app.uninstall stays the record.
func (s *IncomingService) revokeIncomingForApp(ctx context.Context, appID ids.ID) error {
	_, err := s.d.Store.RevokeIncomingByApp(ctx, appID, s.now())
	return err
}
