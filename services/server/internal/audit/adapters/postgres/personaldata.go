// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/audit/adapters/postgres/sqlcgen"
	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
)

// personalPage is how many entries one read of the export holds, so a long history of a person
// never loads at once.
const personalPage = 500

// Actor kinds of an exported audit entry.
const (
	actorPrincipal = "principal"
	actorSystem    = "system"
)

// PersonalData is the privacy participant of the audit module (docs/spec/20-privacy.md sections
// 4.4, 4.5 and 5.2, M7-06). It exports the entries of audit_events with the person as actor or as
// resource of kind user or principal into audit.jsonl, and erases nothing: the IDs in the log
// become pseudonymous through the pseudonymization of identity in step 6, so no row changes and
// the hash chain stays valid (20 section 4.5, ADR 0021). audit_checkpoints and audit_ip_keys hold
// no personal reference (20 section 2). It is safe for concurrent use.
type PersonalData struct {
	pool *pg.Pool
}

// Compile-time check: PersonalData is a participant of the privacy jobs.
var _ personaldata.Participant = (*PersonalData)(nil)

// NewPersonalData returns the participant over pool, a pool of bothy_app (which may only SELECT
// and INSERT audit_events; the participant only reads).
func NewPersonalData(pool *pg.Pool) *PersonalData {
	return &PersonalData{pool: pool}
}

// Module returns "audit" (architecture/modules.yaml).
func (*PersonalData) Module() string { return "audit" }

// Tables returns audit_events, the only table of the module with a personal reference (20 section
// 2: indirect, through actor and resource IDs and the IP pseudonym).
func (*PersonalData) Tables() []string { return []string{"audit_events"} }

// auditLine is one line of audit.jsonl (docs/api/gdpr-export/v1/lines/audit.schema.json). It
// leaves out prev_hash, hash, details (the free-text reason of an administrative action may be
// written by another person; ip_hmac and ip_key_id are pseudonyms with a key), the device,
// session and correlation IDs and the row ID: 20 section 5.2 allows no hashes or keys, and the
// listed members are what the person needs to see what was done by or to them.
type auditLine struct {
	Seq          int64     `json:"seq"`
	OccurredAt   time.Time `json:"occurred_at"`
	Action       string    `json:"action"`
	Decision     string    `json:"decision"`
	ReasonCode   string    `json:"reason_code,omitempty"`
	ResourceKind string    `json:"resource_kind,omitempty"`
	ResourceID   string    `json:"resource_id,omitempty"`
	ActorKind    string    `json:"actor_kind"`
	ActorID      string    `json:"actor_id,omitempty"`
}

// Export writes one line of audit.jsonl per entry with e.Subject as actor or as user/principal
// resource, ascending by seq. It reads in pages of personalPage entries keyed on seq, each page
// one statement on the pool (no long transaction); rows appended meanwhile with a larger seq are
// included, which only extends the export.
//
// Errors: those of the storage and of w.
func (p *PersonalData) Export(ctx context.Context, e personaldata.Export, w personaldata.Writer) error {
	q := sqlcgen.New(pg.Querier(ctx, p.pool))
	var after int64
	for {
		rows, err := q.PersonalEntries(ctx, sqlcgen.PersonalEntriesParams{
			AfterSeq: after, Subject: optionalUUID(e.Subject), RowLimit: personalPage,
		})
		if err != nil {
			return err
		}
		for _, r := range rows {
			if err := w.Line(personaldata.FileAudit, lineOf(r)); err != nil {
				return err
			}
			after = r.Seq
		}
		if len(rows) < personalPage {
			return nil
		}
	}
}

// lineOf converts one row; a NULL actor is the system or a server command (03 audit_events).
func lineOf(r sqlcgen.PersonalEntriesRow) auditLine {
	l := auditLine{
		Seq: r.Seq, OccurredAt: r.OccurredAt.Time.UTC(), Action: r.Action, Decision: r.Decision,
		ReasonCode: r.DenyReason.String, ResourceKind: r.ResourceKind.String, ActorKind: actorSystem,
	}
	if r.ResourceID.Valid {
		l.ResourceID = idOf(r.ResourceID).String()
	}
	if r.ActorPrincipalID.Valid {
		l.ActorKind, l.ActorID = actorPrincipal, idOf(r.ActorPrincipalID).String()
	}
	return l
}

// Erase does nothing in every step and returns nil: audit_events is append-only (bothy_app may
// only SELECT and INSERT), and 20 section 4.5 keeps every row so the chain stays valid; privacy
// writes the audit entry of step 7 itself.
func (*PersonalData) Erase(context.Context, personaldata.Step, personaldata.Erasure) error {
	return nil
}
