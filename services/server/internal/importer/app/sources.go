// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/importer"
	"github.com/OWNER/bothy/services/server/internal/importer/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/textcheck"
)

// Names of the opaque list cursors.
const (
	sourcesList = "imports"
	itemsList   = "import_items"
)

// limit returns the page size of a request: 0 is api.page_size_default, larger values are capped
// at api.page_size_max.
func (s *Service) limit(n int) int {
	switch {
	case n <= 0:
		return s.cfg.PageSizeDefault
	case n > s.cfg.PageSizeMax:
		return s.cfg.PageSizeMax
	}
	return n
}

// sourceCursor is the keyset position of listImports.
type sourceCursor struct {
	At string `json:"at"`
	ID string `json:"id"`
}

// ListSources implements importer.Imports: import.read on the collection, newest first.
func (s *Service) ListSources(ctx context.Context, a actor.Actor, p importer.Page) (importer.SourcePage, error) {
	if err := s.check(ctx, a, authorize.ActionImportRead, ids.ID{}); err != nil {
		return importer.SourcePage{}, err
	}
	var after *SourceKey
	if p.Cursor != "" {
		var k sourceCursor
		if err := s.d.Cursors.Decode(sourcesList, nil, p.Cursor, &k); err != nil {
			return importer.SourcePage{}, err
		}
		at, errT := parseStamp(k.At)
		id, errID := ids.Parse(k.ID)
		if errT != nil || errID != nil {
			return importer.SourcePage{}, errs.New(errs.CodeCursorInvalid, "")
		}
		after = &SourceKey{At: at, ID: id}
	}
	n := s.limit(p.Limit)
	rows, err := s.d.Store.Sources(ctx, after, n+1)
	if err != nil {
		return importer.SourcePage{}, err
	}
	out := importer.SourcePage{Items: rows}
	if len(rows) > n {
		out.Items = rows[:n]
		last := out.Items[n-1]
		if out.NextCursor, err = s.d.Cursors.Encode(sourcesList, nil, sourceCursor{At: last.CreatedAt.Format(cursorTime), ID: last.ID.String()}); err != nil {
			return importer.SourcePage{}, err
		}
	}
	return out, nil
}

// cursorTime keeps the microseconds of created_at in a cursor.
const cursorTime = "2006-01-02T15:04:05.000000Z07:00"

// parseStamp reads a cursor time.
func parseStamp(s string) (time.Time, error) { return time.Parse(cursorTime, s) }

// GetSource implements importer.Imports (import.read).
func (s *Service) GetSource(ctx context.Context, a actor.Actor, id ids.ID) (importer.Source, error) {
	return s.load(ctx, a, authorize.ActionImportRead, id, false)
}

// itemCursor is the keyset position of listImportItems.
type itemCursor struct {
	ID string `json:"id"`
}

// ListItems implements importer.Imports (import.read): ascending id, optionally one state; the
// cursor is bound to the source and the state filter.
func (s *Service) ListItems(ctx context.Context, a actor.Actor, id ids.ID, q importer.ItemQuery) (importer.ItemPage, error) {
	if _, err := s.load(ctx, a, authorize.ActionImportRead, id, false); err != nil {
		return importer.ItemPage{}, err
	}
	binding := []byte(id.String() + "|" + q.State)
	var after ids.ID
	if q.Cursor != "" {
		var k itemCursor
		if err := s.d.Cursors.Decode(itemsList, binding, q.Cursor, &k); err != nil {
			return importer.ItemPage{}, err
		}
		var err error
		if after, err = ids.Parse(k.ID); err != nil {
			return importer.ItemPage{}, errs.New(errs.CodeCursorInvalid, "")
		}
	}
	n := s.limit(q.Limit)
	rows, err := s.d.Store.Items(ctx, id, q.State, after, n+1)
	if err != nil {
		return importer.ItemPage{}, err
	}
	var out importer.ItemPage
	if len(rows) > n {
		rows = rows[:n]
		if out.NextCursor, err = s.d.Cursors.Encode(itemsList, binding, itemCursor{ID: rows[n-1].ID.String()}); err != nil {
			return importer.ItemPage{}, err
		}
	}
	out.Items = make([]importer.Item, 0, len(rows))
	for _, r := range rows {
		out.Items = append(out.Items, itemDTO(r, false))
	}
	return out, nil
}

// itemDTO converts an item row into the port DTO.
func itemDTO(r ItemRow, changed bool) importer.Item {
	return importer.Item{
		ID: r.ID, OriginKey: r.OriginKey, ItemKind: r.ItemKind, TargetKind: r.TargetKind, TargetID: r.TargetID, State: r.State,
		Error: r.Error, SourceChanged: changed, CreatedAt: r.CreatedAt,
	}
}

// CreateSource implements importer.Imports (import.manage, step-up, audit; M8-03 design note 2):
// the kind discord with a mapping of import_source.discord.v1 validated field by field, every
// handle and existing object looked up, all violations reported at once; the normalized mapping
// is stored in a draft source with import.progress.
func (s *Service) CreateSource(ctx context.Context, a actor.Actor, cmd importer.CreateSourceCmd) (importer.Source, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return importer.Source{}, err
	}
	var out importer.Source
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		if err := s.check(ctx, a, authorize.ActionImportManage, ids.ID{}); err != nil {
			return err
		}
		config, err := s.mapping(ctx, cmd)
		if err != nil {
			return err
		}
		if out, err = s.d.Store.InsertSource(ctx, importer.Source{
			ID: ids.New(), Kind: importer.KindDiscord, Name: strings.TrimSpace(cmd.Name), Config: config, CreatedBy: a.ID, CreatedAt: s.now(),
		}); err != nil {
			return err
		}
		if err := s.record(ctx, out.ID, auditing.Attrs{attrOp: auditing.Code("create")}); err != nil {
			return err
		}
		return s.changed(ctx, out)
	})
	return out, err
}

// mapping validates kind, name and config of cmd and returns the normalized mapping.
func (s *Service) mapping(ctx context.Context, cmd importer.CreateSourceCmd) (json.RawMessage, error) {
	var p domain.Problems
	if cmd.Kind == importer.KindArchive {
		p.Add("/kind", domain.FieldNotEnabled, "the source kind archive is not enabled")
		return nil, p.Err()
	}
	if strings.TrimSpace(cmd.Name) == "" {
		p.Add("/name", domain.FieldRequired, "the name is required")
	} else if textcheck.DisplayText(cmd.Name) != nil {
		p.Add("/name", domain.FieldInvalid, "the name contains control or bidi characters")
	}
	m, refs := domain.ParseMapping(cmd.Config, domain.Limits{SpaceName: s.cfg.SpaceNameMaxChars, ChannelName: s.cfg.ChannelNameMaxChars}, &p)
	for _, r := range refs {
		if err := s.resolveRef(ctx, r, &p); err != nil {
			return nil, err
		}
	}
	if err := p.Err(); err != nil {
		return nil, err
	}
	return m.JSON()
}

// resolveRef looks up one reference of the mapping and records its violation in p; the error is
// a failure of a port, not a violation.
func (s *Service) resolveRef(ctx context.Context, r domain.Ref, p *domain.Problems) error {
	switch r.Kind {
	case domain.RefPerson, domain.RefAgent:
		kind := actor.KindUser
		if r.Kind == domain.RefAgent {
			kind = actor.KindAgent
		}
		_, err := s.d.People.Resolve(ctx, identity.ImportRef{Kind: kind, Handle: r.Handle})
		e, isErr := errs.As(err)
		switch {
		case err == nil:
		case isErr && e.Code == errs.CodeNotFound:
			p.Add(r.Pointer, domain.FieldUnknownHandle, "no "+string(r.Kind)+" has this handle")
		case isErr && e.Code == errs.CodeConflict:
			p.Add(r.Pointer, domain.FieldNotActive, "the "+string(r.Kind)+" is not active")
		default:
			return err
		}
	case domain.RefSpace:
		live, err := s.d.Structure.LiveSpace(ctx, r.ID)
		if err != nil {
			return err
		}
		if !live {
			p.Add(r.Pointer, domain.FieldUnknownSpace, "the UUID names no live space")
		}
	case domain.RefChannel:
		_, live, err := s.d.Structure.LiveChannel(ctx, r.ID)
		if err != nil {
			return err
		}
		if !live {
			p.Add(r.Pointer, domain.FieldUnknownChannel, "the UUID names no live channel")
		}
	}
	return nil
}

// Start implements importer.Imports (design note 3).
func (s *Service) Start(ctx context.Context, a actor.Actor, id ids.ID) (importer.Source, error) {
	return s.transition(ctx, a, id, domain.OpStart, nil)
}

// Pause implements importer.Imports (design note 3).
func (s *Service) Pause(ctx context.Context, a actor.Actor, id ids.ID) (importer.Source, error) {
	return s.transition(ctx, a, id, domain.OpPause, nil)
}

// Cancel implements importer.Imports (design note 3): failed with server.cancelled.
func (s *Service) Cancel(ctx context.Context, a actor.Actor, id ids.ID) (importer.Source, error) {
	return s.transition(ctx, a, id, domain.OpCancel, nil)
}

// Complete implements importer.Imports (design note 3, 22 13.5): only when every imported
// channel of the mapping has lead `bothy`.
func (s *Service) Complete(ctx context.Context, a actor.Actor, id ids.ID) (importer.Source, error) {
	return s.transition(ctx, a, id, domain.OpComplete, s.leadsHere)
}

// leadsHere refuses the completion while a mapped channel whose target is not skip and whose
// channel item is posted still has lead discord.
func (s *Service) leadsHere(ctx context.Context, src importer.Source) error {
	m, ok := domain.StoredMapping(src.Config)
	if !ok {
		return errs.New(errs.CodeInternalError, "importer: the stored mapping does not parse")
	}
	posted, err := s.d.Store.ItemsOfKind(ctx, src.ID, domain.KindChannel, importer.ItemPosted)
	if err != nil {
		return err
	}
	for _, it := range posted {
		k, code := domain.ParseOriginKey(domain.KindChannel, it.OriginKey)
		if code != "" {
			continue
		}
		if e := m.Channel(k.IDs[0]); e != nil && !e.Target.Skip && e.Lead != domain.LeadHere {
			return conflict(importer.ReasonLeadNotHere, "an imported channel is still led by the source")
		}
	}
	return nil
}

// transition runs op on the source in one unit of work: import.manage (step-up) on the source,
// the row lock, 404 for an unknown source, 409 import_state for a refused transition, the extra
// condition cond of complete, then state, started_at, finished_at and for cancel server.cancelled,
// the audit entry and import.progress. A repeated transition (start on running, pause on paused)
// records the request with no_change and appends nothing.
func (s *Service) transition(ctx context.Context, a actor.Actor, id ids.ID, op domain.Op,
	cond func(ctx context.Context, src importer.Source) error) (importer.Source, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return importer.Source{}, err
	}
	var out importer.Source
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.transitionUnit(ctx, a, id, op, cond)
		return err
	})
	return out, err
}

// transitionUnit is transition inside its unit of work.
func (s *Service) transitionUnit(ctx context.Context, a actor.Actor, id ids.ID, op domain.Op,
	cond func(ctx context.Context, src importer.Source) error) (importer.Source, error) {
	src, err := s.load(ctx, a, authorize.ActionImportManage, id, true)
	if err != nil {
		return importer.Source{}, err
	}
	step := domain.Transition(src.State, op)
	if !step.Allowed {
		return importer.Source{}, conflict(importer.ReasonImportState, "the operation is not allowed in the state "+src.State)
	}
	attrs := auditing.Attrs{attrOp: auditing.Code(string(op)), attrState: auditing.Code(step.Next)}
	if step.NoEffect {
		attrs[attrNoChange] = auditing.Bool(true)
		return src, s.record(ctx, id, attrs)
	}
	if cond != nil {
		if err := cond(ctx, src); err != nil {
			return importer.Source{}, err
		}
	}
	out, err := s.apply(src, op, step.Next)
	if err != nil {
		return importer.Source{}, err
	}
	if err := s.d.Store.SaveSource(ctx, out); err != nil {
		return importer.Source{}, err
	}
	if err := s.record(ctx, id, attrs); err != nil {
		return importer.Source{}, err
	}
	return out, s.changed(ctx, out)
}

// apply returns src after op into next.
func (s *Service) apply(src importer.Source, op domain.Op, next string) (importer.Source, error) {
	src.State = next
	now := s.now()
	if domain.SetsStartedAt(op) && src.StartedAt.IsZero() {
		src.StartedAt = now
	}
	if domain.SetsFinishedAt(op) {
		src.FinishedAt = now
	}
	if op == domain.OpCancel {
		stats, err := domain.MarkCancelled(src.Stats)
		if err != nil {
			return importer.Source{}, errs.Wrap(errs.CodeInternalError, err, "importer: stats")
		}
		src.Stats = stats
	}
	return src, nil
}

// leadReason is the reason chat receives for a read-only change by the ownership list.
const leadReason = "import lead"

// SetLead implements importer.Imports (design note 4) in one unit of work: import.manage on the
// source, the row lock, 409 import_state for a completed or failed source, 404 for a channel the
// mapping does not name or maps to skip, 409 channel_not_imported while its channel item is not
// posted; then the lead in the stored mapping and the read-only flag of the target channel through
// chat (channel.updated, and the cancellation of active runs by runs when it becomes read-only),
// the audit entry and import.progress. An equal lead records no_change and changes nothing.
func (s *Service) SetLead(ctx context.Context, a actor.Actor, id ids.ID, cmd importer.LeadCmd) (importer.Source, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return importer.Source{}, err
	}
	var out importer.Source
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.setLead(ctx, a, id, cmd)
		return err
	})
	return out, err
}

// leadTarget returns the posted channel item of the mapped channel: 404 for a channel the mapping
// does not name or maps to skip, 409 channel_not_imported while its item is not posted.
func (s *Service) leadTarget(ctx context.Context, src importer.Source, m domain.Mapping, channel string) (*domain.ChannelEntry, ItemRow, error) {
	e := m.Channel(channel)
	if e == nil || e.Target.Skip {
		return nil, ItemRow{}, notFound()
	}
	item, found, err := s.d.Store.Item(ctx, src.ID, domain.Key(domain.KindChannel, channel))
	if err != nil {
		return nil, ItemRow{}, err
	}
	if !found || item.State != importer.ItemPosted || item.TargetID.IsZero() {
		return nil, ItemRow{}, conflict(importer.ReasonChannelNotImported, "the channel is not imported yet")
	}
	return e, item, nil
}

// setLead is SetLead inside its unit of work.
func (s *Service) setLead(ctx context.Context, a actor.Actor, id ids.ID, cmd importer.LeadCmd) (importer.Source, error) {
	src, err := s.load(ctx, a, authorize.ActionImportManage, id, true)
	if err != nil {
		return importer.Source{}, err
	}
	if domain.Ended(src.State) {
		return importer.Source{}, conflict(importer.ReasonImportState, "the source is "+src.State)
	}
	m, ok := domain.StoredMapping(src.Config)
	if !ok {
		return importer.Source{}, errs.New(errs.CodeInternalError, "importer: the stored mapping does not parse")
	}
	e, item, err := s.leadTarget(ctx, src, m, cmd.SourceChannel)
	if err != nil {
		return importer.Source{}, err
	}
	attrs := auditing.Attrs{attrOp: auditing.Code("lead"), attrChannelID: auditing.ID(item.TargetID), attrLead: auditing.Code(cmd.Lead)}
	if e.Lead == cmd.Lead {
		attrs[attrNoChange] = auditing.Bool(true)
		return src, s.record(ctx, id, attrs)
	}
	m.SetLead(cmd.SourceChannel, cmd.Lead)
	if src.Config, err = m.JSON(); err != nil {
		return importer.Source{}, errs.Wrap(errs.CodeInternalError, err, "importer: mapping")
	}
	if err := s.d.Store.SaveSource(ctx, src); err != nil {
		return importer.Source{}, err
	}
	if err := s.d.Structure.SetReadOnly(ctx, item.TargetID, cmd.Lead == importer.LeadDiscord, leadReason); err != nil {
		return importer.Source{}, err
	}
	if err := s.record(ctx, id, attrs); err != nil {
		return importer.Source{}, err
	}
	return src, s.changed(ctx, src)
}

// SetCursor implements importer.Imports (design note 10) in one unit of work: import.apply on the
// source, the row lock, 409 import_not_running unless running; then the cursor replaced and the
// members of stats merged into stats.tool, and import.progress.
func (s *Service) SetCursor(ctx context.Context, a actor.Actor, id ids.ID, cmd importer.CursorCmd) (importer.Source, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return importer.Source{}, err
	}
	var out importer.Source
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		src, err := s.load(ctx, a, authorize.ActionImportApply, id, true)
		if err != nil {
			return err
		}
		if src.State != importer.StateRunning {
			s.deny(ctx, id, importer.ReasonImportNotRunning)
			return conflict(importer.ReasonImportNotRunning, "the source is not running")
		}
		src.Cursor = object(cmd.Cursor)
		if len(cmd.Stats) > 0 {
			if src.Stats, err = domain.MergeTool(src.Stats, cmd.Stats); err != nil {
				return errs.New(errs.CodeValidationFailed, "stats must be an object").WithFields(errs.FieldError{
					Pointer: "/stats", Code: domain.FieldInvalid, Detail: "stats must be an object"})
			}
		}
		if err := s.d.Store.SaveSource(ctx, src); err != nil {
			return err
		}
		out = src
		return s.changed(ctx, src)
	})
	return out, err
}
