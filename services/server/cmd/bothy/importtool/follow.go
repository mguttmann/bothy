// SPDX-License-Identifier: Apache-2.0

package importtool

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/api/restclient"
	"github.com/OWNER/bothy/services/server/cmd/bothy/importtool/discord"
)

// mirror is the read mirror of 22 12 over the walker of a run: rounds of reads with the settle
// rule and, with --apply, the writes of the walker (brief design notes 2 to 4).
type mirror struct {
	o     options
	env   Env
	wk    *walker
	reads bothyReads
	src   *restclient.Uuid // nil in a dry mirror without a source
	ready bool             // the structure is posted and the posted files are loaded (--apply)

	// The state of the current round.
	cur      cursorDoc
	leads    map[string]string
	starters map[string]*discord.Message
	notes    []string
}

// The keys of the debug log of the mirror; the values carry no content.
const (
	logKeyError  = "error"
	logKeyReason = "reason"
	logKeyHeld   = "held"
)

// sourceEndError ends the mirror because the owner completed (err nil, exit 0) or failed (exit 1) the
// source (brief design note 3).
type sourceEndError struct{ err error }

// Error returns the reason.
func (e *sourceEndError) Error() string {
	if e.err == nil {
		return "the import source is completed"
	}
	return e.err.Error()
}

// follow runs the read mirror: with --once one round, else rounds until a signal (ctx), a lost
// lease or the end of the source. A failed round never ends --follow: the next one waits with the
// backoff of brief design note 3. With --apply the run holds the lease of M8-04 the whole time.
func follow(ctx context.Context, o options, env Env, wk *walker, src *restclient.ImportSource) error {
	m := &mirror{o: o, env: env, wk: wk, reads: bothyReads{api: wk.api}}
	if src != nil {
		m.src = &src.Id
	}
	// The work of a round runs on a context a signal does not cancel: the walker finishes the
	// current batch and stops through halt, so a stop never leaves a batch half sent.
	work := context.WithoutCancel(ctx)
	wk.halt = func() bool { return ctx.Err() != nil }
	wk.noStats = true
	if o.apply {
		if err := wk.w.acquire(ctx, o.takeover); err != nil {
			return err
		}
		wk.present = wk.w.server
	}
	failures := 0
	for first := true; ; first = false {
		var err error
		if !first {
			err = m.wait(ctx, backoff(wk.cfg, failures))
		}
		if err == nil {
			err = m.round(work)
		}
		done, ret := m.outcome(ctx, err)
		if done {
			return ret
		}
		if err != nil {
			failures++
			m.failed(err, failures)
		} else {
			failures = 0
		}
	}
}

// outcome decides after a wait or a round whether the mirror ends and with which error: a stop
// request, --once, a lost lease or the end of the source end it; any other error is a failed
// round and the loop goes on.
func (m *mirror) outcome(ctx context.Context, err error) (bool, error) {
	work := context.WithoutCancel(ctx)
	var end *sourceEndError
	var le *leaseError
	switch {
	case errors.As(err, &end):
		return true, end.err
	case errors.As(err, &le):
		return true, err
	case errors.Is(err, errHalted), ctx.Err() != nil:
		return true, m.finish(work)
	case err != nil && m.o.once:
		_ = m.finish(work)
		return true, err
	case err == nil && m.o.once:
		return true, m.finish(work)
	}
	return false, nil
}

// backoff is the wait before the next round (brief design note 3): import.follow.poll_interval
// after a success, poll_interval * 2^failures after failed rounds, at most
// import.follow.max_backoff.
func backoff(cfg Config, failures int) time.Duration {
	d := cfg.FollowPollInterval
	for range failures {
		if d >= cfg.FollowMaxBackoff {
			break
		}
		d *= 2
	}
	return min(d, cfg.FollowMaxBackoff)
}

// failed reports a failed round on stderr; the error texts of the clients carry no content.
func (m *mirror) failed(err error, failures int) {
	_, _ = fmt.Fprintf(m.env.Stderr, "bothy import discord: mirror round failed (%d in a row), next round in %s: %s\n",
		failures, backoff(m.wk.cfg, failures), err.Error())
}

// wait sleeps d in pieces of at most import.follow.poll_interval and, with --apply, renews the
// lease after each piece, so a long backoff never lets the lease of a living mirror age past
// import.lease_stale_after (22 7.4). A renewal refused because the source is not running is no
// failure: the round after the wait notes the paused source.
func (m *mirror) wait(ctx context.Context, d time.Duration) error {
	sleep := m.env.FollowSleep
	if sleep == nil {
		sleep = m.env.sleeper()
	}
	for d > 0 {
		step := min(d, m.wk.cfg.FollowPollInterval)
		if err := sleep(ctx, step); err != nil {
			return err
		}
		d -= step
		if m.o.apply {
			if err := m.wk.w.renew(context.WithoutCancel(ctx)); err != nil && !notRunning(err) {
				return err
			}
		}
	}
	return nil
}

// finish clears the lease at the end of the mirror; a source that is not running any more keeps
// it until it is stale, because setImportCursor refuses such a source.
func (m *mirror) finish(ctx context.Context) error {
	if !m.o.apply {
		return nil
	}
	if err := m.wk.w.release(ctx, nil); err != nil && !notRunning(err) {
		return err
	}
	return nil
}

// notRunning reports the 409 import_not_running of a source that is not running.
func notRunning(err error) bool {
	var a *APIError
	return errors.As(err, &a) && a.Reason == reasonNotRunning
}

// threadLocal reports an error that concerns one thread only (brief design note 3): a 403 or 404
// of Discord on it, or a 4xx of the server for one of its batches except 401, 429 and a source that is
// not running. Every other error (network, 5xx, rate wait above the limit, the lease) fails the
// round.
func threadLocal(err error) bool {
	if errors.Is(err, discord.ErrForbidden) || errors.Is(err, discord.ErrNotFound) {
		return true
	}
	var a *APIError
	if errors.As(err, &a) {
		return a.Status >= http.StatusBadRequest && a.Status < http.StatusInternalServerError && a.Status != http.StatusUnauthorized &&
			a.Status != http.StatusTooManyRequests && a.Reason != reasonNotRunning
	}
	return false
}

// round is one round of 22 12.2: the source and its leads, the lists of Discord, every channel in
// scope, the report. With --apply the lease ticker of M8-04 renews the lease during the round.
func (m *mirror) round(ctx context.Context) error {
	m.notes, m.starters = nil, map[string]*discord.Message{}
	wk := m.wk
	wk.known, wk.userDone = map[string]*refInfo{}, map[string]bool{}
	for _, c := range wk.p.channels {
		c.rep = &channelReport{}
	}
	run, err := m.source(ctx)
	if err != nil || !run {
		if err == nil {
			m.print()
		}
		return err
	}
	if !m.o.apply {
		return m.channels(ctx)
	}
	runCtx, stop := context.WithCancelCause(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		wk.w.renewLoop(runCtx, wk.cfg.FollowPollInterval, m.env.leaseSleeper(), stop)
	}()
	err = m.prepare(runCtx)
	if err == nil {
		err = m.channels(runCtx)
	}
	if cause := context.Cause(runCtx); err != nil && cause != nil && !errors.Is(cause, context.Canceled) {
		err = cause
	}
	stop(nil)
	<-done
	return err
}

// source reads the source of the round: its state, the leads (re-read every round, because the
// owner changes them in the web console, brief design note 2 b) and, in a dry mirror, the cursor.
// run is false when the round has nothing to do (a paused source).
func (m *mirror) source(ctx context.Context) (bool, error) {
	if m.src == nil {
		// A dry mirror without a source: the leads of the file, nothing copied yet.
		m.leads, m.cur = map[string]string{}, decodeCursor(nil)
		for _, c := range m.wk.m.Channels {
			m.leads[c.Channel] = c.Lead
		}
		return true, nil
	}
	src, err := m.reads.source(ctx, *m.src)
	if err != nil {
		return false, err
	}
	m.leads = LeadsOf(map[string]any(src.Config))
	if !m.o.apply {
		m.cur = decodeCursor(src.Cursor)
		return true, nil
	}
	switch src.State {
	case restclient.ImportSourceStateRunning:
		m.cur = m.wk.w.snapshot()
		return true, nil
	case restclient.ImportSourceStateCompleted:
		return false, &sourceEndError{}
	case restclient.ImportSourceStateFailed:
		return false, &sourceEndError{err: errors.New("the import source failed")}
	case restclient.ImportSourceStateDraft, restclient.ImportSourceStatePaused:
	}
	m.notes = append(m.notes, noteSourcePaused)
	return false, nil
}

// prepare posts the structure and loads the posted files once per run (the start of the mirror
// may come before or after a full run).
func (m *mirror) prepare(ctx context.Context) error {
	if m.ready {
		return nil
	}
	if err := m.wk.structure(ctx); err != nil {
		return err
	}
	if err := m.wk.loadPostedFiles(ctx); err != nil {
		return err
	}
	m.ready = true
	return nil
}

// channels reads the lists of Discord and mirrors every channel in scope in Discord order.
func (m *mirror) channels(ctx context.Context) error {
	wk := m.wk
	chans, err := wk.dc.GuildChannels(ctx, wk.m.GuildID)
	if err != nil {
		return err
	}
	active, err := wk.dc.ActiveThreads(ctx, wk.m.GuildID)
	if err != nil {
		return err
	}
	m.refresh(chans, active)
	for _, c := range wk.p.channels {
		if !c.inScope() {
			continue
		}
		if err := m.channel(ctx, c); err != nil {
			return err
		}
		if wk.halt != nil && wk.halt() {
			return errHalted
		}
	}
	m.print()
	return nil
}

// refresh takes the fresh channel objects and the active threads into the guild state.
func (m *mirror) refresh(chans, active []discord.Channel) {
	gs, p := m.wk.gs, m.wk.p
	for _, ch := range chans {
		gs.byID[ch.ID] = ch
		if c := p.byID[ch.ID]; c != nil {
			c.ch = ch
		}
	}
	gs.active, gs.activeBy = map[string]bool{}, map[string][]discord.Channel{}
	for _, t := range active {
		gs.active[t.ID] = true
		gs.byID[t.ID] = t
		gs.activeBy[t.ParentID] = append(gs.activeBy[t.ParentID], t)
	}
}

// channel mirrors one channel: a lead=bothy channel with new Discord messages only gets the
// warning of 22 12.2 item 4; a lead=discord channel copies its main thread and its threads.
func (m *mirror) channel(ctx context.Context, c *channelPlan) error {
	if m.leads[c.ch.ID] == LeadBothy {
		if m.writtenAfterSwitch(c) {
			c.rep.note(noteAfterSwitch)
		}
		return nil
	}
	if c.kind == kindText && c.selected && newer(c.ch.LastMessageID, m.cur.position(true, c.ch.ID)) {
		if err := m.local(c, m.thread(ctx, c, nil)); err != nil {
			return err
		}
	}
	m.addThreads(c)
	for _, t := range c.threads {
		if !t.selected {
			continue
		}
		if m.wk.halt != nil && m.wk.halt() {
			return errHalted
		}
		if err := m.local(c, m.mirrorThread(ctx, c, t)); err != nil {
			return err
		}
	}
	return nil
}

// local turns an error of one thread into a note of its channel, so the round continues with the
// next thread (brief design note 3); every other error is returned.
func (m *mirror) local(c *channelPlan, err error) error {
	if err == nil || !threadLocal(err) {
		return err
	}
	m.wk.log.DebugContext(context.Background(), "mirror: thread failed", slog.String(logKeyError, err.Error()))
	c.rep.note(noteThreadFailed)
	return nil
}

// writtenAfterSwitch reports new Discord messages in a lead=bothy channel: its main thread or an
// active thread beyond the cursor.
func (m *mirror) writtenAfterSwitch(c *channelPlan) bool {
	// A forum has no main thread: its last_message_id is its newest post, which the threads cover.
	if c.kind == kindText && newer(c.ch.LastMessageID, m.cur.position(true, c.ch.ID)) {
		return true
	}
	for _, t := range m.wk.gs.activeBy[c.ch.ID] {
		if newer(t.LastMessageID, m.cur.position(false, t.ID)) {
			return true
		}
	}
	return false
}

// addThreads takes the active threads of a channel into its plan: a known thread gets its fresh
// object, a new one is added (selected when its channel is) in ID order.
func (m *mirror) addThreads(c *channelPlan) {
	p := m.wk.p
	added := false
	for _, a := range m.wk.gs.activeBy[c.ch.ID] {
		if p.threadOf[a.ID] != nil {
			for _, t := range c.threads {
				if t.th.ID == a.ID {
					t.th = a
				}
			}
			continue
		}
		c.threads = append(c.threads, &threadPlan{th: a, selected: c.selected})
		p.threadOf[a.ID] = c
		added = true
	}
	if added {
		slices.SortFunc(c.threads, func(a, b *threadPlan) int { return compareIDs(a.th.ID, b.th.ID) })
	}
}

// mirrorThread mirrors one thread: its done state by the archive rule of 22 5.1.1 (a thread the
// cursor knows as open that is missing from the active list is confirmed with GET /channels/{id}),
// and its new messages. The thread item is posted when the thread is new, when Discord shows its
// name or done state differently from the last posted fingerprint, or with new messages.
func (m *mirror) mirrorThread(ctx context.Context, c *channelPlan, t *threadPlan) error {
	wk := m.wk
	active := wk.gs.active[t.th.ID]
	stored, known := m.cur.mirrored(t.th.ID)
	if !known && m.starterPending(c, t) {
		return nil
	}
	wasOpen := !t.closed
	if known {
		wasOpen = strings.HasPrefix(stored, "o:")
	}
	if !active && wasOpen {
		ch, err := wk.dc.Channel(ctx, t.th.ID)
		if err != nil {
			return err
		}
		t.th = ch
	}
	t.closed, t.closedAt = doneRule(t.th, active, c.ch, wk.cfg.DiscordClosedTags)
	if t.closed {
		c.rep.ThreadsDone++
	} else {
		c.rep.ThreadsOpen++
	}
	changed := !known || stored != threadFingerprint(t.th.Name, t.closed)
	if !changed && !newer(t.th.LastMessageID, m.cur.position(false, t.th.ID)) {
		return nil
	}
	return m.thread(ctx, c, t)
}

// starterPending reports a thread of a text channel that must wait for its starter message: the
// main thread is in scope and still has messages from the thread's ID on that this round did not
// copy (the settle rule held them, or the read failed). The server links a new thread to its
// starter only when the starter is posted first (22 5.1.1), and a re-post never adds the link, so
// such a thread is posted in the round that copies its starter.
func (m *mirror) starterPending(c *channelPlan, t *threadPlan) bool {
	if c.kind != kindText || !c.selected || m.wk.w == nil {
		// A dry round copies nothing, so it counts the thread as an --apply round after the
		// starter would.
		return false
	}
	main := m.wk.w.position(true, c.ch.ID)
	return compareIDs(t.th.ID, main) > 0 && newer(c.ch.LastMessageID, main)
}

// newer reports a message ID after the cursor position.
func newer(last, position string) bool {
	return last != "" && compareIDs(last, position) > 0
}

// thread copies the settled beginning of the new messages of a main thread (t nil) or thread
// (brief design note 2 d): pages from the cursor, the settle rule with the lookback before the
// first new message, the copy through the walker (people, attachments, conversion, batches,
// cursor after each batch); the first message held back ends the reading.
func (m *mirror) thread(ctx context.Context, c *channelPlan, t *threadPlan) error {
	wk := m.wk
	pd := &pending{threadKey: c.key(), readID: c.ch.ID, main: true}
	if t != nil {
		o, err := wk.threadObject(c, t)
		if err != nil {
			return err
		}
		pd = &pending{threadKey: t.key(), readID: t.th.ID, threadObj: &o, items: 1, fp: threadFingerprint(t.th.Name, t.closed)}
	}
	after := m.cur.position(pd.main, pd.readID)
	pd.consumed = after
	if err := m.read(ctx, c, pd, after); err != nil {
		return err
	}
	return wk.flush(ctx, c, pd, true)
}

// read reads the new messages of a thread page by page from after and hands each page to page,
// until a page is empty or the settle rule held a message back.
func (m *mirror) read(ctx context.Context, c *channelPlan, pd *pending, after string) error {
	for first := true; ; first = false {
		msgs, err := m.wk.dc.MessagesAfter(ctx, pd.readID, after, m.wk.cfg.DiscordPageSize)
		if err != nil || len(msgs) == 0 {
			return err
		}
		var lookback []discord.Message
		if first && after != "0" {
			if lookback, err = m.lookback(ctx, pd.readID, msgs[0].ID); err != nil {
				return err
			}
		}
		if held, err := m.page(ctx, c, pd, msgs, lookback); err != nil || held {
			return err
		}
		after = msgs[len(msgs)-1].ID
	}
}

// page applies the settle rule to one page of new messages and hands the copy to the walker; held
// is true when the rule held a message back, which ends the reading of the thread for this round.
func (m *mirror) page(ctx context.Context, c *channelPlan, pd *pending, page, lookback []discord.Message) (bool, error) {
	if err := m.loadStarters(ctx, c, slices.Concat(lookback, page)); err != nil {
		return false, err
	}
	cfg := SettleConfig{BusyReactions: m.wk.cfg.FollowBusyReactions, SettleAfter: m.wk.cfg.FollowSettleAfter}
	cp, held, reason := Settle(m.env.clock().Now(), page, lookback, func(id string) *discord.Message { return m.starters[id] }, cfg)
	for _, msg := range cp {
		if err := m.wk.consume(ctx, c, pd, msg); err != nil {
			return false, err
		}
	}
	if held == 0 {
		return false, nil
	}
	c.rep.MessagesHeld += held
	m.wk.log.DebugContext(ctx, "mirror: thread held back", slog.String(logKeyReason, string(reason)), slog.Int(logKeyHeld, held))
	return true, nil
}

// lookback reads the import.follow.busy_lookback messages before first (22 12.3 item 4), in pages
// of at most import.discord.page_size.
func (m *mirror) lookback(ctx context.Context, readID, first string) ([]discord.Message, error) {
	want := m.wk.cfg.FollowBusyLookback
	var out []discord.Message
	for before := first; len(out) < want; {
		n := min(want-len(out), m.wk.cfg.DiscordPageSize)
		page, err := m.wk.dc.MessagesBefore(ctx, readID, before, n)
		if err != nil {
			return nil, err
		}
		out = append(page, out...)
		if len(page) < n {
			break
		}
		before = page[0].ID
	}
	return out, nil
}

// loadStarters reads the starter message of every type 21 message of msgs once per round, for the
// busy rule of 22 12.3 item 1; a starter that is not readable (403, 404) counts as not busy.
func (m *mirror) loadStarters(ctx context.Context, c *channelPlan, msgs []discord.Message) error {
	for _, msg := range msgs {
		ref := msg.MessageReference
		if msg.Type != msgThreadStarter || ref == nil || ref.MessageID == "" {
			continue
		}
		if _, ok := m.starters[ref.MessageID]; ok {
			continue
		}
		channel := ref.ChannelID
		if channel == "" {
			channel = c.ch.ID
		}
		s, err := m.starter(ctx, channel, ref.MessageID)
		if err != nil {
			return err
		}
		m.starters[ref.MessageID] = s
	}
	return nil
}

// starter reads one message (GET around its ID); nil when it is not readable or gone.
func (m *mirror) starter(ctx context.Context, channel, id string) (*discord.Message, error) {
	around, err := m.wk.dc.MessagesAround(ctx, channel, id, 1)
	if errors.Is(err, discord.ErrForbidden) || errors.Is(err, discord.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	for i := range around {
		if around[i].ID == id {
			return &around[i], nil
		}
	}
	return nil, nil
}

// print writes the report of the round: always in a dry mirror and with --once, with --follow
// only when the round copied, held or noted something.
func (m *mirror) print() {
	rep := newReport(m.o.apply, m.wk.gs, m.wk.p)
	rep.mirror = true
	rep.notes = slices.Concat(rep.notes, m.notes)
	if m.o.apply && m.o.follow && !m.activity(rep) {
		return
	}
	var err error
	if m.o.report == reportJSON {
		err = rep.writeJSON(m.env.Stdout)
	} else {
		err = rep.writeText(m.env.Stdout)
	}
	if err != nil {
		m.wk.log.DebugContext(context.Background(), "mirror: report not written", slog.String(logKeyError, err.Error()))
	}
}

// activity reports a round that copied, held or failed a message or noted something; the notes of
// the start of the run alone are no activity.
func (m *mirror) activity(rep *report) bool {
	t := rep.total()
	return len(m.notes) > 0 || len(t.notes) > 0 || t.MessagesTaken+t.MessagesHeld+t.MessagesFailed+t.SourceChanged > 0
}
