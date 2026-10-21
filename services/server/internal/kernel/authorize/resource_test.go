// SPDX-License-Identifier: Apache-2.0

package authorize_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestResourceFromEvent: a thread, channel or space names itself without hints, any other kind
// carries only its most specific scope; every descriptor carries the full scope in Event, and an
// unknown kind stays empty.
func TestResourceFromEvent(t *testing.T) {
	t.Parallel()
	space, channel, thread, id := ids.New(), ids.New(), ids.New(), ids.New()
	full := &authorize.EventScope{SpaceID: space, ChannelID: channel, ThreadID: thread}
	cases := []struct {
		kind                string
		space, ch, th       ids.ID
		wantKind            authorize.ResourceKind
		wantSp, wantCh, wTh ids.ID
	}{
		{"thread", space, channel, thread, authorize.ResourceThread, ids.ID{}, ids.ID{}, ids.ID{}},
		{"channel", space, channel, ids.ID{}, authorize.ResourceChannel, ids.ID{}, ids.ID{}, ids.ID{}},
		{"space", space, ids.ID{}, ids.ID{}, authorize.ResourceSpace, ids.ID{}, ids.ID{}, ids.ID{}},
		{"message", space, channel, thread, authorize.ResourceMessage, ids.ID{}, ids.ID{}, thread},
		{"file", space, channel, ids.ID{}, authorize.ResourceFile, ids.ID{}, channel, ids.ID{}},
		{"grant", space, ids.ID{}, ids.ID{}, authorize.ResourceGrant, space, ids.ID{}, ids.ID{}},
		{"no_such_kind", space, channel, thread, "", ids.ID{}, ids.ID{}, thread},
	}
	for _, c := range cases {
		r := authorize.ResourceFromEvent(c.kind, id, c.space, c.ch, c.th)
		if r.Kind != c.wantKind || r.ID != id || r.SpaceID != c.wantSp || r.ChannelID != c.wantCh || r.ThreadID != c.wTh || r.Event == nil {
			t.Errorf("%s: %+v", c.kind, r)
			continue
		}
		if c.kind == "message" && *r.Event != *full {
			t.Errorf("message scope %+v", *r.Event)
		}
	}
}

// TestInteractionStages: the two stages of InteractionFacts carry the codes authz expects
// (brief M2-15 design note 8 a and c).
func TestInteractionStages(t *testing.T) {
	t.Parallel()
	if authorize.InteractionStageAccept != "accept" || authorize.InteractionStageDeliver != "deliver" {
		t.Fatalf("stages %q %q", authorize.InteractionStageAccept, authorize.InteractionStageDeliver)
	}
	r := authorize.Resource{Kind: authorize.ResourceComponent, Interaction: &authorize.InteractionFacts{Stage: authorize.InteractionStageAccept}}
	if r.Interaction.Stage != "accept" {
		t.Fatal("interaction facts not kept")
	}
}
