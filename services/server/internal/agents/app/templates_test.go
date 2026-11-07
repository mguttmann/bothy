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

// failingAuthorizer fails every check with err.
type failingAuthorizer struct {
	allowAgents
	err error
}

func (z failingAuthorizer) Authorize(context.Context, authorize.Principal, authorize.Action, authorize.Resource) (authorize.Decision, error) {
	return authorize.Decision{}, z.err
}

// TestListTemplatesErrors: an authorize error is returned unchanged, and a caller that is not
// the principal of the request is an internal error; neither returns the catalogue.
func TestListTemplatesErrors(t *testing.T) {
	t.Parallel()
	items := []agents.Template{{Slug: "notes", Source: agents.TemplateShipped, Definition: json.RawMessage(`{}`)}}
	boom := errors.New("authorize down")
	svc, err := app.NewTemplates(failingAuthorizer{err: boom}, items)
	if err != nil {
		t.Fatal(err)
	}
	caller := actor.Actor{ID: ids.New(), Kind: actor.KindUser}
	if got, err := svc.ListTemplates(context.Background(), caller); !errors.Is(err, boom) || got != nil {
		t.Fatalf("authorize error: %+v, %v", got, err)
	}
	ctx := actor.WithActor(context.Background(), actor.Actor{ID: ids.New(), Kind: actor.KindUser})
	got, err := svc.ListTemplates(ctx, caller)
	var e *errs.Error
	if !errors.As(err, &e) || e.Code != errs.CodeInternalError || got != nil {
		t.Fatalf("other principal: %+v, %v", got, err)
	}
}

// TestTemplatesAsInterviewCatalogue: the templates are the catalogue of the interview (26 7.4):
// every template in its order with slug, name, description, the tool class wish and the MCP
// servers of its definition; a template without servers carries an empty list, and a definition
// that is no stored definition is an error.
func TestTemplatesAsInterviewCatalogue(t *testing.T) {
	t.Parallel()
	var _ app.CatalogueSource = (*app.TemplateService)(nil)
	items := []agents.Template{
		{Slug: "search", Name: "Search", Description: "Searches", Source: agents.TemplateShipped, Definition: json.RawMessage(
			`{"instructions":"x","skills":[],"mcp_servers":[{"name":"s","transport":"http","url":"https://search.example.com/mcp",` +
				`"headers":[{"name":"Authorization","secret_ref":"env:SEARCH_MCP_TOKEN"}]}],"settings":{"tool_class_wish":"scoped"},"origin":{"kind":"template"}}`)},
		{Slug: "notes", Name: "Notes", Description: "Takes notes", Source: agents.TemplatePrivate, Definition: json.RawMessage(
			`{"instructions":"x","skills":[],"mcp_servers":[],"settings":{"tool_class_wish":"chat"},"origin":{"kind":"template"}}`)},
	}
	svc, err := app.NewTemplates(&templateAuthorizer{}, items)
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.InterviewCatalogue(context.Background())
	if err != nil || len(got) != 2 {
		t.Fatalf("catalogue %+v, %v", got, err)
	}
	s := got[0]
	if s.Slug != "search" || s.Name != "Search" || s.Description != "Searches" || s.ToolClassWish != "scoped" || len(s.MCPServers) != 1 ||
		s.MCPServers[0].URL != "https://search.example.com/mcp" || len(s.MCPServers[0].Headers) != 1 ||
		s.MCPServers[0].Headers[0].SecretRef != "env:SEARCH_MCP_TOKEN" {
		t.Fatalf("first entry %+v", s)
	}
	if n := got[1]; n.Slug != "notes" || n.ToolClassWish != "chat" || n.MCPServers == nil || len(n.MCPServers) != 0 {
		t.Fatalf("second entry %+v", n)
	}
	bad, err := app.NewTemplates(&templateAuthorizer{}, []agents.Template{{Slug: "bad", Definition: json.RawMessage(`[`)}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bad.InterviewCatalogue(context.Background()); err == nil {
		t.Fatal("a definition that is no stored definition")
	}
}
