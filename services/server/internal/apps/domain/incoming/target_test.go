// SPDX-License-Identifier: Apache-2.0

package incoming_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/apps/domain/incoming"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

func TestChooseTarget(t *testing.T) {
	channel, other := ids.New(), ids.New()
	main, fixed, bodyThread, foreign := ids.New(), ids.New(), ids.New(), ids.New()
	inChannel := func(id ids.ID) *incoming.ThreadFact {
		return &incoming.ThreadFact{ID: id, ChannelID: channel, Found: true}
	}
	text := func(f incoming.TargetFacts) incoming.TargetFacts {
		f.ChannelID, f.ChannelKind, f.MainThreadID = channel, "text", main
		if f.Format == "" {
			f.Format = incoming.FormatNative
		}
		return f
	}
	forum := func(f incoming.TargetFacts) incoming.TargetFacts {
		f.ChannelID, f.ChannelKind = channel, "forum"
		if f.Format == "" {
			f.Format = incoming.FormatNative
		}
		return f
	}
	title := ptr("Nightly report")

	for _, tc := range []struct {
		name  string
		facts incoming.TargetFacts
		want  incoming.Target
	}{
		{"fixed thread in the channel wins over body thread and title",
			text(incoming.TargetFacts{Fixed: inChannel(fixed), Body: inChannel(bodyThread), Title: title}),
			incoming.Target{Kind: incoming.TargetExisting, ThreadID: fixed}},
		{"fixed thread for alertmanager",
			forum(incoming.TargetFacts{Format: incoming.FormatAlertmanager, Fixed: inChannel(fixed)}),
			incoming.Target{Kind: incoming.TargetExisting, ThreadID: fixed}},
		{"body thread of the channel",
			text(incoming.TargetFacts{Body: inChannel(bodyThread), Title: title}),
			incoming.Target{Kind: incoming.TargetExisting, ThreadID: bodyThread}},
		{"body thread of another channel falls through to the main thread",
			text(incoming.TargetFacts{Body: &incoming.ThreadFact{ID: foreign, ChannelID: other, Found: true}}),
			incoming.Target{Kind: incoming.TargetExisting, ThreadID: main}},
		{"unknown body thread falls through to the main thread",
			text(incoming.TargetFacts{Body: &incoming.ThreadFact{ID: foreign}}),
			incoming.Target{Kind: incoming.TargetExisting, ThreadID: main}},
		{"body thread of another channel falls through to the title",
			forum(incoming.TargetFacts{Body: &incoming.ThreadFact{ID: foreign, ChannelID: other, Found: true}, Title: title}),
			incoming.Target{Kind: incoming.TargetForumPost, Title: *title}},
		{"title in a forum",
			forum(incoming.TargetFacts{Title: title}),
			incoming.Target{Kind: incoming.TargetForumPost, Title: *title}},
		{"title in a text channel",
			text(incoming.TargetFacts{Title: title}),
			incoming.Target{Kind: incoming.TargetTextThread, ThreadID: main, Title: *title}},
		{"nothing in a text channel",
			text(incoming.TargetFacts{}),
			incoming.Target{Kind: incoming.TargetExisting, ThreadID: main}},
		{"alertmanager ignores body thread and title in a text channel",
			text(incoming.TargetFacts{Format: incoming.FormatAlertmanager, Body: inChannel(bodyThread), Title: title}),
			incoming.Target{Kind: incoming.TargetExisting, ThreadID: main}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := incoming.ChooseTarget(tc.facts)
			if err != nil || got != tc.want {
				t.Fatalf("ChooseTarget = %+v, %v; want %+v", got, err, tc.want)
			}
		})
	}

	for _, tc := range []struct {
		name  string
		facts incoming.TargetFacts
	}{
		{"fixed thread moved away", text(incoming.TargetFacts{Fixed: &incoming.ThreadFact{ID: fixed, ChannelID: other, Found: true}, Body: inChannel(bodyThread)})},
		{"fixed thread unknown", text(incoming.TargetFacts{Fixed: &incoming.ThreadFact{ID: fixed}, Title: title})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := incoming.ChooseTarget(tc.facts)
			if e, ok := errs.As(err); !ok || e.Code != errs.CodeNotFound {
				t.Fatalf("error = %v, want not_found", err)
			}
		})
	}

	for _, tc := range []struct {
		name    string
		facts   incoming.TargetFacts
		pointer string
	}{
		{"nothing in a forum, native", forum(incoming.TargetFacts{}), "/title"},
		{"nothing in a forum, alertmanager", forum(incoming.TargetFacts{Format: incoming.FormatAlertmanager, Title: title}), ""},
		{"unknown channel kind, native", incoming.TargetFacts{Format: incoming.FormatNative, ChannelID: channel, ChannelKind: "voice", Title: title}, "/title"},
		{"text channel without main thread", incoming.TargetFacts{Format: incoming.FormatNative, ChannelID: channel, ChannelKind: "text"}, "/title"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := incoming.ChooseTarget(tc.facts)
			wantField(t, err, tc.pointer, incoming.CodeRequiredForForumPost)
		})
	}
}
