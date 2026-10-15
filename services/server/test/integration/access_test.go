// SPDX-License-Identifier: Apache-2.0

package integration_test

import (
	"net/http"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// sourcesOf returns the source kinds and agents of the entry of principal in an explanation.
func sourcesOf(t *testing.T, m map[string]any, principal ids.ID) (string, []string, []string) {
	t.Helper()
	list, _ := m["entries"].([]any)
	for _, e := range list {
		entry, _ := e.(map[string]any)
		p, _ := entry["principal"].(map[string]any)
		if p["id"] != principal.String() {
			continue
		}
		var kinds, agents []string
		srcs, _ := entry["sources"].([]any)
		for _, s := range srcs {
			src, _ := s.(map[string]any)
			k, _ := src["kind"].(string)
			a, _ := src["agent_id"].(string)
			kinds, agents = append(kinds, k), append(agents, a)
		}
		level, _ := entry["level"].(string)
		return level, kinds, agents
	}
	return "", nil, nil
}

// TestAccessExplanation_RealSources runs explainSpaceAccess, explainChannelAccess, previewGrant
// and viewAsUser of M1-49 against the real spaces, channels, operators and agents.
func TestAccessExplanation_RealSources(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	t.Run("operator source", func(t *testing.T) {
		m := w.wantOK(w.do("owner", http.MethodGet, "/api/v1/channels/"+w.kampagnen.String()+"/access", ""))
		level, kinds, agents := sourcesOf(t, m, w.texter)
		if level != "write" || !slices.Contains(kinds, "operator") || !slices.Contains(agents, w.texter.String()) {
			t.Fatalf("Texter on Kampagnen: %s %v %v", level, kinds, agents)
		}
		m = w.wantOK(w.do("owner", http.MethodGet, "/api/v1/spaces/"+w.research.String()+"/access", ""))
		if _, kinds, _ := sourcesOf(t, m, w.assistent); !slices.Contains(kinds, "operator") {
			t.Fatalf("Assistent on Research: %v", kinds)
		}
	})
	t.Run("via agent", func(t *testing.T) {
		// Texter reads Support/Eskalation, which Sam (who triggers Texter through G-2) cannot read:
		// hint H1 of 07 section 10.2. The brief names a personal agent here; a personal agent is
		// triggered only by its holder, so the case needs an organization agent
		// (docs/DECISIONS.md M1-54/6).
		w.grant(`"subject_kind":"agent","subject_id":"` + w.texter.String() + `","target_kind":"channel","target_id":"` + w.eskalation.String() +
			`","level":"read","reason":"Escalations"`)
		m := w.wantOK(w.do("owner", http.MethodGet, "/api/v1/channels/"+w.eskalation.String()+"/access", ""))
		level, kinds, agents := sourcesOf(t, m, w.sam)
		if level != "none" || !slices.Equal(kinds, []string{"via_agent"}) || !slices.Equal(agents, []string{w.texter.String()}) {
			t.Fatalf("Sam on Eskalation: %s %v %v", level, kinds, agents)
		}
	})
	t.Run("preview", func(t *testing.T) {
		m := w.wantOK(w.do("owner", http.MethodPost, "/api/v1/grants/preview", `{"subject_kind":"user","subject_id":"`+w.sam.String()+
			`","target_kind":"space","target_id":"`+w.research.String()+`","level":"trigger","reason":"Research"}`))
		list, _ := m["invariant_violations"].([]any)
		if m["allowed"] != false || len(list) != 1 {
			t.Fatalf("preview %v", m)
		}
		v, _ := list[0].(map[string]any)
		if v["code"] != "i1_violation" || v["agent_id"] != w.assistent.String() || !slices.Equal(strs(v["principal_ids"]), []string{w.sam.String()}) {
			t.Fatalf("violation %v", v)
		}
	})
	t.Run("view as", func(t *testing.T) {
		m := w.wantOK(w.do("owner", http.MethodGet, "/api/v1/users/"+w.sam.String()+"/view-as", ""))
		var got []string
		for _, c := range m["channels"].([]any) {
			got = append(got, c.(map[string]any)["channel"].(map[string]any)["id"].(string))
		}
		want := []string{w.allgemein.String(), w.kampagnen.String()}
		slices.Sort(got)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Fatalf("view as Sam: %v, want %v", got, want)
		}
	})
}
