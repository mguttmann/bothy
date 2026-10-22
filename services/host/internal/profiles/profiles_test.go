// SPDX-License-Identifier: Apache-2.0

package profiles

import (
	"errors"
	"slices"
	"testing"

	"github.com/OWNER/bothy/services/host/internal/config"
	"github.com/OWNER/bothy/services/host/internal/doctor"
	"github.com/OWNER/bothy/services/host/internal/state"
)

func names(ps []doctor.Profile) []string {
	out := []string{}
	for _, p := range ps {
		out = append(out, p.Name)
	}
	return out
}

func detailOf(err error) string {
	var se *StartError
	if errors.As(err, &se) {
		return se.Detail
	}
	return ""
}

func TestOffered(t *testing.T) {
	all := []doctor.Profile{
		{Name: "shell", DisplayName: "Shell", Kind: config.KindShell},
		{Name: "claude", DisplayName: "Claude Code", Kind: config.KindClaude},
		{Name: "tool", DisplayName: "Tool", Kind: config.KindCommand},
	}
	withDirs := map[string][]doctor.Workdir{"bothy-s-alex": {{DisplayName: "repo", Path: "/home/bothy-s-alex/repo"}}, "bothy-s-sam": nil}
	noDirs := map[string][]doctor.Workdir{"bothy-s-alex": {}, "bothy-s-sam": nil}
	const bin = "/opt/claude/bin/claude"
	tests := []struct {
		name       string
		res        *doctor.Result
		documented bool
		want       []string
	}{
		{"nil result", nil, true, []string{}},
		{"no profiles", &doctor.Result{Declaration: state.DeclOrganization, ClaudeBin: bin, Workdirs: withDirs}, true, []string{}},
		{"organization with claude_bin and workdirs", &doctor.Result{Profiles: all, Declaration: state.DeclOrganization, ClaudeBin: bin, Workdirs: withDirs}, true, []string{"shell", "claude", "tool"}},
		{"organization with empty claude_bin", &doctor.Result{Profiles: all, Declaration: state.DeclOrganization, Workdirs: withDirs}, true, []string{"shell", "tool"}},
		// HD10 leaves ClaudeBin empty when the value counts as empty (interpreter not on the search list).
		{"organization with claude_bin counting as empty", &doctor.Result{Profiles: all, Declaration: state.DeclOrganization, ClaudeBin: "", SearchList: []string{"/opt/claude/bin"}, Workdirs: withDirs}, true, []string{"shell", "tool"}},
		{"organization without workdirs", &doctor.Result{Profiles: all, Declaration: state.DeclOrganization, ClaudeBin: bin, Workdirs: noDirs}, true, []string{"shell", "tool"}},
		{"organization with nil workdirs", &doctor.Result{Profiles: all, Declaration: state.DeclOrganization, ClaudeBin: bin}, true, []string{"shell", "tool"}},
		{"personal documented", &doctor.Result{Profiles: all, Declaration: state.DeclPersonal, Workdirs: withDirs}, true, []string{"shell", "claude", "tool"}},
		{"personal documented without workdirs", &doctor.Result{Profiles: all, Declaration: state.DeclPersonal, ClaudeBin: bin, Workdirs: noDirs}, true, []string{"shell", "tool"}},
		{"personal not documented (OQ-03 drop)", &doctor.Result{Profiles: all, Declaration: state.DeclPersonal, ClaudeBin: bin, Workdirs: withDirs}, false, []string{"shell", "tool"}},
		{"organization not touched by OQ-03", &doctor.Result{Profiles: all, Declaration: state.DeclOrganization, ClaudeBin: bin, Workdirs: withDirs}, false, []string{"shell", "claude", "tool"}},
		{"none", &doctor.Result{Profiles: all, Declaration: state.DeclNone, ClaudeBin: bin, Workdirs: withDirs}, true, []string{"shell", "tool"}},
		{"unknown", &doctor.Result{Profiles: all, Declaration: state.DeclUnknown, ClaudeBin: bin, Workdirs: withDirs}, true, []string{"shell", "tool"}},
		{"a value outside the list", &doctor.Result{Profiles: all, Declaration: "subscription", ClaudeBin: bin, Workdirs: withDirs}, true, []string{"shell", "tool"}},
		{"empty declaration", &doctor.Result{Profiles: all, ClaudeBin: bin, Workdirs: withDirs}, true, []string{"shell", "tool"}},
		{"unknown kind is never offered", &doctor.Result{Profiles: append(slices.Clone(all), doctor.Profile{Name: "x", Kind: "browser"}), Declaration: state.DeclOrganization, ClaudeBin: bin, Workdirs: withDirs}, true, []string{"shell", "claude", "tool"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := offered(tc.res, tc.documented)
			if got == nil {
				t.Fatal("nil list")
			}
			if !slices.Equal(names(got), tc.want) {
				t.Errorf("got %v, want %v", names(got), tc.want)
			}
		})
	}
	// The exported function uses the recorded OQ-03 result.
	res := &doctor.Result{Profiles: all, Declaration: state.DeclPersonal, Workdirs: withDirs}
	if got, want := names(Offered(res)), names(offered(res, PersonalLoginDocumented)); !slices.Equal(got, want) {
		t.Errorf("Offered %v, offered with the constant %v", got, want)
	}
	if !PersonalLoginDocumented || !slices.Contains(names(Offered(res)), "claude") {
		t.Error("OQ-03 found path A and B documented: personal must be offered")
	}
	if got := Offered(nil); got == nil || len(got) != 0 {
		t.Errorf("Offered(nil) = %#v", got)
	}
	// The offered items carry the reported fields unchanged and do not alias the result.
	got := Offered(&doctor.Result{Profiles: all, Declaration: state.DeclOrganization, ClaudeBin: bin, Workdirs: withDirs})
	if !slices.Equal(got, all) {
		t.Errorf("items %v", got)
	}
	got[0].Name = "changed"
	if all[0].Name != "shell" {
		t.Error("the list aliases the result")
	}
}

func TestStartError(t *testing.T) {
	err := error(&StartError{Op: "shell", Detail: DetailPTYFailed})
	if err.Error() != "profiles: shell: pty_failed" {
		t.Errorf("text %q", err.Error())
	}
	if detailOf(err) != DetailPTYFailed || DetailSharedLocation != "shared_location" || TERM != "xterm-256color" {
		t.Error("constants")
	}
}
