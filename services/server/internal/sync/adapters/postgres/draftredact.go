// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"encoding/json"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// resourceDraft and typeDraftUpdated are the resource kind and the type of the stored draft
// events.
const (
	resourceDraft    = "draft"
	typeDraftUpdated = "draft.updated"
)

// redactDrafts blanks content_md of every stored draft.updated event of the deleted drafts, in the
// transaction of the deletion (docs/spec/20-privacy.md: event_log, "redaction on deletion"; audit
// srv-gateway-sync-12). The envelope and the other members stay, so the payload keeps its schema;
// draft.deleted follows in the same transaction. Errors: those of Redactor.Redact.
func redactDrafts(ctx context.Context, drafts ...sync.Draft) error {
	if len(drafts) == 0 {
		return nil
	}
	r := &Redactor{} // Redact runs in the ambient transaction of ctx and needs no pool
	for _, d := range drafts {
		if _, err := r.Redact(ctx, resourceDraft, d.ID, blankDraftText); err != nil {
			return err
		}
	}
	return nil
}

// blankDraftText is the RedactFunc of a deleted draft: content_md of draft.updated becomes empty,
// every other event stays as stored.
func blankDraftText(eventType string, _ int, payload json.RawMessage) (json.RawMessage, error) {
	if eventType != typeDraftUpdated {
		return payload, nil
	}
	var p map[string]json.RawMessage
	var d map[string]json.RawMessage
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, err, "sync: a stored draft.updated payload is not an object")
	}
	if err := json.Unmarshal(p["draft"], &d); err != nil || d == nil {
		return nil, errs.New(errs.CodeInternalError, "sync: a stored draft.updated payload has no draft")
	}
	d["content_md"] = json.RawMessage(`""`)
	b, err := json.Marshal(d)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, err, "sync: encoding a redacted draft")
	}
	p["draft"] = b
	return json.Marshal(p)
}
