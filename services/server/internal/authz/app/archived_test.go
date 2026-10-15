// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestArchivedChannel (07 section 4.5 point 1): in an archived channel every level is at most
// read, owners included; write actions end with channel_archived, reading stays, and managing
// actions pass authorize (chat answers them with 409, point 2).
func TestArchivedChannel(t *testing.T) {
	w := newWorld(t)
	w.setChannel(t, w.allgemein, func(c *authz.ChannelFacts) { c.Archived = true })
	forbidden := authorize.OutcomeForbidden
	w.expect(t, w.user(w.owner), authorize.ActionMessageCreate, w.thread(w.allgemein), forbidden, "channel_archived")
	w.expect(t, w.user(w.alex), authorize.ActionAgentInvoke, w.invoke(w.allgemein, w.lotse, true), forbidden, "channel_archived")
	w.expect(t, w.user(w.sam), authorize.ActionMessageCreate, w.thread(w.allgemein), forbidden, "channel_archived")
	w.allow(t, w.user(w.sam), authorize.ActionMessageRead, w.msg(w.allgemein, w.alex))
	w.allow(t, w.user(w.owner), authorize.ActionMessageRead, w.msg(w.allgemein, w.alex))
	w.allow(t, w.user(w.owner), authorize.ActionChannelUpdate, w.channel(w.allgemein))
	w.expect(t, w.agent(w.lotse), authorize.ActionMessageCreate, w.thread(w.allgemein), forbidden, "channel_archived")
	// Another channel of the space stays writable.
	w.allow(t, w.user(w.alex), authorize.ActionMessageCreate, w.thread(w.eskalation))
}

// TestArchivedSpace (07 section 4.5, M1-52): in an archived space every level is at most read,
// owners included; reading and managing pass authorize (chat answers management with 409).
func TestArchivedSpace(t *testing.T) {
	w := newWorld(t)
	w.structure.SetSpaceArchived(w.support, true)
	space := authorize.Resource{Kind: authorize.ResourceSpace, ID: w.support}
	w.allow(t, w.user(w.alex), authorize.ActionSpaceRead, space)
	w.allow(t, w.user(w.owner), authorize.ActionSpaceUpdate, space)
	r, err := w.az.EffectiveRights(context.Background(), w.user(w.owner), authorize.ActionSpaceRead, space)
	if err != nil || r.Level != "read" || len(r.Capabilities) == 0 {
		t.Fatalf("owner in an archived space: %+v, %v", r, err)
	}
}

// TestEffectiveRights (M1-52, SpaceView): the level and capabilities of the snapshot authorize
// uses; none for a space visible only through a channel, for an unknown space and for a caller
// authorize refuses.
func TestEffectiveRights(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	space := authorize.Resource{Kind: authorize.ResourceSpace, ID: w.support}
	for _, c := range []struct {
		name  string
		p     authorize.Principal
		r     authorize.Resource
		level string
		caps  []string
	}{
		{"space grant", w.user(w.alex), space, "trigger", []string{"approve_actions", "upload"}},
		{"owner", w.user(w.owner), space, "trigger", nil},
		{"through a channel only", w.user(w.sam), space, "", []string{}},
		{"unknown space", w.user(w.alex), authorize.Resource{Kind: authorize.ResourceSpace, ID: ids.New()}, "", []string{}},
		{"anonymous", actor.Anonymous(), space, "", []string{}},
	} {
		r, err := w.az.EffectiveRights(ctx, c.p, authorize.ActionSpaceRead, c.r)
		if err != nil || r.Level != c.level || (c.caps != nil && !slices.Equal(r.Capabilities, c.caps)) || r.Capabilities == nil {
			t.Errorf("%s: %+v, %v", c.name, r, err)
		}
	}
	w.structure.SetSpaceArchived(w.support, true)
	if r, err := w.az.EffectiveRights(ctx, w.user(w.alex), authorize.ActionSpaceRead, space); err != nil || r.Level != "read" {
		t.Fatalf("archived: %+v, %v", r, err)
	}
}
