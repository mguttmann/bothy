// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
)

// prepared is a checked request of a new message (brief D2 and D3).
type prepared struct {
	clientMsgID ids.ID
	origin      domain.Origin
	// kind is text or, for an agent's card, card (M2-15).
	kind       domain.Kind
	content    string
	replyTo    ids.ID
	silent     bool
	clearDraft bool
	// mentionCheck is the mention_ids of the request; checkMentions is false when absent.
	mentionCheck  []ids.ID
	checkMentions bool
	// handles are the distinct mention handles of content.
	handles []string
	hash    []byte
	// card holds the checked components of a card (M2-15); empty for every other kind.
	card cardPrepared
}

// Create implements messaging.Messages (brief D1): the thread (404 for an unknown or invisible
// one), message.create, the body rules, then one unit of work that answers a repetition or sends.
// A concurrent identical request that committed first rolls this unit back with its thread
// number, and a second unit answers like a repetition.
func (s *Service) Create(ctx context.Context, a actor.Actor, cmd messaging.CreateMessageCmd) (messaging.Created, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return messaging.Created{}, err
	}
	info, err := s.loadThread(ctx, a, cmd.ThreadID)
	if err != nil {
		return messaging.Created{}, err
	}
	p, err := s.prepare(ctx, a, cmd, info.ID, "thread:"+info.ID.String())
	if err != nil {
		return messaging.Created{}, err
	}
	var out messaging.Created
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.send(ctx, a, p, info, false)
		return err
	})
	if errors.Is(err, ErrClientMsgIDTaken) {
		return s.afterRace(ctx, a, p)
	}
	if err != nil || out.Replay {
		return out, err
	}
	out.Message, err = s.reread(ctx, a, out.Message.ID)
	return out, err
}

// reread returns the stored representation of the new message id after its unit of work (04
// section 16.5, M2-05 brief note 7): a subscriber of MessagePosted may have changed the row in
// the same unit (runs sets trigger_state received, version 2), so the 201 body is the row as
// committed, read through stored like a replay.
func (s *Service) reread(ctx context.Context, a actor.Actor, id ids.ID) (messaging.Message, error) {
	m, found, err := s.d.Store.Message(ctx, id)
	if err != nil {
		return messaging.Message{}, err
	}
	if !found {
		return messaging.Message{}, errs.New(errs.CodeInternalError, "messaging: a created message is missing after its unit of work")
	}
	visible, err := s.visible(ctx, a)
	if err != nil {
		return messaging.Message{}, err
	}
	return s.stored(ctx, m, visible)
}

// afterRace answers the request whose insert lost against a concurrent identical one (04 section
// 5.1 point 4): in a new unit of work the stored message is the replay or the 422. Inside an
// outer unit of work the transaction is lost, so the request fails with 409 conflict instead.
func (s *Service) afterRace(ctx context.Context, a actor.Actor, p prepared) (messaging.Created, error) {
	if txn.InUnit(ctx) {
		return messaging.Created{}, errs.New(errs.CodeConflict, "a concurrent request with this client_msg_id won")
	}
	var out messaging.Created
	err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
		m, found, err := s.repetition(ctx, a, p)
		if err != nil {
			return err
		}
		if !found {
			return errs.New(errs.CodeInternalError, "messaging: the winning message of a client_msg_id is missing")
		}
		out = messaging.Created{Message: m, Replay: true}
		return nil
	})
	return out, err
}

// loadThread reads the routing data of the thread id and authorizes message.create on it before
// it answers a missing thread, so an unknown and an invisible id give the same 404 (02 section
// 9.2); a read-only or archived channel is 403 channel_read_only from authorize.
func (s *Service) loadThread(ctx context.Context, a actor.Actor, id ids.ID) (chat.ThreadInfo, error) {
	info, err := s.d.Threads.Info(ctx, id)
	found := err == nil
	if err != nil && errs.KindOf(err) != errs.KindNotFound {
		return chat.ThreadInfo{}, err
	}
	r := authorize.Resource{Kind: authorize.ResourceThread, ID: id}
	if found {
		r = threadResource(info)
	}
	if err := s.check(ctx, a, authorize.ActionMessageCreate, r); err != nil {
		return chat.ThreadInfo{}, err
	}
	if !found {
		return chat.ThreadInfo{}, invisible()
	}
	return info, nil
}

// prepare checks the body of a new message for the thread (zero for a thread created with it)
// and computes the request hash with hashContext, or with cmd.HashContext when the caller set
// one (brief D2, D3, M2-27 design note 6c).
func (s *Service) prepare(ctx context.Context, a actor.Actor, cmd messaging.CreateMessageCmd, thread ids.ID, hashContext string) (prepared, error) {
	if err := domain.CheckClientMsgID(cmd.ClientMsgID); err != nil {
		return prepared{}, err
	}
	origin, err := domain.OriginFor(string(a.Kind))
	if err != nil {
		return prepared{}, err
	}
	kind, isCard, err := kindOf(a, cmd)
	if err != nil {
		return prepared{}, err
	}
	content := ""
	if cmd.ContentMD != nil {
		content = *cmd.ContentMD
	}
	if err := domain.CheckContent(content, s.cfg.MaxContentBytes); err != nil {
		return prepared{}, err
	}
	p := prepared{clientMsgID: cmd.ClientMsgID, origin: origin, kind: domain.Kind(kind), content: content, silent: cmd.Silent, clearDraft: cmd.ClearDraft}
	// SkipMentions leaves handles empty, so send resolves and stores no mention (R-EX-09).
	if !cmd.SkipMentions {
		p.handles = domain.ScanMentions(content)
		if err := domain.CheckMentionCount(p.handles, s.cfg.MaxMentions); err != nil {
			return prepared{}, err
		}
		p.mentionCheck, p.checkMentions = cmd.MentionIDs, cmd.MentionIDs != nil
	}
	if cmd.HashContext != "" {
		hashContext = cmd.HashContext
	}
	replyTo := ""
	if cmd.ReplyToID != nil {
		if err := s.checkReply(ctx, *cmd.ReplyToID, thread); err != nil {
			return prepared{}, err
		}
		p.replyTo, replyTo = *cmd.ReplyToID, cmd.ReplyToID.String()
	}
	hash := domain.HashInput{Context: hashContext, Kind: kind, Content: content, ReplyTo: replyTo}
	if isCard {
		if p.card.checked, err = s.checkCard(ctx, a, cmd, thread); err != nil {
			return prepared{}, err
		}
		hash.Card, hash.RunID = p.card.checked.Canonical, runIDText(cmd.RunID)
	}
	p.hash = domain.RequestHash(hash)
	return p, nil
}

// kindOf returns the kind of a new message (text without a kind member) and whether it is a card:
// voice and every other kind are not enabled, attachments neither (M3-07), and the card rules of
// M2-15 apply. Errors: 400 validation_failed.
func kindOf(a actor.Actor, cmd messaging.CreateMessageCmd) (string, bool, error) {
	kind := string(domain.KindText)
	if cmd.Kind != nil {
		kind = *cmd.Kind
	}
	if kind != string(domain.KindText) && kind != string(domain.KindCard) {
		return "", false, domain.NotEnabled("/kind")
	}
	if len(cmd.AttachmentIDs) > 0 {
		return "", false, domain.NotEnabled("/attachment_file_ids")
	}
	isCard, err := cardRules(a, kind, cmd)
	return kind, isCard, err
}

// runIDText is the run_id of the request hash, "" when absent.
func runIDText(run *ids.ID) string {
	if run == nil {
		return ""
	}
	return run.String()
}

// checkReply allows a reply only to a message of the same thread, deleted ones included; an
// unknown id and one of another thread give the same 400, so nothing leaks (03
// messages.reply_to_message_id).
func (s *Service) checkReply(ctx context.Context, target, thread ids.ID) error {
	m, found, err := s.d.Store.Message(ctx, target)
	if err != nil {
		return err
	}
	if !found || thread.IsZero() || m.ThreadID != thread {
		return domain.NotInThread()
	}
	return nil
}

// repetition looks up the author's message with the client_msg_id of p (04 section 5.1 point
// 4): the same hash is the stored original, another hash 422 idempotency_key_reuse.
func (s *Service) repetition(ctx context.Context, a actor.Actor, p prepared) (messaging.Message, bool, error) {
	m, found, err := s.d.Store.MessageByClientID(ctx, a.ID, p.clientMsgID)
	if err != nil || !found {
		return messaging.Message{}, false, err
	}
	if !bytes.Equal(m.PayloadSHA256, p.hash) {
		return messaging.Message{}, false, errs.New(errs.CodeIdempotencyKeyReuse, "client_msg_id was used with another request")
	}
	visible, err := s.visible(ctx, a)
	if err != nil {
		return messaging.Message{}, false, err
	}
	v, err := s.stored(ctx, m, visible)
	return v, true, err
}

// send is step 5 of brief D1 inside the unit of work for the thread of info: a repetition answers
// the original; otherwise the mentions, the thread number (with the reopen and its system line)
// and the rows, then message.created of the unit (the system line first, then the message) and
// only then MessagePosted in the same order (ruling D-M2-05-3): a subscriber that appends a
// message to the same thread, the run anchor, must not overtake the message in the log (04
// section 10). created marks the first message of a thread created in the same unit. The
// returned message is the row as inserted; the callers read it again after the unit (reread).
func (s *Service) send(ctx context.Context, a actor.Actor, p prepared, info chat.ThreadInfo, created bool) (messaging.Created, error) {
	if m, found, err := s.repetition(ctx, a, p); err != nil || found {
		return messaging.Created{Message: m, Replay: found}, err
	}
	mentions, err := s.mentionsFor(ctx, a, p, info, created)
	if err != nil {
		return messaging.Created{}, err
	}
	author, err := s.author(ctx, a.ID)
	if err != nil {
		return messaging.Created{}, err
	}
	now := s.now()
	seq, line, info, err := s.nextSeq(ctx, a, info, author, now)
	if err != nil {
		return messaging.Created{}, err
	}
	lang, err := s.searchLanguage(ctx, a)
	if err != nil {
		return messaging.Created{}, err
	}
	row, err := s.d.Store.InsertMessage(ctx, domain.Message{
		ID: ids.New(), ThreadID: info.ID, ChannelID: info.ChannelID, ThreadSeq: seq, AuthorID: a.ID,
		ClientMsgID: p.clientMsgID, PayloadSHA256: p.hash, Origin: p.origin, Kind: p.kind, ContentMD: p.content,
		ContentFormat: domain.ContentFormat, ReplyToID: p.replyTo, Silent: p.silent, Version: 1, CreatedAt: now,
		SearchLanguage: lang,
	})
	if err != nil {
		return messaging.Created{}, err
	}
	s.countCreated(ctx, domain.Message{Origin: p.origin, Kind: p.kind})
	if err := s.d.Store.InsertMentions(ctx, row.ID, info.ID, mentions, now); err != nil {
		return messaging.Created{}, err
	}
	m := view(row, author, mentions)
	// The components of a card come after the message row and before MessagePosted and
	// message.created, which carries them (M2-15 design note 6 d).
	if p.card.checked != nil {
		p.card.checked.Input.ThreadID = info.ID
	}
	if m.Components, err = s.attach(ctx, row, p.card); err != nil {
		return messaging.Created{}, err
	}
	posted := s.posted(a, info, m, p.clearDraft)
	posted.CreatedThread, posted.ReopenedThread = created, line != nil
	if err := s.announce(ctx, a, info, line, m, posted); err != nil {
		return messaging.Created{}, err
	}
	return messaging.Created{Message: m}, nil
}

// announce appends message.created of the reopen line (when there is one) and of m, then
// publishes MessagePosted of the line and posted of m, in this order (ruling D-M2-05-3).
func (s *Service) announce(ctx context.Context, a actor.Actor, info chat.ThreadInfo, line *messaging.Message, m messaging.Message,
	posted messaging.MessagePosted) error {
	if line != nil {
		if err := s.emitCreated(ctx, info, *line, false); err != nil {
			return err
		}
	}
	if err := s.emitCreated(ctx, info, m, true); err != nil {
		return err
	}
	if line != nil {
		lp := s.posted(a, info, *line, false)
		lp.AuthorDeviceID = nil
		if err := txn.Publish(ctx, lp); err != nil {
			return err
		}
	}
	return txn.Publish(ctx, posted)
}

// searchLanguage returns the text search configuration of a message by a (docs/spec/14-search.md
// 3.1, docs/DECISIONS.md D-M3-13-1): origin human takes the language of the person, origin agent
// and app the instance language without reading the principal. It reads inside the unit of work
// of the insert; the value is set once and no later write changes it.
func (s *Service) searchLanguage(ctx context.Context, a actor.Actor) (string, error) {
	if a.Kind != actor.KindUser {
		return s.d.Directory.InstanceSearchLanguage(ctx)
	}
	return s.d.Directory.SearchLanguage(ctx, a.ID)
}

// mentionsFor resolves the mentions of p in the thread of info (the channel path of a thread
// created in this unit) and checks them against mention_ids of the request.
func (s *Service) mentionsFor(ctx context.Context, a actor.Actor, p prepared, info chat.ThreadInfo, created bool) ([]ids.ID, error) {
	res := threadResource(info)
	if created {
		res = newThreadResource(info)
	}
	mentions, err := s.resolveMentions(ctx, a, p.handles, res)
	if err != nil {
		return nil, err
	}
	if p.checkMentions && !sameSet(p.mentionCheck, mentions) {
		return nil, domain.MentionMismatch()
	}
	return mentions, nil
}

// nextSeq takes the thread number of the new message under the row lock of chat. A closed thread
// is reopened: the system line takes the first number and the message the next (brief D6). It
// returns the routing data of the locked thread, the current channel included.
func (s *Service) nextSeq(ctx context.Context, a actor.Actor, info chat.ThreadInfo, author messaging.PrincipalRef,
	now time.Time) (int64, *messaging.Message, chat.ThreadInfo, error) {
	appended, err := s.d.Threads.AppendToThread(ctx, a, chat.AppendCmd{ThreadID: info.ID, ReopenIfClosed: true})
	if err != nil {
		return 0, nil, info, err
	}
	info = appended.Thread
	if !appended.Reopened {
		return appended.Seq, nil, info, nil
	}
	line, err := s.systemLine(ctx, a, info, author, appended.Seq, now)
	if err != nil {
		return 0, nil, info, err
	}
	if appended, err = s.d.Threads.AppendToThread(ctx, a, chat.AppendCmd{ThreadID: info.ID}); err != nil {
		return 0, nil, info, err
	}
	return appended.Seq, line, info, nil
}

// systemLine writes the line a reopen leaves at seq (brief D6); send appends its message.created
// and publishes its MessagePosted.
func (s *Service) systemLine(ctx context.Context, a actor.Actor, info chat.ThreadInfo, author messaging.PrincipalRef,
	seq int64, now time.Time) (*messaging.Message, error) {
	line := domain.SystemLine(domain.SystemThreadReopened, info.ID, info.ChannelID, a.ID, seq)
	line.CreatedAt = now
	// Origin system takes the instance language, whoever caused the line (14 3.1, D-M3-13-1).
	lang, err := s.d.Directory.InstanceSearchLanguage(ctx)
	if err != nil {
		return nil, err
	}
	line.SearchLanguage = lang
	row, err := s.d.Store.InsertMessage(ctx, line)
	if err != nil {
		return nil, err
	}
	s.countCreated(ctx, line)
	m := view(row, author, nil)
	return &m, nil
}

// posted builds the in-transaction event of m (02 section 4.3).
func (s *Service) posted(a actor.Actor, info chat.ThreadInfo, m messaging.Message, clearDraft bool) messaging.MessagePosted {
	ev := messaging.MessagePosted{
		MessageID: m.ID, ThreadID: m.ThreadID, ChannelID: m.ChannelID, SpaceID: info.SpaceID, ThreadKind: info.Kind,
		ThreadSeq: m.ThreadSeq, AuthorID: m.Author.ID, AuthorKind: string(a.Kind), Origin: m.Origin, Kind: m.Kind,
		ClientMsgID: m.ClientMsgID, MentionedIDs: slices.Clone(m.MentionIDs), Silent: m.Silent, ClearDraft: clearDraft,
		CreatedAt: m.CreatedAt,
	}
	if a.Kind == actor.KindUser && !a.DeviceID.IsZero() {
		d := a.DeviceID
		ev.AuthorDeviceID = &d
	}
	if !m.ReplyToID.IsZero() {
		r := m.ReplyToID
		ev.ReplyToID = &r
	}
	return ev
}

// resolveMentions resolves the handles for the author a in the thread r (07 section 4.6, brief
// D5): one batched lookup of active people and agents, then per principal the checks in filter
// mode, so a token that stays text writes no audit entry. A person is mentioned when the author
// may read the person (user.read) or the person may read the thread; an agent when the author may
// read the agent (agent.read). The result is sorted and without duplicates.
func (s *Service) resolveMentions(ctx context.Context, a actor.Actor, handles []string, r authorize.Resource) ([]ids.ID, error) {
	if len(handles) == 0 {
		return []ids.ID{}, nil
	}
	found, err := s.d.Directory.ActiveByHandles(ctx, handles)
	if err != nil {
		return nil, err
	}
	fctx := authorize.Filter(ctx)
	out := []ids.ID{}
	for _, h := range handles {
		v, ok := found[h]
		if !ok {
			continue
		}
		var mention bool
		switch v.Kind {
		case actor.KindUser:
			mention, err = s.personMentioned(fctx, a, v.ID, r)
		case actor.KindAgent:
			mention, err = s.allowed(fctx, a, authorize.ActionAgentRead, authorize.Resource{Kind: authorize.ResourceAgent, ID: v.ID, OwnerID: v.ID})
		case actor.KindApp, actor.KindHost, actor.KindAnonymous:
		}
		if err != nil {
			return nil, err
		}
		if mention && !slices.Contains(out, v.ID) {
			out = append(out, v.ID)
		}
	}
	slices.SortFunc(out, func(x, y ids.ID) int { return bytes.Compare(x[:], y[:]) })
	return out, nil
}

// personMentioned decides the person rule of 07 section 4.6. The person is checked without a
// session, and authz grants the owner role only in a session, so an active owner, who reads
// every thread (07 section 2.1), counts as a reader through the directory (docs/DECISIONS.md
// M1-56/4).
func (s *Service) personMentioned(ctx context.Context, a actor.Actor, person ids.ID, r authorize.Resource) (bool, error) {
	ok, err := s.allowed(ctx, a, authorize.ActionUserRead, authorize.Resource{Kind: authorize.ResourceUser, ID: person, OwnerID: person})
	if err != nil || ok {
		return ok, err
	}
	if ok, err = s.allowed(ctx, actor.Actor{ID: person, Kind: actor.KindUser}, authorize.ActionThreadRead, r); err != nil || ok {
		return ok, err
	}
	u, err := s.d.Directory.User(ctx, person)
	if err != nil {
		return false, err
	}
	return u.IsOwner, nil
}

// sameSet reports whether a and b hold the same ids, ignoring order and repetition.
func sameSet(a, b []ids.ID) bool {
	for _, x := range a {
		if !slices.Contains(b, x) {
			return false
		}
	}
	for _, x := range b {
		if !slices.Contains(a, x) {
			return false
		}
	}
	return true
}
