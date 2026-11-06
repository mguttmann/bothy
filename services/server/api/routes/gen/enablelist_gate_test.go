// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
)

// The gate check of the enable list (V1-01, plan decision P-04): every operation under paths of
// docs/api/openapi.yaml and every client op of the channel gateway of docs/api/asyncapi.yaml is
// enabled or stands under later with milestone and reason, and later holds only operations of
// features that docs/spec/01-product.md section 3 lists with the milestone "later". Such a feature
// comes after the core milestones and V1, so its milestone is V2 or V3.

// realProduct is docs/spec/01-product.md, relative to this package.
const realProduct = "../../../../../docs/spec/01-product.md"

// laterMilestones are the milestones a later entry may name once the enable list is complete:
// the versions after the switchover V1.
var laterMilestones = []string{"V2", "V3"}

// productFeatures reads the feature tables of section 3 of the product chapter at path and
// returns the column "Milestone" per feature (core, later or never).
func productFeatures(t *testing.T, path string) map[string]string {
	t.Helper()
	f, err := os.Open(path) // #nosec G304 -- a fixed path of the repository or a test file
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	out := map[string]string{}
	inSection := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "## ") {
			inSection = strings.HasPrefix(line, "## 3. ")
			continue
		}
		if !inSection || !strings.HasPrefix(line, "| ") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		if len(cells) < 2 {
			continue
		}
		feature, milestone := strings.TrimSpace(cells[0]), strings.TrimSpace(cells[1])
		if feature == "Feature" || strings.HasPrefix(milestone, "---") {
			continue
		}
		out[feature] = milestone
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// laterGateProblems checks the later entries of the REST section and of gateway_ops against the
// features of the product chapter: the feature is a row of section 3 with the milestone "later",
// the milestone is one of laterMilestones and the reason is not blank.
func laterGateProblems(list *enableList, features map[string]string) []string {
	type entry struct{ where, id, milestone, reason, feature string }
	var entries []entry
	for _, e := range list.Later {
		entries = append(entries, entry{sectionLater, e.OperationID, e.Milestone, e.Reason, e.Feature})
	}
	if list.GatewayOps != nil {
		for _, e := range list.GatewayOps.Later {
			entries = append(entries, entry{sectionGatewayOps + "." + sectionLater, e.Op, e.Milestone, e.Reason, e.Feature})
		}
	}
	var out []string
	for _, e := range entries {
		at := fmt.Sprintf("%s: %s", e.where, e.id)
		if strings.TrimSpace(e.reason) == "" {
			out = append(out, at+": missing reason")
		}
		if !slices.Contains(laterMilestones, e.milestone) {
			out = append(out, fmt.Sprintf("%s: milestone %q is a core milestone; build the operation or plan it for %s",
				at, e.milestone, strings.Join(laterMilestones, " or ")))
		}
		switch m, ok := features[e.feature]; {
		case !ok:
			out = append(out, fmt.Sprintf("%s: feature %q is no row of docs/spec/01-product.md section 3", at, e.feature))
		case m != "later":
			out = append(out, fmt.Sprintf("%s: feature %q has the milestone %q in docs/spec/01-product.md, not later", at, e.feature, m))
		}
	}
	return out
}

// TestEnableListComplete is the gate check of V1-01 on the real files.
func TestEnableListComplete(t *testing.T) {
	in, err := realInputs()
	if err != nil {
		t.Fatalf("an operation of openapi.yaml is neither enabled nor later, or the list breaks a rule: %v", err)
	}
	if _, err := loadGatewayOps(in, "../../../../../docs/api/openapi.yaml", "../../operations.yaml", realAsyncAPI); err != nil {
		t.Fatalf("a gateway op of asyncapi.yaml is neither enabled nor later, or gateway_ops breaks a rule: %v", err)
	}
	p := &problemsError{}
	list := readList("../../operations.yaml", p)
	if err := p.err(); err != nil || list == nil {
		t.Fatalf("operations.yaml: %v", err)
	}
	features := productFeatures(t, realProduct)
	if features["Direct messages between people"] != "later" {
		t.Fatalf("the product chapter was not read: %d features", len(features))
	}
	for _, problem := range laterGateProblems(list, features) {
		t.Error(problem)
	}
}

// TestLaterGateProblems proves that the gate check refuses each defect it names.
func TestLaterGateProblems(t *testing.T) {
	features := map[string]string{"Direct messages": "later", "Webhooks": "core"}
	ok := laterEntry{OperationID: "createDm", Milestone: "V3", Reason: "after V1", Feature: "Direct messages"}
	if got := laterGateProblems(&enableList{Later: []laterEntry{ok}}, features); len(got) != 0 {
		t.Fatalf("a valid entry: %v", got)
	}
	cases := map[string]struct {
		change func(*laterEntry)
		want   string
	}{
		"core milestone":  {func(e *laterEntry) { e.Milestone = "M2" }, `milestone "M2"`},
		"v1 milestone":    {func(e *laterEntry) { e.Milestone = "V1" }, `milestone "V1"`},
		"blank reason":    {func(e *laterEntry) { e.Reason = " " }, "missing reason"},
		"unknown feature": {func(e *laterEntry) { e.Feature = "Something else" }, "no row"},
		"core feature":    {func(e *laterEntry) { e.Feature = "Webhooks" }, `milestone "core"`},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			e := ok
			c.change(&e)
			got := strings.Join(laterGateProblems(&enableList{Later: []laterEntry{e}}, features), "\n")
			if !strings.Contains(got, c.want) || !strings.Contains(got, "createDm") {
				t.Errorf("problems %q do not name %q", got, c.want)
			}
		})
	}
	gw := &enableList{GatewayOps: &gatewayOpsList{Later: []gatewayLater{{Op: "dm.focus", Milestone: "M3", Reason: "r", Feature: "Webhooks"}}}}
	got := strings.Join(laterGateProblems(gw, features), "\n")
	for _, w := range []string{"gateway_ops.later: dm.focus", `milestone "M3"`, `milestone "core"`} {
		if !strings.Contains(got, w) {
			t.Errorf("gateway problems %q do not name %q", got, w)
		}
	}
}

// TestProductFeatures reads only the tables of section 3 and skips header and separator rows.
func TestProductFeatures(t *testing.T) {
	path := t.TempDir() + "/01.md"
	writeFile(t, path, "## 2. Before\n\n| Feature | Milestone |\n|---|---|\n| Outside | later |\n\n"+
		"## 3. Feature list\n\n### 3.1 Group\n\n| Feature | Milestone | Where |\n|---|---|---|\n| Inside | later | D |\n| Core one | core | M |\n\n"+
		"## 4. After\n\n| Late | later |\n")
	got := productFeatures(t, path)
	want := map[string]string{"Inside": "later", "Core one": "core"}
	if len(got) != len(want) || got["Inside"] != "later" || got["Core one"] != "core" {
		t.Errorf("features = %v, want %v", got, want)
	}
}
