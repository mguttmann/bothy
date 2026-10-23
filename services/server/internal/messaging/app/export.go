// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"io"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
)

// The audit details of export.channel (19 SEC-AUD-4 form, brief M3-11 note 6).
const (
	attrExportFrom = "from"
	attrExportTo   = "to"
)

// ExportChannel implements messaging.Export over the use cases of Service (exportChannel, brief
// M3-11). It is safe for concurrent use; each stream it returns is not.
type ExportChannel struct{ *Service }

// Export returns the incoming port of the channel export.
func (s *Service) Export() ExportChannel { return ExportChannel{s} }

// Compile-time check: the export port.
var _ messaging.Export = ExportChannel{}

// Channel implements messaging.Export in the order of brief M3-11 note 3: the range rule without
// any lookup; export.channel on the channel in request mode (authz writes and aggregates a denial;
// a missing and an invisible channel take the same path); the visible channels of message.read; the
// allow entry with from and to, committed in its own unit of work before the stream exists. On any
// error no stream is returned, so nothing reaches the client.
func (e ExportChannel) Channel(ctx context.Context, q messaging.ChannelExportQuery) (messaging.ExportStream, error) {
	if err := domain.ValidateExportRange(q.From, q.To); err != nil {
		return nil, err
	}
	if e.d.Credentials == nil {
		return nil, errs.New(errs.CodeInternalError, "messaging: the export needs the credential check")
	}
	a := requestPrincipal(ctx)
	if err := e.check(ctx, a, authorize.ActionExportChannel, exportResource(q.ChannelID)); err != nil {
		return nil, err
	}
	if _, err := e.d.Authorizer.Scope(ctx, a, authorize.ActionMessageRead); err != nil {
		return nil, err
	}
	err := e.d.UoW.Do(ctx, func(ctx context.Context) error {
		return e.d.Audit.Record(ctx, auditing.Entry{
			Action: string(authorize.ActionExportChannel), Decision: auditing.DecisionAllow,
			ResourceKind: string(authorize.ResourceChannel), ResourceID: q.ChannelID,
			Attrs: auditing.Attrs{attrExportFrom: auditing.Time(q.From), attrExportTo: auditing.Time(q.To)},
		})
	})
	if err != nil {
		return nil, err
	}
	return &exportStream{
		s: e.Service, a: a, channel: q.ChannelID, walk: domain.NewExportWalk(e.cfg.PageSizeMax),
		sel: ExportRead{ChannelID: q.ChannelID, From: q.From, To: q.To, Limit: e.cfg.PageSizeMax},
	}, nil
}

// exportResource describes the channel id for export.channel and channel.read.
func exportResource(id ids.ID) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceChannel, ID: id, ChannelID: id}
}

// exportStream is the stream of one export: the walk over thread and message pages of the
// selection (domain.ExportWalk), re-checked before every read (brief M3-11 note 7).
type exportStream struct {
	s       *Service
	a       actor.Actor
	channel ids.ID
	sel     ExportRead
	walk    *domain.ExportWalk
	done    bool
}

// Next implements messaging.ExportStream: it reads pages until one yields a message or the walk
// ends (io.EOF). Before every page it starts a fresh rights memo, checks the credentials and
// channel.read in filter mode and takes the visible channels of message.read at that moment;
// a refusal ends the stream with an error wrapping messaging.ErrExportAborted. Every message
// passes message.read in filter mode; a refused one is dropped and counted as filter_mismatch.
func (st *exportStream) Next(ctx context.Context) ([]messaging.Message, error) {
	for !st.done {
		step := st.walk.Next()
		if step == domain.ExportReadDone {
			st.done = true
			break
		}
		bctx, visible, err := st.recheck(ctx)
		if err != nil {
			st.done = true
			return nil, err
		}
		sel := st.sel
		sel.Visible = visible
		if step == domain.ExportReadThreads {
			page, err := st.s.d.Store.ExportThreads(bctx, sel, st.walk.AfterThread())
			if err != nil {
				st.done = true
				return nil, err
			}
			st.walk.Threads(page)
			continue
		}
		batch, err := st.batch(bctx, sel)
		if err != nil {
			st.done = true
			return nil, err
		}
		if len(batch) > 0 {
			return batch, nil
		}
	}
	return nil, io.EOF
}

// recheck derives the context of one batch with a fresh request memo, so the batch uses the rights
// version current at this moment (07 section 8.2 item 2, 04 section 8.1), and checks the
// principal again: its credentials (a revoked session or device raises no rights version) and
// channel.read in filter mode (no denial audit). It returns the context and the visible channels
// of message.read for the statements. Errors: ErrExportAborted (wrapped) for a refusal; the
// error of a source.
func (st *exportStream) recheck(ctx context.Context) (context.Context, []ids.ID, error) {
	bctx := authorize.WithRequestScope(authorize.WithoutRequestScope(ctx))
	if err := st.s.d.Credentials.Revalidate(bctx, st.a); err != nil {
		if _, ok := errs.As(err); !ok {
			return nil, nil, err
		}
		return nil, nil, fmt.Errorf("%w: credentials: %w", messaging.ErrExportAborted, err)
	}
	fctx := authorize.Filter(bctx)
	ok, err := st.s.allowed(fctx, st.a, authorize.ActionChannelRead, exportResource(st.channel))
	if err != nil {
		return nil, nil, err
	}
	if !ok {
		return nil, nil, fmt.Errorf("%w: channel.read refused", messaging.ErrExportAborted)
	}
	scope, err := st.s.d.Authorizer.Scope(bctx, st.a, authorize.ActionMessageRead)
	if err != nil {
		return nil, nil, err
	}
	return bctx, scope.ChannelIDs, nil
}

// batch reads one message page of the walk in a short unit of work and builds its
// representations there, so the unit of work ends before the first line is written (02 section
// 8.5). Each row passes message.read in filter mode on the memo of the batch; content stays whole
// (D-M3-11-2) and a tombstone keeps its thread_seq.
func (st *exportStream) batch(ctx context.Context, sel ExportRead) ([]messaging.Message, error) {
	var out []messaging.Message
	var seqs []int64
	err := st.s.d.UoW.Do(ctx, func(ctx context.Context) error {
		out, seqs = nil, nil
		rows, err := st.s.d.Store.ExportMessages(ctx, sel, st.walk.Cursor())
		if err != nil {
			return err
		}
		authors := map[ids.ID]messaging.PrincipalRef{}
		fctx := authorize.Filter(ctx)
		for _, m := range rows {
			seqs = append(seqs, m.ThreadSeq)
			ok, err := st.s.allowed(fctx, st.a, authorize.ActionMessageRead, historyResource(m))
			if err != nil {
				return err
			}
			if !ok {
				st.s.historyFilterMismatch(ctx)
				continue
			}
			v, err := st.s.exported(ctx, m, authors)
			if err != nil {
				return err
			}
			out = append(out, v)
		}
		return st.s.decorate(ctx, sel.Visible, out)
	})
	if err != nil {
		return nil, err
	}
	// The walk moves only after the unit of work committed, so a retried one reads the same page.
	st.walk.Messages(seqs)
	return out, nil
}

// exported returns the export representation of m: the representation of Get with the whole
// content (brief M3-11 note 4). authors caches the authors of one batch.
func (s *Service) exported(ctx context.Context, m domain.Message, authors map[ids.ID]messaging.PrincipalRef) (messaging.Message, error) {
	author, ok := authors[m.AuthorID]
	if !ok {
		var err error
		if author, err = s.author(ctx, m.AuthorID); err != nil {
			return messaging.Message{}, err
		}
		authors[m.AuthorID] = author
	}
	var mentions []ids.ID
	if m.DeletedAt.IsZero() {
		var err error
		if mentions, err = s.d.Store.Mentions(ctx, m.ID); err != nil {
			return messaging.Message{}, err
		}
	}
	return view(m, author, mentions), nil
}
