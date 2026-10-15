// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	mw "github.com/OWNER/bothy/services/server/test/messagingworld"
)

// Paths of the template operations.
const myTemplates = "/api/v1/me/templates"

func channelTemplates(ch ids.ID) string { return "/api/v1/channels/" + ch.String() + "/templates" }
func templatePath(id string) string     { return "/api/v1/templates/" + id }

// templateBody returns a TemplateCreate or TemplateUpdate body.
func templateBody(name, content string) string {
	b, err := json.Marshal(map[string]string{"name": name, "content_md": content})
	if err != nil {
		panic(err)
	}
	return string(b)
}

// templateWorld is the world of the template tests: the owner, Alex with read on Support and
// none on Marketing, a text channel in each space and the agent lotse.
type templateWorld struct {
	*mw.World
	alex                       ids.ID
	support, marketing         ids.ID
	supportChan, marketingChan ids.ID
}

func newTemplateWorld(t *testing.T) templateWorld {
	t.Helper()
	w := mw.New(t, subscribe)
	tw := templateWorld{World: w}
	tw.support = w.Space("Support")
	tw.marketing = w.Space("Marketing")
	tw.supportChan = w.Channel(tw.support, "text", "general")
	tw.marketingChan = w.Channel(tw.marketing, "text", "general")
	tw.alex = w.Person("alex", false)
	w.Grant("user", tw.alex, tw.support, "read")
	w.Agent("lotse")
	return tw
}

// create posts a template as token name to path, expects 201 and returns the body.
func (tw templateWorld) create(name, path, tplName string) map[string]any {
	return tw.Want(tw.Do(name, http.MethodPost, path, templateBody(tplName, "Please escalate."), mw.Key()...), http.StatusCreated, "")
}

// itemNames returns the names of a TemplateList.
func itemNames(m map[string]any) []string {
	var out []string
	for _, it := range m["items"].([]any) {
		out = append(out, it.(map[string]any)["name"].(string))
	}
	return out
}

// TestListMyTemplates (brief M1-60 note 7): only the caller's personal templates, also for a
// person without any readable channel; an agent gets 403.
func TestListMyTemplates(t *testing.T) {
	t.Parallel()
	tw := newTemplateWorld(t)
	tw.Person("robin", false) // no grant at all
	tw.create("robin", myTemplates, "Robin one")
	tw.create("alex", myTemplates, "Alex one")
	tw.create("owner", channelTemplates(tw.supportChan), "Channel one")
	got := tw.Want(tw.Do("robin", http.MethodGet, myTemplates, ""), http.StatusOK, "")
	if names := itemNames(got); len(names) != 1 || names[0] != "Robin one" {
		t.Fatalf("robin %v", names)
	}
	item := got["items"].([]any)[0].(map[string]any)
	if item["scope"] != "personal" || item["channel_id"] != nil || item["version"] != float64(1) {
		t.Fatalf("item %v", item)
	}
	p := tw.Want(tw.Do("lotse", http.MethodGet, myTemplates, ""), http.StatusForbidden, "forbidden")
	if p["reason"] != "subject_kind_not_allowed" {
		t.Fatalf("agent %v", p)
	}
}

// TestCreatePersonalTemplate: 201 with scope personal, no channel_id, version 1; the replay of
// an Idempotency-Key returns the stored answer and leaves one row; an agent gets 403.
func TestCreatePersonalTemplate(t *testing.T) {
	t.Parallel()
	tw := newTemplateWorld(t)
	key := mw.Key()
	first := tw.Want(tw.Do("alex", http.MethodPost, myTemplates, templateBody("Escalation", "Please escalate."), key...), http.StatusCreated, "")
	if first["scope"] != "personal" || first["channel_id"] != nil || first["version"] != float64(1) || first["name"] != "Escalation" {
		t.Fatalf("created %v", first)
	}
	again := tw.Do("alex", http.MethodPost, myTemplates, templateBody("Escalation", "Please escalate."), key...)
	second := tw.Want(again, http.StatusCreated, "")
	if second["id"] != first["id"] {
		t.Fatalf("replay %v vs %v", second, first)
	}
	if n := tw.Count(`SELECT count(*)::int FROM message_templates`); n != 1 {
		t.Fatalf("%d rows", n)
	}
	p := tw.Want(tw.Do("lotse", http.MethodPost, myTemplates, templateBody("Bot", "x"), mw.Key()...), http.StatusForbidden, "forbidden")
	if p["reason"] != "subject_kind_not_allowed" {
		t.Fatalf("agent %v", p)
	}
	// Stored as given, without trimming.
	m := tw.Want(tw.Do("alex", http.MethodPost, myTemplates, templateBody(" Spaced ", " text "), mw.Key()...), http.StatusCreated, "")
	if m["name"] != " Spaced " || m["content_md"] != " text " {
		t.Fatalf("trimmed %v", m)
	}
}

// TestCreateTemplate_Validation: the field errors of note 8 reach the client with pointer and
// code; content above message.max_content_bytes is too_long.
func TestCreateTemplate_Validation(t *testing.T) {
	t.Parallel()
	tw := newTemplateWorld(t)
	cases := []struct{ name, content, pointer, code string }{
		{"", "x", "/name", "required"},
		{strings.Repeat("n", mw.TemplateNameMax+1), "x", "/name", "too_long"},
		{"a\u202eb", "x", "/name", "invalid_characters"},
		{"Ok", "  ", "/content_md", "required"},
		{"Ok", strings.Repeat("a", mw.MaxContent+1), "/content_md", "too_long"},
	}
	for _, c := range cases {
		p := tw.Want(tw.Do("alex", http.MethodPost, myTemplates, templateBody(c.name, c.content), mw.Key()...), http.StatusBadRequest, "validation_failed")
		if ptr, code := mw.Field(p); ptr != c.pointer || code != c.code {
			t.Errorf("%q: %s %s, want %s %s", c.name, ptr, code, c.pointer, c.code)
		}
	}
	tw.create("alex", myTemplates, strings.Repeat("n", mw.TemplateNameMax))
}

// TestCreateChannelTemplate_Rights: the owner 201; a reader without manage 403; Alex on a
// Marketing channel 404 equal to a random id. The case "DM participant 403" is recorded in
// docs/DECISIONS.md M1-60/3: dm membership does not exist before V3-01.
func TestCreateChannelTemplate_Rights(t *testing.T) {
	t.Parallel()
	tw := newTemplateWorld(t)
	m := tw.create("owner", channelTemplates(tw.supportChan), "Escalation")
	if m["scope"] != "channel" || m["channel_id"] != tw.supportChan.String() || m["version"] != float64(1) {
		t.Fatalf("created %v", m)
	}
	tw.Want(tw.Do("alex", http.MethodPost, channelTemplates(tw.supportChan), templateBody("x", "y"), mw.Key()...), http.StatusForbidden, "forbidden")
	a := tw.Do("alex", http.MethodPost, channelTemplates(tw.marketingChan), templateBody("x", "y"), mw.Key()...)
	b := tw.Do("alex", http.MethodPost, channelTemplates(ids.New()), templateBody("x", "y"), mw.Key()...)
	mw.Same404(t, a, b)
	tw.Want(tw.Do("lotse", http.MethodPost, channelTemplates(tw.supportChan), templateBody("x", "y"), mw.Key()...), http.StatusNotFound, "not_found")
}

// TestTemplateLimitExactUnderConcurrency (note 9): parallel creates in one scope stop exactly at
// template.max_per_scope; the next one gets limit_reached with an empty pointer.
func TestTemplateLimitExactUnderConcurrency(t *testing.T) {
	t.Parallel()
	tw := newTemplateWorld(t)
	const n = mw.TemplateMax * 3
	var wg sync.WaitGroup
	gate := make(chan struct{})
	codes := make([]int, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-gate
			r := tw.H.Do(tw.H.NewRequest(http.MethodPost, channelTemplates(tw.supportChan), templateBody("T"+strconv.Itoa(i), "x"),
				"Authorization", "Bearer owner", "Idempotency-Key", ids.New().String()))
			codes[i] = r.StatusCode
		}()
	}
	close(gate)
	wg.Wait()
	created := 0
	for _, c := range codes {
		switch c {
		case http.StatusCreated:
			created++
		case http.StatusBadRequest:
		default:
			t.Errorf("status %d", c)
		}
	}
	if created != mw.TemplateMax {
		t.Fatalf("%d created, want %d", created, mw.TemplateMax)
	}
	if got := tw.Count(`SELECT count(*)::int FROM message_templates WHERE channel_id = $1`, tw.supportChan.String()); got != mw.TemplateMax {
		t.Fatalf("%d rows", got)
	}
	p := tw.Want(tw.Do("owner", http.MethodPost, channelTemplates(tw.supportChan), templateBody("late", "x"), mw.Key()...), http.StatusBadRequest, "validation_failed")
	if ptr, code := mw.Field(p); ptr != "" || code != "limit_reached" || !strings.Contains(p["detail"].(string), "template.max_per_scope") {
		t.Fatalf("limit %v", p)
	}
	// The personal scope of the owner is a separate bound.
	tw.create("owner", myTemplates, "mine")
}

// TestListTemplates (note 7): own personal plus channel templates; never another user's
// personal ones; deleted ones excluded; ordered by lower(name), then id; an invisible channel
// 404.
func TestListTemplates(t *testing.T) {
	t.Parallel()
	tw := newTemplateWorld(t)
	tw.create("owner", channelTemplates(tw.supportChan), "beta")
	tw.create("owner", channelTemplates(tw.supportChan), "Alpha")
	gone := tw.create("owner", channelTemplates(tw.supportChan), "gone")
	tw.create("owner", channelTemplates(tw.marketingChan), "Marketing only")
	tw.create("alex", myTemplates, "Charlie")
	tw.create("owner", myTemplates, "Owner private")
	tw.Want(tw.Do("owner", http.MethodDelete, templatePath(gone["id"].(string)), ""), http.StatusNoContent, "")
	got := itemNames(tw.Want(tw.Do("alex", http.MethodGet, channelTemplates(tw.supportChan), ""), http.StatusOK, ""))
	if strings.Join(got, ",") != "Alpha,beta,Charlie" {
		t.Fatalf("alex sees %v", got)
	}
	a := tw.Do("alex", http.MethodGet, channelTemplates(tw.marketingChan), "")
	b := tw.Do("alex", http.MethodGet, channelTemplates(ids.New()), "")
	mw.Same404(t, a, b)
}

// TestUpdateTemplate (note 10): 428 without If-Match; 412 with current; 200 raises the version
// and returns the ETag; another user's personal template 404 (also for the owner); a channel
// template without manage 403; a deleted template 404; a no-op keeps the version.
func TestUpdateTemplate(t *testing.T) {
	t.Parallel()
	tw := newTemplateWorld(t)
	mine := tw.create("alex", myTemplates, "Mine")
	id := mine["id"].(string)
	tw.Want(tw.Do("alex", http.MethodPatch, templatePath(id), `{"name":"New"}`), http.StatusPreconditionRequired, "precondition_required")
	p := tw.Want(tw.Do("alex", http.MethodPatch, templatePath(id), `{"name":"New"}`, "If-Match", `"9"`), http.StatusPreconditionFailed, "version_conflict")
	if cur, _ := p["current"].(map[string]any); cur["version"] != float64(1) || cur["name"] != "Mine" {
		t.Fatalf("current %v", p["current"])
	}
	r := tw.Do("alex", http.MethodPatch, templatePath(id), `{"name":"New"}`, "If-Match", `"1"`)
	m := tw.Want(r, http.StatusOK, "")
	if m["version"] != float64(2) || m["name"] != "New" || r.Header.Get("ETag") != `"2"` {
		t.Fatalf("updated %v %q", m, r.Header.Get("ETag"))
	}
	// No-ops: the same values and an empty body keep the version.
	for _, body := range []string{`{"name":"New"}`, `{}`} {
		r := tw.Do("alex", http.MethodPatch, templatePath(id), body, "If-Match", `"2"`)
		if m := tw.Want(r, http.StatusOK, ""); m["version"] != float64(2) || r.Header.Get("ETag") != `"2"` {
			t.Fatalf("no-op %s: %v", body, m)
		}
	}
	tw.Want(tw.Do("alex", http.MethodPatch, templatePath(id), `{"name":""}`, "If-Match", `"2"`), http.StatusBadRequest, "validation_failed")
	// Another user's personal template, also for the owner: the same 404 as an unknown id.
	a := tw.Do("owner", http.MethodPatch, templatePath(id), `{"name":"x"}`, "If-Match", `"2"`)
	b := tw.Do("owner", http.MethodPatch, templatePath(ids.New().String()), `{"name":"x"}`, "If-Match", `"2"`)
	mw.Same404(t, a, b)
	ch := tw.create("owner", channelTemplates(tw.supportChan), "Channel")
	tw.Want(tw.Do("alex", http.MethodPatch, templatePath(ch["id"].(string)), `{"name":"x"}`, "If-Match", `"1"`), http.StatusForbidden, "forbidden")
	mk := tw.create("owner", channelTemplates(tw.marketingChan), "Marketing")
	tw.Want(tw.Do("alex", http.MethodPatch, templatePath(mk["id"].(string)), `{"name":"x"}`, "If-Match", `"1"`), http.StatusNotFound, "not_found")
	tw.Want(tw.Do("owner", http.MethodPatch, templatePath(ch["id"].(string)), `{"content_md":"Changed."}`, "If-Match", `"1"`), http.StatusOK, "")
	tw.Want(tw.Do("alex", http.MethodDelete, templatePath(id), ""), http.StatusNoContent, "")
	tw.Want(tw.Do("alex", http.MethodPatch, templatePath(id), `{"name":"x"}`, "If-Match", `"3"`), http.StatusNotFound, "not_found")
}

// TestDeleteTemplate (note 11): 204; a second call 204 without write; another user's personal
// template 404; a channel template without manage 403.
func TestDeleteTemplate(t *testing.T) {
	t.Parallel()
	tw := newTemplateWorld(t)
	mine := tw.create("alex", myTemplates, "Mine")
	id := mine["id"].(string)
	a := tw.Do("owner", http.MethodDelete, templatePath(id), "")
	b := tw.Do("owner", http.MethodDelete, templatePath(ids.New().String()), "")
	mw.Same404(t, a, b)
	tw.Want(tw.Do("alex", http.MethodDelete, templatePath(id), ""), http.StatusNoContent, "")
	tw.Want(tw.Do("alex", http.MethodDelete, templatePath(id), ""), http.StatusNoContent, "")
	if v := tw.Count(`SELECT version::int FROM message_templates WHERE id = $1`, id); v != 2 {
		t.Fatalf("version %d after two deletes", v)
	}
	ch := tw.create("owner", channelTemplates(tw.supportChan), "Channel")
	tw.Want(tw.Do("alex", http.MethodDelete, templatePath(ch["id"].(string)), ""), http.StatusForbidden, "forbidden")
	tw.Want(tw.Do("owner", http.MethodDelete, templatePath(ch["id"].(string)), ""), http.StatusNoContent, "")
	got := itemNames(tw.Want(tw.Do("alex", http.MethodGet, myTemplates, ""), http.StatusOK, ""))
	if len(got) != 0 {
		t.Fatalf("deleted still listed: %v", got)
	}
}

// TestTemplates_NoEvents (note 12): no template operation writes an event_log row.
func TestTemplates_NoEvents(t *testing.T) {
	t.Parallel()
	tw := newTemplateWorld(t)
	before := len(tw.Events())
	mine := tw.create("alex", myTemplates, "Mine")
	ch := tw.create("owner", channelTemplates(tw.supportChan), "Channel")
	tw.Want(tw.Do("alex", http.MethodPatch, templatePath(mine["id"].(string)), `{"name":"x"}`, "If-Match", `"1"`), http.StatusOK, "")
	tw.Want(tw.Do("owner", http.MethodPatch, templatePath(ch["id"].(string)), `{"name":"x"}`, "If-Match", `"1"`), http.StatusOK, "")
	tw.Want(tw.Do("alex", http.MethodGet, myTemplates, ""), http.StatusOK, "")
	tw.Want(tw.Do("alex", http.MethodGet, channelTemplates(tw.supportChan), ""), http.StatusOK, "")
	tw.Want(tw.Do("alex", http.MethodDelete, templatePath(mine["id"].(string)), ""), http.StatusNoContent, "")
	tw.Want(tw.Do("owner", http.MethodDelete, templatePath(ch["id"].(string)), ""), http.StatusNoContent, "")
	if after := len(tw.Events()); after != before {
		t.Fatalf("%d events written", after-before)
	}
}

// TestTemplates_ReadOnlyChannelAllowsManage (note 13, 05 rule 1.8): on a read-only channel the
// owner with manage creates, changes and deletes channel templates and a reader lists them.
func TestTemplates_ReadOnlyChannelAllowsManage(t *testing.T) {
	t.Parallel()
	tw := newTemplateWorld(t)
	tw.SQL(`UPDATE channels SET read_only = true, version = version + 1 WHERE id = $1`, tw.supportChan.String())
	ch := tw.create("owner", channelTemplates(tw.supportChan), "Read only")
	id := ch["id"].(string)
	tw.Want(tw.Do("owner", http.MethodPatch, templatePath(id), `{"name":"Still"}`, "If-Match", `"1"`), http.StatusOK, "")
	if got := itemNames(tw.Want(tw.Do("alex", http.MethodGet, channelTemplates(tw.supportChan), ""), http.StatusOK, "")); len(got) != 1 {
		t.Fatalf("reader sees %v", got)
	}
	tw.Want(tw.Do("owner", http.MethodDelete, templatePath(id), ""), http.StatusNoContent, "")
}
