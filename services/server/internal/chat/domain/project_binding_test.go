// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// bindingLimits stand in for project.binding_text_max_chars and audit.reason_max_chars; the tests
// derive every boundary from them, never from literals.
var bindingLimits = domain.BindingLimits{TextMaxChars: 40, ReasonMaxChars: 60}

// str returns a pointer to s.
func str(s string) *string { return &s }

// bindingInput is a valid binding input the cases edit.
func bindingInput() domain.ProjectBindingInput {
	return domain.ProjectBindingInput{
		WorkspaceRef: "website", RepositoryURL: str("https://git.example.com/team/website"), DefaultBranch: str("main"),
		RunnerConfig: domain.RunnerConfig{Model: str("sonnet"), AllowedTools: []string{"Read", "Edit"}}, Reason: "Website project",
	}
}

// bindingCase is one table case: an edit of the valid input and the expected pointer:code list.
type bindingCase struct {
	name string
	edit func(*domain.ProjectBindingInput)
	want []string
}

// runBindingCases runs the cases against ValidateProjectBinding with bindingLimits.
func runBindingCases(t *testing.T, cases []bindingCase) {
	t.Helper()
	for _, c := range cases {
		in := bindingInput()
		c.edit(&in)
		if got := fieldCodes(domain.ValidateProjectBinding(in, bindingLimits)); !slices.Equal(got, c.want) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

// TestValidateProjectBinding_WorkspaceRef: the rules of design note 4 of M2-03 for workspace_ref.
func TestValidateProjectBinding_WorkspaceRef(t *testing.T) {
	t.Parallel()
	limit := bindingLimits.TextMaxChars
	set := func(v string) func(*domain.ProjectBindingInput) {
		return func(in *domain.ProjectBindingInput) { in.WorkspaceRef = v }
	}
	required, format, long := []string{"/workspace_ref:required"}, []string{"/workspace_ref:invalid_format"}, []string{"/workspace_ref:too_long"}
	runBindingCases(t, []bindingCase{
		{"plain", set("website"), nil},
		{"relative with slash", set("team/website"), nil},
		{"dots inside a name", set("team/web..site"), nil},
		{"at the limit", set(strings.Repeat("ü", limit)), nil},
		{"empty", set(""), required},
		{"white space only", set("   "), required},
		{"leading space", set(" website"), format},
		{"trailing space", set("website "), format},
		{"absolute path", set("/srv/website"), format},
		{"backslash path", set(`\srv\website`), format},
		{"drive letter", set(`C:\srv\website`), format},
		{"drive letter lower case", set("c:website"), format},
		{"home", set("~website"), format},
		{"option", set("-website"), format},
		{"parent segment", set("team/../website"), format},
		{"parent segment with backslash", set(`..\website`), format},
		{"only the parent", set(".."), format},
		{"one over the limit", set(strings.Repeat("ü", limit+1)), long},
	})
}

// TestValidateProjectBinding_RepositoryURL: an absolute http or https URL with a host and without
// user information (design note 4 of M2-03).
func TestValidateProjectBinding_RepositoryURL(t *testing.T) {
	t.Parallel()
	set := func(v *string) func(*domain.ProjectBindingInput) {
		return func(in *domain.ProjectBindingInput) { in.RepositoryURL = v }
	}
	format := []string{"/repository_url:invalid_format"}
	runBindingCases(t, []bindingCase{
		{"https", set(str("https://git.example.com/team/website")), nil},
		{"http", set(str("http://git.example.com/team/website")), nil},
		{"absent", set(nil), nil},
		{"ftp", set(str("ftp://git.example.com/x")), format},
		{"javascript", set(str("javascript:alert(1)")), format},
		{"no scheme", set(str("git.example.com/team/website")), format},
		{"user information", set(str("https://alex@git.example.com/x")), format},
		{"no host", set(str("https:///x")), format},
		{"empty", set(str("")), format},
		{"too long", set(str("https://git.example.com/" + strings.Repeat("x", bindingLimits.TextMaxChars))), []string{"/repository_url:too_long"}},
	})
}

// TestValidateProjectBinding_DefaultBranchAndRunnerConfig: the branch and the strings of the
// runner configuration, which the runner passes as process arguments (09 section 18.2).
func TestValidateProjectBinding_DefaultBranchAndRunnerConfig(t *testing.T) {
	t.Parallel()
	branch := func(v *string) func(*domain.ProjectBindingInput) {
		return func(in *domain.ProjectBindingInput) { in.DefaultBranch = v }
	}
	bformat := []string{"/default_branch:invalid_format"}
	runBindingCases(t, []bindingCase{
		{"main", branch(str("main")), nil},
		{"with slash", branch(str("release/2026")), nil},
		{"absent", branch(nil), nil},
		{"empty", branch(str("")), bformat},
		{"space", branch(str("my branch")), bformat},
		{"dot dot", branch(str("a..b")), bformat},
		{"option", branch(str("-main")), bformat},
		{"too long", branch(str(strings.Repeat("b", bindingLimits.TextMaxChars+1))), []string{"/default_branch:too_long"}},
		{"model absent", func(in *domain.ProjectBindingInput) { in.RunnerConfig.Model = nil }, nil},
		{"model empty", func(in *domain.ProjectBindingInput) { in.RunnerConfig.Model = str("") }, []string{"/runner_config/model:invalid_format"}},
		{"model option", func(in *domain.ProjectBindingInput) { in.RunnerConfig.Model = str("-x") }, []string{"/runner_config/model:invalid_format"}},
		{"model too long", func(in *domain.ProjectBindingInput) {
			in.RunnerConfig.Model = str(strings.Repeat("m", bindingLimits.TextMaxChars+1))
		}, []string{"/runner_config/model:too_long"}},
		{"tools absent", func(in *domain.ProjectBindingInput) { in.RunnerConfig.AllowedTools = nil }, nil},
		{"tools empty list", func(in *domain.ProjectBindingInput) { in.RunnerConfig.AllowedTools = []string{} }, nil},
		{"tool option", func(in *domain.ProjectBindingInput) { in.RunnerConfig.AllowedTools = []string{"Read", "-rf"} },
			[]string{"/runner_config/allowed_tools/1:invalid_format"}},
		{"tool empty", func(in *domain.ProjectBindingInput) { in.RunnerConfig.AllowedTools = []string{""} },
			[]string{"/runner_config/allowed_tools/0:invalid_format"}},
		{"tool too long", func(in *domain.ProjectBindingInput) {
			in.RunnerConfig.AllowedTools = []string{strings.Repeat("t", bindingLimits.TextMaxChars+1)}
		}, []string{"/runner_config/allowed_tools/0:too_long"}},
	})
}

// TestValidateProjectBinding_Reason: the mandatory reason (SEC-AUD-5) with audit.reason_max_chars.
func TestValidateProjectBinding_Reason(t *testing.T) {
	t.Parallel()
	set := func(v string) func(*domain.ProjectBindingInput) {
		return func(in *domain.ProjectBindingInput) { in.Reason = v }
	}
	runBindingCases(t, []bindingCase{
		{"at the limit", set(strings.Repeat("r", bindingLimits.ReasonMaxChars)), nil},
		{"blank", set(" \u00a0 "), []string{"/reason:required"}},
		{"empty", set(""), []string{"/reason:required"}},
		{"one over the limit", set(strings.Repeat("r", bindingLimits.ReasonMaxChars+1)), []string{"/reason:too_long"}},
	})
}

// TestValidateProjectBinding_AllErrorsAtOnce: every failing field is reported in one answer.
func TestValidateProjectBinding_AllErrorsAtOnce(t *testing.T) {
	t.Parallel()
	in := domain.ProjectBindingInput{WorkspaceRef: "/srv", RepositoryURL: str("ftp://x"), DefaultBranch: str("-x"),
		RunnerConfig: domain.RunnerConfig{Model: str("-m"), AllowedTools: []string{"-t"}}}
	want := []string{"/workspace_ref:invalid_format", "/repository_url:invalid_format", "/default_branch:invalid_format",
		"/runner_config/model:invalid_format", "/runner_config/allowed_tools/0:invalid_format", "/reason:required"}
	if got := fieldCodes(domain.ValidateProjectBinding(in, bindingLimits)); !slices.Equal(got, want) {
		t.Fatalf("%v, want %v", got, want)
	}
}

// forbiddenRunes are the code points kernel/textcheck rejects (SEC-IN-4): every C0 control, DEL,
// every C1 control and the bidi controls.
func forbiddenRunes() []rune {
	var out []rune
	for r := rune(0); r < 0x20; r++ {
		out = append(out, r)
	}
	for r := rune(0x7f); r <= 0x9f; r++ {
		out = append(out, r)
	}
	out = append(out, 0x061c, 0x200e, 0x200f)
	for r := rune(0x202a); r <= 0x202e; r++ {
		out = append(out, r)
	}
	for r := rune(0x2066); r <= 0x2069; r++ {
		out = append(out, r)
	}
	return out
}

// TestSEC_T07_ProjectBindingRejectsControlAndBidi: every code point of kernel/textcheck in every
// text of the binding gives invalid_characters on its pointer.
func TestSEC_T07_ProjectBindingRejectsControlAndBidi(t *testing.T) {
	t.Parallel()
	fields := []struct {
		pointer string
		set     func(*domain.ProjectBindingInput, string)
	}{
		{"/workspace_ref", func(in *domain.ProjectBindingInput, c string) { in.WorkspaceRef = "web" + c + "site" }},
		{"/repository_url", func(in *domain.ProjectBindingInput, c string) {
			in.RepositoryURL = str("https://git.example.com/a" + c + "b")
		}},
		{"/default_branch", func(in *domain.ProjectBindingInput, c string) { in.DefaultBranch = str("ma" + c + "in") }},
		{"/runner_config/model", func(in *domain.ProjectBindingInput, c string) { in.RunnerConfig.Model = str("son" + c + "net") }},
		{"/runner_config/allowed_tools/1", func(in *domain.ProjectBindingInput, c string) {
			in.RunnerConfig.AllowedTools = []string{"Read", "Ed" + c + "it"}
		}},
		{"/reason", func(in *domain.ProjectBindingInput, c string) { in.Reason = "Website" + c + "project" }},
	}
	for _, r := range forbiddenRunes() {
		for _, f := range fields {
			in := bindingInput()
			f.set(&in, string(r))
			want := []string{f.pointer + ":invalid_characters"}
			if got := fieldCodes(domain.ValidateProjectBinding(in, bindingLimits)); !slices.Equal(got, want) {
				t.Errorf("U+%04X in %s: %v, want %v", r, f.pointer, got, want)
			}
		}
	}
}

// TestPlanBindingPut: create, no-op and replace of design note 5 of M2-03.
func TestPlanBindingPut(t *testing.T) {
	t.Parallel()
	in := bindingInput()
	created := domain.PlanBindingPut(nil, in)
	if created.Kind != domain.BindingCreated || created.Next.Ready || created.Next.Version != 1 || created.ReadyReset {
		t.Fatalf("create: %+v", created)
	}
	if created.Next.WorkspaceRef != "website" || created.Next.RepositoryURL != "https://git.example.com/team/website" ||
		created.Next.DefaultBranch != "main" || !created.Next.RunnerConfig.Equal(in.RunnerConfig) {
		t.Fatalf("create values: %+v", created.Next)
	}
	cur := created.Next
	cur.Ready, cur.Version, cur.CreatedBy, cur.CreatedAt = true, 4, ids.New(), time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	same := domain.PlanBindingPut(&cur, in)
	if same.Kind != domain.BindingUnchanged || !same.Next.Ready || same.Next.Version != 4 || same.ReadyReset {
		t.Fatalf("no-op: %+v", same)
	}
	for _, c := range []struct {
		name string
		edit func(*domain.ProjectBindingInput)
	}{
		{"workspace", func(in *domain.ProjectBindingInput) { in.WorkspaceRef = "Website" }},
		{"repository omitted", func(in *domain.ProjectBindingInput) { in.RepositoryURL = nil }},
		{"branch", func(in *domain.ProjectBindingInput) { in.DefaultBranch = str("develop") }},
		{"branch omitted", func(in *domain.ProjectBindingInput) { in.DefaultBranch = nil }},
		{"model omitted", func(in *domain.ProjectBindingInput) { in.RunnerConfig.Model = nil }},
		{"tool order", func(in *domain.ProjectBindingInput) { in.RunnerConfig.AllowedTools = []string{"Edit", "Read"} }},
		{"tools omitted", func(in *domain.ProjectBindingInput) { in.RunnerConfig.AllowedTools = nil }},
		{"tools empty instead of absent", func(in *domain.ProjectBindingInput) { in.RunnerConfig.AllowedTools = []string{} }},
	} {
		next := bindingInput()
		c.edit(&next)
		got := domain.PlanBindingPut(&cur, next)
		if got.Kind != domain.BindingReplaced || got.Next.Ready || got.Next.Version != cur.Version+1 || !got.ReadyReset {
			t.Errorf("%s: %+v", c.name, got)
		}
		if got.Next.CreatedBy != cur.CreatedBy || !got.Next.CreatedAt.Equal(cur.CreatedAt) {
			t.Errorf("%s: creator or creation time changed: %+v", c.name, got.Next)
		}
	}
	cur.Ready = false
	if got := domain.PlanBindingPut(&cur, domain.ProjectBindingInput{WorkspaceRef: "other", Reason: "r"}); got.Kind != domain.BindingReplaced || got.ReadyReset {
		t.Fatalf("replace of a binding that was not ready: %+v", got)
	}
	// The tools list of "tools empty instead of absent" differs from an absent one, so the
	// absent list stays absent and the empty one stays empty.
	if got := domain.PlanBindingPut(nil, domain.ProjectBindingInput{WorkspaceRef: "w", RunnerConfig: domain.RunnerConfig{AllowedTools: []string{}}}); got.Next.RunnerConfig.AllowedTools == nil {
		t.Fatal("an empty tools list became absent")
	}
}
