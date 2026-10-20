// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	mw "github.com/OWNER/bothy/services/server/test/messagingworld"
)

// TestTemplates_UseCases (M1-60) drives the use cases of messaging.Templates directly with the
// caller in the context: the personal and channel scopes, the bound, the precedence of update
// (428 before any lookup, 404, 403, 412, 400), the no-op and the idempotent delete.
func TestTemplates_UseCases(t *testing.T) {
	t.Parallel()
	w := mw.New(t, subscribe)
	space := w.Space("Support")
	ch := w.Channel(space, "text", "general")
	alex := w.Person("alex", false)
	w.Grant("user", alex, space, "read")
	lotse := w.Agent("lotse")
	tp := w.Svc.Templates()
	asAlex := actor.WithActor(context.Background(), actor.Actor{ID: alex, Kind: actor.KindUser})
	asOwner := actor.WithActor(context.Background(), actor.Actor{ID: w.OwnerID, Kind: actor.KindUser})
	asAgent := actor.WithActor(context.Background(), actor.Actor{ID: lotse, Kind: actor.KindAgent})
	wantCode := func(err error, code errs.Code) {
		t.Helper()
		if e, ok := errs.As(err); !ok || e.Code != code {
			t.Fatalf("error %v, want %s", err, code)
		}
	}
	mine, err := tp.CreatePersonal(asAlex, messaging.TemplateInput{Name: "Mine", ContentMD: "Please escalate."})
	if err != nil || mine.Scope != "personal" || !mine.ChannelID.IsZero() || mine.Version != 1 {
		t.Fatalf("personal %+v %v", mine, err)
	}
	_, err = tp.CreatePersonal(asAlex, messaging.TemplateInput{Name: strings.Repeat("n", mw.TemplateNameMax+1), ContentMD: "x"})
	wantCode(err, errs.CodeValidationFailed)
	_, err = tp.CreatePersonal(asAgent, messaging.TemplateInput{Name: "Bot", ContentMD: "x"})
	wantCode(err, errs.CodeForbidden)
	_, err = tp.ListMine(asAgent)
	wantCode(err, errs.CodeForbidden)
	_, err = tp.CreateForChannel(asAlex, ch, messaging.TemplateInput{Name: "No", ContentMD: "x"})
	wantCode(err, errs.CodeForbidden)
	// The owner acts in a session (owner rights need one), so the owner's calls go through REST.
	path := "/api/v1/channels/" + ch.String() + "/templates"
	created := w.Want(w.Do("owner", http.MethodPost, path, `{"name":"Channel","content_md":"x"}`, mw.Key()...), http.StatusCreated, "")
	chanTpl := messaging.TemplateView{ID: ids.MustParse(created["id"].(string))}
	for i := range mw.TemplateMax - 1 {
		w.Want(w.Do("owner", http.MethodPost, path, `{"name":"C`+string(rune('a'+i))+`","content_md":"x"}`, mw.Key()...), http.StatusCreated, "")
	}
	list, err := tp.List(asAlex, ch)
	if err != nil || len(list) != mw.TemplateMax+1 {
		t.Fatalf("list %d %v", len(list), err)
	}
	if agentList, err := tp.List(asAgent, ch); err == nil {
		t.Fatalf("an agent without read listed %v", agentList)
	}
	if own, err := tp.ListMine(asAlex); err != nil || len(own) != 1 || own[0].ID != mine.ID {
		t.Fatalf("own %+v %v", own, err)
	}
	name, empty := "New", ""
	_, err = tp.Update(asAlex, messaging.TemplateChange{TemplateID: mine.ID, Name: &name})
	wantCode(err, errs.CodePreconditionRequired)
	_, err = tp.Update(asAlex, messaging.TemplateChange{TemplateID: ids.New(), Name: &name, IfMatch: messaging.Precondition{Present: true, Valid: true, Version: 1}})
	wantCode(err, errs.CodeNotFound)
	_, err = tp.Update(asAlex, messaging.TemplateChange{TemplateID: chanTpl.ID, Name: &name, IfMatch: messaging.Precondition{Present: true, Valid: true, Version: 1}})
	wantCode(err, errs.CodeForbidden)
	_, err = tp.Update(asAlex, messaging.TemplateChange{TemplateID: mine.ID, Name: &name, IfMatch: messaging.Precondition{Present: true}})
	wantCode(err, errs.CodeVersionConflict)
	_, err = tp.Update(asAlex, messaging.TemplateChange{TemplateID: mine.ID, Name: &empty, IfMatch: messaging.Precondition{Present: true, Valid: true, Version: 1}})
	wantCode(err, errs.CodeValidationFailed)
	same, err := tp.Update(asAlex, messaging.TemplateChange{TemplateID: mine.ID, IfMatch: messaging.Precondition{Present: true, Valid: true, Version: 1}})
	if err != nil || same.Version != 1 {
		t.Fatalf("no-op %+v %v", same, err)
	}
	up, err := tp.Update(asAlex, messaging.TemplateChange{TemplateID: mine.ID, Name: &name, IfMatch: messaging.Precondition{Present: true, Valid: true, Version: 1}})
	if err != nil || up.Version != 2 || up.Name != "New" {
		t.Fatalf("update %+v %v", up, err)
	}
	wantCode(tp.Delete(asOwner, mine.ID), errs.CodeNotFound)
	wantCode(tp.Delete(asAlex, chanTpl.ID), errs.CodeForbidden)
	for range 2 {
		if err := tp.Delete(asAlex, mine.ID); err != nil {
			t.Fatal(err)
		}
	}
	if n := w.Count(`SELECT version::int FROM message_templates WHERE id = $1`, mine.ID.String()); n != 3 {
		t.Fatalf("version %d", n)
	}
	// The bound: the next create is limit_reached.
	p := w.Want(w.Do("owner", http.MethodPost, path, `{"name":"x","content_md":"y"}`, mw.Key()...), http.StatusBadRequest, "validation_failed")
	if ptr, code := mw.Field(p); ptr != "" || code != "limit_reached" {
		t.Fatalf("limit %s %s", ptr, code)
	}
}
