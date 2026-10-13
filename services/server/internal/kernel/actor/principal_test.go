// SPDX-License-Identifier: Apache-2.0

package actor_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Compile-time check: the request principal is what the Authorizer reads (M1-28).
var _ authorize.Principal = actor.Actor{}

func TestActorAsAuthorizePrincipal(t *testing.T) {
	t.Parallel()
	step := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	a := actor.Actor{
		ID: ids.New(), Kind: actor.KindApp, SessionID: ids.New(), DeviceID: ids.New(), StepUpAt: step,
		TokenID: ids.New(), InstallationID: ids.New(), GrantedScopes: []string{"messages:read"},
		ViewAs: ids.New(), ActingFor: actor.ActingFor{UserID: ids.New(), RunID: ids.New(), ThreadID: ids.New(), JTI: "j"},
	}
	var p authorize.Principal = a
	if p.PrincipalID() != a.ID || p.PrincipalKind() != "app" || p.SessionRef() != a.SessionID ||
		p.DeviceRef() != a.DeviceID || p.TokenRef() != a.TokenID || p.InstallationRef() != a.InstallationID ||
		!p.StepUpTime().Equal(step) || len(p.AppGrantedScopes()) != 1 || p.ViewAsPerson() != a.ViewAs ||
		p.ActingForPerson() != a.ActingFor.UserID {
		t.Fatalf("principal view differs from %+v", a)
	}
	if !(actor.ActingFor{}).IsZero() || a.ActingFor.IsZero() {
		t.Fatal("ActingFor.IsZero")
	}
	got, _ := actor.FromContext(actor.WithActor(context.Background(), a))
	a.GrantedScopes[0] = "changed"
	if got.GrantedScopes[0] != "messages:read" || got.ActingFor != a.ActingFor || got.ViewAs != a.ViewAs {
		t.Fatalf("context copy shares or loses fields: %+v", got)
	}
}
