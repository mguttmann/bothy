// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
)

// readGolden returns the non-comment, non-empty lines of testdata/name, sorted.
func readGolden(t *testing.T, name string) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name)) //nolint:gosec // G304: a fixed golden file of this package
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, l := range strings.Split(string(b), "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
			out = append(out, l)
		}
	}
	slices.Sort(out)
	return out
}

func TestRequirementsCoverCatalogueExactly(t *testing.T) {
	t.Parallel()
	ids := domain.ActionIDs()
	have := domain.RequirementIDs()
	if !slices.Equal(ids, have) {
		for _, id := range ids {
			if !slices.Contains(have, id) {
				t.Errorf("no requirement entry for %s", id)
			}
		}
		for _, id := range have {
			if !slices.Contains(ids, id) {
				t.Errorf("requirement entry %s is not in the catalogue", id)
			}
		}
	}
}

func TestRequirementSourceMatchesCatalogue(t *testing.T) {
	t.Parallel()
	for _, id := range domain.ActionIDs() {
		spec, _ := domain.LookupAction(id)
		req, ok := domain.RequirementOf(id)
		if !ok {
			t.Errorf("%s: no entry", id)
			continue
		}
		if req.Source != spec.Right {
			t.Errorf("%s: source %q, but 05 says %q; review the expression", id, req.Source, spec.Right)
		}
		if spec.System != req.Expr.IsSystem() {
			t.Errorf("%s: system %v, entry system %v", id, spec.System, req.Expr.IsSystem())
		}
	}
}

func TestDeferredActionsGolden(t *testing.T) {
	t.Parallel()
	want := readGolden(t, "deferred_actions.golden")
	got := domain.DeferredActions()
	if !slices.Equal(got, want) {
		t.Fatalf("deferred actions\n got %v\nwant %v", got, want)
	}
	for _, id := range got {
		req, _ := domain.RequirementOf(id)
		if e, ok := req.Expr.DeferredTo(); !ok || e == "" {
			t.Errorf("%s: deferred without a milestone", id)
		}
	}
}

func TestScopeTableActionsExist(t *testing.T) {
	t.Parallel()
	for _, id := range domain.ScopeTableActions() {
		if _, ok := domain.LookupAction(id); !ok {
			t.Errorf("scope table names %s, which is not in the catalogue", id)
		}
	}
	// The table agrees with the AppScope column the catalogue carries (05 section 2.6).
	for _, id := range domain.ActionIDs() {
		spec, _ := domain.LookupAction(id)
		scope, listed := domain.AppScopeOf(id)
		switch spec.AppScope {
		case "":
			if listed {
				t.Errorf("%s: table gives %q, the catalogue no scope row", id, scope)
			}
		case "none":
			if !listed || scope != "" {
				t.Errorf("%s: catalogue says no scope needed, table %q %v", id, scope, listed)
			}
		default:
			if !listed || scope != spec.AppScope {
				t.Errorf("%s: catalogue %q, table %q %v", id, spec.AppScope, scope, listed)
			}
		}
	}
}

func TestWritesSetGolden(t *testing.T) {
	t.Parallel()
	var got []string
	for _, id := range domain.ActionIDs() {
		if domain.Writes(id) {
			got = append(got, id)
		}
	}
	want := readGolden(t, "writes.golden")
	if !slices.Equal(got, want) {
		t.Fatalf("writes set\n got %v\nwant %v", got, want)
	}
}

// chapter07 returns the path of docs/spec/07-permissions.md.
func chapter07(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("no caller")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..", "docs", "spec", "07-permissions.md")
}

var reasonCell = regexp.MustCompile("^\\| `([^`]+)` \\|")

// TestReasonsMatchChapter07 holds R-DM-28 on the side of authz: the reasons of this package are
// the first column of 07 section 4.1, in order, and equal the kernel/auditing.Reason constants.
func TestReasonsMatchChapter07(t *testing.T) {
	t.Parallel()
	f, err := os.Open(chapter07(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	var chapter []string
	in := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "### 4.1"):
			in = true
		case in && strings.HasPrefix(line, "### 4.2"):
			in = false
		case in:
			if m := reasonCell.FindStringSubmatch(line); m != nil {
				chapter = append(chapter, m[1])
			}
		}
	}
	if len(chapter) == 0 {
		t.Fatal("no reasons parsed from 07 section 4.1")
	}
	var ours, kernel []string
	for _, r := range domain.Reasons() {
		ours = append(ours, string(r))
	}
	for _, r := range auditing.KnownReasons() {
		kernel = append(kernel, string(r))
	}
	if !slices.Equal(ours, chapter) {
		t.Errorf("domain reasons\n got %v\nwant %v", ours, chapter)
	}
	if !slices.Equal(ours, kernel) {
		t.Errorf("domain reasons differ from kernel/auditing\n got %v\nwant %v", ours, kernel)
	}
}
