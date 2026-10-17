// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/instance/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// flagModules are the module keys of the fixture register.
var flagModules = []string{"chat", "files", "identity"}

// option returns a valid option flag of key.
func option(key string) domain.FlagSpec {
	return domain.FlagSpec{Key: key, Kind: domain.FlagKindOption, Description: "A switch", Module: "chat"}
}

func TestCheckRegistry(t *testing.T) {
	t.Parallel()
	release := domain.FlagSpec{Key: "chat.live_box", Kind: domain.FlagKindRelease, Description: "Live box", Module: "chat", RemoveBy: "1.4.0"}
	if got := domain.CheckRegistry([]domain.FlagSpec{option("dm.enabled"), release}, flagModules); len(got) != 0 {
		t.Fatalf("valid registry: %q", got)
	}
	tests := []struct {
		name string
		spec domain.FlagSpec
		want string
	}{
		{"one segment", option("dm"), "key"},
		{"upper case", option("dm.Enabled"), "key"},
		{"leading digit", option("dm.1st"), "key"},
		{"dash", option("dm.is-on"), "key"},
		{"unknown kind", domain.FlagSpec{Key: "dm.enabled", Kind: "temporary", Description: "x", Module: "chat"}, "kind"},
		{"blank description", domain.FlagSpec{Key: "dm.enabled", Kind: domain.FlagKindOption, Description: "  ", Module: "chat"}, "description"},
		{"unknown module", domain.FlagSpec{Key: "dm.enabled", Kind: domain.FlagKindOption, Description: "x", Module: "voice"}, "module"},
		{"release without remove_by", domain.FlagSpec{Key: "chat.x", Kind: domain.FlagKindRelease, Description: "x", Module: "chat"}, "remove_by"},
		{"release with v", domain.FlagSpec{Key: "chat.x", Kind: domain.FlagKindRelease, Description: "x", Module: "chat", RemoveBy: "v1.4.0"}, "remove_by"},
		{"release with bad version", domain.FlagSpec{Key: "chat.x", Kind: domain.FlagKindRelease, Description: "x", Module: "chat", RemoveBy: "1.4"}, "remove_by"},
		{"option with remove_by", domain.FlagSpec{Key: "dm.enabled", Kind: domain.FlagKindOption, Description: "x", Module: "chat", RemoveBy: "1.4.0"}, "remove_by"},
	}
	for _, tt := range tests {
		got := domain.CheckRegistry([]domain.FlagSpec{tt.spec}, flagModules)
		if len(got) != 1 || !strings.Contains(got[0], tt.want) || !strings.Contains(got[0], tt.spec.Key) {
			t.Errorf("%s: findings %q, want one naming %s", tt.name, got, tt.want)
		}
	}
	dup := domain.CheckRegistry([]domain.FlagSpec{option("dm.enabled"), option("dm.enabled")}, flagModules)
	if len(dup) != 1 || !strings.Contains(dup[0], "duplicate") {
		t.Errorf("duplicate: %q", dup)
	}
}

func TestReachedRemoveBy(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		version, removeBy string
		want              bool
	}{
		{"1.3.9", "1.4.0", false},
		{"1.4.0-beta.1", "1.4.0", false},
		{"1.4.0", "1.4.0", true},
		{"1.4.0+build.7", "1.4.0", true},
		{"2.0.0", "1.4.0", true},
		{"0.0.0", "0.1.0", false},
	} {
		got, err := domain.ReachedRemoveBy(tt.version, tt.removeBy)
		if err != nil || got != tt.want {
			t.Errorf("ReachedRemoveBy(%s, %s) = %v, %v; want %v", tt.version, tt.removeBy, got, err, tt.want)
		}
	}
	if _, err := domain.ReachedRemoveBy("v1.0.0", "1.0.0"); err == nil {
		t.Error("a version with v must fail")
	}
}

func TestEvaluate(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ global, in, want bool }{
		{false, false, false}, {true, false, true}, {false, true, true}, {true, true, false},
	} {
		if got := domain.Evaluate(tt.global, tt.in); got != tt.want {
			t.Errorf("Evaluate(%v, %v) = %v", tt.global, tt.in, got)
		}
	}
}

func TestInRollout(t *testing.T) {
	t.Parallel()
	sam, alex, agent := ids.New(), ids.New(), ids.New()
	research, support := ids.New(), ids.New()
	r := domain.Rollout{UserIDs: []ids.ID{sam, agent}, GroupIDs: []ids.ID{research}}
	tests := []struct {
		name   string
		kind   string
		id     ids.ID
		groups []ids.ID
		want   bool
	}{
		{"person direct", domain.SubjectUser, sam, nil, true},
		{"person via group", domain.SubjectUser, alex, []ids.ID{support, research}, true},
		{"person outside", domain.SubjectUser, alex, []ids.ID{support}, false},
		{"agent with its id in user_ids stays global", domain.SubjectAgent, agent, nil, false},
		{"app with a group stays global", domain.SubjectApp, agent, []ids.ID{research}, false},
	}
	for _, tt := range tests {
		if got := domain.InRollout(r, tt.kind, tt.id, tt.groups); got != tt.want {
			t.Errorf("%s: %v", tt.name, got)
		}
	}
	if domain.InRollout(domain.Rollout{}, domain.SubjectUser, sam, []ids.ID{research}) {
		t.Error("empty rollout")
	}
}

func TestNormalizeRollout(t *testing.T) {
	t.Parallel()
	a, b, g := ids.New(), ids.New(), ids.New()
	got := domain.NormalizeRollout(domain.Rollout{UserIDs: []ids.ID{b, a, b}, GroupIDs: []ids.ID{g, g}})
	want := domain.Rollout{UserIDs: []ids.ID{a, b}, GroupIDs: []ids.ID{g}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if e := domain.NormalizeRollout(domain.Rollout{UserIDs: []ids.ID{}}); e.UserIDs != nil || e.GroupIDs != nil || !e.Empty() {
		t.Errorf("empty lists must become nil: %#v", e)
	}
	if !domain.NormalizeRollout(domain.Rollout{UserIDs: []ids.ID{b, a}}).Equal(domain.NormalizeRollout(domain.Rollout{UserIDs: []ids.ID{a, b, a}})) {
		t.Error("Equal of normalized rollouts")
	}
	in := []ids.ID{b, a}
	domain.NormalizeRollout(domain.Rollout{UserIDs: in})
	if !slices.Equal(in, []ids.ID{b, a}) {
		t.Error("NormalizeRollout changed its input")
	}
}
