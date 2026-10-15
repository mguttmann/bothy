// SPDX-License-Identifier: Apache-2.0

package messaging

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TemplateView is a template as the API shows it (schema Template). Scope is "personal" or
// "channel"; ChannelID is zero for a personal template.
type TemplateView struct {
	ID        ids.ID
	Scope     string
	ChannelID ids.ID
	Name      string
	ContentMD string
	Version   int64
}

// TemplateInput is the body of a create (schema TemplateCreate). Both members are stored as
// given, without trimming.
type TemplateInput struct {
	Name, ContentMD string
}

// Precondition is the parsed If-Match header of a change: Present for a non-empty header,
// Valid for exactly one strong ETag, Version its number.
type Precondition struct {
	Present, Valid bool
	Version        int64
}

// TemplateChange is the request of updateTemplate (schema TemplateUpdate); nil members are
// absent.
type TemplateChange struct {
	TemplateID      ids.ID
	Name, ContentMD *string
	IfMatch         Precondition
}

// Templates is the incoming port of the message templates (M1-60, 03 message_templates). The
// caller is the principal in ctx (kernel/actor). Personal templates are checked against the
// resource user of their owner (self), channel templates against the channel (read to list,
// manage to change, 06 K15). Templates append no event (05 section 6): clients reload them by
// REST. Errors are kernel/errs errors; a missing, deleted, foreign personal or invisible template
// is the same 404 not_found.
type Templates interface {
	// List returns the caller's personal templates and those of the channel, by lower(name),
	// then id (template.read on the channel; 404 for a missing or invisible channel).
	List(ctx context.Context, channelID ids.ID) ([]TemplateView, error)
	// ListMine returns the caller's personal templates (template.read on the user), also
	// without any readable channel. Errors: 403 for an agent or app.
	ListMine(ctx context.Context) ([]TemplateView, error)
	// CreatePersonal creates a personal template of the caller (template.manage on the user)
	// within template.max_per_scope. Errors: 400 validation_failed (field errors, or
	// limit_reached with an empty pointer).
	//
	// Example:
	//
	//	v, err := templates.CreatePersonal(ctx, messaging.TemplateInput{Name: "Escalation", ContentMD: "Please escalate."})
	CreatePersonal(ctx context.Context, c TemplateInput) (TemplateView, error)
	// CreateForChannel creates a template of the channel (template.manage, needs manage: read
	// without it is 403, no read 404) within template.max_per_scope.
	CreateForChannel(ctx context.Context, channelID ids.ID, c TemplateInput) (TemplateView, error)
	// Update changes name and content under If-Match (412 version_conflict with current); a
	// change without effect returns the template unchanged and writes nothing.
	Update(ctx context.Context, c TemplateChange) (TemplateView, error)
	// Delete deletes the template; deleting a deleted one is a success without write.
	Delete(ctx context.Context, templateID ids.ID) error
}
