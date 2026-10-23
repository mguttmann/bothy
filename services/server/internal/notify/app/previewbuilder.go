// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

// settingPreviewMode is the personal setting that chooses what a preview shows (13 section 6.4).
const settingPreviewMode = "notify.preview_mode"

// imageMIMEPrefix starts the MIME type of an image attachment, whose small thumbnail the full
// preview links (12 section 7.3).
const imageMIMEPrefix = "image/"

// PreviewConfig holds the configuration of the preview builder, bound in internal/wire.
type PreviewConfig struct {
	// DefaultMode is notify.default_preview_mode, the mode of a person without a stored
	// notify.preview_mode.
	DefaultMode string `config:"notify.default_preview_mode"`
	// MaxChars is notify.preview_max_chars, the length of the excerpt in code points.
	MaxChars int `config:"notify.preview_max_chars"`
}

// PreviewMessages is the part of messaging.Messages the preview reads (Messages.ForPreview).
type PreviewMessages interface {
	ForPreview(ctx context.Context, messageID ids.ID) (messaging.PreviewMessage, bool, error)
}

// PreviewNameSource is the part of chat.Structure the preview reads (Structure.PreviewNames).
type PreviewNameSource interface {
	PreviewNames(ctx context.Context, threadID ids.ID) (chat.PreviewNames, error)
}

// AttachmentSource is the part of files.Attachments the full preview reads.
type AttachmentSource interface {
	ForMessages(ctx context.Context, messageIDs []ids.ID) (map[ids.ID][]files.Attachment, error)
}

// PreviewBuilderDeps are the collaborators of the preview builder; none of them authorizes.
type PreviewBuilderDeps struct {
	// Messages reads the message (messaging).
	Messages PreviewMessages
	// Names reads the names of the place (chat).
	Names PreviewNameSource
	// Principals reads the display names and handles of the author and the mentioned (identity).
	Principals PrincipalSource
	// Settings reads the stored notify.preview_mode (sync).
	Settings SettingsSource
	// Attachments reads the attachments of a message with attachments (files).
	Attachments AttachmentSource
}

// PreviewBuilder builds the content of a preview by notify.preview_mode (13 sections 6.3 and 6.4,
// brief M3-22 design note 9): the preview endpoint (Registrations.Preview) and the job
// notify.gateway_send (Push.GatewaySend) both call it, only the mapping into PushPreview or
// NotificationCreatedPayload differs. It never authorizes: the caller checked message.read and
// push.deliver, or push.deliver alone. It is safe for concurrent use.
type PreviewBuilder struct {
	def      domain.PreviewMode
	maxChars int
	d        PreviewBuilderDeps
}

// PreviewFields are the fields of a preview beside the IDs and the badge; zero values are absent
// members. FirstImage is the first attachment in position order whose MIME type is an image, for
// the image_url of the preview endpoint; the gateway event carries no image (design note 9 f).
type PreviewFields struct {
	SpaceName, ChannelName, ThreadTitle, SenderDisplayName, Excerpt string
	AttachmentCount                                                 int
	FirstImage                                                      ids.ID
}

// NewPreviewBuilder returns the builder. Errors: a missing collaborator, a default mode that is
// not full, names or none, or an excerpt length below 1.
func NewPreviewBuilder(cfg PreviewConfig, d PreviewBuilderDeps) (*PreviewBuilder, error) {
	if d.Messages == nil || d.Names == nil || d.Principals == nil || d.Settings == nil || d.Attachments == nil {
		return nil, errors.New("notify: the preview builder needs messaging, chat, identity, sync and files")
	}
	def := domain.PreviewMode(cfg.DefaultMode)
	if !def.Valid() {
		return nil, errors.New("notify: notify.default_preview_mode must be full, names or none")
	}
	if cfg.MaxChars < 1 {
		return nil, errors.New("notify: notify.preview_max_chars must be at least 1")
	}
	return &PreviewBuilder{def: def, maxChars: cfg.MaxChars, d: d}, nil
}

// Mode returns the preview mode of user: the stored notify.preview_mode, or
// notify.default_preview_mode when none or no valid value is stored.
//
// Errors: those of the read.
func (b *PreviewBuilder) Mode(ctx context.Context, user ids.ID) (domain.PreviewMode, error) {
	vals, err := b.d.Settings.ValuesFor(ctx, []ids.ID{user}, []string{settingPreviewMode})
	if err != nil {
		return "", err
	}
	var s string
	if raw, ok := vals[user][settingPreviewMode]; ok && json.Unmarshal(raw, &s) != nil {
		s = ""
	}
	return domain.ParsePreviewMode(s, b.def), nil
}

// Message returns the message id when it may have a preview: found is false for an unknown
// message, a tombstone and the kinds run (a live box anchor) and system (brief M3-22 design note
// 8 g).
//
// Errors: those of the read.
func (b *PreviewBuilder) Message(ctx context.Context, id ids.ID) (messaging.PreviewMessage, bool, error) {
	m, found, err := b.d.Messages.ForPreview(ctx, id)
	if err != nil || !found || m.Deleted || m.Kind == messageKindRun || m.Kind == messageKindSys {
		return messaging.PreviewMessage{}, false, err
	}
	return m, true, nil
}

// Fields returns the fields of mode for m (13 6.4 table): none gives nothing; names the space,
// channel and thread names of the place and the display name of the author; full adds the excerpt
// (domain.Excerpt with the display names of the mentioned principals), the number of attachments
// and the first image. Attachments are read only when m has any.
//
// Errors: those of the reads.
func (b *PreviewBuilder) Fields(ctx context.Context, mode domain.PreviewMode, m messaging.PreviewMessage) (PreviewFields, error) {
	var out PreviewFields
	if !mode.ShowsNames() {
		return out, nil
	}
	names, err := b.d.Names.PreviewNames(ctx, m.ThreadID)
	if err != nil {
		return PreviewFields{}, err
	}
	out.SpaceName, out.ChannelName, out.ThreadTitle = names.SpaceName, names.ChannelName, names.ThreadTitle
	people := []ids.ID{m.AuthorID}
	if mode.ShowsContent() {
		people = append(people, m.MentionIDs...)
	}
	views, err := b.d.Principals.Principals(ctx, people)
	if err != nil {
		return PreviewFields{}, err
	}
	out.SenderDisplayName = views[m.AuthorID].DisplayName
	if !mode.ShowsContent() {
		return out, nil
	}
	mentioned := make(map[string]string, len(m.MentionIDs))
	for _, id := range m.MentionIDs {
		if v, ok := views[id]; ok && v.Handle != "" {
			mentioned[strings.ToLower(v.Handle)] = v.DisplayName
		}
	}
	out.Excerpt = domain.Excerpt(m.ContentMD, mentioned, b.maxChars)
	if !m.HasAttachments {
		return out, nil
	}
	atts, err := b.d.Attachments.ForMessages(ctx, []ids.ID{m.ID})
	if err != nil {
		return PreviewFields{}, err
	}
	list := atts[m.ID]
	out.AttachmentCount = len(list)
	for _, a := range list {
		if strings.HasPrefix(a.File.MIMEType, imageMIMEPrefix) {
			out.FirstImage = a.File.ID
			break
		}
	}
	return out, nil
}
