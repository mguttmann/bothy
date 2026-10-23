// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/notify/app"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

// previewMessages answers ForPreview from a map.
type previewMessages map[ids.ID]messaging.PreviewMessage

func (m previewMessages) ForPreview(_ context.Context, id ids.ID) (messaging.PreviewMessage, bool, error) {
	v, ok := m[id]
	return v, ok, nil
}

// previewNames answers PreviewNames with fixed names.
type previewNames chat.PreviewNames

func (n previewNames) PreviewNames(context.Context, ids.ID) (chat.PreviewNames, error) {
	return chat.PreviewNames(n), nil
}

// previewPrincipals answers Principals from a map.
type previewPrincipals map[ids.ID]identity.PrincipalView

func (p previewPrincipals) Principals(_ context.Context, list []ids.ID) (map[ids.ID]identity.PrincipalView, error) {
	out := map[ids.ID]identity.PrincipalView{}
	for _, id := range list {
		if v, ok := p[id]; ok {
			out[id] = v
		}
	}
	return out, nil
}

// previewSettings answers notify.preview_mode with a raw JSON value per person.
type previewSettings map[ids.ID]string

func (s previewSettings) ValuesFor(_ context.Context, users []ids.ID, _ []string) (map[ids.ID]map[string]json.RawMessage, error) {
	out := map[ids.ID]map[string]json.RawMessage{}
	for _, u := range users {
		if v, ok := s[u]; ok {
			out[u] = map[string]json.RawMessage{"notify.preview_mode": json.RawMessage(v)}
		}
	}
	return out, nil
}

// previewAttachments answers ForMessages from a map and counts the calls.
type previewAttachments struct {
	list  map[ids.ID][]files.Attachment
	calls int
}

func (a *previewAttachments) ForMessages(_ context.Context, msgs []ids.ID) (map[ids.ID][]files.Attachment, error) {
	a.calls++
	out := map[ids.ID][]files.Attachment{}
	for _, m := range msgs {
		if l, ok := a.list[m]; ok {
			out[m] = l
		}
	}
	return out, nil
}

// TestPreviewBuilder_FieldsByMode checks the fields of each mode, the mention names in the
// excerpt, the attachment count, the first image in position order and that attachments are read
// only for a message that has any (design note 9 c).
func TestPreviewBuilder_FieldsByMode(t *testing.T) {
	alex, sam, msg, noAtt := ids.New(), ids.New(), ids.New(), ids.New()
	text, image := ids.New(), ids.New()
	atts := &previewAttachments{list: map[ids.ID][]files.Attachment{msg: {
		{Position: 0, File: files.File{ID: text, MIMEType: "text/plain"}},
		{Position: 1, File: files.File{ID: image, MIMEType: "image/png"}},
	}}}
	b, err := app.NewPreviewBuilder(app.PreviewConfig{DefaultMode: "names", MaxChars: 240}, app.PreviewBuilderDeps{
		Messages: previewMessages{
			msg:   {ID: msg, AuthorID: alex, Kind: "text", ContentMD: "**Hi** @sam", HasAttachments: true, MentionIDs: []ids.ID{sam}},
			noAtt: {ID: noAtt, AuthorID: alex, Kind: "text", ContentMD: "plain"},
		},
		Names:       previewNames{SpaceName: "Support", ChannelName: "Allgemein"},
		Principals:  previewPrincipals{alex: {ID: alex, Handle: "alex", DisplayName: "Alex"}, sam: {ID: sam, Handle: "sam", DisplayName: "Sam"}},
		Settings:    previewSettings{sam: `"full"`, alex: `"yes"`},
		Attachments: atts,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if mode, _ := b.Mode(ctx, sam); mode != domain.PreviewFull {
		t.Errorf("stored full gives %q", mode)
	}
	if mode, _ := b.Mode(ctx, alex); mode != domain.PreviewNames {
		t.Errorf("stored yes gives %q, want the default names", mode)
	}
	if mode, _ := b.Mode(ctx, ids.New()); mode != domain.PreviewNames {
		t.Errorf("nothing stored gives %q, want the default names", mode)
	}
	m, _, _ := b.Message(ctx, msg)
	full, err := b.Fields(ctx, domain.PreviewFull, m)
	if err != nil {
		t.Fatal(err)
	}
	if full.SpaceName != "Support" || full.SenderDisplayName != "Alex" || full.Excerpt != "Hi Sam" || full.AttachmentCount != 2 || full.FirstImage != image {
		t.Errorf("full %+v", full)
	}
	names, _ := b.Fields(ctx, domain.PreviewNames, m)
	if names.SpaceName != "Support" || names.Excerpt != "" || names.AttachmentCount != 0 || !names.FirstImage.IsZero() {
		t.Errorf("names %+v", names)
	}
	if none, _ := b.Fields(ctx, domain.PreviewNone, m); none != (app.PreviewFields{}) {
		t.Errorf("none %+v", none)
	}
	calls := atts.calls
	plain, _, _ := b.Message(ctx, noAtt)
	if _, err := b.Fields(ctx, domain.PreviewFull, plain); err != nil || atts.calls != calls {
		t.Errorf("a message without attachments read them (%d calls, err %v)", atts.calls-calls, err)
	}
	for _, kind := range []string{"run", "system"} {
		id := ids.New()
		b2, _ := app.NewPreviewBuilder(app.PreviewConfig{DefaultMode: "full", MaxChars: 10}, app.PreviewBuilderDeps{
			Messages: previewMessages{id: {ID: id, Kind: kind}}, Names: previewNames{}, Principals: previewPrincipals{},
			Settings: previewSettings{}, Attachments: atts,
		})
		if _, found, _ := b2.Message(ctx, id); found {
			t.Errorf("a %s message has a preview", kind)
		}
	}
}
