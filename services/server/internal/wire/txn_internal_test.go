// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// TestRenderTxnRegistrations: hooks before subscriptions, hooks in phase order, subscriptions
// in registration order, one line each in the format of the golden file.
func TestRenderTxnRegistrations(t *testing.T) {
	t.Parallel()
	got := renderTxnRegistrations([]txn.Registration{
		{Kind: txn.KindSubscribe, Event: "chat.ThreadCreated", Publisher: "chat", Subscriber: "runs", Handler: "r1"},
		{Kind: txn.KindHook, Phase: txn.PhaseAudit, Subscriber: "audit", Handler: "a"},
		{Kind: txn.KindSubscribe, Event: "kernel.AccessChanged", Publisher: "kernel", Subscriber: "agents", Handler: "g1"},
		{Kind: txn.KindHook, Phase: txn.PhaseRightsBegin, Subscriber: "authz", Handler: "b"},
		{Kind: txn.KindHook, Phase: txn.PhaseSync, Subscriber: "sync", Handler: "s"},
	})
	want := txnGoldenHeader + "\n" +
		"hook rights_begin authz b\n" +
		"hook sync sync s\n" +
		"hook audit audit a\n" +
		"subscribe chat.ThreadCreated -> runs r1\n" +
		"subscribe kernel.AccessChanged -> agents g1\n"
	if got != want {
		t.Fatalf("rendering:\n%s\nwant:\n%s", got, want)
	}
	if empty := renderTxnRegistrations(nil); empty != txnGoldenHeader+"\n" {
		t.Fatalf("empty rendering = %q", empty)
	}
}
