// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/messaging/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	mw "github.com/OWNER/bothy/services/server/test/messagingworld"
)

// TestRepository_PushReads (M3-19 design notes 5 and 9 d): the four reads of the consumer push
// over one thread with two run anchors, agent messages of every kind, a message of another agent,
// a tombstone, a system line and a human message:
//
//	1 human text     2 anchor of run one (lotse)   3 lotse text   4 lotse voice   5 other text
//	6 lotse card (deleted)   7 system line   8 anchor of run two (lotse)   9 lotse text
//	10 human text (deleted)   11 system line
//
// LastAnchorBefore and AnchorOf find anchors only (kind run), strictly below the bound;
// LastAgentMessageBefore keeps only the author's text, voice and card messages of origin agent
// strictly between the bounds that are not deleted; Newest skips tombstones, anchors and system
// lines. A thread without such rows answers found false, never an error.
func TestRepository_PushReads(t *testing.T) {
	t.Parallel()
	w := mw.New(t, func(reg *txn.Registry, rec *mw.Recorder) error { return txn.Subscribe(reg, posted{rec}.onPosted) })
	space, ch, main := w.TextChannel()
	otherCh := w.Channel(space, "text", "elsewhere")
	otherMain := w.MainThread(otherCh)
	lotse, other := w.Agent("lotse"), w.Agent("other")
	// At most one run is active per thread (R-DM-14): run one has ended when run two starts.
	runOne := seedRun(w, main, lotse)
	w.SQL(`UPDATE runs SET state = 'completed', finished_at = now() WHERE id = $1`, runOne.String())
	runTwo := seedRun(w, main, lotse)
	repo := postgres.NewRepository(w.Pool)
	reg := txn.NewRegistry()
	reg.Seal()
	uow := pg.NewUnitOfWork(w.Pool, pg.UnitConfig{StatementTimeout: 30 * time.Second, LockTimeout: 10 * time.Second, IdleInTxTimeout: time.Minute}, reg)
	ctx := context.Background()
	at := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	byID := map[int64]ids.ID{}
	row := func(thread, channel ids.ID, seq int64, author ids.ID, origin domain.Origin, kind domain.Kind, run ids.ID) domain.Message {
		m := domain.Message{
			ID: ids.New(), ThreadID: thread, ChannelID: channel, ThreadSeq: seq, AuthorID: author, ClientMsgID: ids.New(),
			PayloadSHA256: []byte{1}, Origin: origin, Kind: kind, ContentMD: "content", ContentFormat: domain.ContentFormat,
			RunID: run, Version: 1, CreatedAt: at, SearchLanguage: "simple",
		}
		switch kind {
		case domain.KindSystem:
			m.SystemCode, m.ContentMD, m.Silent = domain.SystemThreadReopened, "", true
		case domain.KindRun:
			m.ContentMD, m.Silent = "", true
		}
		return m
	}
	rows := []domain.Message{
		row(main, ch, 1, w.OwnerID, domain.OriginHuman, domain.KindText, ids.ID{}),
		row(main, ch, 2, lotse, domain.OriginAgent, domain.KindRun, runOne),
		row(main, ch, 3, lotse, domain.OriginAgent, domain.KindText, ids.ID{}),
		row(main, ch, 4, lotse, domain.OriginAgent, domain.KindVoice, ids.ID{}),
		row(main, ch, 5, other, domain.OriginAgent, domain.KindText, ids.ID{}),
		row(main, ch, 6, lotse, domain.OriginAgent, domain.KindCard, ids.ID{}),
		row(main, ch, 7, w.OwnerID, domain.OriginSystem, domain.KindSystem, ids.ID{}),
		row(main, ch, 8, lotse, domain.OriginAgent, domain.KindRun, runTwo),
		row(main, ch, 9, lotse, domain.OriginAgent, domain.KindText, ids.ID{}),
		row(main, ch, 10, w.OwnerID, domain.OriginHuman, domain.KindText, ids.ID{}),
		row(main, ch, 11, w.OwnerID, domain.OriginSystem, domain.KindSystem, ids.ID{}),
		// The other thread holds only a system line, so it has no newest message.
		row(otherMain, otherCh, 1, w.OwnerID, domain.OriginSystem, domain.KindSystem, ids.ID{}),
	}
	if err := uow.Do(ctx, func(ctx context.Context) error {
		for _, m := range rows {
			if _, err := repo.InsertMessage(ctx, m); err != nil {
				return err
			}
			if m.ThreadID == main {
				byID[m.ThreadSeq] = m.ID
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	w.Tombstone(byID[6])
	w.Tombstone(byID[10])
	anchorOne := messaging.AnchorRef{MessageID: byID[2], ThreadSeq: 2, RunID: runOne}
	anchorTwo := messaging.AnchorRef{MessageID: byID[8], ThreadSeq: 8, RunID: runTwo}

	for _, c := range []struct {
		name   string
		thread ids.ID
		before int64
		want   messaging.AnchorRef
		found  bool
	}{
		{"above every sequence", main, 1 << 53, anchorTwo, true},
		{"just above the second anchor", main, 9, anchorTwo, true},
		{"the second anchor itself is not below", main, 8, anchorOne, true},
		{"just above the first anchor", main, 3, anchorOne, true},
		{"the first anchor itself is not below", main, 2, messaging.AnchorRef{}, false},
		{"a thread without anchors", otherMain, 1 << 53, messaging.AnchorRef{}, false},
	} {
		got, found, err := repo.LastAnchorBefore(ctx, c.thread, c.before)
		if err != nil || found != c.found || got != c.want {
			t.Errorf("LastAnchorBefore %s: %+v %v %v, want %+v %v", c.name, got, found, err, c.want, c.found)
		}
	}

	if got, found, err := repo.AnchorOf(ctx, runOne); err != nil || !found || got != anchorOne {
		t.Errorf("AnchorOf run one: %+v %v %v", got, found, err)
	}
	if got, found, err := repo.AnchorOf(ctx, runTwo); err != nil || !found || got != anchorTwo {
		t.Errorf("AnchorOf run two: %+v %v %v", got, found, err)
	}
	if got, found, err := repo.AnchorOf(ctx, ids.New()); err != nil || found || got != (messaging.AnchorRef{}) {
		t.Errorf("AnchorOf an unknown run: %+v %v %v", got, found, err)
	}

	ref := func(seq int64) messaging.MessageRef { return messaging.MessageRef{ID: byID[seq], ThreadSeq: seq} }
	for _, c := range []struct {
		name          string
		thread, agent ids.ID
		after, before int64
		want          messaging.MessageRef
		found         bool
	}{
		{"newest of the agent after the first anchor", main, lotse, 2, 1 << 53, ref(9), true},
		{"skips the anchor, the system line, the tombstone and the other agent", main, lotse, 2, 8, ref(4), true},
		{"a voice message counts", main, lotse, 2, 5, ref(4), true},
		{"the bounds are exclusive", main, lotse, 2, 4, ref(3), true},
		{"nothing between the anchor and the bound", main, lotse, 2, 3, messaging.MessageRef{}, false},
		{"nothing between the second anchor and its answer", main, lotse, 8, 9, messaging.MessageRef{}, false},
		{"the other agent", main, other, 0, 1 << 53, ref(5), true},
		{"a human is no agent author", main, w.OwnerID, 0, 1 << 53, messaging.MessageRef{}, false},
		{"another thread", otherMain, lotse, 0, 1 << 53, messaging.MessageRef{}, false},
	} {
		got, found, err := repo.LastAgentMessageBefore(ctx, c.thread, c.agent, c.after, c.before)
		if err != nil || found != c.found || got != c.want {
			t.Errorf("LastAgentMessageBefore %s: %+v %v %v, want %+v %v", c.name, got, found, err, c.want, c.found)
		}
	}

	if got, found, err := repo.Newest(ctx, main); err != nil || !found || got != ref(9) {
		t.Errorf("Newest: %+v %v %v, want %+v", got, found, err, ref(9))
	}
	if got, found, err := repo.Newest(ctx, otherMain); err != nil || found || got != (messaging.MessageRef{}) {
		t.Errorf("Newest of a thread with only a system line: %+v %v %v", got, found, err)
	}
	if got, found, err := repo.Newest(ctx, ids.New()); err != nil || found {
		t.Errorf("Newest of an unknown thread: %+v %v %v", got, found, err)
	}
}

// seedRun writes a committed queued run of agent in thread, so the deferred foreign key
// messages.run_id finds it, and returns its id (M2-05 migration 00053).
func seedRun(w *mw.World, thread, agent ids.ID) ids.ID {
	id := ids.New()
	w.SQL(`INSERT INTO runs (id, thread_id, agent_id, trigger_kind, triggered_by, billing_mode_expected, invocation_jti)
		VALUES ($1, $2, $3, 'message', $4, 'organization', $5)`, id.String(), thread.String(), agent.String(), w.OwnerID.String(), ids.New().String())
	return id
}
