// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
)

// TestOperatorRuleUpload (FIX-operator-upload-grant, 07 section 2.9 point 2): the effective operator
// of a channel holds upload together with write, without a grant row, so an agent that sends files
// needs no extra grant: Texter operates Kampagnen and has no grant, yet it may upload and post a
// message with attachments there. The capability follows the operator: after the operator changes
// to Runner, Texter has nothing on the channel and Runner uploads at once.
func TestOperatorRuleUpload(t *testing.T) {
	w := newWorld(t)
	w.allow(t, w.agent(w.texter), authorize.ActionFileUpload, w.channel(w.kampagnen))
	withFiles := w.thread(w.kampagnen)
	withFiles.HasAttachments = true
	w.allow(t, w.agent(w.texter), authorize.ActionMessageCreate, withFiles)

	w.setChannel(t, w.kampagnen, func(c *authz.ChannelFacts) { c.EffectiveOperatorID = w.runner })
	w.expect(t, w.agent(w.texter), authorize.ActionFileUpload, w.channel(w.kampagnen), authorize.OutcomeNotFound, "no_grant")
	w.allow(t, w.agent(w.runner), authorize.ActionFileUpload, w.channel(w.kampagnen))
	w.allow(t, w.agent(w.runner), authorize.ActionMessageCreate, withFiles)
}
