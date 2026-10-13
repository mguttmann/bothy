// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"strings"
	"time"

	"github.com/OWNER/bothy/services/server/internal/audit"
	"github.com/OWNER/bothy/services/server/internal/audit/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// eventsList is the list name of the cursors of listAuditEvents.
const eventsList = "audit_events"

// eventsCursor is the keyset position of a page: the seq of its last row.
type eventsCursor struct {
	Seq int64 `json:"s"`
}

// The audit attributes of the reads (IDs, codes and times only, 19 SEC-AUD-5).
const (
	attrActorID      = "actor_id"
	attrAction       = "action"
	attrDecision     = "decision"
	attrResourceKind = "resource_kind"
	attrResourceID   = "resource_id"
	attrFrom         = "from"
	attrTo           = "to"
	attrFromSeq      = "from_seq"
	attrToSeq        = "to_seq"
	attrOK           = "ok"
	attrBrokenSeq    = "first_broken_seq"
	attrReason       = "failure_reason"
)

// List implements audit.Log: authorize audit.read, check the filters, read one page newest first
// by seq after the keyset of the cursor, and record the allow entry with the filters before the
// page is returned (20: every read of the log is audited). The cursor binds the caller and every
// filter, so another filter gives cursor_invalid; new rows never move a page, because the keyset
// is the seq.
func (l *Log) List(ctx context.Context, q audit.Query) (audit.Page, error) {
	if err := l.check(ctx, authorize.ActionAuditRead); err != nil {
		return audit.Page{}, err
	}
	if err := checkQuery(q); err != nil {
		return audit.Page{}, err
	}
	a, _ := actor.FromContext(ctx)
	binding := queryBinding(a, q)
	f := EventFilter{
		ActorID: q.ActorID, ResourceID: q.ResourceID, Action: q.Action, Decision: q.Decision,
		ResourceKind: q.ResourceKind, From: q.From, To: q.To,
	}
	before, err := l.cursorSeq(binding, q.Cursor)
	if err != nil {
		return audit.Page{}, err
	}
	f.BeforeSeq = before
	limit := q.Limit
	if limit < 1 {
		limit = l.cfg.PageSizeDefault
	}
	limit = min(limit, l.cfg.PageSizeMax)
	f.Limit = limit + 1
	rows, err := l.d.Store.ListEvents(ctx, f)
	if err != nil {
		return audit.Page{}, err
	}
	out, err := l.page(rows, limit, binding)
	if err != nil {
		return audit.Page{}, err
	}
	if err := l.record(ctx, authorize.ActionAuditRead, queryAttrs(q)); err != nil {
		return audit.Page{}, err
	}
	return out, nil
}

// cursorSeq returns the keyset seq of cursor, 0 for none. Errors: cursor_invalid for a cursor of
// another list, caller or filter, or without a positive seq.
func (l *Log) cursorSeq(binding []byte, cursor string) (int64, error) {
	if cursor == "" {
		return 0, nil
	}
	var k eventsCursor
	if err := l.d.Cursors.Decode(eventsList, binding, cursor, &k); err != nil {
		return 0, err
	}
	if k.Seq < 1 {
		return 0, errs.New(errs.CodeCursorInvalid, "")
	}
	return k.Seq, nil
}

// page returns the first limit rows and, when rows holds one more, the cursor after the last.
func (l *Log) page(rows []domain.Row, limit int, binding []byte) (audit.Page, error) {
	out := audit.Page{Items: make([]audit.Event, 0, min(len(rows), limit))}
	for i, r := range rows {
		if i == limit {
			break
		}
		out.Items = append(out.Items, event(r))
	}
	if len(rows) > limit {
		next, err := l.d.Cursors.Encode(eventsList, binding, eventsCursor{Seq: rows[limit-1].Seq})
		if err != nil {
			return audit.Page{}, errs.Wrap(errs.CodeInternalError, err, "audit: list cursor")
		}
		out.NextCursor = next
	}
	return out, nil
}

// checkQuery refuses a filter of action or resource_kind that is not a code: nothing else is
// stored there, and the filter goes into the audit entry, which takes codes only.
func checkQuery(q audit.Query) error {
	var fields []errs.FieldError
	for _, f := range []struct{ name, value string }{{"action", q.Action}, {"resource_kind", q.ResourceKind}} {
		if f.value != "" && !domain.ValidCode(f.value) {
			fields = append(fields, errs.FieldError{Pointer: "#query/" + f.name, Code: "invalid_format", Detail: "The filter is not a code."})
		}
	}
	switch q.Decision {
	case "", domain.DecisionAllow, domain.DecisionDeny, domain.DecisionError:
	default:
		fields = append(fields, errs.FieldError{Pointer: "#query/decision", Code: "invalid_enum", Detail: "The decision is allow, deny or error."})
	}
	if len(fields) > 0 {
		return errs.New(errs.CodeValidationFailed, "").WithFields(fields...)
	}
	return nil
}

// idText is the text of id in a binding; empty for the zero ID.
func idText(id ids.ID) string {
	if id.IsZero() {
		return ""
	}
	return id.String()
}

// timeText is the text of t in a binding; empty for the zero time.
func timeText(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

// queryBinding is the canonical encoding of the caller and every filter, the binding of the
// cursor (06 "Pagination" item 2).
func queryBinding(a actor.Actor, q audit.Query) []byte {
	parts := []string{
		a.ID.String(), "actor=" + idText(q.ActorID), "action=" + q.Action, "decision=" + q.Decision,
		"resource_kind=" + q.ResourceKind, "resource=" + idText(q.ResourceID),
		"from=" + timeText(q.From), "to=" + timeText(q.To), "sort=seq_desc",
	}
	return []byte(strings.Join(parts, "\x00"))
}

// queryAttrs are the audit attributes of a list call: the filters that are set.
func queryAttrs(q audit.Query) auditing.Attrs {
	attrs := auditing.Attrs{}
	if !q.ActorID.IsZero() {
		attrs[attrActorID] = auditing.ID(q.ActorID)
	}
	if !q.ResourceID.IsZero() {
		attrs[attrResourceID] = auditing.ID(q.ResourceID)
	}
	for k, v := range map[string]string{attrAction: q.Action, attrDecision: q.Decision, attrResourceKind: q.ResourceKind} {
		if v != "" {
			attrs[k] = auditing.Code(v)
		}
	}
	if !q.From.IsZero() {
		attrs[attrFrom] = auditing.Time(q.From)
	}
	if !q.To.IsZero() {
		attrs[attrTo] = auditing.Time(q.To)
	}
	return attrs
}
