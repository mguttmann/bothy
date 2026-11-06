// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/agents/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// templateAuthorizer allows agent.templates.read on the instance for owner only.
type templateAuthorizer struct {
	allowAgents
	owner ids.ID
	calls []authorize.Action
}

func (z *templateAuthorizer) Authorize(_ context.Context, p authorize.Principal, a authorize.Action, r authorize.Resource) (authorize.Decision, error) {
	z.calls = append(z.calls, a)
	if pa, ok := p.(actor.Actor); ok && pa.ID == z.owner && a == authorize.ActionAgentTemplatesRead && r.Kind == authorize.ResourceInstance {
		return authorize.Decision{Outcome: authorize.OutcomeAllow}, nil
	}
	return authorize.Decision{Outcome: authorize.OutcomeForbidden, Reason: "owner_only"}, nil
}

// TestListTemplates: the owner reads the catalogue in the order it was loaded; any other caller
// gets the refusal of authorize, 403 forbidden (07 2.13, BA-T04 templates part).
func TestListTemplates(t *testing.T) {
	t.Parallel()
	owner, other := ids.New(), ids.New()
	az := &templateAuthorizer{owner: owner}
	items := []agents.Template{
		{Slug: "notes", Source: agents.TemplateShipped, Definition: json.RawMessage(`{}`)},
		{Slug: "own", Source: agents.TemplatePrivate, Definition: json.RawMessage(`{}`)},
	}
	svc, err := app.NewTemplates(az, items)
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.ListTemplates(context.Background(), actor.Actor{ID: owner, Kind: actor.KindUser})
	if err != nil || len(got) != 2 || got[0].Slug != "notes" || got[1].Source != agents.TemplatePrivate {
		t.Fatalf("owner: %+v, %v", got, err)
	}
	got[0].Slug = "changed"
	again, _ := svc.ListTemplates(context.Background(), actor.Actor{ID: owner, Kind: actor.KindUser})
	if again[0].Slug != "notes" {
		t.Fatal("a caller changed the catalogue")
	}
	_, err = svc.ListTemplates(context.Background(), actor.Actor{ID: other, Kind: actor.KindUser})
	var e *errs.Error
	if !errors.As(err, &e) || e.Code != errs.CodeForbidden {
		t.Fatalf("other: %v", err)
	}
	for _, a := range az.calls {
		if a != authorize.ActionAgentTemplatesRead {
			t.Fatalf("authorized %s", a)
		}
	}
	if _, err := app.NewTemplates(nil, items); err == nil {
		t.Fatal("NewTemplates without an Authorizer")
	}
}
