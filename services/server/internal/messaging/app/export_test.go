// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/messaging/app"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// exportStore serves one thread of the channel to the export statements.
type exportStore struct {
	bareStore
	thread ids.ID
	rows   []domain.Message
}

func (s exportStore) ExportThreads(_ context.Context, _ app.ExportRead, after ids.ID) ([]ids.ID, error) {
	if after == s.thread {
		return nil, nil
	}
	return []ids.ID{s.thread}, nil
}

func (s exportStore) ExportMessages(_ context.Context, q app.ExportRead, c domain.ExportCursor) ([]domain.Message, error) {
	var out []domain.Message
	for _, m := range s.rows {
		if m.ThreadSeq > c.ThreadSeq && len(out) < q.Limit {
			out = append(out, m)
		}
	}
	return out, nil
}

func (exportStore) Mentions(context.Context, ids.ID) ([]ids.ID, error) { return nil, nil }

// runUoW runs the function of a unit of work without a transaction.
type runUoW struct{ txn.UnitOfWork }

func (runUoW) Do(ctx context.Context, fn func(context.Context) error, _ ...txn.Option) error {
	return fn(ctx)
}

// entries records the audit entries of Record.
type entries struct {
	auditing.Recorder
	got []auditing.Entry
}

func (e *entries) Record(_ context.Context, en auditing.Entry) error {
	e.got = append(e.got, en)
	return nil
}

// credentials answers Revalidate with err.
type credentials struct{ err error }

func (c *credentials) Revalidate(context.Context, actor.Actor) error { return c.err }

// exportService builds the use cases over the fakes with page size pageSize.
func exportService(t *testing.T, store app.Store, az authorize.Authorizer, rec auditing.Recorder, cred app.CredentialCheck,
	logs *httpservertest.LogCapture, cnt *counter, pageSize int) *app.Service {
	t.Helper()
	svc, err := app.New(app.Config{MaxContentBytes: 100, MaxMentions: 1, EventPayloadMaxBytes: 100, TemplateNameMaxChars: 10,
		TemplateMaxPerScope: 5, ListContentMaxBytes: 4, PageSizeDefault: 1, PageSizeMax: pageSize, EmojiMaxBytes: 64,
		MaxDistinctReactions: 20, MaxPinsPerThread: 50},
		app.Deps{Store: store, UoW: runUoW{}, Authorizer: az, Events: noEvents{}, Threads: oneThread{info: chat.ThreadInfo{}},
			Directory: anyone{}, Log: slog.New(logs), Errors: cnt, Cursors: noCursors{}, Redactor: struct{ eventlog.Redactor }{},
			Audit: rec, Symbols: anySymbol{}, Credentials: cred})
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

// exportRows returns n messages of thread in channel ch with thread_seq 1 to n.
func exportRows(thread, ch ids.ID, n int) []domain.Message {
	var out []domain.Message
	for seq := int64(1); seq <= int64(n); seq++ {
		out = append(out, domain.Message{ID: ids.New(), ThreadID: thread, ChannelID: ch, ThreadSeq: seq, AuthorID: ids.New(),
			Kind: domain.KindText, Origin: domain.OriginHuman, ContentMD: "private words of the export", Version: 1})
	}
	return out
}

// drain reads st to its end and returns the thread_seq of every message.
func drain(t *testing.T, st messaging.ExportStream) ([]int64, error) {
	t.Helper()
	var seqs []int64
	for {
		batch, err := st.Next(context.Background())
		if errors.Is(err, io.EOF) {
			return seqs, nil
		}
		if err != nil {
			return seqs, err
		}
		if len(batch) == 0 {
			t.Fatal("empty batch")
		}
		for _, m := range batch {
			seqs = append(seqs, m.ThreadSeq)
		}
	}
}

// TestExportStream_FilterMismatchDropsLine (brief M3-11 note 7d, 07 section 7.4 rule 3): a row the
// visible predicate let through and message.read refuses is missing from its batch, logged at
// error level with only action and reason (no id, no content) and counted once as filter_mismatch;
// every row check runs in filter mode; the content of the others is whole.
func TestExportStream_FilterMismatchDropsLine(t *testing.T) {
	t.Parallel()
	thread, ch := ids.New(), ids.New()
	rows := exportRows(thread, ch, 3)
	az := &mismatchAuthz{deny: rows[1].ID, filtered: true}
	logs, cnt, rec := &httpservertest.LogCapture{}, &counter{}, &entries{}
	svc := exportService(t, exportStore{thread: thread, rows: rows}, az, rec, &credentials{}, logs, cnt, 10)
	from := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	st, err := svc.Export().Channel(context.Background(), messaging.ChannelExportQuery{ChannelID: ch, From: from, To: from.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	seqs, err := drain(t, st)
	if err != nil || len(seqs) != 2 || seqs[0] != 1 || seqs[1] != 3 {
		t.Fatalf("seqs %v, %v", seqs, err)
	}
	if !az.filtered {
		t.Error("message.read ran outside filter mode")
	}
	if len(cnt.reasons) != 1 || cnt.reasons[0] != "filter_mismatch" {
		t.Errorf("counted %v", cnt.reasons)
	}
	recs := logs.Records()
	if len(recs) != 1 || recs[0].Level != slog.LevelError || recs[0].Attrs["bothy.action"] != "message.read" || recs[0].Attrs["bothy.reason"] != "filter_mismatch" {
		t.Fatalf("records %+v", recs)
	}
	for _, v := range recs[0].Attrs {
		if v == rows[1].ID.String() || v == thread.String() || v == "private words of the export" {
			t.Errorf("record carries %q", v)
		}
	}
	if len(rec.got) != 1 || rec.got[0].Action != "export.channel" || rec.got[0].Decision != auditing.DecisionAllow || rec.got[0].ResourceID != ch ||
		len(rec.got[0].Attrs) != 2 {
		t.Errorf("audit %+v", rec.got)
	}
}

// TestExportStream_CredentialsRevokedAborts (D-M3-11-9): a session revoked after the first batch
// raises no rights version, so the credential check before the next batch ends the stream with
// ErrExportAborted and no message; the content of the first batch was whole.
func TestExportStream_CredentialsRevokedAborts(t *testing.T) {
	t.Parallel()
	thread, ch := ids.New(), ids.New()
	cred := &credentials{}
	svc := exportService(t, exportStore{thread: thread, rows: exportRows(thread, ch, 4)}, &mismatchAuthz{}, &entries{}, cred,
		&httpservertest.LogCapture{}, &counter{}, 2)
	from := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	st, err := svc.Export().Channel(context.Background(), messaging.ChannelExportQuery{ChannelID: ch, From: from, To: from.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	batch, err := st.Next(context.Background())
	if err != nil || len(batch) != 2 || batch[0].ContentMD != "private words of the export" || batch[0].ContentTruncated {
		t.Fatalf("first batch %+v, %v", batch, err)
	}
	cred.err = errs.New(errs.CodeSessionRevoked, "")
	batch, err = st.Next(context.Background())
	if !errors.Is(err, messaging.ErrExportAborted) || len(batch) != 0 {
		t.Fatalf("second batch %+v, %v", batch, err)
	}
}

// TestExportChannel_RangeBeforeAnyCheck (brief M3-11 note 3c): an empty range is refused before
// export.channel runs, so it cannot reveal anything about the channel; no audit entry is written.
func TestExportChannel_RangeBeforeAnyCheck(t *testing.T) {
	t.Parallel()
	az := &decideAuthz{}
	rec := &entries{}
	svc := exportService(t, exportStore{}, az, rec, &credentials{}, &httpservertest.LogCapture{}, &counter{}, 2)
	at := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	_, err := svc.Export().Channel(context.Background(), messaging.ChannelExportQuery{ChannelID: ids.New(), From: at, To: at})
	if e, ok := errs.As(err); !ok || e.Code != errs.CodeValidationFailed {
		t.Fatalf("err = %v", err)
	}
	if len(rec.got) != 0 {
		t.Errorf("audit %+v", rec.got)
	}
}
