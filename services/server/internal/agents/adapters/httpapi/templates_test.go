// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"net/http"
	"testing"
)

// templatesPath is listAgentTemplates.
const templatesPath = "/api/v1/agent-templates"

// TestListAgentTemplatesOwnerOnly is the templates part of BA-T04 (A1-06): an owner lists the
// shipped templates with their drafts, without step-up; a person who is no owner and an owner's
// personal token (SR5: no owner role) get 403 forbidden, an anonymous caller 401, and the refusal
// is audited as deny.
func TestListAgentTemplatesOwnerOnly(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	owner := w.person("owner", "active", true)
	w.person("sam", "active", false)
	w.personalToken("pat", owner)
	body := w.want(w.do("owner", http.MethodGet, templatesPath, ""), http.StatusOK, "")
	items, _ := body["items"].([]any)
	var slugs []string
	for _, it := range items {
		m, _ := it.(map[string]any)
		slugs = append(slugs, m["slug"].(string))
		if m["source"] != "shipped" {
			t.Errorf("%v: source %v", m["slug"], m["source"])
		}
		def, _ := m["definition"].(map[string]any)
		origin, _ := def["origin"].(map[string]any)
		if origin["kind"] != "template" || origin["template_slug"] != m["slug"] {
			t.Errorf("%v: origin %v", m["slug"], origin)
		}
	}
	if len(slugs) != 4 || slugs[0] != "code-reviewer" || slugs[3] != "writing-helper" {
		t.Fatalf("slugs %v", slugs)
	}
	w.want(w.do("sam", http.MethodGet, templatesPath, ""), http.StatusForbidden, "forbidden")
	w.want(w.do("pat", http.MethodGet, templatesPath, ""), http.StatusForbidden, "forbidden")
	w.want(w.do("", http.MethodGet, templatesPath, ""), http.StatusUnauthorized, "")
	var denies, allows int
	for _, r := range w.audits("agent.templates.read") {
		switch r.Decision {
		case "deny":
			denies++
		case "allow":
			allows++
		}
	}
	if denies < 1 || allows != 0 {
		t.Fatalf("audit: %d deny, %d allow entries; want denies and no allow (the read is not audited)", denies, allows)
	}
}
