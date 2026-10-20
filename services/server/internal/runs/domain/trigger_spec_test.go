// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"bufio"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// repoRoot walks up from the package directory to the directory that holds docs/spec.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if st, err := os.Stat(filepath.Join(dir, "docs", "spec")); err == nil && st.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repository root with docs/spec not found")
		}
		dir = parent
	}
}

// specRejectionCodes reads the first cell of every row of the table under heading 3.5 of the
// agents chapter (docs/spec/09-*.md: the heading is matched by its number, so the test reads the
// chapter in either language of the repository).
func specRejectionCodes(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(repoRoot(t), "docs", "spec", "09-*.md"))
	if err != nil || len(files) != 1 {
		t.Fatalf("want exactly one agents chapter docs/spec/09-*.md, got %v (%v)", files, err)
	}
	f, err := os.Open(files[0])
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	var codes []string
	in := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "### ") {
			in = strings.HasPrefix(line, "### 3.5 ")
			continue
		}
		if !in || !strings.HasPrefix(line, "|") {
			continue
		}
		cell := strings.TrimSpace(strings.Split(line, "|")[1])
		if !strings.HasPrefix(cell, "`") {
			continue // header and separator rows
		}
		codes = append(codes, strings.Trim(cell, "`"))
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if len(codes) == 0 {
		t.Fatal("no rows found under heading 3.5 of the agents chapter")
	}
	return codes
}

// reasonConstants parses trigger_reasons.go and returns the values of every constant of type
// Reason, so a constant added without a spec row fails the test.
func reasonConstants(t *testing.T) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "trigger_reasons.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			vs := spec.(*ast.ValueSpec)
			if id, ok := vs.Type.(*ast.Ident); !ok || id.Name != "Reason" {
				continue
			}
			for _, v := range vs.Values {
				lit := v.(*ast.BasicLit)
				s, err := strconv.Unquote(lit.Value)
				if err != nil {
					t.Fatal(err)
				}
				out = append(out, s)
			}
		}
	}
	return out
}

// TestRejectionTableCoversSpec is the PLAN row of M2-04: every row of the rejection table of 09
// 3.5 has a test case, and the Reason constants are exactly the codes of the table.
func TestRejectionTableCoversSpec(t *testing.T) {
	spec := specRejectionCodes(t)
	consts := reasonConstants(t)
	sort.Strings(spec)
	sort.Strings(consts)
	if strings.Join(spec, ",") != strings.Join(consts, ",") {
		t.Fatalf("09 3.5 codes %v differ from the Reason constants %v", spec, consts)
	}

	viaDecide := func(mod func(*domain.MessageTrigger)) func(*testing.T) domain.Reason {
		return func(t *testing.T) domain.Reason {
			in := threadMsg()
			mod(&in)
			d := mustDecide(t, in)
			s, err := domain.Settle(d, domain.Verdict{Allowed: true})
			if err != nil || s.State != domain.TriggerStateFailed {
				t.Fatalf("Settle(%+v) = %+v, %v", d, s, err)
			}
			return d.Reason
		}
	}
	viaSettle := func(deny string, active bool) func(*testing.T) domain.Reason {
		return func(t *testing.T) domain.Reason {
			s, err := domain.Settle(invokeDecision(t, active), domain.Verdict{Reason: deny})
			if err != nil || s.State != domain.TriggerStateFailed {
				t.Fatalf("Settle = %+v, %v", s, err)
			}
			return s.Reason
		}
	}
	cases := map[string]func(*testing.T) domain.Reason{
		"multiple_agents_mentioned": viaDecide(func(in *domain.MessageTrigger) {
			in.Mentioned = []domain.AgentFacts{lotse(), texter()}
		}),
		"agent_not_allowed_here": viaDecide(func(in *domain.MessageTrigger) { in.Mentioned = []domain.AgentFacts{texter()} }),
		"no_trigger_right": viaDecide(func(in *domain.MessageTrigger) {
			in.AuthorHasTrigger = false
			in.Mentioned = []domain.AgentFacts{lotse()}
		}),
		"run_active_other_agent": viaDecide(func(in *domain.MessageTrigger) {
			in.ActiveRun = &domain.ActiveRun{RunID: runR, AgentID: texterID}
		}),
		"voice_unsupported":                viaDecide(func(in *domain.MessageTrigger) { in.Voice = true }),
		"agent_disabled":                   viaSettle("agent_not_reachable", false),
		"billing_mode_personal_not_holder": viaSettle("billing_mode_personal_not_holder", true),
		"project_not_ready":                viaSettle("project_not_ready", true),
		"run_active": func(t *testing.T) domain.Reason {
			d, err := domain.DecideCommand(domain.CommandTrigger{
				CommandID: commandID, ThreadID: threadT, Owner: lotse(),
				ActiveRun: &domain.ActiveRun{RunID: runR, AgentID: lotseID},
			})
			if err != nil || d.Outcome != domain.OutcomeReject {
				t.Fatalf("DecideCommand = %+v, %v", d, err)
			}
			return d.Reason
		},
	}
	for _, code := range spec {
		t.Run(code, func(t *testing.T) {
			produce, ok := cases[code]
			if !ok {
				t.Fatalf("row %q of 09 3.5 has no test case", code)
			}
			if got := produce(t); string(got) != code {
				t.Fatalf("case produced %q, want %q", got, code)
			}
		})
	}
}
