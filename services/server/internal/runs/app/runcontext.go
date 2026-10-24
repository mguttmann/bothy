// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"log/slog"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// The run context run_context.v1 (09 section 4.6, ADR 0054; M2-13). A context read opens no unit
// of work and takes no lock: every part, trust included, is recomputed on each request (ruling
// D-M2-13-13). Logs carry the run id at most, never content, a handle, a display name or a list
// of message ids (19 section 9).

// transcriptDone is files.transcript_status of a finished transcript (03 files).
const transcriptDone = "done"

// msgContextTriggerKind is the log line of a run whose trigger is not a message.
const (
	msgContextTriggerKind = "runs: run context of a run whose trigger is not a message"
	logKeyTriggerKind     = "bothy.run.trigger_kind"
)

// ContextMessages is the part of messaging.Messages the run context reads (M2-13 design note 3):
// the windows and single messages with full content, each checked with message.read in filter
// mode for the principal of ctx.
type ContextMessages interface {
	ContextWindow(ctx context.Context, q messaging.ContextQuery) (messaging.ContextPage, error)
	ContextMessage(ctx context.Context, messageID ids.ID) (messaging.Message, bool, error)
}

// ContextRepository is the outgoing port of the run-context reads of runs (M2-13 design notes 5
// and 9); adapters/postgres implements it next to Repository. Each read names one run, thread or
// session row of the run whose run.context.read the use case authorized first (07 section 7.4
// rule 1). Neither returns content.
type ContextRepository interface {
	// EarlierSessionRun reports whether a started run of row's agent in row's thread with
	// agent_session_id = sessionID was queued before row (used), and the triggering message of the
	// latest such run that has one (zero for none).
	EarlierSessionRun(ctx context.Context, row RunRow, sessionID ids.ID) (used bool, triggerMessageID ids.ID, err error)
	// DeliveredFollowups returns the messages among messageIDs that a run of the session row in
	// threadID received as follow-ups (run_followups in delivered or absorbed).
	DeliveredFollowups(ctx context.Context, threadID, sessionID ids.ID, messageIDs []ids.ID) ([]ids.ID, error)
}

// Context implements runs.Runs (getRunContext, run.context.read; M2-13 design note 1). Order: the
// run; authorize with the run's agent as owner (agent-self); an inactive run, a run whose
// triggering message is gone and a triggering message the agent may not read give the not_found of
// an unknown run; a trigger that is not a message is internal; then the thread, the channel
// switches, the session plan, the windows, cutting and budget, the trust of every element from
// authorize.TriggerHolders, and already_delivered. Any source error ends the request with its
// kind and never turns into a context with weaker trust (fail closed, 09 1 point 4).
func (s *Service) Context(ctx context.Context, runID ids.ID) (runs.RunContext, error) {
	if s.d.ContextMessages == nil || s.d.TriggerHolders == nil || s.d.ContextRows == nil ||
		s.cfg.ContextMaxBytes < 1 || s.cfg.ContextMessageMaxBytes < 1 {
		return runs.RunContext{}, errs.New(errs.CodeInternalError, "runs: the run context is not configured")
	}
	a := caller(ctx)
	row, found, err := s.d.Repo.Run(ctx, runID)
	if err != nil {
		return runs.RunContext{}, err
	}
	d, err := s.authorizeAgent(ctx, a, authorize.ActionRunContextRead, row, found, runID, nil)
	if err != nil {
		return runs.RunContext{}, err
	}
	if !d.Allowed() {
		return runs.RunContext{}, d.Err()
	}
	// Only an active run has a context (ruling D-M2-13-4); queued is active, because the adapter
	// reads the context before run.start (04 15.3 point 4, 09 4.9).
	if !row.State.Active() {
		return runs.RunContext{}, invisible()
	}
	if row.TriggerKind != string(domain.TriggerMessage) {
		s.d.Log.LogAttrs(ctx, slog.LevelError, msgContextTriggerKind, slog.String(logKeyRunID, row.ID.String()),
			slog.String(logKeyTriggerKind, row.TriggerKind))
		return runs.RunContext{}, errs.New(errs.CodeInternalError, "runs: the run context of this trigger kind is not defined")
	}
	if row.TriggeringMessageID.IsZero() {
		return runs.RunContext{}, invisible()
	}
	b, err := s.contextFacts(ctx, row)
	if err != nil {
		return runs.RunContext{}, err
	}
	return s.buildContext(ctx, b)
}

// contextBuild collects the facts of one context.
type contextBuild struct {
	row     RunRow
	info    chat.ThreadInfo
	limit   int
	plan    domain.ContextPlan
	session ids.ID
	trigger messaging.Message
	thread  messaging.ContextPage
	channel *messaging.ContextPage
}

// contextFacts reads the thread, the channel switches, the session plan, the triggering message
// and the windows of row (design note 1 steps e to h).
func (s *Service) contextFacts(ctx context.Context, row RunRow) (contextBuild, error) {
	b := contextBuild{row: row}
	var err error
	if b.info, err = s.d.Threads.Info(ctx, row.ThreadID); err != nil {
		return contextBuild{}, err
	}
	ch, err := s.d.Channels.TriggerChannel(ctx, b.info.ChannelID)
	if err != nil {
		return contextBuild{}, err
	}
	b.limit = max(ch.HistoryBackfillLimit, 0)
	if b.plan, b.session, err = s.contextPlan(ctx, row, operatorOf(b.info, ch)); err != nil {
		return contextBuild{}, err
	}
	m, ok, err := s.d.ContextMessages.ContextMessage(ctx, row.TriggeringMessageID)
	if err != nil {
		return contextBuild{}, err
	}
	// A triggering message the agent cannot read ends the request like an unknown run (ruling
	// D-M2-13-12); M lies in T or is the parent message of T (auto thread, 09 3.7).
	inThread := ok && m.ThreadID == row.ThreadID
	if !ok || (!inThread && m.ID != b.info.ParentMessageID) {
		return contextBuild{}, invisible()
	}
	b.trigger = m
	if inThread {
		q := messaging.ContextQuery{ThreadID: row.ThreadID, AfterSeq: b.plan.FloorSeq, BeforeSeq: m.ThreadSeq, Limit: b.limit}
		if b.plan.ExcludeOwn {
			q.ExcludeAuthorID = row.AgentID
		}
		if b.thread, err = s.d.ContextMessages.ContextWindow(ctx, q); err != nil {
			return contextBuild{}, err
		}
	} else {
		// T has no message before its parent message: the thread part is empty, the floor zero.
		b.plan.FloorSeq = 0
	}
	if b.channel, err = s.channelWindow(ctx, b); err != nil {
		return contextBuild{}, err
	}
	return b, nil
}

// contextPlan returns the session plan of row (design note 5) and the session row it is based on
// (zero for none): whether the agent operates the thread, the session row the run was invoked
// with (runs.agent_session_id, so a reset or bind during the run does not change the mode the
// invocation announced, 09 9.4) and the earlier started runs in that session.
func (s *Service) contextPlan(ctx context.Context, row RunRow, operator ids.ID) (domain.ContextPlan, ids.ID, error) {
	f := domain.PlanFacts{Operator: !operator.IsZero() && operator == row.AgentID}
	if !f.Operator || row.AgentSessionID.IsZero() || s.d.RunSessions == nil {
		return domain.PlanContext(f), ids.ID{}, nil
	}
	sess, found, err := s.d.RunSessions.RunSession(ctx, row.ThreadID, row.AgentSessionID)
	if err != nil {
		return domain.ContextPlan{}, ids.ID{}, err
	}
	if !found || sess.AgentID != row.AgentID {
		return domain.PlanContext(f), ids.ID{}, nil
	}
	f.HasSession, f.Started = true, sess.Started
	if f.Earlier, err = s.earlierRuns(ctx, row, sess.ID); err != nil {
		return domain.ContextPlan{}, ids.ID{}, err
	}
	return domain.PlanContext(f), sess.ID, nil
}

// earlierRuns returns the earlier started runs of row's agent in the session row, as PlanFacts
// needs them: none, or the latest one with a triggering message (or one without, when no earlier
// run has one).
func (s *Service) earlierRuns(ctx context.Context, row RunRow, sessionID ids.ID) ([]domain.EarlierRun, error) {
	used, trigger, err := s.d.ContextRows.EarlierSessionRun(ctx, row, sessionID)
	if err != nil || !used {
		return nil, err
	}
	e, err := s.earlierTrigger(ctx, row.ThreadID, trigger)
	if err != nil {
		return nil, err
	}
	return []domain.EarlierRun{e}, nil
}

// earlierTrigger describes the triggering message of the latest earlier run of the session: its
// position, and whether it lies in the thread (TriggerMessageOf reads no content). A missing
// message counts as none.
func (s *Service) earlierTrigger(ctx context.Context, threadID, messageID ids.ID) (domain.EarlierRun, error) {
	if messageID.IsZero() {
		return domain.EarlierRun{}, nil
	}
	tm, err := s.d.Messages.TriggerMessageOf(ctx, messageID)
	if errs.KindOf(err) == errs.KindNotFound {
		return domain.EarlierRun{}, nil
	}
	if err != nil {
		return domain.EarlierRun{}, err
	}
	return domain.EarlierRun{HasTrigger: true, TriggerInThread: tm.ThreadID == threadID, TriggerSeq: tm.ThreadSeq}, nil
}

// channelWindow reads channel_context (design note 6, ruling D-M2-13-6): only when the plan wants
// it and the thread has a parent message that lies in the thread's current channel; the parent
// itself is not part of it. nil means the part is absent (also after a move, so content of the
// source channel never reaches the target's readers).
func (s *Service) channelWindow(ctx context.Context, b contextBuild) (*messaging.ContextPage, error) {
	if !b.plan.WantChannel || b.info.ParentMessageID.IsZero() {
		return nil, nil
	}
	p := b.trigger
	if p.ID != b.info.ParentMessageID {
		var ok bool
		var err error
		if p, ok, err = s.d.ContextMessages.ContextMessage(ctx, b.info.ParentMessageID); err != nil || !ok {
			return nil, err
		}
	}
	if p.ChannelID != b.info.ChannelID {
		return nil, nil
	}
	page, err := s.d.ContextMessages.ContextWindow(ctx, messaging.ContextQuery{ThreadID: p.ThreadID, BeforeSeq: p.ThreadSeq, Limit: b.limit})
	if err != nil {
		return nil, err
	}
	return &page, nil
}

// buildContext cuts and budgets the windows, sets truncated_before_seq, classifies every element
// and marks the delivered follow-ups (design note 1 steps i to l).
func (s *Service) buildContext(ctx context.Context, b contextBuild) (runs.RunContext, error) {
	cut := func(list []messaging.Message) []runs.ContextMessage {
		out := make([]runs.ContextMessage, 0, len(list))
		for _, m := range list {
			e := contextElement(m)
			e.ContentMD, e.Truncated = domain.CutContent(m.ContentMD, s.cfg.ContextMessageMaxBytes)
			out = append(out, e)
		}
		return out
	}
	var channel []runs.ContextMessage
	if b.channel != nil {
		channel = cut(b.channel.Items)
	}
	thread := cut(b.thread.Items)
	size := func(m runs.ContextMessage) int { return len(m.ContentMD) }
	keptCh, keptTh := domain.FitBudget(channel, thread, size, s.cfg.ContextMaxBytes)
	seqs := make([]int64, 0, len(keptTh))
	for _, m := range keptTh {
		seqs = append(seqs, m.ThreadSeq)
	}
	out := runs.RunContext{RunID: b.row.ID, ContextFloorSeq: b.plan.FloorSeq, TriggerMessage: contextElement(b.trigger), ThreadContext: keptTh}
	out.TruncatedBeforeSeq, out.HasTruncatedBeforeSeq = domain.TruncatedBeforeSeq(b.thread.HasOlder, len(keptTh) < len(thread), seqs, b.trigger.ThreadSeq)
	if b.channel != nil {
		out.ChannelContext = keptCh
	}
	if err := s.contextAttachments(ctx, b, &out); err != nil {
		return runs.RunContext{}, err
	}
	if err := s.classify(ctx, b, &out); err != nil {
		return runs.RunContext{}, err
	}
	if err := s.markDelivered(ctx, b, &out); err != nil {
		return runs.RunContext{}, err
	}
	return out, nil
}

// contextElement converts a message of messaging into an element without role and trust.
func contextElement(m messaging.Message) runs.ContextMessage {
	return runs.ContextMessage{
		MessageID: m.ID, ChannelID: m.ChannelID, ThreadSeq: m.ThreadSeq, CreatedAt: m.CreatedAt,
		Author: runs.ContextAuthor{ID: m.Author.ID, Kind: m.Author.Kind, Handle: m.Author.Handle, DisplayName: m.Author.DisplayName,
			AvatarFileID: m.Author.AvatarFileID},
		Origin: m.Origin, Kind: m.Kind, ContentMD: m.ContentMD, Attachments: []runs.ContextAttachment{}, ReplyToMessageID: m.ReplyToID,
	}
}

// contextAttachments fills the attachments of the elements of out (09 section 4.6, schema
// RunContextAttachment; M3-09 design note 12 d) with one ForMessages call of files for the
// messages of b flagged with has_attachments that are still elements after the budget; a context
// without such a message and a service without files make no call. An attachment carries
// transcript only while its transcript_status is done; it sits inside its element, so the
// transcript has the trust of its message (the words of its author, ADR 0054 point 4), while the
// file contents stay marked data that the adapter fetches with file.read.
func (s *Service) contextAttachments(ctx context.Context, b contextBuild, out *runs.RunContext) error {
	if s.d.Files == nil {
		return nil
	}
	flagged := flaggedMessages(b)
	all := contextElements(out)
	var want []ids.ID
	for _, m := range all {
		if flagged[m.MessageID] {
			want = append(want, m.MessageID)
		}
	}
	if len(want) == 0 {
		return nil
	}
	got, err := s.d.Files.ForMessages(ctx, want)
	if err != nil {
		return err
	}
	for _, m := range all {
		for _, a := range got[m.MessageID] {
			m.Attachments = append(m.Attachments, contextAttachment(a, m.Kind == kindVoice))
		}
	}
	return nil
}

// flaggedMessages returns the messages of b whose has_attachments is set (a tombstone never has
// it): the triggering message and the items of both windows.
func flaggedMessages(b contextBuild) map[ids.ID]bool {
	list := append([]messaging.Message{b.trigger}, b.thread.Items...)
	if b.channel != nil {
		list = append(list, b.channel.Items...)
	}
	out := map[ids.ID]bool{}
	for _, m := range list {
		if m.HasAttachments {
			out[m.ID] = true
		}
	}
	return out
}

// contextElements returns pointers to every element of out: the triggering message, then
// channel_context, then thread_context.
func contextElements(out *runs.RunContext) []*runs.ContextMessage {
	all := []*runs.ContextMessage{&out.TriggerMessage}
	for i := range out.ChannelContext {
		all = append(all, &out.ChannelContext[i])
	}
	for i := range out.ThreadContext {
		all = append(all, &out.ThreadContext[i])
	}
	return all
}

// contextAttachment converts an attachment of files into an attachment of the run context:
// transcript only for the attachment of a voice message (voice) with transcript_status done,
// absent otherwise (09 4.6, M3-09 design note 12 d).
func contextAttachment(a files.Attachment, voice bool) runs.ContextAttachment {
	out := runs.ContextAttachment{FileID: a.File.ID, OriginalName: a.File.OriginalName, MimeType: a.File.MIMEType,
		SizeBytes: a.File.SizeBytes, ScanStatus: a.File.ScanStatus}
	// Only the transcript of a voice message carries the trust of its message (09 4.6, ADR 0054
	// point 4); a transcript of an audio file attached to any other message is file content and
	// never stands inline (T-AI-03).
	if voice && a.File.TranscriptStatus == transcriptDone {
		out.Transcript = a.Transcript
	}
	return out
}

// classify sets role and trust of every element (design notes 4 and 7): one TriggerHolders call
// for the distinct human authors other than the triggering person, on the thread's current
// channel.
func (s *Service) classify(ctx context.Context, b contextBuild, out *runs.RunContext) error {
	parties := domain.RunParties{AgentID: b.row.AgentID, TriggeredBy: b.row.TriggeredBy}
	all := contextElements(out)
	seen := map[ids.ID]bool{}
	var people []ids.ID
	for _, m := range all {
		if m.Origin == originHuman && m.Author.ID != b.row.TriggeredBy && !seen[m.Author.ID] {
			seen[m.Author.ID] = true
			people = append(people, m.Author.ID)
		}
	}
	holders := map[ids.ID]bool{}
	if len(people) > 0 {
		got, err := s.d.TriggerHolders.WithTrigger(ctx, authorize.Resource{Kind: authorize.ResourceChannel, ID: b.info.ChannelID}, people)
		if err != nil {
			return err
		}
		for _, id := range got {
			holders[id] = true
		}
	}
	for _, m := range all {
		role, trust := domain.Classify(domain.AuthorFacts{AuthorID: m.Author.ID, Origin: m.Origin}, parties, holders)
		m.Role, m.Trust = string(role), string(trust)
	}
	return nil
}

// markDelivered sets already_delivered in mode resume (design note 9).
func (s *Service) markDelivered(ctx context.Context, b contextBuild, out *runs.RunContext) error {
	if b.plan.Mode != domain.SessionResume || b.session.IsZero() || len(out.ThreadContext)+len(out.ChannelContext) == 0 {
		return nil
	}
	list := make([]ids.ID, 0, len(out.ThreadContext)+len(out.ChannelContext))
	for _, m := range out.ChannelContext {
		list = append(list, m.MessageID)
	}
	for _, m := range out.ThreadContext {
		list = append(list, m.MessageID)
	}
	got, err := s.d.ContextRows.DeliveredFollowups(ctx, b.row.ThreadID, b.session, list)
	if err != nil {
		return err
	}
	delivered := map[ids.ID]bool{}
	for _, id := range got {
		delivered[id] = true
	}
	for i := range out.ThreadContext {
		out.ThreadContext[i].AlreadyDelivered = delivered[out.ThreadContext[i].MessageID]
	}
	for i := range out.ChannelContext {
		out.ChannelContext[i].AlreadyDelivered = delivered[out.ChannelContext[i].MessageID]
	}
	return nil
}
