// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
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
