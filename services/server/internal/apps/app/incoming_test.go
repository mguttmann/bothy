// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"testing"

	appsapp "github.com/OWNER/bothy/services/server/internal/apps/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestWebhookActor_ScopesLimitedToMessagesWrite (D-M2-27-3, red team "exceeding the scopes of an
// app", T-TK-03): for an installation granted messages:write and messages:read, the webhook actor
// carries messages:write only, so message.read is forbidden scope_missing while message.create is
// allowed; the same principal with both scopes may read (the control that the limit decides).
func TestWebhookActor_ScopesLimitedToMessagesWrite(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	o := w.RobinPhone.StepUp()
	granted := []string{"messages:read", "messages:write"}
	app := w.Install(o, manifest("tracker", "1.0.0", []string{"messages:write"}, []string{"messages:read"}), granted)
	appID := ids.MustParse(app["id"].(string))
	inst := ids.MustParse(app["installation"].(map[string]any)["id"].(string))
	w.Grant(appID, w.Eskalation, "write")

	a := appsapp.WebhookActor(appID, inst, granted)
	if len(a.GrantedScopes) != 1 || a.GrantedScopes[0] != "messages:write" || !a.TokenID.IsZero() || !a.SessionID.IsZero() {
		t.Fatalf("webhook actor %+v", a)
	}
	if d := w.Decide(a, authorize.ActionMessageRead, messageIn(w.EskalationThread)); d.Outcome != authorize.OutcomeForbidden || d.Reason != "scope_missing" {
		t.Fatalf("message.read: %+v, want forbidden scope_missing", d)
	}
	if d := w.Decide(a, authorize.ActionMessageCreate, threadOf(w.EskalationThread)); d.Outcome != authorize.OutcomeAllow {
		t.Fatalf("message.create: %+v, want allow", d)
	}
	full := a
	full.GrantedScopes = granted
	if d := w.Decide(full, authorize.ActionMessageRead, messageIn(w.EskalationThread)); d.Outcome != authorize.OutcomeAllow {
		t.Fatalf("control message.read with both scopes: %+v", d)
	}
}
